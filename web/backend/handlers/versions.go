package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"txt-cleaning/internal/database"
)

// ListVersions 列出文件的所有版本
func ListVersions(c *gin.Context) {
	md5 := c.Param("md5")

	versions, err := database.GetVersionsByOriginalMd5(md5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "获取版本列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "versions": versions})
}
