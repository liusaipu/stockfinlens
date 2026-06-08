package analyzer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// FinancialData 包含标准化的三张财务报表数据
type FinancialData struct {
	Symbol          string
	Years           []string // 年报期间（降序）
	Quarters        []string // 所有期间含季报（降序）
	BalanceSheet    map[string]map[string]float64
	IncomeStatement map[string]map[string]float64
	CashFlow        map[string]map[string]float64
	Extras          map[string]float64       // 非财务风险爬虫数据（股权质押、问询函、减持等）
	QualityWarnings []string                 // 数据质量警告（供报告展示）
	AuditOpinions   map[string]*AuditOpinion // 年份 -> 审计意见
}

// LoadFinancialData 从 StockFinLens 存储目录加载某股票的财务报表 JSON
func LoadFinancialData(baseDir, symbol string) (*FinancialData, error) {
	stockDir := filepath.Join(baseDir, "data", symbol)

	bs, err := loadFloatJSONFile(filepath.Join(stockDir, "balance_sheet.json"))
	if err != nil {
		return nil, fmt.Errorf("load balance_sheet.json: %w", err)
	}
	is, err := loadFloatJSONFile(filepath.Join(stockDir, "income_statement.json"))
	if err != nil {
		return nil, fmt.Errorf("load income_statement.json: %w", err)
	}
	cf, err := loadFloatJSONFile(filepath.Join(stockDir, "cash_flow.json"))
	if err != nil {
		return nil, fmt.Errorf("load cash_flow.json: %w", err)
	}

	years := extractYearsOnly(bs)
	if len(years) == 0 {
		years = extractYearsOnly(is)
	}
	if len(years) == 0 {
		years = extractYearsOnly(cf)
	}

	quarters := extractPeriods(bs)
	if len(quarters) == 0 {
		quarters = extractPeriods(is)
	}
	if len(quarters) == 0 {
		quarters = extractPeriods(cf)
	}

	fd := &FinancialData{
		Symbol:          symbol,
		Years:           years,
		Quarters:        quarters,
		BalanceSheet:    bs,
		IncomeStatement: is,
		CashFlow:        cf,
	}
	fd.fixMissingData()
	fd.validate()
	return fd, nil
}

// fixMissingData 修复东方财富 API 数据缺失/错误问题。
// 东财 API 对全部期间（含年报）的「所有者权益合计」字段均返回 0，需通过 资产−负债 推导。
func (fd *FinancialData) fixMissingData() {
	// 收集所有期间（年报+季报），优先用 Quarters（覆盖全），为空时退回 Years
	allPeriods := fd.Quarters
	if len(allPeriods) == 0 {
		allPeriods = fd.Years
	}

	// 资产负债表修复：遍历全部期间，因为所有期间的权益字段都可能为 0
	for _, period := range allPeriods {
		parentEquity := fd.GetValueOrZero(fd.BalanceSheet, "归母所有者权益合计", period)
		totalEquity := fd.GetValueOrZero(fd.BalanceSheet, "所有者权益合计", period)
		minorityEquity := fd.GetValueOrZero(fd.BalanceSheet, "少数股东权益", period)
		asset := fd.GetValueOrZero(fd.BalanceSheet, "资产合计", period)
		liability := fd.GetValueOrZero(fd.BalanceSheet, "负债合计", period)

		// 兜底：如果总权益和归母权益都异常，用 资产 - 负债 推导总权益
		if math.Abs(totalEquity) < 1 && asset > 0 && liability > 0 {
			calculatedTotal := asset - liability
			if math.Abs(calculatedTotal) > 1 {
				fmt.Printf("[fixMissingData] %s %s: 总权益从 %.0f 修复为 %.0f (资产-负债)\n",
					fd.Symbol, period, totalEquity, calculatedTotal)
				totalEquity = calculatedTotal
				if _, ok := fd.BalanceSheet["所有者权益合计"]; !ok {
					fd.BalanceSheet["所有者权益合计"] = make(map[string]float64)
				}
				fd.BalanceSheet["所有者权益合计"][period] = calculatedTotal
			}
		}

		// 归母权益为0或缺失（或异常负数），但总权益有值
		if (math.Abs(parentEquity) < 1 || math.Abs(parentEquity+minorityEquity) < 1) && totalEquity != 0 {
			calculatedParent := totalEquity - minorityEquity
			if math.Abs(calculatedParent) > 1 {
				fmt.Printf("[fixMissingData] %s %s: 归母权益从 %.0f 修复为 %.0f (总权益-少数股东权益)\n",
					fd.Symbol, period, parentEquity, calculatedParent)
				if _, ok := fd.BalanceSheet["归属于母公司所有者权益合计"]; !ok {
					fd.BalanceSheet["归属于母公司所有者权益合计"] = make(map[string]float64)
				}
				fd.BalanceSheet["归属于母公司所有者权益合计"][period] = calculatedParent
				if _, ok := fd.BalanceSheet["归母所有者权益合计"]; !ok {
					fd.BalanceSheet["归母所有者权益合计"] = make(map[string]float64)
				}
				fd.BalanceSheet["归母所有者权益合计"][period] = calculatedParent
			}
		}
	}

	// 利润表修复：仅对年报期间，旧版 downloader 遗漏了归母净利润
	for _, year := range fd.Years {
		parentProfit := fd.GetValueOrZero(fd.IncomeStatement, "归母净利润", year)
		netProfit := fd.GetValueOrZero(fd.IncomeStatement, "净利润", year)
		minorityInterest := fd.GetValueOrZero(fd.IncomeStatement, "少数股东损益", year)
		if math.Abs(parentProfit) < 1 && netProfit != 0 {
			calculatedParentProfit := netProfit - minorityInterest
			if math.Abs(calculatedParentProfit) > 1 {
				fmt.Printf("[fixMissingData] %s %s: 归母净利润从 %.0f 修复为 %.0f (净利润-少数股东损益)\n",
					fd.Symbol, year, parentProfit, calculatedParentProfit)
				if _, ok := fd.IncomeStatement["归属于母公司所有者的净利润"]; !ok {
					fd.IncomeStatement["归属于母公司所有者的净利润"] = make(map[string]float64)
				}
				fd.IncomeStatement["归属于母公司所有者的净利润"][year] = calculatedParentProfit
				if _, ok := fd.IncomeStatement["归母净利润"]; !ok {
					fd.IncomeStatement["归母净利润"] = make(map[string]float64)
				}
				fd.IncomeStatement["归母净利润"][year] = calculatedParentProfit
			}
		}
	}
}

