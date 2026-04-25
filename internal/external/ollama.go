package external

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"voidtext/internal/logging"
)

// OllamaClient Ollama客户端
type OllamaClient struct {
	baseURL    string
	modelName  string
	timeout    time.Duration
	httpClient *http.Client
}

// OllamaGenerateRequest Ollama生成请求
type OllamaGenerateRequest struct {
	Model     string  `json:"model"`
	Prompt    string  `json:"prompt"`
	Stream    bool    `json:"stream"`
	Options   Options `json:"options"`
	System    string  `json:"system,omitempty"`
	Template  string  `json:"template,omitempty"`
	Context   []int   `json:"context,omitempty"`
	KeepAlive string  `json:"keep_alive,omitempty"`
}

// Options 模型选项
type Options struct {
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	RepeatPenalty float64 `json:"repeat_penalty,omitempty"`
	Seed        int     `json:"seed,omitempty"`
}

// OllamaGenerateResponse Ollama生成响应
type OllamaGenerateResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	Context            []int     `json:"context,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`
	LoadDuration       int64     `json:"load_duration,omitempty"`
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
}

// OllamaErrorResponse Ollama错误响应
type OllamaErrorResponse struct {
	Error string `json:"error"`
}

// NewOllamaClient 创建Ollama客户端
func NewOllamaClient(baseURL, modelName string, timeout time.Duration) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if modelName == "" {
		modelName = "qwen2.5:7b-instruct-q4_K_M"
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &OllamaClient{
		baseURL:   baseURL,
		modelName: modelName,
		timeout:   timeout,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100,
				MaxIdleConns:        200,
				MaxConnsPerHost:     200,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Generate 生成文本
func (oc *OllamaClient) Generate(prompt, systemPrompt string) (string, error) {
	startTime := time.Now()
	
	reqBody := OllamaGenerateRequest{
		Model:  oc.modelName,
		Prompt: prompt,
		Stream: false,
		Options: Options{
			Temperature: 0.3,
			TopP:        0.9,
			TopK:        40,
		},
	}

	if systemPrompt != "" {
		reqBody.System = systemPrompt
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		logging.Error("ollama_request_marshal_failed", map[string]interface{}{
			"model": oc.modelName,
			"error": err.Error(),
		})
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	url := oc.baseURL + "/api/generate"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logging.Error("ollama_request_create_failed", map[string]interface{}{
			"url":   url,
			"model": oc.modelName,
			"error": err.Error(),
		})
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := oc.httpClient.Do(req)
	if err != nil {
		duration := time.Since(startTime).Milliseconds()
		logging.Error("ollama_request_failed", map[string]interface{}{
			"url":      url,
			"model":    oc.modelName,
			"duration": duration,
			"error":    err.Error(),
			"source":   "local",
		})
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	duration := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read error response: %w", err)
		}
		
		var errResp OllamaErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != "" {
			logging.Error("ollama_api_error", map[string]interface{}{
				"url":      url,
				"model":    oc.modelName,
				"status":   resp.StatusCode,
				"duration": duration,
				"error":    errResp.Error,
				"source":   "local",
			})
			return "", fmt.Errorf("ollama API error: %s", errResp.Error)
		}

		logging.Error("ollama_http_error", map[string]interface{}{
			"url":      url,
			"model":    oc.modelName,
			"status":   resp.StatusCode,
			"duration": duration,
			"source":   "local",
		})
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	var genResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		logging.Error("ollama_response_decode_failed", map[string]interface{}{
			"url":      url,
			"model":    oc.modelName,
			"duration": duration,
			"error":    err.Error(),
			"source":   "local",
		})
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 记录成功日志
	logging.Info("ollama_generate_success", map[string]interface{}{
		"url":                url,
		"model":              oc.modelName,
		"duration":           duration,
		"total_duration":     genResp.TotalDuration,
		"prompt_eval_count":  genResp.PromptEvalCount,
		"eval_count":         genResp.EvalCount,
		"response_len":       len(genResp.Response),
		"source":            "local",
	})

	return genResp.Response, nil
}

// HealthCheck 健康检查
func (oc *OllamaClient) HealthCheck() bool {
	startTime := time.Now()
	url := oc.baseURL + "/api/tags" // 使用tags端点检查服务状态

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := oc.httpClient.Do(req)
	if err != nil {
		logging.Warn("ollama_health_check_failed", map[string]interface{}{
			"url":      url,
			"duration": time.Since(startTime).Milliseconds(),
			"error":    err.Error(),
		})
		return false
	}
	defer resp.Body.Close()

	duration := time.Since(startTime).Milliseconds()
	isHealthy := resp.StatusCode == http.StatusOK

	logging.Info("ollama_health_check", map[string]interface{}{
		"url":      url,
		"status":   resp.StatusCode,
		"healthy":  isHealthy,
		"duration": duration,
	})

	return isHealthy
}

// CorrectText 纠正文本（Ollama专用）
func (oc *OllamaClient) CorrectText(text string) (string, error) {
	systemPrompt := `你是一个专业的中文小说校对编辑。请纠正以下文本中的错别字、语法错误和乱码，保持原文的意思不变。只输出修正后的文本，无需解释。`
	
	// 优化提示词格式
	userPrompt := "请纠正以下文本中的错别字和语法错误：\n" + text + "\n\n修正后的文本："
	
	return oc.Generate(userPrompt, systemPrompt)
}

// GetModelName 获取模型名称
func (oc *OllamaClient) GetModelName() string {
	return oc.modelName
}

// GetBaseURL 获取基础URL
func (oc *OllamaClient) GetBaseURL() string {
	return oc.baseURL
}