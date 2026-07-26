package analyzer

import (
	"strings"
	"testing"
)

// TestFormatSignedYi 资金流向金额格式化：单位为元 → 带符号的亿
func TestFormatSignedYi(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{1.23e8, "+1.23亿"},
		{-1.23e8, "-1.23亿"},
		{19173900, "+0.19亿"},
		{-19173900, "-0.19亿"},
		{0, "0.00亿"},
		{4e5, "0.00亿"}, // 小于 0.005 亿视为 0
	}
	for _, c := range cases {
		if got := formatSignedYi(c.in); got != c.want {
			t.Errorf("formatSignedYi(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestComputeAnalysisDiffPreviousTime 上次分析时间应来自快照的 GeneratedAt，而不是公司名/代码
func TestComputeAnalysisDiffPreviousTime(t *testing.T) {
	prev := &AnalysisReport{
		Symbol:      "600531.SH",
		CompanyName: "600531.SH",
		GeneratedAt: "2026-06-21 09:56",
		Score:       map[string]float64{"weighted": 70},
	}
	cur := &AnalysisReport{
		Symbol: "600531.SH",
		Score:  map[string]float64{"weighted": 77},
	}
	diff := ComputeAnalysisDiff(cur, prev)
	if diff.PreviousTime != "2026-06-21 09:56" {
		t.Errorf("PreviousTime 应为快照生成时间，实际 %q", diff.PreviousTime)
	}
}

// 构造最小可用的 StepResult 集
func buildStepsForReportTest() []StepResult {
	mk := func(stepNum int, kv map[string]any) StepResult {
		return StepResult{
			StepNum: stepNum,
			YearlyData: map[string]map[string]any{
				"2025-12-31": kv,
			},
		}
	}
	return []StepResult{
		mk(3, map[string]any{"debtRatio": 50.0, "cashDebtDiff": 1e9}),
		mk(8, map[string]any{"AScore": 20.0}),
		mk(9, map[string]any{"growthRate": 25.0, "revenueGrowth": 0.25}),
		mk(10, map[string]any{"grossMargin": 45.0}),
		mk(15, map[string]any{"cashRatio": 120.0}),
		mk(16, map[string]any{"roe": 18.0}),
	}
}

// TestWriteModule9EarnedScore 选股条件得分列应显示"该项得分/满分"，而不是累计分
func TestWriteModule9EarnedScore(t *testing.T) {
	steps := buildStepsForReportTest()
	var b strings.Builder
	writeModule9(&b, steps, "2025-12-31", "")
	out := b.String()
	// ROE 18 >= 15 达标，应显示 20/20（该项得分/满分），而不是累计口径
	if !strings.Contains(out, "| ① ROE ≥ 15% | ≥15% | 18.00% | ✅ | 20/20 |") {
		t.Errorf("达标项应显示该项得分 20/20，实际输出:\n%s", out)
	}
	// 未达标项必须显示 0/满分，不得出现累计分大于满分（如 30/15）
	if strings.Contains(out, "30/15") || strings.Contains(out, "40/10") || strings.Contains(out, "30/10") {
		t.Errorf("得分列不应显示超过满分的累计分，实际输出:\n%s", out)
	}
}

// TestWriteModule11SafetyMargin 模块12安全边际：有行情时基于 PE/PB，总分统一 /100 口径
func TestWriteModule11SafetyMargin(t *testing.T) {
	steps := buildStepsForReportTest()
	var b strings.Builder
	quote := &QuoteData{CurrentPrice: 10, PE: 12, PB: 1.5}
	writeModule11(&b, steps, "2025-12-31", nil, quote)
	out := b.String()
	if strings.Contains(out, "暂缺股价") || strings.Contains(out, "未接入实时股价") {
		t.Errorf("有实时行情时不应再显示暂缺股价，实际输出:\n%s", out)
	}
	if !strings.Contains(out, "PE 12.0 / PB 1.50") {
		t.Errorf("安全边际应展示 PE/PB，实际输出:\n%s", out)
	}
	if !strings.Contains(out, "/100") {
		t.Errorf("总分应统一为 /100 口径，实际输出:\n%s", out)
	}

	// 无行情时保留降级提示
	var b2 strings.Builder
	writeModule11(&b2, steps, "2025-12-31", nil, nil)
	if !strings.Contains(b2.String(), "暂缺股价") {
		t.Errorf("无行情时应显示暂缺股价提示")
	}
}

// TestWriteModule4SmallSample 可比公司不足 3 家时：给出提示、隐藏百分位、不下排名结论
func TestWriteModule4SmallSample(t *testing.T) {
	steps := buildStepsForReportTest()
	comp := &ComparableAnalysis{
		HasData: true,
		Metrics: map[string]*ComparableMetrics{
			"000001.SZ": {Symbol: "000001.SZ", Name: "可比A", ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10},
		},
		Average: &ComparableMetrics{ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10},
		Max:     &ComparableMetrics{ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10},
		Min:     &ComparableMetrics{ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10},
	}
	var b strings.Builder
	writeModule4(&b, steps, "2025-12-31", comp, nil, -1)
	out := b.String()
	if !strings.Contains(out, "样本不足") {
		t.Errorf("样本不足时应给出提示，实际输出:\n%s", out)
	}
	if strings.Contains(out, "排名第") {
		t.Errorf("样本不足时不应输出排名结论，实际输出:\n%s", out)
	}
}

// TestWriteModule4FullSample 样本充足时正常显示百分位和排名
func TestWriteModule4FullSample(t *testing.T) {
	steps := buildStepsForReportTest()
	mk := func(sym string) *ComparableMetrics {
		return &ComparableMetrics{Symbol: sym, Name: sym, ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10}
	}
	comp := &ComparableAnalysis{
		HasData: true,
		Metrics: map[string]*ComparableMetrics{"A": mk("A"), "B": mk("B"), "C": mk("C")},
		Average: &ComparableMetrics{ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10},
		Max:     &ComparableMetrics{ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10},
		Min:     &ComparableMetrics{ROE: 20, GrossMargin: 30, RevenueGrowth: 10, DebtRatio: 40, CashRatio: 100, AScore: 10},
	}
	var b strings.Builder
	writeModule4(&b, steps, "2025-12-31", comp, nil, -1)
	out := b.String()
	if strings.Contains(out, "样本不足") {
		t.Errorf("样本充足时不应出现样本不足提示")
	}
	if !strings.Contains(out, "排名第") {
		t.Errorf("样本充足时应输出排名结论")
	}
}

// TestAScoreModuleNumbering 模块7 的子节编号应为 7.x（不与模块8 撞号）
func TestAScoreModuleNumbering(t *testing.T) {
	steps := []StepResult{
		{}, {}, {}, {}, {}, {}, {},
		{
			StepNum: 8,
			YearlyData: map[string]map[string]any{
				"2025-12-31": {"AScore": 21.8, "MScore": -2.4, "ZScore": 2.84},
			},
		},
	}
	var b strings.Builder
	writeAScoreProfile(&b, steps, []string{"2025-12-31"}, "2025-12-31", nil)
	out := b.String()
	if strings.Contains(out, "## 8.") {
		t.Errorf("模块7 子节不应使用 8.x 编号，实际输出:\n%s", out)
	}
	if !strings.Contains(out, "## 7.1") {
		t.Errorf("模块7 子节应使用 7.x 编号，实际输出:\n%s", out)
	}
}

// TestMoneyflowUnitAndSign 资金流向表：单位应为亿（元/1e8）且负值带负号
func TestMoneyflowUnitAndSign(t *testing.T) {
	mf := &MoneyflowData{
		HasData: true,
		Items: []MoneyflowItem{
			{Date: "20260724", MainInflow: -19173900, ElgNetAmount: 1e8, LgNetAmount: -5e7, MdNetAmount: 2e7, SmNetAmount: -1e7},
		},
	}
	var b strings.Builder
	quote := &QuoteData{CurrentPrice: 10, High: 11, Low: 9, Open: 10, PreviousClose: 10, TurnoverRate: 2}
	writeModule7(&b, quote, nil, nil, mf)
	out := b.String()
	if !strings.Contains(out, "-0.19亿") {
		t.Errorf("主力净流入 -1917.39万 应显示为 -0.19亿，实际输出:\n%s", out)
	}
	if strings.Contains(out, "1917.39") {
		t.Errorf("不应再出现 /1e4 的错误量级，实际输出:\n%s", out)
	}
}
