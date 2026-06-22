package ai_researcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Researcher AI 投研编排器
type Researcher struct {
	cfg    *AIConfig
	cache  *CacheManager
	tavily *TavilyClient
	llm    *LLMClient
}

// NewResearcher 创建编排器
func NewResearcher(cfg *AIConfig, storage cacheStorage) (*Researcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("AI 配置不能为空")
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	tavily := NewTavilyClient(
		cfg.SearchAPIKey,
		cfg.SearchDepth,
		cfg.MaxResults,
		cfg.SearchRecencyDays,
		cfg.SearchTimeout,
	)
	llm := NewLLMClient(
		cfg.LLMAPIKey,
		cfg.LLMBaseURL,
		cfg.LLMModel,
		cfg.LLMTimeout,
	)

	return &Researcher{
		cfg:    cfg,
		cache:  NewCacheManager(storage),
		tavily: tavily,
		llm:    llm,
	}, nil
}

// Research 执行 AI 投研分析
// forceRefresh 为 true 时跳过缓存，强制重新搜索分析
func (r *Researcher) Research(ctx context.Context, symbol, name string, forceRefresh bool, progress ProgressFunc) (*AIResearchReport, error) {
	if !r.cfg.Enabled {
		return nil, fmt.Errorf("AI 投研功能未启用")
	}

	emit := func(stage, msg string) {
		if progress != nil {
			progress(stage, msg)
		}
	}

	// 1. 检查缓存（非强制刷新时）
	if !forceRefresh {
		emit("cache", "正在检查本地缓存...")
		if cached, err := r.cache.Get(symbol, r.cfg.CacheTTLHours); err == nil && cached != nil {
			cached.Symbol = symbol
			cached.Name = name
			cached.FromCache = true
			emit("cache", "已命中本地缓存")
			return cached, nil
		}
	}

	// 2. 构造并执行搜索
	emit("search", "正在搜索互联网公开信息...")
	queries := BuildQueries(symbol, name, r.cfg.FocusRegions, r.cfg.EnableSocial, r.cfg.SearchRecencyDays)
	results, err := r.tavily.SearchMulti(ctx, queries, IncludeDomains(), emit)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}
	emit("search", fmt.Sprintf("搜索完成，共获取 %d 条结果", len(results)))

	// 3. 去重、汇总来源
	sources := collectSources(results)

	// 4. 调用 LLM
	emit("llm", "正在调用大模型生成投研报告...")
	systemPrompt := SystemPrompt(r.cfg.OutputLanguage)
	userPrompt := UserPrompt(symbol, name, r.cfg.FocusRegions, r.cfg.EnableSocial, results)

	content, err := r.llm.Complete(
		ctx,
		systemPrompt,
		userPrompt,
		r.cfg.Temperature,
		r.cfg.MaxTokens,
		r.cfg.TopP,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("LLM 分析失败: %w", err)
	}

	// 5. 解析 JSON
	emit("parse", "正在解析结构化报告...")
	parsed, err := parseLLMOutput(content)
	if err != nil {
		return nil, fmt.Errorf("解析 LLM 输出失败: %w", err)
	}

	report := &AIResearchReport{
		Symbol:      symbol,
		Name:        name,
		GeneratedAt: time.Now().Format(time.RFC3339),
		ModelUsed:   r.cfg.LLMModel,
		FromCache:   false,
		Sections:    parsed.Sections,
		Sources:     mergeSources(parsed.Sources, sources),
	}

	// 6. 保存缓存
	emit("cache", "正在保存分析结果...")
	if err := r.cache.Set(symbol, report); err != nil {
		// 缓存失败不影响主流程
		fmt.Printf("[AIResearch] 保存缓存失败: %v\n", err)
	}

	emit("done", "分析完成")
	return report, nil
}

// TestConnection 测试 LLM 和 Tavily 连接
func (r *Researcher) TestConnection() (*TestConnectionResult, error) {
	if !r.cfg.Enabled {
		return &TestConnectionResult{Success: false, Message: "AI 投研功能未启用：请先打开上方「启用 AI 投研」开关，再填写 API Key 并测试连接"}, nil
	}

	// 先测试 Tavily
	if err := r.tavily.Test(); err != nil {
		return &TestConnectionResult{Success: false, Message: fmt.Sprintf("Tavily 搜索连接失败: %v", err)}, nil
	}

	// 再测试 LLM
	if err := r.llm.Test(); err != nil {
		return &TestConnectionResult{Success: false, Message: fmt.Sprintf("LLM 连接失败: %v", err)}, nil
	}

	return &TestConnectionResult{Success: true, Message: "连接成功"}, nil
}

// llmOutput LLM 返回的 JSON 结构
type llmOutput struct {
	Sections []ResearchSection `json:"sections"`
	Sources  []ResearchSource  `json:"sources"`
}

func parseLLMOutput(content string) (*llmOutput, error) {
	content = strings.TrimSpace(content)
	// 去掉可能的 markdown 代码块
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSpace(content)
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var out llmOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, err
	}

	// 确保至少有一个 section
	if len(out.Sections) == 0 {
		return nil, fmt.Errorf("LLM 输出没有 sections")
	}

	// 规范化 sentiment
	for i := range out.Sections {
		s := strings.ToLower(out.Sections[i].Sentiment)
		switch s {
		case "positive", "乐观", "正面", "bullish":
			out.Sections[i].Sentiment = "positive"
		case "negative", "悲观", "负面", "bearish":
			out.Sections[i].Sentiment = "negative"
		default:
			out.Sections[i].Sentiment = "neutral"
		}
	}

	return &out, nil
}

func collectSources(results []SearchResult) []ResearchSource {
	seen := make(map[string]bool)
	var sources []ResearchSource
	for _, r := range results {
		for _, item := range r.Items {
			if item.URL == "" || seen[item.URL] {
				continue
			}
			seen[item.URL] = true
			sources = append(sources, ResearchSource{
				Title: item.Title,
				URL:   item.URL,
				Date:  normalizeDate(item.Published),
			})
		}
	}
	return sources
}

func mergeSources(llmSources, searchSources []ResearchSource) []ResearchSource {
	seen := make(map[string]bool)
	var out []ResearchSource
	// 优先使用 LLM 返回的来源
	for _, s := range llmSources {
		if s.URL == "" || seen[s.URL] {
			continue
		}
		seen[s.URL] = true
		out = append(out, s)
	}
	// 补充搜索来源
	for _, s := range searchSources {
		if s.URL == "" || seen[s.URL] {
			continue
		}
		seen[s.URL] = true
		out = append(out, s)
	}
	// 限制来源数量
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func normalizeDate(d string) string {
	if d == "" {
		return ""
	}
	// Tavily 可能返回 YYYY-MM-DD 或带时间
	if len(d) >= 10 {
		return d[:10]
	}
	return d
}
