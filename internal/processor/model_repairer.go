package processor

import (
	"crypto/md5"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"voidtext/internal/config"
	"voidtext/internal/database"
	"voidtext/internal/external"
	"voidtext/internal/logging"
	"voidtext/internal/processor/preprocess"
)

// 默认分块大小常量
const (
	DefaultMinChunkSize = 1200
	DefaultMaxChunkSize = 1500
)

// ModelRepairerConfig 模型修复器配置
type ModelRepairerConfig struct {
	RepairModelType          string
	RepairModelName          string
	EnableLocalModel         bool
	LocalConfidenceThreshold float64
	LocalModelURL            string
	LocalModelName           string
	LocalModelTimeout        int
	LocalFallbackEnabled     bool
}

// ModelRepairer 模型修复器（混合架构版）
// 集成智能分块、缓存幂等性和动态提示词管理，支持本地优先+远程兜底
type ModelRepairer struct {
	config        ModelRepairerConfig
	promptManager *PromptManager           // 动态提示词管理器
	cacheRepo     *database.ChunkCacheRepo // 块缓存仓库
	workerPool    *ChunkWorkerPool              // 工作池（用于并发处理）
	monitor       *EvolverMonitor          // 自进化监控器（可选）
	ollamaClient  *external.OllamaClient   // Ollama客户端（本地模型）
}

// ModelRepairResult 模型修复结果
type ModelRepairResult struct {
	Content  string              `json:"content"`
	Original string              `json:"original"`
	Changes  []preprocess.Change `json:"changes"`
	Stats    map[string]int      `json:"stats"`
}

// NewModelRepairer 创建模型修复器（混合架构版）
func NewModelRepairer(modelType, modelName string) *ModelRepairer {
	cfg := ModelRepairerConfig{
		RepairModelType:          modelType,
		RepairModelName:          modelName,
		EnableLocalModel:         config.AppConfigInstance.EnableLocalModel,
		LocalConfidenceThreshold: config.AppConfigInstance.LocalConfidenceThreshold,
		LocalModelURL:            config.AppConfigInstance.LocalModelURL,
		LocalModelName:           config.AppConfigInstance.LocalModelName,
		LocalModelTimeout:        config.AppConfigInstance.LocalModelTimeout,
		LocalFallbackEnabled:     config.AppConfigInstance.LocalFallbackEnabled,
	}

	// 初始化提示词管理器
	pm, err := NewPromptManager(PromptConfig{
		Name:          "novel_repair",
		PromptDir:     "config/prompts",
		DefaultPrompt: `You are a professional Chinese novel proofreader. Please correct typos and grammatical errors in the following text while preserving the original meaning. Only output the corrected text, no explanations.`,
		HotReload:     true,
	})
	if err != nil {
		log.Printf("[ModelRepairer] Failed to initialize prompt manager, using default: %v", err)
	}

	// 初始化块缓存仓库
	repo := database.NewChunkCacheRepo()

	// 初始化工作池（默认10个worker）
	workerPool := NewChunkWorkerPool(1, repo, pm)

	// 创建自进化监控器（可选，可根据配置启用）
	monitor := NewEvolverMonitor(pm)
	if config.AppConfigInstance.EnableEvolver {
		if err := monitor.Start(); err != nil {
			log.Printf("[ModelRepairer] Failed to start evolver monitor: %v", err)
		} else {
			log.Printf("[ModelRepairer] Evolver monitor started (thresholds: error rate >20%% or hit rate <30%%)")
		}
	}

	// 初始化Ollama客户端（本地模型）
	var ollamaClient *external.OllamaClient
	if cfg.EnableLocalModel {
		ollamaClient = external.NewOllamaClient(
			cfg.LocalModelURL,
			cfg.LocalModelName,
			time.Duration(cfg.LocalModelTimeout)*time.Second,
		)
		log.Printf("[ModelRepairer] 本地模型已启用 (URL: %s, Model: %s, Timeout: %ds)",
			cfg.LocalModelURL,
			cfg.LocalModelName,
			cfg.LocalModelTimeout)
	} else {
		log.Printf("[ModelRepairer] 本地模型未启用，将使用远程API")
	}

	return &ModelRepairer{
		config:        cfg,
		promptManager: pm,
		cacheRepo:     repo,
		workerPool:    workerPool,
		monitor:       monitor,
		ollamaClient:  ollamaClient,
	}
}

// RepairText 修复文本中的错别字和语法错误（重构版）
func (mr *ModelRepairer) RepairText(content string) ModelRepairResult {
	return mr.RepairTextWithFileMd5("", content, false)
}

