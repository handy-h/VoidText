package external

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/logging"
)

// 全局HTTP客户端单例，配置连接池和超时设置
// 使用单例模式避免创建过多HTTP客户端，提高连接复用率
var (
	globalHTTPClient     *http.Client
	globalHTTPClientOnce sync.Once
)

// APIRateLimiter API限流器
type APIRateLimiter struct {
	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
	tokenBucket int
	maxTokens   int
	refillRate  time.Duration
}

// NewAPIRateLimiter 创建API限流器
func NewAPIRateLimiter(minInterval time.Duration, maxTokens int, refillRate time.Duration) *APIRateLimiter {
	return &APIRateLimiter{
		minInterval: minInterval,
		tokenBucket: maxTokens,
		maxTokens:   maxTokens,
		refillRate:  refillRate,
		lastRequest: time.Now().Add(-minInterval), // 允许立即发送第一个请求
	}
}

// Acquire 获取API调用许可
func (rl *APIRateLimiter) Acquire() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 检查是否需要补充令牌
	now := time.Now()
	elapsed := now.Sub(rl.lastRequest)
	if elapsed >= rl.refillRate && rl.tokenBucket < rl.maxTokens {
		rl.tokenBucket++
		rl.lastRequest = now
	}

	// 等待直到有可用令牌
	for rl.tokenBucket <= 0 {
		rl.mu.Unlock()
		time.Sleep(rl.minInterval)
		rl.mu.Lock()

		// 重新检查时间并补充令牌
		now = time.Now()
		elapsed = now.Sub(rl.lastRequest)
		if elapsed >= rl.refillRate && rl.tokenBucket < rl.maxTokens {
			rl.tokenBucket++
			rl.lastRequest = now
		}
	}

	// 使用令牌
	rl.tokenBucket--
}

// 全局API限流器 - 分离策略（线程安全初始化）
var (
	remoteRateLimiter     *APIRateLimiter
	localRateLimiter      *APIRateLimiter
	rateLimiterInitOnce   sync.Once
)

// RateLimiterConfig 限流器配置
type RateLimiterConfig struct {
	RemoteInterval time.Duration
	RemoteBurst    int
	RemoteTimeout  time.Duration
	LocalInterval  time.Duration
	LocalBurst     int
	LocalTimeout   time.Duration
}

// DefaultRateLimiterConfig 默认限流器配置
var DefaultRateLimiterConfig = RateLimiterConfig{
	RemoteInterval: 200 * time.Millisecond, // 远程API间隔：200ms
	RemoteBurst:    10,                     // 远程API突发：10个请求
	RemoteTimeout:  3 * time.Second,        // 远程API超时：3秒
	LocalInterval:  10 * time.Millisecond,  // 本地模型间隔：10ms
	LocalBurst:     50,                     // 本地模型突发：50个请求
	LocalTimeout:   1 * time.Second,        // 本地模型超时：1秒
}

// initRateLimiters 初始化限流器（线程安全）
func initRateLimiters() {
	rateLimiterInitOnce.Do(func() {
		// 远程API限流器（严格限流，针对Coding Plan）
		remoteRateLimiter = NewAPIRateLimiter(
			DefaultRateLimiterConfig.RemoteInterval,
			DefaultRateLimiterConfig.RemoteBurst,
			DefaultRateLimiterConfig.RemoteTimeout,
		)
		// 本地模型限流器（宽松限流）
		localRateLimiter = NewAPIRateLimiter(
			DefaultRateLimiterConfig.LocalInterval,
			DefaultRateLimiterConfig.LocalBurst,
			DefaultRateLimiterConfig.LocalTimeout,
		)
	})
}

// getRemoteRateLimiter 获取远程API限流器
func getRemoteRateLimiter() *APIRateLimiter {
	initRateLimiters()
	return remoteRateLimiter
}

// getLocalRateLimiter 获取本地模型限流器
func getLocalRateLimiter() *APIRateLimiter {
	initRateLimiters()
	return localRateLimiter
}

