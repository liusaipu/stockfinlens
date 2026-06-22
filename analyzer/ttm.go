package analyzer

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// TTM 计算口径
const (
	TTMModeAnnual         = "annual"          // 最新报告期为年报，直接使用年报全年口径
	TTMModeQuarterly      = "quarterly"       // 标准 TTM 公式：上一年报 + 最新季报 − 去年同期
	TTMModeAnnualFallback = "annual-fallback" // 数据不全，降级为最近年报口径
)

// TTMMetrics 滚动 TTM 指标
//
// 重要：A 股财报值为「累计 YTD」口径——2025-09-30 行的"营业收入"已经是 1-9 月累计；
// 因此不能直接把多个季报累加（旧版 accumulateQuarters 这么做会重复计算）。
// 正确口径见 BuildTTMMetrics。
type TTMMetrics struct {
	HasData       bool    `json:"hasData"`
	Revenue       float64 `json:"revenue"`
	NetProfit     float64 `json:"netProfit"`
	OperatingCash float64 `json:"operatingCash"`
	ROE           float64 `json:"roe"`
	NetMargin     float64 `json:"netMargin"`
	CashRatio     float64 `json:"cashRatio"` // 经营现金流 / 净利润

	Mode      string `json:"mode"`            // 见 TTMMode* 常量
	EndPeriod string `json:"endPeriod"`       // TTM 截止报告期，例如 "2026-03-31"
	Notes     string `json:"notes,omitempty"` // 降级或异常时的说明

	PeriodCount int      `json:"periodCount"` // 参与公式的期数（年报=1，标准公式=3）
	Periods     []string `json:"periods"`     // 参与公式的原始期间，用于追溯

	// 上期 TTM 数据，用于 3.4.1 表格展示同比变化
	HasPrior           bool    `json:"hasPrior"`
	PriorEndPeriod     string  `json:"priorEndPeriod,omitempty"`
	PriorRevenue       float64 `json:"priorRevenue"`
	PriorNetProfit     float64 `json:"priorNetProfit"`
	PriorOperatingCash float64 `json:"priorOperatingCash"`
}

// BuildTTMMetrics 根据财务数据计算 TTM。
//
// 计算逻辑：
//  1. 取 data.Quarters[0]（已降序）作为"最新报告期"。
//  2. 若最新期为年报（YYYY-12-31 或 4 位年份）→ 直接采用年报全年口径，TTM 即年报值。
//  3. 若最新期为季报（YYYY-03/06/09-末日）→ 标准公式：
//     TTM = 上一年的年报值 + 最新季报值 − 去年同期季报值
//     （三者皆为累计口径，相减后恰为最近 12 个月累计）
//  4. 公式所需的"上一年年报"或"去年同期"缺失 → 降级为最近一份年报；都没有 → HasData=false。
func BuildTTMMetrics(data *FinancialData) *TTMMetrics {
	if data == nil || len(data.Quarters) == 0 {
		return &TTMMetrics{}
	}

	latest := data.Quarters[0] // 已降序
	m := buildTTMForEndPeriod(data, latest)

	// 同时计算上一期 TTM，用于 3.4.1 表格展示同比变化
	prior := previousQuarterPeriod(latest)
	if prior != "" && hasPeriod(data.Quarters, prior) {
		pm := buildPriorTTMForPeriod(data, prior)
		if pm.HasData {
			m.HasPrior = true
			m.PriorEndPeriod = prior
			m.PriorRevenue = pm.Revenue
			m.PriorNetProfit = pm.NetProfit
			m.PriorOperatingCash = pm.OperatingCash
		}
	}
	return m
}

// buildTTMForEndPeriod 为指定截止期计算当前 TTM（含降级策略）
func buildTTMForEndPeriod(data *FinancialData, endPeriod string) *TTMMetrics {
	if isAnnualPeriod(endPeriod) {
		return buildFromAnnual(data, endPeriod, TTMModeAnnual, "")
	}
	return buildFromQuarter(data, endPeriod)
}

