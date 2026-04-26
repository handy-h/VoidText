package middleware

import (
  "net/http"
  "strings"

  "github.com/gin-gonic/gin"
  "voidtext/internal/config"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    // 如果未启用认证，直接放行
    if !config.AppConfigInstance.EnableAuth {
      c.Next()
      return
    }

    // 获取请求头中的token
    authHeader := c.GetHeader(config.AppConfigInstance.AuthHeaderName)
    if authHeader == "" {
      c.JSON(http.StatusUnauthorized, gin.H{
        "code":    401,
        "message": "未提供认证token",
        "data":    nil,
      })
      c.Abort()
      return
    }

    // 验证token
    expectedToken := config.AppConfigInstance.AuthToken
    if expectedToken == "" {
      c.JSON(http.StatusInternalServerError, gin.H{
        "code":    500,
        "message": "服务器认证配置错误",
        "data":    nil,
      })
      c.Abort()
      return
    }

    // 简单比较token
    if strings.TrimSpace(authHeader) != expectedToken {
      c.JSON(http.StatusUnauthorized, gin.H{
        "code":    401,
        "message": "认证token无效",
        "data":    nil,
      })
      c.Abort()
      return
    }

    c.Next()
  }
}

// OptionalAuthMiddleware 可选认证中间件
func OptionalAuthMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    // 如果未启用认证，直接放行
    if !config.AppConfigInstance.EnableAuth {
      c.Next()
      return
    }

    // 获取请求头中的token
    authHeader := c.GetHeader(config.AppConfigInstance.AuthHeaderName)
    expectedToken := config.AppConfigInstance.AuthToken

    // 如果提供了token，验证它
    if authHeader != "" && expectedToken != "" {
      if strings.TrimSpace(authHeader) != expectedToken {
        c.JSON(http.StatusUnauthorized, gin.H{
          "code":    401,
          "message": "认证token无效",
          "data":    nil,
        })
        c.Abort()
        return
      }
    }

    c.Next()
  }
}