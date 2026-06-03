package analyzer

import (
	"fmt"
	"math"
)

// ==================== Step 1: 审计意见 ====================
func step1Audit(data *FinancialData) StepResult {
	result := StepResult{
		StepNum:    1,
		StepName:   "审计意见及审计事务所查询",
		YearlyData: make(map[string]map[string]any),
		Pass:       make(map[string]bool),
	}

	top10Auditors := map[string]bool{
		"普华永道": true, "安永": true, "毕马威": true, "德勤": true,
		"天健": true, "立信": true, "大华": true, "容诚": true,
		"天职国际": true, "信永中和": true,
	}

	for _, year := range data.Years {
		yd := map[string]any{
			"opinion":    "请查询年报确认",
			"auditor":    "请查询年报确认",
			"isTop10":    false,
			"isStandard": true,
		}
		pass := true

		if data.AuditOpinions != nil {
			if ao, ok := data.AuditOpinions[year]; ok && ao != nil {
				yd["opinion"] = ao.Opinion
				yd["auditor"] = ao.Auditor
				if ao.Auditor != "" {
					isTop10 := false
					for name := range top10Auditors {
						if contains(ao.Auditor, name) {
							isTop10 = true
							break
						}
					}
					yd["isTop10"] = isTop10
				} else {
					yd["isTop10"] = "未知"
				}
				yd["isStandard"] = ao.IsStandard
				if !ao.IsStandard {
					pass = false
					if ao.NeedsReview {
						yd["opinion"] = ao.Opinion + "（需人工复核）"
					}
				}
			}
		}

		result.YearlyData[year] = yd
		result.Pass[year] = pass
	}

	result.Conclusion = "审计意见数据来自巨潮资讯网公告解析，非标意见已自动标注。十大审计师事务所：普华永道、安永、毕马威、德勤、天健、立信、大华、容诚、天职国际、信永中和。"
	return result
}

