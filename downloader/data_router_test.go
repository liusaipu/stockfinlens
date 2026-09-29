package downloader

import (
	"testing"
)

// TestConvertToFinancialReportDataEquityFallback 验证 SFL 数据源中 total_hldr_eqy 为 0 时，
// 能通过资产 - 负债推导所有者权益，并正确计算归母权益。
func TestConvertToFinancialReportDataEquityFallback(t *testing.T) {
	router := &DataRouter{}
	tfd := &SFLFinancialData{
		BalanceSheet: []SFLBalanceItem{
			{
				TsCode:       "600941.SH",
				EndDate:      "20241231",
				TotalAssets:  2072827000000,
				TotalLiab:    711588000000,
				TotalHldrEqy: 0, // tushare 对该公司该字段返回 0
				MinorityInt:  4507000000,
				MoneyCap:     242275000000,
			},
		},
	}

	result := router.ConvertToFinancialReportData(tfd, "600941.SH")

	year := "2024-12-31"
	cases := []struct {
		name     string
		expected float64
	}{
		{"资产合计", 2072827000000},
		{"负债合计", 711588000000},
		{"所有者权益合计", 2072827000000 - 711588000000},
		{"归属于母公司所有者权益合计", 2072827000000 - 711588000000 - 4507000000},
		{"少数股东权益", 4507000000},
	}

	for _, c := range cases {
		row, ok := result.BalanceSheet[c.name]
		if !ok {
			t.Errorf("缺少科目: %s", c.name)
			continue
		}
		if got := row[year]; got != c.expected {
			t.Errorf("%s = %.0f, want %.0f", c.name, got, c.expected)
		}
	}
}
