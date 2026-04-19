package processor

import (
	"sort"
	"strings"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/processor/preprocess"
	"txt-cleaning/internal/processor/rules"
	"txt-cleaning/internal/review/manager"
)

// ProcessResult 处理结果
type ProcessResult struct {
	Content          string              `json:"content"`
	Original         string              `json:"original"`
	Suggestions      []preprocess.Change `json:"suggestions"`
	Stats            map[string]int      `json:"stats"`
	ReviewSessionID  string              `json:"reviewSessionId"`
	ProcessingStages []ProcessingStage   `json:"processingStages"`
}

// ProcessingStage 处理阶段信息
type ProcessingStage struct {
	Name    string              `json:"name"`
	Stats   map[string]int      `json:"stats"`
	Changes []preprocess.Change `json:"changes"`
}

// 全局审核管理器
var reviewManager = manager.NewManager()

// 全局规则管理器
var ruleManager = rules.NewRuleManager()

// Process 处理文本（三阶段处理流水线）
func Process(content string) ProcessResult {
	var stages []ProcessingStage
	currentContent := content
	allChanges := []preprocess.Change{}
	allStats := make(map[string]int)

	// 第一阶段：基础文本清洗
	if config.AppConfigInstance.EnableBasicCleaning {
		basicCleaner := NewBasicCleaner(config.AppConfigInstance.TraditionalToSimple)
		basicResult := basicCleaner.Clean(currentContent)

		stages = append(stages, ProcessingStage{
			Name:    "基础文本清洗",
			Stats:   basicResult.Stats,
			Changes: basicResult.Changes,
		})

		currentContent = basicResult.Content
		allChanges = append(allChanges, basicResult.Changes...)
		mergeStats(allStats, basicResult.Stats)
	}

	// 第二阶段：向量检测（重复内容检测）
	if config.AppConfigInstance.EnableVectorDetection {
		vectorDetector := NewVectorDetector(
			config.AppConfigInstance.VectorSimilarityThreshold,
			config.AppConfigInstance.VectorModelType,
			config.AppConfigInstance.VectorModelName,
		)
		vectorResult := vectorDetector.DetectDuplicates(currentContent)

		stages = append(stages, ProcessingStage{
			Name:    "向量检测去重",
			Stats:   vectorResult.Stats,
			Changes: vectorResult.Changes,
		})

		currentContent = vectorResult.Content
		allChanges = append(allChanges, vectorResult.Changes...)
		mergeStats(allStats, vectorResult.Stats)
	}

	// 第三阶段：模型修复（错别字和语法错误纠正）
	if config.AppConfigInstance.EnableModelRepair {
		modelRepairer := NewModelRepairer(
			config.AppConfigInstance.RepairModelType,
			config.AppConfigInstance.RepairModelName,
		)
		repairResult := modelRepairer.RepairText(currentContent)

		stages = append(stages, ProcessingStage{
			Name:    "模型修复",
			Stats:   repairResult.Stats,
			Changes: repairResult.Changes,
		})

		currentContent = repairResult.Content
		allChanges = append(allChanges, repairResult.Changes...)
		mergeStats(allStats, repairResult.Stats)
	}

	// 应用自定义规则（可选）
	if len(currentContent) > 0 {
		contentAfterRules := ruleManager.ApplyRules(currentContent)
		if contentAfterRules != currentContent {
			stages = append(stages, ProcessingStage{
				Name:  "自定义规则应用",
				Stats: map[string]int{"rules_applied": 1},
			})
			currentContent = contentAfterRules
		}
	}

	return ProcessResult{
		Content:          currentContent,
		Original:         content,
		Suggestions:      allChanges,
		Stats:            allStats,
		ProcessingStages: stages,
	}
}

// mergeStats 合并统计信息
func mergeStats(target, source map[string]int) {
	for k, v := range source {
		target[k] += v
	}
}

// GetRuleManager 获取规则管理器
func GetRuleManager() *rules.RuleManager {
	return ruleManager
}

// ProcessWithReview 处理文本并创建审核会话
func ProcessWithReview(content, fileID, processID string) (ProcessResult, error) {
	result := Process(content)

	// 创建审核会话
	sessionID := processID + "_review"
	session, err := reviewManager.CreateSession(sessionID, fileID, processID, result.Suggestions)
	if err != nil {
		return result, err
	}

	result.ReviewSessionID = session.ID
	return result, nil
}

// GetSuggestions 获取修改建议
func GetSuggestions(content string) []preprocess.Change {
	result := Process(content)
	return result.Suggestions
}

// ApplySuggestion 应用单个修改建议
func ApplySuggestion(content string, suggestion preprocess.Change) string {
	if suggestion.Original == "" && suggestion.Replacement == "" {
		return content
	}

	if suggestion.Original == "" {
		return content
	}

	if suggestion.Position >= 0 && suggestion.Position < len(content) {
		expectedEnd := suggestion.Position + len(suggestion.Original)
		if expectedEnd <= len(content) && content[suggestion.Position:expectedEnd] == suggestion.Original {
			return content[:suggestion.Position] + suggestion.Replacement + content[expectedEnd:]
		}
	}

	idx := strings.Index(content, suggestion.Original)
	if idx >= 0 {
		return content[:idx] + suggestion.Replacement + content[idx+len(suggestion.Original):]
	}

	if suggestion.Type == "duplicate_paragraph" || suggestion.Type == "advertisement" {
		origTrimmed := strings.TrimSpace(suggestion.Original)
		origRunes := []rune(origTrimmed)
		origLen := len(origRunes)
		threshold := float64(0.6)
		paragraphs := strings.Split(content, "\n\n")
		filtered := []string{}
		for _, para := range paragraphs {
			paraTrimmed := strings.TrimSpace(para)
			paraRunes := []rune(paraTrimmed)
			if origLen > 0 && len(paraRunes) > 0 {
				minLen := origLen
				if len(paraRunes) < minLen {
					minLen = len(paraRunes)
				}
				matchCount := 0
				minIdx := minLen
				for i := 0; i < minIdx; i++ {
					if origRunes[i] == paraRunes[i] {
						matchCount++
					}
				}
				similarity := float64(matchCount) / float64(origLen)
				if len(paraRunes) > origLen {
					similarity = float64(matchCount) / float64(len(paraRunes))
				}
				if similarity >= threshold {
					continue
				}
			}
			filtered = append(filtered, para)
		}
		return strings.Join(filtered, "\n\n")
	}
	return content
}

// ApplyAllSuggestions 应用所有修改建议
func ApplyAllSuggestions(content string, suggestions []preprocess.Change) string {
	sorted := make([]preprocess.Change, len(suggestions))
	copy(sorted, suggestions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position > sorted[j].Position
	})
	for _, s := range sorted {
		content = ApplySuggestion(content, s)
	}
	return content
}

// GetReviewManager 获取审核管理器
func GetReviewManager() *manager.Manager {
	return reviewManager
}
