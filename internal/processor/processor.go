package processor

import (
	"txt-cleaning/internal/processor/model"
	"txt-cleaning/internal/processor/postprocess"
	"txt-cleaning/internal/processor/preprocess"
	"txt-cleaning/internal/processor/rules"
	"txt-cleaning/internal/review/manager"
)

// ProcessResult 处理结果
type ProcessResult struct {
	Content     string                  `json:"content"`
	Original    string                  `json:"original"`
	Suggestions []preprocess.Change     `json:"suggestions"`
	Stats       map[string]int          `json:"stats"`
	ReviewSessionID string              `json:"reviewSessionId"`
}

// 全局审核管理器
var reviewManager = manager.NewManager()

// 全局规则管理器
var ruleManager = rules.NewRuleManager()

// Process 处理文本
func Process(content string) ProcessResult {
	// 1. 预处理
	preResult := preprocess.Preprocess(content)

	// 2. 应用自定义规则
	contentAfterRules := ruleManager.ApplyRules(preResult.Content)

	// 3. NLP处理
	nlpResult := model.Process(contentAfterRules)

	// 4. 后处理
	postResult := postprocess.Postprocess(nlpResult.Content)

	// 合并建议
	suggestions := append(preResult.Changes, nlpResult.Suggestions...)

	// 合并统计
	stats := make(map[string]int)
	for k, v := range nlpResult.Stats {
		stats[k] = v
	}
	for k, v := range postResult.Stats {
		stats[k] += v
	}

	return ProcessResult{
		Content:     postResult.Content,
		Original:    content,
		Suggestions: suggestions,
		Stats:       stats,
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

// ApplySuggestion 应用修改建议
func ApplySuggestion(content string, suggestion preprocess.Change) string {
	// 应用单个修改建议
	if suggestion.Position >= 0 && suggestion.Position < len(content) {
		return content[:suggestion.Position] + suggestion.Replacement + content[suggestion.Position+len(suggestion.Original):]
	}
	return content
}

// ApplyAllSuggestions 应用所有修改建议
func ApplyAllSuggestions(content string, suggestions []preprocess.Change) string {
	// 按位置从后往前应用，避免位置偏移
	for i := len(suggestions) - 1; i >= 0; i-- {
		content = ApplySuggestion(content, suggestions[i])
	}
	return content
}

// GetReviewManager 获取审核管理器
func GetReviewManager() *manager.Manager {
	return reviewManager
}