package analyzer

import (
	"fmt"
	"strings"
)

// TTMMetrics 滚动 TTM 指标
type TTMMetrics struct {
	HasData       bool     `json:"hasData"`
	Revenue       float64  `json:"revenue"`
	NetProfit     float64  `json:"netProfit"`
	OperatingCash float64  `json:"operatingCash"`
	ROE           float64  `json:"roe"`
	NetMargin     float64  `json:"netMargin"`
	CashRatio     float64  `json:"cashRatio"`   // 经营现金流/净利润
	PeriodCount   int      `json:"periodCount"` // 实际累加的季度数
	Periods       []string `json:"periods"`     // 实际累加的报告期（用于校对，时间升序）
}

// BuildTTMMetrics 构建最近 4 个季度的 TTM 滚动指标
func BuildTTMMetrics(data *FinancialData) *TTMMetrics {
	metrics := &TTMMetrics{}
	if data == nil || len(data.Quarters) == 0 {
		return metrics
	}

	// 按年份分组，每组取4个季度累加
	ttm := computeTTMByYear(data)
	if ttm != nil {
		return ttm
	}

	// 如果没有完整年份分组，直接取最近4个季度
	quarters := filterQuarters(data.Quarters)
	if len(quarters) == 0 {
		// 没有季报，尝试用年报
		if len(data.Years) > 0 {
			return buildFromAnnual(data, data.Years[0])
		}
		return metrics
	}

	// 取最近 4 个季度（quarters 是降序，反转成升序传入累加便于展示）
	count := 4
	if len(quarters) < count {
		count = len(quarters)
	}
	recent := make([]string, count)
	for i := 0; i < count; i++ {
		recent[i] = quarters[count-1-i]
	}
	return accumulateQuarters(data, recent)
}

// computeTTMByYear 按最新完整年度计算 TTM
func computeTTMByYear(data *FinancialData) *TTMMetrics {
	// 尝试找到最新的完整年度（有4个季度）
	for _, year := range data.Years {
		prefix := year
		if strings.HasSuffix(year, "-12-31") {
			prefix = year[:4]
		}
		q1 := prefix + "-03-31"
		q2 := prefix + "-06-30"
		q3 := prefix + "-09-30"
		q4 := prefix + "-12-31"

		hasQ1 := hasPeriod(data.Quarters, q1)
		hasQ2 := hasPeriod(data.Quarters, q2)
		hasQ3 := hasPeriod(data.Quarters, q3)
		hasQ4 := hasPeriod(data.Quarters, q4) || hasPeriod(data.Quarters, year)

		if hasQ1 && hasQ2 && hasQ3 && hasQ4 {
			periods := []string{q1, q2, q3, q4}
			// 如果年报存在，用年报替代 Q4
			if hasPeriod(data.Quarters, year) && !hasPeriod(data.Quarters, q4) {
				periods[3] = year
			}
			return accumulateQuarters(data, periods)
		}
	}
	return nil
}

// buildFromAnnual 从单一年报构建 TTM（退化为年报数据）
func buildFromAnnual(data *FinancialData, year string) *TTMMetrics {
	revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
	netProfit := data.GetValueOrZero(data.IncomeStatement, "净利润", year)
	operatingCash := data.GetValueOrZero(data.CashFlow, "经营活动产生的现金流量净额", year)
	equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", year)
	if equity == 0 {
		totalAssets := data.GetValueOrZero(data.BalanceSheet, "总资产", year)
		totalLiabilities := data.GetValueOrZero(data.BalanceSheet, "总负债", year)
		equity = totalAssets - totalLiabilities
	}

	metrics := &TTMMetrics{HasData: true, PeriodCount: 1, Periods: []string{year}}
	metrics.Revenue = revenue
	metrics.NetProfit = netProfit
	metrics.OperatingCash = operatingCash
	if equity > 0 {
		metrics.ROE = netProfit / equity
	}
	if revenue > 0 {
		metrics.NetMargin = netProfit / revenue
	}
	if netProfit > 0 {
		metrics.CashRatio = operatingCash / netProfit
	}
	return metrics
}

