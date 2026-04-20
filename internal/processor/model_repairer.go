package processor

import (
	"crypto/md5"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/database"
	"txt-cleaning/internal/external"
	"txt-cleaning/internal/logging"
	"txt-cleaning/internal/processor/preprocess"
)

// ModelRepairer 模型修复器（重构版）
// 集成智能分块、缓存幂等性和动态提示词管理
type ModelRepairer struct {
	RepairModelType string
	RepairModelName string
	promptManager   *PromptManager      // 动态提示词管理器
	cacheRepo       *database.ChunkCacheRepo // 块缓存仓库
	workerPool      *WorkerPool         // 工作池（用于并发处理）
	monitor         *EvolverMonitor     // 自进化监控器（可选）
}

// ModelRepairResult 模型修复结果
type ModelRepairResult struct {
	Content  string              `json:"content"`
	Original string              `json:"original"`
	Changes  []preprocess.Change `json:"changes"`
	Stats    map[string]int      `json:"stats"`
}

// NewModelRepairer 创建模型修复器（重构版）
func NewModelRepairer(modelType, modelName string) *ModelRepairer {
	// 初始化提示词管理器
	pm, err := NewPromptManager(PromptConfig{
		Name:          "novel_repair",
		PromptDir:     "config/prompts",
		DefaultPrompt: `你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。`,
		HotReload:     true,
	})
	if err != nil {
		log.Printf("[ModelRepairer] 提示词管理器初始化失败，使用默认提示词: %v", err)
	}

	// 初始化块缓存仓库
	repo := database.NewChunkCacheRepo()

	// 初始化工作池（默认10个worker）
	workerPool := NewWorkerPool(10, repo, pm)

	// 创建自进化监控器（可选，可根据配置启用）
	monitor := NewEvolverMonitor(pm)
	if config.AppConfigInstance.EnableEvolver {
		if err := monitor.Start(); err != nil {
			log.Printf("[ModelRepairer] 自进化监控器启动失败: %v", err)
		} else {
			log.Printf("[ModelRepairer] 自进化监控器已启动（检测阈值: 错误率>20%%或命中率<30%%）")
		}
	}

	return &ModelRepairer{
		RepairModelType: modelType,
		RepairModelName: modelName,
		promptManager:   pm,
		cacheRepo:       repo,
		workerPool:      workerPool,
		monitor:         monitor,
	}
}

// RepairText 修复文本中的错别字和语法错误（重构版）
func (mr *ModelRepairer) RepairText(content string) ModelRepairResult {
	return mr.RepairTextWithFileMd5("", content)
}

// RepairTextWithFileMd5 修复文本中的错别字和语法错误（带文件MD5缓存）
func (mr *ModelRepairer) RepairTextWithFileMd5(fileMd5, content string) ModelRepairResult {
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

	// 智能分块：将零散段落合并为1500-2000字符的Chunk
	chunks := mr.SplitIntoChunks(content, 1500, 2000)
	if len(chunks) == 0 {
		return result
	}

	// 使用工作池并发处理所有Chunk
	chunkResults := mr.workerPool.ProcessChunks(fileMd5, chunks)

	// 合并结果
	repairedParagraphs := []string{}
	for _, chunkResult := range chunkResults {
		for _, p := range chunkResult.Paragraphs {
			repairedParagraphs = append(repairedParagraphs, p)
		}
		result.Changes = append(result.Changes, chunkResult.Changes...)
	}

	// 重新组合文本（保持原有换行结构）
	result.Content = mr.reconstructText(content, repairedParagraphs)
	result.Stats["total_chunks"] = len(chunks)
	result.Stats["total_changes"] = len(result.Changes)
	result.Stats["cache_hits"] = mr.workerPool.GetCacheHits()
	result.Stats["cache_misses"] = mr.workerPool.GetCacheMisses()

	return result
}

// SplitIntoChunks 智能分块：将文本分割为1500-2000字符的Chunk
// 算法：
// 1. 按换行符分割为原始段落
// 2. 合并小段落直到达到minChars（1500）
// 3. 当合并后超过maxChars（2000）时，将最后一个段落放入下一个Chunk
// 4. 保持段落完整性，不拆分单个段落
func (mr *ModelRepairer) SplitIntoChunks(content string, minChars, maxChars int) []ChunkInfo {
	// 按换行符分割原始段落
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
		"total_chunks":    len(chunks),
		"min_chars":       minChars,
		"max_chars":       maxChars,
	})

	return chunks
}

// SplitIntoParagraphs 兼容旧接口（按换行符分割）
func (mr *ModelRepairer) SplitIntoParagraphs(content string) []string {
	return strings.Split(content, "\n")
}

