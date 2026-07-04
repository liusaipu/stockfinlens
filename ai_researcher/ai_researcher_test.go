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
	if len(queries) < 4 {
		t.Errorf("应至少生成 4 个查询，实际 %d", len(queries))
	}

	hasProduct := false
	hasPolicy := false
	hasRisk := false
	hasGlobalMapping := false
	hasCompetitor := false
	hasSocial := false
	for _, q := range queries {
		if strings.Contains(q, "产品") || strings.Contains(q, "投产") || strings.Contains(q, "催化") {
			hasProduct = true
		}
		if strings.Contains(q, "政策") || strings.Contains(q, "营收") {
			hasPolicy = true
		}
		if strings.Contains(q, "证监会") || strings.Contains(q, "处罚") || strings.Contains(q, "ST") || strings.Contains(q, "退市") {
			hasRisk = true
		}
		if strings.Contains(q, "全球产业映射") || strings.Contains(q, "Nvidia") || strings.Contains(q, "OpenAI") {
			hasGlobalMapping = true
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