// RepairTextWithFileMd5 修复文本中的错别字和语法错误（带文件MD5缓存和状态恢复）
func (mr *ModelRepairer) RepairTextWithFileMd5(fileMd5, content string, resume bool) ModelRepairResult {
	result := ModelRepairResult{
		Content:  content,
		Original: content,
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	// 如果模型修复被禁用，直接返回
	if !config.AppConfigInstance.EnableModelRepair {
		return result
	}

	// 智能分块：将零散段落合并为800-1000字符的Chunk
	chunks := mr.SplitIntoChunks(content, DefaultMinChunkSize, DefaultMaxChunkSize)
	if len(chunks) == 0 {
		return result
	}

	// 使用工作池并发处理所有Chunk（支持恢复）
	chunkResults := mr.workerPool.ProcessChunks(fileMd5, chunks, resume)

	// 合并结果
	repairedParagraphs := []string{}
	for _, chunkResult := range chunkResults {
		for _, p := range chunkResult.Paragraphs {
			repairedParagraphs = append(repairedParagraphs, p)
		}
		// 调整每个 Change 的 Position，加上 chunk 在文件中的偏移量（ChunkID = chunk.StartIndex）
		for _, change := range chunkResult.Changes {
			adjustedChange := change
			adjustedChange.Position = change.Position + chunkResult.ChunkID
			result.Changes = append(result.Changes, adjustedChange)
		}
	}

	// 重新组合文本（保持原有换行结构）
	result.Content = mr.reconstructText(content, repairedParagraphs)

	// 获取详细统计
	localSuccess, localFailure, remoteFallback := mr.workerPool.GetLocalStats()

	result.Stats["total_chunks"] = len(chunks)
	result.Stats["total_changes"] = len(result.Changes)
	result.Stats["cache_hits"] = mr.workerPool.GetCacheHits()
	result.Stats["cache_misses"] = mr.workerPool.GetCacheMisses()
	result.Stats["local_success"] = localSuccess
	result.Stats["local_failure"] = localFailure
	result.Stats["remote_fallback"] = remoteFallback
	result.Stats["resume_mode"] = 0
	if resume {
		result.Stats["resume_mode"] = 1
	}

	// 完成处理状态
	if mr.workerPool.stateManager != nil {
		status := "completed"
		if localFailure+remoteFallback > len(chunks)/2 {
			status = "partial_failed"
		}
		mr.workerPool.stateManager.FinishProcessing(fileMd5, status, "")
	}

	return result
}

// SplitIntoChunks 智能分块：将文本分割为minChars-maxChars字符的Chunk
// 算法：
// 1. 按换行符分割为原始段落
// 2. 合并小段落直到达到minChars
// 3. 当合并后超过maxChars时，将最后一个段落放入下一个Chunk
// 4. 保持段落完整性，不拆分单个段落
func (mr *ModelRepairer) SplitIntoChunks(content string, minChars, maxChars int) []ChunkInfo {
	// 首先尝试按章节分割
	chapters := mr.splitByChapters(content, maxChars)
	if len(chapters) > 0 {
		logging.Info("chunking_by_chapters", map[string]interface{}{
			"total_chapters": len(chapters),
			"min_chars":      minChars,
			"max_chars":      maxChars,
		})
		return chapters
	}

	// 如果没有找到章节，回退到按换行符分割
	rawParagraphs := strings.Split(content, "\n")
	if len(rawParagraphs) == 0 {
		return []ChunkInfo{}
	}

	// 过滤空段落并记录原始索引
	paragraphs := []string{}
	paragraphIndices := []int{}
	for i, p := range rawParagraphs {
		if strings.TrimSpace(p) != "" {
			paragraphs = append(paragraphs, p)
			paragraphIndices = append(paragraphIndices, i)
		}
	}

	// 如果没有段落（所有行都为空），返回空
	if len(paragraphs) == 0 {
		return []ChunkInfo{}
	}

	// 处理超长段落：如果段落长度超过maxChars，按字符数分割
	processedParagraphs := []string{}
	processedIndices := []int{}
	for i, paragraph := range paragraphs {
		paragraphLen := utf8.RuneCountInString(paragraph)
		
		if paragraphLen <= maxChars {
			// 正常段落，直接添加
			processedParagraphs = append(processedParagraphs, paragraph)
			processedIndices = append(processedIndices, paragraphIndices[i])
		} else {
			// 超长段落，按maxChars分割
			runes := []rune(paragraph)
			for start := 0; start < len(runes); start += maxChars {
				end := start + maxChars
				if end > len(runes) {
					end = len(runes)
				}
				chunkText := string(runes[start:end])
				processedParagraphs = append(processedParagraphs, chunkText)
				processedIndices = append(processedIndices, paragraphIndices[i])
			}
		}
	}

	// 使用处理后的段落进行分块
	paragraphs = processedParagraphs
	paragraphIndices = processedIndices

	chunks := []ChunkInfo{}
	currentChunk := strings.Builder{}
	currentSize := 0
	currentStartIndex := 0

	for i, paragraph := range paragraphs {
		paragraphLen := utf8.RuneCountInString(paragraph)

		// 如果当前Chunk为空，开始新Chunk
		if currentSize == 0 {
			currentStartIndex = paragraphIndices[i]
			currentChunk.WriteString(paragraph)
			currentSize = paragraphLen
			continue
		}

		// 如果添加此段落会超过maxChars，结束当前Chunk
		if currentSize+paragraphLen+1 > maxChars { // +1 为换行符
			// 如果当前Chunk已经达到minChars，保存它
			if currentSize >= minChars {
				chunks = append(chunks, ChunkInfo{
					Content:      currentChunk.String(),
					StartIndex:   currentStartIndex,
					EndIndex:     paragraphIndices[i-1],
					OriginalSize: currentSize,
				})
				// 开始新Chunk
				currentChunk.Reset()
				currentChunk.WriteString(paragraph)
				currentSize = paragraphLen
				currentStartIndex = paragraphIndices[i]
			} else {
				// 当前Chunk小于minChars，继续添加段落
				currentChunk.WriteString("\n")
				currentChunk.WriteString(paragraph)
				currentSize += paragraphLen + 1
			}
		} else {
			// 添加段落到当前Chunk
			currentChunk.WriteString("\n")
			currentChunk.WriteString(paragraph)
			currentSize += paragraphLen + 1
		}
	}

	// 处理最后一个Chunk
	if currentSize > 0 {
		chunks = append(chunks, ChunkInfo{
			Content:      currentChunk.String(),
			StartIndex:   currentStartIndex,
			EndIndex:     paragraphIndices[len(paragraphs)-1],
			OriginalSize: currentSize,
		})
	}

	// 记录分块统计
	logging.Info("chunking_completed", map[string]interface{}{
		"total_paragraphs": len(paragraphs),
		"total_chunks":     len(chunks),
		"min_chars":        minChars,
		"max_chars":        maxChars,
	})

	return chunks
}

// SplitIntoParagraphs 兼容旧接口（按换行符分割）
func (mr *ModelRepairer) SplitIntoParagraphs(content string) []string {
	return strings.Split(content, "\n")
}

// splitByChapters 按章节分割文本
func (mr *ModelRepairer) splitByChapters(content string, maxChars int) []ChunkInfo {
	// 定义章节模式
	chapterPatterns := []string{
		`第[一二三四五六七八九十百千万零\d]+章`,      // 第1章, 第一章
		`第[一二三四五六七八九十百千万零\d]+节`,      // 第1节, 第一节
		`第[一二三四五六七八九十百千万零\d]+回`,      // 第1回, 第一回
		`第[一二三四五六七八九十百千万零\d]+集`,      // 第1集, 第一集
		`第[一二三四五六七八九十百千万零\d]+部分`,     // 第1部分, 第一部分
		`第[一二三四五六七八九十百千万零\d]+卷`,      // 第1卷, 第一卷
		`第[一二三四五六七八九十百千万零\d]+篇`,      // 第1篇, 第一篇
		`第[一二三四五六七八九十百千万零\d]+幕`,      // 第1幕, 第一幕
		`第[一二三四五六七八九十百千万零\d]+场`,      // 第1场, 第一场
		`第[一二三四五六七八九十百千万零\d]+段`,      // 第1段, 第一段
	}

	// 合并所有模式
	pattern := strings.Join(chapterPatterns, "|")
	re := regexp.MustCompile(pattern)

	// 查找所有章节标记
	matches := re.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return []ChunkInfo{}
	}

	chunks := []ChunkInfo{}
	
	// 将内容转换为rune数组以便正确处理中文字符
	runes := []rune(content)
	
	// 将字节索引转换为rune索引
	runeIndices := make([]int, len(matches))
	for i, match := range matches {
		// 将字节索引转换为rune索引
		bytePos := match[0]
		runePos := utf8.RuneCountInString(content[:bytePos])
		runeIndices[i] = runePos
	}
	
	// 处理每个章节
	for i, runePos := range runeIndices {
		start := runePos
		end := len(runes)
		
		// 如果不是最后一个章节，结束位置是下一个章节的开始
		if i < len(runeIndices)-1 {
			end = runeIndices[i+1]
		}
		
		// 提取章节内容
		chapterRunes := runes[start:end]
		chapterLen := len(chapterRunes)
		
		// 如果章节内容超过maxChars，需要进一步分割
		if chapterLen <= maxChars {
			// 章节大小合适，直接作为一个chunk
			chunks = append(chunks, ChunkInfo{
				Content:      string(chapterRunes),
				StartIndex:   start,
				EndIndex:     end - 1,
				OriginalSize: chapterLen,
			})
		} else {
			// 章节太大，需要按maxChars分割
			for chunkStart := 0; chunkStart < chapterLen; chunkStart += maxChars {
				chunkEnd := chunkStart + maxChars
				if chunkEnd > chapterLen {
					chunkEnd = chapterLen
				}
				
				// 提取子chunk
				chunkText := string(chapterRunes[chunkStart:chunkEnd])
				chunkSize := chunkEnd - chunkStart
				
				// 计算在原始文本中的位置
				chunkStartInOriginal := start + chunkStart
				chunkEndInOriginal := start + chunkEnd - 1
				
				chunks = append(chunks, ChunkInfo{
					Content:      chunkText,
					StartIndex:   chunkStartInOriginal,
					EndIndex:     chunkEndInOriginal,
					OriginalSize: chunkSize,
				})
			}
		}
	}

	return chunks
}