// buildFromQuarter 以季报为最新期，套用标准 TTM 公式
func buildFromQuarter(data *FinancialData, latest string) *TTMMetrics {
	if len(latest) < 10 {
		return fallbackToRecentAnnual(data, fmt.Sprintf("最新期间 %q 格式异常", latest))
	}
	year, err := strconv.Atoi(latest[:4])
	if err != nil {
		return fallbackToRecentAnnual(data, fmt.Sprintf("最新期间 %q 年份解析失败", latest))
	}

	priorYearStr := strconv.Itoa(year - 1)
	priorAnnual := priorYearStr + "-12-31"
	samePriorYear := priorYearStr + latest[4:] // 同月同日

	hasPriorAnnual := hasPeriod(data.Quarters, priorAnnual)
	hasSamePrior := hasPeriod(data.Quarters, samePriorYear)

	if !hasPriorAnnual {
		return fallbackToRecentAnnual(data,
			fmt.Sprintf("缺少 %s 年报数据，无法用标准 TTM 公式", priorYearStr))
	}
	if !hasSamePrior {
		return buildFromAnnual(data, priorAnnual, TTMModeAnnualFallback,
			fmt.Sprintf("缺少去年同期 %s 数据，无法用标准 TTM 公式；已降级为 %s 年报全年口径",
				samePriorYear, priorYearStr))
	}

	m := &TTMMetrics{
		HasData:     true,
		Mode:        TTMModeQuarterly,
		EndPeriod:   latest,
		PeriodCount: 3,
		Periods:     []string{priorAnnual, latest, samePriorYear},
	}
	m.Revenue = ttmFormula(data, data.IncomeStatement, "营业收入", priorAnnual, latest, samePriorYear)
	m.NetProfit = ttmFormula(data, data.IncomeStatement, "净利润", priorAnnual, latest, samePriorYear)
	m.OperatingCash = ttmFormula(data, data.CashFlow, "经营活动产生的现金流量净额", priorAnnual, latest, samePriorYear)
	parentProfit := parentNetProfitAt(data, priorAnnual) +
		parentNetProfitAt(data, latest) -
		parentNetProfitAt(data, samePriorYear)
	m.applyRatios(data, latest, samePriorYear, parentProfit)
	return m
}

// ttmFormula 套用 TTM 标准公式：上一年报 + 最新季报 − 去年同期
func ttmFormula(data *FinancialData, table map[string]map[string]float64, account, priorAnnual, latest, samePriorYear string) float64 {
	return data.GetValueOrZero(table, account, priorAnnual) +
		data.GetValueOrZero(table, account, latest) -
		data.GetValueOrZero(table, account, samePriorYear)
}

// fallbackToRecentAnnual 降级使用最近一份年报；连年报都没有则返回 HasData=false
func fallbackToRecentAnnual(data *FinancialData, reason string) *TTMMetrics {
	if len(data.Years) == 0 {
		return &TTMMetrics{
			Notes: reason + "；且数据中无任何年报，无法计算 TTM",
		}
	}
	annual := data.Years[0]
	return buildFromAnnual(data, annual, TTMModeAnnualFallback,
		reason+"；已使用最近年报 "+annual+" 口径")
}

// buildFromAnnual 以单一年报构建 TTM
func buildFromAnnual(data *FinancialData, annual, mode, note string) *TTMMetrics {
	m := &TTMMetrics{
		HasData:     true,
		Mode:        mode,
		EndPeriod:   annual,
		Notes:       note,
		PeriodCount: 1,
		Periods:     []string{annual},
	}
	m.Revenue = data.GetValueOrZero(data.IncomeStatement, "营业收入", annual)
	m.NetProfit = data.GetValueOrZero(data.IncomeStatement, "净利润", annual)
	m.OperatingCash = data.GetValueOrZero(data.CashFlow, "经营活动产生的现金流量净额", annual)
	parentProfit := parentNetProfitAt(data, annual)
	m.applyRatios(data, annual, priorAnnualOf(annual), parentProfit)
	return m
}

// buildPriorTTMForPeriod 为上一期截止期计算 TTM 累计值（仅用于 3.4.1 表格对比）。
// 为保持可比口径，不使用全局 fallback，仅在所需三期/年报齐全时返回数据。
func buildPriorTTMForPeriod(data *FinancialData, endPeriod string) *TTMMetrics {
	if isAnnualPeriod(endPeriod) {
		m := &TTMMetrics{HasData: true}
		m.Revenue = data.GetValueOrZero(data.IncomeStatement, "营业收入", endPeriod)
		m.NetProfit = data.GetValueOrZero(data.IncomeStatement, "净利润", endPeriod)
		m.OperatingCash = data.GetValueOrZero(data.CashFlow, "经营活动产生的现金流量净额", endPeriod)
		return m
	}
	if len(endPeriod) < 10 {
		return &TTMMetrics{}
	}
	year, err := strconv.Atoi(endPeriod[:4])
	if err != nil {
		return &TTMMetrics{}
	}
	priorYearStr := strconv.Itoa(year - 1)
	priorAnnual := priorYearStr + "-12-31"
	samePriorYear := priorYearStr + endPeriod[4:]
	if !hasPeriod(data.Quarters, endPeriod) || !hasPeriod(data.Quarters, priorAnnual) || !hasPeriod(data.Quarters, samePriorYear) {
		return &TTMMetrics{}
	}
	m := &TTMMetrics{HasData: true}
	m.Revenue = ttmFormula(data, data.IncomeStatement, "营业收入", priorAnnual, endPeriod, samePriorYear)
	m.NetProfit = ttmFormula(data, data.IncomeStatement, "净利润", priorAnnual, endPeriod, samePriorYear)
	m.OperatingCash = ttmFormula(data, data.CashFlow, "经营活动产生的现金流量净额", priorAnnual, endPeriod, samePriorYear)
	return m
}

