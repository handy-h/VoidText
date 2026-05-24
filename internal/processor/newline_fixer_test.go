package processor

import (
	"strings"
	"testing"
)

func TestNewlineFixer_Fix(t *testing.T) {
	tests := []struct {
		name                string
		content             string
		expectedParagraphs  int
		shouldHaveNewlines  bool
	}{
		{
			name: "缺少换行符的中文小说",
			content: "狼窝里安了窃听器。」一辆抛锚的汽车给哥拉家族带来希望。族长使用调虎离山计智取窃听器。居住在白塔山上的几十只兔子同属于哥拉家族。哥拉家族祖祖辈辈定居在白塔山上。按兔家族繁衍的速度来计算，哥拉家族的数量早该逾万了。",
			expectedParagraphs: 3,
			shouldHaveNewlines: true,
		},
		{
			name: "正常换行的文本",
			content: "第一段内容。\n\n第二段内容。",
			expectedParagraphs: 2,
			shouldHaveNewlines: false,
		},
		{
			name: "空内容",
			content: "",
			expectedParagraphs: 0,
			shouldHaveNewlines: false,
		},
		{
			name: "短内容",
			content: "短内容",
			expectedParagraphs: 1,
			shouldHaveNewlines: false,
		},
		{
			name: "带有章节标题的文本",
			content: "◇ 一 ◇ 章节标题内容。一些正文内容。一些正文内容。一些正文内容。一些正文内容。一些正文内容。一些正文内容。◇ 二 ◇ 第二章标题。更多内容。更多内容。更多内容。",
			expectedParagraphs: 3,
			shouldHaveNewlines: true,
		},
		{
			name: "带有对话的文本",
			content: "族长一边看一边喜上眉梢。什么东西？远处的兔子们问。这是一张窃听器的使用说明书。族长兴奋得扬扬手中的纸，咱们可以摆脱狼的袭击啦！真的？不可能吧？怎么摆脱？兔子们七嘴八舌。族长说现在咱们去那辆车上弄一台窃听器来！",
			expectedParagraphs: 2,
			shouldHaveNewlines: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixer := NewNewlineFixer()
			result := fixer.Fix(tt.content)

			// 检查是否应该有换行符
			if tt.shouldHaveNewlines {
				if !strings.Contains(result.Content, "\n") {
					t.Errorf("期望有换行符，但没有找到")
				}
			}

			// 检查段落数量
			if tt.expectedParagraphs > 0 {
				paragraphs := strings.Split(result.Content, "\n\n")
				actualParagraphs := 0
				for _, p := range paragraphs {
					if strings.TrimSpace(p) != "" {
						actualParagraphs++
					}
				}
				if actualParagraphs < tt.expectedParagraphs {
					t.Errorf("期望至少 %d 个段落，实际得到 %d 个段落", tt.expectedParagraphs, actualParagraphs)
				}
			}

			// 检查变更记录
			if tt.shouldHaveNewlines && len(result.Changes) == 0 {
				t.Errorf("期望有变更记录，但没有找到")
			}
		})
	}
}

func TestNewlineFixer_needsNewlineFix(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "短内容不需要修复",
			content:  "短内容",
			expected: false,
		},
		{
			name:     "有足够换行符的内容",
			content:  "第一段。\n\n第二段。\n\n第三段。",
			expected: false,
		},
		{
			name:     "缺少换行符的中文内容",
			content:  strings.Repeat("这是一段很长的中文内容，没有换行符。", 10),
			expected: true,
		},
		{
			name:     "有中文标点但换行符足够",
			content:  "第一段内容。\n\n第二段内容。\n\n第三段内容。",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixer := NewNewlineFixer()
			result := fixer.needsNewlineFix(tt.content)
			if result != tt.expected {
				t.Errorf("needsNewlineFix(%q) = %v, 期望 %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestNewlineFixer_isChapterTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		pos      int
		expected bool
	}{
		{
			name:     "菱形章节标题",
			content:  "◇ 一 ◇",
			pos:      0,
			expected: true,
		},
		{
			name:     "第X章格式",
			content:  "第一章 开始",
			pos:      0,
			expected: true,
		},
		{
			name:     "普通文本",
			content:  "这是普通文本",
			pos:      0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixer := NewNewlineFixer()
			runes := []rune(tt.content)
			result := fixer.isChapterTitle(runes, tt.pos)
			if result != tt.expected {
				t.Errorf("isChapterTitle(%q, %d) = %v, 期望 %v", tt.content, tt.pos, result, tt.expected)
			}
		})
	}
}

func TestNewlineFixer_isParagraphBreak(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		pos      int
		expected bool
	}{
		{
			name:     "句号后",
			content:  "内容。",
			pos:      2,
			expected: true,
		},
		{
			name:     "引号闭合后",
			content:  "内容」",
			pos:      1,
			expected: false, // 单独的引号闭合不是段落结束
		},
		{
			name:     "对话结束",
			content:  "内容。」",
			pos:      2,
			expected: true,
		},
		{
			name:     "普通字符",
			content:  "内容",
			pos:      0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixer := NewNewlineFixer()
			runes := []rune(tt.content)
			result := fixer.isParagraphBreak(runes, tt.pos)
			if result != tt.expected {
				t.Errorf("isParagraphBreak(%q, %d) = %v, 期望 %v", tt.content, tt.pos, result, tt.expected)
			}
		})
	}
}

func TestNewlineFixer_DetectParagraphBoundaries(t *testing.T) {
	t.Skip("DetectParagraphBoundaries 方法尚未实现")
}

func TestNewlineFixer_MergeShortParagraphs(t *testing.T) {
	paragraphs := []string{"短", "这是一个很长的段落，包含了很多内容，应该保持独立。"}
	fixer := NewNewlineFixer()
	merged := mergeShortParagraphsHelper(fixer, paragraphs)

	// 短段落应该与下一个段落合并
	if len(merged) != 1 {
		t.Errorf("期望合并后有 1 个段落，实际得到 %d 个", len(merged))
	}
}

// 辅助函数：测试用
func mergeShortParagraphsHelper(fixer *NewlineFixer, paragraphs []string) []string {
	return fixer.mergeShortParagraphs(paragraphs)
}
