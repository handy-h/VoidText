package preprocess

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
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

	// 2. 乱码清理（兜底处理）- 在特殊字符清理之前执行
	result = cleanupGarbledText(result)

	// 3. 特殊字符处理
	result = cleanSpecialCharacters(result)

	// 4. 广告内容识别与移除
	result = removeAdvertisements(result)

	// 5. 空白字符规范化
	result = normalizeWhitespace(result)

	return result
}

// PreprocessBytes 预处理字节数组，自动检测编码
func PreprocessBytes(data []byte) (PreprocessResult, error) {
	// 检测编码并转换为UTF-8
	content, err := detectAndConvertToUTF8(data)
	if err != nil {
		return PreprocessResult{}, err
	}

	// 使用现有的预处理逻辑
	return Preprocess(content), nil
}

// detectAndConvertToUTF8 检测字节数组的编码并转换为UTF-8
func detectAndConvertToUTF8(data []byte) (string, error) {
	// 检查是否包含UTF-8替换字符序列
	containsReplacement := bytes.Contains(data, []byte{0xEF, 0xBF, 0xBD})
	
	// 如果包含替换字符，可能文件原本是其他编码但被错误地保存为UTF-8
	// 我们需要尝试从原始字节中恢复
	if containsReplacement {
		// 尝试常见的编码
		encodings := []struct {
			name string
			enc  encoding.Encoding
		}{
			{"gbk", simplifiedchinese.GBK},
			{"gb18030", simplifiedchinese.GB18030},
		}

		for _, enc := range encodings {
			decoder := enc.enc.NewDecoder()
			reader := transform.NewReader(bytes.NewReader(data), decoder)
			decoded, err := io.ReadAll(reader)
			if err == nil && utf8.Valid(decoded) {
				str := string(decoded)
				// 检查解码后是否还有替换字符
				if !strings.Contains(str, "�") {
					return str, nil
				}
			}
		}
		
		// 如果解码后仍然包含替换字符，尝试修复混合编码
		str := string(data)
		return fixMixedEncoding(str), nil
	}
	
	// 不包含替换字符，检查是否是有效的UTF-8
	if utf8.Valid(data) {
		return string(data), nil
	}
	
	// 不是有效的UTF-8，尝试解码
	encodings := []struct {
		name string
		enc  encoding.Encoding
	}{
		{"gbk", simplifiedchinese.GBK},
		{"gb18030", simplifiedchinese.GB18030},
	}

	for _, enc := range encodings {
		decoder := enc.enc.NewDecoder()
		reader := transform.NewReader(bytes.NewReader(data), decoder)
		decoded, err := io.ReadAll(reader)
		if err == nil && utf8.Valid(decoded) {
			str := string(decoded)
			if !strings.Contains(str, "�") {
				return str, nil
			}
		}
	}

	// 如果所有解码都失败，返回原始字符串
	return string(data), nil
}

