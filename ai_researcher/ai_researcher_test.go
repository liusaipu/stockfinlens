package ai_researcher

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultAIConfig(t *testing.T) {
	cfg := DefaultAIConfig()
	if cfg == nil {
		t.Fatal("默认配置不应为空")
	}
	cfg.Normalize()

	if cfg.LLMProvider != "deepseek" {
		t.Errorf("默认 LLM 供应商应为 deepseek，实际是 %s", cfg.LLMProvider)
	}
	if cfg.LLMBaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("默认 BaseURL 错误: %s", cfg.LLMBaseURL)
	}
	if cfg.SearchProvider != "tavily" {
		t.Errorf("默认搜索引擎应为 tavily，实际是 %s", cfg.SearchProvider)
	}
	if cfg.CacheTTLHours != 6 {
		t.Errorf("默认缓存时间应为 6 小时，实际是 %d", cfg.CacheTTLHours)
	}
	if cfg.MaxResults != 20 {
		t.Errorf("默认 MaxResults 应为 20，实际是 %d", cfg.MaxResults)
	}
	if cfg.SearchRecencyDays != 180 {
		t.Errorf("默认 SearchRecencyDays 应为 180，实际是 %d", cfg.SearchRecencyDays)
	}
}

func TestAIConfigNormalize(t *testing.T) {
	cfg := &AIConfig{
		LLMProvider: "kimi",
	}
	cfg.Normalize()

	if cfg.LLMBaseURL != "https://api.moonshot.cn/v1" {
		t.Errorf("Kimi BaseURL 应为 moonshot，实际是 %s", cfg.LLMBaseURL)
	}
	if cfg.LLMModel != "kimi-k2.6" {
		t.Errorf("Kimi 默认模型错误: %s", cfg.LLMModel)
	}
}

func TestAIConfigNormalizeKimiCode(t *testing.T) {
	cfg := &AIConfig{
		LLMProvider: "kimi-code",
	}
	cfg.Normalize()

	if cfg.LLMBaseURL != "https://api.kimi.com/coding/v1" {
		t.Errorf("Kimi Code BaseURL 错误: %s", cfg.LLMBaseURL)
	}
	if cfg.LLMModel != "kimi-k2.6" {
		t.Errorf("Kimi Code 默认模型错误: %s", cfg.LLMModel)
	}
}

func TestNormalizeTemperature(t *testing.T) {
	if got := normalizeTemperature("kimi-k2.6", 0.2); got != 0.6 {
		t.Errorf("kimi-k2.6 temperature 应强制为 0.6（instant 模式），实际 %v", got)
	}
	if got := normalizeTemperature("kimi-k2.5", 0.2); got != 0.6 {
		t.Errorf("kimi-k2.5 temperature 应强制为 0.6（instant 模式），实际 %v", got)
	}
	if got := normalizeTemperature("kimi-k2.7-code", 0.2); got != 1.0 {
		t.Errorf("kimi-k2.7-code temperature 应强制为 1.0（thinking 模式），实际 %v", got)
	}
	if got := normalizeTemperature("deepseek-v4-pro", 0.2); got != 1.0 {
		t.Errorf("deepseek-v4-pro temperature 应强制为 1.0，实际 %v", got)
	}
	if got := normalizeTopP("kimi-k2.6", 1.0); got != 0.95 {
		t.Errorf("kimi-k2.6 top_p 应强制为 0.95，实际 %v", got)
	}
	if got := normalizeTopP("deepseek-v4-pro", 1.0); got != 1.0 {
		t.Errorf("deepseek top_p 应保持原值，实际 %v", got)
	}
	if got := normalizeTemperature("deepseek-v4-pro", 0.2); got != 1.0 {
		t.Errorf("deepseek-v4-pro temperature 应强制为 1.0，实际 %v", got)
	}
	if got := normalizeTemperature("deepseek-v4-flash", 0.2); got != 1.0 {
		t.Errorf("deepseek-v4-flash temperature 应强制为 1.0，实际 %v", got)
	}
	if got := normalizeTopP("deepseek-v4-pro", 0.5); got != 1.0 {
		t.Errorf("deepseek-v4-pro top_p 应强制为 1.0，实际 %v", got)
	}
	if !disableThinkingForModel("kimi-k2.6") {
		t.Error("kimi-k2.6 应禁用 thinking")
	}
	if !disableThinkingForModel("kimi-k2.5") {
		t.Error("kimi-k2.5 应禁用 thinking")
	}
	if !disableThinkingForModel("deepseek-v4-pro") {
		t.Error("deepseek-v4-pro 应禁用 thinking")
	}
	if !disableThinkingForModel("deepseek-v4-flash") {
		t.Error("deepseek-v4-flash 应禁用 thinking")
	}
	if disableThinkingForModel("kimi-k2.7-code") {
		t.Error("kimi-k2.7-code 不应默认禁用 thinking")
	}
}

