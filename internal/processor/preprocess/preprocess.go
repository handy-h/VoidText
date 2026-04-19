package preprocess

import (
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

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
	Type        string  `json:"type"`
	Original    string  `json:"original"`
	Replacement string  `json:"replacement"`
	Position    int     `json:"position"`
	Confidence  float64 `json:"confidence"`
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
	// 检查是否已经是有效的UTF-8
	if utf8.ValidString(result.Content) {
		return result
	}

	// 处理混合编码：逐行检测并转换
	result.Content = fixMixedEncoding(result.Content)

	return result
}

// gbkToUtf8 将GBK编码的字符串转换为UTF-8
func gbkToUtf8(gbkStr string) (string, error) {
	reader := transform.NewReader(strings.NewReader(gbkStr), simplifiedchinese.GBK.NewDecoder())
	utf8Bytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(utf8Bytes), nil
}

// fixMixedEncoding 修复混合编码问题
func fixMixedEncoding(content string) string {
	// 按行分割，逐行检测和转换
	lines := strings.Split(content, "\n")
	fixedLines := make([]string, len(lines))

	for i, line := range lines {
		if utf8.ValidString(line) {
			// 已经是有效的UTF-8
			fixedLines[i] = line
		} else {
			// 尝试GBK转UTF-8
			converted, err := gbkToUtf8(line)
			if err == nil && utf8.ValidString(converted) {
				fixedLines[i] = converted
			} else {
				// 如果转换失败，替换为替换字符
				fixedLines[i] = strings.ToValidUTF8(line, "")
			}
		}
	}

	return strings.Join(fixedLines, "\n")
}

// isLikelyGBK 判断字节序列是否可能是GBK编码
func isLikelyGBK(data []byte) bool {
	// GBK编码的中文通常包含0x80-0xFF范围的字节
	// 而UTF-8的多字节序列有特定的格式
	gbkScore := 0
	utf8Score := 0

	i := 0
	for i < len(data) {
		b := data[i]

		if b < 0x80 {
			// ASCII字符，两种编码都支持
			utf8Score++
			gbkScore++
			i++
		} else if b >= 0x80 && b <= 0xBF {
			// 可能是UTF-8的 continuation byte，也可能是GBK的第二字节
			if i > 0 && data[i-1] >= 0x81 && data[i-1] <= 0xFE {
				// 前一个字节是GBK的首字节范围
				gbkScore += 2
				i++
			} else {
				// 检查是否是UTF-8多字节序列
				utf8Len := utf8CharLen(b)
				if utf8Len > 0 && i+utf8Len <= len(data) {
					valid := true
					for j := 1; j < utf8Len; j++ {
						if data[i+j]&0xC0 != 0x80 {
							valid = false
							break
						}
					}
					if valid {
						utf8Score += utf8Len
						i += utf8Len
					} else {
						gbkScore += 2
						i += 2
					}
				} else {
					gbkScore += 2
					i += 2
				}
			}
		} else if b >= 0xC0 && b <= 0xFD {
			// 可能是UTF-8的首字节
			utf8Len := utf8CharLen(b)
			if utf8Len > 0 && i+utf8Len <= len(data) {
				valid := true
				for j := 1; j < utf8Len; j++ {
					if data[i+j]&0xC0 != 0x80 {
						valid = false
						break
					}
				}
				if valid {
					utf8Score += utf8Len
					i += utf8Len
					continue
				}
			}
			// 可能是GBK
			if i+1 < len(data) {
				gbkScore += 2
				i += 2
			} else {
				i++
			}
		} else {
			i++
		}
	}

	return gbkScore > utf8Score
}

// utf8CharLen 返回UTF-8字符的字节长度
func utf8CharLen(firstByte byte) int {
	if firstByte&0x80 == 0 {
		return 1
	}
	if firstByte&0xE0 == 0xC0 {
		return 2
	}
	if firstByte&0xF0 == 0xE0 {
		return 3
	}
	if firstByte&0xF8 == 0xF0 {
		return 4
	}
	return 0
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
