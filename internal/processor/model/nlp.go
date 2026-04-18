package model

import (
	"regexp"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/jdkato/prose/v2"

	"txt-cleaning/internal/external"
	"txt-cleaning/internal/processor/preprocess"
)

// NLPResult NLP处理结果
type NLPResult struct {
	Content     string                  `json:"content"`
	Original    string                  `json:"original"`
	Suggestions []preprocess.Change     `json:"suggestions"`
	Stats       map[string]int          `json:"stats"`
}

// 全局外部API客户端
var externalAPI = external.NewAPI()

// Process 处理文本
func Process(content string) NLPResult {
	result := NLPResult{
		Content:     content,
		Original:    content,
		Suggestions: []preprocess.Change{},
		Stats:       make(map[string]int),
	}

	// 1. 错别字检测与纠正
	result = detectTypos(result)

	// 2. 乱码识别与修复
	result = detectGarbage(result)

	// 3. 重复内容检测
	result = detectDuplicates(result)

	// 4. 使用外部模型进一步处理
	result = processWithExternalAPI(result)

	return result
}

// processWithExternalAPI 使用外部API处理文本
func processWithExternalAPI(result NLPResult) NLPResult {
	// 尝试使用外部模型纠正文本
	corrected, err := externalAPI.CorrectText(result.Content)
	if err == nil && corrected != result.Content {
		// 添加建议
		result.Suggestions = append(result.Suggestions, preprocess.Change{
			Type:        "external_correction",
			Original:    result.Content,
			Replacement: corrected,
			Position:    0,
		})

		// 更新内容
		result.Content = corrected
		result.Stats["external_corrections"]++
	}

	return result
}

// detectTypos 检测错别字
func detectTypos(result NLPResult) NLPResult {
	// 常见错别字映射
	typoMap := map[string]string{
		"名子": "名字",
		"在次": "再次",
		"的话": "的话", // 示例，实际项目中需要更完整的错别字库
		"好象": "好像",
		"时侯": "时候",
		"知到": "知道",
		"己经": "已经",
		"坐位": "座位",
		"即然": "既然",
		"部份": "部分",
	}

	// 检测错别字
	for typo, correct := range typoMap {
		if strings.Contains(result.Content, typo) {
			// 查找所有出现的位置
			regex := regexp.MustCompile(regexp.QuoteMeta(typo))
			matches := regex.FindAllStringIndex(result.Content, -1)

			for _, match := range matches {
				// 添加建议
				result.Suggestions = append(result.Suggestions, preprocess.Change{
					Type:        "typo",
					Original:    typo,
					Replacement: correct,
					Position:    match[0],
				})

				// 更新统计
				result.Stats["typos"]++
			}

			// 替换错别字
			result.Content = strings.ReplaceAll(result.Content, typo, correct)
		}
	}

	// 使用prose进行更复杂的文本分析
	// 这里简化处理，实际项目中可以使用prose的实体识别和语法分析功能

	return result
}

// detectGarbage 检测乱码
func detectGarbage(result NLPResult) NLPResult {
	// 检测乱码模式
	// 这里使用简单的规则，实际项目中可以使用更复杂的算法
	garbagePatterns := []string{
		`[\x80-\xFF]{3,}`, // 连续的非ASCII字符
		`[a-zA-Z0-9]{10,}`,  // 过长的英文数字串
		`[\p{P}\p{S}]{5,}`, // 连续的标点符号
	}

	for _, pattern := range garbagePatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringIndex(result.Content, -1)

		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			original := result.Content[match[0]:match[1]]
			replacement := ""

			// 添加建议
			result.Suggestions = append(result.Suggestions, preprocess.Change{
				Type:        "garbage",
				Original:    original,
				Replacement: replacement,
				Position:    match[0],
			})

			// 更新统计
			result.Stats["garbage"]++

			// 移除乱码
			result.Content = result.Content[:match[0]] + replacement + result.Content[match[1]:]
		}
	}

	return result
}

// detectDuplicates 检测重复内容
func detectDuplicates(result NLPResult) NLPResult {
	// 分割文本为句子
	sentences := strings.Split(result.Content, "。")
	seen := make(map[string]int)
	duplicates := []int{}

	// 检测重复句子
	for i, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		if pos, exists := seen[sentence]; exists {
			// 找到重复句子
			duplicates = append(duplicates, i)

			// 添加建议
			result.Suggestions = append(result.Suggestions, preprocess.Change{
				Type:        "duplicate",
				Original:    sentence + "。",
				Replacement: "",
				Position:    pos,
			})

			// 更新统计
			result.Stats["duplicates"]++
		} else {
			seen[sentence] = i
		}
	}

	// 移除重复句子
	for i := len(duplicates) - 1; i >= 0; i-- {
		idx := duplicates[i]
		if idx < len(sentences) {
			sentences = append(sentences[:idx], sentences[idx+1:]...)
		}
	}

	// 重新组合文本
	result.Content = strings.Join(sentences, "。")

	return result
}

// CalculateSimilarity 计算文本相似度
func CalculateSimilarity(a, b string) int {
	return levenshtein.Distance(a, b)
}

// AnalyzeText 分析文本
func AnalyzeText(content string) (map[string]interface{}, error) {
	// 使用prose进行文本分析
	doc, err := prose.NewDocument(content)
	if err != nil {
		return nil, err
	}

	// 提取实体
	entities := []map[string]string{}
	for _, ent := range doc.Entities() {
		entities = append(entities, map[string]string{
			"text":  ent.Text,
			"label": ent.Label,
		})
	}

	// 提取句子
	sentences := []string{}
	for _, sent := range doc.Sentences() {
		sentences = append(sentences, sent.Text)
	}

	return map[string]interface{}{
		"entities":   entities,
		"sentences":  sentences,
		"word_count": len(doc.Tokens()),
		"sent_count": len(sentences),
	}, nil
}