func TestAIConfigValidate(t *testing.T) {
	cfg := DefaultAIConfig()
	cfg.Enabled = true
	cfg.LLMAPIKey = "sk-test"
	cfg.SearchAPIKey = "tvly-test"

	if err := cfg.Validate(); err != nil {
		t.Errorf("合法配置不应报错: %v", err)
	}

	cfg.LLMAPIKey = ""
	if err := cfg.Validate(); err == nil {
		t.Error("缺少 LLM API Key 应报错")
	}
}

func TestBuildQueries(t *testing.T) {
	queries := BuildQueries("603501.SH", "韦尔股份", []string{"us", "jp"}, true, 90)
	if len(queries) < 6 {
		t.Errorf("应至少生成 6 个查询，实际 %d", len(queries))
	}

	hasProduct := false
	hasPolicy := false
	hasRisk := false
	hasGlobalMapping := false
	hasIndustry := false
	hasCompetitor := false
	hasSocial := false
	for _, q := range queries {
		if strings.Contains(q, "订单") || strings.Contains(q, "产能") || strings.Contains(q, "催化") {
			hasProduct = true
		}
		if strings.Contains(q, "政策") || strings.Contains(q, "补贴") || strings.Contains(q, "关税") {
			hasPolicy = true
		}
		if strings.Contains(q, "证监会") || strings.Contains(q, "处罚") || strings.Contains(q, "ST") || strings.Contains(q, "退市") {
			hasRisk = true
		}
		if strings.Contains(q, "产业链") || strings.Contains(q, "Nvidia") || strings.Contains(q, "映射") {
			hasGlobalMapping = true
		}
		if strings.Contains(q, "行业分析") || strings.Contains(q, "竞争格局") || strings.Contains(q, "主营业务") {
			hasIndustry = true
		}
		if strings.Contains(q, "竞争对手") || strings.Contains(q, "对标") {
			hasCompetitor = true
		}
		if strings.Contains(q, "雪球") || strings.Contains(q, "股吧") {
			hasSocial = true
		}
	}
	if !hasProduct {
		t.Error("缺少产品催化剂查询")
	}
	if !hasPolicy {
		t.Error("缺少政策影响查询")
	}
	if !hasRisk {
		t.Error("缺少风险事件与监管处罚查询")
	}
	if !hasGlobalMapping {
		t.Error("缺少全球产业映射查询")
	}
	if !hasIndustry {
		t.Error("缺少行业与竞争格局查询")
	}
	if !hasCompetitor {
		t.Error("缺少国际对标查询")
	}
	if !hasSocial {
		t.Error("缺少社交情绪查询")
	}
}

