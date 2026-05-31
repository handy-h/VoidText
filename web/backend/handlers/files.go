package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"voidtext/internal/config"
	"voidtext/internal/database"
	"voidtext/internal/errors"
	"voidtext/internal/file"
	"voidtext/internal/logging"
)

// uploadCounter 用于生成唯一临时文件名，防止高并发下冲突
var uploadCounter int64

// sanitizeFileName 提取纯文件名，防止路径穿越攻击
func sanitizeFileName(name string) string {
	clean := filepath.Base(filepath.Clean(name))
	clean = strings.ReplaceAll(clean, "/", "_")
	clean = strings.ReplaceAll(clean, "\\", "_")
	if clean == "." || clean == ".." || clean == "" {
		clean = "upload.txt"
	}
	// 白名单过滤：仅保留字母、数字、中文、空格、连字符、下划线、点
	var sb strings.Builder
	for _, r := range clean {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r >= 0x4e00 && r <= 0x9fff || r == ' ' || r == '-' || r == '_' || r == '.' {
			sb.WriteRune(r)
		}
	}
	result := sb.String()
	if result == "" {
		result = "upload.txt"
	}
	return result
}

// UploadFile 上传文件（支持MD5识别和智能行为）
func UploadFile(c *gin.Context) {
	f, err := c.FormFile("file")
	if err != nil {
		logging.Error("获取上传文件失败", err, map[string]interface{}{
			"client_ip": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, errors.NewErrorResponse(
			errors.New(errors.ErrBadRequest, "获取文件失败"),
		))
		return
	}

	// 净化文件名，防止路径穿越
	safeFileName := sanitizeFileName(f.Filename)

	// 记录上传文件信息
	logging.Info("收到文件上传请求", map[string]interface{}{
		"filename":  safeFileName,
		"size":      f.Size,
		"client_ip": c.ClientIP(),
	})

	if f.Size > config.AppConfigInstance.MaxFileSize {
		logging.Warn("文件大小超过限制", map[string]interface{}{
			"filename": safeFileName,
			"size":     f.Size,
			"max_size": config.AppConfigInstance.MaxFileSize,
		})
		c.JSON(http.StatusBadRequest, errors.NewErrorResponse(
			errors.NewWithDetails(errors.ErrBadRequest, "文件大小超过限制",
				fmt.Sprintf("文件大小: %d, 最大限制: %d", f.Size, config.AppConfigInstance.MaxFileSize)),
		))
		return
	}

	if strings.ToLower(filepath.Ext(safeFileName)) != ".txt" {
		logging.Warn("不支持的文件类型", map[string]interface{}{
			"filename":  safeFileName,
			"extension": filepath.Ext(safeFileName),
		})
		c.JSON(http.StatusBadRequest, errors.NewErrorResponse(
			errors.NewWithDetails(errors.ErrBadRequest, "不支持的文件类型",
				fmt.Sprintf("只支持 .txt 文件，当前文件: %s", safeFileName)),
		))
		return
	}

	// 确保临时目录存在
	tempDir := filepath.Join(config.AppConfigInstance.DataDir, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		logging.Error("创建临时目录失败", err, map[string]interface{}{
			"temp_dir": tempDir,
		})
		c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
			errors.Wrap(err, errors.ErrInternalServer, "创建临时目录失败"),
		))
		return
	}

	tempPath := filepath.Join(tempDir, fmt.Sprintf("%d_%d_%s", time.Now().UnixNano(), atomic.AddInt64(&uploadCounter, 1), safeFileName))
	err = c.SaveUploadedFile(f, tempPath)
	if err != nil {
		logging.Error("保存上传文件失败", err, map[string]interface{}{
			"filename":  safeFileName,
			"temp_path": tempPath,
		})
		c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
			errors.Wrap(err, errors.ErrFileUploadFailed, "保存文件失败"),
		))
		return
	}

	fileMd5, err := file.ComputeFileMd5(tempPath)
	if err != nil {
		os.Remove(tempPath)
		logging.Error("计算文件MD5失败", err, map[string]interface{}{
			"filename":  safeFileName,
			"temp_path": tempPath,
		})
		c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
			errors.Wrap(err, errors.ErrFileUploadFailed, "计算文件MD5失败"),
		))
		return
	}

	existingFile, err := database.GetFileByMd5(fileMd5)
	if err != nil {
		os.Remove(tempPath)
		logging.Error("查询文件记录失败", err, map[string]interface{}{
			"file_md5": fileMd5,
		})
		c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
			errors.Wrap(err, errors.ErrDatabase, "查询文件记录失败"),
		))
		return
	}

	versionRecord, err := database.GetVersionByMd5(fileMd5)
	if err != nil {
		logging.Warn("查询版本记录失败", map[string]interface{}{"file_md5": fileMd5, "error": err.Error()})
	}

	if existingFile != nil {
		handleExistingFile(c, existingFile, tempPath, fileMd5)
		return
	}

	if versionRecord != nil {
		handleIntermediateVersion(c, versionRecord, tempPath, fileMd5, safeFileName)
		return
	}

	createNewFileRecord(c, tempPath, fileMd5, safeFileName, f.Size)
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
	originalFile, err := database.GetFileByMd5(version.OriginalMd5)
	if err != nil {
		logging.Warn("查询原始文件记录失败", map[string]interface{}{
			"original_md5": version.OriginalMd5,
			"error":        err.Error(),
		})
	}

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
		logging.Error("移动文件失败", err, map[string]interface{}{
			"temp_path":  tempPath,
			"final_path": finalPath,
		})
		c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
			errors.Wrap(err, errors.ErrFileUploadFailed, "保存文件失败"),
		))
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

	versionRecord := &database.VersionRecord{
		OriginalMd5: fileMd5,
		VersionMd5:  fileMd5,
		VersionType: "original",
		FilePath:    finalPath,
		Step:        "upload",
	}

	// 使用事务创建文件记录和版本记录
	if err := database.CreateFileWithVersion(record, versionRecord); err != nil {
		// 如果数据库操作失败，删除已移动的文件
		os.Remove(finalPath)
		logging.Error("创建文件记录失败", err, map[string]interface{}{
			"file_md5": fileMd5,
			"filename": fileName,
		})
		c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
			errors.Wrap(err, errors.ErrDatabase, "创建文件记录失败"),
		))
		return
	}

	logging.Info("文件上传成功", map[string]interface{}{
		"file_md5":  fileMd5,
		"filename":  fileName,
		"file_size": fileSize,
		"author":    parsed.Author,
		"title":     parsed.Title,
	})

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
		// 使用事务重置文件状态并删除审核项
		err = database.WithTransactionSimple(func(tx database.Transaction) error {
			// 重置文件状态
			_, err := tx.Exec("UPDATE files SET status = ?, current_step = ?, progress = ?, error_msg = ?, updated_at = ? WHERE md5 = ?",
				"pending", "", 0, "", time.Now().Format(time.RFC3339), md5)
			if err != nil {
				return fmt.Errorf("重置文件状态失败: %w", err)
			}

			// 删除段级审核记录
			_, err = tx.Exec("DELETE FROM review_paragraphs WHERE file_md5 = ?", md5)
			if err != nil {
				return fmt.Errorf("清除审核记录失败: %w", err)
			}

			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}

		record.Status = "pending"
		record.CurrentStep = ""
		record.Progress = 0
	case "failed":
		// 使用事务重置文件状态
		err = database.WithTransactionSimple(func(tx database.Transaction) error {
			_, err := tx.Exec("UPDATE files SET status = ?, error_msg = ?, updated_at = ? WHERE md5 = ?",
				"pending", "", time.Now().Format(time.RFC3339), md5)
			return err
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "重置文件状态失败: " + err.Error()})
			return
		}

		record.Status = "pending"
		record.ErrorMsg = ""
	case "processing":
		// 使用事务重置文件状态并清空当前步骤
		err = database.WithTransactionSimple(func(tx database.Transaction) error {
			_, err := tx.Exec("UPDATE files SET status = ?, current_step = ?, error_msg = ?, updated_at = ? WHERE md5 = ?",
				"pending", "", "", time.Now().Format(time.RFC3339), md5)
			return err
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "重置文件状态失败: " + err.Error()})
			return
		}
		record.Status = "pending"
		record.CurrentStep = ""
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

