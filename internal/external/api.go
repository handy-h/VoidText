package external

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"txt-cleaning/internal/config"
)

// API 外部API客户端
type API struct {
	client                *http.Client
	baseURL               string
	apiKey                string
	embeddingModelName    string
	completionModelName   string
	completionTemperature float64
	completionMaxTokens   int
}

// NewAPI 创建新的API客户端
func NewAPI() *API {
	return &API{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL:               config.AppConfigInstance.LLMApiURL,
		apiKey:                config.AppConfigInstance.LLMApiKey,
		embeddingModelName:    config.AppConfigInstance.VectorModelName,
		completionModelName:   config.AppConfigInstance.CompletionModelName,
		completionTemperature: config.AppConfigInstance.CompletionTemperature,
		completionMaxTokens:   config.AppConfigInstance.CompletionMaxTokens,
	}
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

// APIError API错误响应
type APIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// GenerateEmbedding 生成文本嵌入
func (api *API) GenerateEmbedding(texts []string) (*EmbeddingResponse, error) {
	if api.baseURL == "" || api.apiKey == "" {
		log.Printf("[向量模型] API未配置，跳过调用")
		return nil, nil
	}

	url := api.baseURL + "/embeddings"
	startTime := time.Now()

	req := EmbeddingRequest{
		Input: texts,
		Model: api.embeddingModelName,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+api.apiKey)

	resp, err := api.client.Do(httpReq)
	if err != nil {
		log.Printf("[向量模型] 调用失败: URL=%s, Model=%s, 耗时=%v, 错误=%v",
			url, api.embeddingModelName, time.Since(startTime), err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		json.NewDecoder(resp.Body).Decode(&apiErr)
		log.Printf("[向量模型] 调用失败: URL=%s, Model=%s, HTTP状态=%d, 耗时=%v, 错误=%s",
			url, api.embeddingModelName, resp.StatusCode, time.Since(startTime), apiErr.Error.Message)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
	}

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, err
	}

	log.Printf("[向量模型] 调用成功: URL=%s, Model=%s, 文本数=%d, 耗时=%v",
		url, api.embeddingModelName, len(texts), time.Since(startTime))

	return &embeddingResp, nil
}

// GenerateCompletion 生成文本完成（Legacy API）
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

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+api.apiKey)

	resp, err := api.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		json.NewDecoder(resp.Body).Decode(&apiErr)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
	}

	var completionResp CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return nil, err
	}

	return &completionResp, nil
}

// GenerateChatCompletion 生成聊天完成（Chat API，推荐）
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

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+api.apiKey)

	resp, err := api.client.Do(httpReq)
	if err != nil {
		log.Printf("[文本修复] 调用失败: URL=%s, Model=%s, 耗时=%v, 错误=%v",
			url, api.completionModelName, time.Since(startTime), err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		json.NewDecoder(resp.Body).Decode(&apiErr)
		log.Printf("[文本修复] 调用失败: URL=%s, Model=%s, HTTP状态=%d, 耗时=%v, 错误=%s",
			url, api.completionModelName, resp.StatusCode, time.Since(startTime), apiErr.Error.Message)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	log.Printf("[文本修复] 调用成功: URL=%s, Model=%s, 输入长度=%d, 输出长度=%d, Token=%d, 耗时=%v",
		url, api.completionModelName, len(userPrompt), len(chatResp.Choices[0].Message.Content),
		chatResp.Usage.TotalTokens, time.Since(startTime))

	return &chatResp, nil
}

// CorrectText 使用外部模型纠正文本
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

// GenerateSummary 生成文本摘要
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
