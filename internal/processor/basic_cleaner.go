package processor

import (
	"regexp"
	"strings"
	"unicode"

	"txt-cleaning/internal/processor/preprocess"
)

// BasicCleanResult 基础清洗结果
type BasicCleanResult struct {
	Content     string   `json:"content"`
	Original    string   `json:"original"`
	Changes     []preprocess.Change `json:"changes"`
	Stats       map[string]int `json:"stats"`
}

// BasicCleaner 基础文本清洗器
type BasicCleaner struct {
	EnableTraditionalToSimple bool
}

// NewBasicCleaner 创建基础清洗器
func NewBasicCleaner(enableTraditionalToSimple bool) *BasicCleaner {
	return &BasicCleaner{
		EnableTraditionalToSimple: enableTraditionalToSimple,
	}
}

// Clean 执行基础文本清洗
func (bc *BasicCleaner) Clean(content string) BasicCleanResult {
	result := BasicCleanResult{
		Content:  content,
		Original: content,
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	result = bc.cleanHTMLEntities(result)
	result = bc.normalizePunctuation(result)
	result = bc.normalizeWhitespace(result)

	if bc.EnableTraditionalToSimple {
		result = bc.traditionalToSimple(result)
	}

	result = bc.removeAdvertisements(result)

	return result
}

// cleanHTMLEntities 清理HTML实体字符
func (bc *BasicCleaner) cleanHTMLEntities(result BasicCleanResult) BasicCleanResult {
	htmlEntities := map[string]string{
		"&nbsp;": " ",
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": "\"",
		"&apos;": "'",
		"&#160;": " ",
		"&#8203;": "",
		"&#8204;": "",
		"&#8205;": "",
	}

	for entity, replacement := range htmlEntities {
		if strings.Contains(result.Content, entity) {
			count := strings.Count(result.Content, entity)
			result.Content = strings.ReplaceAll(result.Content, entity, replacement)
			bc.addChange(&result, "html_entity", entity, replacement, 0)
			result.Stats["html_entities_cleaned"] += count
		}
	}

	return result
}

// normalizePunctuation 规范化标点符号（半角→全角，适用于中文文本）
func (bc *BasicCleaner) normalizePunctuation(result BasicCleanResult) BasicCleanResult {
	halfToFull := map[rune]rune{
		',': '，',
		'.': '。',
		':': '：',
		';': '；',
		'?': '？',
		'!': '！',
		'(': '（',
		')': '）',
		'[': '【',
		']': '】',
	}

	var builder strings.Builder
	changes := []preprocess.Change{}
	position := 0

	for _, char := range result.Content {
		if replacement, exists := halfToFull[char]; exists {
			if bc.isInChineseContext(result.Content, position) {
				builder.WriteRune(replacement)
				changes = append(changes, preprocess.Change{
					Type:        "punctuation",
					Original:    string(char),
					Replacement: string(replacement),
					Position:    position,
				})
				result.Stats["punctuation_normalized"]++
			} else {
				builder.WriteRune(char)
			}
		} else {
			builder.WriteRune(char)
		}
		position++
	}

	if len(changes) > 0 {
		result.Content = builder.String()
		result.Changes = append(result.Changes, changes...)
	}

	return result
}

// isInChineseContext 判断指定位置是否处于中文上下文中
func (bc *BasicCleaner) isInChineseContext(content string, position int) bool {
	runes := []rune(content)
	runePos := 0

	for i, r := range runes {
		if runePos >= position {
			break
		}
		_ = i
		runePos += len(string(r))
	}

	checkRange := 3
	start := runePos - checkRange
	if start < 0 {
		start = 0
	}
	end := runePos + checkRange
	if end > len(runes) {
		end = len(runes)
	}

	for i := start; i < end; i++ {
		if unicode.Is(unicode.Han, runes[i]) {
			return true
		}
	}

	return false
}

// normalizeWhitespace 规范化空白字符（保留段落结构）
func (bc *BasicCleaner) normalizeWhitespace(result BasicCleanResult) BasicCleanResult {
	controlRegex := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	result.Content = controlRegex.ReplaceAllString(result.Content, "")

	// 将Windows换行符统一为Unix换行符
	result.Content = strings.ReplaceAll(result.Content, "\r\n", "\n")
	result.Content = strings.ReplaceAll(result.Content, "\r", "\n")

	// 将3个及以上连续换行压缩为2个（保留段落分隔）
	multiNewlineRegex := regexp.MustCompile(`\n{3,}`)
	if multiNewlineRegex.MatchString(result.Content) {
		result.Content = multiNewlineRegex.ReplaceAllString(result.Content, "\n\n")
		bc.addChange(&result, "whitespace", "多余空行", "双换行", 0)
		result.Stats["blank_lines_normalized"]++
	}

	// 移除行内多余空格（保留换行符）
	lines := strings.Split(result.Content, "\n")
	cleanedLines := []string{}
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		spaceRegex := regexp.MustCompile(`[ \t]{2,}`)
		trimmed = spaceRegex.ReplaceAllString(trimmed, " ")
		cleanedLines = append(cleanedLines, trimmed)
	}
	result.Content = strings.Join(cleanedLines, "\n")

	result.Content = strings.TrimSpace(result.Content)

	return result
}

