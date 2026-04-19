package postprocess

import (
	"testing"
)

func TestPostprocess_EmptyContent(t *testing.T) {
	result := Postprocess("")

	if result.Content != "" {
		t.Errorf("Expected empty content, got '%s'", result.Content)
	}
}

func TestPostprocess_NormalContent(t *testing.T) {
	content := "这是一段正常的文本。"
	result := Postprocess(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
}

func TestOptimizeFormat_MultipleNewlines(t *testing.T) {
	content := "段落1\n\n\n\n段落2"
	result := Postprocess(content)

	if result.Content == content {
		t.Error("Expected multiple newlines to be optimized")
	}
}

func TestOptimizeFormat_ParagraphSpacing(t *testing.T) {
	content := "段落1\n段落2"
	result := Postprocess(content)

	if result.Content != "段落1\n\n段落2" {
		t.Errorf("Expected paragraph spacing added, got '%s'", result.Content)
	}
}

func TestOptimizeFormat_TrailingWhitespace(t *testing.T) {
	content := "  文本内容  "
	result := Postprocess(content)

	if result.Content != "文本内容" {
		t.Errorf("Expected trimmed content, got '%s'", result.Content)
	}
}

func TestOrganizeChapters_ChineseChapter(t *testing.T) {
	content := "前言内容\n第一章 开始\n第二章 继续"
	result := Postprocess(content)

	if result.Stats["chapters"] < 1 {
		t.Errorf("Expected at least 1 chapter detected, got %d", result.Stats["chapters"])
	}
}

func TestOrganizeChapters_NumberedChapter(t *testing.T) {
	content := "前言内容\n第1章 开始\n第2章 继续"
	result := Postprocess(content)

	if result.Stats["chapters"] < 1 {
		t.Errorf("Expected at least 1 chapter detected, got %d", result.Stats["chapters"])
	}
}

func TestOrganizeChapters_NoChapters(t *testing.T) {
	content := "普通文本内容，没有章节标题。"
	result := Postprocess(content)

	if result.Stats["chapters"] != 0 {
		t.Errorf("Expected 0 chapters detected, got %d", result.Stats["chapters"])
	}
}

func TestNormalizePunctuation_ChinesePunctuation(t *testing.T) {
	content := "Hello, world. How are you? I'm fine!"
	result := Postprocess(content)

	expected := "Hello， world。 How are you？ I'm fine！"
	if result.Content != expected {
		t.Errorf("Expected Chinese punctuation, got '%s'", result.Content)
	}
}

func TestNormalizePunctuation_RepeatedPunctuation(t *testing.T) {
	content := "真的吗？？？"
	result := Postprocess(content)

	if result.Content != "真的吗？" {
		t.Errorf("Expected single punctuation, got '%s'", result.Content)
	}
}

func TestNormalizePunctuation_NoChange(t *testing.T) {
	content := "正常的中文文本，没有英文标点。"
	result := Postprocess(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
}

func TestPostprocess_ChangesAndStats(t *testing.T) {
	content := "段落1\n段落2"
	result := Postprocess(content)

	if result.Original != content {
		t.Errorf("Expected original to be set, got '%s'", result.Original)
	}
}
