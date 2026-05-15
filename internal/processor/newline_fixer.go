package processor

import (
	"strings"
	"unicode"

	"voidtext/internal/processor/preprocess"
)

// NewlineFixResult 换行符修复结果
type NewlineFixResult struct {
	Content  string              `json:"content"`
	Original string              `json:"original"`
	Changes  []preprocess.Change `json:"changes"`
	Stats    map[string]int      `json:"stats"`
}

// NewlineFixer 换行符修复器
type NewlineFixer struct {
	// EnableAutoFix 启用自动修复
	EnableAutoFix bool
	// MinParagraphLength 最小段落长度（字符数）
	MinParagraphLength int
	// MaxParagraphLength 最大段落长度（字符数）
	MaxParagraphLength int
}

// NewNewlineFixer 创建换行符修复器
func NewNewlineFixer() *NewlineFixer {
	return &NewlineFixer{
		EnableAutoFix:      true,
		MinParagraphLength: 20,
		MaxParagraphLength: 500,
	}
}

// Fix 执行换行符修复
func (nf *NewlineFixer) Fix(content string) NewlineFixResult {
	result := NewlineFixResult{
		Content:  content,
		Original: content,
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	if !nf.EnableAutoFix || len(content) == 0 {
		return result
	}

	// 检测是否需要修复换行符
	if !nf.needsNewlineFix(content) {
		result.Stats["skipped"] = 1
		return result
	}

	// 执行换行符修复
	result = nf.fixMissingNewlines(result)

	return result
}

// needsNewlineFix 检测是否需要换行符修复
// 通过分析换行符密度来判断
func (nf *NewlineFixer) needsNewlineFix(content string) bool {
	// 如果内容很短，不需要修复
	if len(content) < 100 {
		return false
	}

	// 计算换行符密度
	newlineCount := strings.Count(content, "\n")
	totalChars := len([]rune(content))

	// 正规排版的小说，大约每100-200个字符就会有一个段落分隔
	// 如果每300个字符才有一个换行符，说明可能缺少段落分隔
	densityThreshold := 300
	charsPerNewline := 0
	if newlineCount > 0 {
		charsPerNewline = totalChars / newlineCount
	}

	if charsPerNewline > densityThreshold || newlineCount == 0 {
		// 进一步检查是否是中文小说（有句号、引号等）
		hasChinesePunctuation := strings.Contains(content, "。") ||
			strings.Contains(content, "」") ||
			strings.Contains(content, "』") ||
			strings.Contains(content, "？") ||
			strings.Contains(content, "！")

		if hasChinesePunctuation {
			return true
		}
	}

	return false
}

// fixMissingNewlines 修复缺失的换行符
func (nf *NewlineFixer) fixMissingNewlines(result NewlineFixResult) NewlineFixResult {
	content := result.Content
	runes := []rune(content)

	// 使用分段策略：基于标点符号和上下文进行分段
	paragraphs := nf.splitIntoParagraphs(string(runes))

	if len(paragraphs) <= 1 {
		// 无法分段，保持原样
		result.Stats["no_paragraphs_found"] = 1
		return result
	}

	// 合并段落
	newContent := strings.Join(paragraphs, "\n\n")

	// 记录变更
	if newContent != content {
		result.Content = newContent
		result.Changes = append(result.Changes, preprocess.Change{
			Type:        "newline_fix",
			Original:    content,
			Replacement: newContent,
			Position:    0,
			Confidence:  0.8,
		})
		result.Stats["paragraphs_added"] = len(paragraphs) - 1
		result.Stats["total_paragraphs"] = len(paragraphs)
	}

	return result
}

// splitIntoParagraphs 将文本分割成段落
func (nf *NewlineFixer) splitIntoParagraphs(content string) []string {
	// 定义段落分割规则
	// 优先级：章节标题 > 对话结束 > 句子结束

	var paragraphs []string
	var currentParagraph strings.Builder

	runes := []rune(content)
	i := 0

	for i < len(runes) {
		char := runes[i]

		// 检查是否是章节标题（如 ◇ 一 ◇）
		if i+2 < len(runes) {
			if nf.isChapterTitle(runes, i) {
				// 保存当前段落
				if currentParagraph.Len() > 0 {
					paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
					currentParagraph.Reset()
				}
				// 提取章节标题
				titleEnd := nf.findChapterTitleEnd(runes, i)
				title := string(runes[i:titleEnd])
				paragraphs = append(paragraphs, strings.TrimSpace(title))
				i = titleEnd
				continue
			}
		}

		// 检查是否是段落分割点
		if nf.isParagraphBreak(runes, i) {
			// 保存当前段落
			if currentParagraph.Len() > 0 {
				paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
				currentParagraph.Reset()
			}
			i++
			continue
		}

		currentParagraph.WriteRune(char)
		i++
	}

	// 保存最后一个段落
	if currentParagraph.Len() > 0 {
		paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
	}

	// 后处理：合并过短的段落
	paragraphs = nf.mergeShortParagraphs(paragraphs)

	return paragraphs
}

// isChapterTitle 检查指定位置是否是章节标题的开始
func (nf *NewlineFixer) isChapterTitle(runes []rune, pos int) bool {
	// 检查常见的章节标题格式
	// 格式1：◇ 一 ◇
	if runes[pos] == '◇' {
		// 向后查找匹配的 ◇
		for j := pos + 1; j < len(runes) && j < pos+20; j++ {
			if runes[j] == '◇' {
				return true
			}
		}
	}

	// 格式2：第X章、第X节等
	if pos+1 < len(runes) && runes[pos] == '第' {
		// 检查后面是否是数字或章节标记
		for j := pos + 1; j < len(runes) && j < pos+10; j++ {
			if unicode.IsDigit(runes[j]) || runes[j] == '章' || runes[j] == '节' || runes[j] == '回' {
				return true
			}
		}
	}

	// 格式3：数字+顿号（如 一、二、）
	if pos+1 < len(runes) && runes[pos] == '一' || runes[pos] == '二' || runes[pos] == '三' ||
		runes[pos] == '四' || runes[pos] == '五' || runes[pos] == '六' ||
		runes[pos] == '七' || runes[pos] == '八' || runes[pos] == '九' || runes[pos] == '十' {
		if pos+1 < len(runes) && runes[pos+1] == '、' {
			return true
		}
	}

	return false
}

// findChapterTitleEnd 查找章节标题的结束位置
func (nf *NewlineFixer) findChapterTitleEnd(runes []rune, start int) int {
	// 查找行尾或特定结束标记
	for i := start; i < len(runes) && i < start+50; i++ {
		// 遇到换行符或另一个章节标记结束
		if runes[i] == '\n' || (i > start && runes[i] == '◇') {
			return i + 1
		}
	}
	// 默认返回起始位置+20或内容结尾
	end := start + 20
	if end > len(runes) {
		end = len(runes)
	}
	return end
}

// isParagraphBreak 检查指定位置是否是段落分割点
func (nf *NewlineFixer) isParagraphBreak(runes []rune, pos int) bool {
	if pos >= len(runes) {
		return false
	}

	char := runes[pos]

	// 规则1：句子结束标点（句号、感叹号、问号）
	if char == '。' || char == '！' || char == '？' {
		// 检查后面是否是引号闭合
		if pos+1 < len(runes) {
			nextChar := runes[pos+1]
			if nextChar == '」' || nextChar == '』' || nextChar == '"' || nextChar == '\'' {
				// 引号闭合后，检查是否是段落结束
				if pos+2 < len(runes) {
					nextNextChar := runes[pos+2]
					// 如果后面是另一个引号开始或章节标记，或者是文本结尾
					if nextNextChar == '「' || nextNextChar == '『' || nextNextChar == '"' || nextNextChar == '\'' ||
						nf.isChapterTitle(runes, pos+2) || pos+2 >= len(runes)-1 {
						return true
					}
				}
			}
		}
	}

	// 规则2：引号闭合 + 句子结束
	if char == '」' || char == '』' || char == '"' || char == '\'' {
		// 检查前面是否是句子结束标点
		if pos > 0 {
			prevChar := runes[pos-1]
			if prevChar == '。' || prevChar == '！' || prevChar == '？' || prevChar == '…' {
				// 检查后面是否是新段落的开始
				if pos+1 < len(runes) {
					nextChar := runes[pos+1]
					// 如果后面是空格、换行、或新句子的开始
					if nextChar == ' ' || nextChar == '\n' || unicode.Is(unicode.Han, nextChar) {
						// 进一步判断是否是段落结束
						// 如果后面是对话开始（引号），则可能是同一段对话
						if nextChar == '「' || nextChar == '『' || nextChar == '"' || nextChar == '\'' {
							return false // 对话继续，不分段
						}
						return true
					}
				}
			}
		}
	}

	// 规则3：连续的省略号后
	if char == '…' && pos+1 < len(runes) && runes[pos+1] == '…' {
		// 连续省略号后可能是段落结束
		if pos+2 < len(runes) && runes[pos+2] == '…' {
			return true
		}
	}

	// 规则4：单独的句号（没有引号闭合的情况）
	if char == '。' {
		// 如果后面是文本结尾
		if pos+1 >= len(runes) {
			return true
		}
		// 如果后面是中文字符，且不是引号，可能是段落结束
		if pos+1 < len(runes) {
			nextChar := runes[pos+1]
			if unicode.Is(unicode.Han, nextChar) && nextChar != '「' && nextChar != '『' && nextChar != '"' && nextChar != '\'' {
				// 进一步检查：如果后面是对话开始，可能是同一段
				if pos+2 < len(runes) {
					nextNextChar := runes[pos+2]
					if nextNextChar == '「' || nextNextChar == '『' || nextNextChar == '"' || nextNextChar == '\'' {
						return false // 对话继续，不分段
					}
				}
				return true
			}
		}
	}

	// 规则5：句号 + 引号闭合 + 文本结尾
	if char == '。' && pos+1 < len(runes) {
		nextChar := runes[pos+1]
		if nextChar == '」' || nextChar == '』' || nextChar == '"' || nextChar == '\'' {
			// 引号闭合后，检查是否是文本结尾
			if pos+2 >= len(runes) {
				return true
			}
		}
	}

	return false
}

// mergeShortParagraphs 合并过短的段落
func (nf *NewlineFixer) mergeShortParagraphs(paragraphs []string) []string {
	if len(paragraphs) <= 1 {
		return paragraphs
	}

	var merged []string
	current := paragraphs[0]

	for i := 1; i < len(paragraphs); i++ {
		next := paragraphs[i]

		// 如果当前段落过短，且不是章节标题，则与下一段合并
		if nf.isShortParagraph(current) && !nf.isChapterTitleText(current) {
			current = current + "\n" + next
		} else {
			merged = append(merged, current)
			current = next
		}
	}

	// 添加最后一个段落
	merged = append(merged, current)

	return merged
}

// isShortParagraph 检查段落是否过短
func (nf *NewlineFixer) isShortParagraph(paragraph string) bool {
	return len([]rune(paragraph)) < nf.MinParagraphLength
}

// isChapterTitleText 检查文本是否是章节标题
func (nf *NewlineFixer) isChapterTitleText(text string) bool {
	text = strings.TrimSpace(text)

	// 检查常见的章节标题格式
	if strings.HasPrefix(text, "◇") && strings.HasSuffix(text, "◇") {
		return true
	}

	if strings.HasPrefix(text, "第") && (strings.HasSuffix(text, "章") || strings.HasSuffix(text, "节") || strings.HasSuffix(text, "回")) {
		return true
	}

	// 检查数字+顿号格式
	if len(text) >= 2 {
		firstChar := rune(text[0])
		secondChar := rune(text[1])
		if (firstChar == '一' || firstChar == '二' || firstChar == '三' ||
			firstChar == '四' || firstChar == '五' || firstChar == '六' ||
			firstChar == '七' || firstChar == '八' || firstChar == '九' || firstChar == '十') &&
			secondChar == '、' {
			return true
		}
	}

	return false
}

// FixWithPosition 使用位置信息进行精确修复
func (nf *NewlineFixer) FixWithPosition(content string, positions []int) NewlineFixResult {
	result := NewlineFixResult{
		Content:  content,
		Original: content,
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	if !nf.EnableAutoFix || len(content) == 0 || len(positions) == 0 {
		return result
	}

	// 在指定位置插入换行符
	runes := []rune(content)
	var newContent strings.Builder

	lastPos := 0
	for _, pos := range positions {
		if pos > lastPos && pos < len(runes) {
			newContent.WriteString(string(runes[lastPos:pos]))
			newContent.WriteString("\n\n")
			lastPos = pos

			result.Stats["newlines_added"]++
		}
	}
	newContent.WriteString(string(runes[lastPos:]))

	result.Content = newContent.String()

	if result.Content != content {
		result.Changes = append(result.Changes, preprocess.Change{
			Type:        "newline_fix_position",
			Original:    content,
			Replacement: result.Content,
			Position:    0,
			Confidence:  0.9,
		})
	}

	return result
}

// DetectParagraphBoundaries 检测段落边界（不修改内容，只返回位置）
func (nf *NewlineFixer) DetectParagraphBoundaries(content string) []int {
	runes := []rune(content)
	var positions []int

	for i := 0; i < len(runes); i++ {
		if nf.isParagraphBreak(runes, i) {
			positions = append(positions, i+1)
		}
	}

	return positions
}
