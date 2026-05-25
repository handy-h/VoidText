package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"voidtext/internal/database"
	"voidtext/internal/processor"
)

// RunAllSteps 异步执行所有步骤直到审核或完成
func RunAllSteps(c *gin.Context) {
	fileMd5 := c.Param("md5")

	record, err := database.GetFileByMd5(fileMd5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件记录失败"})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	if record.Status == "processing" || record.Status == "reviewing" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "文件正在处理中"})
		return
	}

	// 当前步骤已进入最终化阶段：应通过 /finalize 推进，避免落入错误的 review 分支
	if record.CurrentStep == processor.StepFinalizing {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "文件已进入最终化阶段，请调用 /finalize 接口完成处理",
		})
		return
	}

	startStep := processor.StepCleaning
	// review 不能作为起步状态：进入审核后应通过 /finalize 推进，重新点击运行时回退到 cleaning
	if record.CurrentStep != "" && record.CurrentStep != processor.StepReview {
		startStep = record.CurrentStep
	}

	stepsToRun := []string{}
	found := false
	for _, step := range []string{processor.StepCleaning, processor.StepIndexing, processor.StepLlmFix} {
		if step == startStep {
			found = true
		}
		if found {
			stepsToRun = append(stepsToRun, step)
		}
	}

	// 进入运行前清零取消标志，避免上一次残留的 cancel_flag 立刻终止本次执行
	database.SetCancelFlag(fileMd5, 0)
	database.UpdateFileStatus(fileMd5, "processing", startStep, processor.CalculateProgress(startStep, 0), "")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				database.UpdateFileStatus(fileMd5, "failed", "", 0, fmt.Sprintf("处理异常: %v", r))
			}
		}()

		for _, step := range stepsToRun {
			// 步前检查取消标志（针对 cleaning/indexing 这类无内部检查点的步骤）
			cancelled, _ := database.IsFileCancelled(fileMd5)
			if cancelled {
				database.SetCancelFlag(fileMd5, 0)
				database.UpdateFileStatus(fileMd5, "cancelled", step, 0, "用户取消")
				return
			}
			if _, err := processor.ProcessStep(fileMd5, step); err != nil {
				database.UpdateFileStatus(fileMd5, "failed", step, 0, err.Error())
				return
			}
			// 步内若已被取消（如 LLM 检查点内部更新了 status），不要继续推进
			cur, _ := database.GetFileByMd5(fileMd5)
			if cur != nil && cur.Status == "cancelled" {
				return
			}
		}

		// 进入审核阶段；若该步骤判定无需审核（total==0），它会把 nextStep 推到 finalizing，
		// 这里必须主动完成 finalizing，否则前端无法自动收尾
		reviewResult, err := processor.ProcessStep(fileMd5, processor.StepReview)
		if err != nil {
			database.UpdateFileStatus(fileMd5, "failed", processor.StepReview, 0, err.Error())
			return
		}
		if reviewResult != nil && reviewResult.NextStep == processor.StepFinalizing {
			if _, ferr := processor.ProcessStep(fileMd5, processor.StepFinalizing); ferr != nil {
				database.UpdateFileStatus(fileMd5, "failed", processor.StepFinalizing, 0, ferr.Error())
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "处理已启动",
	})
}

// CancelProcessing 取消处理中的任务
func CancelProcessing(c *gin.Context) {
	fileMd5 := c.Param("md5")

	record, err := database.GetFileByMd5(fileMd5)
	if err != nil || record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	if record.Status != "processing" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "文件未在处理中"})
		return
	}

	if err := database.SetCancelFlag(fileMd5, 1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "设置取消标志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "取消信号已发送，将在下一检查点停止"})
}

// GetFileStatus 获取文件处理状态
func GetFileStatus(c *gin.Context) {
	fileMd5 := c.Param("md5")

	record, err := database.GetFileByMd5(fileMd5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件记录失败"})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	response := gin.H{
		"success":     true,
		"md5":         record.Md5,
		"status":      record.Status,
		"currentStep": record.CurrentStep,
		"progress":    record.Progress,
		"errorMsg":    record.ErrorMsg,
		"author":      record.Author,
		"title":       record.Title,
		"fileName":    record.FileName,
	}

	if progressInfo, ok := processor.GlobalProgressTracker.GetProgress(fileMd5); ok {
		response["chunkProgress"] = progressInfo
	} else {
		response["chunkProgress"] = gin.H{
			"totalChunks":            0,
			"processedChunks":        0,
			"remainingChunks":        0,
			"cacheHits":              0,
			"apiCalls":               0,
			"avgChunkTimeMs":         0,
			"estimatedRemainingSecs": 0,
			"progress":               0,
			"elapsedSeconds":         0,
		}
	}

	if record.Status == "processing" && record.ErrorMsg != "" {
		response["message"] = record.ErrorMsg
	}

	latestLog, _ := database.GetLatestProcessingLog(fileMd5)
	if latestLog != nil {
		response["currentAction"] = latestLog.Details
	}

	if record.Status == "reviewing" || record.Status == "processing" || record.CurrentStep == "review" {
		total, resolved, _ := database.GetReviewProgress(fileMd5)
		response["reviewTotal"] = total
		response["reviewResolved"] = resolved
	}

	c.JSON(http.StatusOK, response)
}

