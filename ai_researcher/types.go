package ai_researcher

// ResearchSection AI 投研报告的一个模块
type ResearchSection struct {
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	KeyPoints []string `json:"key_points"`
	Sentiment string   `json:"sentiment"` // "positive" | "neutral" | "negative"
}

// ResearchSource 信息来源
type ResearchSource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Date  string `json:"date"` // 尽可能提取的发布日期，格式 YYYY-MM-DD
}

// AIResearchReport AI 投研结构化报告
type AIResearchReport struct {
	Symbol      string            `json:"symbol"`
	Name        string            `json:"name"`
	GeneratedAt string            `json:"generated_at"` // ISO8601
	ModelUsed   string            `json:"model_used"`
	FromCache   bool              `json:"from_cache"`
	Sections    []ResearchSection `json:"sections"`
	Sources     []ResearchSource  `json:"sources"`
}

// SearchResult 单条搜索结果
type SearchResult struct {
	Query   string       `json:"query"`
	Items   []SearchItem `json:"items"`
	RawJSON string       `json:"raw_json"`
}

// SearchItem Tavily 返回的单条结果
type SearchItem struct {
	Title     string  `json:"title"`
	URL       string  `json:"url"`
	Content   string  `json:"content"`
	Score     float64 `json:"score"`
	Published string  `json:"published"`
}

// TestConnectionResult 连接测试结果
type TestConnectionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ProgressFunc 进度回调函数类型
type ProgressFunc func(stage string, message string)

// AIProgressEvent AI 投研进度事件（通过 Wails 事件推送到前端）
type AIProgressEvent struct {
	Symbol  string `json:"symbol"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}
