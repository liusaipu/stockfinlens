package ai_researcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	// Thinking 控制 Kimi K2 系列是否开启深度思考；仅对部分模型有效。
	// 投研报告生成不需要 reasoning tokens，关闭可让模型把结果输出到 content。
	Thinking *ThinkingControl `json:"thinking,omitempty"`
}

// ThinkingControl Kimi / Moonshot 的 thinking 参数
type ThinkingControl struct {
	Type string `json:"type"` // "enabled" | "disabled"
}

// ResponseFormat JSON 输出格式
type ResponseFormat struct {
	Type string `json:"type"`
}

// ChatCompletionResponse 响应体
type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// normalizeTemperature 根据模型约束返回允许的 temperature。
// - Kimi K2.5/K2.6：关闭 thinking，强制 0.6。
// - Kimi K2.7-code/highspeed：强制 1.0。
// - DeepSeek V4：官方推荐 1.0。
func normalizeTemperature(model string, temp float64) float64 {
	switch model {
	case "kimi-k2.5", "kimi-k2.6":
		return 0.6
	case "kimi-k2.7-code", "kimi-k2.7-code-highspeed", "deepseek-v4-pro", "deepseek-v4-flash":
		return 1.0
	}
	return temp
}

// normalizeTopP 根据模型约束返回允许的 top_p。
// - Kimi K2 系列：强制 0.95。
// - DeepSeek V4：官方推荐 1.0。
func normalizeTopP(model string, topP float64) float64 {
	switch model {
	case "kimi-k2.5", "kimi-k2.6", "kimi-k2.7-code", "kimi-k2.7-code-highspeed":
		return 0.95
	case "deepseek-v4-pro", "deepseek-v4-flash":
		return 1.0
	}
	return topP
}

// disableThinkingForModel 判断是否需要关闭 thinking 的模型。
// 这些模型默认开启 thinking，会导致 JSON 输出时 content 为空，关闭后可把结果写到 content。
func disableThinkingForModel(model string) bool {
	switch model {
	case "kimi-k2.5", "kimi-k2.6", "deepseek-v4-pro", "deepseek-v4-flash":
		return true
	}
	return false
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
func (c *LLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string, temperature float64, maxTokens int, topP float64, forceJSON bool) (string, error) {
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
		Temperature: normalizeTemperature(c.model, temperature),
		MaxTokens:   maxTokens,
		TopP:        normalizeTopP(c.model, topP),
	}
	if forceJSON {
		reqBody.ResponseFormat = &ResponseFormat{Type: "json_object"}
	}
	if disableThinkingForModel(c.model) {
		reqBody.Thinking = &ThinkingControl{Type: "disabled"}
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

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
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
	// 记录原始响应到本地日志（包括 400 等错误响应），便于排查参数问题
	logRawLLMResponse(body)
	if resp.StatusCode != http.StatusOK {
		bodyStr := string(body)
		if strings.Contains(bodyStr, "content_filter") || strings.Contains(bodyStr, "safety policy") || strings.Contains(bodyStr, "blocked") {
			return "", fmt.Errorf("LLM 内容安全策略拦截：输入或输出包含敏感信息，建议关闭社交情绪搜索或更换模型后重试")
		}
		return "", fmt.Errorf("LLM 返回错误状态码 %d: %s", resp.StatusCode, bodyStr)
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		msg := chatResp.Error.Message
		if strings.Contains(msg, "content_filter") || strings.Contains(msg, "safety policy") || strings.Contains(msg, "blocked") {
			return "", fmt.Errorf("LLM 内容安全策略拦截：输入或输出包含敏感信息，建议关闭社交情绪搜索或更换模型后重试")
		}
		return "", fmt.Errorf("LLM API 错误: %s", msg)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM 返回空 choices")
	}
	content := chatResp.Choices[0].Message.Content
	if content == "" {
		reasoning := chatResp.Choices[0].Message.ReasoningContent
		if reasoning != "" {
			return "", fmt.Errorf("LLM content 为空，但返回了 reasoning_content（长度 %d）；当前模型在 JSON 输出模式下未生成正文，请尝试关闭 thinking 或更换模型", len(reasoning))
		}
		return "", fmt.Errorf("LLM 返回空 content")
	}
	return content, nil
}

// logRawLLMResponse 把 LLM 原始响应写入本地调试日志（限长，避免敏感信息过量）。
func logRawLLMResponse(body []byte) {
	defer func() { _ = recover() }()
	if len(body) == 0 {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logDir := filepath.Join(home, ".config", "stock-analyzer", "logs")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "ai_research_llm.log")
	truncated := body
	if len(truncated) > 8192 {
		truncated = append(truncated[:8192], []byte("\n...truncated")...)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "[%s]\n%s\n\n", time.Now().Format(time.RFC3339), truncated)
}

// Test 测试 LLM 连接是否可用
func (c *LLMClient) Test() error {
	_, err := c.Complete(context.Background(), "You are a helpful assistant.", "Hi", normalizeTemperature(c.model, 0.2), 10, normalizeTopP(c.model, 1.0), false)
	return err
}
