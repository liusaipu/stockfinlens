package analyzer

// CalcTrace 单个指标的计算溯源记录
type CalcTrace struct {
	Indicator string                `json:"indicator"`
	Year      string                `json:"year"`
	Formula   string                `json:"formula"`
	Inputs    map[string]InputValue `json:"inputs"`
	Steps     []CalcStep            `json:"steps"`
	Result    float64               `json:"result"`
	Note      string                `json:"note"`
}

type InputValue struct {
	Source string  `json:"source"`
	Item   string  `json:"item"`
	Year   string  `json:"year"`
	Value  float64 `json:"value"`
	Note   string  `json:"note"`
}

type CalcStep struct {
	Desc  string  `json:"desc"`
	Expr  string  `json:"expr"`
	Value float64 `json:"value"`
}

// StepResult 单步分析结果
type StepResult struct {
	StepNum    int                       `json:"stepNum"`
	StepName   string                    `json:"stepName"`
	YearlyData map[string]map[string]any `json:"yearlyData"` // 年份 -> 指标名 -> 值
	Conclusion string                    `json:"conclusion"`
	Pass       map[string]bool           `json:"pass"`   // 年份 -> 是否达标
	Traces     []CalcTrace               `json:"traces"` // 计算溯源
}

// AnalysisReport 完整分析报告
type AnalysisReport struct {
	Symbol          string                `json:"symbol"`
	CompanyName     string                `json:"companyName"`
	Years           []string              `json:"years"`
	StepResults     []StepResult          `json:"stepResults"`
	PassSummary     map[string][]PassItem `json:"passSummary"`
	Score           map[string]float64    `json:"score"`
	ScoreDetails    map[string]*YearScore `json:"scoreDetails,omitempty"` // 完整的年度评分详情
	OverallGrade    string                `json:"overallGrade"`
	MarkdownContent string                `json:"markdownContent"`
	RIM             *RIMData              `json:"rim,omitempty"`
	Highlights      []string              `json:"highlights"`
	Risks           []string              `json:"risks"`
	RiskAlert       *RiskAlertSummary     `json:"riskAlert,omitempty"`
	QualityWarnings []string              `json:"qualityWarnings,omitempty"`
	Diff            *AnalysisDiff         `json:"diff,omitempty"` // 与上次分析的差异
	QuarterlyAlert  *QuarterlyAlert      `json:"quarterlyAlert,omitempty"`
	TTMMetrics      *TTMMetrics          `json:"ttmMetrics,omitempty"`
}

// PassItem 单一年度的达标项
type PassItem struct {
	Year   string `json:"year"`
	Passed bool   `json:"passed"`
	Value  any    `json:"value"`
}

// QuoteData 实时行情数据（用于报告填充）
type QuoteData struct {
	CurrentPrice         float64 `json:"currentPrice"`
	ChangePercent        float64 `json:"changePercent"`
	ChangeAmount         float64 `json:"changeAmount"`
	Volume               float64 `json:"volume"`
	TurnoverAmount       float64 `json:"turnoverAmount"`
	TurnoverRate         float64 `json:"turnoverRate"`
	Amplitude            float64 `json:"amplitude"`
	High                 float64 `json:"high"`
	Low                  float64 `json:"low"`
	Open                 float64 `json:"open"`
	PreviousClose        float64 `json:"previousClose"`
	CirculatingMarketCap float64 `json:"circulatingMarketCap"`
	VolumeRatio          float64 `json:"volumeRatio"`
	PE                   float64 `json:"pe"`
	PB                   float64 `json:"pb"`
	MarketCap            float64 `json:"marketCap"`
}

// SentimentSummary 单条舆情摘要
type SentimentSummary struct {
	Title     string  `json:"title"`
	Source    string  `json:"source"`
	Date      string  `json:"date"`
	Sentiment float64 `json:"sentiment"` // [-1, 1]
}

// SentimentData 社交媒体/舆情情绪数据
type SentimentData struct {
	Score         float64            `json:"score"`
	HeatIndex     int                `json:"heatIndex"`
	PositiveWords []string           `json:"positiveWords"`
	NegativeWords []string           `json:"negativeWords"`
	Summaries     []SentimentSummary `json:"summaries"`
	HasData       bool               `json:"hasData"`
}

