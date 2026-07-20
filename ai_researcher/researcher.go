package ai_researcher

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Researcher AI 投研编排器
type Researcher struct {
	cfg     *AIConfig
	cache   *CacheManager
	storage cacheStorage
	tavily  *TavilyClient
	llm     *LLMClient
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
		cfg.SearchAPIKeys,
		cfg.ExhaustedSearchKeys,
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
		cfg:     cfg,
		cache:   NewCacheManager(storage),
		storage: storage,
		tavily:  tavily,
		llm:     llm,
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
	// 同步本轮新标记的超额 Key 到配置并持久化
	r.syncExhaustedKeys()
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}
	emit("search", fmt.Sprintf("搜索完成，共获取 %d 条结果", len(results)))

	// 3. 过滤低质量/垃圾搜索结果，避免污染 LLM 输入和参考来源
	results = filterSearchResults(results)
	emit("search", fmt.Sprintf("过滤后剩余 %d 条有效结果", countSearchItems(results)))

	// 4. 去重、汇总来源（按相关性、时效性、可信度过滤）
	sources := collectSources(results, symbol, name, r.cfg.SearchRecencyDays)

	// 5. 调用 LLM
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

// syncExhaustedKeys 把 TavilyClient 中新增的超额 Key 同步到 AIConfig 并持久化
func (r *Researcher) syncExhaustedKeys() {
	exhausted := r.tavily.GetExhaustedKeys()
	if len(exhausted) == 0 {
		return
	}
	changed := false
	if r.cfg.ExhaustedSearchKeys == nil {
		r.cfg.ExhaustedSearchKeys = make(map[string]string)
	}
	for k, month := range exhausted {
		if r.cfg.ExhaustedSearchKeys[k] != month {
			r.cfg.ExhaustedSearchKeys[k] = month
			changed = true
		}
	}
	if !changed {
		return
	}
	if err := r.storage.SaveAIConfig(r.cfg); err != nil {
		// 持久化失败只打印日志，不影响主流程
		fmt.Printf("[AIResearch] 保存超额 Key 状态失败: %v\n", err)
	}
}

// TestConnection 测试 LLM 和 Tavily 连接
func (r *Researcher) TestConnection() (*TestConnectionResult, error) {
	if !r.cfg.Enabled {
		return &TestConnectionResult{Success: false, Message: "AI 投研功能未启用：请先打开上方「启用 AI 投研」开关，再填写 API Key 并测试连接"}, nil
	}

	// 先测试 Tavily，并收集每个 Key 的状态
	keyStatuses := r.tavily.TestKeys()
	hasUsableKey := false
	for _, s := range keyStatuses {
		if s.Status == "ok" {
			hasUsableKey = true
			break
		}
	}
	if !hasUsableKey {
		msg := "Tavily 搜索连接失败: 未配置可用 Key"
		if len(keyStatuses) > 0 {
			msg = fmt.Sprintf("Tavily 搜索连接失败: %s", formatKeyStatuses(keyStatuses))
		}
		return &TestConnectionResult{Success: false, Message: msg, SearchKeyStatuses: keyStatuses}, nil
	}

	// 再测试 LLM
	if err := r.llm.Test(); err != nil {
		return &TestConnectionResult{Success: false, Message: fmt.Sprintf("LLM 连接失败: %v", err), SearchKeyStatuses: keyStatuses}, nil
	}

	return &TestConnectionResult{Success: true, Message: "连接成功", SearchKeyStatuses: keyStatuses}, nil
}

// llmOutput LLM 返回的 JSON 结构
type llmOutput struct {
	Sections []ResearchSection `json:"sections"`
	Sources  []ResearchSource  `json:"sources"`
}

// sanitizeJSON 修复 LLM 生成的 JSON 中常见的转义错误。
// 主要处理：
//  1. 字符串值内部的真实换行/回车（LLM 未转义为 \n 导致 json 解析失败）；
//  2. 反斜杠后直接跟真实换行/回车；
//  3. 对象/数组末尾多余的逗号。
func sanitizeJSON(s string) string {
	// 1. 先把字符串值内部的未转义控制字符统一转义
	s = escapeControlCharsInJSONStrings(s)
	// 2. 移除 trailing comma：",}" -> "}" 和 ",]" -> "]"
	s = trailingCommaRegex.ReplaceAllString(s, "$1")
	return s
}

