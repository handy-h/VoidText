package preprocess

import (
	"fmt"
	"io"
	"log"
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
// 支持纯UTF-8、纯GBK、纯GB18030以及混合编码文件
// 对于混合编码文件，采用逐行检测策略，每行独立判断编码
func detectAndConvertToUTF8(data []byte) (string, error) {
	// 快速路径：整个文件已经是有效的UTF-8
	if utf8.Valid(data) {
		return string(data), nil
	}

	// 非有效UTF-8 → 直接走逐行检测（支持混合编码）
	// 不再尝试整文件GBK/GB18030解码，因为混合编码场景下会损坏UTF-8部分
	str := string(data)
	return fixMixedEncoding(str), nil
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

// gb18030ToUtf8 将GB18030编码的字符串转换为UTF-8
func gb18030ToUtf8(gb18030Str string) (string, error) {
	reader := transform.NewReader(strings.NewReader(gb18030Str), simplifiedchinese.GB18030.NewDecoder())
	utf8Bytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(utf8Bytes), nil
}

// fixMixedEncoding 修复混合编码问题
// 逐行检测编码，每行独立尝试 UTF-8 → GBK → GB18030 → 降级清理
// 支持同一文件中不同行使用不同编码的场景
func fixMixedEncoding(content string) string {
	lines := strings.Split(content, "\n")
	fixedLines := make([]string, len(lines))
	convertedCount := 0

	for i, line := range lines {
		if utf8.ValidString(line) {
			// 已经是有效的UTF-8，保留原样
			fixedLines[i] = line
			continue
		}

		// 尝试GBK转UTF-8
		if converted, err := gbkToUtf8(line); err == nil && utf8.ValidString(converted) {
			fixedLines[i] = converted
			convertedCount++
			continue
		}

		// 尝试GB18030转UTF-8
		if converted, err := gb18030ToUtf8(line); err == nil && utf8.ValidString(converted) {
			fixedLines[i] = converted
			convertedCount++
			continue
		}

		// 所有编码都失败，移除无效字节
		fixedLines[i] = strings.ToValidUTF8(line, "")
		convertedCount++
	}

	if convertedCount > 0 {
		log.Printf("[编码修复] 逐行检测完成: 总行数=%d, 修复行数=%d", len(lines), convertedCount)
	}

	return strings.Join(fixedLines, "\n")
}


// removeAdvertisements 移除广告内容
func removeAdvertisements(result PreprocessResult) PreprocessResult {
	// 常见广告模式
	adPatterns := []string{
		`本文由.*提供`,
		`更多精彩内容请访问.*`,
		`下载APP.*`,
		`www\.[a-zA-Z0-9]+\.[a-zA-Z]{2,}`,
		`http[s]?://\S+`,
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

