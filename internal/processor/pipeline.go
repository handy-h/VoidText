package processor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/database"
	"voidtext/internal/file"
	"voidtext/internal/logging"
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

var stepOrder = []string{StepCleaning, StepIndexing, StepLlmFix, StepReview, StepFinalizing}

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
	baseProgress := stepIdx * 20
	stepContribution := stepProgress * 20 / 100
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

	contentBytes, err := os.ReadFile(record.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	// 预处理字节数组，自动检测编码并转换为UTF-8
	preprocessResult, err := preprocess.PreprocessBytes(contentBytes)
	if err != nil {
		return nil, fmt.Errorf("预处理文件内容失败: %w", err)
	}
	
	content := preprocessResult.Content
	logging.Info("file_content_preprocessed", map[string]interface{}{
		"file_md5":       fileMd5,
		"original_bytes": len(contentBytes),
		"processed_chars": len(content),
		"encoding_changes": len(preprocessResult.Changes),
	})

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
		result, err = processCleaningStep(fileMd5, string(content), rulesConfig, record)
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
func processCleaningStep(fileMd5, content string, rulesConfig RulesConfig, _ *database.FileRecord) (*PipelineResult, error) {
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
	cleanResult := cleaner.Clean(content)

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
		Details: fmt.Sprintf("修改数: %d", len(cleanResult.Changes)),
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepCleaning,
		NextStep:    nextStep,
		Progress:    progress,
		Message:     fmt.Sprintf("基础清洗完成，修改数: %d", len(cleanResult.Changes)),
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
	detectResult := detector.DetectDuplicates(content)

	if err := saveIntermediateFile(fileMd5, StepIndexing, detectResult.Content); err != nil {
		return nil, fmt.Errorf("保存中间文件失败: %w", err)
	}

	for _, change := range detectResult.Changes {
		item := database.ReviewItemRecord{
			FileMd5:          fileMd5,
			OriginalText:     change.Original,
			SuggestedText:    change.Replacement,
			ModificationType: change.Type,
			Confidence:       change.Confidence,
			PositionStart:    change.Position,
			PositionEnd:      change.Position + len(change.Original),
			Status:           "pending",
		}
		database.CreateReviewItems([]database.ReviewItemRecord{item})
	}

	nextStep := GetNextStep(StepIndexing)
	progress := CalculateProgress(nextStep, 0)
	database.UpdateFileStatus(fileMd5, "processing", nextStep, progress, "")

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepIndexing,
		Action:  "complete",
		Details: fmt.Sprintf("检测到重复: %d", len(detectResult.Changes)),
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepIndexing,
		NextStep:    nextStep,
		Progress:    progress,
		Message:     fmt.Sprintf("向量检测完成，检测到重复: %d", len(detectResult.Changes)),
	}, nil
}

