package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"voidtext/internal/config"
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

	startStep := processor.StepCleaning
	if record.CurrentStep != "" {
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

	// 使用事务更新状态
	if err := processor.UpdateFileStatusWithStepProgress(fileMd5, "processing", startStep, processor.CalculateProgress(startStep, 0)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新文件状态失败: " + err.Error(),
		})
		return
	}

	// 使用工作池提交处理任务
	err = processor.SubmitFileProcessing(fileMd5, stepsToRun, func(processingErr error) {
		if processingErr != nil {
			processor.FailProcessingStep(fileMd5, startStep, processingErr)
		}
		// 注意：这里不更新状态为完成，因为ProcessStep内部会更新状态
	})

	if err != nil {
		processor.FailProcessingStep(fileMd5, startStep, fmt.Errorf("提交处理任务失败: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "提交处理任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "处理已启动",
	})
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

	if record.Status == "processing" && record.ErrorMsg != "" {
		response["message"] = record.ErrorMsg
	}

	latestLog, _ := database.GetLatestProcessingLog(fileMd5)
	if latestLog != nil {
		response["currentAction"] = formatLogDetails(latestLog)
	}

	if record.Status == "reviewing" || record.Status == "processing" || record.CurrentStep == "review" {
		total, resolved, _ := database.GetReviewProgress(fileMd5)
		response["reviewTotal"] = total
		response["reviewResolved"] = resolved
	}

	recentLogs, _ := database.GetRecentProcessingLogs(fileMd5, 20)
	response["logs"] = recentLogs

	// 添加LLM修复进度信息
	if record.CurrentStep == "llm_fix" && record.Status == "processing" {
		if progressInfo, exists := processor.GlobalProgressTracker.GetProgress(fileMd5); exists {
			response["chunkProgress"] = progressInfo
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetReviewItems 获取审核项
// review_items 中的 PositionStart 是相对于创建时的文件版本，
// 但流水线每个步骤都会修改文件内容（删除重复、LLM替换等）并更新 record.FilePath。
// 因此本函数读取多个中间文件版本，按新旧顺序逐一尝试定位每个审核项。
func GetReviewItems(c *gin.Context) {
	fileMd5 := c.Param("md5")
	statusFilter := c.Query("status")

	record, err := database.GetFileByMd5(fileMd5)
	if err != nil || record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	contents := readAvailableContents(fileMd5, record.FilePath)
	if len(contents) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件失败"})
		return
	}

	items, err := database.GetReviewItemsByFileMd5(fileMd5, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询审核项失败"})
		return
	}

	suggestions := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		// 跳过无修改建议的非删除类项：点通过和点拒绝结果一样（都保留原文），展示无意义
		if item.SuggestedText == "" && item.ModificationType != "text_deletion" &&
			item.ModificationType != "advertisement" && item.ModificationType != "duplicate_paragraph" {
			continue
		}

		lineNum, fullLine, prevLines, nextLines := getLineContextMulti(contents, item.PositionStart, item.OriginalText, item.SuggestedText)

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
			"prevLines":  prevLines,
			"nextLines":  nextLines,
			// 向后兼容：单行上下文
			"prevLine": getLastOrEmpty(prevLines),
			"nextLine": getFirstOrEmpty(nextLines),
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "suggestions": suggestions})
}

// readAvailableContents 读取当前文件及所有中间版本文件的内容（从新到旧）
func readAvailableContents(fileMd5, currentPath string) []string {
	seen := map[string]bool{}
	var contents []string

	// 当前文件（最新版本）
	if currentPath != "" {
		if data, err := os.ReadFile(currentPath); err == nil {
			contents = append(contents, string(data))
			seen[currentPath] = true
		}
	}

	// 中间步骤文件（从新到旧）
	steps := []string{processor.StepLlmFix, processor.StepIndexing, processor.StepCleaning}
	basePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads")
	for _, step := range steps {
		path := filepath.Join(basePath, fileMd5+"_"+step+".txt")
		if seen[path] {
			continue
		}
		if data, err := os.ReadFile(path); err == nil {
			contents = append(contents, string(data))
			seen[path] = true
		}
	}

	return contents
}

// getLineContextMulti 在多个版本的内容中依次尝试定位审核项，使用第一个成功的结果
func getLineContextMulti(contents []string, position int, original, suggested string) (lineNum int, fullLine string, prevLines, nextLines []string) {
	for _, content := range contents {
		lineNum, fullLine, prevLines, nextLines = getLineContext(content, position, original, suggested)
		if lineNum > 0 {
			return
		}
	}
	return 0, "", nil, nil
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
		processor.UpdateFileStatusWithStepProgress(fileMd5, "reviewing", processor.StepReview, progress)
	}
}

// getLineContext 获取指定位置的行号和上下文
// contextLineCount 返回的上下文行数（每侧）
const contextLineCount = 3

