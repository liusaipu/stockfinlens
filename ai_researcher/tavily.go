package ai_researcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const tavilyAPIURL = "https://api.tavily.com/search"

// TavilyClient Tavily 搜索客户端
type TavilyClient struct {
	apiKeys       []string
	exhaustedKeys map[string]string // key -> "YYYY-MM"
	currentIdx    int
	mu            sync.Mutex
	depth         string
	maxResults    int
	recencyDays   int
	httpClient    *http.Client
}

// TavilyResponse Tavily API 响应结构
type TavilyResponse struct {
	Query   string `json:"query"`
	Results []struct {
		Title     string  `json:"title"`
		URL       string  `json:"url"`
		Content   string  `json:"content"`
		Score     float64 `json:"score"`
		Published string  `json:"published_date"`
	} `json:"results"`
	Answer string `json:"answer"`
}

// NewTavilyClient 创建 Tavily 客户端
func NewTavilyClient(apiKeys []string, exhaustedKeys map[string]string, depth string, maxResults, recencyDays int, timeoutSeconds int) *TavilyClient {
	if depth != "basic" && depth != "advanced" {
		depth = "advanced"
	}
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 10
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	if exhaustedKeys == nil {
		exhaustedKeys = make(map[string]string)
	}
	return &TavilyClient{
		apiKeys:       apiKeys,
		exhaustedKeys: exhaustedKeys,
		currentIdx:    0,
		depth:         depth,
		maxResults:    maxResults,
		recencyDays:   recencyDays,
		httpClient:    &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

// pickKey 轮询选择一个 Key，跳过本月已超额的 Key
func (c *TavilyClient) pickKey() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.apiKeys) == 0 {
		return ""
	}
	// 优先选择未超额的 Key
	for i := 0; i < len(c.apiKeys); i++ {
		idx := (c.currentIdx + i) % len(c.apiKeys)
		key := c.apiKeys[idx]
		if !c.isKeyExhaustedLocked(key) {
			c.currentIdx = (idx + 1) % len(c.apiKeys)
			return key
		}
	}
	// 所有 Key 都超额时，仍按轮询返回（避免无 Key 可用）
	key := c.apiKeys[c.currentIdx]
	c.currentIdx = (c.currentIdx + 1) % len(c.apiKeys)
	return key
}

// isKeyExhaustedLocked 判断 Key 在本月是否已超额（调用前必须持有锁）
func (c *TavilyClient) isKeyExhaustedLocked(key string) bool {
	if key == "" {
		return true
	}
	month, ok := c.exhaustedKeys[key]
	if !ok {
		return false
	}
	return month == time.Now().Format("2006-01")
}

// MarkKeyExhausted 标记 Key 本月已超额
func (c *TavilyClient) MarkKeyExhausted(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if key != "" {
		c.exhaustedKeys[key] = time.Now().Format("2006-01")
	}
}

// GetExhaustedKeys 返回当前超额 Key 映射（key -> 月份）
func (c *TavilyClient) GetExhaustedKeys() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.exhaustedKeys))
	for k, v := range c.exhaustedKeys {
		out[k] = v
	}
	return out
}

// Search 执行单次搜索查询，带 3 次重试，并在多个 Key 之间轮换
func (c *TavilyClient) Search(ctx context.Context, query string, includeDomains []string, progress ProgressFunc) (*SearchResult, error) {
	if len(c.apiKeys) == 0 {
		return nil, fmt.Errorf("Tavily API Key 为空")
	}

	// 记录本轮已经尝试过耗尽/无效的 Key，避免重复尝试
	triedKeys := make(map[string]bool)
	var lastErr error

	for keyRound := 0; keyRound < len(c.apiKeys); keyRound++ {
		apiKey := c.pickKey()
		if apiKey == "" || triedKeys[apiKey] {
			continue
		}
		triedKeys[apiKey] = true

		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(attempt) * time.Second):
				}
			}
			if progress != nil {
				progress("search", fmt.Sprintf("正在搜索: %s", query))
			}
			result, err := c.searchOnce(ctx, query, includeDomains, apiKey)
			if err == nil {
				return result, nil
			}
			lastErr = err
			// 400 不重试；额度用完则直接换下一个 Key
			if strings.Contains(err.Error(), "状态码 400") {
				return nil, err
			}
			if strings.Contains(err.Error(), "额度已用完") || strings.Contains(err.Error(), "状态码 432") {
				c.MarkKeyExhausted(apiKey)
				break
			}
		}
	}

	return nil, fmt.Errorf("Tavily 搜索重试后仍失败（已尝试 %d 个 Key）: %w", len(triedKeys), lastErr)
}

