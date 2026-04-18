package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"txt-cleaning/internal/config"
)

// UploadFile 上传文件
func UploadFile(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "获取文件失败: " + err.Error()})
		return
	}

	// 检查文件大小
	if file.Size > config.AppConfigInstance.MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("文件大小超过限制 (最大 %dMB)", config.AppConfigInstance.MaxFileSize/(1024*1024))})
		return
	}

	// 检查文件类型
	if filepath.Ext(file.Filename) != ".txt" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "只支持 .txt 文件"})
		return
	}

	// 生成唯一文件名
	fileId := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)

	// 保存文件
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存文件失败: " + err.Error()})
		return
	}

	// 返回文件ID
	c.JSON(http.StatusOK, gin.H{"success": true, "fileId": fileId})
}

// ListFiles 列出所有文件
func ListFiles(c *gin.Context) {
	uploadsDir := filepath.Join(config.AppConfigInstance.DataDir, "uploads")

	// 读取上传目录
	files, err := os.ReadDir(uploadsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件列表失败: " + err.Error()})
		return
	}

	// 构建文件列表
	fileList := make([]map[string]interface{}, 0)
	for _, file := range files {
		if !file.IsDir() {
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}

			fileList = append(fileList, map[string]interface{}{
				"id":       file.Name(),
				"name":     file.Name(),
				"size":     fileInfo.Size(),
				"modified": fileInfo.ModTime().Format(time.RFC3339),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "files": fileList})
}

// GetFile 获取文件内容
func GetFile(c *gin.Context) {
	fileId := c.Param("id")
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "content": string(content)})
}

// DeleteFile 删除文件
func DeleteFile(c *gin.Context) {
	fileId := c.Param("id")
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "文件删除成功"})
}