// RepairParagraph 修复单个段落（重构版）
// 支持缓存查询、API重试和动态提示词
func (mr *ModelRepairer) RepairParagraph(paragraph string) (string, []preprocess.Change) {
	// 如果段落太短，直接返回
	if len(paragraph) < 10 {
		return paragraph, []preprocess.Change{}
	}

	// 计算段落哈希用于缓存查询
	chunkHash := calculateChunkHash(paragraph)

	// 先尝试从缓存获取（幂等性）
	cacheRecord, err := mr.cacheRepo.GetChunkRepair("", chunkHash) // fileMd5在workerPool中传递
	if err != nil {
		logging.Error("cache_query_failed", nil, map[string]interface{}{
			"chunk_hash": chunkHash,
			"error":      err.Error(),
		})
	} else if cacheRecord != nil {
		// 缓存命中，记录统计
		logging.CacheHit("", 0, chunkHash)
		return cacheRecord.RepairedText, []preprocess.Change{}
	}

	// 缓存未命中，使用适当的方法修复
	if mr.config.RepairModelType == "api" {
		return mr.repairWithAPI(paragraph, chunkHash)
	}

	// 本地修复（简化实现）
	return mr.repairLocally(paragraph, chunkHash)
}

// repairWithAPI 使用外部API修复文本（重构版）
// 集成动态提示词、重试机制和缓存保存
func (mr *ModelRepairer) repairWithAPI(paragraph, chunkHash string) (string, []preprocess.Change) {
	// 获取当前提示词和版本
	prompt, version := mr.promptManager.GetCurrentPrompt()

	api := external.NewAPI()

	// 构建用户提示词（优化格式，避免特殊字符）
	cleanedParagraph := mr.cleanTextForAPI(paragraph)
	userPrompt := "输入：她高兴及了，跑过去抱住他。\n输出：她高兴极了，跑过去抱住他。\n\n当前任务：\n输入：" + cleanedParagraph + "\n输出："

	startTime := time.Now()
	resp, err := api.GenerateChatCompletion(prompt, userPrompt, 0, -1)
	duration := time.Since(startTime).Milliseconds()

	if err != nil || resp == nil || len(resp.Choices) == 0 {
		// 检查是否是400错误，如果是，尝试简化请求
		if strings.Contains(err.Error(), "400") {
			return mr.retryWithSimplifiedRequest(paragraph, chunkHash, prompt, version)
		}

		logging.Error("api_repair_failed", nil, map[string]interface{}{
			"chunk_hash":     chunkHash,
			"paragraph_len":  len(paragraph),
			"duration_ms":    duration,
			"error":          err.Error(),
			"prompt_version": version,
		})

		// 记录失败，添加到重试队列
		retryRecord := &database.RetryQueueRecord{
			FileMd5:       "", // 由调用者设置
			ChunkID:       0,  // 由调用者设置
			OriginalText:  paragraph,
			FailureReason: err.Error(),
			ErrorType:     "api_error",
			ErrorContext:  "",
			PromptVersion: version,
			RetryCount:    0,
			MaxRetries:    3,
		}
		mr.cacheRepo.AddToRetryQueue(retryRecord)

		// 回退到本地修复
		return mr.repairLocally(paragraph, chunkHash)
	}

	// 提取修复后的文本
	repairedText := resp.Choices[0].Message.Content

	// 清理修复后的文本
	repairedText = mr.cleanRepairedText(repairedText)

	if repairedText == "" || repairedText == paragraph {
		// 没有变化，记录空结果
		return paragraph, []preprocess.Change{}
	}

	changes := mr.compareTexts(paragraph, repairedText)

	// 记录成功
	logging.Info("api_repair_success", map[string]interface{}{
		"chunk_hash":     chunkHash,
		"paragraph_len":  len(paragraph),
		"repaired_len":   len(repairedText),
		"changes_count":  len(changes),
		"duration_ms":    duration,
		"prompt_version": version,
	})

	// 记录提示词使用情况（成功）
	mr.promptManager.RecordUsage(true)

	// 保存到缓存（由workerPool在适当的上下文调用）
	// 注意：缓存保存由workerPool在获取到fileMd5后处理

	return repairedText, changes
}