// getGlobalHTTPClient 获取全局HTTP客户端单例
// 配置MaxIdleConnsPerHost=100提高并发性能，支持大量API调用
func getGlobalHTTPClient() *http.Client {
	globalHTTPClientOnce.Do(func() {
		// 创建自定义Transport，优化连接池设置
		transport := &http.Transport{
			// 每个主机的最大空闲连接数，设置为100以支持高并发
			MaxIdleConnsPerHost: 100,
			// 总的最大空闲连接数
			MaxIdleConns: 200,
			// 最大打开连接数
			MaxConnsPerHost: 200,
			// 连接空闲超时时间
			IdleConnTimeout: 90 * time.Second,
			// TLS握手超时
			TLSHandshakeTimeout: 10 * time.Second,
			// 期望保持连接
			DisableKeepAlives: false,
		}

		globalHTTPClient = &http.Client{
			Transport: transport,
			// 总请求超时时间（包括连接、重定向、读取响应体）
			Timeout: 120 * time.Second,
			// 不跟随重定向
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	})

	return globalHTTPClient
}

// BackoffStrategy 退避策略接口
type BackoffStrategy interface {
	Next(retry int) time.Duration
}

// ExponentialBackoff 指数退避策略
type ExponentialBackoff struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
	Jitter    bool
}

// Next 计算下一次重试的延迟时间
// 公式：delay = baseDelay * 2^retry + jitter
// 添加jitter避免惊群效应
func (eb *ExponentialBackoff) Next(retry int) time.Duration {
	if retry < 0 {
		retry = 0
	}

	// 计算指数退避延迟
	delay := eb.BaseDelay * time.Duration(math.Pow(2, float64(retry)))
	if delay > eb.MaxDelay {
		delay = eb.MaxDelay
	}

	// 添加随机抖动（±15%），避免所有请求同时重试
	if eb.Jitter {
		jitter := 0.15 // ±15%
		jitterFactor := 1 + jitter*(2*rand.Float64()-1)
		delay = time.Duration(float64(delay) * jitterFactor)
	}

	return delay
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries      int
	BackoffStrategy BackoffStrategy
	RetryableStatus []int // 可重试的HTTP状态码
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 5,
		BackoffStrategy: &ExponentialBackoff{
			BaseDelay: 500 * time.Millisecond,
			MaxDelay:  30 * time.Second,
			Jitter:    true,
		},
		// 429: 限流，500-599: 服务器错误
		RetryableStatus: []int{429, 500, 502, 503, 504},
	}
}

// APIError API错误
type APIError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("API错误 (状态码: %d): %s - %v", e.StatusCode, e.Message, e.Err)
	}
	return fmt.Sprintf("API错误 (状态码: %d): %s", e.StatusCode, e.Message)
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// 检查状态码是否在可重试范围内
		for _, status := range DefaultRetryConfig().RetryableStatus {
			if apiErr.StatusCode == status {
				return true
			}
		}
	}
	// 网络错误、超时错误等也可重试
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) ||
		isTimeoutError(err)
}

// isTimeoutError 判断是否为超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errors.Is(err, http.ErrHandlerTimeout) ||
		errors.Is(err, context.DeadlineExceeded) ||
		containsAny(errStr, []string{"timeout", "Timeout", "timed out", "Timed out"})
}

// containsAny 检查字符串是否包含任意子串
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}

// API 外部API客户端（重构版）
type API struct {
	client                *http.Client
	baseURL               string
	apiKey                string
	embeddingModelName    string
	completionModelName   string
	completionTemperature float64
	completionMaxTokens   int
	retryConfig           *RetryConfig
	mu                    sync.RWMutex // 保护配置更新
	isLocalModel          bool         // 是否为本地模型
}

// IsLocalModel 检查是否为本地模型
func (api *API) IsLocalModel() bool {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.isLocalModel
}

// GetBaseURL 获取基础URL
func (api *API) GetBaseURL() string {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.baseURL
}

// GetCompletionModelName 获取完成模型名称
func (api *API) GetCompletionModelName() string {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.completionModelName
}

// NewAPI 创建新的API客户端
func NewAPI() *API {
	return &API{
		client:                getGlobalHTTPClient(),
		baseURL:               config.AppConfigInstance.LLMApiURL,
		apiKey:                config.AppConfigInstance.LLMApiKey,
		embeddingModelName:    config.AppConfigInstance.VectorModelName,
		completionModelName:   config.AppConfigInstance.CompletionModelName,
		completionTemperature: config.AppConfigInstance.CompletionTemperature,
		completionMaxTokens:   config.AppConfigInstance.CompletionMaxTokens,
		retryConfig:           DefaultRetryConfig(),
		isLocalModel:          false, // 默认为远程API
	}
}

