package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/database"
	"txt-cleaning/internal/file"
)

// UploadFile 上传文件（支持MD5识别和智能行为）
func UploadFile(c *gin.Context) {
	f, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "获取文件失败: " + err.Error()})
		return
	}

	if f.Size > config.AppConfigInstance.MaxFileSize {
		maxSizeStr := formatFileSize(config.AppConfigInstance.MaxFileSize)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("文件大小超过限制 (最大 %s)", maxSizeStr)})
		return
	}

	if strings.ToLower(filepath.Ext(f.Filename)) != ".txt" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "只支持 .txt 文件，当前文件: " + f.Filename})
		return
	}

	tempPath := filepath.Join(config.AppConfigInstance.DataDir, "temp", fmt.Sprintf("%d_%s", time.Now().UnixNano(), f.Filename))
	err = c.SaveUploadedFile(f, tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存文件失败: " + err.Error()})
		return
	}

	fileMd5, err := file.ComputeFileMd5(tempPath)
	if err != nil {
		os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "计算文件MD5失败: " + err.Error()})
		return
	}

	existingFile, err := database.GetFileByMd5(fileMd5)
	if err != nil {
		os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件记录失败: " + err.Error()})
		return
	}

	versionRecord, _ := database.GetVersionByMd5(fileMd5)

	if existingFile != nil {
		handleExistingFile(c, existingFile, tempPath, fileMd5)
		return
	}

	if versionRecord != nil {
		handleIntermediateVersion(c, versionRecord, tempPath, fileMd5, f.Filename)
		return
	}

	createNewFileRecord(c, tempPath, fileMd5, f.Filename, f.Size)
}

// handleExistingFile 处理已存在的文件
func handleExistingFile(c *gin.Context, existing *database.FileRecord, tempPath, fileMd5 string) {
	switch existing.Status {
	case "completed":
		os.Remove(tempPath)
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"exists":     true,
			"status":     "completed",
			"md5":        fileMd5,
			"message":    "该文件已处理完成",
			"suggestion": "重新处理",
		})
	case "failed":
		os.Remove(tempPath)
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"exists":      true,
			"status":      "failed",
			"md5":         fileMd5,
			"currentStep": existing.CurrentStep,
			"errorMsg":    existing.ErrorMsg,
			"message":     "该文件处理失败",
			"suggestion":  "从失败步骤重新处理",
		})
	default:
		os.Remove(tempPath)
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"exists":      true,
			"status":      existing.Status,
			"md5":         fileMd5,
			"currentStep": existing.CurrentStep,
			"progress":    existing.Progress,
			"message":     "该文件正在处理中",
			"suggestion":  "继续上次进度",
		})
	}
}

// handleIntermediateVersion 处理中间版本文件
func handleIntermediateVersion(c *gin.Context, version *database.VersionRecord, tempPath, fileMd5, fileName string) {
	originalFile, _ := database.GetFileByMd5(version.OriginalMd5)

	finalPath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileMd5+"_"+fileName)
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存文件失败"})
		return
	}

	parsed := file.ParseFileName(fileName)

	record := &database.FileRecord{
		Md5:         fileMd5,
		OriginalMd5: version.OriginalMd5,
		Author:      parsed.Author,
		Title:       parsed.Title,
		FileName:    fileName,
		FilePath:    finalPath,
		Status:      "pending",
		CurrentStep: version.Step,
	}

	if originalFile != nil {
		record.RulesConfig = originalFile.RulesConfig
	}

	if err := database.CreateFile(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建文件记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"exists":         false,
		"md5":            fileMd5,
		"originalMd5":    version.OriginalMd5,
		"isIntermediate": true,
		"resumeStep":     version.Step,
		"message":        "检测到中间版本，可继续处理",
	})
}

// createNewFileRecord 创建新文件记录
func createNewFileRecord(c *gin.Context, tempPath, fileMd5, fileName string, fileSize int64) {
	finalPath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileMd5+"_"+fileName)
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存文件失败"})
		return
	}

	parsed := file.ParseFileName(fileName)

	record := &database.FileRecord{
		Md5:      fileMd5,
		Author:   parsed.Author,
		Title:    parsed.Title,
		FileName: fileName,
		FileSize: fileSize,
		FilePath: finalPath,
		Status:   "pending",
	}

	if err := database.CreateFile(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建文件记录失败"})
		return
	}

	versionRecord := &database.VersionRecord{
		OriginalMd5: fileMd5,
		VersionMd5:  fileMd5,
		VersionType: "original",
		FilePath:    finalPath,
		Step:        "upload",
	}
	database.CreateVersion(versionRecord)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"exists":  false,
		"md5":     fileMd5,
		"message": "文件上传成功",
	})
}

// ResumeFile 恢复文件处理
func ResumeFile(c *gin.Context) {
	md5 := c.Param("md5")

	record, err := database.GetFileByMd5(md5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件记录失败"})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	switch record.Status {
	case "completed":
		if err := database.UpdateFileStatus(md5, "pending", "", 0, ""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "重置文件状态失败"})
			return
		}
		if err := database.DeleteReviewItemsByFileMd5(md5); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "清除审核记录失败"})
			return
		}
		record.Status = "pending"
		record.CurrentStep = ""
		record.Progress = 0
	case "failed":
		if err := database.UpdateFileStatus(md5, "pending", record.CurrentStep, record.Progress, ""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "重置文件状态失败"})
			return
		}
		record.Status = "pending"
		record.ErrorMsg = ""
	case "processing":
		if err := database.UpdateFileStatus(md5, "pending", record.CurrentStep, record.Progress, ""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "重置文件状态失败"})
			return
		}
		record.Status = "pending"
		record.ErrorMsg = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"md5":         md5,
		"status":      record.Status,
		"currentStep": record.CurrentStep,
		"progress":    record.Progress,
		"message":     "已恢复处理",
	})
}

// ListFiles 列出所有文件
func ListFiles(c *gin.Context) {
	records, err := database.ListAllFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "files": records})
}

// GetFile 获取文件详情
func GetFile(c *gin.Context) {
	md5 := c.Param("md5")

	record, err := database.GetFileByMd5(md5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件失败"})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "file": record})
}

// GetFileContent 获取文件内容
func GetFileContent(c *gin.Context) {
	md5 := c.Param("md5")

	record, err := database.GetFileByMd5(md5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件失败"})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	content, err := os.ReadFile(record.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "content": string(content)})
}

// DownloadFile 下载文件（支持中间版本下载）
func DownloadFile(c *gin.Context) {
	md5 := c.Param("md5")

	record, err := database.GetFileByMd5(md5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件失败"})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	filePath := record.FilePath
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件已被删除"})
		return
	}

	c.FileAttachment(filePath, record.FileName)
}

// DeleteFile 删除文件记录
func DeleteFile(c *gin.Context) {
	md5 := c.Param("md5")

	record, err := database.GetFileByMd5(md5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询文件失败"})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	keepFile := c.Query("keepFile") == "true"

	if err := database.DeleteFile(md5); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除记录失败"})
		return
	}

	database.DeleteReviewItemsByFileMd5(md5)

	if !keepFile {
		os.Remove(record.FilePath)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "文件删除成功"})
}

// UpdateFileRules 更新文件规则配置
func UpdateFileRules(c *gin.Context) {
	md5 := c.Param("md5")

	var req struct {
		RulesConfig string `json:"rulesConfig"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}

	if err := database.UpdateFileRules(md5, req.RulesConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新规则失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "规则更新成功"})
}

func formatFileSize(bytes int64) string {
	const kb = 1024
	const mb = kb * 1024
	const gb = mb * 1024
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