// cleanTextForAPI 清理文本，避免API请求格式问题
func (mr *ModelRepairer) cleanTextForAPI(text string) string {
	// 移除控制字符
	cleaned := strings.Map(func(r rune) rune {
		if r >= 32 && r != 127 || r == '\n' || r == '\t' {
			return r
		}
		return -1
	}, text)

	// 限制最大长度（避免API限制）
	maxLength := 1500
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength] + "..."
	}

	return cleaned
}

// cleanRepairedText 清理API返回的文本
func (mr *ModelRepairer) cleanRepairedText(text string) string {
	// 移除可能的JSON转义字符
	text = strings.ReplaceAll(text, "\\n", "\n")
	text = strings.ReplaceAll(text, "\\t", "\t")
	text = strings.ReplaceAll(text, "\\\"", "\"")

	// 移除首尾空白
	text = strings.TrimSpace(text)

	return text
}

// retryWithSimplifiedRequest 使用简化请求重试400错误
func (mr *ModelRepairer) retryWithSimplifiedRequest(paragraph, chunkHash, prompt, version string) (string, []preprocess.Change) {
	logging.Warn("api_400_error_retry", map[string]interface{}{
		"chunk_hash":    chunkHash,
		"paragraph_len": len(paragraph),
		"action":        "尝试简化请求",
	})

	// 简化段落内容
	simplifiedParagraph := mr.simplifyText(paragraph)
	if simplifiedParagraph == "" {
		simplifiedParagraph = paragraph[:min(len(paragraph), 500)] // 截断过长的文本
	}

	// 简化提示词
	simplifiedPrompt := "请修正以下文本中的错别字：" + simplifiedParagraph

	api := external.NewAPI()
	startTime := time.Now()
	resp, err := api.GenerateChatCompletion(simplifiedPrompt, "", 0, -1)
	duration := time.Since(startTime).Milliseconds()

	if err != nil || resp == nil || len(resp.Choices) == 0 {
		logging.Error("api_simplified_retry_failed", nil, map[string]interface{}{
			"chunk_hash":  chunkHash,
			"duration_ms": duration,
			"error":       err.Error(),
		})
		return mr.repairLocally(paragraph, chunkHash)
	}

	repairedText := mr.cleanRepairedText(resp.Choices[0].Message.Content)

	if repairedText == "" || repairedText == paragraph {
		return paragraph, []preprocess.Change{}
	}

	changes := mr.compareTexts(paragraph, repairedText)

	logging.Info("api_simplified_retry_success", map[string]interface{}{
		"chunk_hash":    chunkHash,
		"changes_count": len(changes),
		"duration_ms":   duration,
	})

	return repairedText, changes
}

// simplifyText 简化文本内容
func (mr *ModelRepairer) simplifyText(text string) string {
	// 移除特殊字符和过多空白
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n\n", "\n")

	// 限制长度
	if len(text) > 800 {
		text = text[:800] + "..."
	}

	return text
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// repairLocally 本地修复（带缓存支持）
func (mr *ModelRepairer) repairLocally(paragraph, chunkHash string) (string, []preprocess.Change) {
	changes := []preprocess.Change{}

	// 应用错别字修正
	repaired := paragraph
	for typo, correct := range CommonTypos {
		if strings.Contains(repaired, typo) && typo != correct {
			// 记录变更
			position := strings.Index(repaired, typo)
			changes = append(changes, preprocess.Change{
				Type:        "typo_correction",
				Original:    typo,
				Replacement: correct,
				Position:    position,
			})

			// 应用修正
			repaired = strings.Replace(repaired, typo, correct, 1)
		}
	}

	if len(changes) == 0 {
		return paragraph, []preprocess.Change{}
	}

	// 记录提示词使用情况（本地修复）
	mr.promptManager.RecordUsage(false)

	// 保存到缓存（由workerPool在适当的上下文调用）

	return repaired, changes
}

// compareTexts 比较两个文本，生成变更记录（与之前相同）
func (mr *ModelRepairer) compareTexts(original, repaired string) []preprocess.Change {
	changes := []preprocess.Change{}

	if original == repaired {
		return changes
	}

	origRunes := []rune(original)
	repRunes := []rune(repaired)

	if len(origRunes) == len(repRunes) {
		byteOffset := 0
		for i := 0; i < len(origRunes); i++ {
			if origRunes[i] != repRunes[i] {
				origStr := string(origRunes[i])
				repStr := string(repRunes[i])

				j := i + 1
				for j < len(origRunes) && origRunes[j] != repRunes[j] {
					origStr += string(origRunes[j])
					repStr += string(repRunes[j])
					j++
				}

				changes = append(changes, preprocess.Change{
					Type:        "character_correction",
					Original:    origStr,
					Replacement: repStr,
					Position:    byteOffset,
				})
				i = j - 1
			}
			byteOffset += len(string(origRunes[i]))
		}
	} else {
		minLen := len(origRunes)
		if len(repRunes) < minLen {
			minLen = len(repRunes)
		}

		byteOffset := 0
		i := 0
		for i < minLen {
			if origRunes[i] != repRunes[i] {
				origChunk := string(origRunes[i])
				repChunk := string(repRunes[i])

				j := i + 1
				oob := j < len(origRunes)
				rob := j < len(repRunes)
				for oob && rob && origRunes[j] != repRunes[j] {
					origChunk += string(origRunes[j])
					repChunk += string(repRunes[j])
					j++
					oob = j < len(origRunes)
					rob = j < len(repRunes)
				}

				changes = append(changes, preprocess.Change{
					Type:        "character_correction",
					Original:    origChunk,
					Replacement: repChunk,
					Position:    byteOffset,
				})
				byteOffset += len(string(origRunes[i]))
				i = j
			} else {
				byteOffset += len(string(origRunes[i]))
				i++
			}
		}

		if len(origRunes) > minLen {
			remaining := ""
			for k := minLen; k < len(origRunes); k++ {
				remaining += string(origRunes[k])
			}
			if remaining != "" {
				changes = append(changes, preprocess.Change{
					Type:        "text_deletion",
					Original:    remaining,
					Replacement: "",
					Position:    byteOffset,
				})
			}
		} else if len(repRunes) > minLen {
			remaining := ""
			for k := minLen; k < len(repRunes); k++ {
				remaining += string(repRunes[k])
			}
			if remaining != "" {
				changes = append(changes, preprocess.Change{
					Type:        "text_insertion",
					Original:    "",
					Replacement: remaining,
					Position:    byteOffset,
				})
			}
		}
	}

	return changes
}

// reconstructText 根据修复后的段落重建文本，保持原始换行结构
func (mr *ModelRepairer) reconstructText(original string, repairedParagraphs []string) string {
	// 按换行符分割原始文本
	lines := strings.Split(original, "\n")
	if len(lines) == 0 {
		return original
	}

	// 构建修复后的段落映射
	repairedMap := make(map[int]string)
	paragraphIndex := 0
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			if paragraphIndex < len(repairedParagraphs) {
				repairedMap[i] = repairedParagraphs[paragraphIndex]
				paragraphIndex++
			}
		}
	}

	// 重建文本
	result := strings.Builder{}
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		if repairedText, ok := repairedMap[i]; ok {
			result.WriteString(repairedText)
		} else {
			result.WriteString(line)
		}
	}

	return result.String()
}

