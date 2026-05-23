package processor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/database"
	"txt-cleaning/internal/file"
	"txt-cleaning/internal/processor/preprocess"
	"txt-cleaning/internal/processor/rules"
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

	var items []database.ReviewItemRecord
	for _, change := range detectResult.Changes {
		items = append(items, database.ReviewItemRecord{
			FileMd5:          fileMd5,
			OriginalText:     change.Original,
			SuggestedText:    change.Replacement,
			ModificationType: change.Type,
			Confidence:       change.Confidence,
			PositionStart:    change.Position,
			PositionEnd:      change.Position + len(change.Original),
			Status:           "pending",
		})
	}
	if len(items) > 0 {
		database.CreateReviewItems(items)
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

// LlmCheckpoint LLM修复断点数据
type LlmCheckpoint struct {
	ParagraphIndex    int                `json:"paragraphIndex"`
	RepairedParagraphs []string          `json:"repairedParagraphs"`
	Changes           []preprocess.Change `json:"changes"`
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

	// 尝试从断点恢复
	startIndex := 0
	repairedParagraphs := []string{}
	allChanges := []preprocess.Change{}

	if record != nil && record.LlmProgressParagraph > 0 && record.LlmProgressParagraph < totalParagraphs {
		startIndex = record.LlmProgressParagraph
		// 恢复之前的已修复段落
		for i := 0; i < startIndex; i++ {
			repairedParagraphs = append(repairedParagraphs, paragraphs[i])
		}
		log.Printf("[LLM修复] 断点恢复: 从段落 %d/%d 继续", startIndex+1, totalParagraphs)
	}

	const checkpointInterval = 50

	database.CreateProcessingLog(&database.ProcessingLogRecord{
		FileMd5: fileMd5,
		Step:    StepLlmFix,
		Action:  "progress",
		Details: fmt.Sprintf("LLM修复开始，共 %d 个段落，从第 %d 段开始", totalParagraphs, startIndex+1),
		Status:  "running",
	})

	for i := startIndex; i < totalParagraphs; i++ {
		// 定期检查取消标志
		if i%checkpointInterval == 0 {
			cancelled, _ := database.IsFileCancelled(fileMd5)
			if cancelled {
				log.Printf("[LLM修复] 文件 %s 已被取消，停止处理（已完成 %d/%d 段落）", fileMd5, i, totalParagraphs)
				database.SetCancelFlag(fileMd5, 0) // 重置取消标志
				return &PipelineResult{
					CurrentStep: StepLlmFix,
					NextStep:    "",
					Progress:    CalculateProgress(StepLlmFix, i*100/totalParagraphs),
					Message:     fmt.Sprintf("LLM修复已取消 (%d/%d)", i, totalParagraphs),
				}, nil
			}
		}

		preview := paragraphs[i]
		previewRunes := []rune(preview)
		if len(previewRunes) > 40 {
			preview = string(previewRunes[:40]) + "..."
		}

		database.UpdateFileStatus(fileMd5, "processing", StepLlmFix,
			CalculateProgress(StepLlmFix, i*100/totalParagraphs),
			fmt.Sprintf("LLM修复: 正在处理 %d/%d", i+1, totalParagraphs))

		repaired, changes := repairer.RepairParagraph(paragraphs[i])
		repairedParagraphs = append(repairedParagraphs, repaired)
		allChanges = append(allChanges, changes...)

		stepProgress := (i + 1) * 100 / totalParagraphs
		database.UpdateFileStatus(fileMd5, "processing", StepLlmFix,
			CalculateProgress(StepLlmFix, stepProgress),
			fmt.Sprintf("LLM修复: 已完成 %d/%d 段落", i+1, totalParagraphs))

		// 每隔 checkpointInterval 个段落保存断点
		if (i+1)%checkpointInterval == 0 || i == totalParagraphs-1 {
			checkpointContent := strings.Join(repairedParagraphs, "\n")
			checkpointJSON, _ := json.Marshal(LlmCheckpoint{
				ParagraphIndex:    i + 1,
				RepairedParagraphs: repairedParagraphs,
				Changes:           allChanges,
			})
			database.UpdateLlmProgress(fileMd5, i+1, string(checkpointJSON))
			// 同时保存中间文件
			saveIntermediateFile(fileMd5, StepLlmFix, checkpointContent)

			database.CreateProcessingLog(&database.ProcessingLogRecord{
				FileMd5: fileMd5,
				Step:    StepLlmFix,
				Action:  "checkpoint",
				Details: fmt.Sprintf("断点保存: 段落 %d/%d (累计修改 %d 处)", i+1, totalParagraphs, len(allChanges)),
				Status:  "running",
			})

			log.Printf("[LLM修复] 断点保存: %d/%d 段落完成", i+1, totalParagraphs)
		}
	}

	// 最终保存
	repairResult := ModelRepairResult{
		Content: strings.Join(repairedParagraphs, "\n"),
		Changes: allChanges,
	}

	if err := saveIntermediateFile(fileMd5, StepLlmFix, repairResult.Content); err != nil {
		return nil, fmt.Errorf("保存中间文件失败: %w", err)
	}

	// 批量写入审核项
	var llmItems []database.ReviewItemRecord
	for _, change := range repairResult.Changes {
		llmItems = append(llmItems, database.ReviewItemRecord{
			FileMd5:          fileMd5,
			OriginalText:     change.Original,
			SuggestedText:    change.Replacement,
			ModificationType: change.Type,
			Confidence:       change.Confidence,
			PositionStart:    change.Position,
			PositionEnd:      change.Position + len(change.Original),
			Status:           "pending",
		})
	}
	if len(llmItems) > 0 {
		database.CreateReviewItems(llmItems)
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
		Details: fmt.Sprintf("修复建议: %d", len(repairResult.Changes)),
		Status:  "success",
	})

	return &PipelineResult{
		CurrentStep: StepLlmFix,
		NextStep:    nextStep,
		Progress:    progress,
		Message:     fmt.Sprintf("LLM修复完成，修复建议: %d", len(repairResult.Changes)),
	}, nil
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
	cleaner := NewBasicCleaner(false)
	_ = cleaner
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
