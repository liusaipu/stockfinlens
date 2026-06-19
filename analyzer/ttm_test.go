package analyzer

import (
	"math"
	"strings"
	"testing"
)

// ttmFixture 构造测试用 FinancialData。
// periods 应按降序提供。每个 map 的 key 是期间（如 "2025-12-31"），value 是该期累计值。
type ttmFixture struct {
	periods     []string // 全部期间（含年报+季报），降序
	years       []string // 仅 -12-31 / 4 位年份，降序
	revenue     map[string]float64
	profit      map[string]float64
	cash        map[string]float64
	equity      map[string]float64 // 所有者权益合计（>=0 时设置，0 或缺失时走 fallback）
	totalAssets map[string]float64 // 资产合计（fallback 用）
	totalLiab   map[string]float64 // 负债合计（fallback 用）
}

func buildFD(f ttmFixture) *FinancialData {
	fd := &FinancialData{
		Quarters:        append([]string(nil), f.periods...),
		Years:           append([]string(nil), f.years...),
		IncomeStatement: map[string]map[string]float64{},
		CashFlow:        map[string]map[string]float64{},
		BalanceSheet:    map[string]map[string]float64{},
	}
	if f.revenue != nil {
		fd.IncomeStatement["营业收入"] = copyMap(f.revenue)
	}
	if f.profit != nil {
		fd.IncomeStatement["净利润"] = copyMap(f.profit)
	}
	if f.cash != nil {
		fd.CashFlow["经营活动产生的现金流量净额"] = copyMap(f.cash)
	}
	if f.equity != nil {
		fd.BalanceSheet["所有者权益合计"] = copyMap(f.equity)
	}
	if f.totalAssets != nil {
		fd.BalanceSheet["资产合计"] = copyMap(f.totalAssets)
	}
	if f.totalLiab != nil {
		fd.BalanceSheet["负债合计"] = copyMap(f.totalLiab)
	}
	return fd
}

func copyMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

// TestBuildTTM_AnnualLatest 最新期为年报时，直接用年报全年口径
func TestBuildTTM_AnnualLatest(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2025-12-31", "2024-12-31"},
		years:   []string{"2025-12-31", "2024-12-31"},
		revenue: map[string]float64{"2025-12-31": 1000, "2024-12-31": 800},
		profit:  map[string]float64{"2025-12-31": 100, "2024-12-31": 80},
		cash:    map[string]float64{"2025-12-31": 150, "2024-12-31": 120},
		equity:  map[string]float64{"2025-12-31": 500, "2024-12-31": 400},
	})
	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	if m.Mode != TTMModeAnnual {
		t.Errorf("Mode 应为 %s，实际 %s", TTMModeAnnual, m.Mode)
	}
	if m.EndPeriod != "2025-12-31" {
		t.Errorf("EndPeriod 应为 2025-12-31，实际 %s", m.EndPeriod)
	}
	if !approxEq(m.Revenue, 1000) {
		t.Errorf("Revenue 应为 1000，实际 %f", m.Revenue)
	}
	if !approxEq(m.NetProfit, 100) {
		t.Errorf("NetProfit 应为 100，实际 %f", m.NetProfit)
	}
	if !approxEq(m.OperatingCash, 150) {
		t.Errorf("OperatingCash 应为 150，实际 %f", m.OperatingCash)
	}
	if !approxEq(m.ROE, 0.2) {
		t.Errorf("ROE 应为 0.2 (100/500)，实际 %f", m.ROE)
	}
	if !approxEq(m.NetMargin, 0.1) {
		t.Errorf("NetMargin 应为 0.1 (100/1000)，实际 %f", m.NetMargin)
	}
	if !approxEq(m.CashRatio, 1.5) {
		t.Errorf("CashRatio 应为 1.5 (150/100)，实际 %f", m.CashRatio)
	}
}

