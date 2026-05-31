package processor

import (
	"regexp"
	"strings"
	"unicode"

	"voidtext/internal/processor/preprocess"
)

// 预编译正则表达式，避免每次调用时重复编译
var (
	controlRegex      = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	multiNewlineRegex = regexp.MustCompile(`\n{3,}`)
	spaceRegex        = regexp.MustCompile(`[ \t]{2,}`)

	// 广告过滤正则（预编译，避免每次清洗时重新编译）
	adRegexes = []struct {
		regex *regexp.Regexp
		name  string
	}{
		{regexp.MustCompile(`关注微信公众号[\""]?[^\""\n]*[\""]?获取更多免费小说[！!]?`), "微信公众号广告"},
		{regexp.MustCompile(`关注[^\n]{0,20}公众号[^\n]*`), "公众号推广"},
		{regexp.MustCompile(`本文由[^\n]{2,30}提供[^\n]*`), "来源声明"},
		{regexp.MustCompile(`更多精彩内容请访问[^\n]*`), "网站推广"},
		{regexp.MustCompile(`下载[^\n]{0,10}APP[^\n]*`), "APP推广"},
		{regexp.MustCompile(`www\.[a-zA-Z0-9]+\.[a-zA-Z]{2,}\S*`), "网址广告"},
		{regexp.MustCompile(`https?://\S+`), "链接广告"},
		{regexp.MustCompile(`欢迎加入[^\n]{0,20}(群|频道|社区)[^\n]*`), "群推广"},
		{regexp.MustCompile(`QQ群[:：]\d+[^\n]*`), "QQ群推广"},
		{regexp.MustCompile(`本章未完[，,]点击下一页继续[^\n]*`), "翻页提示"},
		{regexp.MustCompile(`天才一秒记住[^\n]*`), "网站推广"},
		{regexp.MustCompile(`手机用户请浏览[^\n]*`), "手机推广"},
	}
)

// BasicCleanResult 基础清洗结果
type BasicCleanResult struct {
	Content  string              `json:"content"`
	Original string              `json:"original"`
	Changes  []preprocess.Change `json:"changes"`
	Stats    map[string]int      `json:"stats"`
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
		"&nbsp;":  " ",
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&apos;":  "'",
		"&#160;":  " ",
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

	runes := []rune(result.Content)
	var builder strings.Builder
	changes := []preprocess.Change{}

	for i, char := range runes {
		if replacement, exists := halfToFull[char]; exists {
			if bc.isInChineseContext(runes, i) {
				builder.WriteRune(replacement)
				changes = append(changes, preprocess.Change{
					Type:        "punctuation",
					Original:    string(char),
					Replacement: string(replacement),
					Position:    i,
				})
				result.Stats["punctuation_normalized"]++
			} else {
				builder.WriteRune(char)
			}
		} else {
			builder.WriteRune(char)
		}
	}

	if len(changes) > 0 {
		result.Content = builder.String()
		result.Changes = append(result.Changes, changes...)
	}

	return result
}

// isInChineseContext 判断指定rune位置是否处于中文上下文中
func (bc *BasicCleaner) isInChineseContext(runes []rune, position int) bool {
	checkRange := 3
	start := position - checkRange
	if start < 0 {
		start = 0
	}
	end := position + checkRange
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
	result.Content = controlRegex.ReplaceAllString(result.Content, "")

	// 将Windows换行符统一为Unix换行符
	result.Content = strings.ReplaceAll(result.Content, "\r\n", "\n")
	result.Content = strings.ReplaceAll(result.Content, "\r", "\n")

	// 将3个及以上连续换行压缩为2个（保留段落分隔）
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
		// 扩展常用繁简映射（覆盖小说常见用字）
		'龍': '龙', '馬': '马', '魚': '鱼', '鳥': '鸟',
		'熱': '热', '愛': '爱', '後': '后', '從': '从',
		'來': '来', '這': '这', '個': '个', '們': '们',
		'嗎': '吗', '為': '为', '過': '过', '發': '发',
		'現': '现', '進': '进', '問': '问', '對': '对',
		'頭': '头', '實': '实', '義': '义', '務': '务',
		'處': '处', '總': '总', '親': '亲', '結': '结',
		'無': '无', '裡': '里', '當': '当', '讓': '让',
		'還': '还', '應': '应', '該': '该', '氣': '气',
		'車': '车', '軍': '军', '辦': '办', '強': '强',
		'戰': '战', '師': '师', '醫': '医', '藥': '药',
		'護': '护', '衛': '卫', '員': '员', '費': '费',
		'買': '买', '賣': '卖', '貨': '货', '質': '质',
		'詞': '词', '語': '语', '識': '识', '評': '评',
		'試': '试', '證': '证', '認': '认', '誤': '误',
		'誌': '志', '記': '记', '許': '许', '論': '论',
		'設': '设', '計': '计', '變': '变', '達': '达',
		'遠': '远', '運': '运', '連': '连', '選': '选',
		'遺': '遗', '陳': '陈', '陸': '陆', '隊': '队',
		'階': '阶', '隻': '只', '雙': '双', '難': '难',
		'雲': '云', '須': '须', '項': '项', '順': '顺',
		'預': '预', '頑': '顽', '頓': '顿', '頻': '频',
		'題': '题', '額': '额', '顏': '颜', '風': '风',
		'飛': '飞', '飯': '饭', '飲': '饮', '飼': '饲',
		'館': '馆', '駐': '驻', '駕': '驾', '騎': '骑',
		'驅': '驱', '驗': '验', '髮': '发', '鬥': '斗',
		'鬱': '郁', '鮮': '鲜', '麗': '丽', '麥': '麦',
		'黃': '黄', '黨': '党', '齊': '齐', '齣': '出',
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

// removeAdvertisements 移除广告内容（使用预编译正则，避免每次调用重复编译）
func (bc *BasicCleaner) removeAdvertisements(result BasicCleanResult) BasicCleanResult {
	for _, ad := range adRegexes {
		matches := ad.regex.FindAllStringIndex(result.Content, -1)

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
