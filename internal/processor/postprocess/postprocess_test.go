package postprocess

import (
	"testing"
)

func TestPostprocess_ShouldReturnResult(t *testing.T) {
	content := "测试文本内容"
	result := Postprocess(content)

	if result.Original != content {
		t.Errorf("Postprocess() Original mismatch")
	}
}

func TestOptimizeFormat_ShouldNormalizeNewlines(t *testing.T) {
	result := PostprocessResult{
		Content:  "段落1\n\n\n\n段落2",
		Original: "段落1\n\n\n\n段落2",
		Changes:  []Change{},
		Stats:    make(map[string]int),
	}
	result = optimizeFormat(result)

	if result.Content != "段落1\n\n段落2" {
		t.Errorf("optimizeFormat() = %q, want %q", result.Content, "段落1\n\n段落2")
	}
}

func TestOptimizeFormat_ShouldTrimWhitespace(t *testing.T) {
	result := PostprocessResult{
		Content:  "  前后空格  ",
		Original: "  前后空格  ",
		Changes:  []Change{},
		Stats:    make(map[string]int),
	}
	result = optimizeFormat(result)

	if result.Content != "前后空格" {
		t.Errorf("optimizeFormat() = %q, want %q", result.Content, "前后空格")
	}
}

func TestOrganizeChapters_ShouldDetectChineseChapter(t *testing.T) {
	result := PostprocessResult{
		Content:  "前面内容第一章标题后面内容",
		Original: "前面内容第一章标题后面内容",
		Changes:  []Change{},
		Stats:    make(map[string]int),
	}
	result = organizeChapters(result)

	if result.Stats["chapters"] < 1 {
		t.Errorf("organizeChapters() should detect 第一章")
	}
}

func TestOrganizeChapters_ShouldDetectNumericChapter(t *testing.T) {
	result := PostprocessResult{
		Content:  "前面内容第1章标题后面内容",
		Original: "前面内容第1章标题后面内容",
		Changes:  []Change{},
		Stats:    make(map[string]int),
	}
	result = organizeChapters(result)

	if result.Stats["chapters"] < 1 {
		t.Errorf("organizeChapters() should detect 第1章")
	}
}

func TestNormalizePunctuation_ShouldConvertToChinese(t *testing.T) {
	result := PostprocessResult{
		Content:  "你好,世界.测试:完成;",
		Original: "你好,世界.测试:完成;",
		Changes:  []Change{},
		Stats:    make(map[string]int),
	}
	result = normalizePunctuation(result)

	if result.Content != "你好，世界。测试：完成；" {
		t.Errorf("normalizePunctuation() = %q, want %q", result.Content, "你好，世界。测试：完成；")
	}
}

func TestNormalizePunctuation_ShouldRemoveDuplicatePunctuation(t *testing.T) {
	result := PostprocessResult{
		Content:  "你好，，，世界。。。测试",
		Original: "你好，，，世界。。。测试",
		Changes:  []Change{},
		Stats:    make(map[string]int),
	}
	result = normalizePunctuation(result)

	if result.Content != "你好，世界。测试" {
		t.Errorf("normalizePunctuation() = %q, want %q", result.Content, "你好，世界。测试")
	}
}
