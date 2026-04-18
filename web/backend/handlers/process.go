package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/processor"
	"txt-cleaning/internal/processor/preprocess"
	"txt-cleaning/internal/review/manager"
)

// 处理状态
var processStatus = make(map[string]map[string]interface{})

// StartProcessing 开始处理文件
func StartProcessing(c *gin.Context) {
	// 获取文件ID
	var req struct {
		FileId string `json:"fileId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	// 检查文件是否存在
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", req.FileId)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 生成处理ID
	processId := fmt.Sprintf("%d_%s", time.Now().UnixNano(), req.FileId)

	// 初始化处理状态
	processStatus[processId] = map[string]interface{}{
		"fileId":   req.FileId,
		"status":   "processing",
		"progress": 0,
		"startAt":  time.Now().Format(time.RFC3339),
	}

	// 异步处理文件
	go func() {
		// 读取文件内容
		content, err := os.ReadFile(filePath)
		if err != nil {
			processStatus[processId]["status"] = "failed"
			processStatus[processId]["message"] = "读取文件失败: " + err.Error()
			processStatus[processId]["endAt"] = time.Now().Format(time.RFC3339)
			return
		}

		// 更新进度
		processStatus[processId]["progress"] = 30

		// 处理文本
		result, err := processor.ProcessWithReview(string(content), req.FileId, processId)
		if err != nil {
			processStatus[processId]["status"] = "failed"
			processStatus[processId]["message"] = "处理文本失败: " + err.Error()
			processStatus[processId]["endAt"] = time.Now().Format(time.RFC3339)
			return
		}

		// 更新进度
		processStatus[processId]["progress"] = 100

		// 完成处理
		processStatus[processId]["status"] = "completed"
		processStatus[processId]["endAt"] = time.Now().Format(time.RFC3339)
		processStatus[processId]["reviewSessionId"] = result.ReviewSessionID
		processStatus[processId]["suggestionsCount"] = len(result.Suggestions)
	}()

	c.JSON(http.StatusOK, gin.H{"success": true, "processId": processId})
}

// GetProcessStatus 获取处理状态
func GetProcessStatus(c *gin.Context) {
	processId := c.Param("id")

	// 检查处理是否存在
	status, exists := processStatus[processId]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "处理不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"status":   status["status"],
		"progress": status["progress"],
		"startAt":  status["startAt"],
		"endAt":    status["endAt"],
		"message":  status["message"],
	})
}

// GetSuggestions 获取修改建议
func GetSuggestions(c *gin.Context) {
	processId := c.Param("id")

	// 检查处理是否存在
	status, exists := processStatus[processId]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "处理不存在"})
		return
	}

	// 检查处理是否完成
	if status["status"] != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "处理尚未完成"})
		return
	}

	// 获取审核会话ID
	reviewSessionId, ok := status["reviewSessionId"].(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "审核会话不存在"})
		return
	}

	// 获取审核会话
	reviewMgr := processor.GetReviewManager()
	session, err := reviewMgr.GetSession(reviewSessionId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "获取审核会话失败: " + err.Error()})
		return
	}

	// 构建建议列表
	suggestions := make([]map[string]interface{}, len(session.Items))
	for i, item := range session.Items {
		suggestions[i] = map[string]interface{}{
			"id":        item.ID,
			"type":      item.Suggestion.Type,
			"original":  item.Suggestion.Original,
			"suggested": item.Suggestion.Replacement,
			"position":  item.Suggestion.Position,
			"status":    item.Status,
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "suggestions": suggestions})
}

// ApproveSuggestion 批准修改建议
func ApproveSuggestion(c *gin.Context) {
	processId := c.Param("id")

	// 获取建议ID
	var req struct {
		SuggestionId string `json:"suggestionId"`
		Note         string `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	// 检查处理是否存在
	status, exists := processStatus[processId]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "处理不存在"})
		return
	}

	// 获取审核会话ID
	reviewSessionId, ok := status["reviewSessionId"].(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "审核会话不存在"})
		return
	}

	// 更新审核状态
	reviewMgr := processor.GetReviewManager()
	err := reviewMgr.UpdateItemStatus(reviewSessionId, req.SuggestionId, manager.StatusApproved, req.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新审核状态失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "建议已批准"})
}

// RejectSuggestion 拒绝修改建议
func RejectSuggestion(c *gin.Context) {
	processId := c.Param("id")

	// 获取建议ID
	var req struct {
		SuggestionId string `json:"suggestionId"`
		Note         string `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	// 检查处理是否存在
	status, exists := processStatus[processId]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "处理不存在"})
		return
	}

	// 获取审核会话ID
	reviewSessionId, ok := status["reviewSessionId"].(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "审核会话不存在"})
		return
	}

	// 更新审核状态
	reviewMgr := processor.GetReviewManager()
	err := reviewMgr.UpdateItemStatus(reviewSessionId, req.SuggestionId, manager.StatusRejected, req.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新审核状态失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "建议已拒绝"})
}

// SaveProgress 保存审核进度
func SaveProgress(c *gin.Context) {
	processId := c.Param("id")

	// 获取保存参数
	var req struct {
		Note string `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	// 检查处理是否存在
	status, exists := processStatus[processId]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "处理不存在"})
		return
	}

	// 获取审核会话ID
	reviewSessionId, ok := status["reviewSessionId"].(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "审核会话不存在"})
		return
	}

	// 保存进度
	reviewMgr := processor.GetReviewManager()
	err := reviewMgr.SaveProgress(reviewSessionId, req.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存进度失败: " + err.Error()})
		return
	}

	// 获取审核会话中已批准的建议
	session, err := reviewMgr.GetSession(reviewSessionId)
	if err == nil && session != nil {
		approvedSuggestions := []preprocess.Change{}
		for _, item := range session.Items {
			if item.Status == manager.StatusApproved {
				approvedSuggestions = append(approvedSuggestions, item.Suggestion)
			}
		}

		if len(approvedSuggestions) > 0 {
			fileId := status["fileId"].(string)
			filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)

			content, err := os.ReadFile(filePath)
			if err == nil {
				updatedContent := processor.ApplyAllSuggestions(string(content), approvedSuggestions)
				os.WriteFile(filePath, []byte(updatedContent), 0644)
			}
		}
	}

	// 创建版本备份
	fileId := status["fileId"].(string)
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)
	content, err := os.ReadFile(filePath)
	if err == nil {
		versionManager.CreateVersion(fileId, string(content), "保存审核进度")
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "进度保存成功"})
}