// accumulateQuarters 累加多个季度的数据
func accumulateQuarters(data *FinancialData, periods []string) *TTMMetrics {
	metrics := &TTMMetrics{HasData: true, PeriodCount: len(periods), Periods: append([]string(nil), periods...)}
	latestPeriod := periods[len(periods)-1]

	for _, period := range periods {
		metrics.Revenue += data.GetValueOrZero(data.IncomeStatement, "营业收入", period)
		metrics.NetProfit += data.GetValueOrZero(data.IncomeStatement, "净利润", period)
		metrics.OperatingCash += data.GetValueOrZero(data.CashFlow, "经营活动产生的现金流量净额", period)
	}

	// 净资产取最新一期的
	equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", latestPeriod)
	if equity == 0 {
		totalAssets := data.GetValueOrZero(data.BalanceSheet, "总资产", latestPeriod)
		totalLiabilities := data.GetValueOrZero(data.BalanceSheet, "总负债", latestPeriod)
		equity = totalAssets - totalLiabilities
	}
	if equity > 0 {
		metrics.ROE = metrics.NetProfit / equity
	}
	if metrics.Revenue > 0 {
		metrics.NetMargin = metrics.NetProfit / metrics.Revenue
	}
	if metrics.NetProfit > 0 {
		metrics.CashRatio = metrics.OperatingCash / metrics.NetProfit
	}
	return metrics
}

// hasPeriod 检查期间列表中是否包含指定期间
func hasPeriod(periods []string, target string) bool {
	for _, p := range periods {
		if p == target {
			return true
		}
	}
	return false
}

// FormatTTMReport 格式化 TTM 指标为三个子表格
func (m *TTMMetrics) FormatTTMReport() string {
	if !m.HasData {
		return "> **TTM 数据不足**: 需要至少 1 个季度/年度财务数据\n\n"
	}
	var b strings.Builder

	// 累加期间清单（便于核对：模块3.4 数据时效以最新一期为准）
	if len(m.Periods) > 0 {
		b.WriteString("> **累加期间**: ")
		b.WriteString(strings.Join(m.Periods, " + "))
		if m.PeriodCount == 1 {
			b.WriteString("（季报数据不足，已退化为最新年报口径）")
		} else if m.PeriodCount < 4 {
			b.WriteString(fmt.Sprintf("（仅 %d 个期间，TTM 口径不完整）", m.PeriodCount))
		}
		b.WriteString("\n\n")
	}

	// 表1：经营规模（累计值）
	b.WriteString("### 3.4.1 经营规模（累计值）\n\n")
	b.WriteString("| 指标 | 数值 | 计算说明 |\n")
	b.WriteString("|------|------|----------|\n")
	b.WriteString(fmt.Sprintf("| 营业收入 | %.2f 亿元 | 最近%d个季度累加 |\n", m.Revenue/1e8, m.PeriodCount))
	b.WriteString(fmt.Sprintf("| 净利润 | %.2f 亿元 | 最近%d个季度累加 |\n", m.NetProfit/1e8, m.PeriodCount))
	b.WriteString(fmt.Sprintf("| 经营现金流 | %.2f 亿元 | 最近%d个季度累加 |\n", m.OperatingCash/1e8, m.PeriodCount))
	b.WriteString("\n")

	// 表2：盈利能力（比率）
	b.WriteString("### 3.4.2 盈利能力（比率）\n\n")
	b.WriteString("| 指标 | 数值 | 参考标准 |\n")
	b.WriteString("|------|------|----------|\n")
	b.WriteString(fmt.Sprintf("| 净资产收益率（ROE） | %.2f%% | >15%% 优秀，>10%% 良好 |\n", m.ROE*100))
	b.WriteString(fmt.Sprintf("| 净利率 | %.2f%% | 越高说明盈利空间越大 |\n", m.NetMargin*100))
	b.WriteString("\n")

	// 表3：现金流质量
	b.WriteString("### 3.4.3 现金流质量\n\n")
	b.WriteString("| 指标 | 数值 | 健康标准 |\n")
	b.WriteString("|------|------|----------|\n")
	b.WriteString(fmt.Sprintf("| 经营现金流/净利润 | %.2f%% | >100%% 说明利润有现金支撑，<100%% 需警惕应收账款虚增 |\n", m.CashRatio*100))
	b.WriteString("\n")

	return b.String()
}