// GetReviewItems 获取审核项
func GetReviewItems(c *gin.Context) {
	fileMd5 := c.Param("md5")
	statusFilter := c.Query("status")

	record, err := database.GetFileByMd5(fileMd5)
	if err != nil || record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 审核阶段使用审核基线的文件内容来解析行号上下文
	filePath := record.FilePath
	if record.Status == "reviewing" && record.ReviewBaselinePath != "" {
		filePath = record.ReviewBaselinePath
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件失败"})
		return
	}

	items, err := database.GetReviewItemsByFileMd5(fileMd5, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询审核项失败"})
		return
	}

	// 全文按行切分一次，循环复用，避免每个审核项都重复 Split
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	suggestions := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		lineNum, fullLine, prevLine, nextLine := getLineContext(contentStr, lines, item.PositionStart, item.OriginalText)

		suggestions = append(suggestions, map[string]interface{}{
			"id":         item.ID,
			"type":       item.ModificationType,
			"original":   item.OriginalText,
			"suggested":  item.SuggestedText,
			"position":   item.PositionStart,
			"status":     item.Status,
			"confidence": item.Confidence,
			"editedText": item.EditedText,
			"lineNum":    lineNum,
			"fullLine":   fullLine,
			"prevLine":   prevLine,
			"nextLine":   nextLine,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "suggestions": suggestions})
}

// ApproveReviewItem 批准审核项
func ApproveReviewItem(c *gin.Context) {
	fileMd5 := c.Param("md5")

	var req struct {
		ItemId int64 `json:"itemId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	if err := database.UpdateReviewItemStatus(req.ItemId, "approved", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新审核状态失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "建议已批准"})
}

// RejectReviewItem 拒绝审核项
func RejectReviewItem(c *gin.Context) {
	fileMd5 := c.Param("md5")

	var req struct {
		ItemId int64 `json:"itemId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	if err := database.UpdateReviewItemStatus(req.ItemId, "rejected", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新审核状态失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "建议已拒绝"})
}

// EditReviewItem 手动编辑审核项
func EditReviewItem(c *gin.Context) {
	fileMd5 := c.Param("md5")

	var req struct {
		ItemId     int64  `json:"itemId"`
		EditedText string `json:"editedText"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	if err := database.UpdateReviewItemStatus(req.ItemId, "edited", req.EditedText); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新审核状态失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "建议已编辑"})
}

// RestoreReviewItem 恢复审核项为待审核
func RestoreReviewItem(c *gin.Context) {
	fileMd5 := c.Param("md5")

	var req struct {
		ItemId int64 `json:"itemId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	if err := database.UpdateReviewItemStatus(req.ItemId, "pending", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "恢复失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "建议已恢复为待审核"})
}

// BatchApproveReviewItems 批量批准审核项
func BatchApproveReviewItems(c *gin.Context) {
	fileMd5 := c.Param("md5")

	var req struct {
		ItemIds []int64 `json:"itemIds"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	if err := database.BatchUpdateReviewItemStatus(req.ItemIds, "approved"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "批量批准失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("成功批准 %d 条建议", len(req.ItemIds))})
}

// BatchRejectReviewItems 批量拒绝审核项
func BatchRejectReviewItems(c *gin.Context) {
	fileMd5 := c.Param("md5")

	var req struct {
		ItemIds []int64 `json:"itemIds"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	if err := database.BatchUpdateReviewItemStatus(req.ItemIds, "rejected"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "批量拒绝失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("成功拒绝 %d 条建议", len(req.ItemIds))})
}

// FinalizeFile 完成审核并生成最终文件
func FinalizeFile(c *gin.Context) {
	fileMd5 := c.Param("md5")

	complete, err := processor.CheckReviewComplete(fileMd5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "检查审核状态失败"})
		return
	}
	if !complete {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "还有未审核的项目"})
		return
	}

	result, err := processor.AdvanceFromReview(fileMd5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "生成最终文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": result.Message,
	})
}

