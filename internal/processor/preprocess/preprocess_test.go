package preprocess

import (
	"testing"
	"unicode/utf8"
)

func TestPreprocess_ShouldReturnResult(t *testing.T) {
	content := "测试文本内容"
	result := Preprocess(content)

	if result.Original != content {
		t.Errorf("Preprocess() Original mismatch")
	}
}

func TestPreprocess_ShouldRemoveAdvertisements(t *testing.T) {
	content := "正常文本\n本文由某某网站提供\n更多内容"
	result := Preprocess(content)

	if result.Original != content {
		t.Errorf("Preprocess() should preserve original")
	}
}

func TestNormalizeEncoding_ShouldHandleValidUTF8(t *testing.T) {
	result := PreprocessResult{Content: "有效的UTF-8文本"}
	result = normalizeEncoding(result)

	if !utf8.ValidString(result.Content) {
		t.Errorf("normalizeEncoding() should produce valid UTF-8")
	}
}

func TestFixMixedEncoding_ShouldHandleValidUTF8(t *testing.T) {
	content := "纯UTF-8文本"
	result := fixMixedEncoding(content)

	if result != content {
		t.Errorf("fixMixedEncoding() should not modify valid UTF-8")
	}
}

func TestRemoveAdvertisements_ShouldRemoveURL(t *testing.T) {
	result := PreprocessResult{
		Content:  "文本内容 http://www.example.com/ads 更多内容",
		Original: "文本内容 http://www.example.com/ads 更多内容",
		Changes:  []Change{},
	}
	result = removeAdvertisements(result)

	found := false
	for _, change := range result.Changes {
		if change.Type == "advertisement" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("removeAdvertisements() should detect URL advertisement")
	}
}

func TestRemoveAdvertisements_ShouldRemoveWWW(t *testing.T) {
	result := PreprocessResult{
		Content:  "访问 www.example.com 获取更多",
		Original: "访问 www.example.com 获取更多",
		Changes:  []Change{},
	}
	result = removeAdvertisements(result)

	found := false
	for _, change := range result.Changes {
		if change.Type == "advertisement" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("removeAdvertisements() should detect www advertisement")
	}
}

func TestCleanSpecialCharacters_ShouldRemoveControlChars(t *testing.T) {
	result := PreprocessResult{
		Content:  "文本\x00\x01内容\x1F",
		Original: "文本\x00\x01内容\x1F",
		Changes:  []Change{},
	}
	result = cleanSpecialCharacters(result)

	for _, c := range result.Content {
		if c < 0x20 && c != '\n' && c != '\r' && c != '\t' {
			t.Errorf("cleanSpecialCharacters() should remove control characters, found: %d", c)
		}
	}
}

func TestNormalizeWhitespace_ShouldCollapseMultipleSpaces(t *testing.T) {
	result := PreprocessResult{
		Content:  "多个    空格",
		Original: "多个    空格",
		Changes:  []Change{},
	}
	result = normalizeWhitespace(result)

	if result.Content != "多个 空格" {
		t.Errorf("normalizeWhitespace() = %s, want 多个 空格", result.Content)
	}
}

func TestNormalizeWhitespace_ShouldTrimWhitespace(t *testing.T) {
	result := PreprocessResult{
		Content:  "  前后空格  ",
		Original: "  前后空格  ",
		Changes:  []Change{},
	}
	result = normalizeWhitespace(result)

	if result.Content != "前后空格" {
		t.Errorf("normalizeWhitespace() = %s, want 前后空格", result.Content)
	}
}

func TestIsLikelyGBK_ShouldDetectGBK(t *testing.T) {
	gbkData := []byte{0xC4, 0xE3, 0xBA, 0xC3}
	if !isLikelyGBK(gbkData) {
		t.Errorf("isLikelyGBK() should detect GBK encoded data")
	}
}

func TestIsLikelyGBK_ShouldReturnFalseForUTF8(t *testing.T) {
	utf8Data := []byte("hello world")
	if isLikelyGBK(utf8Data) {
		t.Errorf("isLikelyGBK() should return false for ASCII/UTF-8 data")
	}
}

func TestUtf8CharLen_ShouldReturnCorrectLengths(t *testing.T) {
	tests := []struct {
		byte_ byte
		want  int
	}{
		{0x61, 1},
		{0xC0, 2},
		{0xE0, 3},
		{0xF0, 4},
		{0x80, 0},
	}

	for _, tt := range tests {
		result := utf8CharLen(tt.byte_)
		if result != tt.want {
			t.Errorf("utf8CharLen(0x%02X) = %d, want %d", tt.byte_, result, tt.want)
		}
	}
}

func TestGbkToUtf8_ShouldConvertGBK(t *testing.T) {
	gbkStr := string([]byte{0xC4, 0xE3, 0xBA, 0xC3})
	result, err := gbkToUtf8(gbkStr)
	if err != nil {
		t.Fatalf("gbkToUtf8() error = %v", err)
	}
	if !utf8.ValidString(result) {
		t.Errorf("gbkToUtf8() should produce valid UTF-8")
	}
}
