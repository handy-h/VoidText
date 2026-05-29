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
	if jsonStr == "" {
		return DefaultRulesConfig()
	}
	var cfg RulesConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		log.Printf("[配置] 规则配置 JSON 解析失败，使用默认配置: %v", err)
		return DefaultRulesConfig()
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

// CalculateProgress 计算进度百分比
func CalculateProgress(step string, stepProgress int) int {
	stepIdx := GetStepIndex(step)
	stepsTotal := len(stepOrder)
	stepWeight := 100 / stepsTotal
	baseProgress := stepIdx * stepWeight
	stepContribution := stepProgress * stepWeight / 100
	return baseProgress + stepContribution
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
	paragraphs := strings.Split(content, "\n")
	indexByText := make(map[string][]int, len(paragraphs))
	for i, p := range paragraphs {
		indexByText[p] = append(indexByText[p], i)
	}
	var records []database.ReviewParagraphRecord
	for _, change := range detectResult.Changes {
		if change.Type != "duplicate_paragraph" {
			continue
		}
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

	nextStep := GetNextStep(StepIndexing)
	progress := CalculateProgress(nextStep, 0)
	database.UpdateFileStatus(fileMd5, "processing", nextStep, progress, "")

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepIndexing,
		Action:  "complete",
		Details: fmt.Sprintf("检测到重复段: %d", len(records)),
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepIndexing,
		NextStep:    nextStep,
		Progress:    progress,
		Message:     fmt.Sprintf("向量检测完成，检测到重复段: %d", len(records)),
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

	repairer := NewModelRepairer(
		config.AppConfigInstance.RepairModelType,
		config.AppConfigInstance.RepairModelName,
	)

	// 段落重组：使用LLM智能识别语义段落边界，合并被硬切断的行
	if config.AppConfigInstance.EnableLlmParagraphReconstruct {
		origLen := len([]rune(content))
		reconstructed, err := repairer.ReconstructParagraphsWithCheckpoint(content, fileMd5, func(done, total int) {
			// 每完成一块就更新进度
			log.Printf("[段落重组] 进度: %d/%d 块完成", done, total)
		})
		if err != nil {
			log.Printf("[段落重组] 失败，回退到原始段落结构: %v", err)
		} else {
			content = reconstructed
			log.Printf("[段落重组] 完成: 原始长度=%d字符, 重组后长度=%d字符",
				origLen, len([]rune(reconstructed)))
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

		if completed%checkpointInterval == 0 {
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
		return content
	}
	return re.ReplaceAllString(content, "")
}
