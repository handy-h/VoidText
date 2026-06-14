package processor

import (
	"os"
	"strings"
	"testing"
)

func TestNewlineFixer_Chapter3BoundaryAnalysis(t *testing.T) {
	content, err := os.ReadFile("../../test_data/chapter3_test.txt")
	if err != nil {
		t.Fatalf("读取测试文件失败: %v", err)
	}

	fixer := NewNewlineFixer()
	result := fixer.Fix(string(content))

	paragraphs := strings.Split(result.Content, "\n\n")

	t.Logf("=== 段落边界分析（共 %d 段）===", len(paragraphs))
	for i, p := range paragraphs {
		runes := []rune(p)
		runeCount := len(runes)

		// 提取段落末尾10字符和开头10字符
		tailLen := 15
		if tailLen > runeCount {
			tailLen = runeCount
		}
		headLen := 15
		if headLen > runeCount {
			headLen = runeCount
		}
		tail := string(runes[runeCount-tailLen:])
		head := string(runes[:headLen])
		tail = strings.ReplaceAll(tail, "\n", "↵")
		head = strings.ReplaceAll(head, "\n", "↵")

		t.Logf("段落%2d [%3d字符] 尾=%q → 头=%q", i+1, runeCount, tail, head)

		// 检查段落间的衔接
		if i > 0 {
			prevRunes := []rune(paragraphs[i-1])
			prevTail := string(prevRunes[len(prevRunes)-1])
			currHead := string(runes[0])
			t.Logf("         衔接: ...%s → %s...", prevTail, currHead)
		}
	}
}
