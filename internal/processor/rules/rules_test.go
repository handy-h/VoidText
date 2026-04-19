package rules

import (
	"os"
	"path/filepath"
	"testing"
	"txt-cleaning/internal/config"
)

func setupRulesTest(t *testing.T) (*RuleManager, func()) {
	tempDir, err := os.MkdirTemp("", "rules_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config.AppConfigInstance.DataDir = tempDir

	manager := NewRuleManager()

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return manager, cleanup
}

func TestNewRuleManager(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	rules := manager.GetRules()
	if len(rules) == 0 {
		t.Error("Expected default rules to be loaded")
	}
}

func TestLoadRules_DefaultRules(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rules := manager.GetRules()

	if len(rules) < 2 {
		t.Errorf("Expected at least 2 default rules, got %d", len(rules))
	}
}

func TestAddRule(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rule := Rule{
		Name:        "测试规则",
		Pattern:     "测试",
		Replacement: "替换",
		Description: "这是一个测试规则",
		Enabled:     true,
	}

	err := manager.AddRule(rule)
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	rules := manager.GetRules()
	if len(rules) < 3 {
		t.Errorf("Expected at least 3 rules after adding, got %d", len(rules))
	}
}

func TestAddRule_WithID(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rule := Rule{
		ID:          "custom_id",
		Name:        "测试规则",
		Pattern:     "测试",
		Replacement: "替换",
		Description: "这是一个测试规则",
		Enabled:     true,
	}

	err := manager.AddRule(rule)
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	foundRule, err := manager.GetRule("custom_id")
	if err != nil {
		t.Fatalf("Failed to get rule: %v", err)
	}

	if foundRule.ID != "custom_id" {
		t.Errorf("Expected rule ID 'custom_id', got '%s'", foundRule.ID)
	}
}

func TestGetRule_Existing(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rule, err := manager.GetRule("1")
	if err != nil {
		t.Fatalf("Failed to get rule: %v", err)
	}

	if rule.ID != "1" {
		t.Errorf("Expected rule ID '1', got '%s'", rule.ID)
	}
}

func TestGetRule_NonExisting(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	_, err := manager.GetRule("non_existing")
	if err == nil {
		t.Error("Expected error for non-existing rule")
	}
}

func TestUpdateRule(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rule := Rule{
		ID:          "1",
		Name:        "更新后的规则",
		Pattern:     "新图案",
		Replacement: "新替换",
		Description: "更新后的描述",
		Enabled:     false,
	}

	err := manager.UpdateRule(rule)
	if err != nil {
		t.Fatalf("Failed to update rule: %v", err)
	}

	updatedRule, err := manager.GetRule("1")
	if err != nil {
		t.Fatalf("Failed to get updated rule: %v", err)
	}

	if updatedRule.Name != "更新后的规则" {
		t.Errorf("Expected updated name, got '%s'", updatedRule.Name)
	}
}

func TestUpdateRule_NonExisting(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rule := Rule{
		ID:          "non_existing",
		Name:        "测试",
		Pattern:     "测试",
		Replacement: "测试",
		Enabled:     true,
	}

	err := manager.UpdateRule(rule)
	if err == nil {
		t.Error("Expected error for updating non-existing rule")
	}
}

func TestDeleteRule(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	initialCount := len(manager.GetRules())

	err := manager.DeleteRule("1")
	if err != nil {
		t.Fatalf("Failed to delete rule: %v", err)
	}

	rules := manager.GetRules()
	if len(rules) != initialCount-1 {
		t.Errorf("Expected %d rules after deletion, got %d", initialCount-1, len(rules))
	}
}

func TestDeleteRule_NonExisting(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	err := manager.DeleteRule("non_existing")
	if err == nil {
		t.Error("Expected error for deleting non-existing rule")
	}
}

func TestApplyRules_EnabledRule(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	content := "本文由某网站提供"
	result := manager.ApplyRules(content)

	if result == content {
		t.Error("Expected content to be modified by enabled rule")
	}
}

func TestApplyRules_DisabledRule(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rule := Rule{
		ID:          "disabled",
		Name:        "禁用规则",
		Pattern:     "测试",
		Replacement: "替换",
		Enabled:     false,
	}
	manager.AddRule(rule)

	content := "测试内容"
	result := manager.ApplyRules(content)

	if result != content {
		t.Errorf("Expected content unchanged for disabled rule, got '%s'", result)
	}
}

func TestApplyRules_NoMatch(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	content := "这是一段正常的文本，没有匹配任何规则的内容。"
	result := manager.ApplyRules(content)

	if result != content {
		t.Errorf("Expected content unchanged, got '%s'", result)
	}
}

func TestApplyRules_MultipleRules(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	content := "本文由某网站提供，名子写错了。"
	result := manager.ApplyRules(content)

	if result == content {
		t.Error("Expected content to be modified by multiple rules")
	}
}

func TestSaveRules(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	err := manager.SaveRules()
	if err != nil {
		t.Fatalf("Failed to save rules: %v", err)
	}

	rulesPath := filepath.Join(config.AppConfigInstance.DataDir, "rules.json")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		t.Error("Expected rules file to be created")
	}
}

func TestLoadRules_FromFile(t *testing.T) {
	manager, cleanup := setupRulesTest(t)
	defer cleanup()

	rulesPath := filepath.Join(config.AppConfigInstance.DataDir, "rules.json")
	rulesData := `[{"id":"custom","name":"自定义","pattern":"测试","replacement":"替换","description":"测试","enabled":true}]`

	err := os.WriteFile(rulesPath, []byte(rulesData), 0644)
	if err != nil {
		t.Fatalf("Failed to write rules file: %v", err)
	}

	err = manager.LoadRules()
	if err != nil {
		t.Fatalf("Failed to load rules from file: %v", err)
	}

	rules := manager.GetRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule from file, got %d", len(rules))
	}
}
