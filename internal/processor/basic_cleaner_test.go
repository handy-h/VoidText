package processor

import (
	"testing"
	"txt-cleaning/internal/processor/preprocess"
)

func TestNewBasicCleaner(t *testing.T) {
	cleaner := NewBasicCleaner(true)

	if !cleaner.EnableTraditionalToSimple {
		t.Error("Expected EnableTraditionalToSimple to be true")
	}
}

func TestClean_EmptyContent(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	result := cleaner.Clean("")

	if result.Content != "" {
		t.Errorf("Expected empty content, got '%s'", result.Content)
	}
}

func TestClean_NormalContent(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "这是一段正常的文本。"
	result := cleaner.Clean(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
}

func TestCleanHTMLEntities_Nbsp(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "文本&nbsp;内容"
	result := cleaner.Clean(content)

	if result.Content != "文本 内容" {
		t.Errorf("Expected nbsp replaced with space, got '%s'", result.Content)
	}
}

func TestCleanHTMLEntities_Amp(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "文本&amp;内容"
	result := cleaner.Clean(content)

	if result.Content != "文本&内容" {
		t.Errorf("Expected amp replaced with &, got '%s'", result.Content)
	}
}

func TestCleanHTMLEntities_Multiple(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "&lt;div&gt;&nbsp;&quot;test&quot;&lt;/div&gt;"
	result := cleaner.Clean(content)

	if result.Stats["html_entities_cleaned"] == 0 {
		t.Error("Expected HTML entities to be cleaned")
	}
}

func TestNormalizePunctuation_HalfToFull(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "中文文本，使用半角标点, and more."
	result := cleaner.Clean(content)

	if result.Stats["punctuation_normalized"] == 0 {
		t.Error("Expected punctuation to be normalized")
	}
}

func TestNormalizePunctuation_ChineseContext(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "中文文本，使用半角标点, and more."
	result := cleaner.Clean(content)

	if result.Stats["punctuation_normalized"] == 0 {
		t.Error("Expected punctuation in Chinese context to be normalized")
	}
}

func TestNormalizeWhitespace_ControlChars(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "文本\x00内容\x1F"
	result := cleaner.Clean(content)

	if result.Content == content {
		t.Error("Expected control characters to be removed")
	}
}

func TestNormalizeWhitespace_WindowsNewlines(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "段落1\r\n段落2"
	result := cleaner.Clean(content)

	if result.Content != "段落1\n段落2" {
		t.Errorf("Expected Windows newlines converted to Unix, got '%s'", result.Content)
	}
}

func TestNormalizeWhitespace_MultipleNewlines(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "段落1\n\n\n\n段落2"
	result := cleaner.Clean(content)

	if result.Content != "段落1\n\n段落2" {
		t.Errorf("Expected multiple newlines compressed to 2, got '%s'", result.Content)
	}
}

func TestNormalizeWhitespace_TrailingSpaces(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "文本   "
	result := cleaner.Clean(content)

	if result.Content != "文本" {
		t.Errorf("Expected trailing spaces removed, got '%s'", result.Content)
	}
}

func TestTraditionalToSimple_Enabled(t *testing.T) {
	cleaner := NewBasicCleaner(true)

	content := "臺灣"
	result := cleaner.Clean(content)

	if result.Content != "台湾" {
		t.Errorf("Expected traditional converted to simple, got '%s'", result.Content)
	}
}

func TestTraditionalToSimple_Disabled(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "臺灣"
	result := cleaner.Clean(content)

	if result.Content != content {
		t.Errorf("Expected traditional unchanged when disabled, got '%s'", result.Content)
	}
}

func TestTraditionalToSimple_MultipleChars(t *testing.T) {
	cleaner := NewBasicCleaner(true)

	content := "臺灣國"
	result := cleaner.Clean(content)

	if result.Content != "台湾国" {
		t.Errorf("Expected '臺灣國' converted to '台湾国', got '%s'", result.Content)
	}
}

func TestRemoveAdvertisements_WeChat(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "正文内容\n关注微信公众号test获取更多免费小说！"
	result := cleaner.Clean(content)

	if result.Stats["advertisements_removed"] == 0 {
		t.Error("Expected WeChat advertisement to be removed")
	}
}

func TestRemoveAdvertisements_URL(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "正文内容\nwww.example.com"
	result := cleaner.Clean(content)

	if result.Stats["advertisements_removed"] == 0 {
		t.Error("Expected URL advertisement to be removed")
	}
}

func TestRemoveAdvertisements_NoAds(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "这是一段正常的文本，没有广告。"
	result := cleaner.Clean(content)

	if result.Stats["advertisements_removed"] != 0 {
		t.Errorf("Expected 0 advertisements removed, got %d", result.Stats["advertisements_removed"])
	}
}

func TestClean_ChangesRecorded(t *testing.T) {
	cleaner := NewBasicCleaner(true)

	content := "&nbsp;臺灣"
	result := cleaner.Clean(content)

	if len(result.Changes) == 0 {
		t.Error("Expected changes to be recorded")
	}

	for _, change := range result.Changes {
		if change.Type == "" {
			t.Error("Expected change type to be set")
		}
	}
}

func TestClean_StatsRecorded(t *testing.T) {
	cleaner := NewBasicCleaner(true)

	content := "&nbsp;臺灣"
	result := cleaner.Clean(content)

	if len(result.Stats) == 0 {
		t.Error("Expected stats to be recorded")
	}
}

func TestIsInChineseContext_Chinese(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "中文文本"
	if !cleaner.isInChineseContext(content, 0) {
		t.Error("Expected to be in Chinese context")
	}
}

func TestIsInChineseContext_English(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	content := "English text"
	if cleaner.isInChineseContext(content, 0) {
		t.Error("Expected not to be in Chinese context")
	}
}

func TestAddChange(t *testing.T) {
	cleaner := NewBasicCleaner(false)

	result := BasicCleanResult{
		Content:  "test",
		Original: "test",
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	cleaner.addChange(&result, "test_type", "original", "replacement", 0)

	if len(result.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(result.Changes))
	}

	if result.Changes[0].Type != "test_type" {
		t.Errorf("Expected change type 'test_type', got '%s'", result.Changes[0].Type)
	}
}
