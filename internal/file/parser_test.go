package file

import (
	"os"
	"testing"

	"voidtext/internal/config"
)

func initTestConfig() {
	config.AppConfigInstance = config.AppConfig{
		DataDir:        os.TempDir(),
		NameSeparators: "-|—|·|~|_| ",
	}
}

func TestParseFileName_ShouldExtractAuthorAndTitle(t *testing.T) {
	initTestConfig()

	result := ParseFileName("张三~射雕英雄传.txt")
	if result.Author != "张三" {
		t.Errorf("ParseFileName() Author = %s, want 张三", result.Author)
	}
	if result.Title != "射雕英雄传" {
		t.Errorf("ParseFileName() Title = %s, want 射雕英雄传", result.Title)
	}
}

func TestParseFileName_ShouldHandleNoSeparator(t *testing.T) {
	initTestConfig()

	result := ParseFileName("射雕英雄传.txt")
	if result.Author != "" {
		t.Errorf("ParseFileName() Author = %s, want empty", result.Author)
	}
	if result.Title != "射雕英雄传" {
		t.Errorf("ParseFileName() Title = %s, want 射雕英雄传", result.Title)
	}
}

func TestParseFileName_ShouldHandleDashSeparator(t *testing.T) {
	initTestConfig()

	result := ParseFileName("肖忉-赶尸家族.txt")
	if result.Author != "肖忉" {
		t.Errorf("ParseFileName() Author = %s, want 肖忉", result.Author)
	}
	if result.Title != "赶尸家族" {
		t.Errorf("ParseFileName() Title = %s, want 赶尸家族", result.Title)
	}
}

func TestParseFileName_ShouldHandleUnderscoreSeparator(t *testing.T) {
	initTestConfig()

	result := ParseFileName("作者_书名.txt")
	if result.Author != "作者" {
		t.Errorf("ParseFileName() Author = %s, want 作者", result.Author)
	}
	if result.Title != "书名" {
		t.Errorf("ParseFileName() Title = %s, want 书名", result.Title)
	}
}

func TestParseFileName_ShouldHandleSpaceSeparator(t *testing.T) {
	initTestConfig()

	result := ParseFileName("金庸 天龙八部.txt")
	if result.Author != "金庸" {
		t.Errorf("ParseFileName() Author = %s, want 金庸", result.Author)
	}
	if result.Title != "天龙八部" {
		t.Errorf("ParseFileName() Title = %s, want 天龙八部", result.Title)
	}
}

func TestParseFileName_ShouldHandleNoExtension(t *testing.T) {
	initTestConfig()

	result := ParseFileName("射雕英雄传")
	if result.Title != "射雕英雄传" {
		t.Errorf("ParseFileName() Title = %s, want 射雕英雄传", result.Title)
	}
}

func TestIsLikelyAuthor_ShouldReturnTrueForShortString(t *testing.T) {
	if !isLikelyAuthor("张三") {
		t.Errorf("isLikelyAuthor('张三') should return true")
	}
}

func TestIsLikelyAuthor_ShouldReturnFalseForLongString(t *testing.T) {
	longName := "这是一个非常非常长的名字超过十个字符"
	if isLikelyAuthor(longName) {
		t.Errorf("isLikelyAuthor() should return false for long string")
	}
}

func TestIsLikelyAuthor_ShouldReturnFalseForEmptyString(t *testing.T) {
	if isLikelyAuthor("") {
		t.Errorf("isLikelyAuthor('') should return false")
	}
}
