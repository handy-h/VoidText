package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/logging"

	"github.com/gin-gonic/gin"
)

// RateLimiter 限流器
type RateLimiter struct {
	mu          sync.RWMutex
	limits      map[string]*rateLimit
	maxRequests int           // 最大请求数
	window      time.Duration // 时间窗口
	cleanup     time.Duration // 清理间隔
	stopCh      chan struct{} // 停止清理 goroutine 的信号
}

// rateLimit 单个IP的限流信息
type rateLimit struct {
	count     int       // 请求计数
	resetTime time.Time // 重置时间
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(maxRequests int, window, cleanup time.Duration) *RateLimiter {
	limiter := &RateLimiter{
		limits:      make(map[string]*rateLimit),
		maxRequests: maxRequests,
		window:      window,
		cleanup:     cleanup,
		stopCh:      make(chan struct{}),
	}

	// 启动清理goroutine
	go limiter.cleanupExpired()

	return limiter
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 获取或创建限流记录
	limit, exists := rl.limits[ip]
	if !exists {
		rl.limits[ip] = &rateLimit{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	// 检查是否需要重置
	if now.After(limit.resetTime) {
		limit.count = 1
		limit.resetTime = now.Add(rl.window)
		return true
	}

	// 检查是否超过限制
	if limit.count >= rl.maxRequests {
		return false
	}

	limit.count++
	return true
}

// Close 停止清理 goroutine，释放资源
func (rl *RateLimiter) Close() {
	close(rl.stopCh)
}

// cleanupExpired 清理过期的限流记录
func (rl *RateLimiter) cleanupExpired() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, limit := range rl.limits {
				if now.After(limit.resetTime) {
					delete(rl.limits, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// GetRemaining 获取剩余请求数
func (rl *RateLimiter) GetRemaining(ip string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limit, exists := rl.limits[ip]
	if !exists {
		return rl.maxRequests
	}

	if time.Now().After(limit.resetTime) {
		return rl.maxRequests
	}

	remaining := rl.maxRequests - limit.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetResetTime 获取重置时间
func (rl *RateLimiter) GetResetTime(ip string) time.Time {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limit, exists := rl.limits[ip]
	if !exists {
		return time.Now().Add(rl.window)
	}

	return limit.resetTime
}

// 全局限流器实例
var (
	globalRateLimiter *RateLimiter
	rateLimiterOnce   sync.Once
)

// GetRateLimiter 获取全局限流器实例
func GetRateLimiter() *RateLimiter {
	rateLimiterOnce.Do(func() {
		config := config.GetRateLimitConfig()
		if !config.Enabled {
			// 如果限流被禁用，创建一个不限制的限流器
			globalRateLimiter = NewRateLimiter(1000000, time.Hour, time.Hour)
			return
		}

		globalRateLimiter = NewRateLimiter(
			config.Global.MaxRequests,
			config.Global.Window,
			config.Global.Cleanup,
		)
	})
	return globalRateLimiter
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware() gin.HandlerFunc {
	limiter := GetRateLimiter()
	limitStr := strconv.Itoa(limiter.maxRequests)

	return func(c *gin.Context) {
		// 获取客户端IP
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		// 检查是否允许请求
		if !limiter.Allow(ip) {
			remaining := limiter.GetRemaining(ip)
			resetTime := limiter.GetResetTime(ip)

			logging.Warn("请求频率超限", map[string]interface{}{
				"client_ip": ip,
				"path":      c.Request.URL.Path,
				"method":    c.Request.Method,
				"remaining": remaining,
				"reset_in":  time.Until(resetTime).String(),
			})

			c.Header("X-RateLimit-Limit", limitStr)
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", resetTime.Format(time.RFC1123))
			c.Header("Retry-After", resetTime.Format(time.RFC1123))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"success":     false,
				"error":       "请求频率超限，请稍后再试",
				"retry_after": resetTime.Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// 添加限流头信息
		remaining := limiter.GetRemaining(ip)
		resetTime := limiter.GetResetTime(ip)

		c.Header("X-RateLimit-Limit", limitStr)
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", resetTime.Format(time.RFC1123))

		c.Next()
	}
}

// sharedLimiters 缓存共享限流器实例，避免每次调用创建新实例导致 goroutine 泄漏
var (
	sharedLimiters   = make(map[string]*RateLimiter)
	sharedLimitersMu sync.Mutex
)

// getSharedLimiter 获取或创建共享限流器
func getSharedLimiter(maxRequests int, window, cleanup time.Duration) *RateLimiter {
	key := strconv.Itoa(maxRequests) + ":" + window.String() + ":" + cleanup.String()
	sharedLimitersMu.Lock()
	defer sharedLimitersMu.Unlock()
	if l, ok := sharedLimiters[key]; ok {
		return l
	}
	l := NewRateLimiter(maxRequests, window, cleanup)
	sharedLimiters[key] = l
	return l
}

// IPBasedRateLimit IP基础限流中间件
func IPBasedRateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	limiter := getSharedLimiter(maxRequests, window, window*2)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		if !limiter.Allow(ip) {
			remaining := limiter.GetRemaining(ip)
			resetTime := limiter.GetResetTime(ip)

			logging.Warn("IP请求频率超限", map[string]interface{}{
				"client_ip":    ip,
				"path":         c.Request.URL.Path,
				"method":       c.Request.Method,
				"max_requests": maxRequests,
				"window":       window.String(),
				"remaining":    remaining,
				"reset_in":     time.Until(resetTime).String(),
			})

			c.JSON(http.StatusTooManyRequests, gin.H{
				"success":     false,
				"error":       "请求频率超限，请稍后再试",
				"retry_after": resetTime.Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// EndpointBasedRateLimit 端点基础限流中间件
func EndpointBasedRateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	// 所有端点共享同一个限流器实例，以 endpoint 键区分
	limiter := getSharedLimiter(maxRequests, window, window*2)

	return func(c *gin.Context) {
		// 使用路径和方法作为键
		key := c.Request.Method + ":" + c.Request.URL.Path
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		if !limiter.Allow(ip + "|" + key) {
			remaining := limiter.GetRemaining(ip + "|" + key)
			resetTime := limiter.GetResetTime(ip + "|" + key)

			logging.Warn("端点请求频率超限", map[string]interface{}{
				"client_ip":    ip,
				"endpoint":     key,
				"max_requests": maxRequests,
				"window":       window.String(),
				"remaining":    remaining,
				"reset_in":     time.Until(resetTime).String(),
			})

			c.JSON(http.StatusTooManyRequests, gin.H{
				"success":     false,
				"error":       "该端点请求频率超限，请稍后再试",
				"retry_after": resetTime.Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UploadRateLimit 上传文件限流中间件
func UploadRateLimit() gin.HandlerFunc {
	config := config.GetRateLimitConfig()
	if !config.Enabled {
		// 如果限流被禁用，返回一个空中间件
		return func(c *gin.Context) { c.Next() }
	}
	return IPBasedRateLimit(config.Upload.MaxRequests, config.Upload.Window)
}

// APIRateLimit API限流中间件
func APIRateLimit() gin.HandlerFunc {
	config := config.GetRateLimitConfig()
	if !config.Enabled {
		// 如果限流被禁用，返回一个空中间件
		return func(c *gin.Context) { c.Next() }
	}
	return IPBasedRateLimit(config.API.MaxRequests, config.API.Window)
}

// StrictRateLimit 严格限流中间件
func StrictRateLimit() gin.HandlerFunc {
	config := config.GetRateLimitConfig()
	if !config.Enabled {
		// 如果限流被禁用，返回一个空中间件
		return func(c *gin.Context) { c.Next() }
	}
	return IPBasedRateLimit(config.Strict.MaxRequests, config.Strict.Window)
}

// EndpointRateLimit 端点限流中间件
func EndpointRateLimit() gin.HandlerFunc {
	config := config.GetRateLimitConfig()
	if !config.Enabled {
		// 如果限流被禁用，返回一个空中间件
		return func(c *gin.Context) { c.Next() }
	}
	return EndpointBasedRateLimit(config.Endpoint.MaxRequests, config.Endpoint.Window)
}
