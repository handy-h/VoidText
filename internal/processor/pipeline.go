package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/database"
	"voidtext/internal/file"
	"voidtext/internal/processor/preprocess"
	"voidtext/internal/processor/rules"
)

const (
	StepCleaning   = "cleaning"
	StepIndexing   = "indexing"
	StepLlmFix     = "llm_fix"
	StepReview     = "review"
	StepFinalizing = "finalizing"
)

var stepOrder = [...]string{StepCleaning, StepIndexing, StepLlmFix, StepReview, StepFinalizing}

// PipelineResult 流水线处理结果
type PipelineResult struct {
	CurrentStep string `json:"currentStep"`
	NextStep    string `json:"nextStep"`
	Progress    int    `json:"progress"`
	Message     string `json:"message"`
}

// RulesConfig 文件级规则配置
type RulesConfig struct {
	EnableBasicCleaning   bool              `json:"enableBasicCleaning"`
	TraditionalToSimple   bool              `json:"traditionalToSimple"`
	EnableVectorDetection bool              `json:"enableVectorDetection"`
	SimilarityThreshold   float64           `json:"similarityThreshold"`
	EnableModelRepair     bool              `json:"enableModelRepair"`
	TypoMap               map[string]string `json:"typoMap"`
	AdBlacklist           []string          `json:"adBlacklist"`
}

