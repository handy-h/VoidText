package processor

import (
	"os"
	"strings"
	"testing"
)

func TestNewlineFixer_Chapter4TitleDetection(t *testing.T) {
	content, err := os.ReadFile("../../test_data/chapter4_test.txt")
	if err != nil {
		t.Fatalf("读取测试文件失败: %v", err)
	}

	text := string(content)
	t.Logf("输入文本前30字符: %q", string([]rune(text)[:30]))
	t.Logf("输入文本换行符数: %d", strings.Count(text, "\n"))

	fixer := NewNewlineFixer()
	fixer.MinParagraphLength = 40

	// 先检查 needsNewlineFix
	needFix := fixer.needsNewlineFix(text)
	t.Logf("needsNewlineFix: %v", needFix)

	result := fixer.Fix(text)

	paragraphs := strings.Split(result.Content, "\n\n")
	t.Logf("=== 分段结果：共 %d 个段落 ===", len(paragraphs))
	for i, p := range paragraphs {
		runeCount := len([]rune(p))
		preview := p
		if runeCount > 60 {
			preview = string([]rune(p)[:60]) + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", "↵")
		t.Logf("  段落%2d [%3d字符] %s", i+1, runeCount, preview)
	}

	// 核心验证：◇ 第四章 ◇ 应被识别为独立章节标题
	foundTitle := false
	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "◇ 第四章 ◇" {
			foundTitle = true
			break
		}
	}
	if !foundTitle {
		t.Errorf("'◇ 第四章 ◇' 未被识别为独立章节标题")
	}
}