// escapeControlCharsInJSONStrings 在 JSON 字符串值内部，把未转义的控制字符
// （真实换行、回车、Tab 等）替换为合法转义序列。
// 同时处理反斜杠后直接跟真实换行的情况（LLM 未完成转义），将其视为 \n。
// 它只处理字符串值内部，不会破坏 JSON 结构中的空白分隔。
func escapeControlCharsInJSONStrings(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			// 反斜杠后跟真实换行/回车：视为 LLM 想写 \n
			if c == '\n' || c == '\r' {
				b.WriteString("\\n")
				escaped = false
				continue
			}
			b.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inString = !inString
			b.WriteByte(c)
			continue
		}
		if inString && c < 0x20 {
			switch c {
			case '\n', '\r':
				b.WriteString("\\n")
			case '\t':
				b.WriteString("\\t")
			default:
				// 其他控制字符直接跳过，避免解析失败
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

var trailingCommaRegex = regexp.MustCompile(`,(\s*[}\]])`)

// smartRepairJSON 对 LLM 输出的 JSON 做结构感知修复：
// 1. 转义字符串值内部未转义的双引号；
// 2. 转义字符串值内部的真实换行/回车/Tab；
// 3. 处理反斜杠后直接跟真实换行的情况；
// 4. 移除对象/数组末尾多余的逗号。
func smartRepairJSON(s string) string {
	s = repairJSON(s)
	s = trailingCommaRegex.ReplaceAllString(s, "$1")
	return s
}

// repairJSON 使用状态机重新扫描 JSON，把字符串值内部未转义的双引号统一转义。
// 它通过当前容器上下文（对象/数组）判断一个双引号是字符串结束符还是内层引号：
//   - 对象键字符串后面只能是 ':'；
//   - 对象值/数组元素字符串后面只能是 ',' 或对应的闭合符；
//   - 其他情况均视为内层引号，前面加反斜杠。
//
// 同时会修复真实换行、回车、Tab 等未转义控制字符，以及反斜杠后直接跟真实换行的情况。
func repairJSON(s string) string {
	type frame struct {
		kind       byte
		expectsKey bool // 仅当 kind == '{' 时有效
	}
	var stack []frame

	var b strings.Builder
	b.Grow(len(s) + 10)

	inString := false
	inKey := false
	escaped := false

	nextNonSpace := func(start int) (byte, bool) {
		for i := start; i < len(s); i++ {
			c := s[i]
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				return c, true
			}
		}
		return 0, false
	}

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			if c == '\n' || c == '\r' {
				b.WriteString("\\n")
			} else {
				b.WriteByte(c)
			}
			escaped = false
			continue
		}

		if c == '\\' {
			if inString {
				escaped = true
			}
			b.WriteByte(c)
			continue
		}

		if c == '"' {
			if !inString {
				inString = true
				inKey = false
				if len(stack) > 0 {
					top := &stack[len(stack)-1]
					if top.kind == '{' && top.expectsKey {
						inKey = true
					}
				}
				b.WriteByte(c)
				continue
			}

			// 字符串内部，判断是结束引号还是内层引号
			next, ok := nextNonSpace(i + 1)
			valid := false
			if inKey {
				valid = ok && next == ':'
			} else {
				if len(stack) == 0 {
					valid = !ok
				} else if stack[len(stack)-1].kind == '{' {
					valid = ok && (next == ',' || next == '}')
				} else { // '['
					valid = ok && (next == ',' || next == ']')
				}
			}

			wasKey := inKey
			if valid {
				inString = false
				inKey = false
				if !wasKey && len(stack) > 0 && stack[len(stack)-1].kind == '{' {
					stack[len(stack)-1].expectsKey = false
				}
				b.WriteByte(c)
				continue
			}

			// 内层引号：转义
			b.WriteByte('\\')
			b.WriteByte(c)
			continue
		}

		if !inString {
			switch c {
			case '{':
				stack = append(stack, frame{kind: '{', expectsKey: true})
				b.WriteByte(c)
			case '[':
				stack = append(stack, frame{kind: '['})
				b.WriteByte(c)
			case '}':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
					if len(stack) > 0 && stack[len(stack)-1].kind == '{' {
						stack[len(stack)-1].expectsKey = false
					}
				}
				b.WriteByte(c)
			case ']':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
					if len(stack) > 0 && stack[len(stack)-1].kind == '{' {
						stack[len(stack)-1].expectsKey = false
					}
				}
				b.WriteByte(c)
			case ':':
				if len(stack) > 0 && stack[len(stack)-1].kind == '{' {
					stack[len(stack)-1].expectsKey = false
				}
				b.WriteByte(c)
			case ',':
				if len(stack) > 0 && stack[len(stack)-1].kind == '{' {
					stack[len(stack)-1].expectsKey = true
				}
				b.WriteByte(c)
			default:
				b.WriteByte(c)
			}
			continue
		}

		// 字符串内部的普通字符
		if c < 0x20 {
			switch c {
			case '\n', '\r':
				b.WriteString("\\n")
			case '\t':
				b.WriteString("\\t")
			default:
				// 其他控制字符直接跳过
			}
			continue
		}
		b.WriteByte(c)
	}

	return b.String()
}

