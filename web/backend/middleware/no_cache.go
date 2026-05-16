package middleware

import (
	"github.com/gin-gonic/gin"
)

// NoCache 禁用静态文件缓存（开发模式）
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}