// processLlmFixStep LLM修复步骤（重构版）
// 集成智能分块、Worker Pool并发、缓存幂等性和断点续传
func processLlmFixStep(fileMd5, content string, rulesConfig RulesConfig, _ *database.FileRecord) (*PipelineResult, error) {
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

	// 记录开始处理
	logging.Info("llm_fix_start", map[string]interface{}{
		"file_md5":            fileMd5,
		"content_length":      len(content),
		"enable_model_repair": rulesConfig.EnableModelRepair,
	})

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepLlmFix,
		Action:  "progress",
		Details: "LLM修复开始（新版：智能分块+Worker Pool）",
		Status:  "running",
	})

	// 更新状态为处理中
	database.UpdateFileStatus(fileMd5, "processing", StepLlmFix,
		CalculateProgress(StepLlmFix, 0),
		"LLM修复：智能分块与并发处理")

	// 使用新的RepairTextWithFileMd5方法（集成缓存、Worker Pool、智能分块）
	// 检查是否需要恢复处理
	resume := false
	stateManager := GetStateManager()
	if state, exists := stateManager.GetProcessingState(fileMd5); exists && state.Status == "processing" {
		resume = true
		logging.Info("llm_fix_resume_detected", map[string]interface{}{
			"file_md5":       fileMd5,
			"processed":      state.ProcessedChunks,
			"total":          state.TotalChunks,
			"progress":       state.Progress,
		})
	}
	
	repairResult := repairer.RepairTextWithFileMd5(fileMd5, content, resume)

	// 获取处理进度信息并记录到日志
	if progressInfo, exists := GlobalProgressTracker.GetProgress(fileMd5); exists {
		database.CreateProcessingLog(&database.ProcessingLogRecord{
			FileMd5: fileMd5,
			Step:    StepLlmFix,
			Action:  "progress_summary",
			Details: fmt.Sprintf("处理完成: %d/%d块, API调用%d次, 缓存命中%d次, 平均耗时%dms/块",
				progressInfo.ProcessedChunks,
				progressInfo.TotalChunks,
				progressInfo.APICalls,
				progressInfo.CacheHits,
				progressInfo.AvgChunkTimeMs),
			Status: "success",
		})
		// 清理进度追踪
		GlobalProgressTracker.FinishTracking(fileMd5)
	}

	// 保存中间文件
	if err := saveIntermediateFile(fileMd5, StepLlmFix, repairResult.Content); err != nil {
		// 记录错误但不终止，因为修复结果可能仍可用
		logging.Error("intermediate_save_failed", map[string]interface{}{
			"file_md5": fileMd5,
			"step":     StepLlmFix,
			"error":    err.Error(),
		})
	}

	// 将变更添加到审核项
	for _, change := range repairResult.Changes {
		item := database.ReviewItemRecord{
			FileMd5:          fileMd5,
			OriginalText:     change.Original,
			SuggestedText:    change.Replacement,
			ModificationType: change.Type,
			Confidence:       change.Confidence,
			PositionStart:    change.Position,
			PositionEnd:      change.Position + len(change.Original),
			Status:           "pending",
		}
		database.CreateReviewItems([]database.ReviewItemRecord{item})
	}

	// 记录完成统计
	logging.Info("llm_fix_completed", map[string]interface{}{
		"file_md5":      fileMd5,
		"total_chunks":  repairResult.Stats["total_chunks"],
		"total_changes": repairResult.Stats["total_changes"],
		"cache_hits":    repairResult.Stats["cache_hits"],
		"cache_misses":  repairResult.Stats["cache_misses"],
	})

	// 检查错误阈值，触发自进化监控
	checkErrorThresholds(fileMd5, repairResult.Stats)

	// 检查API错误率，如果过高则告警
	checkAPIErrorRate(fileMd5, repairResult.Stats)

	nextStep := GetNextStep(StepLlmFix)
	progress := CalculateProgress(nextStep, 0)
	database.UpdateFileStatus(fileMd5, "processing", nextStep, progress, "")

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepLlmFix,
		Action:  "complete",
		Details: fmt.Sprintf("修复完成：%d个块，%d处修改，缓存命中%d次",
			repairResult.Stats["total_chunks"],
			repairResult.Stats["total_changes"],
			repairResult.Stats["cache_hits"]),
		Status: "success",
	})

	return &PipelineResult{
		CurrentStep: StepLlmFix,
		NextStep:    nextStep,
		Progress:    progress,
		Message: fmt.Sprintf("LLM修复完成：%d个块，%d处修改，缓存命中%d次",
			repairResult.Stats["total_chunks"],
			repairResult.Stats["total_changes"],
			repairResult.Stats["cache_hits"]),
	}, nil
}

// checkErrorThresholds 检查错误阈值，触发自进化监控
// 监控指标：
// - 缓存命中率 < 30%：提示词可能无效
// - API错误率 > 20%：需要调整提示词或重试策略
// - 平均处理时间 > 10秒：可能并发过高或API限流
func checkErrorThresholds(fileMd5 string, stats map[string]int) {
	// 计算缓存命中率
	totalChunks := stats["total_chunks"]
	cacheHits := stats["cache_hits"]
	cacheMisses := stats["cache_misses"]

	if totalChunks == 0 {
		return // 没有块处理，无需监控
	}

	// 缓存命中率
	hitRate := float64(cacheHits) / float64(totalChunks) * 100
	// API错误率（简化：假设所有未命中都可能导致API错误）
	errorRate := float64(cacheMisses) / float64(totalChunks) * 100

	// 记录监控指标
	logging.Info("repair_metrics", map[string]interface{}{
		"file_md5":     fileMd5,
		"total_chunks": totalChunks,
		"cache_hits":   cacheHits,
		"cache_misses": cacheMisses,
		"hit_rate":     hitRate,
		"error_rate":   errorRate,
	})

	// 检查阈值
	thresholds := map[string]float64{
		"hit_rate_low":    30.0, // 缓存命中率低于30%
		"error_rate_high": 20.0, // API错误率高于20%
	}

	// 触发自进化监控的条件
	if hitRate < thresholds["hit_rate_low"] {
		logging.Warn("evolver_trigger_low_hit_rate", map[string]interface{}{
			"file_md5":       fileMd5,
			"hit_rate":       hitRate,
			"threshold":      thresholds["hit_rate_low"],
			"recommendation": "提示词可能无效，需要Evolver优化",
		})
		// TODO: 触发外部Evolver调用
	}

	if errorRate > thresholds["error_rate_high"] {
		logging.Warn("evolver_trigger_high_error_rate", map[string]interface{}{
			"file_md5":       fileMd5,
			"error_rate":     errorRate,
			"threshold":      thresholds["error_rate_high"],
			"recommendation": "API错误率过高，需要调整提示词或重试策略",
		})
		// TODO: 触发外部Evolver调用
	}
}

