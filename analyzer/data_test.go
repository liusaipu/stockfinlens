package analyzer

import (
	"testing"
)

// TestFixMissingData 验证运行时数据修复逻辑
func TestFixMissingData(t *testing.T) {
	fd := &FinancialData{
		Symbol: "002594.SZ",
		Years:  []string{"2025-12-31"},
		BalanceSheet: map[string]map[string]float64{
			"资产合计":          {"2025-12-31": 883729883000},
			"负债合计":          {"2025-12-31": 625190698000},
			"所有者权益合计":       {"2025-12-31": 0},
			"归属于母公司所有者权益合计": {"2025-12-31": -12264579000},
			"少数股东权益":        {"2025-12-31": 12264579000},
		},
		IncomeStatement: map[string]map[string]float64{
			"净利润":    {"2025-12-31": 33760758000},
			"少数股东损益": {"2025-12-31": 1141736000},
		},
		CashFlow: map[string]map[string]float64{},
	}

	fd.fixMissingData()

	// 验证总权益被修复：资产 - 负债
	totalEquity := fd.GetValueOrZero(fd.BalanceSheet, "所有者权益合计", "2025-12-31")
	expectedTotal := 883729883000.0 - 625190698000.0
	if totalEquity != expectedTotal {
		t.Errorf("总权益修复错误: got %.0f, want %.0f", totalEquity, expectedTotal)
	}

	// 验证归母权益被修复：总权益 - 少数股东权益
	parentEquity := fd.GetValueOrZero(fd.BalanceSheet, "归母所有者权益合计", "2025-12-31")
	expectedParent := expectedTotal - 12264579000
	if parentEquity != expectedParent {
		t.Errorf("归母权益修复错误: got %.0f, want %.0f", parentEquity, expectedParent)
	}

	// 验证归母净利润被修复：净利润 - 少数股东损益
	parentProfit := fd.GetValueOrZero(fd.IncomeStatement, "归母净利润", "2025-12-31")
	expectedProfit := 33760758000.0 - 1141736000.0
	if parentProfit != expectedProfit {
		t.Errorf("归母净利润修复错误: got %.0f, want %.0f", parentProfit, expectedProfit)
	}
}
