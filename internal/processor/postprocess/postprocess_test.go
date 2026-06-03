package postprocess

import (
	"testing"
)

func TestOrganizeChapters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "◇格式章节标题前后添加空行",
			input:    "这是前面的内容◇ 五 ◇这是后面的内容",
			expected: "这是前面的内容\n\n◇ 五 ◇\n\n这是后面的内容",
		},
		{
			name:     "第X章格式章节标题前后添加空行",
			input:    "这是前面的内容第五章这是后面的内容",
			expected: "这是前面的内容\n\n第五章\n\n这是后面的内容",
		},
		{
			name:     "多个章节标题",
			input:    "开头◇ 一 ◇第一章内容◇ 二 ◇第二章内容",
			expected: "开头\n\n◇ 一 ◇\n\n第一章\n\n内容\n\n◇ 二 ◇\n\n第二章\n\n内容",
		},
		{
			name:     "章节标题已经在独立行",
			input:    "前面内容\n◇ 三 ◇\n后面内容",
			expected: "前面内容\n◇ 三 ◇\n后面内容",
		},
		{
			name:     "混合格式章节标题",
			input:    "开头◇ 一 ◇第一章内容第五章第二章内容",
			expected: "开头\n\n◇ 一 ◇\n\n第一章\n\n内容\n\n第五章\n\n第二章\n\n内容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := organizeChapters(PostprocessResult{
				Content:  tt.input,
				Original: tt.input,
				Changes:  []Change{},
				Stats:    make(map[string]int),
			})

			if result.Content != tt.expected {
				t.Errorf("organizeChapters() = %q, want %q", result.Content, tt.expected)
			}
		})
	}
}