package middleware

import (
	"os"

	"github.com/gin-gonic/gin"
)

// NoCache 禁用静态文件缓存（开发模式）
// 生产环境（GIN_MODE=release）下自动跳过，允许浏览器缓存提升性能
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("GIN_MODE") == "release" {
			c.Next()
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}
