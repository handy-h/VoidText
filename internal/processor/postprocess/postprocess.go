package postprocess

import (
	"regexp"
	"strings"
)

// PostprocessResult 后处理结果
type PostprocessResult struct {
	Content  string         `json:"content"`
	Original string         `json:"original"`
	Changes  []Change       `json:"changes"`
	Stats    map[string]int `json:"stats"`
}

// Change 文本变更
type Change struct {
	Type        string `json:"type"`
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
	Position    int    `json:"position"`
}

// Postprocess 后处理文本
func Postprocess(content string) PostprocessResult {
	result := PostprocessResult{
		Content:  content,
		Original: content,
		Changes:  []Change{},
		Stats:    make(map[string]int),
	}

	// 1. 文本格式优化
	result = optimizeFormat(result)

	// 2. 章节结构整理
	result = organizeChapters(result)

	// 3. 标点符号规范化
	result = normalizePunctuation(result)

	return result
}

// optimizeFormat 优化文本格式
func optimizeFormat(result PostprocessResult) PostprocessResult {
	// 确保每个段落以换行开始
	result.Content = regexp.MustCompile(`\n+`).ReplaceAllString(result.Content, "\n")

	// 确保段落之间有适当的空行
	result.Content = regexp.MustCompile(`\n([^\n])`).ReplaceAllString(result.Content, "\n\n$1")

	// 去除首尾空白
	result.Content = strings.TrimSpace(result.Content)

	return result
}

// organizeChapters 整理章节结构
func organizeChapters(result PostprocessResult) PostprocessResult {
	// 章节模式
	chapterPatterns := []string{
		`第[一二三四五六七八九十百千]+章`,
		`第[0-9]+章`,
		`Chapter [0-9]+`,
		`章节 [0-9]+`,
	}

	for _, pattern := range chapterPatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringIndex(result.Content, -1)

		for _, match := range matches {
			result.Stats["chapters"]++
			if match[0] > 0 && result.Content[match[0]-1] != '\n' {
				result.Content = result.Content[:match[0]] + "\n\n" + result.Content[match[0]:]
			}
		}
	}

	return result
}

// normalizePunctuation 规范化标点符号
func normalizePunctuation(result PostprocessResult) PostprocessResult {
	// 中文标点符号规范化
	punctuationMap := map[string]string{
		",":  "，",
		".":  "。",
		":":  "：",
		";":  "；",
		"?":  "？",
		"!":  "！",
		"'":  "'",
		"\"": "\"",
	}

	for en, zh := range punctuationMap {
		result.Content = strings.ReplaceAll(result.Content, en, zh)
	}

	// 移除重复的标点符号
	punctuationMarks := []string{"，", "。", "：", "；", "？", "！"}
	for _, mark := range punctuationMarks {
		pattern := regexp.MustCompile(regexp.QuoteMeta(mark) + "+")
		result.Content = pattern.ReplaceAllString(result.Content, mark)
	}

	return result
}