// getLineContext 查找文本中指定位置附近的上下文，返回行号、完整行、前后各3行
// 如果 original 在文件中已不存在（如被LLM修复替换），会尝试搜索 suggested 文本定位
func getLineContext(content string, position int, original string, suggested string) (lineNum int, fullLine string, prevLines, nextLines []string) {
	lines := strings.Split(content, "\n")

	// 查找匹配行索引（0-based）
	matchIdx := findMatchLine(lines, content, position, original, suggested)

	if matchIdx < 0 || matchIdx >= len(lines) {
		// 最终兜底：逐行扫描任何包含原文或建议文本的行
		matchIdx = finalScan(lines, original, suggested)
	}

	if matchIdx < 0 || matchIdx >= len(lines) {
		return 0, "", nil, nil
	}

	lineNum = matchIdx + 1
	fullLine = lines[matchIdx]
	prevLines = getPrevLines(lines, matchIdx, contextLineCount)
	nextLines = getNextLines(lines, matchIdx, contextLineCount)
	return
}

// findMatchLine 在行列表中查找匹配位置所在的行的索引
// 因为流水线各步骤会修改文件内容（删除重复、LLM替换等），
// review_items 中存储的 byte 位置可能指向旧版本文件。
// 本函数通过 位置+内容验证 以及 附近搜索 来处理位置偏移。
// 对于 LLM 修复项，如果 original 已被替换，会尝试搜索 suggested 文本定位。
func findMatchLine(lines []string, content string, position int, original string, suggested string) int {
	// 辅助函数：在指定行附近 ±range 行内搜索文本
	searchNearby := func(estimatedLine int, searchText string, searchRange int) int {
		if searchText == "" {
			return -1
		}
		if estimatedLine >= 0 && estimatedLine < len(lines) &&
			strings.Contains(lines[estimatedLine], searchText) {
			return estimatedLine
		}
		for offset := 1; offset <= searchRange; offset++ {
			if estimatedLine-offset >= 0 && strings.Contains(lines[estimatedLine-offset], searchText) {
				return estimatedLine - offset
			}
			if estimatedLine+offset < len(lines) && strings.Contains(lines[estimatedLine+offset], searchText) {
				return estimatedLine + offset
			}
		}
		return -1
	}

	// 辅助函数：全局搜索文本，附带逐步缩短的兜底
	searchGlobal := func(searchText string) int {
		if searchText == "" {
			return -1
		}
		// 精确匹配整段文本
		for i, line := range lines {
			if strings.Contains(line, searchText) {
				return i
			}
		}
		// 去掉空格后再试
		trimmed := strings.TrimSpace(searchText)
		for i, line := range lines {
			if strings.Contains(strings.TrimSpace(line), trimmed) {
				return i
			}
		}
		return -1
	}

	// 逐步缩短搜索：从完整文本到短片段，逐级尝试
	searchProgressive := func(text string) int {
		if text == "" {
			return -1
		}
		// 尝试全文
		if idx := searchGlobal(text); idx >= 0 {
			return idx
		}
		// 尝试前半
		if len(text) > 30 {
			if idx := searchGlobal(text[:len(text)/2]); idx >= 0 {
				return idx
			}
		}
		// 尝试前 20 字
		if len(text) > 20 {
			runes := []rune(text)
			if len(runes) > 20 {
				if idx := searchGlobal(string(runes[:20])); idx >= 0 {
					return idx
				}
			}
		}
		// 尝试前 10 字
		if len(text) > 10 {
			runes := []rune(text)
			if len(runes) > 10 {
				if idx := searchGlobal(string(runes[:10])); idx >= 0 {
					return idx
				}
			}
		}
		return -1
	}

	// 1. 位置匹配 + 内容验证（最可靠的方式）
	if position >= 0 && position <= len(content) {
		currentPos := 0
		estimatedLine := -1
		for i, line := range lines {
			lineEnd := currentPos + len(line)
			if currentPos <= position && position <= lineEnd {
				estimatedLine = i
				break
			}
			currentPos = lineEnd + 1
		}

		if estimatedLine >= 0 {
			// 1a. 在位置附近搜索原文（处理内容轻微偏移）
			if idx := searchNearby(estimatedLine, original, 8); idx >= 0 {
				return idx
			}

			// 1b. 原文不存在（已被LLM替换），在位置附近搜索建议文本
			if idx := searchNearby(estimatedLine, suggested, 8); idx >= 0 {
				return idx
			}

			// 1c. 原文附近尝试逐步缩短搜索
			if original != "" {
				runes := []rune(original)
				for shortenLen := 30; shortenLen >= 5; shortenLen -= 5 {
					if len(runes) > shortenLen {
						shortText := string(runes[:shortenLen])
						if idx := searchNearby(estimatedLine, shortText, 8); idx >= 0 {
							return idx
						}
					}
				}
			}
		}
	}

	// 2. 全局逐步搜索原文（位置完全失效时）
	if original != "" {
		if idx := searchProgressive(original); idx >= 0 {
			return idx
		}
	}

	// 3. 全局逐步搜索建议文本（original 被替换时）
	if suggested != "" {
		if idx := searchProgressive(suggested); idx >= 0 {
			return idx
		}
	}

	// 4. 纯位置估算（最终兜底：至少返回一个近似行）
	if position > 0 {
		currentPos := 0
		for i, line := range lines {
			currentPos += len(line) + 1
			if currentPos >= position {
				return i
			}
		}
	}

	return -1
}

