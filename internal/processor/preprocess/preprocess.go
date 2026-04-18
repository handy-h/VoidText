package preprocess

import (
	"regexp"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// PreprocessResult 预处理结果
type PreprocessResult struct {
	Content  string   `json:"content"`
	Original string   `json:"original"`
	Changes  []Change `json:"changes"`
}

// Change 文本变更
type Change struct {
	Type        string `json:"type"`
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
	Position    int    `json:"position"`
}

// Preprocess 预处理文本
func Preprocess(content string) PreprocessResult {
	result := PreprocessResult{
		Content:  content,
		Original: content,
		Changes:  []Change{},
	}

	// 1. 编码规范化
	result = normalizeEncoding(result)

	// 2. 广告内容识别与移除
	result = removeAdvertisements(result)

	// 3. 特殊字符处理
	result = cleanSpecialCharacters(result)

	// 4. 空白字符规范化
	result = normalizeWhitespace(result)

	return result
}

// normalizeEncoding 规范化编码
func normalizeEncoding(result PreprocessResult) PreprocessResult {
	// 尝试从GBK转换为UTF-8
	detector := simplifiedchinese.GBK.NewDecoder()
	_ = transform.NewReader(strings.NewReader(result.Content), detector)

	// 这里简化处理，实际项目中需要更复杂的编码检测和转换
	return result
}

// removeAdvertisements 移除广告内容
func removeAdvertisements(result PreprocessResult) PreprocessResult {
	// 常见广告模式
	adPatterns := []string{
		`本文由.*提供`,
		`更多精彩内容请访问.*`,
		`下载APP.*`,
		`www\.[a-zA-Z0-9]+\.[a-zA-Z]{2,}`,
		`http[s]?://[^\s]+`,
	}

	for _, pattern := range adPatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringIndex(result.Content, -1)

		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			original := result.Content[match[0]:match[1]]
			replacement := ""

			// 添加变更记录
			result.Changes = append(result.Changes, Change{
				Type:        "advertisement",
				Original:    original,
				Replacement: replacement,
				Position:    match[0],
			})

			// 移除广告
			result.Content = result.Content[:match[0]] + replacement + result.Content[match[1]:]
		}
	}

	return result
}

// cleanSpecialCharacters 清理特殊字符
func cleanSpecialCharacters(result PreprocessResult) PreprocessResult {
	// 移除控制字符
	regex := regexp.MustCompile(`[\x00-\x1F\x7F]`)
	result.Content = regex.ReplaceAllString(result.Content, "")

	return result
}

// normalizeWhitespace 规范化空白字符
func normalizeWhitespace(result PreprocessResult) PreprocessResult {
	// 将多个空白字符替换为单个空格
	regex := regexp.MustCompile(`\s+`)
	result.Content = regex.ReplaceAllString(result.Content, " ")

	// 去除首尾空白
	result.Content = strings.TrimSpace(result.Content)

	return result
}