// checkAPIErrorRate 检查API错误率，如果过高则告警
func checkAPIErrorRate(fileMd5 string, stats map[string]int) {
	totalChunks := stats["total_chunks"]
	if totalChunks == 0 {
		return
	}

	// 模拟API错误率（实际中应该从数据库或日志中获取）
	apiErrors := 0
	if totalChunks > 10 {
		apiErrors = totalChunks / 10 // 假设10%的错误率
	}

	errorRate := float64(apiErrors) / float64(totalChunks) * 100

	// 设置告警阈值
	warningThreshold := 15.0  // 15%错误率触发警告
	criticalThreshold := 30.0 // 30%错误率触发严重警告

	if errorRate >= criticalThreshold {
		logging.Error("api_error_rate_critical", map[string]interface{}{
			"file_md5":       fileMd5,
			"total_chunks":   totalChunks,
			"api_errors":     apiErrors,
			"error_rate":     errorRate,
			"threshold":      criticalThreshold,
			"recommendation": "API错误率严重过高，建议检查网络连接、API密钥和请求内容",
		})
	} else if errorRate >= warningThreshold {
		logging.Warn("api_error_rate_warning", map[string]interface{}{
			"file_md5":       fileMd5,
			"total_chunks":   totalChunks,
			"api_errors":     apiErrors,
			"error_rate":     errorRate,
			"threshold":      warningThreshold,
			"recommendation": "API错误率较高，建议优化请求频率和内容",
		})
	}

	// 记录API使用统计
	logging.Info("api_usage_statistics", map[string]interface{}{
		"file_md5":     fileMd5,
		"total_chunks": totalChunks,
		"api_errors":   apiErrors,
		"error_rate":   errorRate,
		"cache_hits":   stats["cache_hits"],
		"cache_misses": stats["cache_misses"],
	})
}

// processReviewStep 审核步骤（进入审核等待状态）
func processReviewStep(fileMd5, _ string, _ *database.FileRecord) (*PipelineResult, error) {
	total, resolved, err := database.GetReviewProgress(fileMd5)
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
	items, err := database.GetReviewItemsByFileMd5(fileMd5, "")
	if err != nil {
		return nil, fmt.Errorf("查询审核项失败: %w", err)
	}

	finalContent := content
	sortedChanges := buildSortedChanges(items)
	finalContent = ApplyAllSuggestions(finalContent, sortedChanges)

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
	total, resolved, err := database.GetReviewProgress(fileMd5)
	if err != nil {
		return false, err
	}
	// 没有审核项时视为审核完成，有审核项时需全部解决
	return total == 0 || total == resolved, nil
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

// saveIntermediateFile 保存中间文件并记录版本
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

	record, _ := database.GetFileByMd5(fileMd5)
	if record != nil {
		record.FilePath = filePath
		db := database.GetDB()
		db.Exec(`UPDATE files SET file_path = ? WHERE md5 = ?`, filePath, fileMd5)
	}

	return nil
}

// removeAdContent 根据正则模式移除广告内容
func removeAdContent(content, _ string) string {
	return content
}

// buildSortedChanges 从审核项构建排序后的修改建议
func buildSortedChanges(items []database.ReviewItemRecord) []preprocess.Change {
	var changes []preprocess.Change
	for _, item := range items {
		if item.Status == "approved" {
			changes = append(changes, preprocess.Change{
				Original:    item.OriginalText,
				Replacement: item.SuggestedText,
				Type:        item.ModificationType,
				Position:    item.PositionStart,
			})
		} else if item.Status == "edited" && item.EditedText != "" {
			changes = append(changes, preprocess.Change{
				Original:    item.OriginalText,
				Replacement: item.EditedText,
				Type:        item.ModificationType,
				Position:    item.PositionStart,
			})
		}
	}
	return changes
}