// applyRatios 计算比率指标。
//   - ROE 主路径：归母净利润 / 加权平均归母权益（与年度模块 step 16 口径对齐，
//     便于 3.4.2 的 ROE 与「模块 1 核心指标」「风险提示」「行业横向对比」等处保持一致）。
//   - 当数据源缺失「归母」字段（如旧 fixture 或部分非 A 股标的）时，降级回退到
//     整体口径：TTM 净利润 / 期末所有者权益合计（保持向后兼容）。
//   - 净利率 / 经营现金流比率仍按 TTM 整体净利润计算，与 3.4.1 显示的「净利润」一致。
func (m *TTMMetrics) applyRatios(data *FinancialData, equityPeriod, priorEquityPeriod string, parentProfit float64) {
	parentEquity := parentEquityAt(data, equityPeriod)
	priorParentEquity := 0.0
	if priorEquityPeriod != "" {
		priorParentEquity = parentEquityAt(data, priorEquityPeriod)
	}
	weightedParentEquity := parentEquity
	if priorParentEquity > 0 && parentEquity > 0 {
		weightedParentEquity = (priorParentEquity + parentEquity) / 2
	}

	switch {
	case weightedParentEquity > 0 && parentProfit != 0:
		m.ROE = parentProfit / weightedParentEquity
	default:
		// 降级：归母数据缺失 → 退回到整体口径
		equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", equityPeriod)
		if equity == 0 {
			totalAssets := data.GetValueOrZero(data.BalanceSheet, "资产合计", equityPeriod)
			totalLiabilities := data.GetValueOrZero(data.BalanceSheet, "负债合计", equityPeriod)
			equity = totalAssets - totalLiabilities
		}
		if equity > 0 {
			m.ROE = m.NetProfit / equity
		}
	}

	if m.Revenue > 0 {
		m.NetMargin = m.NetProfit / m.Revenue
	}
	if m.NetProfit > 0 {
		m.CashRatio = m.OperatingCash / m.NetProfit
	}
}

// parentNetProfitAt 取期末归母净利润，兼容多种 account 名称
func parentNetProfitAt(data *FinancialData, period string) float64 {
	v := data.GetValueOrZero(data.IncomeStatement, "归母净利润", period)
	if v == 0 {
		v = data.GetValueOrZero(data.IncomeStatement, "归属于母公司所有者的净利润", period)
	}
	return v
}

// parentEquityAt 取期末归母权益，兼容多种 account 名称
func parentEquityAt(data *FinancialData, period string) float64 {
	v := data.GetValueOrZero(data.BalanceSheet, "归母所有者权益合计", period)
	if v == 0 {
		v = data.GetValueOrZero(data.BalanceSheet, "归属于母公司所有者权益合计", period)
	}
	return v
}

// priorAnnualOf 给定年报期，返回去年同期年报；解析失败返回空串
func priorAnnualOf(annual string) string {
	if len(annual) < 4 {
		return ""
	}
	year, err := strconv.Atoi(annual[:4])
	if err != nil {
		return ""
	}
	if len(annual) == 4 {
		return strconv.Itoa(year - 1)
	}
	return strconv.Itoa(year-1) + annual[4:]
}

