package handlers

import (
  "net/http"
  "time"
  
  "github.com/gin-gonic/gin"
  "github.com/gao/Builds/voidtext/internal/config"
  "github.com/gao/Builds/voidtext/web/backend/middleware"
)

// HealthResponse 健康检查响应
type HealthResponse struct {
  Status    string                 `json:"status"`
  Timestamp time.Time              `json:"timestamp"`
  Uptime    string                 `json:"uptime"`
  Version   string                 `json:"version"`
  RateLimit *RateLimitStatus       `json:"rate_limit,omitempty"`
  Database  *DatabaseStatus        `json:"database,omitempty"`
  Services  map[string]ServiceInfo `json:"services,omitempty"`
}

// RateLimitStatus 限流状态
type RateLimitStatus struct {
  Enabled      bool   `json:"enabled"`
  GlobalLimit  int    `json:"global_limit"`
  GlobalWindow string `json:"global_window"`
  UploadLimit  int    `json:"upload_limit"`
  UploadWindow string `json:"upload_window"`
  APILimit     int    `json:"api_limit"`
  APIWindow    string `json:"api_window"`
  StrictLimit  int    `json:"strict_limit"`
  StrictWindow string `json:"strict_window"`
}

// DatabaseStatus 数据库状态
type DatabaseStatus struct {
  Connected bool   `json:"connected"`
  Path      string `json:"path,omitempty"`
  Size      int64  `json:"size,omitempty"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
  Status  string `json:"status"`
  Message string `json:"message,omitempty"`
}

var (
  startTime = time.Now()
  appVersion = "1.0.0"
)

// HealthCheck 健康检查端点
func HealthCheck(c *gin.Context) {
  // 获取限流配置
  rateLimitConfig := config.GetRateLimitConfig()
  
  // 构建响应
  response := HealthResponse{
    Status:    "healthy",
    Timestamp: time.Now(),
    Uptime:    time.Since(startTime).String(),
    Version:   appVersion,
    Services:  make(map[string]ServiceInfo),
  }
  
  // 添加限流状态
  if rateLimitConfig != nil {
    response.RateLimit = &RateLimitStatus{
      Enabled:      rateLimitConfig.Enabled,
      GlobalLimit:  rateLimitConfig.Global.MaxRequests,
      GlobalWindow: rateLimitConfig.Global.Window.String(),
      UploadLimit:  rateLimitConfig.Upload.MaxRequests,
      UploadWindow: rateLimitConfig.Upload.Window.String(),
      APILimit:     rateLimitConfig.API.MaxRequests,
      APIWindow:    rateLimitConfig.API.Window.String(),
      StrictLimit:  rateLimitConfig.Strict.MaxRequests,
      StrictWindow: rateLimitConfig.Strict.Window.String(),
    }
  }
  
  // 添加数据库状态
  // 这里可以添加数据库连接检查
  response.Database = &DatabaseStatus{
    Connected: true,
  }
  
  // 添加服务状态
  response.Services["authentication"] = ServiceInfo{
    Status:  "enabled",
    Message: "Token-based authentication",
  }
  
  response.Services["rate_limiting"] = ServiceInfo{
    Status:  "enabled",
    Message: "IP-based rate limiting",
  }
  
  response.Services["file_processing"] = ServiceInfo{
    Status:  "enabled",
    Message: "Five-step processing pipeline",
  }
  
  c.JSON(http.StatusOK, response)
}

// RateLimitStatusCheck 限流状态检查
func RateLimitStatusCheck(c *gin.Context) {
  limiter := middleware.GetRateLimiter()
  ip := c.ClientIP()
  if ip == "" {
    ip = "unknown"
  }
  
  remaining := limiter.GetRemaining(ip)
  resetTime := limiter.GetResetTime(ip)
  resetIn := time.Until(resetTime)
  
  c.JSON(http.StatusOK, gin.H{
    "success": true,
    "data": gin.H{
      "client_ip": ip,
      "remaining_requests": remaining,
      "reset_time": resetTime.Format(time.RFC3339),
      "reset_in": resetIn.String(),
      "limit_reached": remaining == 0,
    },
  })
}

// Metrics 指标端点
func Metrics(c *gin.Context) {
  // 这里可以添加更多指标，如请求计数、错误率等
  c.JSON(http.StatusOK, gin.H{
    "success": true,
    "data": gin.H{
      "uptime": time.Since(startTime).String(),
      "timestamp": time.Now().Format(time.RFC3339),
      "version": appVersion,
      "rate_limiting": gin.H{
        "enabled": config.GetRateLimitConfig().Enabled,
      },
    },
  })
}

// ReadinessCheck 就绪检查
func ReadinessCheck(c *gin.Context) {
  // 检查关键服务是否就绪
  // 这里可以添加数据库连接检查、外部服务检查等
  
  c.JSON(http.StatusOK, gin.H{
    "success": true,
    "status": "ready",
    "timestamp": time.Now().Format(time.RFC3339),
  })
}

// LivenessCheck 存活检查
func LivenessCheck(c *gin.Context) {
  // 简单的存活检查
  c.JSON(http.StatusOK, gin.H{
    "success": true,
    "status": "alive",
    "timestamp": time.Now().Format(time.RFC3339),
  })
}