// extractJSONObject 从文本中提取第一个完整的 JSON 对象（按大括号深度匹配，尊重字符串）。
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	inString := false
	escaped := false
	depth := 0
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

// normalizeWhitespaceForJSON 作为最后兜底，将真实换行/回车替换为空格，
// 避免字符串值内未转义换行导致解析失败（会丢失字符串内的换行格式，但保证结构可用）。
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// escapeInnerQuotes 尝试修复 JSON 字符串值内部未转义的双引号。
// 启发式规则：在字符串内部遇到双引号时，若其后紧跟可选空白 + 结构字符（: , } ]），
// 则视为字符串结束符；否则视为内层双引号，在前面加反斜杠转义。
// 注意：这是尽力修复，不能保证 100%% 准确，仅作为解析兜底。
func escapeInnerQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			if inString {
				// 判断是字符串结束符还是内层引号
				j := i + 1
				for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
					j++
				}
				if j < len(s) && (s[j] == ':' || s[j] == ',' || s[j] == '}' || s[j] == ']') {
					inString = false
				} else {
					b.WriteByte('\\')
				}
			} else {
				inString = true
			}
			b.WriteByte(c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func normalizeWhitespaceForJSON(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func parseLLMOutput(content string) (*llmOutput, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("LLM 输出为空")
	}
	// 去掉可能的 markdown 代码块
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSpace(content)
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	out, lastErr := tryParseLLMOutput(content)
	if lastErr != nil {
		// 打印原始响应便于排查（截断避免日志过大）
		debugRaw := content
		if len(debugRaw) > 2000 {
			debugRaw = debugRaw[:2000] + "..."
		}
		fmt.Printf("[AIResearch] 解析 LLM 输出失败，最后错误: %v，原始响应:\n%s\n", lastErr, debugRaw)
		return nil, fmt.Errorf("解析 LLM 输出失败: %w (原始响应前 500 字符: %s)", lastErr, truncateString(content, 500))
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

	return out, nil
}

// tryParseLLMOutput 尝试多种方式解析 LLM 输出，返回成功时的结果或最后一次错误。
func tryParseLLMOutput(content string) (*llmOutput, error) {
	var out llmOutput
	var lastErr error

	// 1) 直接解析
	lastErr = json.Unmarshal([]byte(content), &out)
	if lastErr == nil {
		return &out, nil
	}

	// 2) 修复转义后解析
	fixed := sanitizeJSON(content)
	if fixed != content {
		lastErr = json.Unmarshal([]byte(fixed), &out)
		if lastErr == nil {
			return &out, nil
		}
	}

	// 3) 结构感知智能修复（重点处理字符串内未转义双引号、真实换行等）
	repaired := smartRepairJSON(content)
	if repaired != content {
		lastErr = json.Unmarshal([]byte(repaired), &out)
		if lastErr == nil {
			return &out, nil
		}
	}
	if fixed != content {
		repairedFixed := smartRepairJSON(fixed)
		if repairedFixed != fixed {
			lastErr = json.Unmarshal([]byte(repairedFixed), &out)
			if lastErr == nil {
				return &out, nil
			}
		}
	}

	// 4) 提取第一个完整 JSON 对象
	candidate := extractJSONObject(content)
	if candidate != content && candidate != "" {
		lastErr = json.Unmarshal([]byte(candidate), &out)
		if lastErr == nil {
			return &out, nil
		}
		// 5) 对提取对象做转义修复
		fixedCandidate := sanitizeJSON(candidate)
		if fixedCandidate != candidate {
			lastErr = json.Unmarshal([]byte(fixedCandidate), &out)
			if lastErr == nil {
				return &out, nil
			}
		}
		repairedCandidate := smartRepairJSON(candidate)
		if repairedCandidate != candidate {
			lastErr = json.Unmarshal([]byte(repairedCandidate), &out)
			if lastErr == nil {
				return &out, nil
			}
		}
	}

	// 6) 最后兜底：移除所有真实换行/回车/Tab
	lastResort := normalizeWhitespaceForJSON(content)
	if lastResort != content {
		lastErr = json.Unmarshal([]byte(lastResort), &out)
		if lastErr == nil {
			return &out, nil
		}
	}

	// 7) 旧版启发式修复字符串内未转义双引号（作为最终兜底）
	quoteFixed := escapeInnerQuotes(content)
	if quoteFixed != content {
		lastErr = json.Unmarshal([]byte(quoteFixed), &out)
		if lastErr == nil {
			return &out, nil
		}
		// 结合 sanitize
		quoteFixedSanitized := sanitizeJSON(quoteFixed)
		if quoteFixedSanitized != quoteFixed {
			lastErr = json.Unmarshal([]byte(quoteFixedSanitized), &out)
			if lastErr == nil {
				return &out, nil
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("所有解析尝试均失败")
	}
	return nil, fmt.Errorf("所有解析尝试均失败 (最后错误: %w)", lastErr)
}

// filterSearchResults 对每个查询返回的结果做垃圾过滤。
func filterSearchResults(results []SearchResult) []SearchResult {
	out := make([]SearchResult, 0, len(results))
	for _, r := range results {
		filtered := make([]SearchItem, 0, len(r.Items))
		for _, item := range r.Items {
			if isQualitySource(item) {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) > 0 {
			out = append(out, SearchResult{
				Query:   r.Query,
				Items:   filtered,
				RawJSON: r.RawJSON,
			})
		}
	}
	return out
}

func countSearchItems(results []SearchResult) int {
	n := 0
	for _, r := range results {
		n += len(r.Items)
	}
	return n
}

func collectSources(results []SearchResult, symbol, name string, recencyDays int) []ResearchSource {
	filter := newSourceFilter(symbol, name, recencyDays)
	seen := make(map[string]bool)
	var sources []ResearchSource
	for _, r := range results {
		for _, item := range r.Items {
			if item.URL == "" || seen[item.URL] {
				continue
			}
			if !isQualitySource(item) {
				continue
			}
			if !filter.isRelevant(item, r.Query) {
				continue
			}
			if !filter.isWithinRecency(item) {
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

// isQualitySource 判断单条搜索结果是否可信。
func isQualitySource(item SearchItem) bool {
	// 1. Tavily 相关度分数过低则丢弃；阈值放宽到 0.2，避免误杀有效行业新闻
	if item.Score > 0 && item.Score < 0.20 {
		return false
	}
	// 2. 标题或摘要含垃圾关键词则丢弃
	combined := strings.ToLower(item.Title + " " + item.Content + " " + item.URL)
	for _, kw := range SpamKeywords() {
		if strings.Contains(combined, strings.ToLower(kw)) {
			return false
		}
	}
	// 3. 排除黑名单域名关键词
	host := strings.ToLower(item.URL)
	for _, d := range SpamDomainKeywords() {
		if strings.Contains(host, strings.ToLower(d)) {
			return false
		}
	}
	return true
}

// sourceFilter 封装参考来源的相关性、时效性过滤逻辑。
type sourceFilter struct {
	symbol      string
	name        string
	recencyDays int
}

func newSourceFilter(symbol, name string, recencyDays int) sourceFilter {
	if recencyDays <= 0 {
		recencyDays = 180
	}
	return sourceFilter{symbol: symbol, name: name, recencyDays: recencyDays}
}

func (f sourceFilter) pureCode() string {
	parts := strings.Split(f.symbol, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return f.symbol
}

// isRelevant 判断来源是否与当前股票（或全球产业映射主题）相关。
// 相关性不再硬要求出现股票全称或代码：如果来源与查询主题（产品/政策/风险/产业链等）高度契合，也应保留。
func (f sourceFilter) isRelevant(item SearchItem, query string) bool {
	text := strings.ToLower(item.Title + " " + item.Content + " " + item.URL)
	name := strings.ToLower(strings.TrimSpace(f.name))
	code := strings.ToLower(strings.TrimSpace(f.pureCode()))

	hasStock := (name != "" && strings.Contains(text, name)) || (code != "" && strings.Contains(text, code))
	if hasStock {
		return true
	}

	// 全球产业映射类查询：允许海外龙头或产业链映射相关来源
	if isGlobalMappingQuery(query) {
		if containsAnyKeyword(text, overseasCompanyKeywords) {
			return true
		}
		return containsAnyKeyword(text, mappingKeywords)
	}

	// 普通查询：若未直接出现股票名称/代码，则依据查询中的主题关键词判断。
	// 命中至少 2 个主题关键词视为相关；命中 1 个高置信度关键词也视为相关。
	topicKeywords := extractQueryKeywords(query, name, code)
	matched := 0
	for _, kw := range topicKeywords {
		if strings.Contains(text, kw) {
			matched++
		}
	}
	if matched >= 2 {
		return true
	}
	if matched >= 1 && containsAnyKeyword(text, highRelevanceKeywords) {
		return true
	}
	return false
}

// isWithinRecency 判断来源是否在时效范围内；监管/交易所/官方公告豁免。
func (f sourceFilter) isWithinRecency(item SearchItem) bool {
	if item.Published == "" {
		return true
	}
	date := normalizeDate(item.Published)
	if date == "" {
		return true
	}
	if isOfficialSource(item.URL) {
		return true
	}
	cutoff := time.Now().AddDate(0, 0, -f.recencyDays).Format("2006-01-02")
	return date >= cutoff
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
	// 按可信度排序：官方/主流财经 > 国际媒体/科技媒体 > 其他
	sort.SliceStable(out, func(i, j int) bool {
		return sourceTier(out[i].URL) < sourceTier(out[j].URL)
	})
	// 限制来源数量，优先保留高可信度来源
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// sourceTier 返回来源可信度层级，数字越小越可信。
func sourceTier(url string) int {
	u := strings.ToLower(url)
	switch {
	case containsAnyKeyword(u, tier1Domains):
		return 1
	case containsAnyKeyword(u, tier2Domains):
		return 2
	case containsAnyKeyword(u, tier3Domains):
		return 3
	default:
		return 4
	}
}

// isOfficialSource 判断是否为监管/交易所/公司公告等官方来源。
func isOfficialSource(url string) bool {
	return containsAnyKeyword(strings.ToLower(url), tier1Domains)
}

func isGlobalMappingQuery(query string) bool {
	q := strings.ToLower(query)
	return strings.Contains(q, "全球产业映射") || strings.Contains(q, "海外龙头")
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// extractQueryKeywords 从查询字符串中提取除股票名称/代码外的主题关键词，用于相关性兜底判断。
func extractQueryKeywords(query, name, code string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if name != "" {
		q = strings.ReplaceAll(q, strings.ToLower(name), "")
	}
	if code != "" {
		q = strings.ReplaceAll(q, strings.ToLower(code), "")
	}
	// 移除股票代码后缀 .sh/.sz/.hk 等
	q = symbolSuffixRegex.ReplaceAllString(q, "")

	words := strings.Fields(q)
	seen := make(map[string]bool)
	var out []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" || len(w) < 2 || queryStopWords[w] {
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

var (
	symbolSuffixRegex = regexp.MustCompile(`\.(sh|sz|hk|bj|us|ss|nq)\b`)

	// 查询停用词：在相关性判断中忽略过于宽泛或无意义的词
	queryStopWords = map[string]bool{
		"a股": true, "股票": true, "公司": true, "有限": true, "股份": true,
		"分析": true, "研究": true, "报告": true, "查询": true, "搜索": true,
		"相关": true, "有关": true, "关于": true, "最新": true, "动态": true,
		"新闻": true, "资讯": true, "头条": true, "快讯": true,
		"的": true, "和": true, "与": true, "或": true, "等": true, "对": true,
		"及": true, "在": true, "是": true, "有": true, "了": true, "为": true,
	}

	// 高置信度主题关键词：命中一次即可认为与查询主题相关
	highRelevanceKeywords = []string{
		"产业链", "供应链", "行业", "板块", " sectors ", "sector",
		"政策", "监管", "补贴", "关税", "营收", "业绩", "财报", "盈利",
		"订单", "产能", "投产", "中标", "扩产", "涨价", "降价", "价格",
		"技术突破", "技术进展", "新品", "新产品", "发布",
		"立案调查", "行政处罚", "财务造假", "退市", "问询函", "监管函", "警示",
		"对标", "竞争对手", "竞争格局", "市场份额", "估值",
		"映射", "预期差", "利好", "利空", "带动", "受益", "拖累",
		"雪球", "股吧", "reddit", "twitter", "x.com", "社交",
	}

	// 海外龙头公司关键词，用于全球产业映射查询的相关性判断
	// 海外龙头公司关键词，用于全球产业映射查询的相关性判断
	overseasCompanyKeywords = []string{
		"nvidia", "amd", "tsmc", "台积电", "micron", "sk hynix", "skhynix", "hynix",
		"openai", "anthropic", "google", "alphabet", "microsoft", "tesla", "xai",
		"asml", "samsung", "apple", "intel", "qualcomm", "broadcom", "amazon",
		"英伟达", "超威", "微软", "特斯拉", "苹果", "三星", "谷歌", "台积电",
		"高通", "英特尔", "美光", "海力士", "阿斯麦", "马斯克", "musk",
	}
	// 产业链映射关键词
	mappingKeywords = []string{
		"映射", "产业链", "供应链", "预期差", "利好", "利空", "影响", "对标",
		"受益", "拖累", "带动", "波及", "传导", "上游", "下游", "龙头",
	}
	// Tier 1: 监管/交易所/公司公告官方来源
	tier1Domains = []string{
		"csrc.gov.cn", "sse.com.cn", "szse.cn", "bse.cn", "cninfo.com.cn", "hkexnews.hk",
	}
	// Tier 2: 国内主流财经媒体/券商/社区
	tier2Domains = []string{
		"eastmoney.com", "sina.com.cn", "xueqiu.com", "caixin.com", "cls.cn", "stcn.com",
		"jiemian.com", "wallstreetcn.com", "cs.com.cn", "hexun.com", "10jqka.com.cn",
		"gelonghui.com", "cnstock.com", "p5w.net", "jrj.com", "ccstock.cn",
	}
	// Tier 3: 国际主流财经/科技媒体
	tier3Domains = []string{
		"bloomberg.com", "reuters.com", "ft.com", "cnbc.com", "techcrunch.com",
		"theverge.com", "wired.com", "marketwatch.com", "seekingalpha.com", "investing.com",
		"yahoo.com", "nikkei.com", "forbes.com", "businessinsider.com", "nasdaq.com",
		"semianalysis.com", "tomshardware.com", "anandtech.com", "ars technica", "tomshardware.com",
		"nvidia.com", "openai.com", "anthropic.com", "microsoft.com", "google.com",
		"tesla.com", "tsmc.com", "samsung.com", "asml.com", "micron.com", "skhynix.com",
	}
)

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