// 以下为内部数据结构

// ChunkInfo 块信息
type ChunkInfo struct {
	Content      string `json:"content"`      // 块内容
	StartIndex   int    `json:"startIndex"`   // 在原始段落中的起始索引
	EndIndex     int    `json:"endIndex"`     // 在原始段落中的结束索引
	OriginalSize int    `json:"originalSize"` // 原始字符数
}

// ChunkResult 块处理结果
type ChunkResult struct {
	ChunkID    int                 `json:"chunk_id"`
	Paragraphs []string            `json:"paragraphs"` // 修复后的段落列表
	Changes    []preprocess.Change `json:"changes"`
	CacheHit   bool                `json:"cache_hit"`
}

// ChunkWorkerPool 块处理工作池
type ChunkWorkerPool struct {
	workerCount     int
	taskQueue       chan ChunkTask
	results         chan ChunkResult
	cacheRepo       *database.ChunkCacheRepo
	promptManager   *PromptManager
	progress        *ProcessingProgress
	stateManager    *StateManager
	healthManager   *HealthCheckManager
	ollamaClient    *external.OllamaClient
	enableLocal     bool
	localConfidence float64
	fallbackEnabled bool
	wg              sync.WaitGroup
	mu              sync.RWMutex
	cacheHits       int
	cacheMisses     int
	localSuccess    int
	localFailure    int
	remoteFallback  int
}

// ChunkTask 块任务
type ChunkTask struct {
	ChunkID       int
	Content       string
	FileMd5       string
	PromptVersion string
}

// NewWorkerPool 创建工作池（混合架构版）
func NewChunkWorkerPool(workerCount int, cacheRepo *database.ChunkCacheRepo, promptManager *PromptManager) *ChunkWorkerPool {
	wp := &ChunkWorkerPool{
		workerCount:     workerCount,
		taskQueue:       make(chan ChunkTask, workerCount*10),
		results:         make(chan ChunkResult, workerCount*10),
		cacheRepo:       cacheRepo,
		promptManager:   promptManager,
		enableLocal:     config.AppConfigInstance.EnableLocalModel,
		localConfidence: config.AppConfigInstance.LocalConfidenceThreshold,
		fallbackEnabled: config.AppConfigInstance.LocalFallbackEnabled,
		stateManager:    GetStateManager(),
		healthManager:   GetHealthManager(),
	}

	// 初始化Ollama客户端
	if wp.enableLocal {
		wp.ollamaClient = external.NewOllamaClient(
			config.AppConfigInstance.LocalModelURL,
			config.AppConfigInstance.LocalModelName,
			time.Duration(config.AppConfigInstance.LocalModelTimeout)*time.Second,
		)

		// 启动健康检查
		if wp.healthManager != nil {
			if err := wp.healthManager.Start(); err != nil {
				logging.Error("health_check_start_failed", nil, map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}

	// 启动worker
	for i := 0; i < workerCount; i++ {
		go wp.worker()
	}

	return wp
}

// SetProgress 设置进度追踪器
func (wp *ChunkWorkerPool) SetProgress(progress *ProcessingProgress) {
	wp.progress = progress
}

// worker 工作线程
func (wp *ChunkWorkerPool) worker() {
	for task := range wp.taskQueue {
		wp.processTask(task)
	}
}

// processTask 处理单个任务
func (wp *ChunkWorkerPool) processTask(task ChunkTask) {
	defer wp.wg.Done()

	// Recover from panic in processChunk to prevent ProcessChunks from hanging forever.
	// Without this, a panic skips wp.results <- result, and ProcessChunks waits for
	// N results that will never arrive.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ChunkWorkerPool] panic in processTask (chunk %d, file %s): %v", task.ChunkID, task.FileMd5, r)
			// Send original content as fallback result so ProcessChunks can continue
			wp.results <- ChunkResult{
				ChunkID:    task.ChunkID,
				Paragraphs: strings.Split(task.Content, "\n"),
				Changes:    []preprocess.Change{},
				CacheHit:   false,
			}
		}
	}()

	// 调用processChunk处理
	result := wp.processChunk(task)

	// 发送结果
	wp.results <- result
}

// calculateChunkHash 计算Chunk哈希
func calculateChunkHash(content string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(content)))
}

