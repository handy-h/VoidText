package external

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"voidtext/internal/logging"
)

// OllamaClient Ollama客户端
type OllamaClient struct {
	baseURL       string
	modelName     string
	timeout       time.Duration
	httpClient    *http.Client // 用于生成请求（长超时）
	healthClient  *http.Client // 用于健康检查（短超时）
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

// OllamaGenerateResponse Ollama生成响应（流式每个chunk和最终响应共用）
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
		timeout = 180 * time.Second
	}

	transport := &http.Transport{
		MaxIdleConnsPerHost: 100,
		MaxIdleConns:        200,
		MaxConnsPerHost:     200,
		IdleConnTimeout:     90 * time.Second,
	}

	return &OllamaClient{
		baseURL:   baseURL,
		modelName: modelName,
		timeout:   timeout,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		// 健康检查使用独立的短超时客户端，避免与生成请求共用超时
		healthClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

// Generate 生成文本（使用流式模式避免HTTP超时）
func (oc *OllamaClient) Generate(prompt, systemPrompt string) (string, error) {
	startTime := time.Now()

	reqBody := OllamaGenerateRequest{
		Model:      oc.modelName,
		Prompt:     prompt,
		Stream:     true, // 使用流式模式，避免长时间无数据导致HTTP超时
		KeepAlive:  "5m", // 保持模型在内存中5分钟，避免冷启动延迟
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
		logging.Error("ollama_request_marshal_failed", nil, map[string]interface{}{
			"model": oc.modelName,
			"error": err.Error(),
		})
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	url := oc.baseURL + "/api/generate"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logging.Error("ollama_request_create_failed", nil, map[string]interface{}{
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
		logging.Error("ollama_request_failed", nil, map[string]interface{}{
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

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read error response: %w", err)
		}

		var errResp OllamaErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != "" {
			duration := time.Since(startTime).Milliseconds()
			logging.Error("ollama_api_error", nil, map[string]interface{}{
				"url":      url,
				"model":    oc.modelName,
				"status":   resp.StatusCode,
				"duration": duration,
				"error":    errResp.Error,
				"source":   "local",
			})
			return "", fmt.Errorf("ollama API error: %s", errResp.Error)
		}

		duration := time.Since(startTime).Milliseconds()
		logging.Error("ollama_http_error", nil, map[string]interface{}{
			"url":      url,
			"model":    oc.modelName,
			"status":   resp.StatusCode,
			"duration": duration,
			"source":   "local",
		})
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// 流式读取响应：逐行解析JSON，拼接response字段
	var fullResponse strings.Builder
	var finalResp OllamaGenerateResponse

	scanner := bufio.NewScanner(resp.Body)
	// 增大scanner缓冲区以处理长行
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk OllamaGenerateResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			// 跳过无法解析的行，但记录警告
			logging.Warn("ollama_stream_parse_skip", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		// 拼接响应文本
		if chunk.Response != "" {
			fullResponse.WriteString(chunk.Response)
		}

		// 流式响应的最后一个chunk包含完整统计信息
		if chunk.Done {
			finalResp = chunk
			break
		}
	}

	if err := scanner.Err(); err != nil {
		duration := time.Since(startTime).Milliseconds()
		logging.Error("ollama_stream_read_failed", nil, map[string]interface{}{
			"url":      url,
			"model":    oc.modelName,
			"duration": duration,
			"error":    err.Error(),
			"source":   "local",
		})
		return "", fmt.Errorf("流式响应读取失败: %w", err)
	}

	duration := time.Since(startTime).Milliseconds()
	resultText := fullResponse.String()

	// 记录成功日志
	logging.Info("ollama_generate_success", map[string]interface{}{
		"url":                url,
		"model":              oc.modelName,
		"duration":           duration,
		"total_duration":     finalResp.TotalDuration,
		"prompt_eval_count":  finalResp.PromptEvalCount,
		"eval_count":         finalResp.EvalCount,
		"response_len":       len(resultText),
		"source":             "local",
		"stream":             true,
	})

	return resultText, nil
}

// HealthCheck 健康检查（使用独立的短超时客户端）
func (oc *OllamaClient) HealthCheck() bool {
	startTime := time.Now()
	url := oc.baseURL + "/api/tags"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	// 使用独立的短超时客户端，避免与生成请求共用超时
	resp, err := oc.healthClient.Do(req)
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
	systemPrompt := `你是一个专业的中文小说校对编辑。请严格遵循以下规则：
1. 只纠正文本中的错别字、语法错误和乱码
2. 保持原文的意思、风格和长度不变
3. 严禁修改任何专有名词：人物姓名（如圣骑士、尸体发火、亚马逊等）、地名（如萝格营地、邪恶洞窟等）、物品名称（如黄宝石、片刀等）、技能名称等
4. 不要添加任何解释、评论或额外内容
5. 不要重复或扩展原文
6. 输出必须与输入长度相近，不要大幅增加或减少文本长度
7. 只输出修正后的文本，不要有任何前缀或后缀`

	userPrompt := "请纠正以下文本中的错别字和语法错误。注意：人物姓名、地名、物品名称等专有名词一律不可修改，保持原样。只输出修正后的文本：\n" + text + "\n\n修正后的文本："

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