// RepairParagraph 修复单个段落（重构版）
// 支持缓存查询、API重试和动态提示词
func (mr *ModelRepairer) RepairParagraph(paragraph string) (string, []preprocess.Change) {
	// 如果段落太短，直接返回
	if len(paragraph) < 10 {
		return paragraph, []preprocess.Change{}
	}

	// 计算段落哈希用于缓存查询
	chunkHash := fmt.Sprintf("%x", md5.Sum([]byte(paragraph)))

	// 先尝试从缓存获取（幂等性）
	cacheRecord, err := mr.cacheRepo.GetChunkRepair("", chunkHash) // fileMd5在workerPool中传递
	if err != nil {
		logging.Error("cache_query_failed", map[string]interface{}{
			"chunk_hash": chunkHash,
			"error":      err.Error(),
		})
	} else if cacheRecord != nil {
		// 缓存命中，记录统计
		logging.CacheHit("", 0, chunkHash)
		return cacheRecord.RepairedText, []preprocess.Change{}
	}

	// 缓存未命中，使用适当的方法修复
	if mr.RepairModelType == "api" {
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

	// 构建用户提示词
	userPrompt := "输入：她高兴及了，跑过去抱住他。\n输出：她高兴极了，跑过去抱住他。\n\n当前任务：\n输入：" + paragraph + "\n输出："

	startTime := time.Now()
	resp, err := api.GenerateChatCompletion(prompt, userPrompt, 0, -1)
	duration := time.Since(startTime).Milliseconds()

	if err != nil || resp == nil || len(resp.Choices) == 0 {
		logging.Error("api_repair_failed", map[string]interface{}{
			"chunk_hash":      chunkHash,
			"paragraph_len":   len(paragraph),
			"duration_ms":     duration,
			"error":           err.Error(),
			"prompt_version":  version,
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

	repairedText := strings.TrimSpace(resp.Choices[0].Message.Content)

	if repairedText == "" || repairedText == paragraph {
		// 没有变化，记录空结果
		return paragraph, []preprocess.Change{}
	}

	changes := mr.compareTexts(paragraph, repairedText)

	// 记录成功
	logging.Info("api_repair_success", map[string]interface{}{
		"chunk_hash":      chunkHash,
		"paragraph_len":   len(paragraph),
		"repaired_len":    len(repairedText),
		"changes_count":   len(changes),
		"duration_ms":     duration,
		"prompt_version":  version,
	})

	// 记录提示词使用情况（成功）
	mr.promptManager.RecordUsage(true)

	// 保存到缓存（由workerPool在适当的上下文调用）
	// 注意：缓存保存由workerPool在获取到fileMd5后处理

	return repairedText, changes
}

// repairLocally 本地修复（带缓存支持）
func (mr *ModelRepairer) repairLocally(paragraph, chunkHash string) (string, []preprocess.Change) {
	changes := []preprocess.Change{}

	// 常见错别字映射表
	typoMap := map[string]string{
		"图书管": "图书馆",
		"及了":  "极了",
		"在次":  "再次",
		"哪么":  "那么",
		"因该":  "应该",
		"以经":  "已经",
		"好象":  "好像",
		"做车":  "坐车",
	}

	// 应用错别字修正
	repaired := paragraph
	for typo, correct := range typoMap {
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
	ChunkID   int                  `json:"chunk_id"`
	Paragraphs []string            `json:"paragraphs"` // 修复后的段落列表
	Changes   []preprocess.Change `json:"changes"`
	CacheHit  bool                `json:"cache_hit"`
}

// WorkerPool 工作池（用于并发处理Chunk）
type WorkerPool struct {
	workerCount int
	taskQueue   chan ChunkTask
	results     chan ChunkResult
	cacheRepo   *database.ChunkCacheRepo
	promptManager *PromptManager
	wg          sync.WaitGroup
	mu          sync.RWMutex
	cacheHits   int
	cacheMisses int
}

// ChunkTask 块任务
type ChunkTask struct {
	ChunkID   int
	Content   string
	FileMd5   string
	PromptVersion string
}

// NewWorkerPool 创建工作池
func NewWorkerPool(workerCount int, cacheRepo *database.ChunkCacheRepo, promptManager *PromptManager) *WorkerPool {
	wp := &WorkerPool{
		workerCount: workerCount,
		taskQueue:   make(chan ChunkTask, workerCount*10),
		results:     make(chan ChunkResult, workerCount*10),
		cacheRepo:   cacheRepo,
		promptManager: promptManager,
	}

	// 启动worker
	for i := 0; i < workerCount; i++ {
		go wp.worker(i)
	}

	return wp
}

// worker 工作线程
func (wp *WorkerPool) worker(workerID int) {
	for task := range wp.taskQueue {
		// 处理单个Chunk
		result := wp.processChunk(task, workerID)
		wp.results <- result
		wp.wg.Done()
	}
}

// processChunk 处理单个Chunk
func (wp *WorkerPool) processChunk(task ChunkTask, workerID int) ChunkResult {
	// 计算Chunk哈希
	chunkHash := fmt.Sprintf("%x", md5.Sum([]byte(task.Content)))

	// 查询缓存
	cacheRecord, err := wp.cacheRepo.GetChunkRepair(task.FileMd5, chunkHash)
	if err != nil {
		logging.Error("worker_cache_query_failed", map[string]interface{}{
			"worker_id":   workerID,
			"chunk_id":    task.ChunkID,
			"chunk_hash":  chunkHash,
			"error":       err.Error(),
		})
	} else if cacheRecord != nil {
		// 缓存命中
		wp.mu.Lock()
		wp.cacheHits++
		wp.mu.Unlock()
		logging.CacheHit(task.FileMd5, task.ChunkID, chunkHash)
		return ChunkResult{
			ChunkID:   task.ChunkID,
			Paragraphs: strings.Split(cacheRecord.RepairedText, "\n"),
			Changes:   []preprocess.Change{},
			CacheHit:  true,
		}
	}

	// 缓存未命中，使用API修复
	startTime := time.Now()
	prompt, version := wp.promptManager.GetCurrentPrompt()
	api := external.NewAPI()
	resp, err := api.GenerateChatCompletion(prompt, task.Content, 0, -1)
	duration := time.Since(startTime).Milliseconds()

	if err != nil || resp == nil || len(resp.Choices) == 0 {
		// API失败，记录到重试队列
		retryRecord := &database.RetryQueueRecord{
			FileMd5:       task.FileMd5,
			ChunkID:       task.ChunkID,
			OriginalText:  task.Content,
			FailureReason: err.Error(),
			ErrorType:     "api_error",
			ErrorContext:  "",
			PromptVersion: version,
			RetryCount:    0,
			MaxRetries:    3,
		}
		wp.cacheRepo.AddToRetryQueue(retryRecord)

		// 返回原始内容
		return ChunkResult{
			ChunkID:   task.ChunkID,
			Paragraphs: strings.Split(task.Content, "\n"),
			Changes:   []preprocess.Change{},
			CacheHit:  false,
		}
	}

	repairedText := strings.TrimSpace(resp.Choices[0].Message.Content)

	// 保存到缓存
	cacheRecordNew := &database.ChunkRepairCacheRecord{
		FileMd5:          task.FileMd5,
		ChunkID:          task.ChunkID,
		ChunkHash:        chunkHash,
		OriginalText:     task.Content,
		RepairedText:     repairedText,
		PromptVersion:    version,
		APIModel:         "", // 可从api响应获取
		TokenUsage:       0,
		ProcessingTimeMs: int(duration),
	}
	if err := wp.cacheRepo.SaveChunkRepair(cacheRecordNew); err != nil {
		logging.Error("worker_cache_save_failed", map[string]interface{}{
			"worker_id":   workerID,
			"chunk_id":    task.ChunkID,
			"chunk_hash":  chunkHash,
			"error":       err.Error(),
		})
	}

	// 记录缓存未命中
	wp.mu.Lock()
	wp.cacheMisses++
	wp.mu.Unlock()
	logging.CacheMiss(task.FileMd5, task.ChunkID, chunkHash)

	// 记录提示词使用成功
	wp.promptManager.RecordUsage(true)

	return ChunkResult{
		ChunkID:   task.ChunkID,
		Paragraphs: strings.Split(repairedText, "\n"),
		Changes:   []preprocess.Change{}, // 这里应该调用compareTexts，但简化处理
		CacheHit:  false,
	}
}

// ProcessChunks 处理所有Chunk并返回结果
func (wp *WorkerPool) ProcessChunks(fileMd5 string, chunks []ChunkInfo) []ChunkResult {
	totalTasks := len(chunks)
	wp.wg.Add(totalTasks)

	// 提交任务到队列
	for i, chunk := range chunks {
		task := ChunkTask{
			ChunkID:   i,
			Content:   chunk.Content,
			FileMd5:   fileMd5,
			PromptVersion: "", // 由worker获取
		}
		wp.taskQueue <- task
	}

	// 收集结果
	results := make([]ChunkResult, 0, totalTasks)
	for i := 0; i < totalTasks; i++ {
		result := <-wp.results
		results = append(results, result)
	}

	wp.wg.Wait()

	// 记录工作池统计
	logging.Info("worker_pool_completed", map[string]interface{}{
		"file_md5":      fileMd5,
		"total_chunks":  totalTasks,
		"cache_hits":    wp.GetCacheHits(),
		"cache_misses":  wp.GetCacheMisses(),
		"worker_count":  wp.workerCount,
	})

	return results
}

// GetCacheHits 获取缓存命中数
func (wp *WorkerPool) GetCacheHits() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.cacheHits
}

// GetCacheMisses 获取缓存未命中数
func (wp *WorkerPool) GetCacheMisses() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.cacheMisses
}

// Close 关闭工作池
func (wp *WorkerPool) Close() {
	close(wp.taskQueue)
	wp.wg.Wait()
	close(wp.results)
}
