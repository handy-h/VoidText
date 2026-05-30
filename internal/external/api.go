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
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/logging"
)

// 全局HTTP客户端单例，配置连接池和超时设置
// 使用单例模式避免创建过多HTTP客户端，提高连接复用率
var (
	globalHTTPClient     *http.Client
	globalHTTPClientOnce sync.Once
)

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
		for _, status := range retryableStatuses {
			if apiErr.StatusCode == status {
				return true
			}
		}
	}
	// 网络错误、超时错误等也可重试（context.Canceled 不重试，因为表示主动取消）
	return errors.Is(err, context.DeadlineExceeded) ||
		isTimeoutError(err)
}

// isTimeoutError 判断是否为超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// 使用 net.Error 接口的 Timeout() 方法，比字符串匹配更可靠
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, http.ErrHandlerTimeout) ||
		errors.Is(err, context.DeadlineExceeded)
}

// isQuotaExhausted 检测是否为额度耗尽类错误（不可恢复，应立即切换到下一个模型）
// 匹配条件：429/403/402 状态码 + 错误消息中包含额度相关关键词
func isQuotaExhausted(statusCode int, errorMessage string) bool {
	if statusCode != 429 && statusCode != 403 && statusCode != 402 {
		return false
	}
	msg := strings.ToLower(errorMessage)
	quotaKeywords := []string{
		"insufficient_quota",
		"quota exceeded",
		"quota_exceeded",
		"exceeded_current_quota",
		"balance insufficient",
		"insufficient balance",
		"余额不足",
		"额度不足",
		"payment required",
		"billing",
	}
	for _, keyword := range quotaKeywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

// containsAny 检查字符串是否包含任意子串
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// isMaxTokensError 检测是否为 max_tokens 超出范围的错误
// 匹配条件：400 状态码 + 错误消息中包含 max_tokens 相关关键词
func isMaxTokensError(statusCode int, errorMessage string) bool {
	if statusCode != 400 {
		return false
	}
	msg := strings.ToLower(errorMessage)
	return strings.Contains(msg, "max_tokens") && containsAny(msg, []string{"range", "invalid", "exceed", "limit"})
}

// 可重试的 HTTP 状态码列表（包级常量，避免每次调用 IsRetryableError 都创建 DefaultRetryConfig）
var retryableStatuses = []int{429, 500, 502, 503, 504}

// ErrAPINotConfigured 表示 API 未配置的错误
var ErrAPINotConfigured = errors.New("API未配置：请设置相应的 URL 和 APIKey")

// QuotaExhaustedError 额度耗尽错误（上层可针对性处理，如切换端点）
type QuotaExhaustedError struct {
	StatusCode int
	Message    string
}

func (e *QuotaExhaustedError) Error() string {
	return fmt.Sprintf("额度耗尽 (状态码: %d): %s", e.StatusCode, e.Message)
}

// API 外部API客户端（重构版）
type API struct {
	endpoints             []config.ModelEndpoint // 多模型端点列表（用于 chat completion 切换）
	baseURL               string                 // 保留向后兼容（embedding / legacy completion）
	apiKey                string                 // 保留向后兼容
	embeddingModelName    string
	completionModelName   string // 默认模型名（向后兼容）
	completionTemperature float64
	completionMaxTokens   int
	maxOutputTokens       int // LLM 最大输出 token 数上限（防止 max_tokens 超出模型限制）
	retryConfig           *RetryConfig
	isLocalModel          bool // 是否为本地模型
}

// NewAPI 创建新的API客户端
func NewAPI() *API {
	cfg := config.AppConfigInstance
	return &API{
		endpoints:             cfg.ModelEndpoints,
		baseURL:               cfg.LLMApiURL,
		apiKey:                cfg.LLMApiKey,
		embeddingModelName:    cfg.VectorModelName,
		completionModelName:   cfg.CompletionModelName,
		completionTemperature: cfg.CompletionTemperature,
		completionMaxTokens:   cfg.CompletionMaxTokens,
		maxOutputTokens:       cfg.LLMMaxOutputTokens,
		retryConfig:           DefaultRetryConfig(),
		isLocalModel:          cfg.EnableLocalModel,
	}
}

// NewEmbeddingAPI 创建专用于 Embedding 的 API 客户端，使用向量检测独立配置
func NewEmbeddingAPI() *API {
	cfg := config.AppConfigInstance
	return &API{
		baseURL:            cfg.VectorModelURL,
		apiKey:             cfg.VectorModelApiKey,
		embeddingModelName: cfg.VectorModelName,
		retryConfig:        DefaultRetryConfig(),
		isLocalModel:       cfg.EnableLocalModel && cfg.VectorModelURL == cfg.LocalModelURL,
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
			RetryableStatus: retryableStatuses,
		}
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
			logging.Error("api_request_error", nil, map[string]interface{}{
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
		if runes := []rune(errorBody); len(runes) > 500 {
			errorBody = string(runes[:500]) + "..."
		}

		// 根据状态码决定是否重试
		shouldRetry := false

		switch {
		case statusCode == 429: // 限流
			// 先检测是否为额度耗尽（不可恢复），如果是则不重试，立即返回让调用方切换模型
			if isQuotaExhausted(statusCode, errorBody) {
				logging.Warn("api_quota_exhausted", map[string]interface{}{
					"status": statusCode,
					"error":  errorBody,
					"url":    req.URL.String(),
				})
				lastErr = &QuotaExhaustedError{
					StatusCode: statusCode,
					Message:    errorBody,
				}
				return nil, lastErr
			}
			shouldRetry = true
			logging.Warn("api_rate_limited", map[string]interface{}{
				"retry":  retry,
				"status": statusCode,
				"error":  errorBody,
				"url":    req.URL.String(),
			})

		case statusCode >= 500: // 服务器错误
			shouldRetry = true
			logging.Error("api_server_error", nil, map[string]interface{}{
				"retry":  retry,
				"status": statusCode,
				"error":  errorBody,
				"url":    req.URL.String(),
			})

		case statusCode == 400: // 客户端错误（通常不重试）
			logging.Error("api_client_error", nil, map[string]interface{}{
				"retry":  retry,
				"status": statusCode,
				"error":  errorBody,
				"url":    req.URL.String(),
			})
			lastErr = fmt.Errorf("API错误 (状态码: %d): %s", statusCode, errorBody)

		default: // 其他错误
			logging.Error("api_other_error", nil, map[string]interface{}{
				"retry":  retry,
				"status": statusCode,
				"error":  errorBody,
				"url":    req.URL.String(),
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

// doJSONRequestWithRetry 执行JSON请求并自动重试（高级封装，使用 API 实例的 apiKey）
func (api *API) doJSONRequestWithRetry(url string, requestData interface{}) (*http.Response, error) {
	return api.doJSONRequestWithRetryKeyed(url, requestData, api.apiKey)
}

// doJSONRequestWithRetryKeyed 执行JSON请求并自动重试（支持指定 API 密钥，用于多端点切换）
func (api *API) doJSONRequestWithRetryKeyed(url string, requestData interface{}, apiKey string) (*http.Response, error) {
	data, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求数据失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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
		return nil, ErrAPINotConfigured
	}

	if len(texts) == 0 {
		return &EmbeddingResponse{
			Data: []struct {
				Object    string    `json:"object"`
				Index     int       `json:"index"`
				Embedding []float64 `json:"embedding"`
			}{},
		}, nil
	}

	const embeddingBatchSize = 25
	if len(texts) > embeddingBatchSize {
		return api.generateEmbeddingBatches(texts, embeddingBatchSize)
	}

	// Ollama 本地模型的 embeddings 端点需要 /api 前缀
	url := api.baseURL + "/embeddings"
	if api.isLocalModel {
		url = api.baseURL + "/api/embeddings"
	}
	startTime := time.Now()

	req := EmbeddingRequest{
		Input: texts,
		Model: api.embeddingModelName,
	}

	resp, err := api.doJSONRequestWithRetry(url, req)
	if err != nil {
		logging.Error("embedding_api_failed", nil, map[string]interface{}{
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
		logging.Error("embedding_api_error", nil, map[string]interface{}{
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
		logging.Error("embedding_decode_failed", nil, map[string]interface{}{
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

// generateEmbeddingBatches 分批生成文本嵌入，避免单次请求超过服务端限制
func (api *API) generateEmbeddingBatches(texts []string, batchSize int) (*EmbeddingResponse, error) {
	combined := &EmbeddingResponse{
		Data: make([]struct {
			Object    string    `json:"object"`
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		}, len(texts)),
	}

	totalTokens := 0
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[start:end]
		resp, err := api.GenerateEmbedding(batch)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("embedding API 返回空结果")
		}
		totalTokens += resp.Usage.TotalTokens

		for _, item := range resp.Data {
			if item.Index < 0 || item.Index >= len(batch) {
				return nil, fmt.Errorf("embedding API 返回索引越界: %d", item.Index)
			}
			combined.Data[start+item.Index] = item
		}
	}

	combined.Usage.TotalTokens = totalTokens
	return combined, nil
}

// GenerateCompletion 生成文本完成（Legacy API，带重试机制）
func (api *API) GenerateCompletion(prompt string, maxTokens int, temperature float64) (*CompletionResponse, error) {
	if api.baseURL == "" || api.apiKey == "" {
		return nil, ErrAPINotConfigured
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
		logging.Error("completion_api_failed", nil, map[string]interface{}{
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
		logging.Error("completion_api_error", nil, map[string]interface{}{
			"url":    url,
			"model":  api.completionModelName,
			"status": resp.StatusCode,
			"error":  apiErrResp.Error.Message,
		})
		return nil, err
	}

	var completionResp CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		logging.Error("completion_decode_failed", nil, map[string]interface{}{
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

// GenerateChatCompletion 生成聊天完成（Chat API，支持多端点自动切换）
// 按端点列表顺序依次尝试，每个端点内部自带 HTTP 级重试；
// 一个端点完全失败后自动切换到下一个，直到成功或所有端点均失败。
func (api *API) GenerateChatCompletion(systemPrompt, userPrompt string, maxTokens int, temperature float64) (*ChatCompletionResponse, error) {
	// 兼容旧逻辑：如果多端点列表为空，回退到单端点模式
	endpointList := api.endpoints
	if len(endpointList) == 0 {
		if api.baseURL == "" || api.apiKey == "" {
			return nil, fmt.Errorf("API未配置：请设置 LLM_API_URL 和 LLM_API_KEY")
		}
		endpointList = []config.ModelEndpoint{{
			URL:       api.baseURL,
			APIKey:    api.apiKey,
			ModelName: api.completionModelName,
		}}
	}

	if maxTokens <= 0 {
		maxTokens = api.completionMaxTokens
	}
	// 安全兜底：确保 maxTokens 不超过配置上限（防止调用方传入过大值）
	if api.maxOutputTokens > 0 && maxTokens > api.maxOutputTokens {
		logging.Warn("chat_completion_max_tokens_capped", map[string]interface{}{
			"requested": maxTokens,
			"capped_to": api.maxOutputTokens,
		})
		maxTokens = api.maxOutputTokens
	}
	if temperature < 0 {
		temperature = api.completionTemperature
	}

	messages := []ChatMessage{}
	if systemPrompt != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, ChatMessage{Role: "user", Content: userPrompt})

	var lastErr error
	totalEndpoints := len(endpointList)

	for idx, endpoint := range endpointList {
		if endpoint.URL == "" || endpoint.APIKey == "" {
			continue
		}

		url := endpoint.URL + "/chat/completions"
		startTime := time.Now()
		endpointNum := idx + 1
		currentMaxTokens := maxTokens

		// 内层循环：max_tokens 超限时自动减半重试（最多 3 次）
		for maxTokensRetry := 0; maxTokensRetry <= 3; maxTokensRetry++ {
			req := ChatCompletionRequest{
				Model:       endpoint.ModelName,
				Messages:    messages,
				MaxTokens:   currentMaxTokens,
				Temperature: temperature,
			}

			resp, err := api.doJSONRequestWithRetryKeyed(url, req, endpoint.APIKey)
			if err != nil {
				// HTTP 级重试已用尽，记录并尝试下一个端点
				logging.Error("chat_completion_api_failed", nil, map[string]interface{}{
					"url":          url,
					"model":        endpoint.ModelName,
					"endpoint_num": endpointNum,
					"input_len":    len(userPrompt),
					"duration":     time.Since(startTime).Milliseconds(),
					"error":        err.Error(),
				})
				lastErr = err
				break // 跳出内层循环，尝试下一个端点
			}

			// resp 非 nil 说明 HTTP 200，解析响应
			if resp.StatusCode != http.StatusOK {
				var apiErrResp APIErrorResponse
				json.NewDecoder(resp.Body).Decode(&apiErrResp)
				resp.Body.Close()

				// max_tokens 超限错误：减半后重试当前端点
				if isMaxTokensError(resp.StatusCode, apiErrResp.Error.Message) && maxTokensRetry < 3 {
					newMaxTokens := currentMaxTokens / 2
					if newMaxTokens < 256 {
						newMaxTokens = 256 // 最低保底值，避免无限缩减
					}
					logging.Warn("chat_completion_max_tokens_retry", map[string]interface{}{
						"url":            url,
						"model":          endpoint.ModelName,
						"endpoint_num":   endpointNum,
						"old_max_tokens": currentMaxTokens,
						"new_max_tokens": newMaxTokens,
						"retry":          maxTokensRetry + 1,
						"error":          apiErrResp.Error.Message,
					})
					currentMaxTokens = newMaxTokens
					continue // 减半后重试
				}

				err := &APIError{
					StatusCode: resp.StatusCode,
					Message:    apiErrResp.Error.Message,
				}

				// 记录API拒绝错误（用于Evolver分析）
				if resp.StatusCode == 400 || resp.StatusCode == 429 {
					logging.APIRefusal(0, "v1", userPrompt[:min(100, len(userPrompt))], apiErrResp.Error.Message)
				}

				logging.Error("chat_completion_api_error_status", nil, map[string]interface{}{
					"url":          url,
					"model":        endpoint.ModelName,
					"endpoint_num": endpointNum,
					"status":       resp.StatusCode,
					"duration":     time.Since(startTime).Milliseconds(),
					"error":        apiErrResp.Error.Message,
				})
				lastErr = err
				break // 非 max_tokens 错误，跳出内层循环，尝试下一个端点
			}

			var chatResp ChatCompletionResponse
			decodeErr := json.NewDecoder(resp.Body).Decode(&chatResp)
			resp.Body.Close()

			if decodeErr != nil {
				logging.Error("chat_completion_decode_failed", nil, map[string]interface{}{
					"url":          url,
					"model":        endpoint.ModelName,
					"endpoint_num": endpointNum,
					"duration":     time.Since(startTime).Milliseconds(),
					"error":        decodeErr.Error(),
				})
				lastErr = fmt.Errorf("解析响应失败: %w", decodeErr)
				break // 跳出内层循环，尝试下一个端点
			}

			// 响应为空或无效
			if len(chatResp.Choices) == 0 {
				logging.Warn("chat_completion_empty_choices", map[string]interface{}{
					"url":          url,
					"model":        endpoint.ModelName,
					"endpoint_num": endpointNum,
					"duration":     time.Since(startTime).Milliseconds(),
				})
				lastErr = fmt.Errorf("模型 %s 返回空结果", endpoint.ModelName)
				break // 跳出内层循环，尝试下一个端点
			}

			// 成功
			logging.Info("chat_completion_api_success", map[string]interface{}{
				"url":            url,
				"model":          endpoint.ModelName,
				"endpoint_num":   endpointNum,
				"input_len":      len(userPrompt),
				"output_len":     len(chatResp.Choices[0].Message.Content),
				"tokens":         chatResp.Usage.TotalTokens,
				"max_tokens_req": currentMaxTokens,
				"duration":       time.Since(startTime).Milliseconds(),
			})

			return &chatResp, nil
		} // end maxTokensRetry loop

		// 内层循环结束（全部失败），记录回退日志，继续尝试下一个端点
		api.tryFallback(idx, totalEndpoints, endpointNum, fmt.Sprintf("端点 %d 最终失败", endpointNum))
	} // end endpoint loop

	return nil, fmt.Errorf("所有%d个模型端点均已尝试失败，最后错误: %w", totalEndpoints, lastErr)
}

// tryFallback 记录从当前端点切换到下一个端点的日志
func (api *API) tryFallback(currentIdx, totalEndpoints, endpointNum int, reason string) {
	if currentIdx < totalEndpoints-1 {
		logging.Info("chat_completion_fallback", map[string]interface{}{
			"from_endpoint": endpointNum,
			"to_endpoint":   endpointNum + 1,
			"reason":        reason,
		})
	}
}

// CorrectText 使用外部模型纠正文本（带重试和缓存）
func (api *API) CorrectText(text string) (string, error) {
	systemPrompt := "你是一个专业的中文小说校对编辑。请严格遵循以下规则：1. 只纠正错别字、语法错误和乱码；2. 保持原文意思、风格和长度；3. 严禁修改人物姓名、地名、物品名称等任何专有名词；4. 只输出修正后的文本，不要任何解释。"
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