// NewLocalAPI 创建本地模型API客户端
func NewLocalAPI(baseURL string) *API {
	return &API{
		client:                getGlobalHTTPClient(),
		baseURL:               baseURL,
		apiKey:                "", // 本地模型不需要API Key
		embeddingModelName:    "",
		completionModelName:   "qwen2.5:7b-instruct-q4_K_M",
		completionTemperature: 0.3,
		completionMaxTokens:   2048,
		retryConfig:           DefaultRetryConfig(),
		isLocalModel:          true,
	}
}

// 原有API错误响应结构体（与第三方API响应格式匹配）
// 注意：这个结构体与上面的APIError不同，这个是解析第三方API错误响应的
type APIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// EmbeddingResponse 嵌入响应
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// CompletionRequest 完成请求（Legacy API）
type CompletionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// CompletionResponse 完成响应（Legacy API）
type CompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Text         string `json:"text"`
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest 聊天完成请求
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

// ChatCompletionResponse 聊天完成响应
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ==================== 带重试机制的请求方法 ====================

// doRequestWithRetry 执行HTTP请求并自动重试
// 使用指数退避策略处理429限流和5xx服务器错误
func (api *API) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	config := api.retryConfig
	if config == nil {
		config = &RetryConfig{
			MaxRetries: 3,
			BackoffStrategy: &ExponentialBackoff{
				BaseDelay: 1 * time.Second,
				MaxDelay:  30 * time.Second,
				Jitter:    true,
			},
		}
	}

	// 应用API限流控制（根据API类型选择限流器）
	if api.isLocalModel {
		getLocalRateLimiter().Acquire()
	} else {
		getRemoteRateLimiter().Acquire()
	}

	for retry := 0; retry <= config.MaxRetries; retry++ {
		if retry > 0 {
			// 计算退避延迟
			delay := config.BackoffStrategy.Next(retry - 1)
			logging.Info("api_retry_delay", map[string]interface{}{
				"retry":    retry,
				"delay_ms": delay.Milliseconds(),
				"url":      req.URL.String(),
			})
			time.Sleep(delay)
		}

		// 克隆请求以避免重试时修改原始请求
		clonedReq := req.Clone(req.Context())

		// 添加智能请求头优化
		clonedReq.Header.Set("User-Agent", "TxtCleaning/1.0")
		clonedReq.Header.Set("Content-Type", "application/json; charset=utf-8")
		clonedReq.Header.Set("Accept", "application/json")

		startTime := time.Now()
		resp, lastErr = getGlobalHTTPClient().Do(clonedReq)
		duration := time.Since(startTime).Milliseconds()

		if lastErr != nil {
			logging.Error("api_request_error", map[string]interface{}{
				"retry":    retry,
				"duration": duration,
				"error":    lastErr.Error(),
				"url":      req.URL.String(),
			})
			continue
		}

		// 检查响应状态码
		statusCode := resp.StatusCode

		// 记录请求详情
		logging.Info("api_request_completed", map[string]interface{}{
			"retry":    retry,
			"duration": duration,
			"status":   statusCode,
			"url":      req.URL.String(),
		})

		// 成功响应
		if statusCode >= 200 && statusCode < 300 {
			return resp, nil
		}

		// 读取错误响应体
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		errorBody := string(bodyBytes)
		if len(errorBody) > 500 {
			errorBody = errorBody[:500] + "..."
		}

		// 根据状态码决定是否重试
		shouldRetry := false

		switch {
		case statusCode == 429: // 限流
			shouldRetry = true
			logging.Warn("api_rate_limited", map[string]interface{}{
				"retry":     retry,
				"status":    statusCode,
				"error":     errorBody,
				"url":       req.URL.String(),
			})
			
		case statusCode >= 500: // 服务器错误
			shouldRetry = true
			logging.Error("api_server_error", map[string]interface{}{
				"retry":     retry,
				"status":    statusCode,
				"error":     errorBody,
				"url":       req.URL.String(),
			})
			
		case statusCode == 400: // 客户端错误（通常不重试）
			logging.Error("api_client_error", map[string]interface{}{
				"retry":     retry,
				"status":    statusCode,
				"error":     errorBody,
				"url":       req.URL.String(),
			})
			lastErr = fmt.Errorf("API错误 (状态码: %d): %s", statusCode, errorBody)
			
		default: // 其他错误
			logging.Error("api_other_error", map[string]interface{}{
				"retry":     retry,
				"status":    statusCode,
				"error":     errorBody,
				"url":       req.URL.String(),
			})
			lastErr = fmt.Errorf("API错误 (状态码: %d): %s", statusCode, errorBody)
		}

		// 如果不应该重试，立即返回错误
		if !shouldRetry {
			return nil, lastErr
		}

		// 如果是最后一次重试，返回错误
		if retry == config.MaxRetries {
			return nil, fmt.Errorf("API请求失败，重试%d次后仍然失败: %v", config.MaxRetries, lastErr)
		}
	}

	return nil, lastErr
}