// UnmarshalJSON 自定义反序列化，兼容 typoMap/adBlacklist 为字符串或数组/对象的格式
func (r *RulesConfig) UnmarshalJSON(data []byte) error {
	// 用中间结构避免无限递归
	type Alias RulesConfig
	var raw struct {
		Alias
		TypoMap     json.RawMessage `json:"typoMap"`
		AdBlacklist json.RawMessage `json:"adBlacklist"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = RulesConfig(raw.Alias)

	// 解析 typoMap：可能是 map[string]string 或字符串（"key=value,..." 或 JSON 对象字符串）
	if len(raw.TypoMap) > 0 {
		r.TypoMap = make(map[string]string)
		var m map[string]string
		if err := json.Unmarshal(raw.TypoMap, &m); err == nil {
			r.TypoMap = m
		} else {
			// 尝试作为字符串解析："错字=正字,错字2=正字2"
			var s string
			if err2 := json.Unmarshal(raw.TypoMap, &s); err2 == nil && s != "" {
				r.TypoMap = parseTypoMapString(s)
			}
		}
	} else {
		r.TypoMap = make(map[string]string)
	}

	// 解析 adBlacklist：可能是 []string 或字符串（"广告1,广告2"）
	if len(raw.AdBlacklist) > 0 {
		var arr []string
		if err := json.Unmarshal(raw.AdBlacklist, &arr); err == nil {
			r.AdBlacklist = arr
		} else {
			var s string
			if err2 := json.Unmarshal(raw.AdBlacklist, &s); err2 == nil && s != "" {
				r.AdBlacklist = parseBlacklistString(s)
			}
		}
	} else {
		r.AdBlacklist = []string{}
	}

	return nil
}

// parseTypoMapString 解析 "错字=正字,错字2=正字2" 格式的错别字映射
func parseTypoMapString(s string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

// parseBlacklistString 解析 "广告1,广告2" 格式的黑名单
func parseBlacklistString(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}

// DefaultRulesConfig 默认规则配置
func DefaultRulesConfig() RulesConfig {
	return RulesConfig{
		EnableBasicCleaning:   config.AppConfigInstance.EnableBasicCleaning,
		TraditionalToSimple:   config.AppConfigInstance.TraditionalToSimple,
		EnableVectorDetection: config.AppConfigInstance.EnableVectorDetection,
		SimilarityThreshold:   config.AppConfigInstance.VectorSimilarityThreshold,
		EnableModelRepair:     config.AppConfigInstance.EnableModelRepair,
		TypoMap:               make(map[string]string),
		AdBlacklist:           []string{},
	}
}

// ParseRulesConfig 解析规则配置JSON
func ParseRulesConfig(jsonStr string) RulesConfig {
	cfg := DefaultRulesConfig() // 先取默认值，确保缺失字段有合理默认值
	if jsonStr == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		log.Printf("[配置] 规则配置 JSON 解析失败，使用默认配置: %v", err)
		return DefaultRulesConfig()
	}
	// 防御：反序列化后阈值若为 0 或负数，恢复默认值
	if cfg.SimilarityThreshold <= 0 {
		cfg.SimilarityThreshold = config.AppConfigInstance.VectorSimilarityThreshold
	}
	// TypoMap 和 AdBlacklist 若为 nil，初始化为空集合
	if cfg.TypoMap == nil {
		cfg.TypoMap = make(map[string]string)
	}
	if cfg.AdBlacklist == nil {
		cfg.AdBlacklist = []string{}
	}
	return cfg
}

// GetNextStep 获取下一步骤
func GetNextStep(currentStep string) string {
	for i, step := range stepOrder {
		if step == currentStep {
			if i+1 < len(stepOrder) {
				return stepOrder[i+1]
			}
			return ""
		}
	}
	return stepOrder[0]
}

// GetStepIndex 获取步骤索引
func GetStepIndex(step string) int {
	for i, s := range stepOrder {
		if s == step {
			return i
		}
	}
	return 0
}

// CalculateProgress 计算进度百分比（使用浮点计算避免整数除法精度丢失）
func CalculateProgress(step string, stepProgress int) int {
	stepIdx := GetStepIndex(step)
	stepsTotal := len(stepOrder)
	stepWeight := 100.0 / float64(stepsTotal)
	baseProgress := float64(stepIdx) * stepWeight
	stepContribution := float64(stepProgress) * stepWeight / 100.0
	return int(baseProgress + stepContribution)
}

// ProcessStep 执行单个处理步骤
func ProcessStep(fileMd5 string, step string) (*PipelineResult, error) {
	record, err := database.GetFileByMd5(fileMd5)
	if err != nil {
		return nil, fmt.Errorf("查询文件记录失败: %w", err)
	}
	if record == nil {
		return nil, fmt.Errorf("文件不存在: %s", fileMd5)
	}

	content, err := os.ReadFile(record.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	rulesConfig := ParseRulesConfig(record.RulesConfig)

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    step,
		Action:  "start",
		Status:  "running",
	})

	var result *PipelineResult

	switch step {
	case StepCleaning:
		// 使用 preprocess.PreprocessBytes 进行编码检测和乱码修复
		// 这是编码检测的唯一入口，确保后续步骤处理的都是有效 UTF-8 文本
		preprocessResult, pErr := preprocess.PreprocessBytes(content)
		if pErr != nil {
			return nil, fmt.Errorf("预处理编码检测失败: %w", pErr)
		}
		if len(preprocessResult.Changes) > 0 {
			log.Printf("[流水线] 预处理完成: 编码修复/乱码清理变更数=%d", len(preprocessResult.Changes))
		}
		result, err = processCleaningStep(fileMd5, preprocessResult, rulesConfig, record)
	case StepIndexing:
		result, err = processIndexingStep(fileMd5, string(content), rulesConfig, record)
	case StepLlmFix:
		result, err = processLlmFixStep(fileMd5, string(content), rulesConfig, record)
	case StepReview:
		result, err = processReviewStep(fileMd5, string(content), record)
	case StepFinalizing:
		result, err = processFinalizingStep(fileMd5, string(content), record)
	default:
		err = fmt.Errorf("未知步骤: %s", step)
	}

	if err != nil {
		database.UpdateFileStatus(fileMd5, "failed", step, 0, err.Error())
		database.CreateProcessingLog(&database.ProcessingLogRecord{
			FileMd5: fileMd5,
			Step:    step,
			Action:  "error",
			Details: err.Error(),
			Status:  "failed",
		})
		return nil, err
	}

	return result, nil
}

// processCleaningStep 基础清洗步骤
// preprocessResult 包含编码检测和乱码修复的预处理结果
func processCleaningStep(fileMd5 string, preprocessResult preprocess.PreprocessResult, rulesConfig RulesConfig, _ *database.FileRecord) (*PipelineResult, error) {
	// 合并预处理阶段的变更（编码修复、乱码清理）到清洗结果中
	allChanges := make([]preprocess.Change, 0, len(preprocessResult.Changes))
	allChanges = append(allChanges, preprocessResult.Changes...)

	if !rulesConfig.EnableBasicCleaning {
		nextStep := GetNextStep(StepCleaning)
		database.UpdateFileStatus(fileMd5, "processing", nextStep, CalculateProgress(nextStep, 0), "")
		return &PipelineResult{
			CurrentStep: StepCleaning,
			NextStep:    nextStep,
			Progress:    CalculateProgress(nextStep, 0),
			Message:     "基础清洗已跳过",
		}, nil
	}

	cleaner := NewBasicCleaner(rulesConfig.TraditionalToSimple)
	cleanResult := cleaner.Clean(preprocessResult.Content)

	// 合并 BasicCleaner 的变更
	allChanges = append(allChanges, cleanResult.Changes...)

	if len(rulesConfig.AdBlacklist) > 0 {
		for _, pattern := range rulesConfig.AdBlacklist {
			cleanResult.Content = removeAdContent(cleanResult.Content, pattern)
		}
	}

	if len(rulesConfig.TypoMap) > 0 {
		for wrong, correct := range rulesConfig.TypoMap {
			// 对包含 ASCII 字符的规则使用词边界，降低子串误替换风险
			hasASCII := false
			for _, r := range wrong {
				if r < 128 {
					hasASCII = true
					break
				}
			}
			if hasASCII {
				re, err := regexp.Compile(`\b` + regexp.QuoteMeta(wrong) + `\b`)
				if err == nil {
					cleanResult.Content = re.ReplaceAllString(cleanResult.Content, correct)
					continue
				}
			}
			cleanResult.Content = strings.ReplaceAll(cleanResult.Content, wrong, correct)
		}
	}

	ruleMgr := rules.NewRuleManager()
	cleanResult.Content = ruleMgr.ApplyRules(cleanResult.Content)

	// 换行修复：为缺少换行符的文本智能添加段落分隔
	newlineFixer := NewNewlineFixer()
	fixResult := newlineFixer.Fix(cleanResult.Content)
	if fixResult.Content != cleanResult.Content {
		cleanResult.Content = fixResult.Content
		log.Printf("[基础清洗] 换行修复完成: 段落数=%d, 修改=%d", fixResult.Stats["total_paragraphs"], len(fixResult.Changes))
	}

	if err := saveIntermediateFile(fileMd5, StepCleaning, cleanResult.Content); err != nil {
		return nil, fmt.Errorf("保存中间文件失败: %w", err)
	}

	nextStep := GetNextStep(StepCleaning)
	progress := CalculateProgress(nextStep, 0)
	database.UpdateFileStatus(fileMd5, "processing", nextStep, progress, "")

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepCleaning,
		Action:  "complete",
		Details: fmt.Sprintf("修改数: %d (编码修复: %d)", len(allChanges), len(preprocessResult.Changes)),
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepCleaning,
		NextStep:    nextStep,
		Progress:    progress,
		Message:     fmt.Sprintf("基础清洗完成，修改数: %d (编码修复: %d)", len(allChanges), len(preprocessResult.Changes)),
	}, nil
}

// processIndexingStep 向量索引与检测步骤
func processIndexingStep(fileMd5, content string, rulesConfig RulesConfig, _ *database.FileRecord) (*PipelineResult, error) {
	if !rulesConfig.EnableVectorDetection {
		nextStep := GetNextStep(StepIndexing)
		database.UpdateFileStatus(fileMd5, "processing", nextStep, CalculateProgress(nextStep, 0), "")
		return &PipelineResult{
			CurrentStep: StepIndexing,
			NextStep:    nextStep,
			Progress:    CalculateProgress(nextStep, 0),
			Message:     "向量检测已跳过",
		}, nil
	}

	detector := NewVectorDetector(
		rulesConfig.SimilarityThreshold,
		config.AppConfigInstance.VectorModelType,
		config.AppConfigInstance.VectorModelName,
	)
	detectResult, err := detector.DetectDuplicates(content)
	if err != nil {
		return nil, fmt.Errorf("向量检测失败: %w", err)
	}

	// 重复段检测仅产生待审核记录，不直接从内容里删除：
	// 保持下一步 (LLM 修复) 段索引与原文一致，让用户在审核阶段决定是否真删
	if err := saveIntermediateFile(fileMd5, StepIndexing, content); err != nil {
		return nil, fmt.Errorf("保存中间文件失败: %w", err)
	}

	// 用 change.Original 精确匹配段索引（vector_detector 输出按段顺序产生 change）
	// 构建索引时 trim 每个段落并跳过空行，与 splitIntoParagraphs 处理保持一致
	paragraphs := strings.Split(content, "\n")
	indexByText := make(map[string][]int, len(paragraphs))
	for i, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue // 跳过空行，与 splitIntoParagraphs 保持一致
		}
		indexByText[trimmed] = append(indexByText[trimmed], i)
	}
	var records []database.ReviewParagraphRecord
	var removedContents []string
	for _, change := range detectResult.Changes {
		if change.Type != "duplicate_paragraph" {
			continue
		}
		removedContents = append(removedContents, change.Original)
		candidates := indexByText[change.Original]
		if len(candidates) == 0 {
			continue
		}
		// 同一文本可能多次出现，每次按顺序消费一个段索引
		paragraphIndex := candidates[0]
		indexByText[change.Original] = candidates[1:]
		records = append(records, database.ReviewParagraphRecord{
			FileMd5:          fileMd5,
			ParagraphIndex:   paragraphIndex,
			OriginalText:     change.Original,
			SuggestedText:    "",
			ModificationType: "duplicate_paragraph",
			Status:           "pending",
		})
	}
	if len(records) > 0 {
		if err := database.CreateReviewParagraphs(records); err != nil {
			return nil, fmt.Errorf("写入段级审核记录失败: %w", err)
		}
	}

	// 构建结构化日志详情
	dupCount := len(records)
	charsRemoved := detectResult.Stats["duplicate_chars_removed"]
	logDetails := fmt.Sprintf("检测到重复段: %d, 减少文字: %d", dupCount, charsRemoved)
	if len(removedContents) > 0 {
		// 截断控制：最多存 20 条，总字符超过 5000 时截断
		truncated := false
		displayContents := removedContents
		if len(displayContents) > 20 {
			displayContents = displayContents[:20]
			truncated = true
		}
		totalChars := 0
		for _, c := range displayContents {
			totalChars += len([]rune(c))
		}
		if totalChars > 5000 {
			displayContents = displayContents[:10]
			truncated = true
		}
		detailsMap := map[string]interface{}{
			"action":               "vector_dedup_complete",
			"duplicate_paragraphs": dupCount,
			"duplicate_chars":      charsRemoved,
			"removed_contents":     displayContents,
		}
		if truncated {
			detailsMap["truncated"] = true
			detailsMap["total_removed"] = len(removedContents)
		}
		detailsJSON, err := json.Marshal(detailsMap)
		if err == nil {
			logDetails = string(detailsJSON)
		}
	}

	nextStep := GetNextStep(StepIndexing)
	progress := CalculateProgress(nextStep, 0)
	database.UpdateFileStatus(fileMd5, "processing", nextStep, progress, "")

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepIndexing,
		Action:  "complete",
		Details: logDetails,
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepIndexing,
		NextStep:    nextStep,
		Progress:    progress,
		Message:     fmt.Sprintf("向量检测完成，去除重复段: %d, 减少文字: %d", dupCount, charsRemoved),
	}, nil
}

// LlmCheckpoint LLM修复断点数据
type LlmCheckpoint struct {
	ParagraphIndex     int                              `json:"paragraphIndex"`
	RepairedParagraphs []string                         `json:"repairedParagraphs"`
	Records            []database.ReviewParagraphRecord `json:"records"`
}

type llmParagraphResult struct {
	Index    int
	Repaired string
	Err      error
}

// processLlmFixStep LLM修复步骤（支持断点恢复和取消）
func processLlmFixStep(fileMd5, content string, rulesConfig RulesConfig, record *database.FileRecord) (*PipelineResult, error) {
	if !rulesConfig.EnableModelRepair {
		nextStep := GetNextStep(StepLlmFix)
		database.UpdateFileStatus(fileMd5, "processing", nextStep, CalculateProgress(nextStep, 0), "")
		return &PipelineResult{
			CurrentStep: StepLlmFix,
			NextStep:    nextStep,
			Progress:    CalculateProgress(nextStep, 0),
			Message:     "LLM修复已跳过",
		}, nil
	}

	// 步前取消检查：覆盖小文件等无法触发 step 间检查的场景
	if cancelled, _ := database.IsFileCancelled(fileMd5); cancelled {
		database.SetCancelFlag(fileMd5, 0)
		database.UpdateFileStatus(fileMd5, "cancelled", StepLlmFix, 0, "用户取消")
		return &PipelineResult{
			CurrentStep: StepLlmFix,
			NextStep:    "",
			Progress:    CalculateProgress(StepLlmFix, 0),
			Message:     "LLM修复已取消",
		}, nil
	}

	repairer := NewModelRepairer(
		config.AppConfigInstance.RepairModelType,
		config.AppConfigInstance.RepairModelName,
	)

	// 段落重组：使用LLM智能识别语义段落边界，合并被硬切断的行
	if config.AppConfigInstance.EnableLlmParagraphReconstruct {
		database.CreateProcessingLog(&database.ProcessingLogRecord{
			FileMd5: fileMd5,
			Step:    StepLlmFix,
			Action:  "progress",
			Details: "段落重组开始，正在智能识别段落边界...",
			Status:  "running",
		})
		origLen := len([]rune(content))
		reconstructed, err := repairer.ReconstructParagraphsWithCheckpoint(content, fileMd5, func(done, total int) {
			// 更新数据库进度：段落重组占 llm_fix 步骤的前 50% 进度
			stepProgress := done * 50 / total
			progress := CalculateProgress(StepLlmFix, stepProgress)
			database.UpdateFileStatus(fileMd5, "processing", StepLlmFix, progress,
				fmt.Sprintf("段落重组: %d/%d 块完成", done, total))
			database.CreateProcessingLog(&database.ProcessingLogRecord{
				FileMd5: fileMd5,
				Step:    StepLlmFix,
				Action:  "progress",
				Details: fmt.Sprintf("段落重组进度: %d/%d 块完成", done, total),
				Status:  "running",
			})
			log.Printf("[段落重组] 进度: %d/%d 块完成 (进度=%d%%)", done, total, progress)
		})
		if err != nil {
			log.Printf("[段落重组] 失败，回退到原始段落结构: %v", err)
			database.CreateProcessingLog(&database.ProcessingLogRecord{
				FileMd5: fileMd5,
				Step:    StepLlmFix,
				Action:  "progress",
				Details: fmt.Sprintf("段落重组失败，回退到原始结构: %v", err),
				Status:  "warning",
			})
		} else {
			content = reconstructed
			log.Printf("[段落重组] 完成: 原始长度=%d字符, 重组后长度=%d字符",
				origLen, len([]rune(reconstructed)))
			database.CreateProcessingLog(&database.ProcessingLogRecord{
				FileMd5: fileMd5,
				Step:    StepLlmFix,
				Action:  "progress",
				Details: fmt.Sprintf("段落重组完成: 原始%d字符 → 重组后%d字符", origLen, len([]rune(reconstructed))),
				Status:  "success",
			})
		}
	}

	paragraphs := repairer.SplitIntoParagraphs(content)
	totalParagraphs := len(paragraphs)
	if totalParagraphs == 0 {
		nextStep := GetNextStep(StepLlmFix)
		database.UpdateFileStatus(fileMd5, "processing", nextStep, CalculateProgress(nextStep, 0), "")
		return &PipelineResult{
			CurrentStep: StepLlmFix,
			NextStep:    nextStep,
			Progress:    CalculateProgress(nextStep, 0),
			Message:     "LLM修复完成，无需处理的段落",
		}, nil
	}

	// 尝试从断点恢复
	startIndex := 0
	repairedParagraphs := make([]string, totalParagraphs)
	for i := range repairedParagraphs {
		repairedParagraphs[i] = paragraphs[i]
	}
	allRecords := []database.ReviewParagraphRecord{}

	if record != nil && record.LlmProgressParagraph > 0 && record.LlmProgressParagraph < totalParagraphs {
		startIndex = record.LlmProgressParagraph
		if record.LlmProgressCheckpoint != "" {
			var checkpoint LlmCheckpoint
			if err := json.Unmarshal([]byte(record.LlmProgressCheckpoint), &checkpoint); err == nil {
				for i := 0; i < startIndex && i < len(checkpoint.RepairedParagraphs); i++ {
					repairedParagraphs[i] = checkpoint.RepairedParagraphs[i]
				}
				allRecords = checkpoint.Records
			} else {
				log.Printf("[LLM修复] 断点数据解析失败，仅恢复段落进度: %v", err)
			}
		}
		log.Printf("[LLM修复] 断点恢复: 从段落 %d/%d 继续", startIndex+1, totalParagraphs)
	}

	const checkpointInterval = 50
	concurrency := config.AppConfigInstance.LLMConcurrency
	if concurrency < 1 {
		concurrency = 1
	}
	// 本地 Ollama 启用时强制串行，避免单实例并发推理触发 OOM
	if config.AppConfigInstance.EnableLocalModel && GetHealthManager().ShouldUseLocalModel() && concurrency > 1 {
		log.Printf("[LLM修复] 本地模型启用且健康，并发数从 %d 降为 1 以避免 OOM", concurrency)
		concurrency = 1
	}
	remaining := totalParagraphs - startIndex
	if concurrency > remaining {
		concurrency = remaining
	}
	progressTracker := GlobalProgressTracker.StartTracking(fileMd5, totalParagraphs)
	if startIndex > 0 {
		for i := 0; i < startIndex; i++ {
			progressTracker.RecordChunkComplete(true, 0, "checkpoint")
		}
	}
	defer GlobalProgressTracker.FinishTracking(fileMd5)

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepLlmFix,
		Action:  "progress",
		Details: fmt.Sprintf("LLM修复开始，共 %d 个段落，从第 %d 段开始，并发数 %d", totalParagraphs, startIndex+1, concurrency),
		Status:  "running",
	})

	jobs := make(chan int)
	results := make(chan llmParagraphResult)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for workerID := 0; workerID < concurrency; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				repaired := repairer.RepairParagraph(paragraphs[index])
				select {
				case results <- llmParagraphResult{
					Index:    index,
					Repaired: repaired,
					Err:      nil,
				}:
				case <-ctx.Done():
					return
				}
				source := "remote"
				if repairer.RepairModelType != "api" {
					source = "local"
				}
				progressTracker.RecordChunkComplete(true, time.Since(start).Milliseconds(), source)
			}
		}()
	}
	go func() {
		for i := startIndex; i < totalParagraphs; i++ {
			select {
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				close(results)
				return
			case jobs <- i:
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	nextToCommit := startIndex
	pendingResults := make(map[int]llmParagraphResult)
	completed := startIndex
	cancelRequested := false
	for result := range results {
		pendingResults[result.Index] = result
		completed++

		stepProgress := completed * 100 / totalParagraphs
		database.UpdateFileStatus(fileMd5, "processing", StepLlmFix,
			CalculateProgress(StepLlmFix, stepProgress),
			fmt.Sprintf("LLM修复: 已完成 %d/%d 段落", completed, totalParagraphs))

		for {
			committed, ok := pendingResults[nextToCommit]
			if !ok {
				break
			}
			repairedParagraphs[nextToCommit] = committed.Repaired
			if committed.Repaired != paragraphs[nextToCommit] {
				allRecords = append(allRecords, database.ReviewParagraphRecord{
					FileMd5:          fileMd5,
					ParagraphIndex:   nextToCommit,
					OriginalText:     paragraphs[nextToCommit],
					SuggestedText:    committed.Repaired,
					ModificationType: "llm_repair",
					Status:           "pending",
				})
			}
			delete(pendingResults, nextToCommit)
			nextToCommit++

			if nextToCommit%checkpointInterval == 0 || nextToCommit == totalParagraphs {
				if err := saveLlmCheckpoint(fileMd5, repairedParagraphs[:nextToCommit], allRecords, nextToCommit, totalParagraphs); err != nil {
					log.Printf("[LLM修复] 断点保存失败: %v", err)
				}
			}
		}

		// 更频繁的取消检查：每 10 个段落检查一次（原 50），确保小文件也能响应取消
		if completed%10 == 0 {
			cancelled, _ := database.IsFileCancelled(fileMd5)
			if cancelled {
				cancelRequested = true
				cancel()
				database.SetCancelFlag(fileMd5, 0)
				log.Printf("[LLM修复] 文件 %s 已被取消，已完成 %d/%d 段落，已提交 %d/%d 段落",
					fileMd5, completed, totalParagraphs, nextToCommit, totalParagraphs)
			}
		}
	}
	if cancelRequested {
		// 同步将 status 标记为 cancelled，外层 RunAllSteps 可据此中止后续步骤，
		// 避免取消后仍被推进到 review/finalizing
		progress := CalculateProgress(StepLlmFix, completed*100/totalParagraphs)
		database.UpdateFileStatus(fileMd5, "cancelled", StepLlmFix, progress,
			fmt.Sprintf("用户取消（已完成 %d/%d 段落）", completed, totalParagraphs))
		return &PipelineResult{
			CurrentStep: StepLlmFix,
			NextStep:    "",
			Progress:    progress,
			Message:     fmt.Sprintf("LLM修复已取消 (%d/%d)", completed, totalParagraphs),
		}, nil
	}
	// 审核基线为原段拼接（LLM 修复前），与 review_paragraphs.paragraph_index 一一对应
	baselineContent := strings.Join(paragraphs, "\n")
	if err := saveIntermediateFile(fileMd5, StepLlmFix, baselineContent); err != nil {
		return nil, fmt.Errorf("保存中间文件失败: %w", err)
	}

	// 批量写入段级审核记录
	if len(allRecords) > 0 {
		if err := database.CreateReviewParagraphs(allRecords); err != nil {
			return nil, fmt.Errorf("写入段级审核记录失败: %w", err)
		}
	}

	// 清除断点记录
	database.UpdateLlmProgress(fileMd5, 0, "")

	nextStep := GetNextStep(StepLlmFix)
	progress := CalculateProgress(nextStep, 0)
	database.UpdateFileStatus(fileMd5, "processing", nextStep, progress, "")

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepLlmFix,
		Action:  "complete",
		Details: fmt.Sprintf("修复段落: %d", len(allRecords)),
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepLlmFix,
		NextStep:    nextStep,
		Progress:    progress,
		Message:     fmt.Sprintf("LLM修复完成，修复段落: %d", len(allRecords)),
	}, nil
}

func saveLlmCheckpoint(fileMd5 string, repairedParagraphs []string, records []database.ReviewParagraphRecord, committedCount, totalParagraphs int) error {
	checkpointContent := strings.Join(repairedParagraphs, "\n")
	checkpointJSON, err := json.Marshal(LlmCheckpoint{
		ParagraphIndex:     committedCount,
		RepairedParagraphs: repairedParagraphs,
		Records:            records,
	})
	if err != nil {
		return err
	}
	if err := database.UpdateLlmProgress(fileMd5, committedCount, string(checkpointJSON)); err != nil {
		return err
	}
	if err := saveIntermediateFile(fileMd5, StepLlmFix, checkpointContent); err != nil {
		return err
	}

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepLlmFix,
		Action:  "checkpoint",
		Details: fmt.Sprintf("断点保存: 段落 %d/%d (累计修改 %d 段)", committedCount, totalParagraphs, len(records)),
		Status:  "running",
	})
	log.Printf("[LLM修复] 断点保存: %d/%d 段落完成", committedCount, totalParagraphs)
	return nil
}

// processReviewStep 审核步骤（进入审核等待状态）
func processReviewStep(fileMd5, content string, _ *database.FileRecord) (*PipelineResult, error) {
	total, resolved, err := database.GetReviewParagraphProgress(fileMd5)
	if err != nil {
		return nil, fmt.Errorf("查询审核进度失败: %w", err)
	}

	if total == 0 {
		nextStep := GetNextStep(StepReview)
		database.UpdateFileStatus(fileMd5, "processing", nextStep, CalculateProgress(nextStep, 0), "")
		return &PipelineResult{
			CurrentStep: StepReview,
			NextStep:    nextStep,
			Progress:    CalculateProgress(nextStep, 0),
			Message:     "无需审核，直接进入下一步",
		}, nil
	}

	// 保存审核基线：固定 LLM 修复后的文本，确保审核期间不会被覆盖
	baselinePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileMd5+"_review_baseline.txt")
	if err := os.WriteFile(baselinePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("保存审核基线文件失败: %w", err)
	}
	if err := database.SetReviewBaselinePath(fileMd5, baselinePath); err != nil {
		return nil, fmt.Errorf("记录审核基线路径失败: %w", err)
	}

	stepProgress := 0
	if total > 0 {
		stepProgress = resolved * 100 / total
	}

	progress := CalculateProgress(StepReview, stepProgress)
	database.UpdateFileStatus(fileMd5, "reviewing", StepReview, progress, "")

	return &PipelineResult{
		CurrentStep: StepReview,
		NextStep:    StepReview,
		Progress:    progress,
		Message:     fmt.Sprintf("等待审核 (%d/%d)", resolved, total),
	}, nil
}

// processFinalizingStep 生成最终文件步骤
func processFinalizingStep(fileMd5, content string, record *database.FileRecord) (*PipelineResult, error) {
	records, err := database.GetReviewParagraphsByFileMd5(fileMd5, "")
	if err != nil {
		return nil, fmt.Errorf("查询段级审核记录失败: %w", err)
	}

	// 按段索引重建最终文本：approved 用 suggested，edited 用 editedText，
	// rejected/pending 保留原段；duplicate_paragraph 在 approved 时整段删除
	byIndex := make(map[int]*database.ReviewParagraphRecord, len(records))
	for i := range records {
		byIndex[records[i].ParagraphIndex] = &records[i]
	}
	baselineParagraphs := strings.Split(content, "\n")
	finalParagraphs := make([]string, 0, len(baselineParagraphs))
	for i, original := range baselineParagraphs {
		r, hit := byIndex[i]
		if !hit {
			finalParagraphs = append(finalParagraphs, original)
			continue
		}
		switch r.Status {
		case "approved":
			if r.ModificationType == "duplicate_paragraph" {
				continue
			}
			finalParagraphs = append(finalParagraphs, r.SuggestedText)
		case "edited":
			finalParagraphs = append(finalParagraphs, r.EditedText)
		default:
			finalParagraphs = append(finalParagraphs, original)
		}
	}
	finalContent := strings.Join(finalParagraphs, "\n")

	author := record.Author
	title := record.Title
	namePrefix := fmt.Sprintf("%d", record.ID)
	if author != "" && title != "" {
		namePrefix = fmt.Sprintf("%s_%s", author, title)
	} else if title != "" {
		namePrefix = title
	}
	finalFileName := fmt.Sprintf("%s_cleaned_%d.txt", namePrefix, time.Now().Unix())

	finalPath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileMd5+"_final_"+finalFileName)
	if err := os.WriteFile(finalPath, []byte(finalContent), 0644); err != nil {
		return nil, fmt.Errorf("保存最终文件失败: %w", err)
	}

	finalMd5 := file.ComputeContentMd5(finalContent)
	database.CreateVersion(&database.VersionRecord{
		OriginalMd5: fileMd5,
		VersionMd5:  finalMd5,
		ParentMd5:   fileMd5,
		VersionType: "final",
		FilePath:    finalPath,
		Step:        StepFinalizing,
	})

	database.UpdateFileStatus(fileMd5, "completed", StepFinalizing, 100, "")

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepFinalizing,
		Action:  "complete",
		Details: fmt.Sprintf("最终文件: %s", finalFileName),
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepFinalizing,
		NextStep:    "",
		Progress:    100,
		Message:     fmt.Sprintf("处理完成，最终文件: %s", finalFileName),
	}, nil
}

// CheckReviewComplete 检查审核是否全部完成
func CheckReviewComplete(fileMd5 string) (bool, error) {
	total, resolved, err := database.GetReviewParagraphProgress(fileMd5)
	if err != nil {
		return false, err
	}
	return total > 0 && total == resolved, nil
}

// AdvanceFromReview 审核完成后推进到下一步
func AdvanceFromReview(fileMd5 string) (*PipelineResult, error) {
	record, err := database.GetFileByMd5(fileMd5)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("文件不存在")
	}

	nextStep := GetNextStep(StepReview)
	return ProcessStep(fileMd5, nextStep)
}

// saveIntermediateFile 保存中间文件并记录版本（不更新 file_path，避免审核基线被覆盖）
func saveIntermediateFile(fileMd5, step, content string) error {
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileMd5+"_"+step+".txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return err
	}

	contentMd5 := file.ComputeContentMd5(content)

	parentMd5 := fileMd5
	latestVersion, _ := database.GetLatestVersion(fileMd5)
	if latestVersion != nil {
		parentMd5 = latestVersion.VersionMd5
	}

	database.CreateVersion(&database.VersionRecord{
		OriginalMd5: fileMd5,
		VersionMd5:  contentMd5,
		ParentMd5:   parentMd5,
		VersionType: "intermediate",
		FilePath:    filePath,
		Step:        step,
	})

	// 仅更新 file_path（非审核步骤时），审核基线由 processReviewStep 独立维护
	record, _ := database.GetFileByMd5(fileMd5)
	if record != nil {
		// 审核步骤不更新 file_path，因为过程审核已使用独立基线
		if step != StepReview && step != StepFinalizing {
			record.FilePath = filePath
			db := database.GetDB()
			if _, err := db.Exec(`UPDATE files SET file_path = ? WHERE md5 = ?`, filePath, fileMd5); err != nil {
				log.Printf("[中间文件] 更新 file_path 失败: %v", err)
			}
		}
	}

	return nil
}

// removeAdContent 根据正则模式移除广告内容
func removeAdContent(content, pattern string) string {
	if pattern == "" {
		return content
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("[广告过滤] 正则表达式编译失败 (pattern=%s): %v", pattern, err)
		return content
	}
	return re.ReplaceAllString(content, "")
}
