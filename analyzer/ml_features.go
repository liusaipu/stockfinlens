package analyzer

import (
	"math"
)

// MLKlineData 机器学习模块使用的K线数据（避免循环导入）
type MLKlineData struct {
	Time   string
	Open   float64
	Close  float64
	Low    float64
	High   float64
	Volume float64
	Amount float64
}

// BuildMLEngineBInput 从 FinancialData 提取最近 8 个年度的财务特征序列
// 返回固定8个时间步的序列，如果数据不足8年，用零值填充
func BuildMLEngineBInput(data *FinancialData) [][]float64 {
	if data == nil || len(data.Years) == 0 {
		return nil
	}
	records := []map[string]float64{}
	for _, year := range data.Years {
		rec := extractFinancialFeatures(data, year)
		if rec != nil {
			records = append(records, rec)
		}
	}
	if len(records) == 0 {
		return nil
	}

	// 确保返回8个时间步
	seq := make([][]float64, 0, 8)
	for i := 0; i < 8; i++ {
		if i < len(records) {
			r := records[i]
			seq = append(seq, []float64{
				r["roe"],
				r["gross_margin"],
				r["debt_ratio"],
				r["cash_ratio"],
				r["turnover"],
				r["mscore_proxy"],
				r["revenue"] / 1e8,
				r["net_profit"] / 1e8,
			})
		} else {
			// 用零值填充
			seq = append(seq, []float64{0, 0, 0, 0, 0, 0, 0, 0})
		}
	}
	return seq
}

func extractFinancialFeatures(data *FinancialData, year string) map[string]float64 {
	// 使用 GetValueOrZero 正确访问 {科目: {年份: 值}} 结构
	revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
	cost := data.GetValueOrZero(data.IncomeStatement, "营业成本", year)
	netProfit := data.GetValueOrZero(data.IncomeStatement, "净利润", year)
	totalAssets := data.GetValueOrZero(data.BalanceSheet, "总资产", year)
	totalLiabilities := data.GetValueOrZero(data.BalanceSheet, "总负债", year)
	equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", year)
	if equity == 0 {
		equity = totalAssets - totalLiabilities
	}
	if equity == 0 {
		equity = 1e-6
	}

	grossMargin := 0.0
	if revenue != 0 {
		grossMargin = (revenue - cost) / revenue
	}
	roe := netProfit / equity
	debtRatio := 0.0
	if totalAssets != 0 {
		debtRatio = totalLiabilities / totalAssets
	}
	opCash := data.GetValueOrZero(data.CashFlow, "经营活动产生的现金流量净额", year)
	cashRatio := 0.0
	if netProfit != 0 {
		cashRatio = opCash / netProfit
	}

	inventory := data.GetValueOrZero(data.BalanceSheet, "存货", year)
	receivables := data.GetValueOrZero(data.BalanceSheet, "应收账款", year)
	turnover := 0.0
	if inventory+receivables > 0 {
		turnover = revenue / (inventory + receivables)
	}

	accruals := netProfit - opCash
	mscoreProxy := 0.0
	if totalAssets != 0 {
		mscoreProxy = -accruals / totalAssets
	}

	return map[string]float64{
		"roe":          roe,
		"gross_margin": grossMargin,
		"debt_ratio":   debtRatio,
		"cash_ratio":   cashRatio,
		"turnover":     turnover,
		"mscore_proxy": mscoreProxy,
		"revenue":      revenue,
		"net_profit":   netProfit,
	}
}

func getFloat(m map[string]float64, key string) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return 0.0
}

// BuildMLEngineAInputFromKlines 用日K线+舆情数据构造引擎A的简化输入
// text_seq: [16, 32], price_seq: [16, 24]
func BuildMLEngineAInputFromKlines(klines []MLKlineData, sentiment *SentimentData) ([][]float64, [][]float64) {
	if len(klines) < 16 {
		return nil, nil
	}
	// 取最近 16 根日K作为 16 个时间桶（简化版）
	recent := klines[len(klines)-16:]

	priceSeq := make([][]float64, 16)
	for i, k := range recent {
		returns := 0.0
		if i > 0 && recent[i-1].Close != 0 {
			returns = (k.Close - recent[i-1].Close) / recent[i-1].Close
		}
		volMa := 0.0
		if i >= 5 {
			sum := 0.0
			for j := i - 4; j <= i; j++ {
				sum += recent[j].Volume
			}
			volMa = sum / 5.0
		}
		volRatio := 1.0
		if volMa > 0 {
			volRatio = k.Volume / volMa
		}
		amplitude := 0.0
		if k.Open != 0 {
			amplitude = (k.High - k.Low) / k.Open
		}
		// 构建 24 维价格特征（后面用0填充）
		feat := make([]float64, 24)
		feat[0] = returns
		feat[1] = volRatio
		feat[2] = amplitude
		feat[3] = k.Close
		feat[4] = k.Volume / 1e6
		priceSeq[i] = feat
	}

	textSeq := make([][]float64, 16)
	for i := range textSeq {
		feat := make([]float64, 32)
		if sentiment != nil && sentiment.HasData {
			feat[0] = sentiment.Score / 100.0              // 情感分数归一化
			feat[1] = float64(sentiment.HeatIndex) / 100.0 // 热度
			feat[2] = float64(len(sentiment.PositiveWords)) / 10.0
			feat[3] = float64(len(sentiment.NegativeWords)) / 10.0
		}
		// 其余维度保持 0（模型训练时允许部分缺失）
		textSeq[i] = feat
	}
	return textSeq, priceSeq
}

