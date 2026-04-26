package middleware

import (
  "net/http"
  "runtime/debug"
  "time"
  
  "voidtext/internal/errors"
  "voidtext/internal/logging"
  "github.com/gin-gonic/gin"
)

// ErrorHandler 错误处理中间件
func ErrorHandler() gin.HandlerFunc {
  return func(c *gin.Context) {
    defer func() {
      if r := recover(); r != nil {
        // 记录panic信息
        stack := debug.Stack()
        logging.Error("处理请求时发生panic", nil, map[string]interface{}{
          "method": c.Request.Method,
          "path":   c.Request.URL.Path,
          "panic":  r,
          "stack":  string(stack),
        })
        
        // 返回500错误
        c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
          errors.New(errors.ErrInternalServer, "服务器内部错误"),
        ))
        c.Abort()
      }
    }()
    
    c.Next()
    
    // 检查是否有错误
    if len(c.Errors) > 0 {
      // 获取最后一个错误
      err := c.Errors.Last().Err
      
      // 记录错误
      logging.Error("处理请求时发生错误", err, map[string]interface{}{
        "method": c.Request.Method,
        "path":   c.Request.URL.Path,
        "status": c.Writer.Status(),
      })
      
      // 返回错误响应
      c.JSON(c.Writer.Status(), errors.NewErrorResponse(err))
      c.Abort()
    }
  }
}

// Recovery 恢复中间件（处理panic）
func Recovery() gin.HandlerFunc {
  return func(c *gin.Context) {
    defer func() {
      if r := recover(); r != nil {
        // 记录panic信息
        stack := debug.Stack()
        logging.Error("处理请求时发生panic", nil, map[string]interface{}{
          "method": c.Request.Method,
          "path":   c.Request.URL.Path,
          "panic":  r,
          "stack":  string(stack),
        })
        
        // 返回500错误
        c.JSON(http.StatusInternalServerError, errors.NewErrorResponse(
          errors.New(errors.ErrInternalServer, "服务器内部错误"),
        ))
        c.Abort()
      }
    }()
    
    c.Next()
  }
}

// NotFoundHandler 404处理
func NotFoundHandler() gin.HandlerFunc {
  return func(c *gin.Context) {
    logging.Warn("请求的资源不存在", map[string]interface{}{
      "method": c.Request.Method,
      "path":   c.Request.URL.Path,
    })
    
    c.JSON(http.StatusNotFound, errors.NewErrorResponse(
      errors.New(errors.ErrNotFound, "请求的资源不存在"),
    ))
  }
}

// MethodNotAllowedHandler 405处理
func MethodNotAllowedHandler() gin.HandlerFunc {
  return func(c *gin.Context) {
    logging.Warn("请求方法不允许", map[string]interface{}{
      "method": c.Request.Method,
      "path":   c.Request.URL.Path,
    })
    
    c.JSON(http.StatusMethodNotAllowed, errors.NewErrorResponse(
      errors.New(errors.ErrBadRequest, "请求方法不允许"),
    ))
  }
}

// ValidationMiddleware 验证中间件
func ValidationMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    c.Next()
    
    // 这里可以添加验证逻辑
    // 例如：验证请求参数、请求体等
  }
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    // 记录请求开始
    start := time.Now()
    path := c.Request.URL.Path
    method := c.Request.Method
    
    // 处理请求
    c.Next()
    
    // 记录请求完成
    latency := time.Since(start)
    status := c.Writer.Status()
    
    fields := map[string]interface{}{
      "method":     method,
      "path":       path,
      "status":     status,
      "latency":    latency.String(),
      "client_ip":  c.ClientIP(),
      "user_agent": c.Request.UserAgent(),
    }
    
    // 根据状态码记录不同级别的日志
    if status >= 500 {
      logging.Error("服务器错误", nil, fields)
    } else if status >= 400 {
      logging.Warn("客户端错误", fields)
    } else {
      logging.Info("请求完成", fields)
    }
  }
}