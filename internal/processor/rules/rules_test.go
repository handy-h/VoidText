package rules

import (
	"os"
	"strings"
	"testing"

	"txt-cleaning/internal/config"
)

func initTestConfig(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	config.AppConfigInstance = config.AppConfig{
		DataDir: tmpDir,
	}
}

func TestNewRuleManager_ShouldCreateManager(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	if rm == nil {
		t.Fatalf("NewRuleManager() should return non-nil")
	}
}

func TestLoadRules_ShouldCreateDefaultRules(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rules := rm.GetRules()

	if len(rules) == 0 {
		t.Errorf("LoadRules() should create default rules")
	}
}

func TestAddRule_ShouldAddNewRule(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	initialCount := len(rm.GetRules())

	rule := Rule{
		Name:        "测试规则",
		Pattern:     "测试模式",
		Replacement: "替换内容",
		Description: "测试用",
		Enabled:     true,
	}

	err := rm.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules := rm.GetRules()
	if len(rules) != initialCount+1 {
		t.Errorf("AddRule() should increase rule count by 1")
	}
}

func TestAddRule_ShouldAutoGenerateID(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rule := Rule{Name: "自动ID", Pattern: "test", Replacement: "ok", Enabled: true}

	rm.AddRule(rule)

	rules := rm.GetRules()
	lastRule := rules[len(rules)-1]
	if lastRule.ID == "" {
		t.Errorf("AddRule() should auto-generate ID")
	}
}

func TestGetRule_ShouldReturnExistingRule(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rules := rm.GetRules()
	if len(rules) == 0 {
		t.Skip("No rules to test")
	}

	rule, err := rm.GetRule(rules[0].ID)
	if err != nil {
		t.Fatalf("GetRule() error = %v", err)
	}
	if rule.ID != rules[0].ID {
		t.Errorf("GetRule() ID = %s, want %s", rule.ID, rules[0].ID)
	}
}

func TestGetRule_ShouldReturnErrorForNonExistent(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	_, err := rm.GetRule("nonexistent_id")
	if err == nil {
		t.Errorf("GetRule() should return error for non-existent rule")
	}
}

func TestUpdateRule_ShouldUpdateExistingRule(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rule := Rule{Name: "更新测试", Pattern: "旧模式", Replacement: "旧替换", Enabled: true}
	rm.AddRule(rule)

	rules := rm.GetRules()
	lastRule := rules[len(rules)-1]
	lastRule.Pattern = "新模式"

	err := rm.UpdateRule(lastRule)
	if err != nil {
		t.Fatalf("UpdateRule() error = %v", err)
	}

	updated, _ := rm.GetRule(lastRule.ID)
	if updated.Pattern != "新模式" {
		t.Errorf("UpdateRule() Pattern = %s, want 新模式", updated.Pattern)
	}
}

func TestUpdateRule_ShouldReturnErrorForNonExistent(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rule := Rule{ID: "nonexistent", Name: "不存在", Pattern: "test", Enabled: true}

	err := rm.UpdateRule(rule)
	if err == nil {
		t.Errorf("UpdateRule() should return error for non-existent rule")
	}
}

func TestDeleteRule_ShouldRemoveRule(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rule := Rule{Name: "删除测试", Pattern: "test", Replacement: "ok", Enabled: true}
	rm.AddRule(rule)

	rules := rm.GetRules()
	initialCount := len(rules)
	lastRule := rules[len(rules)-1]

	err := rm.DeleteRule(lastRule.ID)
	if err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}

	rules = rm.GetRules()
	if len(rules) != initialCount-1 {
		t.Errorf("DeleteRule() should decrease rule count by 1")
	}
}

func TestDeleteRule_ShouldReturnErrorForNonExistent(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	err := rm.DeleteRule("nonexistent_id")
	if err == nil {
		t.Errorf("DeleteRule() should return error for non-existent rule")
	}
}

func TestApplyRules_ShouldApplyEnabledRules(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rm.AddRule(Rule{
		Name:        "替换测试",
		Pattern:     "名子",
		Replacement: "名字",
		Enabled:     true,
	})

	result := rm.ApplyRules("他的名子很好听")
	if result != "他的名字很好听" {
		t.Errorf("ApplyRules() = %s, want 他的名字很好听", result)
	}
}

func TestApplyRules_ShouldSkipDisabledRules(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rm.AddRule(Rule{
		Name:        "禁用规则",
		Pattern:     "测试禁用",
		Replacement: "替换成功",
		Enabled:     false,
	})

	result := rm.ApplyRules("这是测试禁用内容")
	if strings.Contains(result, "替换成功") {
		t.Errorf("ApplyRules() should skip disabled rules")
	}
}

func TestApplyRules_ShouldHandleInvalidPattern(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rm.AddRule(Rule{
		Name:        "无效正则",
		Pattern:     "[invalid",
		Replacement: "替换",
		Enabled:     true,
	})

	result := rm.ApplyRules("测试文本")
	if result != "测试文本" {
		t.Errorf("ApplyRules() should handle invalid regex gracefully")
	}
}

func TestSaveAndLoadRules_ShouldPersistRules(t *testing.T) {
	initTestConfig(t)

	rm := NewRuleManager()
	rm.AddRule(Rule{
		Name:        "持久化测试",
		Pattern:     "测试",
		Replacement: "替换",
		Enabled:     true,
	})

	rm2 := NewRuleManager()
	rules := rm2.GetRules()

	found := false
	for _, r := range rules {
		if r.Name == "持久化测试" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SaveAndLoadRules() should persist rules to disk")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
