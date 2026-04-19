package preprocess

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPreprocess_EmptyContent(t *testing.T) {
	result := Preprocess("")

	if result.Content != "" {
		t.Errorf("Expected empty content, got '%s'", result.Content)
	}
	if result.Original != "" {
		t.Errorf("Expected empty original, got '%s'", result.Original)
	}
}

func TestPreprocess_NormalContent(t *testing.T) {
	content := "这是一段正常的文本。"
	result := Preprocess(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
	if result.Original != content {
		t.Errorf("Expected original unchanged, got '%s'", result.Original)
	}
}

func TestRemoveAdvertisements_URL(t *testing.T) {
	content := "正文内容。www.example.com 广告链接。"
	result := Preprocess(content)

	if len(result.Changes) == 0 {
		t.Error("Expected changes to be recorded for URL advertisement")
	}

	if result.Content == content {
		t.Error("Expected content to be modified after removing URL")
	}
}

func TestRemoveAdvertisements_HTTP(t *testing.T) {
	content := "访问 https://example.com 获取更多信息。"
	result := Preprocess(content)

	if len(result.Changes) == 0 {
		t.Error("Expected changes to be recorded for HTTP advertisement")
	}
}

func TestRemoveAdvertisements_NoAds(t *testing.T) {
	content := "这是一段没有广告的正文内容。"
	result := Preprocess(content)

	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestCleanSpecialCharacters_ControlChars(t *testing.T) {
	content := "文本\x00内容\x1F测试\x7F"
	result := Preprocess(content)

	if result.Content == content {
		t.Error("Expected control characters to be removed")
	}

	if len(result.Content) < len(content) {
		// 控制字符被移除
		return
	}
	t.Errorf("Expected content length to decrease, got same length")
}

func TestCleanSpecialCharacters_NoControlChars(t *testing.T) {
	content := "正常的文本内容，没有控制字符。"
	result := Preprocess(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
}

func TestNormalizeWhitespace_MultipleSpaces(t *testing.T) {
	content := "文本   内容    测试"
	result := Preprocess(content)

	if result.Content == content {
		t.Error("Expected multiple spaces to be normalized")
	}
}

func TestNormalizeWhitespace_TrailingWhitespace(t *testing.T) {
	content := "  文本内容  "
	result := Preprocess(content)

	if result.Content != "文本内容" {
		t.Errorf("Expected trimmed content, got '%s'", result.Content)
	}
}

func TestNormalizeWhitespace_Newlines(t *testing.T) {
	content := "段落1\n\n\n段落2"
	result := Preprocess(content)

	if result.Content == content {
		t.Error("Expected newlines to be normalized")
	}
}

func TestPreprocess_ChangesRecorded(t *testing.T) {
	content := "文本   内容 www.example.com"
	result := Preprocess(content)

	if len(result.Changes) == 0 {
		t.Error("Expected changes to be recorded")
	}

	for _, change := range result.Changes {
		if change.Type == "" {
			t.Error("Expected change type to be set")
		}
	}
}

func TestNormalizeEncoding_GBKToUTF8(t *testing.T) {
	// GBK编码的"你好"：\xC4\xE3\xBA\xC3
	gbkContent := "\xC4\xE3\xBA\xC3"
	result := Preprocess(gbkContent)

	// 验证转换后的内容是有效的UTF-8
	if !utf8.ValidString(result.Content) {
		t.Errorf("Expected valid UTF-8 after encoding normalization, got invalid UTF-8")
	}

	// 验证转换后的内容包含"你好"
	if result.Content != "你好" {
		t.Errorf("Expected '你好', got '%s'", result.Content)
	}
}

func TestNormalizeEncoding_ValidUTF8(t *testing.T) {
	content := "正常的UTF-8内容"
	result := Preprocess(content)

	// 已经是有效的UTF-8，不应该被修改
	if result.Content != content {
		t.Errorf("Expected valid UTF-8 unchanged, got '%s'", result.Content)
	}
}

func TestFixMixedEncoding(t *testing.T) {
	// 模拟混合编码：使用字节数组构造
	// UTF-8: "第一行" = \xE7\xAC\xAC\xE4\xB8\x80\xE8\xA1\x8C
	// GBK: "\xB5\xDA\xB6\xFE\xD0\xD0" (第二行)
	utf8Line := []byte{0xE7, 0xAC, 0xAC, 0xE4, 0xB8, 0x80, 0xE8, 0xA1, 0x8C} // 第一行
	gbkLine := []byte{0xB5, 0xDA, 0xB6, 0xFE, 0xD0, 0xD0}                    // 第二行 (GBK)

	// 构造混合内容
	mixedBytes := append(utf8Line, '\n')
	mixedBytes = append(mixedBytes, gbkLine...)
	mixedContent := string(mixedBytes)

	result := Preprocess(mixedContent)

	// 验证转换后的内容是有效的UTF-8
	if !utf8.ValidString(result.Content) {
		t.Errorf("Expected valid UTF-8 after fixing mixed encoding, got invalid UTF-8")
	}

	// 验证包含正确的中文字符（注意：空白规范化会将换行符替换为空格）
	if !strings.Contains(result.Content, "第一行") || !strings.Contains(result.Content, "第二行") {
		t.Errorf("Expected content to contain both lines, got '%s'", result.Content)
	}
}

func TestGbkToUtf8_InvalidGBK(t *testing.T) {
	// 无效的GBK序列
	invalidContent := "\xFF\xFF\xFF\xFF"
	result := Preprocess(invalidContent)

	// 应该返回空字符串或清理后的内容
	if len(result.Content) > 0 && !utf8.ValidString(result.Content) {
		t.Errorf("Expected valid UTF-8 or empty string for invalid GBK input")
	}
}