// doJSONRequestWithRetry 执行JSON请求并自动重试（高级封装）
func (api *API) doJSONRequestWithRetry(url string, requestData interface{}) (*http.Response, error) {
	data, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求数据失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+api.apiKey)

	return api.doRequestWithRetry(req)
}

// ==================== 更新现有API方法 ====================

// GenerateEmbedding 生成文本嵌入（带重试机制）
func (api *API) GenerateEmbedding(texts []string) (*EmbeddingResponse, error) {
	if api.baseURL == "" || api.apiKey == "" {
		logging.Warn("api_disabled", map[string]interface{}{
			"service": "embedding",
			"reason":  "API未配置",
		})
		return nil, nil
	}

	url := api.baseURL + "/embeddings"
	startTime := time.Now()

	req := EmbeddingRequest{
		Input: texts,
		Model: api.embeddingModelName,
	}

	resp, err := api.doJSONRequestWithRetry(url, req)
	if err != nil {
		logging.Error("embedding_api_failed", map[string]interface{}{
			"url":        url,
			"model":      api.embeddingModelName,
			"text_count": len(texts),
			"duration":   time.Since(startTime).Milliseconds(),
			"error":      err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErrResp APIErrorResponse
		json.NewDecoder(resp.Body).Decode(&apiErrResp)
		err := &APIError{
			StatusCode: resp.StatusCode,
			Message:    apiErrResp.Error.Message,
		}
		logging.Error("embedding_api_error", map[string]interface{}{
			"url":      url,
			"model":    api.embeddingModelName,
			"status":   resp.StatusCode,
			"duration": time.Since(startTime).Milliseconds(),
			"error":    apiErrResp.Error.Message,
		})
		return nil, err
	}

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		logging.Error("embedding_decode_failed", map[string]interface{}{
			"url":      url,
			"model":    api.embeddingModelName,
			"duration": time.Since(startTime).Milliseconds(),
			"error":    err.Error(),
		})
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	logging.Info("embedding_api_success", map[string]interface{}{
		"url":        url,
		"model":      api.embeddingModelName,
		"text_count": len(texts),
		"duration":   time.Since(startTime).Milliseconds(),
		"tokens":     embeddingResp.Usage.TotalTokens,
	})

	return &embeddingResp, nil
}

// GenerateCompletion 生成文本完成（Legacy API，带重试机制）
func (api *API) GenerateCompletion(prompt string, maxTokens int, temperature float64) (*CompletionResponse, error) {
	if api.baseURL == "" || api.apiKey == "" {
		return nil, nil
	}

	if maxTokens <= 0 {
		maxTokens = api.completionMaxTokens
	}
	if temperature < 0 {
		temperature = api.completionTemperature
	}

	url := api.baseURL + "/completions"

	req := CompletionRequest{
		Model:       api.completionModelName,
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	resp, err := api.doJSONRequestWithRetry(url, req)
	if err != nil {
		logging.Error("completion_api_failed", map[string]interface{}{
			"url":      url,
			"model":    api.completionModelName,
			"duration": 0,
			"error":    err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErrResp APIErrorResponse
		json.NewDecoder(resp.Body).Decode(&apiErrResp)
		err := &APIError{
			StatusCode: resp.StatusCode,
			Message:    apiErrResp.Error.Message,
		}
		logging.Error("completion_api_error", map[string]interface{}{
			"url":    url,
			"model":  api.completionModelName,
			"status": resp.StatusCode,
			"error":  apiErrResp.Error.Message,
		})
		return nil, err
	}

	var completionResp CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		logging.Error("completion_decode_failed", map[string]interface{}{
			"url":   url,
			"model": api.completionModelName,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	logging.Info("completion_api_success", map[string]interface{}{
		"url":    url,
		"model":  api.completionModelName,
		"tokens": completionResp.Usage.TotalTokens,
	})

	return &completionResp, nil
}

// GenerateChatCompletion 生成聊天完成（Chat API，推荐，带重试机制）
func (api *API) GenerateChatCompletion(systemPrompt, userPrompt string, maxTokens int, temperature float64) (*ChatCompletionResponse, error) {
	if api.baseURL == "" || api.apiKey == "" {
		return nil, fmt.Errorf("API未配置")
	}

	if maxTokens <= 0 {
		maxTokens = api.completionMaxTokens
	}
	if temperature < 0 {
		temperature = api.completionTemperature
	}

	url := api.baseURL + "/chat/completions"
	startTime := time.Now()

	messages := []ChatMessage{}
	if systemPrompt != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, ChatMessage{Role: "user", Content: userPrompt})

	req := ChatCompletionRequest{
		Model:       api.completionModelName,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	resp, err := api.doJSONRequestWithRetry(url, req)
	if err != nil {
		logging.Error("chat_completion_api_failed", map[string]interface{}{
			"url":         url,
			"model":       api.completionModelName,
			"input_len":   len(userPrompt),
			"duration":    time.Since(startTime).Milliseconds(),
			"error":       err.Error(),
			"retry_count": 0,
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErrResp APIErrorResponse
		json.NewDecoder(resp.Body).Decode(&apiErrResp)
		err := &APIError{
			StatusCode: resp.StatusCode,
			Message:    apiErrResp.Error.Message,
		}

		// 记录API拒绝错误（用于Evolver分析）
		if resp.StatusCode == 400 || resp.StatusCode == 429 {
			logging.APIRefusal(0, "v1", userPrompt[:min(100, len(userPrompt))], apiErrResp.Error.Message)
		}

		logging.Error("chat_completion_api_error", map[string]interface{}{
			"url":        url,
			"model":      api.completionModelName,
			"status":     resp.StatusCode,
			"duration":   time.Since(startTime).Milliseconds(),
			"error":      apiErrResp.Error.Message,
			"error_type": "api_error",
		})
		return nil, err
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		logging.Error("chat_completion_decode_failed", map[string]interface{}{
			"url":      url,
			"model":    api.completionModelName,
			"duration": time.Since(startTime).Milliseconds(),
			"error":    err.Error(),
		})
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	logging.Info("chat_completion_api_success", map[string]interface{}{
		"url":        url,
		"model":      api.completionModelName,
		"input_len":  len(userPrompt),
		"output_len": len(chatResp.Choices[0].Message.Content),
		"tokens":     chatResp.Usage.TotalTokens,
		"duration":   time.Since(startTime).Milliseconds(),
	})

	return &chatResp, nil
}

// CorrectText 使用外部模型纠正文本（带重试和缓存）
func (api *API) CorrectText(text string) (string, error) {
	systemPrompt := "你是一个专业的中文小说校对编辑。请纠正以下文本中的错别字、语法错误和乱码，保持原文的意思不变。只输出修正后的文本，无需解释。"
	userPrompt := text

	resp, err := api.GenerateChatCompletion(systemPrompt, userPrompt, api.completionMaxTokens, api.completionTemperature)
	if err != nil {
		return text, err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return text, nil
}

// GenerateSummary 生成文本摘要（带重试）
func (api *API) GenerateSummary(text string) (string, error) {
	systemPrompt := "你是一个专业的文本摘要生成器。请为以下文本生成一个简洁的摘要。"
	userPrompt := text

	resp, err := api.GenerateChatCompletion(systemPrompt, userPrompt, api.completionMaxTokens, api.completionTemperature)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return "", nil
}

// SetRetryConfig 设置重试配置（线程安全）
func (api *API) SetRetryConfig(config *RetryConfig) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.retryConfig = config
}

// GetRetryConfig 获取重试配置（线程安全）
func (api *API) GetRetryConfig() *RetryConfig {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.retryConfig
}

// UpdateBaseURL 更新API基础URL（线程安全）
func (api *API) UpdateBaseURL(baseURL string) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.baseURL = baseURL
}

// UpdateAPIKey 更新API密钥（线程安全）
func (api *API) UpdateAPIKey(apiKey string) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.apiKey = apiKey
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