// MLSummary 基于双引擎融合的2-4周综合预测摘要
type MLSummary struct {
	Direction string  `json:"direction"`
	RangeLow  float64 `json:"rangeLow"`
	RangeHigh float64 `json:"rangeHigh"`
	Reason    string  `json:"reason"`
	HasData   bool    `json:"hasData"`
}

// MLPredictionData 机器学习预测结果
type MLPredictionData struct {
	Sentiment *MLSentimentPrediction `json:"sentiment,omitempty"`
	Financial *MLFinancialPrediction `json:"financial,omitempty"`
	EngineD   *MLDRiskPrediction     `json:"engine_d,omitempty"`
	Summary   *MLSummary             `json:"summary,omitempty"`
	MLError   string                 `json:"ml_error,omitempty"` // Python 推理失败的错误信息
	// Confidence 模型输入完整度/置信等级：high（数据充分）/ medium（部分默认值）/ low（大量缺失）
	Confidence string `json:"confidence,omitempty"`
}

// RIMData 剩余收益模型数据
type RIMData struct {
	HasData bool               `json:"hasData"`
	Params  RIMParams          `json:"params"`
	Result  *RIMResult         `json:"result,omitempty"`
	Error   string             `json:"error,omitempty"`
	EPSRaw  map[string]float64 `json:"epsRaw,omitempty"`
	Rf      float64            `json:"rf"`
	Beta    float64            `json:"beta"`
	RmRf    float64            `json:"rmRf"`
}

// MoneyflowItem 单日资金流向数据
type MoneyflowItem struct {
	Date         string  `json:"date"`
	MainInflow   float64 `json:"main_inflow"`   // 主力净流入（大单+特大单）
	SmNetAmount  float64 `json:"sm_net_amount"` // 小单净流入
	MdNetAmount  float64 `json:"md_net_amount"` // 中单净流入
	LgNetAmount  float64 `json:"lg_net_amount"` // 大单净流入
	ElgNetAmount float64 `json:"elg_net_amount"` // 特大单净流入
}

// MoneyflowData 个股资金流向分析数据
type MoneyflowData struct {
	HasData bool            `json:"has_data"`
	Items   []MoneyflowItem `json:"items"`
	Summary string          `json:"summary"` // 简要分析
}

// RiskAlertFlag 单条风险标记
type RiskAlertFlag struct {
	Code    string   `json:"code"`    // 指标编码，如 "ascore_high"
	Name    string   `json:"name"`    // 人类可读名称，如 "A-Score 偏高"
	Value   float64  `json:"value"`   // 具体数值
	Format  string   `json:"format"`  // 格式化模板，如 "A-Score %.0f"
	Level   string   `json:"level"`   // high / medium / info
	Source  string   `json:"source"`  // 数据来源，如 "step8" / "crawler"
	Details []string `json:"details"` // 原始详情列表（如公告标题）
	Note    string   `json:"note"`    // 提示说明（用于 hover tooltip，如正常审计轮换的原因解释）
}

// RiskAlertSummary 风险警示摘要
type RiskAlertSummary struct {
	Level      string          `json:"level"`      // high / medium / low
	Score      float64         `json:"score"`      // A-Score 值
	OneVeto    bool            `json:"oneVeto"`    // 是否一票否决
	Flags      []RiskAlertFlag `json:"flags"`      // 触发的风险项
	PrimaryMsg string          `json:"primaryMsg"` // 主提示文案
}

// AuditorChangeDetail 审计机构变更详情
type AuditorChangeDetail struct {
	Date                 string `json:"date"`
	OldAuditor           string `json:"old_auditor"`
	NewAuditor           string `json:"new_auditor"`
	Reason               string `json:"reason"`
	IsBeforeAnnualReport bool   `json:"is_before_annual_report"`
	AnnualReportDeadline string `json:"annual_report_deadline"`
	RawTitle             string `json:"raw_title"`
	IsPolicyCompliance   bool   `json:"is_policy_compliance"` // 是否为政策合规更换（如国企8年强制轮换）
	IsAbnormal           bool   `json:"is_abnormal"`          // 是否为异常更换（需警惕）
	IsPassiveChange      bool   `json:"is_passive_change"`    // 是否为被动更换（原事务所被处罚/禁入等，非公司自身问题）
	IsNormalRotation     bool   `json:"is_normal_rotation"`   // 是否为正常轮换（看似异常实则正常，如招标选聘、连续服务多年后更换）
}

