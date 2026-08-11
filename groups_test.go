package main

import (
	"testing"

	"github.com/liusaipu/stockfinlens/analyzer"
)

func TestFillCompositeScores(t *testing.T) {
	rows := []GroupComparisonRow{
		{Code: "A", Name: "A公司", ROE: 20, GrossMargin: 40, RevenueGrowth: 30, DebtRatio: 30, HasDebtRatio: true, CashRatio: 120, HasCashRatio: true, AScore: 20, ActivityScore: 80, HasActivity: true},
		{Code: "B", Name: "B公司", ROE: 5, GrossMargin: 10, RevenueGrowth: -10, DebtRatio: 70, HasDebtRatio: true, CashRatio: 50, HasCashRatio: true, AScore: 70, ActivityScore: 40, HasActivity: true},
		{Code: "C", Name: "C公司", ROE: 10, GrossMargin: 25, RevenueGrowth: 10, DebtRatio: 0, HasDebtRatio: false, CashRatio: 0, HasCashRatio: false, AScore: 50, ActivityScore: 0, HasActivity: false},
	}

	fillCompositeScores(rows)

	// C 的缺失指标应被中位数替代
	if rows[2].DebtRatio != 50 {
		t.Errorf("C 负债率应被中位数 50 替代，实际 %.2f", rows[2].DebtRatio)
	}
	if rows[2].CashRatio != 85 { // (50+120)/2
		t.Errorf("C 现金含量应被中位数 85 替代，实际 %.2f", rows[2].CashRatio)
	}
	if rows[2].ActivityScore != 60 { // (40+80)/2
		t.Errorf("C 活跃度应被中位数 60 替代，实际 %.2f", rows[2].ActivityScore)
	}

	// 排名：A 应第 1，C 第 2（替代后得分中等），B 第 3
	if rows[0].CompositeRank != 1 {
		t.Errorf("A 应排名第 1，实际 %d", rows[0].CompositeRank)
	}
	if rows[2].CompositeRank != 2 {
		t.Errorf("C 应排名第 2，实际 %d", rows[2].CompositeRank)
	}
	if rows[1].CompositeRank != 3 {
		t.Errorf("B 应排名第 3，实际 %d", rows[1].CompositeRank)
	}

	// 综合得分应已计算且大于 0
	for _, r := range rows {
		if r.CompositeScore <= 0 {
			t.Errorf("%s 综合得分应大于 0，实际 %.2f", r.Code, r.CompositeScore)
		}
	}
}

func TestFillCompositeScoresEmpty(t *testing.T) {
	fillCompositeScores(nil)
	fillCompositeScores([]GroupComparisonRow{})
	// 不应 panic
}

func TestMedianFloat64(t *testing.T) {
	if got := medianFloat64([]float64{1, 3, 2}); got != 2 {
		t.Errorf("中位数应为 2，实际 %.2f", got)
	}
	if got := medianFloat64([]float64{1, 4, 3, 2}); got != 2.5 {
		t.Errorf("中位数应为 2.5，实际 %.2f", got)
	}
	if got := medianFloat64(nil); got != 0 {
		t.Errorf("空切片应返回 0，实际 %.2f", got)
	}
}

func TestCalcComparableScoreExported(t *testing.T) {
	m := &analyzer.ComparableMetrics{
		ROE:           20,
		GrossMargin:   40,
		RevenueGrowth: 30,
		DebtRatio:     30,
		CashRatio:     120,
		AScore:        20,
		ActivityScore: 80,
	}
	score := analyzer.CalcComparableScore(m, []*analyzer.ComparableMetrics{m}, 80)
	if score <= 0 || score > 100 {
		t.Errorf("综合得分应在 0-100 之间，实际 %.2f", score)
	}
}
