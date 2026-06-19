package ai_researcher

import "fmt"

// AIConfig AI 投研功能配置
// 由用户在 Settings 中配置，持久化到 ~/.config/stock-analyzer/ai_config.json
type AIConfig struct {
	Enabled bool `json:"enabled"`

	// LLM 第一层：连接层（用户必填）
	LLMProvider string `json:"llm_provider"` // "kimi" | "deepseek"
	LLMAPIKey   string `json:"llm_api_key"`
	LLMBaseURL  string `json:"llm_base_url"`
	LLMModel    string `json:"llm_model"`
	LLMTimeout  int    `json:"llm_timeout"` // 秒

	// LLM 第二层：生成控制（使用默认值，高级用户可调整）
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`

	// 搜索引擎配置（首期仅支持 tavily）
	SearchProvider    string `json:"search_provider"` // "tavily"
	SearchAPIKey      string `json:"search_api_key"`
	SearchDepth       string `json:"search_depth"`        // "basic" | "advanced"
	SearchTimeout     int    `json:"search_timeout"`      // 秒，Tavily 请求超时
	MaxResults        int    `json:"max_results"`
	SearchRecencyDays int    `json:"search_recency_days"` // 例如 90

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
		LLMModel:          "deepseek-chat",
		LLMTimeout:        90,
		Temperature:       0.2,
		MaxTokens:         4096,
		TopP:              1.0,
		SearchProvider:    "tavily",
		SearchDepth:       "advanced",
		SearchTimeout:     180,
		MaxResults:        10,
		SearchRecencyDays: 90,
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
	"kimi":     {BaseURL: "https://api.moonshot.cn/v1", Model: "moonshot-v1-8k"},
	"deepseek": {BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat"},
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
	if c.MaxTokens <= 0 {
		c.MaxTokens = 4096
	}
	if c.TopP <= 0 || c.TopP > 1 {
		c.TopP = 1.0
	}
	if c.SearchProvider == "" {
		c.SearchProvider = "tavily"
	}
	if c.SearchDepth == "" {
		c.SearchDepth = "advanced"
	}
	if c.SearchTimeout <= 0 {
		c.SearchTimeout = 180
	}
	if c.MaxResults <= 0 || c.MaxResults > 20 {
		c.MaxResults = 10
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
	if c.SearchAPIKey == "" {
		return fmt.Errorf("搜索引擎 API Key 未配置")
	}
	return nil
}