// normalizeEncoding 规范化编码
func normalizeEncoding(result PreprocessResult) PreprocessResult {
	// 检查是否包含替换字符
	if strings.Contains(result.Content, "�") {
		// 尝试修复包含替换字符的文本
		fixed := fixMixedEncoding(result.Content)
		if fixed != result.Content {
			result.Changes = append(result.Changes, Change{
				Type:        "encoding_fix",
				Original:    result.Content,
				Replacement: fixed,
				Position:    0,
			})
			result.Content = fixed
		}
		return result
	}

	// 检查是否已经是有效的UTF-8
	if utf8.ValidString(result.Content) {
		return result
	}

	// 处理混合编码：逐行检测并转换
	fixed := fixMixedEncoding(result.Content)
	if fixed != result.Content {
		result.Changes = append(result.Changes, Change{
			Type:        "encoding_fix",
			Original:    result.Content,
			Replacement: fixed,
			Position:    0,
		})
		result.Content = fixed
	}

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

// cleanSpecialCharacters 清理特殊字符（保留 \n, \r, \t 以维护段落结构）
func cleanSpecialCharacters(result PreprocessResult) PreprocessResult {
	// 移除控制字符，但保留 \t(0x09), \n(0x0A), \r(0x0D)
	regex := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	result.Content = regex.ReplaceAllString(result.Content, "")

	return result
}

// normalizeWhitespace 规范化空白字符
func normalizeWhitespace(result PreprocessResult) PreprocessResult {
	// 保留换行符，仅规范化水平空白字符（空格、制表符等）
	// 注意：不替换 \n 和 \r\n，保留原文段落结构
	horizontalWS := regexp.MustCompile(`[ \t\f\r]+`)
	result.Content = horizontalWS.ReplaceAllString(result.Content, " ")

	// 清理行首行尾多余空格（由水平空白规范化产生）
	result.Content = strings.ReplaceAll(result.Content, " \n", "\n")
	result.Content = strings.ReplaceAll(result.Content, "\n ", "\n")

	// 压缩连续空行（3个以上变2个），保留段落分隔
	multiBlank := regexp.MustCompile(`\n{3,}`)
	result.Content = multiBlank.ReplaceAllString(result.Content, "\n\n")

	// 去除首尾空白
	result.Content = strings.TrimSpace(result.Content)

	return result
}

// cleanupGarbledText 清理无法修复的乱码（兜底处理）
func cleanupGarbledText(result PreprocessResult) PreprocessResult {
	// 定义乱码模式
	// 1. 连续的替换字符（U+FFFD） - 包括单个和多个
	replacementPattern := regexp.MustCompile(`�+`)
	
	// 2. 常见的乱码模式（如"锟斤拷"等）
	commonGarbledPatterns := []string{
		`(锟斤拷)+`,
		`(烫烫烫)+`,
		`(屯屯屯)+`,
		`(����������������)+`,
	}
	
	originalContent := result.Content
	changes := result.Changes
	
	// 处理连续的替换字符（至少2个连续的替换字符才认为是乱码）
	matches := replacementPattern.FindAllStringIndex(originalContent, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		start, end := match[0], match[1]
		garbledText := originalContent[start:end]
		
		// 只处理至少2个连续的替换字符
		if utf8.RuneCountInString(garbledText) >= 2 {
			// 计算乱码字符数
			charCount := utf8.RuneCountInString(garbledText)
			
			// 创建替换文本
			replacement := fmt.Sprintf("[因无法修复的乱码删除了%d个字符]", charCount)
			
			// 添加变更记录
			changes = append(changes, Change{
				Type:        "garbled_text_removal",
				Original:    garbledText,
				Replacement: replacement,
				Position:    start,
				Confidence:  1.0,
			})
			
			// 替换乱码
			originalContent = originalContent[:start] + replacement + originalContent[end:]
		}
	}
	

	
	// 处理常见的乱码模式
	for _, pattern := range commonGarbledPatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringIndex(originalContent, -1)
		
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			start, end := match[0], match[1]
			garbledText := originalContent[start:end]
			
			charCount := utf8.RuneCountInString(garbledText)
			replacement := fmt.Sprintf("[因无法修复的乱码删除了%d个字符]", charCount)
			
			changes = append(changes, Change{
				Type:        "garbled_text_removal",
				Original:    garbledText,
				Replacement: replacement,
				Position:    start,
				Confidence:  1.0,
			})
			
			originalContent = originalContent[:start] + replacement + originalContent[end:]
		}
	}
	
	result.Content = originalContent
	result.Changes = changes
	return result
}

// isLikelyGarbled 判断文本是否可能是乱码
func isLikelyGarbled(text string) bool {
	// 如果文本很短，可能不是乱码
	if utf8.RuneCountInString(text) < 3 {
		return false
	}
	
	// 检查是否包含常见的乱码模式
	commonGarbledPatterns := []string{
		"锟斤拷", "烫烫烫", "屯屯屯", "����������������",
	}
	
	for _, pattern := range commonGarbledPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	
	// 检查是否包含大量替换字符
	replacementCount := strings.Count(text, "�")
	if replacementCount >= utf8.RuneCountInString(text)/2 {
		// 如果超过一半的字符是替换字符，认为是乱码
		return true
	}
	
	// 检查是否包含大量不可打印字符
	unprintableCount := 0
	totalRunes := 0
	for _, r := range text {
		totalRunes++
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			unprintableCount++
		}
	}
	
	if totalRunes == 0 {
		return false
	}
	
	unprintableRatio := float64(unprintableCount) / float64(totalRunes)
	
	// 如果不可打印字符比例高于50%，认为是乱码
	return unprintableRatio > 0.5
}
