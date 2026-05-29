package handlers

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"

	"voidtext/internal/database"
	"voidtext/internal/processor"
)

// processSemaphore 并发控制：最多同时处理 4 个文件
var processSemaphore = make(chan struct{}, 4)

// fileProcessingMu 保护 processingFiles 映射，防止同一文件竞态
var fileProcessingMu sync.Mutex
var processingFiles = make(map[string]bool)

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

	// 检查是否真有 goroutine 在处理：数据库状态为 processing 但内存中无对应任务
	// 说明是服务器重启后的残留状态，允许重新处理
	fileProcessingMu.Lock()
	actuallyProcessing := processingFiles[fileMd5]
	fileProcessingMu.Unlock()

	if record.Status == "reviewing" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "文件正在审核中，请使用审核页面操作"})
		return
	}
	if record.Status == "processing" && actuallyProcessing {
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

	// 竞态保护：加锁确保状态检查和更新的原子性
	fileProcessingMu.Lock()
	if processingFiles[fileMd5] {
		fileProcessingMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "文件正在处理中"})
		return
	}
	processingFiles[fileMd5] = true
	fileProcessingMu.Unlock()

	// 并发控制：限制同时处理的文件数
	select {
	case processSemaphore <- struct{}{}:
		// 进入运行前清零取消标志，避免上一次残留的 cancel_flag 立刻终止本次执行
		database.SetCancelFlag(fileMd5, 0)
		database.UpdateFileStatus(fileMd5, "processing", startStep, processor.CalculateProgress(startStep, 0), "")

		go func() {
			defer func() { <-processSemaphore }()
			defer func() {
				fileProcessingMu.Lock()
				delete(processingFiles, fileMd5)
				fileProcessingMu.Unlock()
			}()
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
				// 步内若已经被取消（如 LLM 检查点内部更新了 status），不要继续推进
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

	default:
		fileProcessingMu.Lock()
		delete(processingFiles, fileMd5)
		fileProcessingMu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "message": "服务器繁忙，请稍后重试"})
		return
	}
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

	// 返回最近 50 条处理日志，供前端展示
	logs, _ := database.GetRecentProcessingLogs(fileMd5, 50)
	if logs != nil {
		response["logs"] = logs
	}

	if record.Status == "reviewing" || record.Status == "processing" || record.CurrentStep == "review" {
		total, resolved, _ := database.GetReviewParagraphProgress(fileMd5)
		response["reviewTotal"] = total
		response["reviewResolved"] = resolved
	}

	c.JSON(http.StatusOK, response)
}

// GetReviewItems 获取段级审核记录与基线全文
func GetReviewItems(c *gin.Context) {
	fileMd5 := c.Param("md5")
	statusFilter := c.Query("status")

	record, err := database.GetFileByMd5(fileMd5)
	if err != nil || record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 审核阶段使用审核基线作为全文展示底
	filePath := record.FilePath
	if record.Status == "reviewing" && record.ReviewBaselinePath != "" {
		filePath = record.ReviewBaselinePath
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件失败"})
		return
	}

	records, err := database.GetReviewParagraphsByFileMd5(fileMd5, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询段级审核记录失败"})
		return
	}

	paragraphs := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		paragraphs = append(paragraphs, map[string]interface{}{
			"id":             r.ID,
			"paragraphIndex": r.ParagraphIndex,
			"original":       r.OriginalText,
			"suggested":      r.SuggestedText,
			"type":           r.ModificationType,
			"status":         r.Status,
			"editedText":     r.EditedText,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"baselineContent": string(content),
		"paragraphs":      paragraphs,
	})
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

	if err := database.UpdateReviewParagraphStatus(req.ItemId, "approved", ""); err != nil {
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

	if err := database.UpdateReviewParagraphStatus(req.ItemId, "rejected", ""); err != nil {
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

	if err := database.UpdateReviewParagraphStatus(req.ItemId, "edited", req.EditedText); err != nil {
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

	if err := database.UpdateReviewParagraphStatus(req.ItemId, "pending", ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "恢复失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "建议已恢复为待审核"})
}

// BatchApproveReviewItems 批量批准当前文件所有 pending 段
func BatchApproveReviewItems(c *gin.Context) {
	fileMd5 := c.Param("md5")

	if err := database.BatchUpdateReviewParagraphStatus(fileMd5, "approved"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "批量批准失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "已批准全部待审段落"})
}

// BatchRejectReviewItems 批量拒绝当前文件所有 pending 段
func BatchRejectReviewItems(c *gin.Context) {
	fileMd5 := c.Param("md5")

	if err := database.BatchUpdateReviewParagraphStatus(fileMd5, "rejected"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "批量拒绝失败"})
		return
	}

	updateReviewProgress(fileMd5)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "已拒绝全部待审段落"})
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

	total, resolved, _ := database.GetReviewParagraphProgress(fileMd5)

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
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(buildReportHTML(report)))
		return
	}

	c.JSON(http.StatusOK, report)
}

// updateReviewProgress 更新审核进度
func updateReviewProgress(fileMd5 string) {
	total, resolved, _ := database.GetReviewParagraphProgress(fileMd5)
	if total > 0 {
		stepProgress := resolved * 100 / total
		progress := processor.CalculateProgress(processor.StepReview, stepProgress)
		database.UpdateFileStatus(fileMd5, "reviewing", processor.StepReview, progress, "")
	}
}

// buildReportHTML 构建HTML格式报告
func buildReportHTML(data gin.H) string {
	file := data["file"].(gin.H)
	review := data["review"].(gin.H)

	// 对用户输入进行 HTML 转义，防止 XSS 攻击
	title := html.EscapeString(fmt.Sprintf("%v", file["title"]))
	author := html.EscapeString(fmt.Sprintf("%v", file["author"]))
	fileName := html.EscapeString(fmt.Sprintf("%v", file["fileName"]))
	status := html.EscapeString(fmt.Sprintf("%v", file["status"]))

	reportHTML := fmt.Sprintf(`<!DOCTYPE html>
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
		title, title, author, fileName,
		status, file["progress"], review["total"], review["resolved"])

	return reportHTML
}