// zscore normalize with zscore simple version
func zscore(values []float64) []float64 {
	if len(values) == 0 {
		return values
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	std := 0.0
	for _, v := range values {
		d := v - mean
		std += d * d
	}
	std = math.Sqrt(std / float64(len(values)))
	if std == 0 {
		std = 1e-6
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = (v - mean) / std
	}
	return out
}

// BuildMLEngineDInput 构建 Engine-D 25维风险特征向量
// 特征顺序：财务14维 + 市场6维 + 非财务5维
// quote 为实时行情数据，用于填充市场指标；为 nil 时使用默认值
//
// 注意：mscore / ascore 由 step8RiskAnalysis 计算并复用真实 Beneish M-Score 与综合 A-Score，
// gm_risk 改为单边（仅在毛利率 YoY 下滑时为正，单位百分点小数）。
// 这些都与 ml_models/engine_d_risk/train.py 的特征语义保持一致。
func BuildMLEngineDInput(data *FinancialData, quote *QuoteData) []float64 {
	if data == nil || len(data.Years) == 0 {
		return nil
	}

	// 使用最新年份数据
	year := data.Years[0]

	// 复用 step8 的真实 A-Score / M-Score / 现金流偏离度 / GMRisk / ARRisk
	step8 := step8RiskAnalysis(data)
	var realAScore, realMScore, realCashDev float64
	var step8GMRiskPctDecline float64 // step8 的 0-100 GMRisk，需要换算回百分点跌幅
	if yd, ok := step8.YearlyData[year]; ok {
		if v, ok := yd["AScore"].(float64); ok {
			realAScore = v
		}
		if v, ok := yd["MScore"].(float64); ok {
			realMScore = v
		}
		if v, ok := yd["CashDev"].(float64); ok {
			realCashDev = v
		}
		if v, ok := yd["GMRisk"].(float64); ok {
			step8GMRiskPctDecline = v
		}
	}

	// 基础数据提取（使用 GetValueOrZero 正确访问 {科目: {年份: 值}} 结构）
	revenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", year)
	cost := data.GetValueOrZero(data.IncomeStatement, "营业成本", year)
	netProfit := data.GetValueOrZero(data.IncomeStatement, "净利润", year)
	totalAssets := data.GetValueOrZero(data.BalanceSheet, "总资产", year)
	totalLiabilities := data.GetValueOrZero(data.BalanceSheet, "总负债", year)
	equity := data.GetValueOrZero(data.BalanceSheet, "所有者权益合计", year)
	if equity == 0 {
		equity = totalAssets - totalLiabilities
	}
	if equity == 0 {
		equity = 1e-6
	}

	opCash := data.GetValueOrZero(data.CashFlow, "经营活动产生的现金流量净额", year)
	inventory := data.GetValueOrZero(data.BalanceSheet, "存货", year)
	receivables := data.GetValueOrZero(data.BalanceSheet, "应收账款", year)
	goodwill := data.GetValueOrZero(data.BalanceSheet, "商誉", year)

	// 财务指标 (14维)
	// 1. M-Score：直接用 step8 的真实 Beneish M-Score（缺数据时 fallback 到代理）
	mscore := realMScore
	if mscore == 0 {
		// fallback：step8 因年份不足等原因没算出来时，使用应计项目代理
		accruals := netProfit - opCash
		mscore = -safeDivide(accruals, totalAssets)*5 - 2.5 // 平移到 Beneish 区间
	}

	// 2. Z-Score 简化版
	workingCapital := data.GetValueOrZero(data.BalanceSheet, "流动资产合计", year) -
		data.GetValueOrZero(data.BalanceSheet, "流动负债合计", year)
	x1 := safeDivide(workingCapital, totalAssets)
	retainedEarnings := data.GetValueOrZero(data.BalanceSheet, "未分配利润", year)
	x2 := safeDivide(retainedEarnings, totalAssets)
	ebit := data.GetValueOrZero(data.IncomeStatement, "营业利润", year) +
		data.GetValueOrZero(data.IncomeStatement, "财务费用", year)
	x3 := safeDivide(ebit, totalAssets)
	x4 := safeDivide(equity, totalLiabilities)
	if totalLiabilities == 0 {
		x4 = 1
	}
	x5 := safeDivide(revenue, totalAssets)
	zscore := 1.2*x1 + 1.4*x2 + 3.3*x3 + 0.6*x4 + 1.0*x5

	// 3. 现金流偏离度：优先复用 step8；缺失时用 |accruals|/totalAssets
	cashDeviation := realCashDev
	if cashDeviation == 0 {
		accruals := netProfit - opCash
		cashDeviation = safeDivide(math.Abs(accruals), totalAssets)
	}

	// 4. 应收账款异常度（应收/营收，0-1 区间，匹配训练侧分布）
	arRisk := safeDivide(receivables, revenue)

	// 5. 毛利率风险（单边）：仅在 YoY 毛利率下滑时为正，否则为 0
	// 单位为小数跌幅（如 0.05 = 5 个百分点）。step8.GMRisk = 跌幅小数 × 2000，反推即可。
	grossMargin := safeDivide(revenue-cost, revenue)
	gmRisk := step8GMRiskPctDecline / 2000.0
	if gmRisk < 0 {
		gmRisk = 0
	}

	// 6. A-Score：直接用 step8 的真实综合评分（0-100，与训练数据 ascore 同尺度）
	ascore := realAScore

	// 7. ROE
	roe := safeDivide(netProfit, equity)

	// 8. 营收增长率
	revenueGrowth := 0.0
	if len(data.Years) > 1 {
		prevYear := data.Years[1]
		prevRevenue := data.GetValueOrZero(data.IncomeStatement, "营业收入", prevYear)
		if prevRevenue > 0 {
			revenueGrowth = (revenue - prevRevenue) / prevRevenue
		}
	}

	// 9. 资产负债率
	debtRatio := safeDivide(totalLiabilities, totalAssets)

	// 10. 净利润现金含量
	ncfToProfit := 0.0
	if netProfit > 0 {
		ncfToProfit = opCash / netProfit
	}

	// 11. 商誉/净资产
	goodwillToEquity := safeDivide(goodwill, equity)

	// 12. 存货周转率
	cogs := cost
	avgInventory := inventory
	inventoryTurnover := safeDivide(cogs, avgInventory)

	// 13. 应收账款周转率
	avgReceivables := receivables
	receivableTurnover := safeDivide(revenue, avgReceivables)

	// 市场指标 (6维) - 优先从行情数据获取，缺失时使用默认值
	peTTM := 25.0
	pb := 2.5
	marketCap := totalAssets / 1e8 // 简化估算
	turnover20d := 0.03
	volatility60d := 0.3
	maxDrawdown1y := -0.15
	if quote != nil {
		if quote.PE > 0 {
			peTTM = quote.PE
		}
		if quote.PB > 0 {
			pb = quote.PB
		}
		if quote.MarketCap > 0 {
			marketCap = quote.MarketCap / 1e8
		}
		if quote.TurnoverRate > 0 {
			turnover20d = quote.TurnoverRate
		}
	}

	// 非财务指标 (5维) - 使用默认值
	pledgeRatio := 0.15
	regulatoryInquiry := 0.0
	shareholderReduction := 0.0
	auditorSwitch := 0.0
	cfoChange := 0.0

	// 构建25维特征向量
	features := []float64{
		// 财务指标 (14)
		mscore, zscore, cashDeviation, arRisk, gmRisk, ascore,
		roe, grossMargin, revenueGrowth, debtRatio, ncfToProfit,
		goodwillToEquity, inventoryTurnover, receivableTurnover,
		// 市场指标 (6)
		peTTM, pb, marketCap, turnover20d, volatility60d, maxDrawdown1y,
		// 非财务指标 (5)
		pledgeRatio, regulatoryInquiry, shareholderReduction,
		auditorSwitch, cfoChange,
	}

	return features
}

// safeDivide 安全除法
func safeDivide(a, b float64) float64 {
	if b == 0 || math.IsNaN(b) || math.IsInf(b, 0) {
		return 0
	}
	result := a / b
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}