// TestBuildTTM_StandardQuarterly 季报为最新期且去年同期、上一年报齐全 → 标准 TTM 公式
//
// 数据形如 002598：
//
//	2026-03-31 (Q1 2026 累计 3 个月) = 412
//	2025-12-31 (2025 年报全年)        = 1938
//	2025-03-31 (Q1 2025 累计 3 个月) = 400  ← 测试用，002598 实际缺失
//
// 期望 TTM = 1938 + 412 − 400 = 1950（覆盖 2025-04 ~ 2026-03 共 12 个月）
func TestBuildTTM_StandardQuarterly(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2026-03-31", "2025-12-31", "2025-09-30", "2025-06-30", "2025-03-31", "2024-12-31"},
		years:   []string{"2025-12-31", "2024-12-31"},
		revenue: map[string]float64{
			"2026-03-31": 412,
			"2025-12-31": 1938,
			"2025-09-30": 1458,
			"2025-06-30": 968,
			"2025-03-31": 400,
			"2024-12-31": 2086,
		},
		profit: map[string]float64{
			"2026-03-31": 40,
			"2025-12-31": 200,
			"2025-03-31": 35,
		},
		cash: map[string]float64{
			"2026-03-31": 60,
			"2025-12-31": 250,
			"2025-03-31": 50,
		},
		equity: map[string]float64{
			"2026-03-31": 1500,
		},
	})
	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	if m.Mode != TTMModeQuarterly {
		t.Errorf("Mode 应为 %s，实际 %s", TTMModeQuarterly, m.Mode)
	}
	if m.EndPeriod != "2026-03-31" {
		t.Errorf("EndPeriod 应为 2026-03-31，实际 %s", m.EndPeriod)
	}
	if m.PeriodCount != 3 {
		t.Errorf("PeriodCount 应为 3，实际 %d", m.PeriodCount)
	}
	wantPeriods := []string{"2025-12-31", "2026-03-31", "2025-03-31"}
	if len(m.Periods) != 3 || m.Periods[0] != wantPeriods[0] || m.Periods[1] != wantPeriods[1] || m.Periods[2] != wantPeriods[2] {
		t.Errorf("Periods 应为 %v，实际 %v", wantPeriods, m.Periods)
	}
	// Revenue: 1938 + 412 − 400 = 1950
	if !approxEq(m.Revenue, 1950) {
		t.Errorf("Revenue 应为 1950 (1938+412-400)，实际 %f", m.Revenue)
	}
	// NetProfit: 200 + 40 − 35 = 205
	if !approxEq(m.NetProfit, 205) {
		t.Errorf("NetProfit 应为 205 (200+40-35)，实际 %f", m.NetProfit)
	}
	// OperatingCash: 250 + 60 − 50 = 260
	if !approxEq(m.OperatingCash, 260) {
		t.Errorf("OperatingCash 应为 260 (250+60-50)，实际 %f", m.OperatingCash)
	}
	// 用最新期 2026-03-31 的净资产 1500 计算 ROE = 205/1500
	if !approxEq(m.ROE, 205.0/1500.0) {
		t.Errorf("ROE 应为 205/1500，实际 %f", m.ROE)
	}
}

// TestBuildTTM_EquityFallback 模拟东财实际数据：季报「所有者权益合计」为 0，依赖 资产合计−负债合计 推算
func TestBuildTTM_EquityFallback(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2026-03-31", "2025-12-31", "2025-03-31", "2024-12-31"},
		years:   []string{"2025-12-31", "2024-12-31"},
		revenue: map[string]float64{"2026-03-31": 412, "2025-12-31": 1938, "2025-03-31": 400},
		profit:  map[string]float64{"2026-03-31": 40, "2025-12-31": 200, "2025-03-31": 35},
		// 模拟东财真实数据：季报所有者权益合计 = 0
		equity:      map[string]float64{"2026-03-31": 0, "2025-12-31": 800},
		totalAssets: map[string]float64{"2026-03-31": 2000},
		totalLiab:   map[string]float64{"2026-03-31": 500},
	})
	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	if m.Mode != TTMModeQuarterly {
		t.Fatalf("Mode 应为 quarterly，实际 %s", m.Mode)
	}
	// TTM NetProfit = 200 + 40 - 35 = 205
	if !approxEq(m.NetProfit, 205) {
		t.Errorf("NetProfit 应为 205，实际 %f", m.NetProfit)
	}
	// equity = 资产合计 − 负债合计 = 2000 - 500 = 1500
	// ROE = 205 / 1500 = 0.136666...
	if !approxEq(m.ROE, 205.0/1500.0) {
		t.Errorf("ROE 应为 205/1500=%.4f（走 fallback），实际 %.4f", 205.0/1500.0, m.ROE)
	}
}

