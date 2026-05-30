package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"voidtext/web/backend/middleware"
)

func TestRateLimiter(t *testing.T) {
	t.Run("创建限流器", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(10, time.Minute, 5*time.Minute)
		assert.NotNil(t, limiter)
	})

	t.Run("允许请求", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(10, time.Minute, 5*time.Minute)

		// 前10个请求应该被允许
		for i := 0; i < 10; i++ {
			allowed := limiter.Allow("test_ip")
			assert.True(t, allowed, "请求 %d 应该被允许", i+1)
		}

		// 第11个请求应该被拒绝
		allowed := limiter.Allow("test_ip")
		assert.False(t, allowed, "第11个请求应该被拒绝")
	})

	t.Run("不同IP独立限流", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(5, time.Minute, 5*time.Minute)

		// IP1 使用5个请求
		for i := 0; i < 5; i++ {
			allowed := limiter.Allow("ip1")
			assert.True(t, allowed, "IP1 请求 %d 应该被允许", i+1)
		}

		// IP1 第6个请求应该被拒绝
		allowed := limiter.Allow("ip1")
		assert.False(t, allowed, "IP1 第6个请求应该被拒绝")

		// IP2 应该还有5个请求额度
		for i := 0; i < 5; i++ {
			allowed := limiter.Allow("ip2")
			assert.True(t, allowed, "IP2 请求 %d 应该被允许", i+1)
		}

		// IP2 第6个请求应该被拒绝
		allowed = limiter.Allow("ip2")
		assert.False(t, allowed, "IP2 第6个请求应该被拒绝")
	})

	t.Run("时间窗口重置", func(t *testing.T) {
		// 使用很短的时间窗口进行测试
		limiter := middleware.NewRateLimiter(3, 100*time.Millisecond, 200*time.Millisecond)

		// 使用3个请求
		for i := 0; i < 3; i++ {
			allowed := limiter.Allow("test_ip")
			assert.True(t, allowed, "请求 %d 应该被允许", i+1)
		}

		// 第4个请求应该被拒绝
		allowed := limiter.Allow("test_ip")
		assert.False(t, allowed, "第4个请求应该被拒绝")

		// 等待时间窗口过去
		time.Sleep(150 * time.Millisecond)

		// 现在应该可以再次请求
		allowed = limiter.Allow("test_ip")
		assert.True(t, allowed, "时间窗口重置后应该允许请求")
	})

	t.Run("获取剩余请求数", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(5, time.Minute, 5*time.Minute)

		// 初始应该有5个剩余
		remaining := limiter.GetRemaining("test_ip")
		assert.Equal(t, 5, remaining)

		// 使用2个请求
		limiter.Allow("test_ip")
		limiter.Allow("test_ip")

		// 剩余3个
		remaining = limiter.GetRemaining("test_ip")
		assert.Equal(t, 3, remaining)

		// 使用完所有请求
		limiter.Allow("test_ip")
		limiter.Allow("test_ip")
		limiter.Allow("test_ip")

		// 剩余0个
		remaining = limiter.GetRemaining("test_ip")
		assert.Equal(t, 0, remaining)
	})

	t.Run("获取重置时间", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(5, time.Minute, 5*time.Minute)

		resetTime := limiter.GetResetTime("test_ip")
		assert.NotZero(t, resetTime)
		assert.True(t, time.Until(resetTime) > 0, "重置时间应该在将来")
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	// 设置Gin测试模式
	gin.SetMode(gin.TestMode)

	t.Run("限流中间件允许请求", func(t *testing.T) {
		router := gin.New()
		limiter := middleware.NewRateLimiter(10, time.Minute, 5*time.Minute)

		// 创建自定义中间件使用测试限流器
		testMiddleware := func(c *gin.Context) {
			ip := c.ClientIP()
			if ip == "" {
				ip = "unknown"
			}

			if !limiter.Allow(ip) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success": false,
					"error":   "请求频率超限",
				})
				c.Abort()
				return
			}

			c.Next()
		}

		router.Use(testMiddleware)

		requestCount := 0
		router.GET("/test", func(c *gin.Context) {
			requestCount++
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		// 发送10个请求
		for i := 0; i < 10; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "请求 %d 应该成功", i+1)
		}

		// 第11个请求应该被限流
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("不同IP独立限流", func(t *testing.T) {
		router := gin.New()
		limiter := middleware.NewRateLimiter(5, time.Minute, 5*time.Minute)

		testMiddleware := func(c *gin.Context) {
			ip := c.ClientIP()
			if ip == "" {
				ip = "unknown"
			}

			if !limiter.Allow(ip) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success": false,
					"error":   "请求频率超限",
				})
				c.Abort()
				return
			}

			c.Next()
		}

		router.Use(testMiddleware)

		ip1Count := 0
		ip2Count := 0
		router.GET("/test", func(c *gin.Context) {
			if c.ClientIP() == "192.168.1.1" {
				ip1Count++
			} else if c.ClientIP() == "192.168.1.2" {
				ip2Count++
			}
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		// IP1 发送5个请求
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// IP1 第6个请求应该被限流
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		// IP2 应该还可以发送5个请求
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.2:12345"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// IP2 第6个请求应该被限流
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		assert.Equal(t, 5, ip1Count)
		assert.Equal(t, 5, ip2Count)
	})

	t.Run("限流头信息", func(t *testing.T) {
		router := gin.New()

		// 使用内置的RateLimitMiddleware
		router.Use(middleware.RateLimitMiddleware())

		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		// 发送请求
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// 检查限流头信息
		assert.Contains(t, w.Header(), "X-Ratelimit-Limit")
		assert.Contains(t, w.Header(), "X-Ratelimit-Remaining")
		assert.Contains(t, w.Header(), "X-Ratelimit-Reset")
	})

	t.Run("限流被禁用", func(t *testing.T) {
		// 创建一个被禁用的限流器
		limiter := middleware.NewRateLimiter(1000000, time.Hour, time.Hour)

		router := gin.New()
		testMiddleware := func(c *gin.Context) {
			ip := c.ClientIP()
			if ip == "" {
				ip = "unknown"
			}

			if !limiter.Allow(ip) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success": false,
					"error":   "请求频率超限",
				})
				c.Abort()
				return
			}

			c.Next()
		}

		router.Use(testMiddleware)

		requestCount := 0
		router.GET("/test", func(c *gin.Context) {
			requestCount++
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		// 发送大量请求（应该都被允许）
		for i := 0; i < 1000; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "请求 %d 应该成功", i+1)
		}

		assert.Equal(t, 1000, requestCount)
	})
}

func TestIPBasedRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("IP基础限流", func(t *testing.T) {
		router := gin.New()

		// 使用严格的限流：每分钟2个请求
		router.Use(middleware.IPBasedRateLimit(2, time.Minute))

		requestCount := 0
		router.GET("/test", func(c *gin.Context) {
			requestCount++
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		// 前2个请求应该成功
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "请求 %d 应该成功", i+1)
		}

		// 第3个请求应该被限流
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		assert.Equal(t, 2, requestCount)
	})

	t.Run("端点基础限流", func(t *testing.T) {
		router := gin.New()

		// 端点限流：每个端点每分钟2个请求
		router.Use(middleware.EndpointBasedRateLimit(2, time.Minute))

		endpoint1Count := 0
		endpoint2Count := 0

		router.GET("/endpoint1", func(c *gin.Context) {
			endpoint1Count++
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		router.GET("/endpoint2", func(c *gin.Context) {
			endpoint2Count++
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		// 端点1：前2个请求成功
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/endpoint1", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// 端点1：第3个请求被限流
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/endpoint1", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		// 端点2：应该还有2个请求额度
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/endpoint2", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// 端点2：第3个请求被限流
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/endpoint2", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		assert.Equal(t, 2, endpoint1Count)
		assert.Equal(t, 2, endpoint2Count)
	})
}

func TestRateLimitCleanup(t *testing.T) {
	t.Run("清理过期记录", func(t *testing.T) {
		// 使用很短的时间窗口和清理间隔
		limiter := middleware.NewRateLimiter(5, 50*time.Millisecond, 100*time.Millisecond)

		// 使用所有请求
		for i := 0; i < 5; i++ {
			allowed := limiter.Allow("test_ip")
			assert.True(t, allowed)
		}

		// 等待时间窗口过去
		time.Sleep(100 * time.Millisecond)

		// 等待清理goroutine运行
		time.Sleep(150 * time.Millisecond)

		// 现在应该可以再次请求（记录已被清理）
		allowed := limiter.Allow("test_ip")
		assert.True(t, allowed, "清理后应该允许请求")
	})

	t.Run("并发访问", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(100, time.Minute, 5*time.Minute)

		const numGoroutines = 10
		const requestsPerGoroutine = 20

		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				for j := 0; j < requestsPerGoroutine; j++ {
					ip := "ip_" + string(rune(id))
					if !limiter.Allow(ip) {
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
	})
}
