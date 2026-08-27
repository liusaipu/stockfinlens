package analyzer

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateMarkdownMock(t *testing.T) {
	years := []string{"2025", "2024", "2023"}

	steps := []StepResult{
		{StepNum: 3, StepName: "资产负债率", Pass: map[string]bool{"2025": true, "2024": true, "2023": false}, YearlyData: map[string]map[string]any{
			"2025": {"debtRatio": 35.4, "cashDebtDiff": 8.2e8},
			"2024": {"debtRatio": 37.9, "cashDebtDiff": 6.1e8},
			"2023": {"debtRatio": 43.1, "cashDebtDiff": -1.2e8},
		}},
		{StepNum: 5, StepName: "应收账款", Pass: map[string]bool{"2025": false, "2024": false, "2023": false}, YearlyData: map[string]map[string]any{
			"2025": {"ratio": 13.7},
			"2024": {"ratio": 14.2},
			"2023": {"ratio": 15.1},
		}},
		{StepNum: 6, StepName: "固定资产工程", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"ratio": 18.5},
			"2024": {"ratio": 19.1},
			"2023": {"ratio": 21.3},
		}},
		{StepNum: 7, StepName: "投资类资产", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"ratio": 4.2},
			"2024": {"ratio": 3.8},
			"2023": {"ratio": 5.1},
		}},
		{StepNum: 8, StepName: "Beneish M-Score", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"MScore": -2.45},
			"2024": {"MScore": -2.38},
			"2023": {"MScore": -2.12},
		}},
		{StepNum: 9, StepName: "营业收入", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"revenue": 288.5e8, "growthRate": 23.4},
			"2024": {"revenue": 233.8e8, "growthRate": 18.2},
			"2023": {"revenue": 197.8e8, "growthRate": 12.5},
		}},
		{StepNum: 10, StepName: "毛利率", Pass: map[string]bool{"2025": false, "2024": false, "2023": false}, YearlyData: map[string]map[string]any{
			"2025": {"grossMargin": 30.6},
			"2024": {"grossMargin": 29.1},
			"2023": {"grossMargin": 27.8},
		}},
		{StepNum: 11, StepName: "存货周转率", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"inventoryTurnover": 3.2},
			"2024": {"inventoryTurnover": 2.9},
			"2023": {"inventoryTurnover": 2.7},
		}},
		{StepNum: 12, StepName: "期间费用率/毛利率", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"expenseToMargin": 25.4},
			"2024": {"expenseToMargin": 27.1},
			"2023": {"expenseToMargin": 28.3},
		}},
		{StepNum: 13, StepName: "研发费用率", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"rdRatio": 9.8},
			"2024": {"rdRatio": 8.5},
			"2023": {"rdRatio": 7.9},
		}},
		{StepNum: 14, StepName: "主营利润率", Pass: map[string]bool{"2025": false, "2024": false, "2023": false}, YearlyData: map[string]map[string]any{
			"2025": {"coreProfitMargin": 12.4},
			"2024": {"coreProfitMargin": 11.8},
			"2023": {"coreProfitMargin": 10.5},
		}},
		{StepNum: 15, StepName: "净利润现金含量", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"cashRatio": 101.8, "operatingCF": 41.2e8},
			"2024": {"cashRatio": 95.3, "operatingCF": 35.1e8},
			"2023": {"cashRatio": 88.7, "operatingCF": 28.4e8},
		}},
		{StepNum: 16, StepName: "ROE", Pass: map[string]bool{"2025": false, "2024": false, "2023": false}, YearlyData: map[string]map[string]any{
			"2025": {"roe": 14.4, "profit": 40.45e8, "profitGrowth": 31.5},
			"2024": {"roe": 12.1, "profit": 30.76e8, "profitGrowth": 28.3},
			"2023": {"roe": 9.8, "profit": 23.97e8, "profitGrowth": 15.2},
		}},
		{StepNum: 17, StepName: "购建长期资产", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"ratio": 56.7},
			"2024": {"ratio": 52.3},
			"2023": {"ratio": 48.1},
		}},
		{StepNum: 18, StepName: "分红现金支出", Pass: map[string]bool{"2025": true, "2024": true, "2023": true}, YearlyData: map[string]map[string]any{
			"2025": {"ratio": 23.8},
			"2024": {"ratio": 22.1},
			"2023": {"ratio": 25.4},
		}},
	}

	scores := map[string]*YearScore{
		"2025": {RawScore: 82.5, Grade: "B", PassCount: 12, FailCount: 3, Deductions: []Deduction{
			{StepNum: 5, StepName: "应收账款", Points: 5, Reason: "占比13.7%，超过10%标准"},
			{StepNum: 10, StepName: "毛利率", Points: 5, Reason: "毛利率30.6%，低于40%标准"},
		}},
		"2024": {RawScore: 80.0, Grade: "B", PassCount: 11, FailCount: 4},
		"2023": {RawScore: 75.0, Grade: "B-", PassCount: 10, FailCount: 5},
	}

	md := GenerateMarkdown("豪威集团", "603501", years, steps, scores, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if !strings.Contains(md, "豪威集团") {
		t.Error("missing company name")
	}
	if !strings.Contains(md, "603501") {
		t.Error("missing symbol")
	}
	if !strings.Contains(md, "82") && !strings.Contains(md, "87") {
		t.Error("missing score")
	}
	if !strings.Contains(md, "资产负债率") {
		t.Error("missing debt ratio")
	}
	fmt.Println(md[:2000])
	fmt.Println("\n... (truncated) ...")
}

// TestInfoTooltipHTML 验证 infoTooltipHTML 已废弃返回空字符串
func TestInfoTooltipHTML(t *testing.T) {
	html := infoTooltipHTML("测试标题", "测试内容")
	if html != "" {
		t.Errorf("infoTooltipHTML 已废弃，应返回空字符串，实际返回: %s", html)
	}
}