func isAnnualPeriod(p string) bool {
	if strings.HasSuffix(p, "-12-31") {
		return true
	}
	// 兼容 "2025" 这种 4 位年份格式
	if len(p) == 4 {
		if _, err := strconv.Atoi(p); err == nil {
			return true
		}
	}
	return false
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

// FormatTTMReport 格式化 TTM 指标为 Markdown 报告片段
func (m *TTMMetrics) FormatTTMReport() string {
	if !m.HasData {
		s := "> **TTM 数据不足**: 财报数据缺失，无法计算"
		if m.Notes != "" {
			s += "（" + m.Notes + "）"
		}
		return s + "\n\n"
	}
	var b strings.Builder

	// 口径说明
	b.WriteString("> **TTM 截止期**: ")
	b.WriteString(m.EndPeriod)
	b.WriteString("（覆盖区间 ")
	b.WriteString(ttmWindowHint(m.EndPeriod))
	b.WriteString("）\n>\n")

	b.WriteString("> **计算口径**: ")
	switch m.Mode {
	case TTMModeAnnual:
		b.WriteString("直接采用 " + m.EndPeriod + " 年报全年口径")
	case TTMModeQuarterly:
		if len(m.Periods) == 3 {
			b.WriteString(fmt.Sprintf("%s（上一年报） + %s（最新季报） − %s（去年同期）",
				m.Periods[0], m.Periods[1], m.Periods[2]))
		} else {
			b.WriteString("标准 TTM 公式")
		}
	case TTMModeAnnualFallback:
		b.WriteString("数据不足，降级为最近年报全年口径")
	default:
		b.WriteString("未知")
	}
	b.WriteString("\n")

	if m.Notes != "" {
		b.WriteString(">\n> **说明**: " + m.Notes + "\n")
	}
	b.WriteString("\n")

	// 表1：经营规模（TTM 累计值）
	b.WriteString("### 3.4.1 经营规模（TTM 累计值）\n\n")
	b.WriteString(fmt.Sprintf("| 指标 | 当期 TTM（%s） | 上期 TTM%s | TTM 同比 |\n", m.EndPeriod, formatPriorTTMHeader(m)))
	b.WriteString("|------|----------------|------------|----------|\n")
	b.WriteString(fmt.Sprintf("| 营业收入 | %.2f 亿元 | %s | %s |\n",
		m.Revenue/1e8, formatTTMValue(m.PriorRevenue, m.HasPrior), formatTTMChange(m.Revenue, m.PriorRevenue, m.HasPrior)))
	b.WriteString(fmt.Sprintf("| 净利润 | %.2f 亿元 | %s | %s |\n",
		m.NetProfit/1e8, formatTTMValue(m.PriorNetProfit, m.HasPrior), formatTTMChange(m.NetProfit, m.PriorNetProfit, m.HasPrior)))
	b.WriteString(fmt.Sprintf("| 经营现金流 | %.2f 亿元 | %s | %s |\n",
		m.OperatingCash/1e8, formatTTMValue(m.PriorOperatingCash, m.HasPrior), formatTTMChange(m.OperatingCash, m.PriorOperatingCash, m.HasPrior)))
	b.WriteString("\n")

	// 表2：盈利能力（比率）
	b.WriteString("### 3.4.2 盈利能力（比率）\n\n")
	b.WriteString("> ROE 采用「TTM 归母净利润 / 加权平均归母权益」口径（与模块 1、风险提示、行业横向对比保持一致）；净利率为 TTM 整体口径。\n\n")
	b.WriteString("| 指标 | 数值 | 参考标准 |\n")
	b.WriteString("|------|------|----------|\n")
	b.WriteString(fmt.Sprintf("| 净资产收益率（ROE，归母·加权） | %.2f%% | >15%% 优秀，>10%% 良好 |\n", m.ROE*100))
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

// ttmWindowHint 给出 TTM 12 个月覆盖区间的可读提示
// 例：2026-03-31 → "2025-04 ~ 2026-03"；2025-12-31 → "2025-01 ~ 2025-12"
func ttmWindowHint(endPeriod string) string {
	if isAnnualPeriod(endPeriod) {
		yearStr := endPeriod
		if len(endPeriod) >= 4 {
			yearStr = endPeriod[:4]
		}
		return fmt.Sprintf("%s-01 ~ %s-12", yearStr, yearStr)
	}
	if len(endPeriod) < 10 {
		return endPeriod
	}
	year, err := strconv.Atoi(endPeriod[:4])
	if err != nil {
		return endPeriod
	}
	month, err := strconv.Atoi(endPeriod[5:7])
	if err != nil {
		return endPeriod
	}
	startYear := year - 1
	startMonth := month + 1
	if startMonth > 12 {
		startMonth = 1
		startYear = year
	}
	return fmt.Sprintf("%d-%02d ~ %d-%02d", startYear, startMonth, year, month)
}

// formatPriorTTMHeader 上期 TTM 列头，带上截止期便于理解
func formatPriorTTMHeader(m *TTMMetrics) string {
	if !m.HasPrior || m.PriorEndPeriod == "" {
		return ""
	}
	return fmt.Sprintf("（%s）", m.PriorEndPeriod)
}

// formatTTMValue 格式化 TTM 数值列
func formatTTMValue(v float64, has bool) string {
	if !has {
		return "—"
	}
	return fmt.Sprintf("%.2f 亿元", v/1e8)
}

// formatTTMChange 格式化 TTM 同比变化，以绝对值作为分母避免负基数异常
func formatTTMChange(current, prior float64, has bool) string {
	if !has || prior == 0 {
		return "—"
	}
	change := (current - prior) / math.Abs(prior)
	sign := "+"
	if change < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.2f%%", sign, change*100)
}