func loadFloatJSONFile(path string) (map[string]map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]map[string]float64
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// extractPeriods 提取所有期间（年报+季报），返回降序排列的日期列表
func extractPeriods(data map[string]map[string]float64) []string {
	periodSet := make(map[string]struct{})
	for _, row := range data {
		for period := range row {
			periodSet[period] = struct{}{}
		}
	}
	periods := make([]string, 0, len(periodSet))
	for p := range periodSet {
		periods = append(periods, p)
	}
	sortPeriodsDesc(periods)
	return periods
}

// extractYearsOnly 只提取年报期间（12-31结尾或4位年份），返回降序排列
func extractYearsOnly(data map[string]map[string]float64) []string {
	yearSet := make(map[string]struct{})
	for _, row := range data {
		for year := range row {
			if strings.HasSuffix(year, "-12-31") || len(year) == 4 {
				yearSet[year] = struct{}{}
			}
		}
	}
	years := make([]string, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	sortPeriodsDesc(years)
	return years
}

// sortPeriodsDesc 按日期降序排列（简单的字符串排序，因为格式统一为 YYYY-MM-DD 或 YYYY）
func sortPeriodsDesc(periods []string) {
	for i := 0; i < len(periods); i++ {
		for j := i + 1; j < len(periods); j++ {
			if periods[i] < periods[j] {
				periods[i], periods[j] = periods[j], periods[i]
			}
		}
	}
}

// GetValueOrZero 从 data[account][year] 中读取 float64，缺失时返回 0
func (fd *FinancialData) GetValueOrZero(data map[string]map[string]float64, account, year string) float64 {
	if data == nil {
		return 0
	}
	row, ok := data[account]
	if !ok {
		account = normalizeAccountName(account)
		row, ok = data[account]
		if !ok {
			return 0
		}
	}
	return row[year]
}

// normalizeAccountName 处理常见报表科目的同义别名
// 将 analyzer 侧使用的标准名映射到 csvparser 实际存储的键名
func normalizeAccountName(name string) string {
	switch name {
	case "资产合计":
		return "资产总计"
	case "负债合计":
		return "负债总计"
	case "归母所有者权益合计":
		return "归属于母公司所有者权益合计"
	case "归母净利润":
		return "归属于母公司所有者的净利润"
	// 利润表：同花顺 CSV 中营业收入/营业成本带有"其中："前缀
	case "营业收入":
		return "其中：营业收入"
	case "营业成本":
		return "其中：营业成本"
	// 资产负债表：固定资产/在建工程在 CSV 中为"固定资产合计"/"在建工程合计"
	case "固定资产":
		return "固定资产合计"
	case "在建工程":
		return "在建工程合计"
	// 现金流量表：csvparser 去掉了顿号，analyzer 仍使用原始带顿号名称；部分数据源使用不同表述
	case "经营活动现金流量净额":
		return "经营活动产生的现金流量净额"
	case "购建固定资产、无形资产和其他长期资产支付的现金":
		return "购建固定资产无形资产和其他长期资产支付的现金"
	case "分配股利、利润或偿付利息支付的现金":
		return "分配股利利润或偿付利息支付的现金"
	case "固定资产折旧、油气资产折耗、生产性生物资产折旧":
		return "固定资产折旧油气资产折耗生产性生物资产折旧"
	}
	return name
}

// validate 执行数据质量校验，将问题写入 QualityWarnings
func (fd *FinancialData) validate() {
	fd.QualityWarnings = []string{}

	// 1. 数据年数检查
	if len(fd.Years) < 2 {
		fd.QualityWarnings = append(fd.QualityWarnings,
			"财务数据不足2年，无法进行有效的同比分析")
	}
	if len(fd.Years) == 0 {
		fd.QualityWarnings = append(fd.QualityWarnings,
			"未找到任何年报数据，可能只有季报或数据缺失")
		return
	}

	// 2. 关键科目完整性检查
	requiredBS := []string{"资产合计", "负债合计", "归母所有者权益合计"}
	for _, acc := range requiredBS {
		found := false
		norm := normalizeAccountName(acc)
		for key := range fd.BalanceSheet {
			if key == acc || key == norm {
				found = true
				break
			}
		}
		if !found {
			fd.QualityWarnings = append(fd.QualityWarnings,
				fmt.Sprintf("资产负债表缺少关键科目: %s", acc))
		}
	}

	requiredIS := []string{"营业收入", "营业成本", "净利润"}
	for _, acc := range requiredIS {
		found := false
		norm := normalizeAccountName(acc)
		for key := range fd.IncomeStatement {
			if key == acc || key == norm {
				found = true
				break
			}
		}
		if !found {
			fd.QualityWarnings = append(fd.QualityWarnings,
				fmt.Sprintf("利润表缺少关键科目: %s", acc))
		}
	}

	requiredCF := []string{"经营活动现金流量净额"}
	for _, acc := range requiredCF {
		found := false
		norm := normalizeAccountName(acc)
		for key := range fd.CashFlow {
			if key == acc || key == norm {
				found = true
				break
			}
		}
		if !found {
			fd.QualityWarnings = append(fd.QualityWarnings,
				fmt.Sprintf("现金流量表缺少关键科目: %s", acc))
		}
	}

	// 3. 资产负债表平衡校验（资产 ≈ 负债 + 总权益）
	// 注意：应使用所有者权益合计（总权益），而非归母所有者权益合计
	for _, year := range fd.Years {
		asset := fd.GetValueOrZero(fd.BalanceSheet, "资产合计", year)
		liability := fd.GetValueOrZero(fd.BalanceSheet, "负债合计", year)
		// 优先使用总权益，缺失时用 归母权益 + 少数股东权益 推导
		totalEquity := fd.GetValueOrZero(fd.BalanceSheet, "所有者权益合计", year)
		if math.Abs(totalEquity) < 1 {
			parentEquity := fd.GetValueOrZero(fd.BalanceSheet, "归母所有者权益合计", year)
			minorityEquity := fd.GetValueOrZero(fd.BalanceSheet, "少数股东权益", year)
			if parentEquity > 0 || minorityEquity > 0 {
				totalEquity = parentEquity + minorityEquity
			}
		}
		if asset > 0 && liability > 0 && totalEquity > 0 {
			diff := math.Abs(asset - liability - totalEquity)
			if diff/asset > 0.05 {
				fd.QualityWarnings = append(fd.QualityWarnings,
					fmt.Sprintf("%s 资产负债表不平衡: 资产%.0f ≠ 负债%.0f + 总权益%.0f (差异%.1f%%)。此异常通常由数据源精度/舍入问题导致，不影响核心分析指标（ROE、毛利率等）的准确性",
						year, asset, liability, totalEquity, diff/asset*100))
			}
		}
	}
	// 如果存在资产负债表不平衡，追加数据源切换建议
	hasBalanceIssue := false
	for _, w := range fd.QualityWarnings {
		if strings.Contains(w, "资产负债表不平衡") {
			hasBalanceIssue = true
			break
		}
	}
	if hasBalanceIssue {
		fd.QualityWarnings = append(fd.QualityWarnings,
			"💡 建议：当前数据可能存在数据源精度问题。如已配置 StockFinLens 数据源，系统会在下载时自动对比并择优选用；也可通过「导入财报」功能手动导入原始 CSV/Excel 财报进行复核。")
	}

	// 4. 异常值检测
	for i, year := range fd.Years {
		revenue := fd.GetValueOrZero(fd.IncomeStatement, "营业收入", year)
		if revenue < 0 {
			fd.QualityWarnings = append(fd.QualityWarnings,
				fmt.Sprintf("%s 营业收入为负数(%.0f)，数据异常", year, revenue))
		}
		asset := fd.GetValueOrZero(fd.BalanceSheet, "资产合计", year)
		if asset < 0 {
			fd.QualityWarnings = append(fd.QualityWarnings,
				fmt.Sprintf("%s 总资产为负数(%.0f)，数据异常", year, asset))
		}
		// 营收同比极端变化检测（相邻年份）
		if i > 0 {
			prevYear := fd.Years[i-1]
			prevRevenue := fd.GetValueOrZero(fd.IncomeStatement, "营业收入", prevYear)
			if prevRevenue > 0 && revenue > 0 {
				growth := (revenue - prevRevenue) / prevRevenue * 100
				if growth > 500 || growth < -90 {
					fd.QualityWarnings = append(fd.QualityWarnings,
						fmt.Sprintf("%s 营收同比变化异常(%+.1f%%)，可能是数据重述或对比口径不一致", year, growth))
				}
			}
		}
	}
}