// TestBuildTTM_SanityNotOvercounting 防止旧版 bug 回归：
// 即使全部 4 个 YTD 季报都在，标准公式也不能等价于"4 个累计值相加"那种 30 个月的重复计算。
func TestBuildTTM_SanityNotOvercounting(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2025-12-31", "2025-09-30", "2025-06-30", "2025-03-31", "2024-12-31", "2024-03-31"},
		years:   []string{"2025-12-31", "2024-12-31"},
		revenue: map[string]float64{
			"2025-12-31": 1200, // 全年
			"2025-09-30": 900,
			"2025-06-30": 600,
			"2025-03-31": 300,
			"2024-12-31": 1100,
			"2024-03-31": 280,
		},
	})
	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	// 最新期是 2025-12-31 年报 → 应直接采用 1200
	if m.Mode != TTMModeAnnual {
		t.Errorf("Mode 应为 annual，实际 %s", m.Mode)
	}
	if !approxEq(m.Revenue, 1200) {
		t.Errorf("Revenue 应为 1200（年报全年），实际 %f；若出现 3000 说明旧 bug 复现", m.Revenue)
	}
	// 旧 bug 会把 300+600+900+1200 = 3000 当成 TTM —— 用来保证回归不可能成立
	if approxEq(m.Revenue, 3000) {
		t.Errorf("出现旧版累加 bug：Revenue=3000")
	}
}

// TestBuildTTM_MissingSamePriorYear 缺去年同期 → 降级为上一年报
func TestBuildTTM_MissingSamePriorYear(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2026-03-31", "2025-12-31", "2025-09-30", "2024-12-31"}, // 缺 2025-03-31
		years:   []string{"2025-12-31", "2024-12-31"},
		revenue: map[string]float64{
			"2026-03-31": 412,
			"2025-12-31": 1938,
			"2024-12-31": 2086,
		},
		profit: map[string]float64{
			"2025-12-31": 200,
		},
	})
	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	if m.Mode != TTMModeAnnualFallback {
		t.Errorf("Mode 应为 annual-fallback，实际 %s", m.Mode)
	}
	if m.EndPeriod != "2025-12-31" {
		t.Errorf("EndPeriod 应为 2025-12-31（最近年报），实际 %s", m.EndPeriod)
	}
	if !approxEq(m.Revenue, 1938) {
		t.Errorf("Revenue 应为 1938（2025 年报），实际 %f", m.Revenue)
	}
	if !approxEq(m.NetProfit, 200) {
		t.Errorf("NetProfit 应为 200，实际 %f", m.NetProfit)
	}
	if m.Notes == "" {
		t.Error("Notes 应说明缺失原因，实际为空")
	}
	if !strings.Contains(m.Notes, "2025-03-31") {
		t.Errorf("Notes 应提到缺失的期间，实际：%s", m.Notes)
	}
}

// TestBuildTTM_MissingPriorAnnual 缺上一年报 → 降级到最近一份年报
func TestBuildTTM_MissingPriorAnnual(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2026-03-31", "2025-09-30", "2024-12-31"}, // 缺 2025-12-31
		years:   []string{"2024-12-31"},
		revenue: map[string]float64{
			"2026-03-31": 412,
			"2024-12-31": 2086,
		},
	})
	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	if m.Mode != TTMModeAnnualFallback {
		t.Errorf("Mode 应为 annual-fallback，实际 %s", m.Mode)
	}
	if m.EndPeriod != "2024-12-31" {
		t.Errorf("EndPeriod 应为 2024-12-31（最近年报），实际 %s", m.EndPeriod)
	}
	if !approxEq(m.Revenue, 2086) {
		t.Errorf("Revenue 应为 2086，实际 %f", m.Revenue)
	}
}

