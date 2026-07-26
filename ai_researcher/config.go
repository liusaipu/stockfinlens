package ai_researcher

import (
	"fmt"
	"strings"
	"time"
)

// AIConfig AI 投研功能配置
// 由用户在 Settings 中配置，持久化到 ~/.config/stock-analyzer/ai_config.json
type AIConfig struct {
	Enabled bool `json:"enabled"`

	// LLM 第一层：连接层（用户必填）
	LLMProvider string `json:"llm_provider"` // "kimi" | "kimi-code" | "deepseek"
	LLMAPIKey   string `json:"llm_api_key"`
	LLMBaseURL  string `json:"llm_base_url"`
	LLMModel    string `json:"llm_model"`
	LLMTimeout  int    `json:"llm_timeout"` // 秒

	// LLM 第二层：生成控制（使用默认值，高级用户可调整）
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`

	// 搜索引擎配置（首期仅支持 tavily）
	SearchProvider    string   `json:"search_provider"` // "tavily"
	SearchAPIKey      string   `json:"search_api_key"`  // 兼容旧配置：单 Key
	SearchAPIKeys     []string `json:"search_api_keys"` // 多 Key 备用（最多 5 个）
	SearchDepth       string   `json:"search_depth"`    // "basic" | "advanced"
	SearchTimeout     int      `json:"search_timeout"`  // 秒，Tavily 请求超时
	MaxResults        int      `json:"max_results"`
	SearchRecencyDays int      `json:"search_recency_days"` // 例如 90

	// 本月已额度用尽的 Tavily Key（key -> "YYYY-MM"），下月自动重试
	ExhaustedSearchKeys map[string]string `json:"exhausted_search_keys"`

	// 业务偏好
	FocusRegions   []string `json:"focus_regions"`   // ["us","jp","eu","hk"]
	OutputLanguage string   `json:"output_language"` // "zh-CN"
	EnableSocial   bool     `json:"enable_social"`   // 是否抓取社交情绪
	CacheTTLHours  int      `json:"cache_ttl_hours"` // 缓存有效期，默认 6
}

// DefaultAIConfig 返回默认配置
func DefaultAIConfig() *AIConfig {
	return &AIConfig{
		Enabled:           false,
		LLMProvider:       "deepseek",
		LLMBaseURL:        "https://api.deepseek.com/v1",
		LLMModel:          "deepseek-v4-pro",
		LLMTimeout:        90,
		Temperature:       0.2,
		MaxTokens:         8192,
		TopP:              1.0,
		SearchProvider:    "tavily",
		SearchDepth:       "advanced",
		SearchTimeout:     180,
		MaxResults:        20,
		SearchRecencyDays: 180,
		FocusRegions:      []string{"us", "jp"},
		OutputLanguage:    "zh-CN",
		EnableSocial:      true,
		CacheTTLHours:     6,
	}
}

// ProviderDefaults 不同 LLM 供应商的默认配置
var ProviderDefaults = map[string]struct {
	BaseURL string
	Model   string
}{
	"kimi":      {BaseURL: "https://api.moonshot.cn/v1", Model: "kimi-k2.6"},
	"kimi-code": {BaseURL: "https://api.kimi.com/coding/v1", Model: "kimi-k2.6"},
	"deepseek":  {BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-v4-pro"},
}

// Normalize 补全缺失的默认值并校验
func (c *AIConfig) Normalize() {
	if c.LLMProvider == "" {
		c.LLMProvider = "deepseek"
	}
	if defaults, ok := ProviderDefaults[c.LLMProvider]; ok {
		if c.LLMBaseURL == "" {
			c.LLMBaseURL = defaults.BaseURL
		}
		if c.LLMModel == "" {
			c.LLMModel = defaults.Model
		}
	}
	if c.LLMTimeout <= 0 {
		c.LLMTimeout = 90
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		c.Temperature = 0.2
	}
	// 4096 是旧版默认值，投研报告 JSON 较长容易被截断，自动迁移到 8192
	if c.MaxTokens <= 0 || c.MaxTokens == 4096 {
		c.MaxTokens = 8192
	}
	if c.TopP <= 0 || c.TopP > 1 {
		c.TopP = 1.0
	}
	if c.SearchProvider == "" {
		c.SearchProvider = "tavily"
	}
	// 兼容旧配置：如果单 Key 存在但多 Key 为空，则迁移到多 Key 列表
	if c.SearchAPIKey != "" {
		found := false
		for _, k := range c.SearchAPIKeys {
			if k == c.SearchAPIKey {
				found = true
				break
			}
		}
		if !found {
			c.SearchAPIKeys = append([]string{c.SearchAPIKey}, c.SearchAPIKeys...)
		}
	}
	// 清理空 Key 并限制最多 5 个
	var keys []string
	seen := make(map[string]bool)
	for _, k := range c.SearchAPIKeys {
		k = strings.TrimSpace(k)
		if k != "" && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	if len(keys) > 5 {
		keys = keys[:5]
	}
	c.SearchAPIKeys = keys
	// 保持单 Key 字段与多 Key 列表第一个一致，便于旧代码/前端过渡
	if len(c.SearchAPIKeys) > 0 {
		c.SearchAPIKey = c.SearchAPIKeys[0]
	} else {
		c.SearchAPIKey = ""
	}
	if c.SearchDepth == "" {
		c.SearchDepth = "advanced"
	}
	if c.SearchTimeout <= 0 {
		c.SearchTimeout = 180
	}
	if c.MaxResults <= 0 || c.MaxResults > 20 {
		c.MaxResults = 20
	}
	if c.SearchRecencyDays <= 0 {
		c.SearchRecencyDays = 90
	}
	if c.OutputLanguage == "" {
		c.OutputLanguage = "zh-CN"
	}
	if c.CacheTTLHours <= 0 {
		c.CacheTTLHours = 6
	}
	if len(c.FocusRegions) == 0 {
		c.FocusRegions = []string{"us", "jp"}
	}
	// 清理过期的超额 Key 标记（ Tavily 每月 1 日重置额度）
	if c.ExhaustedSearchKeys == nil {
		c.ExhaustedSearchKeys = make(map[string]string)
	}
	currentMonth := time.Now().Format("2006-01")
	for k, month := range c.ExhaustedSearchKeys {
		if month != currentMonth {
			delete(c.ExhaustedSearchKeys, k)
		}
	}
}

// Validate 校验配置是否可用
func (c *AIConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.LLMProvider == "" {
		return fmt.Errorf("LLM 供应商未配置")
	}
	if c.LLMAPIKey == "" {
		return fmt.Errorf("LLM API Key 未配置")
	}
	if c.LLMBaseURL == "" {
		return fmt.Errorf("LLM Base URL 未配置")
	}
	if c.LLMModel == "" {
		return fmt.Errorf("LLM 模型未配置")
	}
	// 同时兼容 SearchAPIKeys 数组和旧的 SearchAPIKey 单字段
	hasKey := len(c.SearchAPIKeys) > 0 || c.SearchAPIKey != ""
	if !hasKey {
		return fmt.Errorf("搜索引擎 API Key 未配置")
	}
	return nil
}
