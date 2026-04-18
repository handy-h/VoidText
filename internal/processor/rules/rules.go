package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"

	"txt-cleaning/internal/config"
)

// Rule 自定义规则
type Rule struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Pattern      string `json:"pattern"`
	Replacement  string `json:"replacement"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
}

// RuleManager 规则管理器
type RuleManager struct {
	rules []Rule
}

// NewRuleManager 创建新的规则管理器
func NewRuleManager() *RuleManager {
	manager := &RuleManager{}
	manager.LoadRules()
	return manager
}

// LoadRules 加载规则
func (rm *RuleManager) LoadRules() error {
	rulesPath := filepath.Join(config.AppConfig.DataDir, "rules.json")

	// 检查文件是否存在
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		// 创建默认规则
		rm.rules = []Rule{
			{
				ID:          "1",
				Name:        "广告清理",
				Pattern:     "本文由.*提供",
				Replacement: "",
				Description: "清理常见的广告文本",
				Enabled:     true,
			},
			{
				ID:          "2",
				Name:        "错别字修正",
				Pattern:     "名子",
				Replacement: "名字",
				Description: "修正常见错别字",
				Enabled:     true,
			},
		}
		rm.SaveRules()
		return nil
	}

	// 读取规则文件
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return err
	}

	// 反序列化
	return json.Unmarshal(data, &rm.rules)
}

// SaveRules 保存规则
func (rm *RuleManager) SaveRules() error {
	rulesPath := filepath.Join(config.AppConfig.DataDir, "rules.json")

	// 序列化
	data, err := json.MarshalIndent(rm.rules, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(rulesPath, data, 0644)
}

// GetRules 获取所有规则
func (rm *RuleManager) GetRules() []Rule {
	return rm.rules
}

// GetRule 获取特定规则
func (rm *RuleManager) GetRule(id string) (Rule, error) {
	for _, rule := range rm.rules {
		if rule.ID == id {
			return rule, nil
		}
	}
	return Rule{}, os.ErrNotExist
}

// AddRule 添加规则
func (rm *RuleManager) AddRule(rule Rule) error {
	// 生成ID
	if rule.ID == "" {
		rule.ID = string(len(rm.rules) + 1)
	}

	// 添加规则
	rm.rules = append(rm.rules, rule)

	// 保存规则
	return rm.SaveRules()
}

// UpdateRule 更新规则
func (rm *RuleManager) UpdateRule(rule Rule) error {
	for i, r := range rm.rules {
		if r.ID == rule.ID {
			rm.rules[i] = rule
			return rm.SaveRules()
		}
	}
	return os.ErrNotExist
}

// DeleteRule 删除规则
func (rm *RuleManager) DeleteRule(id string) error {
	for i, rule := range rm.rules {
		if rule.ID == id {
			rm.rules = append(rm.rules[:i], rm.rules[i+1:]...)
			return rm.SaveRules()
		}
	}
	return os.ErrNotExist
}

// ApplyRules 应用规则到文本
func (rm *RuleManager) ApplyRules(content string) string {
	for _, rule := range rm.rules {
		if rule.Enabled {
			regex, err := regexp.Compile(rule.Pattern)
			if err == nil {
				content = regex.ReplaceAllString(content, rule.Replacement)
			}
		}
	}
	return content
}