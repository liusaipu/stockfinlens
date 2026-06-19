package ai_researcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const tavilyAPIURL = "https://api.tavily.com/search"

// TavilyClient Tavily 搜索客户端
type TavilyClient struct {
	apiKey      string
	depth       string
	maxResults  int
	recencyDays int
	httpClient  *http.Client
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
func NewTavilyClient(apiKey, depth string, maxResults, recencyDays int, timeoutSeconds int) *TavilyClient {
	if depth != "basic" && depth != "advanced" {
		depth = "advanced"
	}
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 10
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	return &TavilyClient{
		apiKey:      apiKey,
		depth:       depth,
		maxResults:  maxResults,
		recencyDays: recencyDays,
		httpClient:  &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

// Search 执行单次搜索查询，带 3 次重试
func (c *TavilyClient) Search(query string, includeDomains []string) (*SearchResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Tavily API Key 为空")
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		result, err := c.searchOnce(query, includeDomains)
		if err == nil {
			return result, nil
		}
		lastErr = err
		// 400 等客户端错误不重试
		if strings.Contains(err.Error(), "状态码 400") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("Tavily 搜索重试 3 次后仍失败: %w", lastErr)
}

func (c *TavilyClient) searchOnce(query string, includeDomains []string) (*SearchResult, error) {
	reqBody := map[string]interface{}{
		"api_key":             c.apiKey,
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

	req, err := http.NewRequest("POST", tavilyAPIURL, bytes.NewReader(jsonBody))
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
		return nil, fmt.Errorf("Tavily 返回错误状态码 %d: %s", resp.StatusCode, string(body))
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
func (c *TavilyClient) SearchMulti(queries []string, includeDomains []string) ([]SearchResult, error) {
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
			res, err := c.Search(query, includeDomains)
			ch <- pair{idx: idx, result: res, err: err}
		}(i, q)
	}

	results := make([]SearchResult, 0, len(queries))
	var errs []string
	for i := 0; i < len(queries); i++ {
		p := <-ch
		if p.err != nil {
			errs = append(errs, fmt.Sprintf("查询[%d]失败: %v", p.idx, p.err))
		}
		if p.result != nil {
			results = append(results, *p.result)
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

// Test 测试 Tavily 连接是否可用
func (c *TavilyClient) Test() error {
	_, err := c.Search("test", nil)
	return err
}