// TestBuildTTM_ParentEquityWeighted 归母字段齐全时：
// ROE 应使用「归母净利润 / 加权平均归母权益」，与年度 step 16 口径对齐。
//
// 场景：最新期 2026-03-31 季报，整体净利润 > 归母净利润（含少数股东），
// 整体期末权益 > 归母期末权益。期初取去年同期 2025-03-31 的归母权益做加权。
func TestBuildTTM_ParentEquityWeighted(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2026-03-31", "2025-12-31", "2025-03-31", "2024-12-31"},
		years:   []string{"2025-12-31", "2024-12-31"},
		revenue: map[string]float64{"2026-03-31": 412, "2025-12-31": 1938, "2025-03-31": 400},
		profit:  map[string]float64{"2026-03-31": 60, "2025-12-31": 240, "2025-03-31": 50},
		// 整体口径数据保留（用于 NetMargin / 降级路径）
		equity: map[string]float64{"2026-03-31": 1800},
	})
	// 归母字段：归母净利润 < 整体净利润（含少数股东），归母权益 < 整体权益
	fd.IncomeStatement["归母净利润"] = map[string]float64{
		"2026-03-31": 40,
		"2025-12-31": 200,
		"2025-03-31": 35,
	}
	fd.BalanceSheet["归母所有者权益合计"] = map[string]float64{
		"2026-03-31": 1500,
		"2025-03-31": 1200,
	}

	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	// TTM 归母净利润 = 200 + 40 − 35 = 205
	// 加权归母权益 = (1500 + 1200) / 2 = 1350
	// ROE = 205 / 1350 ≈ 0.15185
	wantROE := 205.0 / 1350.0
	if !approxEq(m.ROE, wantROE) {
		t.Errorf("ROE 应为 %.6f（归母+加权），实际 %.6f", wantROE, m.ROE)
	}
	// TTM 整体净利润 = 240 + 60 − 50 = 250；TTM 营收 = 1938 + 412 − 400 = 1950
	// NetMargin = 250 / 1950（整体口径保持不变）
	wantMargin := 250.0 / 1950.0
	if !approxEq(m.NetMargin, wantMargin) {
		t.Errorf("NetMargin 应为 %.6f（整体口径），实际 %.6f", wantMargin, m.NetMargin)
	}
}

// TestBuildTTM_ParentEquityAnnual 年报模式下的归母+加权 ROE
func TestBuildTTM_ParentEquityAnnual(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2025-12-31", "2024-12-31"},
		years:   []string{"2025-12-31", "2024-12-31"},
		revenue: map[string]float64{"2025-12-31": 1000, "2024-12-31": 800},
		profit:  map[string]float64{"2025-12-31": 120, "2024-12-31": 90},
		equity:  map[string]float64{"2025-12-31": 600, "2024-12-31": 500},
	})
	fd.IncomeStatement["归母净利润"] = map[string]float64{
		"2025-12-31": 100,
		"2024-12-31": 80,
	}
	fd.BalanceSheet["归母所有者权益合计"] = map[string]float64{
		"2025-12-31": 500,
		"2024-12-31": 400,
	}

	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	if m.Mode != TTMModeAnnual {
		t.Errorf("Mode 应为 annual，实际 %s", m.Mode)
	}
	// 加权归母权益 = (400 + 500) / 2 = 450
	// ROE = 100 / 450 ≈ 0.2222
	wantROE := 100.0 / 450.0
	if !approxEq(m.ROE, wantROE) {
		t.Errorf("ROE 应为 %.6f（归母+加权），实际 %.6f", wantROE, m.ROE)
	}
}

// TestBuildTTM_ParentEquityMissingPrior 归母期末数据齐全但缺去年同期 → 不加权
func TestBuildTTM_ParentEquityMissingPrior(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2025-12-31"},
		years:   []string{"2025-12-31"},
		revenue: map[string]float64{"2025-12-31": 1000},
		profit:  map[string]float64{"2025-12-31": 120},
		equity:  map[string]float64{"2025-12-31": 600},
	})
	fd.IncomeStatement["归母净利润"] = map[string]float64{"2025-12-31": 100}
	fd.BalanceSheet["归母所有者权益合计"] = map[string]float64{"2025-12-31": 500}

	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	// 缺期初 → 仅用期末归母权益。ROE = 100 / 500 = 0.2
	if !approxEq(m.ROE, 0.2) {
		t.Errorf("ROE 应为 0.2（仅期末归母权益），实际 %.6f", m.ROE)
	}
}