// ListFiles 列出所有文件（支持分页：limit/offset 查询参数）
func ListFiles(c *gin.Context) {
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if o := c.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	records, total, err := database.ListAllFiles(limit, offset)
	if err != nil {
		log.Printf("[文件列表] 查询失败: %v", err) // 内部记录详细错误
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "files": records, "total": total, "limit": limit, "offset": offset})
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

// GetFileContent 获取文件内容（limit 参数单位为字节，默认 1MB，最大 10MB）
// 审核阶段优先使用 review_baseline_path 作为稳定的审核基线
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

	// 审核阶段使用固定的审核基线文件，避免流水线覆盖 file_path 导致底稿不稳定
	filePath := record.FilePath
	if record.Status == "reviewing" && record.ReviewBaselinePath != "" {
		filePath = record.ReviewBaselinePath
	}

	const defaultLimit = 1 * 1024 * 1024 // 1MB
	const maxLimit = 10 * 1024 * 1024    // 10MB
	readLimit := int64(defaultLimit)
	if q := c.Query("limit"); q != "" {
		if n, err := strconv.ParseInt(q, 10, 64); err == nil && n > 0 {
			if n > maxLimit {
				n = maxLimit
			}
			readLimit = n
		}
	}

	f, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件失败: " + err.Error()})
		return
	}
	defer f.Close()

	// 使用 LimitReader + ReadAll 语义更清晰，避免 io.ReadFull 对小文件返回 ErrUnexpectedEOF
	limitedReader := io.LimitReader(f, readLimit)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件失败: " + err.Error()})
		return
	}

	fi, _ := f.Stat()
	var fileSize int64
	if fi != nil {
		fileSize = fi.Size()
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"content":   string(content),
		"truncated": int64(len(content)) < fileSize,
		"fileSize":  fileSize,
	})
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

	// 使用 RFC 5987 编码支持中文文件名
	// filename*=UTF-8'' 在前，浏览器优先使用；filename 作为 ASCII fallback
	fileName := record.FileName
	var asciiName string
	if isASCII(fileName) {
		asciiName = fileName
	} else {
		asciiName = "download.txt"
	}
	encodedName := url.QueryEscape(fileName)
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename*=UTF-8''%s; filename="%s"`, encodedName, asciiName))
	c.File(filePath)
}

// isASCII 判断字符串是否全部由 ASCII 字符组成
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
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

	// 使用事务删除所有相关数据
	if err := database.DeleteFileWithRelatedData(md5); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除记录失败: " + err.Error()})
		return
	}

	// 同步删除物理文件，避免产生无法管理的孤儿文件
	if err := os.Remove(record.FilePath); err != nil && !os.IsNotExist(err) {
		logging.Warn("删除文件失败", map[string]interface{}{
			"file_path": record.FilePath,
			"error":     err.Error(),
		})
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
