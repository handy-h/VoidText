package processor

import (
	"fmt"
	"strings"
	"txt-cleaning/internal/config"
	"txt-cleaning/internal/external"
	"txt-cleaning/internal/processor/preprocess"
)

// ModelRepairer 模型修复器
type ModelRepairer struct {
	RepairModelType string
	RepairModelName string
}

// ModelRepairResult 模型修复结果
type ModelRepairResult struct {
	Content  string              `json:"content"`
	Original string              `json:"original"`
	Changes  []preprocess.Change `json:"changes"`
	Stats    map[string]int      `json:"stats"`
}

// NewModelRepairer 创建模型修复器
func NewModelRepairer(modelType, modelName string) *ModelRepairer {
	return &ModelRepairer{
		RepairModelType: modelType,
		RepairModelName: modelName,
	}
}

// RepairText 修复文本中的错别字和语法错误
func (mr *ModelRepairer) RepairText(content string) ModelRepairResult {
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

	// 按段落分割文本
	paragraphs := mr.SplitIntoParagraphs(content)

	if len(paragraphs) == 0 {
		return result
	}

	// 修复每个段落
	repairedParagraphs := []string{}

	for _, paragraph := range paragraphs {
		repaired, changes := mr.RepairParagraph(paragraph)
		repairedParagraphs = append(repairedParagraphs, repaired)
		result.Changes = append(result.Changes, changes...)
	}

	// 重新组合文本
	result.Content = strings.Join(repairedParagraphs, "\n")
	result.Stats["paragraphs_repaired"] = len(paragraphs)
	result.Stats["total_changes"] = len(result.Changes)

	return result
}

// SplitIntoParagraphs 将文本分割为段落
func (mr *ModelRepairer) SplitIntoParagraphs(content string) []string {
	// 按换行符分割
	paragraphs := strings.Split(content, "\n")

	// 过滤空段落
	filtered := []string{}
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if len(p) > 0 {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

// RepairParagraph 修复单个段落
func (mr *ModelRepairer) RepairParagraph(paragraph string) (string, []preprocess.Change) {
	// 如果段落太短，直接返回
	if len(paragraph) < 10 {
		return paragraph, []preprocess.Change{}
	}

	// 使用外部API进行修复
	if mr.RepairModelType == "api" {
		return mr.repairWithAPI(paragraph)
	}

	// 本地修复（简化实现）
	return mr.repairLocally(paragraph)
}

// repairWithAPI 使用外部API修复文本
func (mr *ModelRepairer) repairWithAPI(paragraph string) (string, []preprocess.Change) {
	api := external.NewAPI()

	systemPrompt := "你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。"
	userPrompt := "输入：她高兴及了，跑过去抱住他。\n输出：她高兴极了，跑过去抱住他。\n\n当前任务：\n输入：" + paragraph + "\n输出："

	resp, err := api.GenerateChatCompletion(systemPrompt, userPrompt, 0, -1)
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		fmt.Printf("[LLM修复] API调用失败，回退到本地修复: %v\n", err)
		return mr.repairLocally(paragraph)
	}

	repairedText := strings.TrimSpace(resp.Choices[0].Message.Content)

	if repairedText == "" || repairedText == paragraph {
		return paragraph, []preprocess.Change{}
	}

	changes := mr.compareTexts(paragraph, repairedText)

	return repairedText, changes
}

// buildRepairPrompt 构造修复提示词
func (mr *ModelRepairer) buildRepairPrompt(text string) string {
	return `你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。

示例：
输入：她高兴及了，跑过去抱住他。
输出：她高兴极了，跑过去抱住他。

当前任务：
输入：` + text + `
输出：`
}

// repairLocally 本地修复（简化实现）
func (mr *ModelRepairer) repairLocally(paragraph string) (string, []preprocess.Change) {
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
		"的士":  "的士", // 保留
		"他":   "他",  // 保留
		"她":   "她",  // 保留
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

	return repaired, changes
}

// compareTexts 比较两个文本，生成变更记录
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

// detectCommonTypos 检测常见错别字
func (mr *ModelRepairer) detectCommonTypos(text string) []preprocess.Change {
	changes := []preprocess.Change{}

	// 常见错别字模式
	typoPatterns := map[string]string{
		"管":  "馆",  // 图书管 -> 图书馆
		"及了": "极了", // 高兴及了 -> 高兴极了
		"在次": "再次", // 在次见面 -> 再次见面
		"哪么": "那么", // 哪么好 -> 那么好
		"因该": "应该", // 因该去 -> 应该去
		"以经": "已经", // 以经完成 -> 已经完成
		"好象": "好像", // 好象是 -> 好像是
		"做车": "坐车", // 做车去 -> 坐车去
	}

	// 应用字符串模式匹配
	for pattern, replacement := range typoPatterns {
		if strings.Contains(text, pattern) {
			position := strings.Index(text, pattern)
			changes = append(changes, preprocess.Change{
				Type:        "typo_correction",
				Original:    pattern,
				Replacement: replacement,
				Position:    position,
			})
		}
	}

	return changes
}
