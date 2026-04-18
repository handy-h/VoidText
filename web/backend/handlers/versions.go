package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/file"
)

// 全局版本管理器
var versionManager = file.NewVersionManager()

// ListVersions 列出文件的所有版本
func ListVersions(c *gin.Context) {
	fileId := c.Param("id")

	// 检查文件是否存在
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 获取版本列表
	versions, err := versionManager.GetVersions(fileId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "获取版本列表失败: " + err.Error()})
		return
	}

	// 构建版本列表
	versionList := make([]map[string]interface{}, len(versions))
	for i, v := range versions {
		versionList[i] = map[string]interface{}{
			"version":   v.Version,
			"timestamp": v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"size":      v.Size,
			"note":      v.Note,
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "versions": versionList})
}

// GetVersion 获取特定版本的文件内容
func GetVersion(c *gin.Context) {
	fileId := c.Param("id")
	version := c.Param("version")

	// 检查文件是否存在
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 获取版本
	v, err := versionManager.GetVersion(fileId, version)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "版本不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "content": v.Content})
}

// RestoreVersion 恢复到特定版本
func RestoreVersion(c *gin.Context) {
	fileId := c.Param("id")
	version := c.Param("version")

	// 检查文件是否存在
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 恢复版本
	content, err := versionManager.RestoreVersion(fileId, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "恢复版本失败: " + err.Error()})
		return
	}

	// 更新原文件
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "版本恢复成功"})
}

// DeleteVersion 删除特定版本
func DeleteVersion(c *gin.Context) {
	fileId := c.Param("id")
	version := c.Param("version")

	// 检查文件是否存在
	filePath := filepath.Join(config.AppConfigInstance.DataDir, "uploads", fileId)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	// 删除版本
	if err := versionManager.DeleteVersion(fileId, version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "删除版本失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "版本删除成功"})
}