// traditionalToSimple 繁体转简体
func (bc *BasicCleaner) traditionalToSimple(result BasicCleanResult) BasicCleanResult {
	traditionalToSimpleMap := map[rune]rune{
		'臺': '台', '灣': '湾', '國': '国', '體': '体',
		'會': '会', '學': '学', '電': '电', '腦': '脑',
		'網': '网', '絡': '络', '開': '开', '關': '关',
		'東': '东', '西': '西', '南': '南', '北': '北',
		'經': '经', '濟': '济', '動': '动', '靜': '静',
		'書': '书', '畫': '画', '長': '长', '門': '门',
		'時': '时', '間': '间', '說': '说', '話': '话',
		'見': '见', '視': '视', '聽': '听', '覺': '觉',
		'點': '点', '線': '线', '麵': '面', '機': '机',
	}

	var builder strings.Builder
	changes := []preprocess.Change{}
	position := 0

	for _, char := range result.Content {
		if replacement, exists := traditionalToSimpleMap[char]; exists && unicode.Is(unicode.Han, char) {
			builder.WriteRune(replacement)
			changes = append(changes, preprocess.Change{
				Type:        "traditional_to_simple",
				Original:    string(char),
				Replacement: string(replacement),
				Position:    position,
			})
			result.Stats["traditional_to_simple"]++
		} else {
			builder.WriteRune(char)
		}
		position++
	}

	if len(changes) > 0 {
		result.Content = builder.String()
		result.Changes = append(result.Changes, changes...)
	}

	return result
}

// removeAdvertisements 移除广告内容
func (bc *BasicCleaner) removeAdvertisements(result BasicCleanResult) BasicCleanResult {
	adPatterns := []struct {
		pattern string
		name    string
	}{
		{`关注微信公众号[\""]?[^\""\n]*[\""]?获取更多免费小说[！!]?`, "微信公众号广告"},
		{`关注[^\n]{0,20}公众号[^\n]*`, "公众号推广"},
		{`本文由[^\n]{2,30}提供[^\n]*`, "来源声明"},
		{`更多精彩内容请访问[^\n]*`, "网站推广"},
		{`下载[^\n]{0,10}APP[^\n]*`, "APP推广"},
		{`www\.[a-zA-Z0-9]+\.[a-zA-Z]{2,}[^\s]*`, "网址广告"},
		{`https?://[^\s\n]+`, "链接广告"},
		{`欢迎加入[^\n]{0,20}(群|频道|社区)[^\n]*`, "群推广"},
		{`QQ群[:：]\d+[^\n]*`, "QQ群推广"},
		{`本章未完[，,]点击下一页继续[^\n]*`, "翻页提示"},
		{`天才一秒记住[^\n]*`, "网站推广"},
		{`手机用户请浏览[^\n]*`, "手机推广"},
	}

	for _, ad := range adPatterns {
		regex := regexp.MustCompile(ad.pattern)
		matches := regex.FindAllStringIndex(result.Content, -1)

		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			original := result.Content[match[0]:match[1]]

			bc.addChange(&result, "advertisement", original, "", match[0])

			result.Content = result.Content[:match[0]] + result.Content[match[1]:]
			result.Stats["advertisements_removed"]++
		}
	}

	return result
}

// addChange 添加变更记录
func (bc *BasicCleaner) addChange(result *BasicCleanResult, changeType, original, replacement string, position int) {
	result.Changes = append(result.Changes, preprocess.Change{
		Type:        changeType,
		Original:    original,
		Replacement: replacement,
		Position:    position,
	})
}