func (c *TavilyClient) searchOnce(ctx context.Context, query string, includeDomains []string, apiKey string) (*SearchResult, error) {
	reqBody := map[string]interface{}{
		"api_key":             apiKey,
		"query":               query,
		"search_depth":        c.depth,
		"max_results":         c.maxResults,
		"include_answer":      false,
		"include_raw_content": c.depth == "advanced",
	}
	if c.recencyDays > 0 {
		days := c.recencyDays
		if days <= 7 {
			reqBody["time_range"] = "week"
		} else if days <= 30 {
			reqBody["time_range"] = "month"
		} else {
			reqBody["time_range"] = "year"
		}
	}
	if len(includeDomains) > 0 {
		reqBody["include_domains"] = includeDomains
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构造 Tavily 请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tavilyAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建 Tavily 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Tavily 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Tavily 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		bodyStr := string(body)
		// 432 = 超出套餐额度，直接给出可阅读提示，避免无意义重试
		if resp.StatusCode == 432 || strings.Contains(bodyStr, "exceeds your plan") || strings.Contains(bodyStr, "usage limit") {
			return nil, fmt.Errorf("Tavily 搜索额度已用完（状态码 432），请升级套餐或更换 API Key")
		}
		return nil, fmt.Errorf("Tavily 返回错误状态码 %d: %s", resp.StatusCode, bodyStr)
	}

	var tavilyResp TavilyResponse
	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return nil, fmt.Errorf("解析 Tavily 响应失败: %w", err)
	}

	result := &SearchResult{
		Query:   query,
		RawJSON: string(body),
	}
	for _, r := range tavilyResp.Results {
		content := r.Content
		if content == "" {
			content = r.Title
		}
		result.Items = append(result.Items, SearchItem{
			Title:     r.Title,
			URL:       r.URL,
			Content:   content,
			Score:     r.Score,
			Published: r.Published,
		})
	}
	return result, nil
}

// SearchMulti 并发执行多个查询，允许部分失败
func (c *TavilyClient) SearchMulti(ctx context.Context, queries []string, includeDomains []string, progress ProgressFunc) ([]SearchResult, error) {
	if len(queries) == 0 {
		return nil, nil
	}
	type pair struct {
		idx    int
		result *SearchResult
		err    error
	}
	ch := make(chan pair, len(queries))
	for i, q := range queries {
		go func(idx int, query string) {
			res, err := c.Search(ctx, query, includeDomains, progress)
			ch <- pair{idx: idx, result: res, err: err}
		}(i, q)
	}

	results := make([]SearchResult, 0, len(queries))
	var errs []string
	for i := 0; i < len(queries); i++ {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case p := <-ch:
			if p.err != nil {
				errs = append(errs, fmt.Sprintf("查询[%d]失败: %v", p.idx, p.err))
			}
			if p.result != nil {
				results = append(results, *p.result)
			}
		}
	}

	// 只要有一个查询成功，就返回成功结果，并把错误作为提示
	if len(results) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("所有 Tavily 搜索均失败: %s", strings.Join(errs, "; "))
	}
	if len(errs) > 0 {
		fmt.Printf("[AIResearch] 部分 Tavily 搜索失败: %s\n", strings.Join(errs, "; "))
	}
	return results, nil
}

// TavilyKeyStatus 单个 Tavily Key 的测试结果
type TavilyKeyStatus struct {
	Key     string `json:"key"`     // 脱敏后的 Key 前缀
	Status  string `json:"status"`  // "ok" | "exhausted" | "invalid" | "error"
	Message string `json:"message"` // 详细说明
}

// Test 测试 Tavily 连接是否可用（只要有一个 Key 可用即成功）
func (c *TavilyClient) Test() error {
	statuses := c.TestKeys()
	for _, s := range statuses {
		if s.Status == "ok" {
			return nil
		}
	}
	if len(statuses) == 0 {
		return fmt.Errorf("未配置 Tavily API Key")
	}
	return fmt.Errorf("所有 Tavily Key 均不可用: %s", formatKeyStatuses(statuses))
}

// TestKeys 逐个测试每个 Tavily Key，返回状态列表
func (c *TavilyClient) TestKeys() []TavilyKeyStatus {
	var statuses []TavilyKeyStatus
	for _, key := range c.apiKeys {
		if key == "" {
			continue
		}
		mask := maskAPIKey(key)
		_, err := c.searchOnce(context.Background(), "test", nil, key)
		if err == nil {
			statuses = append(statuses, TavilyKeyStatus{Key: mask, Status: "ok", Message: "可用"})
			continue
		}
		errStr := err.Error()
		status := "error"
		msg := errStr
		switch {
		case strings.Contains(errStr, "额度已用完") || strings.Contains(errStr, "状态码 432"):
			status = "exhausted"
			msg = "额度已用完"
		case strings.Contains(errStr, "状态码 401") || strings.Contains(errStr, "Unauthorized") || strings.Contains(errStr, "missing or invalid API key"):
			status = "invalid"
			msg = "Key 无效或已过期"
		}
		statuses = append(statuses, TavilyKeyStatus{Key: mask, Status: status, Message: msg})
	}
	return statuses
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func formatKeyStatuses(statuses []TavilyKeyStatus) string {
	parts := make([]string, 0, len(statuses))
	for _, s := range statuses {
		parts = append(parts, fmt.Sprintf("%s(%s)", s.Key, s.Message))
	}
	return strings.Join(parts, ", ")
}
