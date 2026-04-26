package test

import (
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "testing"
  
  "github.com/gin-gonic/gin"
  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
  
  "voidtext/internal/config"
  "voidtext/web/backend/middleware"
)

func TestAuthMiddleware(t *testing.T) {
  // 设置Gin测试模式
  gin.SetMode(gin.TestMode)
  
  t.Run("认证已启用-有效token", func(t *testing.T) {
    // 设置配置
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "test_token_123",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 使用有效token的请求
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "test_token_123")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
  })
  
  t.Run("认证已启用-无效token", func(t *testing.T) {
    // 设置配置
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "test_token_123",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 使用无效token的请求
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "wrong_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    
    // 验证响应
    var response map[string]interface{}
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Equal(t, false, response["success"])
    assert.Contains(t, response["error"], "无效的认证token")
  })
  
  t.Run("认证已启用-缺少token", func(t *testing.T) {
    // 设置配置
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "test_token_123",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 缺少token的请求
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    
    // 验证响应
    var response map[string]interface{}
    err := json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.Equal(t, false, response["success"])
    assert.Contains(t, response["error"], "缺少认证token")
  })
  
  t.Run("认证已禁用", func(t *testing.T) {
    // 设置配置：禁用认证
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth: false,
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 没有token的请求应该通过
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    // 有token的请求也应该通过
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "any_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
  })
  
  t.Run("自定义header名称", func(t *testing.T) {
    // 设置配置：自定义header名称
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "custom_token",
      AuthHeaderName: "Authorization",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 使用默认header名称的请求应该失败
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "custom_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    
    // 使用自定义header名称的请求应该成功
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "custom_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
  })
  
  t.Run("空token配置", func(t *testing.T) {
    // 设置配置：启用认证但token为空
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 任何token都应该通过
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "any_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    // 没有token也应该通过
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
  })
  
  t.Run("token验证大小写敏感", func(t *testing.T) {
    // 设置配置
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "CaseSensitiveToken",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 正确的大小写应该通过
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "CaseSensitiveToken")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    // 不同大小写应该失败
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "casesensitivetoken")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "CASESENSITIVETOKEN")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
  })
  
  t.Run("token包含空格", func(t *testing.T) {
    // 设置配置
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "token with spaces",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 精确匹配应该通过
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "token with spaces")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    // 前后有空格应该失败
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", " token with spaces ")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    
    // 缺少空格应该失败
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "tokenwithspaces")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
  })
  
  t.Run("多个header值", func(t *testing.T) {
    // 设置配置
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "valid_token",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    router.Use(middleware.AuthMiddleware())
    
    router.GET("/test", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 多个header值，第一个有效
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Add("X-API-Token", "valid_token")
    req.Header.Add("X-API-Token", "invalid_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    // 多个header值，第一个无效
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/test", nil)
    req.Header.Add("X-API-Token", "invalid_token")
    req.Header.Add("X-API-Token", "valid_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
  })
  
  t.Run("跳过特定路径", func(t *testing.T) {
    // 设置配置
    config.AppConfigInstance = &config.AppConfig{
      EnableAuth:     true,
      AuthToken:      "test_token",
      AuthHeaderName: "X-API-Token",
    }
    
    router := gin.New()
    
    // 健康检查端点不需要认证
    router.GET("/health", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"status": "healthy"})
    })
    
    // 其他端点需要认证
    router.Use(middleware.AuthMiddleware())
    router.GET("/api/files", func(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"success": true})
    })
    
    // 健康检查端点不需要token
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/health", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    // API端点需要token
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/api/files", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    
    // API端点有有效token
    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/api/files", nil)
    req.Header.Set("X-API-Token", "test_token")
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
  })
}

func TestAuthMiddlewareConcurrency(t *testing.T) {
  // 设置配置
  config.AppConfigInstance = &config.AppConfig{
    EnableAuth:     true,
    AuthToken:      "concurrent_token",
    AuthHeaderName: "X-API-Token",
  }
  
  router := gin.New()
  router.Use(middleware.AuthMiddleware())
  
  requestCount := 0
  router.GET("/test", func(c *gin.Context) {
    requestCount++
    c.JSON(http.StatusOK, gin.H{"success": true})
  })
  
  const numGoroutines = 10
  const requestsPerGoroutine = 100
  
  errors := make(chan error, numGoroutines)
  
  for i := 0; i < numGoroutines; i++ {
    go func(id int) {
      for j := 0; j < requestsPerGoroutine; j++ {
        w := httptest.NewRecorder()
        req, _ := http.NewRequest("GET", "/test", nil)
        req.Header.Set("X-API-Token", "concurrent_token")
        
        router.ServeHTTP(w, req)
        
        if w.Code != http.StatusOK {
          errors <- assert.AnError
          return
        }
      }
      errors <- nil
    }(i)
  }
  
  // 收集错误
  for i := 0; i < numGoroutines; i++ {
    err := <-errors
    assert.NoError(t, err)
  }
  
  // 验证所有请求都成功
  assert.Equal(t, numGoroutines*requestsPerGoroutine, requestCount)
}

func TestAuthMiddlewarePerformance(t *testing.T) {
  // 设置配置
  config.AppConfigInstance = &config.AppConfig{
    EnableAuth:     true,
    AuthToken:      "performance_token",
    AuthHeaderName: "X-API-Token",
  }
  
  router := gin.New()
  router.Use(middleware.AuthMiddleware())
  
  router.GET("/test", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"success": true})
  })
  
  // 性能测试：发送大量请求
  const numRequests = 1000
  
  for i := 0; i < numRequests; i++ {
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-API-Token", "performance_token")
    
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code, "请求 %d 应该成功", i+1)
  }
}

// 辅助函数：模拟JSON反序列化
func jsonUnmarshal(data []byte, v interface{}) error {
  // 这里我们只是模拟，实际测试中应该使用encoding/json
  // 为了测试，我们手动解析简单的JSON
  str := string(data)
  if str == `{"success":true}` {
    if m, ok := v.(*map[string]interface{}); ok {
      *m = map[string]interface{}{
        "success": true,
      }
    }
  } else if str == `{"status":"healthy"}` {
    if m, ok := v.(*map[string]interface{}); ok {
      *m = map[string]interface{}{
        "status": "healthy",
      }
    }
  } else if str == `{"success":false,"error":"无效的认证token"}` {
    if m, ok := v.(*map[string]interface{}); ok {
      *m = map[string]interface{}{
        "success": false,
        "error":   "无效的认证token",
      }
    }
  } else if str == `{"success":false,"error":"缺少认证token"}` {
    if m, ok := v.(*map[string]interface{}); ok {
      *m = map[string]interface{}{
        "success": false,
        "error":   "缺少认证token",
      }
    }
  }
  return nil
}