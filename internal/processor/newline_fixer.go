package processor

import (
	"strings"
	"unicode"

	"voidtext/internal/config"
	"voidtext/internal/logging"
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
}

// NewNewlineFixer 创建换行符修复器
func NewNewlineFixer() *NewlineFixer {
	minLen := config.AppConfigInstance.MinParagraphLength
	if minLen <= 0 {
		minLen = 80
	}
	return &NewlineFixer{
		EnableAutoFix:      true,
		MinParagraphLength: minLen,
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
	needFix := nf.needsNewlineFix(content)
	logging.Info("newline_fix_detect", map[string]interface{}{
		"content_len":         len(content),
		"rune_count":          len([]rune(content)),
		"newline_count":       strings.Count(content, "\n"),
		"needs_fix":           needFix,
		"min_paragraph_length": nf.MinParagraphLength,
	})
	if !needFix {
		result.Stats["skipped"] = 1
		return result
	}

	// 执行换行符修复
	result = nf.fixMissingNewlines(result)

	return result
}

// needsNewlineFix 检测是否需要换行符修复
// 通过分析换行符密度和单行长度来判断
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

	// 检查是否有超长行（单行超过500字符说明缺少段落分隔）
	hasLongLine := false
	if newlineCount > 0 {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if len([]rune(strings.TrimSpace(line))) > 500 {
				hasLongLine = true
				break
			}
		}
	}

	// 检查是否是中文小说（有句号、引号等）
	hasChinesePunctuation := strings.Contains(content, "。") ||
		strings.Contains(content, "」") ||
		strings.Contains(content, "』") ||
		strings.Contains(content, "？") ||
		strings.Contains(content, "！")

	if !hasChinesePunctuation {
		return false
	}

	// 条件1：换行密度过低 → 需要修复
	if charsPerNewline > densityThreshold || newlineCount == 0 {
		return true
	}

	// 条件2：存在超长行 → 需要修复（即使整体密度看似正常）
	if hasLongLine {
		return true
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

	logging.Info("newline_fix_split", map[string]interface{}{
		"paragraph_count":   len(paragraphs),
		"newline_count_new": strings.Count(newContent, "\n"),
		"newline_count_old": strings.Count(content, "\n"),
		"content_changed":   newContent != content,
	})

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
// 使用保守的分段策略：只在章节标题、对话切换、话题转换处分割
// 连续叙述保持在同一自然段中，避免过度切分
func (nf *NewlineFixer) splitIntoParagraphs(content string) []string {
	var paragraphs []string
	var currentParagraph strings.Builder

	runes := []rune(content)
	i := 0

	for i < len(runes) {
		char := runes[i]

		// 检查是否是章节标题（如 ◇ 一 ◇、第X章、一、）
		if titleEnd := nf.isChapterTitleAt(runes, i); titleEnd > i {
			// 保存当前段落
			if currentParagraph.Len() > 0 {
				paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
				currentParagraph.Reset()
			}
			// 提取章节标题为独立段落
			title := string(runes[i:titleEnd])
			paragraphs = append(paragraphs, strings.TrimSpace(title))
			i = titleEnd
			continue
		}

		// 将当前字符添加到段落（确保句号、引号等标点不丢失）
		currentParagraph.WriteRune(char)

		// 检测是否为段落分割点，返回应跳过的字符数
		if skip := nf.detectParagraphBreak(runes, i); skip > 0 {
			if currentParagraph.Len() > 0 {
				paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
				currentParagraph.Reset()
			}
			i += skip
			continue
		}

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

// isChapterTitleAt 检查指定位置是否为章节标题的开始
// 如果是，返回标题结束位置（不含）；否则返回 pos
func (nf *NewlineFixer) isChapterTitleAt(runes []rune, pos int) int {
	// 格式1：◇ 一 ◇
	if runes[pos] == '◇' {
		for j := pos + 1; j < len(runes) && j < pos+20; j++ {
			if runes[j] == '◇' {
				return j + 1
			}
		}
	}

	// 格式2：第X章、第X节等
	if pos+1 < len(runes) && runes[pos] == '第' {
		for j := pos + 1; j < len(runes) && j < pos+10; j++ {
			if unicode.IsDigit(runes[j]) || runes[j] == '章' || runes[j] == '节' || runes[j] == '回' {
				return nf.findTitleEnd(runes, pos)
			}
		}
	}

	// 格式3：中文数字+顿号（如 一、二、三、）
	if pos+1 < len(runes) {
		cnNums := "一二三四五六七八九十"
		if strings.ContainsRune(cnNums, runes[pos]) && runes[pos+1] == '、' {
			return nf.findTitleEnd(runes, pos)
		}
	}

	return pos
}

// findTitleEnd 查找章节标题的结束位置
func (nf *NewlineFixer) findTitleEnd(runes []rune, start int) int {
	for i := start; i < len(runes) && i < start+50; i++ {
		if runes[i] == '\n' {
			return i + 1
		}
		// ◇ 闭合标记
		if i > start && runes[i] == '◇' {
			return i + 1
		}
		// 句子结束标点表示标题结束（适用于"一、标题内容。"格式）
		if i > start+2 && (runes[i] == '。' || runes[i] == '！' || runes[i] == '？') {
			return i + 1
		}
	}
	end := start + 20
	if end > len(runes) {
		end = len(runes)
	}
	return end
}

// skipSpaces 跳过空格和制表符，返回下一个非空格字符的位置
func skipSpaces(runes []rune, pos int) int {
	for i := pos; i < len(runes); i++ {
		if runes[i] != ' ' && runes[i] != '\t' {
			return i
		}
	}
	return len(runes)
}

// detectParagraphBreak 检测位置 pos 是否为段落分割点
// 返回值 > 0 表示是分割点，值为应跳过的字符数
// 返回 0 表示不是分割点
//
// 分段信号（按优先级）：
// 1. 章节标题（由 splitIntoParagraphs 单独处理）
// 2. 结束标点 + 闭合引号 + 开启引号（对话切换）
// 3. 结束标点 + 时间标记/话题转换词
func (nf *NewlineFixer) detectParagraphBreak(runes []rune, pos int) int {
	if pos >= len(runes) {
		return 0
	}

	char := runes[pos]

	// 处理结束标点（。！？）后的段落边界
	if char == '。' || char == '！' || char == '？' {
		skip := 1
		afterPunct := pos + 1

		// 跳过空格
		afterPunct = skipSpaces(runes, afterPunct)

		// 文本结束：最后一个句子结束，无需分割
		if afterPunct >= len(runes) {
			return 0
		}

		// 处理结束标点后的引号
		// 注意：直引号 " 在中文 OCR 文本中同时用于开合引号，无法静态区分
		// 策略：将 endPunct + " 视为潜在段落边界，通过后续上下文判断
		if isQuoteChar(runes[afterPunct]) {
			afterQuote := skipSpaces(runes, afterPunct+1)

			if afterQuote >= len(runes) {
				return 0 // 文本结束
			}

			// 明确闭合引号（」』）后出现开启引号 = 新对话 → 分段
			if isUnambiguousClosingQuote(runes[afterPunct]) && isOpeningQuote(runes[afterQuote]) {
				return afterQuote - pos
			}

			// 直引号 " 在结束标点后：需区分两种本质不同的模式
			// 模式A: "皮皮鲁问。" → 闭合引号+对话归属标签（标签在引号外，归属上一句）
			// 模式B: 小神马说。"皮皮鲁问小神马。" → 叙述结束+新对话开始（动词在引号内）
			// 优先级：。" > 。作为段落结尾信号
			if isAmbiguousQuote(runes[afterPunct]) {
				afterQuoteNext := skipSpaces(runes, afterPunct+1)

				if afterQuoteNext >= len(runes) {
					return 0 // 文本结束
				}

				// 引号后紧跟另一个引号 → 当前引号是闭合引号，新引号开启新对话 → 分段
				if isQuoteChar(runes[afterQuoteNext]) {
					return afterPunct - pos // 在引号前分段
				}

				// 检查引号后是否为 "XXX说/问/道。" 格式的对话归属标签
				// 该模式的动词在引号外部，是上一句引文的归属标签
				// 排除模式B：动词在新引号内部（如 "皮皮鲁问小神马。"），属于新对话
				if !isQuoteAttributionTag(runes, afterQuoteNext) {
					return afterPunct - pos // 不是归属标签 → 分段
				}
				// 是归属标签，但需检查归属句是否已经结束
				// "苏宁说。" → 归属句以句号结束 → 归属已完成，下一引号是新对话 → 分段
				// "苏宁说，" → 归属句以逗号延续 → 归属未完成，下一引号是归属的延续 → 不分段
				if attrTagEndsSentence(runes, afterQuoteNext) {
					return afterPunct - pos // 归属标签已完成，分段
				}
			}

			// 引号后有时间/话题转换标记 → 分段
			if hasTimeMarker(runes, afterQuote) || hasTopicTransition(runes, afterQuote) {
				return afterQuote - pos
			}

			return skip
		}

		// 信号2：时间标记 → 分段（如"第二天""几个月后"）
		if hasTimeMarker(runes, afterPunct) {
			return afterPunct - pos
		}

		// 信号3：话题/场景转换 → 分段
		if hasTopicTransition(runes, afterPunct) {
			return afterPunct - pos
		}

		// 无明确段落边界信号，不分割（连续叙述保持在同一段落）
		// 仅在检测到引号变化、时间标记或话题转换词时才分割
		return 0
	}
	// 注：直引号 " 的处理已在上方结束标点块中完成
	if isUnambiguousClosingQuote(char) && pos > 0 {
		prevChar := runes[pos-1]
		if prevChar == '。' || prevChar == '！' || prevChar == '？' || prevChar == '…' {
			nextPos := skipSpaces(runes, pos+1)
			if nextPos >= len(runes) {
				return 0 // 文本结束
			}
			// 开启引号 = 新对话 → 分段
			if isOpeningQuote(runes[nextPos]) {
				return nextPos - pos
			}
			// 时间标记或话题转换 → 分段
			if hasTimeMarker(runes, nextPos) || hasTopicTransition(runes, nextPos) {
				return nextPos - pos
			}
		}
	}

	return 0
}

// isQuoteChar 检查是否为任何引号字符
func isQuoteChar(r rune) bool {
	return r == '"' || r == '\'' || r == '「' || r == '」' || r == '『' || r == '』'
}

// isUnambiguousClosingQuote 检查是否为明确的闭合引号（不可能是开启引号）
func isUnambiguousClosingQuote(r rune) bool {
	return r == '」' || r == '』'
}

// isAmbiguousQuote 检查是否为无法区分开合的直引号
func isAmbiguousQuote(r rune) bool {
	return r == '"' || r == '\''
}

// isOpeningQuote 检查是否为明确的开启引号
func isOpeningQuote(r rune) bool {
	return r == '"' || r == '「' || r == '『' || r == '\''
}

// isDialogueTag 检查 pos 处是否为说话动词（用于判断 "...XXX说" 格式的对话标签）
// 检查前2-4个字符是否构成 "角色名+说话动词" 模式
func isDialogueTag(runes []rune, pos int) bool {
	saidVerbs := []rune{'说', '问', '喊', '叫', '道', '答', '骂', '笑', '哭', '吼', '叹', '嚷'}
	for offset := 0; offset < 4 && pos+offset < len(runes); offset++ {
		for _, v := range saidVerbs {
			if runes[pos+offset] == v {
				return true
			}
		}
		// 遇到标点或引号停止扫描
		if runes[pos+offset] == '，' || runes[pos+offset] == '。' || runes[pos+offset] == '！' ||
			runes[pos+offset] == '？' || runes[pos+offset] == '"' {
			break
		}
	}
	return false
}

// isQuoteAttributionTag 检查 pos 处是否为 "XXX说/问/道。" 格式的对话归属标签
// 与 isDialogueTag 的关键区别：要求动词后紧跟句尾标点（。！？），
// 确保匹配的是独立的归属短句（如 "皮皮鲁问。"），
// 而不是新对话内部的动词（如 "皮皮鲁问小神马。" 中的 "问" 后面是角色名，不是标点）
// 同时支持双字动词（如 "答道"、"喊道"）
func isQuoteAttributionTag(runes []rune, pos int) bool {
	saidVerbs := []rune{'说', '问', '喊', '叫', '道', '答', '骂', '笑', '哭', '吼', '叹', '嚷'}
	endPuncts := []rune{'。', '！', '？'}
	isPunctOrQuote := func(r rune) bool {
		return r == '，' || r == '。' || r == '！' || r == '？' || r == '"'
	}

	for offset := 0; offset < 4 && pos+offset < len(runes); offset++ {
		char := runes[pos+offset]
		// 遇到标点或引号停止扫描（说明动词不在预期位置）
		if isPunctOrQuote(char) {
			break
		}

		// 检查当前字符是否为说话动词
		isVerb := false
		for _, v := range saidVerbs {
			if char == v {
				isVerb = true
				break
			}
		}
		if !isVerb {
			continue
		}

		// 当前字符是动词，检查是否为双字动词（如 "答道"、"喊道"、"叫道"）
		if pos+offset+1 < len(runes) {
			nextChar := runes[pos+offset+1]
			for _, v := range saidVerbs {
				if nextChar == v {
					// 双字动词，检查动词后是否紧跟句尾标点
					verbEnd := pos + offset + 2
					if verbEnd < len(runes) {
						for _, p := range endPuncts {
							if runes[verbEnd] == p {
								return true
							}
						}
					}
					return false
				}
			}
		}

		// 单字动词，检查动词后是否紧跟句尾标点
		verbEnd := pos + offset + 1
		if verbEnd < len(runes) {
			for _, p := range endPuncts {
				if runes[verbEnd] == p {
					return true
				}
			}
		}
		// 动词后不是句尾标点 → 不是归属标签
		return false
	}
	return false
}

// attrTagEndsSentence 检查归属标签是否以句尾标点（。！？）结束
// 用于区分 "苏宁说。"（归属完成，后续引号是新对话）和 "苏宁说，"（归属延续，后续引号是归属内容）
func attrTagEndsSentence(runes []rune, pos int) bool {
	saidVerbs := []rune{'说', '问', '喊', '叫', '道', '答', '骂', '笑', '哭', '吼', '叹', '嚷'}
	endPuncts := []rune{'。', '！', '？'}

	for offset := 0; offset < 4 && pos+offset < len(runes); offset++ {
		char := runes[pos+offset]
		if char == '，' || char == '。' || char == '！' || char == '？' || char == '"' {
			break
		}

		// 检查是否为说话动词
		isVerb := false
		for _, v := range saidVerbs {
			if char == v {
				isVerb = true
				break
			}
		}
		if !isVerb {
			continue
		}

		// 找到动词，检查动词后的标点类型
		// 优先检查双字动词（如 "答道"、"喊道"）
		if pos+offset+1 < len(runes) {
			nextChar := runes[pos+offset+1]
			for _, v := range saidVerbs {
				if nextChar == v {
					// 双字动词，检查动词后的标点
					verbEnd := pos + offset + 2
					if verbEnd < len(runes) {
						for _, p := range endPuncts {
							if runes[verbEnd] == p {
								return true
							}
						}
					}
					return false
				}
			}
		}

		// 单字动词，检查动词后的标点
		verbEnd := pos + offset + 1
		if verbEnd < len(runes) {
			for _, p := range endPuncts {
				if runes[verbEnd] == p {
					return true
				}
			}
		}
		return false
	}
	return false
}

// hasTimeMarker 检查位置 pos 处是否有时间标记（表示新段落开始）
func hasTimeMarker(runes []rune, pos int) bool {
	timeMarkers := []string{
		"第二天", "第三天", "几天后", "几天以后", "几天过去了",
		"几个月后", "几个月以后", "一年后", "几年后", "多年以后", "多年后",
		"那年", "那年夏天", "那年冬天", "那年秋天", "那年春天",
		"这天", "那天", "当天", "每天", "后来",
		"此刻", "此时", "这时", "那时", "这时侯", "这时候", "那时候",
		"一会儿", "过了", "不久", "随即", "随后", "紧接着",
		"终于", "最终", "最后",
		"从此", "从那以后", "从今以后", "打那以后",
		"转眼间", "转眼间", "一晃", "不知不觉",
		"早晨", "中午", "下午", "傍晚", "晚上", "深夜", "半夜", "凌晨", "清晨",
	}
	if beginsWithAny(runes, pos, timeMarkers) {
		return true
	}
	// 数字格式时间标记：如"10个月后"、"3天后"、"2年后"
	return isNumericTimeMarker(runes, pos)
}

// isNumericTimeMarker 检查是否为数字格式的时间标记
// 匹配模式：数字 + "个"(可选) + 时间单位 + "后/以后/前/以前"
func isNumericTimeMarker(runes []rune, pos int) bool {
	if pos >= len(runes) {
		return false
	}
	// 必须以数字开头
	i := pos
	for i < len(runes) && (runes[i] >= '0' && runes[i] <= '9') {
		i++
	}
	if i == pos {
		return false // 没有数字
	}
	// 跳过可选的"个"
	if i < len(runes) && runes[i] == '个' {
		i++
	}
	// 检查时间单位
	timeUnits := []string{"月", "天", "年", "小时", "分钟", "星期", "周"}
	matchedUnit := false
	for _, unit := range timeUnits {
		unitRunes := []rune(unit)
		if i+len(unitRunes) <= len(runes) {
			match := true
			for j, ur := range unitRunes {
				if runes[i+j] != ur {
					match = false
					break
				}
			}
			if match {
				i += len(unitRunes)
				matchedUnit = true
				break
			}
		}
	}
	if !matchedUnit {
		return false
	}
	// 检查后缀
	suffixes := []string{"以后", "后", "以前", "前"}
	for _, suffix := range suffixes {
		suffixRunes := []rune(suffix)
		if i+len(suffixRunes) <= len(runes) {
			match := true
			for j, sr := range suffixRunes {
				if runes[i+j] != sr {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// hasTopicTransition 检查位置 pos 处是否有话题/场景转换标记
// 注意：角色名不在此列表中——角色名后跟说话动词（说/问/道）
// 是对话标签，不应触发分段；角色名后跟动作虽属新叙述，
// 但由 MinParagraphLength 的合并机制来兜底处理
func hasTopicTransition(runes []rune, pos int) bool {
	transitions := []string{
		// 叙述转换
		"就这样", "于是", "可是", "然而", "但是", "不过",
		"忽然", "突然", "没想到", "谁知", "没想到的是",
		// 话题转换
		"果然", "原来", "其实", "毕竟", "当然",
		"再说", "另外", "此外", "事实上",
		"且说", "再说", "却说", "话虽如此",
		// 场景描写
		"外面的世界", "外面",
		"另一边", "与此同时",
		// 时间推移（补充）
		"当晚", "次日", "翌日", "那天晚上", "这天晚上",
		"从这天起", "打这天起",
	}
	return beginsWithAny(runes, pos, transitions)
}

// beginsWithAny 检查 runes[pos:] 是否以 words 中的某个词开头
func beginsWithAny(runes []rune, pos int, words []string) bool {
	if pos >= len(runes) {
		return false
	}
	for _, word := range words {
		wordRunes := []rune(word)
		if pos+len(wordRunes) > len(runes) {
			continue
		}
		match := true
		for j, wr := range wordRunes {
			if runes[pos+j] != wr {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// mergeShortParagraphs 合并过短的段落
// 策略：短段落（< MinParagraphLength）持续与后续段落合并，直到达到最小长度
// 章节标题保持独立，不与内容合并
func (nf *NewlineFixer) mergeShortParagraphs(paragraphs []string) []string {
	if len(paragraphs) <= 1 {
		return paragraphs
	}

	var merged []string
	i := 0

	for i < len(paragraphs) {
		current := paragraphs[i]

		// 章节标题保持独立，不合并
		if nf.isChapterTitleText(current) {
			merged = append(merged, current)
			i++
			continue
		}

		// 过短的段落持续与后续段落合并，直到达到最小长度
		for nf.isShortParagraph(current) && i+1 < len(paragraphs) {
			i++
			next := paragraphs[i]
			// 如果下一段是章节标题，先结束合并（标题必须独立）
			if nf.isChapterTitleText(next) {
				i-- // 回退，让标题在下轮循环中独立处理
				break
			}
			current = current + "\n" + next
		}

		merged = append(merged, current)
		i++
	}

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
	runes := []rune(text)
	if len(runes) >= 2 {
		firstChar := runes[0]
		secondChar := runes[1]
		if (firstChar == '一' || firstChar == '二' || firstChar == '三' ||
			firstChar == '四' || firstChar == '五' || firstChar == '六' ||
			firstChar == '七' || firstChar == '八' || firstChar == '九' || firstChar == '十') &&
			secondChar == '、' {
			return true
		}
	}

	return false
}

// SplitChapterTitles 从文本中分离章节标题
// 独立于换行修复器，可在任何处理阶段后调用，确保章节标题始终独立成段
// 检测格式：◇...◇、第X章/节/回、中文数字+顿号（一、二、三...）
func SplitChapterTitles(content string) string {
	runes := []rune(content)
	if len(runes) < 5 {
		return content
	}

	var result strings.Builder
	i := 0

	for i < len(runes) {
		titleEnd := detectChapterTitleAt(runes, i)
		if titleEnd <= i {
			result.WriteRune(runes[i])
			i++
			continue
		}

		// 找到章节标题，确保前后有段落分隔
		title := string(runes[i:titleEnd])

		// 标题前加分隔（如果前面有内容且不以双换行结尾）
		if result.Len() > 0 {
			existing := result.String()
			if !strings.HasSuffix(existing, "\n\n") {
				if strings.HasSuffix(existing, "\n") {
					result.WriteString("\n")
				} else {
					result.WriteString("\n\n")
				}
			}
		}

		result.WriteString(title)

		// 跳过标题后的空白字符（空格、换行）
		i = titleEnd
		for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t' || runes[i] == '\n' || runes[i] == '\r') {
			i++
		}

		// 标题后加分隔（如果后面还有内容）
		if i < len(runes) {
			result.WriteString("\n\n")
		}
	}

	return result.String()
}

// detectChapterTitleAt 检测 pos 处是否为章节标题，返回标题结束位置
// 与 isChapterTitleAt 类似，但作为独立函数使用，不依赖 NewlineFixer 实例
func detectChapterTitleAt(runes []rune, pos int) int {
	if pos >= len(runes) {
		return pos
	}

	// 格式1：◇...◇
	if runes[pos] == '◇' {
		for j := pos + 1; j < len(runes) && j < pos+20; j++ {
			if runes[j] == '◇' {
				return j + 1
			}
		}
	}

	// 格式2：第X章/节/回
	if pos+1 < len(runes) && runes[pos] == '第' {
		for j := pos + 1; j < len(runes) && j < pos+10; j++ {
			if runes[j] == '章' || runes[j] == '节' || runes[j] == '回' {
				return j + 1
			}
			if unicode.IsDigit(runes[j]) {
				continue
			}
			break
		}
	}

	// 格式3：中文数字+顿号（一、二、三...）
	if pos+1 < len(runes) {
		cnNums := "一二三四五六七八九十"
		if strings.ContainsRune(cnNums, runes[pos]) && runes[pos+1] == '、' {
			// 找到该行结束位置
			for j := pos + 2; j < len(runes) && j < pos+30; j++ {
				if runes[j] == '\n' {
					return j
				}
			}
			// 没有换行，取到下一个句号或内容开始处
			for j := pos + 2; j < len(runes) && j < pos+30; j++ {
				if runes[j] == '。' || runes[j] == '！' || runes[j] == '？' {
					return j + 1
				}
			}
		}
	}

	return pos
}