package processor

import (
	"testing"
)

func TestParseRulesConfig_typoMapAsMap(t *testing.T) {
	// typoMap 为标准 JSON 对象
	jsonStr := `{"typoMap":{"名子":"名字","在坐":"在座"}}`
	cfg, err := ParseRulesConfig(jsonStr)
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	if cfg.TypoMap["名子"] != "名字" {
		t.Errorf("期望 名子->名字，实际 %v", cfg.TypoMap)
	}
	if cfg.TypoMap["在坐"] != "在座" {
		t.Errorf("期望 在坐->在座，实际 %v", cfg.TypoMap)
	}
}

func TestParseRulesConfig_typoMapAsString(t *testing.T) {
	// typoMap 为前端 textarea 发送的字符串格式
	jsonStr := `{"typoMap":"名子=名字,在坐=在座"}`
	cfg, err := ParseRulesConfig(jsonStr)
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	if cfg.TypoMap["名子"] != "名字" {
		t.Errorf("期望 名子->名字，实际 %v", cfg.TypoMap)
	}
	if cfg.TypoMap["在坐"] != "在座" {
		t.Errorf("期望 在坐->在座，实际 %v", cfg.TypoMap)
	}
}

func TestParseRulesConfig_typoMapEmpty(t *testing.T) {
	// typoMap 为空字符串
	jsonStr := `{"typoMap":""}`
	cfg, err := ParseRulesConfig(jsonStr)
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	if len(cfg.TypoMap) != 0 {
		t.Errorf("期望空 map，实际 %v", cfg.TypoMap)
	}
}

func TestParseRulesConfig_typoMapOmitted(t *testing.T) {
	// typoMap 字段缺失
	jsonStr := `{"enableBasicCleaning":true}`
	cfg, err := ParseRulesConfig(jsonStr)
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	if cfg.TypoMap == nil {
		t.Error("TypoMap 不应为 nil")
	}
	if len(cfg.TypoMap) != 0 {
		t.Errorf("期望空 map，实际 %v", cfg.TypoMap)
	}
}

func TestParseRulesConfig_adBlacklistAsString(t *testing.T) {
	// adBlacklist 为字符串格式
	jsonStr := `{"adBlacklist":"广告1,广告2,广告3"}`
	cfg, err := ParseRulesConfig(jsonStr)
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	if len(cfg.AdBlacklist) != 3 {
		t.Errorf("期望 3 项，实际 %d 项: %v", len(cfg.AdBlacklist), cfg.AdBlacklist)
	}
	if cfg.AdBlacklist[0] != "广告1" || cfg.AdBlacklist[1] != "广告2" || cfg.AdBlacklist[2] != "广告3" {
		t.Errorf("黑名单内容不符: %v", cfg.AdBlacklist)
	}
}

func TestParseRulesConfig_fullConfig(t *testing.T) {
	// 完整配置，typoMap 和 adBlacklist 都是字符串
	jsonStr := `{
		"enableBasicCleaning": true,
		"traditionalToSimple": true,
		"enableVectorDetection": true,
		"similarityThreshold": 0.90,
		"enableModelRepair": true,
		"typoMap": "名子=名字",
		"adBlacklist": "本站提供"
	}`
	cfg, err := ParseRulesConfig(jsonStr)
	if err != nil {
		t.Fatalf("解析配置失败: %v", err)
	}
	if !cfg.EnableBasicCleaning {
		t.Error("EnableBasicCleaning 应为 true")
	}
	if !cfg.TraditionalToSimple {
		t.Error("TraditionalToSimple 应为 true")
	}
	if cfg.SimilarityThreshold != 0.90 {
		t.Errorf("SimilarityThreshold 期望 0.90，实际 %f", cfg.SimilarityThreshold)
	}
	if cfg.TypoMap["名子"] != "名字" {
		t.Errorf("typoMap 解析失败: %v", cfg.TypoMap)
	}
	if len(cfg.AdBlacklist) != 1 || cfg.AdBlacklist[0] != "本站提供" {
		t.Errorf("adBlacklist 解析失败: %v", cfg.AdBlacklist)
	}
}
