package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 简单 token 鉴权中间件。
// 通过 API_TOKEN 环境变量配置。若未设置，则跳过鉴权（兼容本地开发）。
// 客户端在请求头 X-API-Token 中传递 token。
func AuthMiddleware() gin.HandlerFunc {
	token := os.Getenv("API_TOKEN")
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		if c.GetHeader("X-API-Token") != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未授权：缺少或无效的 API Token",
			})
			return
		}
		c.Next()
	}
}
