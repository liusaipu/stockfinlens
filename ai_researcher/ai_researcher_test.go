package ai_researcher

import (
	"strings"
	"testing"
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
	if cfg.LLMModel != "moonshot-v1-8k" {
		t.Errorf("Kimi 默认模型错误: %s", cfg.LLMModel)
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

func TestCollectSources(t *testing.T) {
	results := []SearchResult{
		{
			Query: "q1",
			Items: []SearchItem{
				{Title: "A", URL: "https://a.com", Content: "content a", Published: "2025-06-01"},
				{Title: "B", URL: "https://b.com", Content: "content b"},
			},
		},
		{
			Query: "q2",
			Items: []SearchItem{
				{Title: "A2", URL: "https://a.com", Content: "dup"}, // 重复 URL
			},
		},
	}

	sources := collectSources(results)
	if len(sources) != 2 {
		t.Errorf("去重后应剩 2 条来源，实际 %d", len(sources))
	}
}