// processChunk 处理单个Chunk（混合架构版）
func (wp *ChunkWorkerPool) processChunk(task ChunkTask) ChunkResult {
	// 计算Chunk哈希
	chunkHash := calculateChunkHash(task.Content)

	// 更新块状态为处理中
	if wp.stateManager != nil {
		wp.stateManager.UpdateChunkState(task.FileMd5, task.ChunkID, "processing", "", 0, 0, "")
	}

	// 查询缓存
	cacheRecord, err := wp.cacheRepo.GetChunkRepair(task.FileMd5, chunkHash)
	if err != nil {
		logging.Error("worker_cache_query_failed", nil, map[string]interface{}{
			"chunk_id":   task.ChunkID,
			"chunk_hash": chunkHash,
			"error":      err.Error(),
		})
	} else if cacheRecord != nil {
		// 缓存命中
		wp.mu.Lock()
		wp.cacheHits++
		wp.mu.Unlock()
		logging.CacheHit(task.FileMd5, task.ChunkID, chunkHash)

		// 记录进度
		var processingMs int64
		if cacheRecord != nil {
			processingMs = int64(cacheRecord.ProcessingTimeMs)
		}
		if wp.progress != nil {
			wp.progress.RecordChunkComplete(true, processingMs, "cache")
		}

		// 更新块状态为已完成
		if wp.stateManager != nil {
			wp.stateManager.UpdateChunkState(task.FileMd5, task.ChunkID, "completed", "cache", 1.0, processingMs, "")
		}

		return ChunkResult{
			ChunkID:    task.ChunkID,
			Paragraphs: strings.Split(cacheRecord.RepairedText, "\n"),
			Changes:    []preprocess.Change{},
			CacheHit:   true,
		}
	}

	// 缓存未命中，使用混合策略处理
	var repairedText string
	var source string
	var confidence float64
	var errMsg string

	startTime := time.Now()

	// 策略1：优先使用本地模型（考虑健康状态）
	useLocal := wp.enableLocal && wp.ollamaClient != nil
	if useLocal && wp.healthManager != nil {
		useLocal = wp.healthManager.ShouldUseLocalModel()
	}

	logging.Info("chunk_processing_strategy", map[string]interface{}{
		"chunk_id":    task.ChunkID,
		"enableLocal": wp.enableLocal,
		"hasClient":   wp.ollamaClient != nil,
		"useLocal":    useLocal,
	})

	if useLocal {
		repairedText, confidence, errMsg = wp.processWithLocalModel(task.Content)
		source = "local"

		if errMsg == "" && confidence >= wp.localConfidence {
			// 本地模型成功且置信度达标
			wp.mu.Lock()
			wp.localSuccess++
			wp.mu.Unlock()
		} else {
			// 本地模型失败或置信度低，尝试远程API兜底
			shouldFallback := wp.fallbackEnabled
			if wp.healthManager != nil {
				shouldFallback = shouldFallback && wp.healthManager.ShouldFallbackToRemote()
			}

			if shouldFallback {
				repairedText, source, errMsg = wp.processWithRemoteAPI(task)
				if errMsg == "" {
					wp.mu.Lock()
					wp.remoteFallback++
					wp.mu.Unlock()
				} else {
					wp.mu.Lock()
					wp.localFailure++
					wp.mu.Unlock()
				}
			} else {
				// 不启用降级，使用原始文本
				repairedText = task.Content
				wp.mu.Lock()
				wp.localFailure++
				wp.mu.Unlock()
			}
		}
	} else {
		// 策略2：直接使用远程API
		repairedText, source, errMsg = wp.processWithRemoteAPI(task)
	}

	duration := time.Since(startTime).Milliseconds()

	// 记录进度
	if wp.progress != nil {
		success := errMsg == ""
		wp.progress.RecordChunkComplete(success, duration, source)
	}

	// 更新块状态
	if wp.stateManager != nil {
		status := "completed"
		if errMsg != "" {
			status = "failed"
		}
		wp.stateManager.UpdateChunkState(task.FileMd5, task.ChunkID, status, source, confidence, duration, errMsg)
	}

	// 如果有错误，记录到重试队列
	if errMsg != "" {
		_, version := wp.promptManager.GetCurrentPrompt()
		retryRecord := &database.RetryQueueRecord{
			FileMd5:       task.FileMd5,
			ChunkID:       task.ChunkID,
			OriginalText:  task.Content,
			FailureReason: errMsg,
			ErrorType:     "api_error",
			ErrorContext:  "",
			PromptVersion: version,
			RetryCount:    0,
			MaxRetries:    3,
		}
		wp.cacheRepo.AddToRetryQueue(retryRecord)

		// 返回原始内容
		return ChunkResult{
			ChunkID:    task.ChunkID,
			Paragraphs: strings.Split(task.Content, "\n"),
			Changes:    []preprocess.Change{},
			CacheHit:   false,
		}
	}

	// 保存到缓存
	_, version := wp.promptManager.GetCurrentPrompt()
	cacheRecordNew := &database.ChunkRepairCacheRecord{
		FileMd5:          task.FileMd5,
		ChunkID:          task.ChunkID,
		ChunkHash:        chunkHash,
		OriginalText:     task.Content,
		RepairedText:     repairedText,
		PromptVersion:    version,
		APIModel:         source,
		TokenUsage:       0,
		ProcessingTimeMs: int(duration),
		Confidence:       confidence,
		Source:           source,
	}
	if err := wp.cacheRepo.SaveChunkRepair(cacheRecordNew); err != nil {
		logging.Error("worker_cache_save_failed", nil, map[string]interface{}{
			"chunk_id":   task.ChunkID,
			"chunk_hash": chunkHash,
			"error":      err.Error(),
		})
	}

	// 记录缓存未命中
	wp.mu.Lock()
	wp.cacheMisses++
	wp.mu.Unlock()
	logging.CacheMiss(task.FileMd5, task.ChunkID, chunkHash)

	// 记录提示词使用成功
	wp.promptManager.RecordUsage(true)

	// 记录处理来源
	logging.ProcessingSource(task.FileMd5, task.ChunkID, source, confidence, duration, false)

	// 比较原文和修复后的文本，生成变更列表
	changes := wp.detectChanges(task.Content, repairedText)

	return ChunkResult{
		ChunkID:    task.ChunkID,
		Paragraphs: strings.Split(repairedText, "\n"),
		Changes:    changes,
		CacheHit:   false,
	}
}

