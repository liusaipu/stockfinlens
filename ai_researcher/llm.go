package ai_researcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMClient 通用 OpenAI-compatible LLM 客户端
type LLMClient struct {
	apiKey     string
	baseURL    string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest 请求体
type ChatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []ChatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	TopP           float64         `json:"top_p,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat JSON 输出格式
type ResponseFormat struct {
	Type string `json:"type"`
}

// ChatCompletionResponse 响应体
type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// NewLLMClient 创建 LLM 客户端
func NewLLMClient(apiKey, baseURL, model string, timeoutSeconds int) *LLMClient {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 90
	}
	return &LLMClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		timeout:    time.Duration(timeoutSeconds) * time.Second,
		httpClient: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

// Complete 发送对话请求并返回文本内容
func (c *LLMClient) Complete(systemPrompt, userPrompt string, temperature float64, maxTokens int, topP float64, forceJSON bool) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("LLM API Key 为空")
	}
	if c.baseURL == "" {
		return "", fmt.Errorf("LLM Base URL 为空")
	}
	if c.model == "" {
		return "", fmt.Errorf("LLM 模型为空")
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := ChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		TopP:        topP,
	}
	if forceJSON {
		reqBody.ResponseFormat = &ResponseFormat{Type: "json_object"}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("构造 LLM 请求失败: %w", err)
	}

	url := c.baseURL
	if url[len(url)-1:] != "/" {
		url += "/"
	}
	url += "chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建 LLM 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 LLM 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return "", fmt.Errorf("LLM API 错误: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM 返回空 choices")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// Test 测试 LLM 连接是否可用
func (c *LLMClient) Test() error {
	_, err := c.Complete("You are a helpful assistant.", "Hi", 0.2, 10, 1.0, false)
	return err
}
