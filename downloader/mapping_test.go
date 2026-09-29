package downloader

import (
	"encoding/json"
	"testing"
)

// TestMergeBalanceSheetEastMoneyFields 验证东财 HSF10 新版字段映射正确性。
// 实测东财 API 对 600941.SH 的 2024 年报：
//   - PARENT_EQUITY_BALANCE = 0
//   - TOTAL_PARENT_EQUITY = 1356732000000
//   - RETAINED_EARNINGS 缺失或为 0
//   - UNASSIGN_RPOFIT = 1188577000000
func TestMergeBalanceSheetEastMoneyFields(t *testing.T) {
	raw := `{
		"TOTAL_ASSETS": 2072827000000,
		"TOTAL_LIABILITIES": 711588000000,
		"TOTAL_EQUITY": 1361239000000,
		"PARENT_EQUITY_BALANCE": 0,
		"TOTAL_PARENT_EQUITY": 1356732000000,
		"MINORITY_EQUITY": 4507000000,
		"RETAINED_EARNINGS": 0,
		"UNASSIGN_RPOFIT": 1188577000000,
		"SHARE_CAPITAL": 461838000000,
		"TAX_PAYABLE": 21549000000,
		"MONETARYFUNDS": 242275000000
	}`

	var src map[string]any
	if err := json.Unmarshal([]byte(raw), &src); err != nil {
		t.Fatalf("解析测试数据失败: %v", err)
	}

	target := make(map[string]map[string]float64)
	mergeBalanceSheet(target, src, "2024-12-31")

	cases := []struct {
		name     string
		expected float64
	}{
		{"资产合计", 2072827000000},
		{"负债合计", 711588000000},
		{"所有者权益合计", 1361239000000},
		{"归属于母公司所有者权益合计", 1356732000000},
		{"少数股东权益", 4507000000},
		{"未分配利润", 1188577000000},
		{"实收资本（或股本）", 461838000000},
		{"应交税费", 21549000000},
		{"货币资金", 242275000000},
	}

	for _, c := range cases {
		row, ok := target[c.name]
		if !ok {
			t.Errorf("缺少科目: %s", c.name)
			continue
		}
		if got := row["2024-12-31"]; got != c.expected {
			t.Errorf("%s = %.0f, want %.0f", c.name, got, c.expected)
		}
	}
}

// TestMergeBalanceSheetParentEquityFallback 验证当 TOTAL_PARENT_EQUITY 缺失时，
// 能用 TOTAL_EQUITY - MINORITY_EQUITY 推导归母权益。
func TestMergeBalanceSheetParentEquityFallback(t *testing.T) {
	raw := `{
		"TOTAL_ASSETS": 1000,
		"TOTAL_LIABILITIES": 400,
		"TOTAL_EQUITY": 600,
		"PARENT_EQUITY_BALANCE": 0,
		"MINORITY_EQUITY": 50
	}`

	var src map[string]any
	if err := json.Unmarshal([]byte(raw), &src); err != nil {
		t.Fatalf("解析测试数据失败: %v", err)
	}

	target := make(map[string]map[string]float64)
	mergeBalanceSheet(target, src, "2024-12-31")

	if got := target["归属于母公司所有者权益合计"]["2024-12-31"]; got != 550 {
		t.Errorf("归母权益推导结果 = %.0f, want 550", got)
	}
}