// detectChanges 比较原文和修复后的文本，生成变更列表
func (wp *ChunkWorkerPool) detectChanges(original, repaired string) []preprocess.Change {
	if original == repaired {
		return []preprocess.Change{}
	}

	changes := []preprocess.Change{}

	// 按行分割
	origLines := strings.Split(original, "\n")
	repairLines := strings.Split(repaired, "\n")

	// 逐行比较
	maxLines := len(origLines)
	if len(repairLines) > maxLines {
		maxLines = len(repairLines)
	}

	position := 0
	for i := 0; i < maxLines; i++ {
		origLine := ""
		if i < len(origLines) {
			origLine = origLines[i]
		}

		repairLine := ""
		if i < len(repairLines) {
			repairLine = repairLines[i]
		}

		if origLine != repairLine {
			// 检测到变更
			change := preprocess.Change{
				Type:        "llm_fix",
				Original:    origLine,
				Replacement: repairLine,
				Position:    position,
				Confidence:  0.8,
			}
			changes = append(changes, change)
		}

		// 更新位置（包含换行符）
		if i < len(origLines) {
			position += len(origLines[i]) + 1 // +1 for \n
		}
	}

	return changes
}

// ProcessChunks 处理所有Chunk并返回结果（支持状态恢复）
func (wp *ChunkWorkerPool) ProcessChunks(fileMd5 string, chunks []ChunkInfo, resume bool) []ChunkResult {
	totalTasks := len(chunks)

	// 初始化状态管理器（恢复时也重新初始化，因为chunk数量可能变化）
	if wp.stateManager != nil {
		wp.stateManager.StartProcessing(fileMd5, totalTasks, "llm_fix")
	}

	// 初始化进度追踪器
	if wp.progress == nil {
		wp.progress = GlobalProgressTracker.StartTracking(fileMd5, totalTasks)
	}

	// 如果需要恢复，获取未完成的块
	chunksToProcess := chunks
	if resume && wp.stateManager != nil {
		unfinishedChunks, err := wp.stateManager.ResumeProcessing(fileMd5)
		if err == nil && len(unfinishedChunks) > 0 {
			// 只处理未完成的块
			filteredChunks := []ChunkInfo{}
			for _, chunkID := range unfinishedChunks {
				if chunkID < len(chunks) {
					filteredChunks = append(filteredChunks, chunks[chunkID])
				}
			}
			chunksToProcess = filteredChunks
			logging.Info("processing_resumed", map[string]interface{}{
				"file_md5":          fileMd5,
				"original_chunks":   len(chunks),
				"unfinished_chunks": len(unfinishedChunks),
				"chunks_to_process": len(chunksToProcess),
			})
		}
	}

	wp.wg.Add(len(chunksToProcess))

	// 在单独的 goroutine 中提交任务，避免与结果收集产生死锁。
	// taskQueue 和 results 通道的缓冲大小均为 workerCount*10（默认10），
	// 当待处理的 chunks 总数超过缓冲容量时，如果先全部提交再收集结果，
	// 会导致：提交阻塞等待 taskQueue 有空位 → worker 无法取任务 → 
	// worker 完成处理后 results 通道也被填满 → worker 也阻塞 → 全体死锁。
	// 将提交放在 goroutine 中，让主 goroutine 可以同时收集结果，形成
	// 提交→处理→收集 的流水线，各环节独立阻塞而不互相死锁。
	go func() {
		for _, chunk := range chunksToProcess {
			task := ChunkTask{
				ChunkID:       chunk.StartIndex, // 使用起始索引作为ChunkID
				Content:       chunk.Content,
				FileMd5:       fileMd5,
				PromptVersion: "", // 由worker获取
			}
			wp.taskQueue <- task
		}
	}()

	// 收集结果（与任务提交并发执行，避免通道缓冲打满导致的死锁）
	results := make([]ChunkResult, 0, len(chunksToProcess))
	for i := 0; i < len(chunksToProcess); i++ {
		result := <-wp.results
		results = append(results, result)
	}

	wp.wg.Wait()

	// 同步状态到数据库
	if wp.stateManager != nil {
		if err := wp.stateManager.SyncWithDatabase(fileMd5); err != nil {
			logging.Error("state_sync_failed", nil, map[string]interface{}{
				"file_md5": fileMd5,
				"error":    err.Error(),
			})
		}
	}

	// 记录工作池统计
	localSuccess, localFailure, remoteFallback := wp.GetLocalStats()
	logging.Info("worker_pool_completed", map[string]interface{}{
		"file_md5":         fileMd5,
		"total_chunks":     totalTasks,
		"processed_chunks": len(chunksToProcess),
		"cache_hits":       wp.GetCacheHits(),
		"cache_misses":     wp.GetCacheMisses(),
		"local_success":    localSuccess,
		"local_failure":    localFailure,
		"remote_fallback":  remoteFallback,
		"worker_count":     wp.workerCount,
		"resume_mode":      resume,
	})

	return results
}

// GetCacheHits 获取缓存命中数
func (wp *ChunkWorkerPool) GetCacheHits() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.cacheHits
}

// processWithLocalModel 使用本地模型处理文本
func (wp *ChunkWorkerPool) processWithLocalModel(content string) (string, float64, string) {
	if wp.ollamaClient == nil {
		return "", 0.0, "local model client not initialized"
	}

	logging.Info("local_model_processing_start", map[string]interface{}{
		"content_len": len(content),
		"model":       wp.ollamaClient.GetModelName(),
	})

	startTime := time.Now()
	repairedText, err := wp.ollamaClient.CorrectText(content)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		logging.Error("local_model_processing_failed", nil, map[string]interface{}{
			"duration_ms": duration,
			"error":       err.Error(),
			"model":       wp.ollamaClient.GetModelName(),
		})
		return "", 0.0, err.Error()
	}

	// 计算置信度
	confidence := wp.calculateConfidence(content, repairedText)

	logging.Info("local_model_processing_success", map[string]interface{}{
		"duration_ms":  duration,
		"confidence":   confidence,
		"model":        wp.ollamaClient.GetModelName(),
		"content_len":  len(content),
		"repaired_len": len(repairedText),
	})

	return repairedText, confidence, ""
}

