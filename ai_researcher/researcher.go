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
	// 1. 先把字符串值内部的未转义换行/回车统一转义为 \n
	s = escapeNewlinesInJSONStrings(s)
	// 2. 反斜杠后直接跟真实换行/回车：转成合法的 \n
	s = strings.ReplaceAll(s, "\\\r\n", "\\n")
	s = strings.ReplaceAll(s, "\\\n", "\\n")
	s = strings.ReplaceAll(s, "\\\r", "\\n")
	// 3. 移除 trailing comma：",}" -> "}" 和 ",]" -> "]"
	s = trailingCommaRegex.ReplaceAllString(s, "$1")
	return s
}

// escapeNewlinesInJSONStrings 在 JSON 字符串值内部，把未转义的真实换行/回车替换为 \n。
// 它只处理字符串值内部，不会破坏 JSON 结构中的空白分隔。
func escapeNewlinesInJSONStrings(s string) string {
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
			inString = !inString
			b.WriteByte(c)
			continue
		}
		if inString && (c == '\n' || c == '\r') {
			b.WriteString("\\n")
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

var trailingCommaRegex = regexp.MustCompile(`,(\s*[}\]])`)

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

	// 如果直接解析失败，尝试从文本中提取第一个 { ... } JSON 对象
	extractJSON := func(s string) string {
		start := strings.Index(s, "{")
		end := strings.LastIndex(s, "}")
		if start >= 0 && end > start {
			return s[start : end+1]
		}
		return s
	}

	var out llmOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		candidate := extractJSON(content)
		if candidate != content {
			if err2 := json.Unmarshal([]byte(candidate), &out); err2 == nil {
				goto validate
			}
		}
		// 尝试修复 LLM 常见 JSON 转义错误后再解析
		fixed := sanitizeJSON(content)
		if fixed != content {
			if err2 := json.Unmarshal([]byte(fixed), &out); err2 == nil {
				goto validate
			}
		}
		return nil, err
	}

validate:

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
	// 1. Tavily 相关度分数过低则丢弃
	if item.Score > 0 && item.Score < 0.35 {
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
		recencyDays = 60
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
func (f sourceFilter) isRelevant(item SearchItem, query string) bool {
	text := strings.ToLower(item.Title + " " + item.Content + " " + item.URL)
	name := strings.ToLower(strings.TrimSpace(f.name))
	code := strings.ToLower(strings.TrimSpace(f.pureCode()))

	hasStock := (name != "" && strings.Contains(text, name)) || (code != "" && strings.Contains(text, code))

	// 全球产业映射类查询：允许海外龙头或产业链映射相关来源，但仍需和主题相关
	if isGlobalMappingQuery(query) {
		if hasStock {
			return true
		}
		if containsAnyKeyword(text, overseasCompanyKeywords) {
			return true
		}
		return containsAnyKeyword(text, mappingKeywords)
	}

	// 普通查询：必须直接出现股票名称或代码
	return hasStock
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

var (
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