// TestBuildTTM_NoData 完全无数据
func TestBuildTTM_NoData(t *testing.T) {
	m := BuildTTMMetrics(nil)
	if m == nil || m.HasData {
		t.Errorf("nil data 应返回 HasData=false")
	}
	m = BuildTTMMetrics(&FinancialData{})
	if m == nil || m.HasData {
		t.Errorf("空 FinancialData 应返回 HasData=false")
	}
}

// TestBuildTTM_NoAnnualAtAll 季报为最新期但完全没有年报
func TestBuildTTM_NoAnnualAtAll(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2026-03-31", "2025-09-30"},
		years:   []string{},
		revenue: map[string]float64{
			"2026-03-31": 412,
		},
	})
	m := BuildTTMMetrics(fd)
	if m.HasData {
		t.Errorf("无任何年报时应返回 HasData=false")
	}
	if m.Notes == "" {
		t.Error("应给出说明")
	}
}

// TestBuildTTM_Year4Digit 兼容 4 位年份（"2025"）格式
func TestBuildTTM_Year4Digit(t *testing.T) {
	fd := buildFD(ttmFixture{
		periods: []string{"2025", "2024"},
		years:   []string{"2025", "2024"},
		revenue: map[string]float64{"2025": 1000, "2024": 900},
	})
	m := BuildTTMMetrics(fd)
	if !m.HasData {
		t.Fatal("应有数据")
	}
	if m.Mode != TTMModeAnnual {
		t.Errorf("Mode 应为 annual，实际 %s", m.Mode)
	}
	if !approxEq(m.Revenue, 1000) {
		t.Errorf("Revenue 应为 1000，实际 %f", m.Revenue)
	}
}

// TestIsAnnualPeriod 各种期间格式判定
func TestIsAnnualPeriod(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"2025-12-31", true},
		{"2025-09-30", false},
		{"2025-03-31", false},
		{"2025-06-30", false},
		{"2025", true},
		{"abcd", false},
		{"", false},
		{"2025-1-1", false},
	}
	for _, c := range cases {
		if got := isAnnualPeriod(c.in); got != c.want {
			t.Errorf("isAnnualPeriod(%q) = %v，期望 %v", c.in, got, c.want)
		}
	}
}

// TestTTMWindowHint TTM 覆盖区间提示
func TestTTMWindowHint(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2025-12-31", "2025-01 ~ 2025-12"},
		{"2026-03-31", "2025-04 ~ 2026-03"},
		{"2025-06-30", "2024-07 ~ 2025-06"},
		{"2025-09-30", "2024-10 ~ 2025-09"},
		{"2025", "2025-01 ~ 2025-12"},
	}
	for _, c := range cases {
		if got := ttmWindowHint(c.in); got != c.want {
			t.Errorf("ttmWindowHint(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// TestFormatTTMReport 渲染包含关键字段（用实际「元」单位值，验证 1e8 缩放）
func TestFormatTTMReport(t *testing.T) {
	// 类 002598 真实量级（元）：1938M 全年、412M Q1 2026、400M Q1 2025
	// TTM Revenue = 1938 + 412 − 400 = 1950 (单位: 百万元) = 19.50 亿元
	const m = 1e6
	fd := buildFD(ttmFixture{
		periods: []string{"2026-03-31", "2025-12-31", "2025-03-31"},
		years:   []string{"2025-12-31"},
		revenue: map[string]float64{"2026-03-31": 412 * m, "2025-12-31": 1938 * m, "2025-03-31": 400 * m},
		profit:  map[string]float64{"2026-03-31": 40 * m, "2025-12-31": 200 * m, "2025-03-31": 35 * m},
	})
	res := BuildTTMMetrics(fd)
	out := res.FormatTTMReport()
	expects := []string{
		"TTM 截止期",
		"2026-03-31",
		"2025-04 ~ 2026-03",
		"上一年报",
		"19.50 亿元", // (1938+412-400) * 1e6 / 1e8 = 19.50
	}
	for _, want := range expects {
		if !strings.Contains(out, want) {
			t.Errorf("输出应含 %q，实际：\n%s", want, out)
		}
	}
}
