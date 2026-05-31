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
	// 将3个及以上连续换行压缩为2个（归一化空行）
	result.Content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result.Content, "\n\n")

	// 去除首尾空白
	result.Content = strings.TrimSpace(result.Content)

	return result
}

// organizeChapters 整理章节结构
// 修复：使用累积偏移量跟踪操作后字符串长度变化，
// 避免多次插入换行后索引错乱导致内容丢失
// 优化：将所有章节模式合并为一个正则，避免多次扫描文本
func organizeChapters(result PostprocessResult) PostprocessResult {
	// 合并所有章节模式为单一正则（优先级由高到低）
	chapterRegex := regexp.MustCompile(`第[一二三四五六七八九十百千]+章|第[0-9]+章|Chapter [0-9]+|章节 [0-9]+`)
	matches := chapterRegex.FindAllStringIndex(result.Content, -1)

	offset := 0 // 累积偏移量，跟踪前序插入导致的字符串位置变化
	for _, match := range matches {
		result.Stats["chapters"]++
		actualPos := match[0] + offset
		if actualPos > 0 && result.Content[actualPos-1] != '\n' {
			insertion := "\n\n"
			result.Content = result.Content[:actualPos] + insertion + result.Content[actualPos:]
			offset += len(insertion) // 更新偏移量
		}
	}

	return result
}

// normalizePunctuation 规范化标点符号
// 修复：英文句号替换为中文句号时排除数字间的小数点（如 3.14），
// 使用负向前瞻/后顾断言，只替换非数字之间的句号
func normalizePunctuation(result PostprocessResult) PostprocessResult {
	// 中文标点符号规范化（英文标点 → 中文标点）
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
		if en == "." {
			// 小数点保护：只替换非数字间的英文句号，避免 3.14 → 3。14
			decimalPattern := regexp.MustCompile(`(?<![0-9])\.(?![0-9])`)
			result.Content = decimalPattern.ReplaceAllString(result.Content, zh)
		} else {
			result.Content = strings.ReplaceAll(result.Content, en, zh)
		}
	}

	// 移除重复的标点符号
	punctuationMarks := []string{"，", "。", "：", "；", "？", "！"}
	for _, mark := range punctuationMarks {
		pattern := regexp.MustCompile(regexp.QuoteMeta(mark) + "+")
		result.Content = pattern.ReplaceAllString(result.Content, mark)
	}

	return result
}