func TestParseLLMOutput(t *testing.T) {
	jsonContent := `{
		"sections": [
			{
				"title": "产品催化剂",
				"summary": "新品投产带来增长",
				"key_points": ["点1", "点2"],
				"sentiment": "positive"
			}
		],
		"sources": [
			{"title": "来源1", "url": "https://example.com/1", "date": "2025-06-01"}
		]
	}`

	out, err := parseLLMOutput("```json\n" + jsonContent + "\n```")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Errorf("应有 1 个 section，实际 %d", len(out.Sections))
	}
	if out.Sections[0].Sentiment != "positive" {
		t.Errorf("sentiment 应为 positive，实际是 %s", out.Sections[0].Sentiment)
	}
}

func TestExtractJSONObject(t *testing.T) {
	// 正常对象
	got := extractJSONObject(`prefix {"a":1} suffix`)
	if got != `{"a":1}` {
		t.Errorf("应提取完整对象，实际: %s", got)
	}
	// 字符串内包含 } 时不应误停
	got = extractJSONObject(`text {"summary":"ok}","b":2}`)
	if got != `{"summary":"ok}","b":2}` {
		t.Errorf("应正确处理字符串内的大括号，实际: %s", got)
	}
	// 嵌套对象
	got = extractJSONObject(`{"a":{"b":1}}`)
	if got != `{"a":{"b":1}}` {
		t.Errorf("应正确提取嵌套对象，实际: %s", got)
	}
}