// GetProcessingReport 获取处理报告
func GetProcessingReport(c *gin.Context) {
	fileMd5 := c.Param("md5")

	record, err := database.GetFileByMd5(fileMd5)
	if err != nil || record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	logs, err := database.GetProcessingLogsByFileMd5(fileMd5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询处理日志失败"})
		return
	}

	total, resolved, _ := database.GetReviewProgress(fileMd5)

	versions, _ := database.GetVersionsByOriginalMd5(fileMd5)

	report := gin.H{
		"success": true,
		"file": gin.H{
			"md5":         record.Md5,
			"author":      record.Author,
			"title":       record.Title,
			"fileName":    record.FileName,
			"fileSize":    record.FileSize,
			"status":      record.Status,
			"currentStep": record.CurrentStep,
			"progress":    record.Progress,
			"createdAt":   record.CreatedAt,
			"updatedAt":   record.UpdatedAt,
		},
		"review": gin.H{
			"total":    total,
			"resolved": resolved,
		},
		"logs":     logs,
		"versions": versions,
	}

	format := c.Query("format")
	if format == "html" {
		c.HTML(http.StatusOK, "", buildReportHTML(report))
		return
	}

	c.JSON(http.StatusOK, report)
}

// updateReviewProgress 更新审核进度
func updateReviewProgress(fileMd5 string) {
	total, resolved, _ := database.GetReviewProgress(fileMd5)
	if total > 0 {
		stepProgress := resolved * 100 / total
		progress := processor.CalculateProgress(processor.StepReview, stepProgress)
		database.UpdateFileStatus(fileMd5, "reviewing", processor.StepReview, progress, "")
	}
}

// getLineContext 获取指定位置的行号和上下文
// getLineContext 获取指定位置的行号和上下文
// lines 由调用方预先切分并复用，避免在大文件场景下每个审核项都重复 strings.Split
func getLineContext(content string, lines []string, position int, original string) (lineNum int, fullLine, prevLine, nextLine string) {
	if position >= 0 && position <= len(content) {
		currentPos := 0
		for i, line := range lines {
			lineEnd := currentPos + len(line)
			if currentPos <= position && position <= lineEnd {
				lineNum = i + 1
				fullLine = line
				if i > 0 {
					prevLine = lines[i-1]
				}
				if i < len(lines)-1 {
					nextLine = lines[i+1]
				}
				return lineNum, fullLine, prevLine, nextLine
			}
			currentPos = lineEnd + 1
		}
	}

	if original != "" {
		for i, line := range lines {
			if strings.Contains(line, original) {
				lineNum = i + 1
				fullLine = line
				if i > 0 {
					prevLine = lines[i-1]
				}
				if i < len(lines)-1 {
					nextLine = lines[i+1]
				}
				return lineNum, fullLine, prevLine, nextLine
			}
		}

		trimmedOriginal := strings.TrimSpace(original)
		for i, line := range lines {
			if strings.Contains(strings.TrimSpace(line), trimmedOriginal) {
				lineNum = i + 1
				fullLine = line
				if i > 0 {
					prevLine = lines[i-1]
				}
				if i < len(lines)-1 {
					nextLine = lines[i+1]
				}
				return lineNum, fullLine, prevLine, nextLine
			}
		}

		if len(original) > 10 {
			substr := original[:len(original)/2]
			for i, line := range lines {
				if strings.Contains(line, substr) {
					lineNum = i + 1
					fullLine = line
					if i > 0 {
						prevLine = lines[i-1]
					}
					if i < len(lines)-1 {
						nextLine = lines[i+1]
					}
					return lineNum, fullLine, prevLine, nextLine
				}
			}
		}
	}

	if position > 0 {
		estimatedLine := 0
		currentPos := 0
		for i, line := range lines {
			currentPos += len(line) + 1
			if currentPos >= position {
				estimatedLine = i
				break
			}
		}
		if estimatedLine < len(lines) {
			lineNum = estimatedLine + 1
			fullLine = lines[estimatedLine]
			if estimatedLine > 0 {
				prevLine = lines[estimatedLine-1]
			}
			if estimatedLine < len(lines)-1 {
				nextLine = lines[estimatedLine+1]
			}
		}
	}

	return lineNum, fullLine, prevLine, nextLine
}

// buildReportHTML 构建HTML格式报告
func buildReportHTML(data gin.H) string {
	file := data["file"].(gin.H)
	review := data["review"].(gin.H)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>处理报告 - %s</title>
<style>body{font-family:sans-serif;margin:20px}table{border-collapse:collapse;width:100%%}td,th{border:1px solid #ddd;padding:8px;text-align:left}</style>
</head><body>
<h1>处理报告</h1>
<h2>文件信息</h2>
<table><tr><th>属性</th><th>值</th></tr>
<tr><td>小说名称</td><td>%s</td></tr>
<tr><td>作者</td><td>%s</td></tr>
<tr><td>文件名</td><td>%s</td></tr>
<tr><td>状态</td><td>%s</td></tr>
<tr><td>进度</td><td>%d%%</td></tr>
</table>
<h2>审核统计</h2>
<p>总条目: %d, 已处理: %d</p>
</body></html>`,
		file["title"], file["title"], file["author"], file["fileName"],
		file["status"], file["progress"], review["total"], review["resolved"])

	return html
}