// ==================== Step 2: 资产规模 ====================
func step2AssetScale(data *FinancialData) StepResult {
	result := StepResult{StepNum: 2, StepName: "资产规模分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	prevAsset := 0.0
	for i := len(data.Years) - 1; i >= 0; i-- {
		year := data.Years[i]
		asset := data.GetValueOrZero(data.BalanceSheet, "资产合计", year)
		growth := 0.0
		status := ""
		if prevAsset != 0 {
			growth = (asset - prevAsset) / prevAsset * 100
		}
		if growth >= 10 {
			status = "扩张期"
		} else if growth >= 0 {
			status = "稳定期"
		} else {
			status = "收缩期"
		}
		result.YearlyData[year] = map[string]any{"totalAsset": asset, "growthRate": growth, "status": status}
		result.Pass[year] = growth >= 0
		if prevAsset != 0 {
			trace := CalcTrace{
				Indicator: "总资产增长率",
				Year:      year,
				Formula:   "(本年资产合计 - 上年资产合计) / 上年资产合计 × 100%",
				Inputs: map[string]InputValue{
					"asset":     {Source: "资产负债表", Item: "资产合计", Year: year, Value: asset},
					"prevAsset": {Source: "资产负债表", Item: "资产合计", Year: "上一年", Value: prevAsset},
				},
				Steps: []CalcStep{
					{Desc: "计算总资产增长率", Expr: fmt.Sprintf("(%.0f - %.0f) / %.0f × 100%%", asset, prevAsset, prevAsset), Value: growth},
				},
				Result: growth,
			}
			result.Traces = append(result.Traces, trace)
		}
		prevAsset = asset
	}
	result.Conclusion = fmt.Sprintf("%s 最近一年资产规模%s。", data.Symbol, result.YearlyData[data.Years[0]]["status"])
	return result
}

// ==================== Step 3: 偿债能力 ====================
func step3Solvency(data *FinancialData) StepResult {
	result := StepResult{StepNum: 3, StepName: "偿债能力分析（资产负债率+准货币资金与有息负债）", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		asset := data.GetValueOrZero(data.BalanceSheet, "资产合计", year)
		liability := data.GetValueOrZero(data.BalanceSheet, "负债合计", year)
		debtRatio := 0.0
		if asset != 0 {
			debtRatio = liability / asset * 100
		}
		riskLevel := "低风险"
		if debtRatio > 70 {
			riskLevel = "高风险"
		} else if debtRatio > 60 {
			riskLevel = "中风险"
		} else if debtRatio > 40 {
			riskLevel = "中低风险"
		}

		cash := data.GetValueOrZero(data.BalanceSheet, "货币资金", year)
		tradingFin := data.GetValueOrZero(data.BalanceSheet, "交易性金融资产", year)
		quasiCash := cash + tradingFin

		interestBearing := 0.0
		for _, col := range []string{"短期借款", "一年内到期的非流动负债", "长期借款", "应付债券", "长期应付款"} {
			interestBearing += data.GetValueOrZero(data.BalanceSheet, col, year)
		}
		diff := quasiCash - interestBearing

		result.YearlyData[year] = map[string]any{
			"debtRatio":       debtRatio,
			"riskLevel":       riskLevel,
			"quasiCash":       quasiCash,
			"interestBearing": interestBearing,
			"cashDebtDiff":    diff,
		}
		result.Pass[year] = debtRatio <= 60 && diff >= 0

		traceDebt := CalcTrace{
			Indicator: "资产负债率",
			Year:      year,
			Formula:   "负债合计 / 资产合计 × 100%",
			Inputs: map[string]InputValue{
				"liability": {Source: "资产负债表", Item: "负债合计", Year: year, Value: liability},
				"asset":     {Source: "资产负债表", Item: "资产合计", Year: year, Value: asset},
			},
			Steps: []CalcStep{
				{Desc: "计算资产负债率", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", liability, asset), Value: debtRatio},
			},
			Result: debtRatio,
		}
		traceCash := CalcTrace{
			Indicator: "准货币资金-有息负债差额",
			Year:      year,
			Formula:   "(货币资金 + 交易性金融资产) - (短期借款 + 一年内到期的非流动负债 + 长期借款 + 应付债券 + 长期应付款)",
			Inputs: map[string]InputValue{
				"cash":            {Source: "资产负债表", Item: "货币资金", Year: year, Value: cash},
				"tradingFin":      {Source: "资产负债表", Item: "交易性金融资产", Year: year, Value: tradingFin},
				"interestBearing": {Source: "资产负债表", Item: "有息负债合计", Year: year, Value: interestBearing},
			},
			Steps: []CalcStep{
				{Desc: "计算准货币资金", Expr: fmt.Sprintf("%.0f + %.0f", cash, tradingFin), Value: quasiCash},
				{Desc: "计算差额", Expr: fmt.Sprintf("%.0f - %.0f", quasiCash, interestBearing), Value: diff},
			},
			Result: diff,
		}
		result.Traces = append(result.Traces, traceDebt, traceCash)
	}
	result.Conclusion = "资产负债率与准货币资金-有息负债差额综合判断偿债风险。"
	return result
}

// ==================== Step 4: 两头吃能力 ====================
func step4CompetitivePosition(data *FinancialData) StepResult {
	result := StepResult{StepNum: 4, StepName: "应付预收与应收预付分析（两头吃能力）", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		payable := data.GetValueOrZero(data.BalanceSheet, "应付票据及应付账款", year)
		advance := data.GetValueOrZero(data.BalanceSheet, "预收款项", year)
		contract := data.GetValueOrZero(data.BalanceSheet, "合同负债", year)
		receivable := data.GetValueOrZero(data.BalanceSheet, "应收票据及应收账款", year)
		prepayment := data.GetValueOrZero(data.BalanceSheet, "预付款项", year)
		totalPayable := payable + advance + contract
		totalReceivable := receivable + prepayment
		diff := totalPayable - totalReceivable
		result.YearlyData[year] = map[string]any{
			"payableAdvance":       totalPayable,
			"receivablePrepayment": totalReceivable,
			"diff":                 diff,
		}
		result.Pass[year] = diff > 0

		trace := CalcTrace{
			Indicator: "两头吃能力差额",
			Year:      year,
			Formula:   "(应付票据及应付账款 + 预收款项 + 合同负债) - (应收票据及应收账款 + 预付款项)",
			Inputs: map[string]InputValue{
				"payable":    {Source: "资产负债表", Item: "应付票据及应付账款", Year: year, Value: payable},
				"advance":    {Source: "资产负债表", Item: "预收款项", Year: year, Value: advance},
				"contract":   {Source: "资产负债表", Item: "合同负债", Year: year, Value: contract},
				"receivable": {Source: "资产负债表", Item: "应收票据及应收账款", Year: year, Value: receivable},
				"prepayment": {Source: "资产负债表", Item: "预付款项", Year: year, Value: prepayment},
			},
			Steps: []CalcStep{
				{Desc: "计算应付预收合计", Expr: fmt.Sprintf("%.0f + %.0f + %.0f", payable, advance, contract), Value: totalPayable},
				{Desc: "计算应收预付合计", Expr: fmt.Sprintf("%.0f + %.0f", receivable, prepayment), Value: totalReceivable},
				{Desc: "计算差额", Expr: fmt.Sprintf("%.0f - %.0f", totalPayable, totalReceivable), Value: diff},
			},
			Result: diff,
		}
		result.Traces = append(result.Traces, trace)
	}
	result.Conclusion = "差额大于0表示公司在产业链中具备强势地位，能占用上下游资金。"
	return result
}

// ==================== Step 5: 应收账款 ====================
func step5Receivables(data *FinancialData) StepResult {
	result := StepResult{StepNum: 5, StepName: "应收账款与合同资产分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		asset := data.GetValueOrZero(data.BalanceSheet, "资产合计", year)
		receivable := data.GetValueOrZero(data.BalanceSheet, "应收票据及应收账款", year)
		contractAsset := data.GetValueOrZero(data.BalanceSheet, "合同资产", year)
		total := receivable + contractAsset
		ratio := 0.0
		if asset != 0 {
			ratio = total / asset * 100
		}
		comp := ""
		switch {
		case ratio < 1:
			comp = "产品极畅销"
		case ratio < 3:
			comp = "产品畅销"
		case ratio < 10:
			comp = "产品销售正常"
		case ratio < 20:
			comp = "产品销售难度大"
		default:
			comp = "产品销售极难"
		}
		result.YearlyData[year] = map[string]any{"totalReceivable": total, "ratio": ratio, "competitiveness": comp}
		result.Pass[year] = ratio <= 10

		trace := CalcTrace{
			Indicator: "应收类资产占比",
			Year:      year,
			Formula:   "(应收票据及应收账款 + 合同资产) / 资产合计 × 100%",
			Inputs: map[string]InputValue{
				"receivable":    {Source: "资产负债表", Item: "应收票据及应收账款", Year: year, Value: receivable},
				"contractAsset": {Source: "资产负债表", Item: "合同资产", Year: year, Value: contractAsset},
				"asset":         {Source: "资产负债表", Item: "资产合计", Year: year, Value: asset},
			},
			Steps: []CalcStep{
				{Desc: "计算应收类资产合计", Expr: fmt.Sprintf("%.0f + %.0f", receivable, contractAsset), Value: total},
				{Desc: "计算占比", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", total, asset), Value: ratio},
			},
			Result: ratio,
		}
		result.Traces = append(result.Traces, trace)
	}
	result.Conclusion = "应收类资产占比越低，说明产品竞争力越强、销售回款越好。"
	return result
}

// ==================== Step 6: 固定资产 ====================
func step6FixedAssets(data *FinancialData) StepResult {
	result := StepResult{StepNum: 6, StepName: "固定资产分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		asset := data.GetValueOrZero(data.BalanceSheet, "资产合计", year)
		fixed := data.GetValueOrZero(data.BalanceSheet, "固定资产", year)
		construction := data.GetValueOrZero(data.BalanceSheet, "在建工程", year)
		materials := data.GetValueOrZero(data.BalanceSheet, "工程物资", year)
		total := fixed + construction + materials
		ratio := 0.0
		if asset != 0 {
			ratio = total / asset * 100
		}
		companyType := "轻资产型"
		if ratio >= 40 {
			companyType = "重资产型"
		}
		result.YearlyData[year] = map[string]any{"totalFixed": total, "ratio": ratio, "companyType": companyType}
		result.Pass[year] = ratio <= 40

		trace := CalcTrace{
			Indicator: "固定资产工程占比",
			Year:      year,
			Formula:   "(固定资产 + 在建工程 + 工程物资) / 资产合计 × 100%",
			Inputs: map[string]InputValue{
				"fixed":        {Source: "资产负债表", Item: "固定资产", Year: year, Value: fixed},
				"construction": {Source: "资产负债表", Item: "在建工程", Year: year, Value: construction},
				"materials":    {Source: "资产负债表", Item: "工程物资", Year: year, Value: materials},
				"asset":        {Source: "资产负债表", Item: "资产合计", Year: year, Value: asset},
			},
			Steps: []CalcStep{
				{Desc: "计算固定资产工程合计", Expr: fmt.Sprintf("%.0f + %.0f + %.0f", fixed, construction, materials), Value: total},
				{Desc: "计算占比", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", total, asset), Value: ratio},
			},
			Result: ratio,
		}
		result.Traces = append(result.Traces, trace)
	}
	result.Conclusion = "固定资产工程占比低于40%为轻资产型，维持竞争力成本相对较低。"
	return result
}

// ==================== Step 7: 投资类资产 ====================
func step7InvestmentAssets(data *FinancialData) StepResult {
	result := StepResult{StepNum: 7, StepName: "投资类资产与主业专注度分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	cols := []string{"可供出售金融资产", "持有至到期投资", "长期股权投资", "其他权益工具投资", "其他非流动金融资产"}
	for _, year := range data.Years {
		asset := data.GetValueOrZero(data.BalanceSheet, "资产合计", year)
		investment := 0.0
		for _, col := range cols {
			investment += data.GetValueOrZero(data.BalanceSheet, col, year)
		}
		ratio := 0.0
		if asset != 0 {
			ratio = investment / asset * 100
		}
		focus := "专注主业"
		if ratio >= 10 {
			focus = "主业不够专注"
		}
		result.YearlyData[year] = map[string]any{"investmentAssets": investment, "ratio": ratio, "focus": focus}
		result.Pass[year] = ratio <= 10

		trace := CalcTrace{
			Indicator: "投资类资产占比",
			Year:      year,
			Formula:   "(可供出售金融资产 + 持有至到期投资 + 长期股权投资 + 其他权益工具投资 + 其他非流动金融资产) / 资产合计 × 100%",
			Inputs: map[string]InputValue{
				"investment": {Source: "资产负债表", Item: "投资类资产合计", Year: year, Value: investment},
				"asset":      {Source: "资产负债表", Item: "资产合计", Year: year, Value: asset},
			},
			Steps: []CalcStep{
				{Desc: "计算投资类资产占比", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", investment, asset), Value: ratio},
			},
			Result: ratio,
		}
		result.Traces = append(result.Traces, trace)
	}
	result.Conclusion = "投资类资产占比低于10%说明公司专注主业，为优秀公司特征。"
	return result
}

// ==================== Step 9: 营业收入 ====================
func step9RevenueGrowth(data *FinancialData) StepResult {
	result := StepResult{StepNum: 9, StepName: "营业收入与成长性分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	prevRevenue := 0.0
	for i := len(data.Years) - 1; i >= 0; i-- {
		year := data.Years[i]
		revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
		growth := 0.0
		status := ""
		if prevRevenue != 0 {
			growth = (revenue - prevRevenue) / prevRevenue * 100
		}
		if growth > 10 {
			status = "成长较快"
		} else if growth > 0 {
			status = "成长缓慢"
		} else {
			status = "可能衰落"
		}
		result.YearlyData[year] = map[string]any{"revenue": revenue, "growthRate": growth, "status": status}
		result.Pass[year] = growth >= 10
		if prevRevenue != 0 {
			trace := CalcTrace{
				Indicator: "营业收入增长率",
				Year:      year,
				Formula:   "(本年营业收入 - 上年营业收入) / 上年营业收入 × 100%",
				Inputs: map[string]InputValue{
					"revenue":     {Source: "利润表", Item: "营业收入", Year: year, Value: revenue},
					"prevRevenue": {Source: "利润表", Item: "营业收入", Year: "上一年", Value: prevRevenue},
				},
				Steps: []CalcStep{
					{Desc: "计算营业收入增长率", Expr: fmt.Sprintf("(%.0f - %.0f) / %.0f × 100%%", revenue, prevRevenue, prevRevenue), Value: growth},
				},
				Result: growth,
			}
			result.Traces = append(result.Traces, trace)
		}
		prevRevenue = revenue
	}
	result.Conclusion = "营业收入增长率反映公司成长性，持续高于10%为优秀。"
	return result
}

// getFirstAvailable 尝试多个候选键名，返回第一个非零值
func getFirstAvailable(data *FinancialData, stmt map[string]map[string]float64, year string, candidates ...string) float64 {
	for _, key := range candidates {
		val := data.GetValueOrZero(stmt, key, year)
		if val != 0 {
			return val
		}
	}
	return 0
}

// ==================== Step 10: 毛利率 ====================
func step10GrossMargin(data *FinancialData) StepResult {
	result := StepResult{StepNum: 10, StepName: "毛利率与产品竞争力分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	prevMargin := 0.0
	for i := len(data.Years) - 1; i >= 0; i-- {
		year := data.Years[i]

		// 收入字段：港股常用"营运收入"，A股用"营业收入"；部分报表也会直接提供"营业额""收益"
		revenue := getFirstAvailable(data, data.IncomeStatement, year, "营运收入", "营业收入", "营业额", "收益")
		// 成本字段：港股常用"销售成本"，A股用"营业成本"
		cost := getFirstAvailable(data, data.IncomeStatement, year, "销售成本", "营业成本")

		margin := 0.0
		if revenue != 0 {
			margin = (revenue - cost) / revenue * 100
		}

		// 交叉验证：若计算结果异常（负值或>100%）但报表已提供正毛利，用报表毛利复核
		reportedGrossProfit := getFirstAvailable(data, data.IncomeStatement, year, "毛利")
		if (margin < 0 || margin > 100) && reportedGrossProfit > 0 && revenue > 0 {
			margin = reportedGrossProfit / revenue * 100
		}

		volatility := 0.0
		if prevMargin != 0 {
			volatility = math.Abs(margin-prevMargin) / prevMargin * 100
		}
		comp := "竞争力强"
		if margin < 40 {
			comp = "竞争力弱"
		}
		risk := "经营稳定"
		if volatility > 20 {
			risk = "经营风险大"
		}
		result.YearlyData[year] = map[string]any{"grossMargin": margin, "volatility": volatility, "competitiveness": comp, "risk": risk}
		result.Pass[year] = margin >= 40
		prevMargin = margin

		trace := CalcTrace{
			Indicator: "毛利率",
			Year:      year,
			Formula:   "(营业收入 - 营业成本) / 营业收入 × 100%",
			Inputs: map[string]InputValue{
				"revenue": {Source: "利润表", Item: "营业收入", Year: year, Value: revenue},
				"cost":    {Source: "利润表", Item: "营业成本", Year: year, Value: cost},
			},
			Steps: []CalcStep{
				{Desc: "计算毛利额", Expr: fmt.Sprintf("%.0f - %.0f", revenue, cost), Value: revenue - cost},
				{Desc: "计算毛利率", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", revenue-cost, revenue), Value: margin},
			},
			Result: margin,
			Note:   "",
		}
		result.Traces = append(result.Traces, trace)
	}
	result.Conclusion = "毛利率高于40%为高毛利，竞争力强；波动幅度大于20%需警惕经营或造假风险。"
	return result
}

// ==================== Step 11: 营运能力 ====================
func step11OperationEfficiency(data *FinancialData) StepResult {
	result := StepResult{StepNum: 11, StepName: "营运能力分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	prevInventory := 0.0
	prevAsset := 0.0
	for i := len(data.Years) - 1; i >= 0; i-- {
		year := data.Years[i]
		revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
		cost := data.GetValueOrZero(data.IncomeStatement, "营业成本", year)
		asset := data.GetValueOrZero(data.BalanceSheet, "资产合计", year)
		inventory := data.GetValueOrZero(data.BalanceSheet, "存货", year)

		invTurnover := 0.0
		if prevInventory != 0 {
			avgInv := (inventory + prevInventory) / 2
			if avgInv > 0 {
				invTurnover = cost / avgInv
			}
		}
		assetTurnover := 0.0
		if prevAsset != 0 {
			avgAsset := (asset + prevAsset) / 2
			if avgAsset > 0 {
				assetTurnover = revenue / avgAsset
			}
		}
		result.YearlyData[year] = map[string]any{"inventoryTurnover": invTurnover, "assetTurnover": assetTurnover}
		result.Pass[year] = invTurnover >= 1 && assetTurnover >= 0.8
		if prevInventory != 0 {
			avgInv := (inventory + prevInventory) / 2
			traceInv := CalcTrace{
				Indicator: "存货周转率",
				Year:      year,
				Formula:   "营业成本 / [(本年存货 + 上年存货) / 2]",
				Inputs: map[string]InputValue{
					"cost":          {Source: "利润表", Item: "营业成本", Year: year, Value: cost},
					"inventory":     {Source: "资产负债表", Item: "存货", Year: year, Value: inventory},
					"prevInventory": {Source: "资产负债表", Item: "存货", Year: "上一年", Value: prevInventory},
				},
				Steps: []CalcStep{
					{Desc: "计算平均存货", Expr: fmt.Sprintf("(%.0f + %.0f) / 2", inventory, prevInventory), Value: avgInv},
					{Desc: "计算存货周转率", Expr: fmt.Sprintf("%.0f / %.2f", cost, avgInv), Value: invTurnover},
				},
				Result: invTurnover,
			}
			result.Traces = append(result.Traces, traceInv)
		}
		if prevAsset != 0 {
			avgAsset := (asset + prevAsset) / 2
			traceAsset := CalcTrace{
				Indicator: "总资产周转率",
				Year:      year,
				Formula:   "营业收入 / [(本年资产合计 + 上年资产合计) / 2]",
				Inputs: map[string]InputValue{
					"revenue":   {Source: "利润表", Item: "营业收入", Year: year, Value: revenue},
					"asset":     {Source: "资产负债表", Item: "资产合计", Year: year, Value: asset},
					"prevAsset": {Source: "资产负债表", Item: "资产合计", Year: "上一年", Value: prevAsset},
				},
				Steps: []CalcStep{
					{Desc: "计算平均总资产", Expr: fmt.Sprintf("(%.0f + %.0f) / 2", asset, prevAsset), Value: avgAsset},
					{Desc: "计算总资产周转率", Expr: fmt.Sprintf("%.0f / %.2f", revenue, avgAsset), Value: assetTurnover},
				},
				Result: assetTurnover,
			}
			result.Traces = append(result.Traces, traceAsset)
		}
		prevInventory = inventory
		prevAsset = asset
	}
	result.Conclusion = "存货周转率越高越好，总资产周转率大于0.8说明资产利用效率较好。"
	return result
}

// ==================== Step 12: 成本管控 ====================
func step12CostControl(data *FinancialData) StepResult {
	result := StepResult{StepNum: 12, StepName: "成本管控能力分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
		cost := data.GetValueOrZero(data.IncomeStatement, "营业成本", year)
		expense := 0.0
		for _, col := range []string{"销售费用", "管理费用", "研发费用", "财务费用"} {
			expense += data.GetValueOrZero(data.IncomeStatement, col, year)
		}
		expenseRatio := 0.0
		if revenue != 0 {
			expenseRatio = expense / revenue * 100
		}
		grossMargin := 0.0
		if revenue != 0 {
			grossMargin = (revenue - cost) / revenue * 100
		}
		expenseToMargin := 0.0
		control := "毛利为负"
		if grossMargin > 0 {
			expenseToMargin = expenseRatio / grossMargin * 100
			if expenseToMargin < 40 {
				control = "成本管控好"
			} else {
				control = "成本管控差"
			}
		}
		result.YearlyData[year] = map[string]any{"expenseRatio": expenseRatio, "grossMargin": grossMargin, "expenseToMargin": expenseToMargin, "control": control}
		result.Pass[year] = grossMargin > 0 && expenseToMargin < 40

		trace := CalcTrace{
			Indicator: "期间费用率占毛利率之比",
			Year:      year,
			Formula:   "(销售费用 + 管理费用 + 研发费用 + 财务费用) / 营业收入 ÷ [(营业收入 - 营业成本) / 营业收入] × 100%",
			Inputs: map[string]InputValue{
				"expense": {Source: "利润表", Item: "期间费用合计", Year: year, Value: expense},
				"revenue": {Source: "利润表", Item: "营业收入", Year: year, Value: revenue},
				"cost":    {Source: "利润表", Item: "营业成本", Year: year, Value: cost},
			},
			Steps: []CalcStep{
				{Desc: "计算期间费用率", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", expense, revenue), Value: expenseRatio},
				{Desc: "计算毛利率", Expr: fmt.Sprintf("(%.0f - %.0f) / %.0f × 100%%", revenue, cost, revenue), Value: grossMargin},
				{Desc: "计算费用占毛利比", Expr: fmt.Sprintf("%.2f / %.2f × 100%%", expenseRatio, grossMargin), Value: expenseToMargin},
			},
			Result: expenseToMargin,
		}
		result.Traces = append(result.Traces, trace)
	}
	result.Conclusion = "期间费用率占毛利率之比低于40%，说明成本管控能力较好。"
	return result
}

// ==================== Step 13: 费用率 ====================
func step13ExpenseRatio(data *FinancialData) StepResult {
	result := StepResult{StepNum: 13, StepName: "研发费用率与销售费用率分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
		rd := data.GetValueOrZero(data.IncomeStatement, "研发费用", year)
		sales := data.GetValueOrZero(data.IncomeStatement, "销售费用", year)
		rdRatio := 0.0
		salesRatio := 0.0
		if revenue != 0 {
			rdRatio = rd / revenue * 100
			salesRatio = sales / revenue * 100
		}
		innovation := "创新投入不足"
		if rdRatio > 5 {
			innovation = "重视创新"
		}
		salesDiff := "销售正常"
		if salesRatio > 30 {
			salesDiff = "销售难度大"
		}
		result.YearlyData[year] = map[string]any{"rdRatio": rdRatio, "salesRatio": salesRatio, "innovation": innovation, "salesDifficulty": salesDiff}
		result.Pass[year] = rdRatio >= 5 && salesRatio <= 30

		traceRD := CalcTrace{
			Indicator: "研发费用率",
			Year:      year,
			Formula:   "研发费用 / 营业收入 × 100%",
			Inputs: map[string]InputValue{
				"rd":      {Source: "利润表", Item: "研发费用", Year: year, Value: rd},
				"revenue": {Source: "利润表", Item: "营业收入", Year: year, Value: revenue},
			},
			Steps: []CalcStep{
				{Desc: "计算研发费用率", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", rd, revenue), Value: rdRatio},
			},
			Result: rdRatio,
		}
		traceSales := CalcTrace{
			Indicator: "销售费用率",
			Year:      year,
			Formula:   "销售费用 / 营业收入 × 100%",
			Inputs: map[string]InputValue{
				"sales":   {Source: "利润表", Item: "销售费用", Year: year, Value: sales},
				"revenue": {Source: "利润表", Item: "营业收入", Year: year, Value: revenue},
			},
			Steps: []CalcStep{
				{Desc: "计算销售费用率", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", sales, revenue), Value: salesRatio},
			},
			Result: salesRatio,
		}
		result.Traces = append(result.Traces, traceRD, traceSales)
	}
	result.Conclusion = "研发费用率大于5%说明重视创新，销售费用率低于30%说明产品销售难度较小。"
	return result
}

// ==================== Step 14: 主营利润 ====================
func step14CoreProfit(data *FinancialData) StepResult {
	result := StepResult{StepNum: 14, StepName: "主营利润与主业盈利能力、利润质量分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
		cost := data.GetValueOrZero(data.IncomeStatement, "营业成本", year)
		tax := data.GetValueOrZero(data.IncomeStatement, "税金及附加", year)
		expense := 0.0
		for _, col := range []string{"销售费用", "管理费用", "研发费用", "财务费用"} {
			expense += data.GetValueOrZero(data.IncomeStatement, col, year)
		}
		coreProfit := revenue - cost - tax - expense
		operatingProfit := data.GetValueOrZero(data.IncomeStatement, "营业利润", year)
		coreMargin := 0.0
		if revenue != 0 {
			coreMargin = coreProfit / revenue * 100
		}
		coreRatio := 0.0
		if operatingProfit != 0 {
			coreRatio = coreProfit / operatingProfit * 100
		}
		profitability := "盈利能力弱"
		if coreMargin > 15 {
			profitability = "盈利能力强"
		}
		quality := "利润质量低"
		if coreRatio > 80 {
			quality = "利润质量高"
		}
		result.YearlyData[year] = map[string]any{"coreProfit": coreProfit, "coreProfitMargin": coreMargin, "coreProfitRatio": coreRatio, "profitability": profitability, "quality": quality}
		result.Pass[year] = coreMargin >= 15 && coreRatio >= 80

		traceCoreMargin := CalcTrace{
			Indicator: "主营利润率",
			Year:      year,
			Formula:   "(营业收入 - 营业成本 - 税金及附加 - 期间费用) / 营业收入 × 100%",
			Inputs: map[string]InputValue{
				"revenue": {Source: "利润表", Item: "营业收入", Year: year, Value: revenue},
				"cost":    {Source: "利润表", Item: "营业成本", Year: year, Value: cost},
				"tax":     {Source: "利润表", Item: "税金及附加", Year: year, Value: tax},
				"expense": {Source: "利润表", Item: "期间费用合计", Year: year, Value: expense},
			},
			Steps: []CalcStep{
				{Desc: "计算主营利润", Expr: fmt.Sprintf("%.0f - %.0f - %.0f - %.0f", revenue, cost, tax, expense), Value: coreProfit},
				{Desc: "计算主营利润率", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", coreProfit, revenue), Value: coreMargin},
			},
			Result: coreMargin,
		}
		traceCoreRatio := CalcTrace{
			Indicator: "主营利润占营业利润比",
			Year:      year,
			Formula:   "主营利润 / 营业利润 × 100%",
			Inputs: map[string]InputValue{
				"coreProfit":      {Source: "利润表", Item: "主营利润", Year: year, Value: coreProfit},
				"operatingProfit": {Source: "利润表", Item: "营业利润", Year: year, Value: operatingProfit},
			},
			Steps: []CalcStep{
				{Desc: "计算主营利润占比", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", coreProfit, operatingProfit), Value: coreRatio},
			},
			Result: coreRatio,
		}
		result.Traces = append(result.Traces, traceCoreMargin, traceCoreRatio)
	}
	result.Conclusion = "主营利润率大于15%且主营利润占营业利润比大于80%，说明主业盈利能力强、利润质量高。"
	return result
}

// ==================== Step 15: 现金流质量 ====================
func step15CashFlowQuality(data *FinancialData) StepResult {
	result := StepResult{StepNum: 15, StepName: "净利润与现金流分析（净利润含金量）", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		ocf := data.GetValueOrZero(data.CashFlow, "经营活动现金流量净额", year)
		netProfit := data.GetValueOrZero(data.IncomeStatement, "净利润", year)
		ratio := 0.0
		if netProfit != 0 {
			ratio = ocf / netProfit * 100
		}
		quality := "含金量低"
		if ratio >= 100 {
			quality = "含金量高"
		}
		result.YearlyData[year] = map[string]any{"operatingCF": ocf, "netProfit": netProfit, "cashRatio": ratio, "quality": quality}

		trace := CalcTrace{
			Indicator: "净利润现金比率",
			Year:      year,
			Formula:   "经营活动现金流量净额 / 净利润 × 100%",
			Inputs: map[string]InputValue{
				"ocf":       {Source: "现金流量表", Item: "经营活动现金流量净额", Year: year, Value: ocf},
				"netProfit": {Source: "利润表", Item: "净利润", Year: year, Value: netProfit},
			},
			Steps: []CalcStep{
				{Desc: "计算净利润现金比率", Expr: fmt.Sprintf("%.0f / %.0f × 100%%", ocf, netProfit), Value: ratio},
			},
			Result: ratio,
		}
		result.Traces = append(result.Traces, trace)
	}
	// 5年均值判断
	if len(data.Years) >= 5 {
		sum := 0.0
		count := 0
		for i := 0; i < 5 && i < len(data.Years); i++ {
			if v, ok := result.YearlyData[data.Years[i]]["cashRatio"].(float64); ok {
				sum += v
				count++
			}
		}
		avg := 0.0
		if count > 0 {
			avg = sum / float64(count)
		}
		for i := 0; i < 5 && i < len(data.Years); i++ {
			result.Pass[data.Years[i]] = avg >= 100
		}
		for _, year := range data.Years[5:] {
			result.Pass[year] = true // 数据不足5年的默认通过
		}
	} else {
		for _, year := range data.Years {
			result.Pass[year] = true // 数据不足5年暂不扣分
		}
	}
	result.Conclusion = "过去5年净利润现金比率平均值大于100%，说明净利润含金量高。"
	return result
}

// ==================== Step 16: ROE ====================
func step16ROE(data *FinancialData) StepResult {
	result := StepResult{StepNum: 16, StepName: "净利润与净资产收益率分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	prevProfit := 0.0
	prevEquity := 0.0
	for i := len(data.Years) - 1; i >= 0; i-- {
		year := data.Years[i]
		equity := data.GetValueOrZero(data.BalanceSheet, "归母所有者权益合计", year)
		equityItem := "归母所有者权益合计"
		if equity == 0 {
			equity = data.GetValueOrZero(data.BalanceSheet, "归属于母公司所有者权益合计", year)
			equityItem = "归属于母公司所有者权益合计"
		}
		if equity == 0 {
			equity = data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", year)
			equityItem = "所有者权益合计"
		}
		profit := data.GetValueOrZero(data.IncomeStatement, "归母净利润", year)
		profitItem := "归母净利润"
		if profit == 0 {
			profit = data.GetValueOrZero(data.IncomeStatement, "归属于母公司所有者的净利润", year)
			profitItem = "归属于母公司所有者的净利润"
		}

		// 加权平均ROE：分母采用 (期初权益 + 期末权益) / 2 的近似
		weightedEquity := equity
		if prevEquity > 0 {
			weightedEquity = (prevEquity + equity) / 2
		}
		roe := 0.0
		if weightedEquity > 0 {
			roe = profit / weightedEquity * 100
		}

		growth := 0.0
		if prevProfit != 0 {
			growth = (profit - prevProfit) / math.Abs(prevProfit) * 100
		}
		level := "需关注"
		if roe > 20 {
			level = "最优"
		} else if roe > 15 {
			level = "优秀"
		}
		result.YearlyData[year] = map[string]any{"roe": roe, "profit": profit, "profitGrowth": growth, "roeLevel": level}
		result.Pass[year] = roe >= 15 && growth >= 10
		if i == len(data.Years)-1 {
			result.Pass[year] = roe >= 15 // 第一年没有上年数据，增长率不考核
		}

		trace := CalcTrace{
			Indicator: "ROE",
			Year:      year,
			Formula:   "归母净利润 / [(期初归母所有者权益 + 期末归母所有者权益) / 2] × 100%",
			Inputs: map[string]InputValue{
				"profit":     {Source: "利润表", Item: profitItem, Year: year, Value: profit},
				"equity":     {Source: "资产负债表", Item: equityItem, Year: year, Value: equity, Note: "取期末值"},
				"prevEquity": {Source: "资产负债表", Item: equityItem, Year: "上一年末", Value: prevEquity},
			},
			Steps: []CalcStep{
				{Desc: "计算加权平均净资产", Expr: fmt.Sprintf("(%.0f + %.0f) / 2", prevEquity, equity), Value: weightedEquity},
				{Desc: "计算加权平均ROE", Expr: fmt.Sprintf("%.0f / %.2f × 100%%", profit, weightedEquity), Value: roe},
			},
			Result: roe,
			Note:   "当前使用加权平均ROE近似值（分母 = (期初+期末)/2）。若年内存在增发、回购或大额分红，建议以年报披露的精确加权平均ROE为准。",
		}
		result.Traces = append(result.Traces, trace)

		prevProfit = profit
		prevEquity = equity
	}
	result.Conclusion = "ROE持续大于15%为优秀，大于20%为最优；归母净利润增长率大于10%说明成长性较好。"
	return result
}

// ==================== Step 17: 资本支出 ====================
func step17CAPEX(data *FinancialData) StepResult {
	result := StepResult{StepNum: 17, StepName: "购建长期资产现金支出分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	for _, year := range data.Years {
		ocf := data.GetValueOrZero(data.CashFlow, "经营活动现金流量净额", year)
		capex := data.GetValueOrZero(data.CashFlow, "购建固定资产、无形资产和其他长期资产支付的现金", year)
		ratio := 0.0
		if ocf != 0 {
			ratio = capex / math.Abs(ocf) * 100
		}
		assessment := "一般"
		if ratio >= 3 && ratio <= 60 {
			assessment = "增长潜力大、风险小"
		} else if ratio > 100 {
			assessment = "风险大"
		} else if ratio < 3 {
			assessment = "回报低"
		}
		result.YearlyData[year] = map[string]any{"capex": capex, "operatingCF": ocf, "ratio": ratio, "assessment": assessment}
		result.Pass[year] = ratio >= 3 && ratio <= 60

		trace := CalcTrace{
			Indicator: "购建长期资产现金支出占比",
			Year:      year,
			Formula:   "购建固定资产、无形资产和其他长期资产支付的现金 / |经营活动现金流量净额| × 100%",
			Inputs: map[string]InputValue{
				"capex": {Source: "现金流量表", Item: "购建固定资产、无形资产和其他长期资产支付的现金", Year: year, Value: capex},
				"ocf":   {Source: "现金流量表", Item: "经营活动现金流量净额", Year: year, Value: ocf},
			},
			Steps: []CalcStep{
				{Desc: "计算占比", Expr: fmt.Sprintf("%.0f / |%.0f| × 100%%", capex, ocf), Value: ratio},
			},
			Result: ratio,
		}
		result.Traces = append(result.Traces, trace)
	}
	result.Conclusion = "购建长期资产现金支出占经营活动现金流比率在3%~60%之间，说明增长潜力大且风险可控。"
	return result
}

// ==================== 分红分析 ====================
func step18Dividend(data *FinancialData) StepResult {
	result := StepResult{StepNum: 18, StepName: "分配股利、利润或偿付利息支付的现金分析", YearlyData: make(map[string]map[string]any), Pass: make(map[string]bool)}
	allZero := true
	for _, year := range data.Years {
		ocf := data.GetValueOrZero(data.CashFlow, "经营活动现金流量净额", year)
		dividend := data.GetValueOrZero(data.CashFlow, "分配股利、利润或偿付利息支付的现金", year)
		// 若主字段缺失为0，尝试其中明细字段（部分数据源仅有子项）
		if dividend == 0 {
			dividend = data.GetValueOrZero(data.CashFlow, "其中：子公司支付给少数股东的股利、利润", year)
		}
		ratio := 0.0
		if ocf != 0 {
			ratio = dividend / math.Abs(ocf) * 100
		}
		sustainability := "分红能力有问题"
		if dividend == 0 && ocf != 0 {
			sustainability = "分红数据缺失或该年度未实施现金分红"
		} else if ratio >= 20 && ratio <= 70 {
			sustainability = "分红可持续"
		} else if ratio > 70 {
			sustainability = "分红难持续"
		}
		if dividend != 0 {
			allZero = false
		}
		result.YearlyData[year] = map[string]any{"dividend": dividend, "operatingCF": ocf, "ratio": ratio, "sustainability": sustainability}
		result.Pass[year] = ratio >= 20 && ratio <= 70

		trace := CalcTrace{
			Indicator: "分红现金支出占经营现金流比",
			Year:      year,
			Formula:   "分配股利、利润或偿付利息支付的现金 / |经营活动现金流量净额| × 100%",
			Inputs: map[string]InputValue{
				"dividend": {Source: "现金流量表", Item: "分配股利、利润或偿付利息支付的现金", Year: year, Value: dividend},
				"ocf":      {Source: "现金流量表", Item: "经营活动现金流量净额", Year: year, Value: ocf},
			},
			Steps: []CalcStep{
				{Desc: "计算分红占比", Expr: fmt.Sprintf("%.0f / |%.0f| × 100%%", dividend, ocf), Value: ratio},
			},
			Result: ratio,
		}
		result.Traces = append(result.Traces, trace)
	}
	if allZero {
		result.Conclusion = "各年度现金流量表中均未找到分红支出数据，可能因数据源未提供该字段（建议重新下载财报或切换数据源）。"
	} else {
		result.Conclusion = "分红现金支出占经营活动现金流比率在20%~70%之间，说明分红长期可持续性较强。"
	}
	return result
}