func TestParseLLMOutputWithBackslashNewline(t *testing.T) {
	// 模拟 LLM 在字符串值内部输出 "\<真实换行>"（未完成转义）的情况
	jsonContent := "{\n" +
		`"sections": [` + "\n" +
		`  {` + "\n" +
		`    "title": "产品催化剂",` + "\n" +
		`    "summary": "订单增长\` + "\n" + `产能扩张",` + "\n" +
		`    "key_points": ["点1"],` + "\n" +
		`    "sentiment": "positive"` + "\n" +
		`  }` + "\n" +
		`],` + "\n" +
		`"sources": []` + "\n" +
		`}`

	out, err := parseLLMOutput(jsonContent)
	if err != nil {
		t.Fatalf("含反斜杠+真实换行的 JSON 应被修复并解析成功: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Errorf("应有 1 个 section，实际 %d", len(out.Sections))
	}
	if !strings.Contains(out.Sections[0].Summary, "订单增长") {
		t.Errorf("summary 应保留订单增长: %s", out.Sections[0].Summary)
	}
}

func TestParseLLMOutputWithUnescapedInnerQuote(t *testing.T) {
	// 模拟 LLM 在字符串值内部使用未转义双引号
	jsonContent := `{"sections": [{"title": "产品催化剂", "summary": "公司与"某医院"签订大单", "key_points": ["点1"], "sentiment": "positive"}], "sources": []}`

	out, err := parseLLMOutput(jsonContent)
	if err != nil {
		t.Fatalf("含未转义内层双引号的 JSON 应被修复并解析成功: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Errorf("应有 1 个 section，实际 %d", len(out.Sections))
	}
	if !strings.Contains(out.Sections[0].Summary, "某医院") {
		t.Errorf("summary 应保留某医院: %s", out.Sections[0].Summary)
	}
}

func TestParseLLMOutputWithTabInString(t *testing.T) {
	// 字符串值内部出现真实 Tab
	jsonContent := "{\n" +
		`"sections": [` + "\n" +
		`  {` + "\n" +
		`    "title": "产品催化剂",` + "\n" +
		`    "summary": "订单增长	产能扩张",` + "\n" +
		`    "key_points": ["点1"],` + "\n" +
		`    "sentiment": "positive"` + "\n" +
		`  }` + "\n" +
		`],` + "\n" +
		`"sources": []` + "\n" +
		`}`

	out, err := parseLLMOutput(jsonContent)
	if err != nil {
		t.Fatalf("含真实 Tab 的 JSON 应被修复并解析成功: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Errorf("应有 1 个 section，实际 %d", len(out.Sections))
	}
}

func TestParseLLMOutputWithUnescapedQuoteFollowedByColon(t *testing.T) {
	// 模拟 LLM 在字符串值内部使用未转义双引号，且该引号后紧跟英文冒号，
	// 导致旧版启发式修复误判为字符串结束符，最终报错：
	// invalid character ':' after object key:value pair
	jsonContent := `{"sections": [{"title": "产品催化剂", "summary": "公司简称"A股":拓展业务", "key_points": ["点1"], "sentiment": "positive"}], "sources": []}`

	out, err := parseLLMOutput(jsonContent)
	if err != nil {
		t.Fatalf("含未转义引号且后跟冒号的 JSON 应被修复并解析成功: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Errorf("应有 1 个 section，实际 %d", len(out.Sections))
	}
	if !strings.Contains(out.Sections[0].Summary, "A股") {
		t.Errorf("summary 应保留 A股: %s", out.Sections[0].Summary)
	}
}

func TestParseLLMOutputWithUnescapedNewlines(t *testing.T) {
	// 模拟 LLM 在字符串值内部输出真实换行（未转义为 \n）的情况
	jsonContent := "{\n" +
		`"sections": [` + "\n" +
		`  {` + "\n" +
		`    "title": "产品催化剂",` + "\n" +
		`    "summary": "小米汽车业务持续放量` + "\n" + `新款车型订单超预期",` + "\n" +
		`    "key_points": ["点1", "点2"],` + "\n" +
		`    "sentiment": "positive"` + "\n" +
		`  }` + "\n" +
		`],` + "\n" +
		`"sources": [` + "\n" +
		`  {"title": "来源1", "url": "https://example.com/1", "date": "2025-06-01"}` + "\n" +
		`]` + "\n" +
		`}`

	out, err := parseLLMOutput(jsonContent)
	if err != nil {
		t.Fatalf("含未转义换行的 JSON 应被修复并解析成功: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Errorf("应有 1 个 section，实际 %d", len(out.Sections))
	}
	if out.Sections[0].Title != "产品催化剂" {
		t.Errorf("title 错误: %s", out.Sections[0].Title)
	}
	if out.Sections[0].Sentiment != "positive" {
		t.Errorf("sentiment 应为 positive，实际是 %s", out.Sections[0].Sentiment)
	}
}

func TestSourceFilterRelevance(t *testing.T) {
	f := newSourceFilter("01478.HK", "丘钛科技", 60)

	// 普通查询：包含股票名称 -> 相关
	item := SearchItem{Title: "丘钛科技发布业绩预告", URL: "https://xueqiu.com/123", Content: "内容"}
	if !f.isRelevant(item, "丘钛科技 01478 业绩") {
		t.Error("包含股票名称应判定为相关")
	}

	// 普通查询：不包含股票名称/代码 -> 不相关
	item = SearchItem{Title: "Apple Vision Pro 评测", URL: "https://theverge.com/1", Content: "内容"}
	if f.isRelevant(item, "丘钛科技 01478 业绩") {
		t.Error("普通查询不含股票名称应判定为不相关")
	}

	// 全球产业映射查询：包含海外龙头 -> 相关
	item = SearchItem{Title: "Apple Vision Pro 带动供应链", URL: "https://theverge.com/2", Content: "内容"}
	if !f.isRelevant(item, "丘钛科技 01478 全球产业映射 Apple Vision Pro") {
		t.Error("全球产业映射查询含海外龙头应判定为相关")
	}

	// 全球产业映射查询：包含映射关键词 -> 相关
	item = SearchItem{Title: "手机产业链预期差分析", URL: "https://example.com/1", Content: "内容"}
	if !f.isRelevant(item, "丘钛科技 01478 全球产业映射") {
		t.Error("全球产业映射查询含映射关键词应判定为相关")
	}

	// 普通查询：未出现股票名，但命中多个查询主题关键词 -> 相关
	item = SearchItem{Title: "手机摄像头模组订单饱满 产能持续扩张", URL: "https://example.com/2", Content: "内容"}
	if !f.isRelevant(item, "丘钛科技 01478 新产品 投产 订单 产能") {
		t.Error("命中多个主题关键词应判定为相关")
	}

	// 普通查询：未出现股票名，只命中 1 个普通主题词 -> 不相关
	item = SearchItem{Title: "某公司业绩分析", URL: "https://example.com/3", Content: "内容"}
	if f.isRelevant(item, "丘钛科技 01478 新产品 投产 订单 产能") {
		t.Error("只命中宽泛主题词应判定为不相关")
	}

	// 普通查询：未出现股票名，但命中高置信度主题词 -> 相关
	item = SearchItem{Title: "手机产业链订单供不应求", URL: "https://example.com/4", Content: "内容"}
	if !f.isRelevant(item, "丘钛科技 01478 订单 产能") {
		t.Error("命中高置信度主题词应判定为相关")
	}
}

func TestExtractQueryKeywords(t *testing.T) {
	kws := extractQueryKeywords("正海磁材 300224 新产品 投产 订单 产能", "正海磁材", "300224")
	if len(kws) < 4 {
		t.Errorf("应提取至少 4 个主题关键词，实际 %d", len(kws))
	}
	for _, kw := range []string{"新产品", "投产", "订单", "产能"} {
		found := false
		for _, k := range kws {
			if k == kw {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("应包含主题关键词 %s", kw)
		}
	}
}

func TestSourceFilterRecency(t *testing.T) {
	f := newSourceFilter("01478.HK", "丘钛科技", 60)

	// 普通财经新闻，日期在 60 天内 -> 通过
	item := SearchItem{Title: "新闻", URL: "https://xueqiu.com/1", Published: time.Now().AddDate(0, 0, -10).Format("2006-01-02")}
	if !f.isWithinRecency(item) {
		t.Error("10天前应通过时效过滤")
	}

	// 普通财经新闻，日期超过 60 天 -> 过滤
	item = SearchItem{Title: "旧闻", URL: "https://xueqiu.com/2", Published: time.Now().AddDate(0, 0, -100).Format("2006-01-02")}
	if f.isWithinRecency(item) {
		t.Error("100天前的普通新闻应被过滤")
	}

	// 监管来源，日期超过 60 天 -> 豁免
	item = SearchItem{Title: "问询函", URL: "https://szse.cn/1", Published: time.Now().AddDate(0, 0, -200).Format("2006-01-02")}
	if !f.isWithinRecency(item) {
		t.Error("监管来源应豁免时效过滤")
	}
}

func TestSourceTier(t *testing.T) {
	if sourceTier("https://szse.cn/abc") != 1 {
		t.Errorf("交易所应为 Tier 1，实际是 %d", sourceTier("https://szse.cn/abc"))
	}
	if sourceTier("https://xueqiu.com/abc") != 2 {
		t.Errorf("雪球应为 Tier 2，实际是 %d", sourceTier("https://xueqiu.com/abc"))
	}
	if sourceTier("https://bloomberg.com/abc") != 3 {
		t.Errorf("Bloomberg 应为 Tier 3，实际是 %d", sourceTier("https://bloomberg.com/abc"))
	}
	if sourceTier("https://example.com/abc") != 4 {
		t.Errorf("未知域名应为 Tier 4，实际是 %d", sourceTier("https://example.com/abc"))
	}
}

func TestCollectSources(t *testing.T) {
	results := []SearchResult{
		{
			Query: "韦尔股份 603501 业绩",
			Items: []SearchItem{
				{Title: "韦尔股份业绩超预期", URL: "https://a.com", Content: "content a", Published: "2025-06-01"},
				{Title: "B", URL: "https://b.com", Content: "content b"}, // 不相关，应被过滤
			},
		},
		{
			Query: "韦尔股份 603501 风险",
			Items: []SearchItem{
				{Title: "韦尔股份遭问询", URL: "https://a.com", Content: "dup"}, // 重复 URL
			},
		},
	}

	sources := collectSources(results, "603501.SH", "韦尔股份", 60)
	if len(sources) != 1 {
		t.Errorf("去重并过滤后应剩 1 条来源，实际 %d", len(sources))
	}
}

func TestParseLLMOutputTruncatedMidSection(t *testing.T) {
	// 模拟 max_tokens 截断：第二个 section 写到一半被切断
	truncated := `{"sections": [
		{"title": "产品/业务催化剂", "summary": "第一段摘要", "key_points": ["要点1", "要点2"], "sentiment": "positive"},
		{"title": "国际对标", "summary": "第二段摘要被截断，贵金`
	out, err := parseLLMOutput(truncated)
	if err != nil {
		t.Fatalf("截断的 JSON 应能通过截断修复解析，实际报错: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Fatalf("应保留 1 个完整 section，实际 %d", len(out.Sections))
	}
	if out.Sections[0].Title != "产品/业务催化剂" {
		t.Errorf("section 标题错误: %s", out.Sections[0].Title)
	}
	if out.Sections[0].Sentiment != "positive" {
		t.Errorf("sentiment 应规范化为 positive，实际 %s", out.Sections[0].Sentiment)
	}
}

func TestParseLLMOutputTruncatedInSources(t *testing.T) {
	// sections 完整，sources 数组被截断：应保留全部 sections，sources 尽力保留
	truncated := `{"sections": [
		{"title": "板块一", "summary": "摘要一", "key_points": ["要点1"], "sentiment": "乐观"},
		{"title": "板块二", "summary": "摘要二", "key_points": [], "sentiment": "neutral"}
	],
	"sources": [
		{"title": "来源1", "url": "https://a.com", "date": "2025-07-25"},
		{"title": "来源2", "url": "https://b.com", "da`
	out, err := parseLLMOutput(truncated)
	if err != nil {
		t.Fatalf("截断的 JSON 应能通过截断修复解析，实际报错: %v", err)
	}
	if len(out.Sections) != 2 {
		t.Fatalf("应保留 2 个完整 section，实际 %d", len(out.Sections))
	}
	if out.Sections[0].Sentiment != "positive" {
		t.Errorf("乐观 应规范化为 positive，实际 %s", out.Sections[0].Sentiment)
	}
	if len(out.Sources) != 1 {
		t.Errorf("sources 应保留 1 条完整记录，实际 %d", len(out.Sources))
	}
}

func TestParseLLMOutputTruncatedMidScalar(t *testing.T) {
	// 截断点在标量（数字/布尔）中间：残缺标量应被丢弃
	truncated := `{"sections": [{"title": "板块一", "summary": "摘要", "key_points": [], "sentiment": "neutral"}], "relevance_score": 0.8`
	out, err := parseLLMOutput(truncated)
	if err != nil {
		t.Fatalf("截断的 JSON 应能通过截断修复解析，实际报错: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Fatalf("应保留 1 个完整 section，实际 %d", len(out.Sections))
	}
}

func TestAIConfigNormalizeMigratesMaxTokens(t *testing.T) {
	// 旧版默认 4096 容易导致投研报告 JSON 被截断，应自动迁移到 8192
	cfg := &AIConfig{MaxTokens: 4096}
	cfg.Normalize()
	if cfg.MaxTokens != 8192 {
		t.Errorf("旧默认 4096 应迁移为 8192，实际 %d", cfg.MaxTokens)
	}
	// 用户显式设置的其他值不应被覆盖
	cfg = &AIConfig{MaxTokens: 16384}
	cfg.Normalize()
	if cfg.MaxTokens != 16384 {
		t.Errorf("用户自定义 max_tokens 不应被覆盖，实际 %d", cfg.MaxTokens)
	}
}
