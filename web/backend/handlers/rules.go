package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/processor"
	"txt-cleaning/internal/processor/rules"
)

// ListRules 列出所有自定义规则
func ListRules(c *gin.Context) {
	// 获取规则列表
	ruleMgr := processor.GetRuleManager()
	rulesList := ruleMgr.GetRules()

	c.JSON(http.StatusOK, gin.H{"success": true, "rules": rulesList})
}

// AddRule 添加自定义规则
func AddRule(c *gin.Context) {
	// 获取规则参数
	var req struct {
		Name         string `json:"name"`
		Pattern      string `json:"pattern"`
		Replacement  string `json:"replacement"`
		Description  string `json:"description"`
		Enabled      bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	// 创建规则
	rule := rules.Rule{
		Name:         req.Name,
		Pattern:      req.Pattern,
		Replacement:  req.Replacement,
		Description:  req.Description,
		Enabled:      req.Enabled,
	}

	// 添加规则
	ruleMgr := processor.GetRuleManager()
	if err := ruleMgr.AddRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "添加规则失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "规则添加成功"})
}

// DeleteRule 删除自定义规则
func DeleteRule(c *gin.Context) {
	ruleId := c.Param("id")

	// 删除规则
	ruleMgr := processor.GetRuleManager()
	if err := ruleMgr.DeleteRule(ruleId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除规则失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "规则删除成功"})
}

// GetExternalConfig 获取外部API配置
func GetExternalConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config": gin.H{
			"vectorUrl":    config.AppConfigInstance.VectorModelURL,
			"vectorKey":    config.AppConfigInstance.VectorModelApiKey,
			"llmUrl":       config.AppConfigInstance.LLMApiURL,
			"llmKey":       config.AppConfigInstance.LLMApiKey,
		},
	})
}

// UpdateExternalConfig 更新外部API配置
func UpdateExternalConfig(c *gin.Context) {
	var req struct {
		VectorURL string `json:"vectorUrl"`
		VectorKey string `json:"vectorKey"`
		LLMURL    string `json:"llmUrl"`
		LLMKey    string `json:"llmKey"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	config.AppConfigInstance.VectorModelURL = req.VectorURL
	config.AppConfigInstance.VectorModelApiKey = req.VectorKey
	config.AppConfigInstance.LLMApiURL = req.LLMURL
	config.AppConfigInstance.LLMApiKey = req.LLMKey

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置保存成功"})
}