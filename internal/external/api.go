package external

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"txt-cleaning/internal/config"
)

// API 外部API客户端
type API struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// NewAPI 创建新的API客户端
func NewAPI() *API {
	return &API{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: config.AppConfig.ExternalAPIURL,
		apiKey:  config.AppConfig.ExternalAPIKey,
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
		PromptTokens     int `json:"prompt_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// CompletionRequest 完成请求
type CompletionRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	MaxTokens   int    `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// CompletionResponse 完成响应
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

// GenerateEmbedding 生成文本嵌入
func (api *API) GenerateEmbedding(texts []string) (*EmbeddingResponse, error) {
	if api.baseURL == "" || api.apiKey == "" {
		return nil, nil
	}

	url := api.baseURL + "/embeddings"

	req := EmbeddingRequest{
		Input: texts,
		Model: "text-embedding-ada-002",
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

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, err
	}

	return &embeddingResp, nil
}

// GenerateCompletion 生成文本完成
func (api *API) GenerateCompletion(prompt string, maxTokens int, temperature float64) (*CompletionResponse, error) {
	if api.baseURL == "" || api.apiKey == "" {
		return nil, nil
	}

	url := api.baseURL + "/completions"

	req := CompletionRequest{
		Model:       "gpt-3.5-turbo-instruct",
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

	var completionResp CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return nil, err
	}

	return &completionResp, nil
}

// CorrectText 使用外部模型纠正文本
func (api *API) CorrectText(text string) (string, error) {
	prompt := "请纠正以下文本中的错别字、语法错误和乱码，保持原文的意思不变：\n" + text

	resp, err := api.GenerateCompletion(prompt, 1000, 0.3)
	if err != nil {
		return text, err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Text, nil
	}

	return text, nil
}

// GenerateSummary 生成文本摘要
func (api *API) GenerateSummary(text string) (string, error) {
	prompt := "请为以下文本生成一个简洁的摘要：\n" + text

	resp, err := api.GenerateCompletion(prompt, 500, 0.5)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Text, nil
	}

	return "", nil
}