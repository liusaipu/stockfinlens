package analyzer

import (
	"math"
	"testing"
)

// 构造一个最小化的 FinancialData，只填季度预警用到的表和期间
func newQuarterlyTestData(periods []string, income, cashflow map[string]map[string]float64) *FinancialData {
	return &FinancialData{
		Symbol:          "TEST",
		Quarters:        periods,
		IncomeStatement: income,
		CashFlow:        cashflow,
	}
}

// TestSingleQuarterValue 验证累计 YTD 还原为单季值
func TestSingleQuarterValue(t *testing.T) {
	income := map[string]map[string]float64{
		"营业收入": {
			"2025-03-31": 10,
			"2025-06-30": 25, // Q2 单季 15
			"2025-09-30": 45, // Q3 单季 20
			"2025-12-31": 70, // Q4 单季 25
			"2026-03-31": 18, // Q1 单季 18
		},
	}
	data := newQuarterlyTestData([]string{"2026-03-31", "2025-12-31", "2025-09-30", "2025-06-30", "2025-03-31"}, income, nil)

	cases := []struct {
		period string
		want   float64
	}{
		{"2026-03-31", 18},
		{"2025-12-31", 25}, // 70 - 45
		{"2025-09-30", 20}, // 45 - 25
		{"2025-06-30", 15}, // 25 - 10
		{"2025-03-31", 10},
	}
	for _, c := range cases {
		got := singleQuarterValue(data, data.IncomeStatement, "营业收入", c.period)
		if math.Abs(got-c.want) > 1e-6 {
			t.Errorf("singleQuarterValue(营业收入, %s) = %.2f, want %.2f", c.period, got, c.want)
		}
	}
}

// TestBuildQuarterlyAlertQoQ 验证环比使用单季值，且年报参与相邻季度比较
func TestBuildQuarterlyAlertQoQ(t *testing.T) {
	// 模拟道通科技场景，但把 2026Q1 营收设低一些以触发预警阈值
	// 2025Q4 单季营收 = 48.33 - 34.96 = 13.37亿；2026Q1 单季营收 11.5亿 => 环比约 -14%
	income := map[string]map[string]float64{
		"营业收入": {
			"2025-03-31": 10.94e8,
			"2025-06-30": 23.45e8,
			"2025-09-30": 34.96e8,
			"2025-12-31": 48.33e8,
			"2026-03-31": 11.50e8,
		},
		"营业成本": {
			"2025-03-31": 6.0e8,
			"2025-06-30": 13.0e8,
			"2025-09-30": 19.5e8,
			"2025-12-31": 27.0e8,
			"2026-03-31": 7.0e8,
		},
		"净利润": {
			"2025-03-31": 1.0e8,
			"2025-06-30": 2.5e8,
			"2025-09-30": 4.0e8,
			"2025-12-31": 5.5e8,
			"2026-03-31": 1.2e8,
		},
	}
	cashflow := map[string]map[string]float64{
		"经营活动产生的现金流量净额": {
			"2025-03-31": 0.5e8,
			"2025-06-30": 1.2e8,
			"2025-09-30": 2.0e8,
			"2025-12-31": 3.0e8,
			"2026-03-31": 0.6e8,
		},
	}
	periods := []string{"2026-03-31", "2025-12-31", "2025-09-30", "2025-06-30", "2025-03-31"}
	data := newQuarterlyTestData(periods, income, cashflow)

	alert := BuildQuarterlyAlert(data)
	if !alert.HasData {
		t.Fatal("expected HasData=true")
	}

	// 找到 2026-03-31 营收环比项
	var revQoQ *QuarterlyAlertItem
	for i := range alert.Items {
		item := &alert.Items[i]
		if item.Period == "2026-03-31" && item.Metric == "营业收入" && item.CompareType == "环比" {
			revQoQ = item
			break
		}
	}
	if revQoQ == nil {
		t.Fatal("missing 2026-03-31 营业收入 环比 item")
	}

	// 当前值应为 2026Q1 单季营收 11.50e8，基准应为 2025Q4 单季营收 (48.33-34.96)e8 = 13.37e8
	if math.Abs(revQoQ.Current-11.50e8) > 1e4 {
		t.Errorf("Current = %.2f, want 11.50e8", revQoQ.Current)
	}
	wantPrev := (48.33 - 34.96) * 1e8
	if math.Abs(revQoQ.Previous-wantPrev) > 1e4 {
		t.Errorf("Previous = %.2f, want %.2f", revQoQ.Previous, wantPrev)
	}
	if revQoQ.PreviousPeriod != "2025-12-31" {
		t.Errorf("PreviousPeriod = %s, want 2025-12-31", revQoQ.PreviousPeriod)
	}

	// 环比变化应约为 -14%，不再是从前累计口径的 -62.8%
	wantChange := (11.50e8 - wantPrev) / wantPrev
	if math.Abs(revQoQ.ChangePct-wantChange) > 1e-4 {
		t.Errorf("ChangePct = %.4f, want %.4f", revQoQ.ChangePct, wantChange)
	}
}

// TestBuildQuarterlyAlertYoY 验证同比也使用单季值
func TestBuildQuarterlyAlertYoY(t *testing.T) {
	income := map[string]map[string]float64{
		"营业收入": {
			"2025-03-31": 20.0e8,
			"2026-03-31": 12.0e8,
		},
		"营业成本": {
			"2025-03-31": 12.0e8,
			"2026-03-31": 7.0e8,
		},
		"净利润": {
			"2025-03-31": 2.0e8,
			"2026-03-31": 1.2e8,
		},
	}
	periods := []string{"2026-03-31", "2025-03-31"}
	data := newQuarterlyTestData(periods, income, nil)

	alert := BuildQuarterlyAlert(data)
	var revYoY *QuarterlyAlertItem
	for i := range alert.Items {
		item := &alert.Items[i]
		if item.Period == "2026-03-31" && item.Metric == "营业收入" && item.CompareType == "同比" {
			revYoY = item
			break
		}
	}
	if revYoY == nil {
		t.Fatal("missing 2026-03-31 营业收入 同比 item")
	}
	if math.Abs(revYoY.Current-12.0e8) > 1e-4 || math.Abs(revYoY.Previous-20.0e8) > 1e-4 {
		t.Errorf("同比 Current=%.2f Previous=%.2f, want 12e8/20e8", revYoY.Current, revYoY.Previous)
	}
	if revYoY.PreviousPeriod != "2025-03-31" {
		t.Errorf("PreviousPeriod = %s, want 2025-03-31", revYoY.PreviousPeriod)
	}
}

// TestPreviousQuarterPeriod 验证相邻季度推算
func TestPreviousQuarterPeriod(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2026-03-31", "2025-12-31"},
		{"2026-06-30", "2026-03-31"},
		{"2026-09-30", "2026-06-30"},
		{"2026-12-31", "2026-09-30"},
	}
	for _, c := range cases {
		got := previousQuarterPeriod(c.in)
		if got != c.want {
			t.Errorf("previousQuarterPeriod(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}