// ExternalRiskData 外部风险数据（审计机构、高管变动、诉讼等）
type ExternalRiskData struct {
	AuditorChanged      bool                  `json:"auditorChanged"`      // 近3年是否更换审计机构
	AuditorName         string                `json:"auditorName"`         // 当前审计机构名称
	AuditorChangeDetails []AuditorChangeDetail `json:"auditorChangeDetails"` // 审计机构变更详情
	ExecChanged         bool     `json:"execChanged"`         // 近1年财务负责人是否频繁更换
	ExecChangeCount     int      `json:"execChangeCount"`     // 高管变动次数
	ExecHistory         []string `json:"execHistory"`         // 高管变动原始公告列表
	HasLitigation         bool     `json:"hasLitigation"`         // 是否存在诉讼/违规担保
	LitigationCount       int      `json:"litigationCount"`       // 高风险诉讼/担保公告数量
	LitigationHistory     []string `json:"litigationHistory"`     // 诉讼/担保原始公告列表
	HasHighRiskGuarantee  bool     `json:"hasHighRiskGuarantee"`  // 是否存在高风险担保（违规/逾期/代偿）
	HasGuarantee          bool     `json:"hasGuarantee"`          // 是否存在普通对外担保
	HasFundOccupation     bool     `json:"hasFundOccupation"`     // 是否存在资金占用
	HasInternalBuy      bool     `json:"hasInternalBuy"`      // 近半年是否有内部人增持
	HasInternalSell     bool     `json:"hasInternalSell"`     // 近半年是否有内部人大额减持
	SealControlRumor    bool     `json:"sealControlRumor"`    // 是否存在印章失控传闻（舆情）
	Error               string   `json:"error,omitempty"`     // 数据获取错误
}

// AuditOpinion 单年度审计意见
type AuditOpinion struct {
	Year        string   `json:"year"`
	Opinion     string   `json:"opinion"`
	Auditor     string   `json:"auditor"`
	IsStandard  bool     `json:"isStandard"`
	NeedsReview bool     `json:"needsReview"`
	Emphasis    []string `json:"emphasis,omitempty"`
	KeyMatters  []string `json:"keyMatters,omitempty"`
}

// SensitivityLevel 风险警示敏感度
type SensitivityLevel string

const (
	SensitivityStrict  SensitivityLevel = "strict"  // 严格
	SensitivityStandard SensitivityLevel = "standard" // 标准
	SensitivityLoose   SensitivityLevel = "loose"   // 宽松
)

// MetricChange 单指标变化记录
type MetricChange struct {
	Name     string  `json:"name"`
	Previous float64 `json:"previous"`
	Current  float64 `json:"current"`
	Delta    float64 `json:"delta"`    // 绝对变化
	DeltaPct float64 `json:"deltaPct"` // 百分比变化（基点或百分比）
	Significant bool `json:"significant"` // 是否超过阈值
}

// AnalysisDiff 两次分析之间的差异摘要
type AnalysisDiff struct {
	HasPrevious       bool             `json:"hasPrevious"`
	PreviousTime      string           `json:"previousTime,omitempty"`
	CurrentTime       string           `json:"currentTime,omitempty"`
	ScoreChange       float64          `json:"scoreChange"`       // 总分变化（当前 - 上次）
	GradeChanged      bool             `json:"gradeChanged"`
	PreviousGrade     string           `json:"previousGrade,omitempty"`
	CurrentGrade      string           `json:"currentGrade,omitempty"`
	NewFlags          []RiskAlertFlag  `json:"newFlags,omitempty"`      // 新增风险
	ResolvedFlags     []RiskAlertFlag  `json:"resolvedFlags,omitempty"` // 解除的风险
	PersistentFlags   []RiskAlertFlag  `json:"persistentFlags,omitempty"` // 持续风险
	KeyMetricChanges  []MetricChange   `json:"keyMetricChanges,omitempty"`
}