// getPrevLines 获取匹配行之前的 n 行
func getPrevLines(lines []string, idx int, n int) []string {
	start := idx - n
	if start < 0 {
		start = 0
	}
	if start >= idx {
		return nil
	}
	result := make([]string, idx-start)
	for i := start; i < idx; i++ {
		result[i-start] = lines[i]
	}
	return result
}

// getNextLines 获取匹配行之后的 n 行
func getNextLines(lines []string, idx int, n int) []string {
	end := idx + n + 1
	if end > len(lines) {
		end = len(lines)
	}
	if end <= idx+1 {
		return nil
	}
	result := make([]string, end-(idx+1))
	for i := idx + 1; i < end; i++ {
		result[i-(idx+1)] = lines[i]
	}
	return result
}

// getLastOrEmpty 取切片最后一项，空切片返回空字符串（兼容旧版prevLine字段）
func getLastOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

// getFirstOrEmpty 取切片第一项，空切片返回空字符串（兼容旧版nextLine字段）
func getFirstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// finalScan 绝望扫描：逐行检查原文/建议文本及其短片段
func finalScan(lines []string, original, suggested string) int {
	texts := []string{}
	if original != "" {
		texts = append(texts, original)
	}
	if suggested != "" && suggested != original {
		texts = append(texts, suggested)
	}

	for _, text := range texts {
		// 整段匹配
		for i, line := range lines {
			if strings.Contains(line, text) {
				return i
			}
		}
		// 逐步缩短后匹配
		runes := []rune(text)
		for shortenLen := 40; shortenLen >= 3; shortenLen -= 5 {
			if len(runes) > shortenLen {
				shortText := string(runes[:shortenLen])
				for i, line := range lines {
					if strings.Contains(line, shortText) {
						return i
					}
				}
			}
		}
	}
	return -1
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

func formatLogDetails(log *database.ProcessingLogRecord) string {
	if log.Details == "" {
		stepTexts := map[string]string{
			"cleaning": "基础清洗", "indexing": "向量检测",
			"llm_fix": "LLM修复", "review": "人工审核", "finalizing": "生成文件",
		}
		actionTexts := map[string]string{
			"start": "开始", "progress": "处理中", "success": "成功", "error": "错误",
		}
		step := stepTexts[log.Step]
		if step == "" {
			step = log.Step
		}
		action := actionTexts[log.Action]
		if action == "" {
			action = log.Action
		}
		return step + " " + action
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(log.Details), &parsed); err != nil {
		return log.Details
	}

	action, _ := parsed["action"].(string)
	details, _ := parsed["details"].(map[string]interface{})

	stepTexts := map[string]string{
		"cleaning": "基础清洗", "indexing": "向量检测",
		"llm_fix": "LLM修复", "review": "人工审核", "finalizing": "生成文件",
	}
	actionTexts := map[string]string{
		"step_started": "步骤开始", "step_completed": "步骤完成",
		"step_skipped": "步骤跳过", "step_failed": "步骤失败",
	}

	result := actionTexts[action]
	if result == "" {
		result = action
	}

	if details != nil {
		if step, ok := details["step"].(string); ok {
			if st := stepTexts[step]; st != "" {
				result += " - " + st
			} else {
				result += " - " + step
			}
		}
		if reason, ok := details["reason"].(string); ok {
			result += " (" + reason + ")"
		}
		if res, ok := details["result"].(map[string]interface{}); ok {
			var parts []string
			if v, ok := res["changes_count"].(float64); ok {
				parts = append(parts, fmt.Sprintf("修改数: %d", int(v)))
			}
			if v, ok := res["duplicates_detected"].(float64); ok {
				parts = append(parts, fmt.Sprintf("重复: %d", int(v)))
			}
			if v, ok := res["total_chunks"].(float64); ok {
				parts = append(parts, fmt.Sprintf("块数: %d", int(v)))
			}
			if v, ok := res["total_changes"].(float64); ok {
				parts = append(parts, fmt.Sprintf("变更: %d", int(v)))
			}
			if v, ok := res["cache_hits"].(float64); ok {
				parts = append(parts, fmt.Sprintf("缓存命中: %d", int(v)))
			}
			if len(parts) > 0 {
				result += " [" + strings.Join(parts, ", ") + "]"
			}
		}
	}

	if result == "" {
		return log.Details
	}
	return result
}