// processWithRemoteAPI 使用远程API处理文本
func (wp *ChunkWorkerPool) processWithRemoteAPI(task ChunkTask) (string, string, string) {
	startTime := time.Now()
	prompt, _ := wp.promptManager.GetCurrentPrompt()
	api := external.NewAPI()
	resp, err := api.GenerateChatCompletion(prompt, task.Content, 0, -1)
	_ = time.Since(startTime)

	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return "", "remote", err.Error()
	}

	repairedText := strings.TrimSpace(resp.Choices[0].Message.Content)

	return repairedText, "remote", ""
}

// ConfidenceConfig 置信度计算配置
type ConfidenceConfig struct {
	SimilarityWeight     float64
	ReasonablenessWeight float64
	MinConfidence        float64
	MaxConfidence        float64
}

// DefaultConfidenceConfig 默认置信度配置
var DefaultConfidenceConfig = ConfidenceConfig{
	SimilarityWeight:     0.7,
	ReasonablenessWeight: 0.3,
	MinConfidence:        0.0,
	MaxConfidence:        1.0,
}

// calculateConfidence 计算本地模型处理结果的置信度
func (wp *ChunkWorkerPool) calculateConfidence(original, repaired string) float64 {
	if original == repaired {
		return DefaultConfidenceConfig.MaxConfidence
	}

	if len(original) == 0 || len(repaired) == 0 {
		return 0.0
	}

	similarity := wp.calculateEditDistanceSimilarity(original, repaired)
	reasonableness := wp.checkReasonableness(original, repaired)

	confidence := similarity*DefaultConfidenceConfig.SimilarityWeight +
		reasonableness*DefaultConfidenceConfig.ReasonablenessWeight

	if confidence < DefaultConfidenceConfig.MinConfidence {
		return DefaultConfidenceConfig.MinConfidence
	}
	if confidence > DefaultConfidenceConfig.MaxConfidence {
		return DefaultConfidenceConfig.MaxConfidence
	}

	return confidence
}

// calculateEditDistanceSimilarity 基于编辑距离计算相似度
func (wp *ChunkWorkerPool) calculateEditDistanceSimilarity(str1, str2 string) float64 {
	runes1 := []rune(str1)
	runes2 := []rune(str2)
	maxLen := max(len(runes1), len(runes2))

	if maxLen == 0 {
		return 1.0
	}

	editDist := wp.levenshteinDistance(runes1, runes2)
	similarity := 1.0 - float64(editDist)/float64(maxLen)

	if similarity < 0.0 {
		return 0.0
	}
	return similarity
}

// levenshteinDistance 计算 Levenshtein 编辑距离
func (wp *ChunkWorkerPool) levenshteinDistance(a, b []rune) int {
	m, n := len(a), len(b)

	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 0; i <= m; i++ {
		dp[i][0] = i
	}
	for j := 0; j <= n; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			dp[i][j] = min(
				dp[i-1][j]+1,
				min(dp[i][j-1]+1, dp[i-1][j-1]+cost),
			)
		}
	}

	return dp[m][n]
}

// CommonTypos 常见错别字映射
var CommonTypos = map[string]string{
	"图书管": "图书馆",
	"及了":  "极了",
	"在次":  "再次",
	"哪么":  "那么",
	"因该":  "应该",
	"以经":  "已经",
	"好象":  "好像",
	"做车":  "坐车",
}

// checkReasonableness 检查修改的合理性
func (wp *ChunkWorkerPool) checkReasonableness(original, repaired string) float64 {
	origLen := len([]rune(original))
	repLen := len([]rune(repaired))

	if origLen == 0 && repLen == 0 {
		return 1.0
	}

	lengthRatio := 1.0
	if origLen > 0 && repLen > 0 {
		maxLen := max(origLen, repLen)
		minLen := min(origLen, repLen)
		lengthRatio = float64(minLen) / float64(maxLen)
	}

	changeRatio := 0.0
	if origLen > 0 {
		editDist := wp.levenshteinDistance([]rune(original), []rune(repaired))
		changeRatio = 1.0 - float64(editDist)/float64(origLen)
		if changeRatio < 0.0 {
			changeRatio = 0.0
		}
	}

	reasonableness := (lengthRatio + changeRatio) / 2.0

	if reasonableness < 0.0 {
		return 0.0
	}
	if reasonableness > 1.0 {
		return 1.0
	}

	return reasonableness
}

// GetCacheMisses 获取缓存未命中数
func (wp *ChunkWorkerPool) GetCacheMisses() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.cacheMisses
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetLocalStats 获取本地模型统计
func (wp *ChunkWorkerPool) GetLocalStats() (int, int, int) {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.localSuccess, wp.localFailure, wp.remoteFallback
}

// GetHealthStatus 获取健康状态
func (wp *ChunkWorkerPool) GetHealthStatus() map[string]interface{} {
	if wp.healthManager == nil {
		return map[string]interface{}{
			"error": "health manager not initialized",
		}
	}

	statuses := wp.healthManager.GetAllStatuses()
	result := make(map[string]interface{})

	for service, status := range statuses {
		lastCheck := ""
		if !status.LastCheck.IsZero() {
			lastCheck = status.LastCheck.Format(time.RFC3339)
		}

		result[service] = map[string]interface{}{
			"healthy":         status.Healthy,
			"enabled":         status.Enabled,
			"last_check":      lastCheck,
			"error_count":     status.ErrorCount,
			"success_count":   status.SuccessCount,
			"total_checks":    status.TotalChecks,
			"avg_response_ms": status.AvgResponseMs,
		}
	}

	// 添加降级建议
	if wp.healthManager != nil {
		result["fallback_recommendation"] = wp.healthManager.GetFallbackRecommendation()
	}

	return result
}

// Close 关闭工作池
func (wp *ChunkWorkerPool) Close() {
	close(wp.taskQueue)
	wp.wg.Wait()
	close(wp.results)
}

// Close 关闭ModelRepairer，释放工作池等资源
func (mr *ModelRepairer) Close() {
	if mr.workerPool != nil {
		mr.workerPool.Close()
	}
}
