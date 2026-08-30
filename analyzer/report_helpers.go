package analyzer

import (
	"fmt"
	"math"
	"strings"
)

// formatSignedYi 把单位为元金额格式化为带符号的"亿"（如 +1.23亿 / -0.45亿）。
// 用于资金流向等需要明确正负方向的场景，避免负值丢失负号。
func formatSignedYi(v float64) string {
	yi := v / 1e8
	switch {
	case yi >= 0.005:
		return fmt.Sprintf("+%.2f亿", yi)
	case yi <= -0.005:
		return fmt.Sprintf("%.2f亿", yi) // 负数自带负号
	default:
		return "0.00亿"
	}
}

func getDiffEmoji(diff float64) string {
	if diff >= 5 {
		return "🟢 优于行业"
	} else if diff <= -5 {
		return "🔴 低于行业"
	}
	return "➡️ 与行业接近"
}

func policySignalText(level int) string {
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	return fmt.Sprintf("signal:%d", level)
}

// ========== 模块6: RIM估值（基于多期预测） ==========
func rimSourceDesc(rim *RIMData, quote *QuoteData) string {
	if rim == nil || rim.Params.BPS0 <= 0 {
		return "待计算"
	}
	if quote != nil && quote.PB > 0 && quote.CurrentPrice > 0 {
		calc := quote.CurrentPrice / quote.PB
		if math.Abs(calc-rim.Params.BPS0) < 0.5 {
			return "股价/PB推算"
		}
	}
	return "财报股东权益/总股本推算"
}

// ========== 模块8: 技术面分析 ==========
func mlDirectionCN(label string) string {
	switch label {
	case "up":
		return "上涨 ↑"
	case "down":
		return "下跌 ↓"
	case "flat":
		return "持平 →"
	default:
		return label
	}
}

func formatTradeLevels(quote *QuoteData, rim *RIMData, technical *TechnicalData, ml *MLPredictionData, ascore float64) (entry, stop, target string) {
	if quote == nil || quote.CurrentPrice <= 0 {
		return "待接入实时行情后计算", "待接入实时行情后计算", "待接入RIM估值与行情后计算"
	}
	price := quote.CurrentPrice
	hasRIM := rim != nil && rim.HasData && rim.Result != nil && rim.Result.Baseline.Value > 0

	// 动态系数调整
	stopTighten := 1.0   // 止损收紧系数（越小越紧）
	targetFactor := 1.0  // 目标位折扣
	entryDiscount := 1.0 // 入场区间下限折扣（越小越保守）

	if ascore >= 70 {
		stopTighten = 0.92
		entryDiscount = 0.95
	} else if ascore >= 60 {
		stopTighten = 0.95
		entryDiscount = 0.97
	}
	if technical != nil && technical.Score < 40 {
		targetFactor = math.Max(0, targetFactor-0.05)
		entryDiscount = math.Max(0, entryDiscount-0.03)
	}
	if ml != nil && ml.Summary != nil && ml.Summary.HasData {
		if ml.Summary.RangeLow < 0 {
			// ML 预测包含下跌
			targetFactor = math.Max(0, targetFactor-0.05)
			entryDiscount = math.Max(0, entryDiscount-0.03)
		}
	}

	if hasRIM {
		baseline := rim.Result.Baseline.Value
		pessimistic := rim.Result.Pessimistic.Value
		optimistic := rim.Result.Optimistic.Value

		// 目标位：乐观情景价值 × 动态系数
		targetVal := optimistic * targetFactor
		target = fmt.Sprintf("%.2f元 (乐观情景", targetVal)
		if targetFactor < 1.0 {
			target += fmt.Sprintf("×%.0f%%", targetFactor*100)
		}
		target += ")"

		// 入场区间（先计算）
		var low, high float64
		switch {
		case price <= baseline:
			low = price * 0.97 * entryDiscount
			high = price * 1.02
			entry = fmt.Sprintf("%.2f ~ %.2f元 (现价附近可建仓)", low, high)
		case price <= optimistic:
			low = baseline * 0.95 * entryDiscount
			high = baseline
			entry = fmt.Sprintf("%.2f ~ %.2f元 (等待回调至基准价值)", low, high)
		default:
			low = baseline * 0.90 * entryDiscount
			high = baseline * 0.95
			entry = fmt.Sprintf("%.2f ~ %.2f元 (高估/观望)", low, high)
		}

		// 止损位：悲观情景的85%、当前价的88%×收紧系数、入场区间低位的95% 三者取较高者
		// 但必须低于入场区间低位，给买入后留足波动空间
		stopPrice := math.Max(pessimistic*0.85, price*0.88*stopTighten)
		// 确保止损位低于入场区间低位（预留至少5%的缓冲）
		maxStopPrice := low * 0.95
		if stopPrice >= low {
			stopPrice = maxStopPrice
		}
		stop = fmt.Sprintf("%.2f元 (-%.1f%%)", stopPrice, (price-stopPrice)/price*100)
	} else {
		// 无 RIM 时的简易估算
		target = "待接入RIM估值后计算"
		low := price * 0.98 * entryDiscount
		high := price * 1.02
		entry = fmt.Sprintf("%.2f ~ %.2f元 (现价±2%%试探)", low, high)

		stopPrice := price * 0.90 * stopTighten
		// 确保止损位低于入场区间低位（预留至少5%的缓冲）
		maxStopPrice := low * 0.95
		if stopPrice >= low {
			stopPrice = maxStopPrice
		}
		stop = fmt.Sprintf("%.2f元 (-%.1f%%)", stopPrice, (price-stopPrice)/price*100)
	}
	return
}

// ========== 模块15: 结论与附录 ==========
func sentimentSummary(sentiment *SentimentData) string {
	if sentiment == nil || !sentiment.HasData {
		return "暂无数据"
	}
	desc := "中性"
	if sentiment.Score > 0.3 {
		desc = "偏多"
	} else if sentiment.Score < -0.3 {
		desc = "偏空"
	}
	return fmt.Sprintf("%s（热度 %d 条）", desc, sentiment.HeatIndex)
}

func scoreToStars(score float64) string {
	switch {
	case score >= 90:
		return "⭐⭐⭐⭐⭐"
	case score >= 80:
		return "⭐⭐⭐⭐"
	case score >= 70:
		return "⭐⭐⭐"
	case score >= 60:
		return "⭐⭐"
	default:
		return "⭐"
	}
}

func gradeComment(score float64) string {
	switch {
	case score >= 90:
		return "财务结构非常健康，各项指标优秀"
	case score >= 80:
		return "财务状况良好，少数维度有改善空间"
	case score >= 70:
		return "财务整体中等，需关注部分风险点"
	case score >= 60:
		return "财务存在明显短板，建议深入核查"
	default:
		return "财务风险较高，建议谨慎对待"
	}
}

func investmentGrade(score float64) string {
	switch {
	case score >= 90:
		return "强烈推荐"
	case score >= 80:
		return "推荐"
	case score >= 70:
		return "谨慎推荐"
	case score >= 60:
		return "观望"
	default:
		return "回避"
	}
}

func positionAdvice(score float64) string {
	switch {
	case score >= 90:
		return "8-12%"
	case score >= 80:
		return "5-8%"
	case score >= 70:
		return "3-5%"
	case score >= 60:
		return "1-3%或观望"
	default:
		return "0-1%或回避"
	}
}

func strategyAdvice(score float64) string {
	switch {
	case score >= 90:
		return "积极配置，长期持有"
	case score >= 80:
		return "逢低分批建仓"
	case score >= 70:
		return "观望/逢低轻仓试探"
	case score >= 60:
		return "观望为主，等待基本面改善"
	default:
		return "回避，等待风险释放"
	}
}

func oneSentenceAdvice(symbol string, score float64, steps []StepResult, year string) string {
	var parts []string
	if score >= 80 {
		parts = append(parts, "财务基本面整体健康")
	} else if score >= 70 {
		parts = append(parts, "财务基本面尚可，但存在部分短板")
	} else {
		parts = append(parts, "财务基本面偏弱，风险点较多")
	}

	roe := getStepValue(steps, 16, year, "roe")
	if roe >= 15 {
		parts = append(parts, fmt.Sprintf("ROE %.1f%%显示公司具备较好的资本回报能力", roe))
	} else if roe > 0 {
		parts = append(parts, fmt.Sprintf("ROE %.1f%%低于理想水平，资本回报能力有待提升", roe))
	}

	debtRatio := getStepValue(steps, 3, year, "debtRatio")
	if debtRatio <= 40 {
		parts = append(parts, "负债率低，财务结构稳健")
	} else if debtRatio > 60 {
		parts = append(parts, "负债率偏高，需关注偿债压力")
	}

	ascore := getStepValue(steps, 8, year, "AScore")
	if ascore >= 60 {
		parts = append(parts, fmt.Sprintf("A-Score %.1f 提示需警惕财报操纵或偿债风险", ascore))
	}

	parts = append(parts, fmt.Sprintf("建议%s", strategyAdvice(score)))
	return strings.Join(parts, "。") + "。"
}

func growthScore(steps []StepResult, year string) float64 {
	g := getStepValue(steps, 9, year, "growthRate")
	if g >= 20 {
		return 90
	} else if g >= 10 {
		return 80
	} else if g >= 0 {
		return 60
	} else if g >= -10 {
		return 40
	}
	return 20
}

func growthComment(steps []StepResult, year string) string {
	g := getStepValue(steps, 9, year, "growthRate")
	if g >= 20 {
		return "高速增长"
	} else if g >= 10 {
		return "稳健增长"
	} else if g >= 0 {
		return "增长放缓"
	} else if g >= -10 {
		return "轻微下滑"
	}
	return "显著下滑"
}

func profitScore(steps []StepResult, year string) float64 {
	roe := getStepValue(steps, 16, year, "roe")
	gm := getStepValue(steps, 10, year, "grossMargin")
	cp := getStepValue(steps, 14, year, "coreProfitMargin")
	score := 50.0
	if roe >= 20 {
		score += 25
	} else if roe >= 15 {
		score += 20
	} else if roe >= 10 {
		score += 10
	} else if roe > 0 {
		score += 5
	}
	if gm >= 40 {
		score += 15
	} else if gm >= 30 {
		score += 10
	} else if gm >= 20 {
		score += 5
	}
	if cp >= 15 {
		score += 10
	} else if cp >= 10 {
		score += 5
	}
	return math.Min(100, score)
}

func profitComment(steps []StepResult, year string) string {
	roe := getStepValue(steps, 16, year, "roe")
	gm := getStepValue(steps, 10, year, "grossMargin")
	if roe >= 15 && gm >= 40 {
		return "盈利能力优秀，高毛利+高ROE"
	} else if roe >= 15 {
		return "盈利能力良好，ROE达标但毛利率偏低"
	} else if roe >= 10 {
		return "盈利能力一般，ROE有提升空间"
	} else if roe > 0 {
		return "盈利能力偏弱，ROE低于理想水平"
	}
	return "盈利能力较差，ROE为负或极低"
}

func cashScore(steps []StepResult, year string) float64 {
	cr := getStepValue(steps, 15, year, "cashRatio")
	ocf := getStepValue(steps, 15, year, "operatingCF")
	score := 50.0
	if cr >= 100 {
		score += 30
	} else if cr >= 50 {
		score += 15
	} else if cr > 0 {
		score += 5
	}
	if ocf > 0 {
		score += 20
	}
	return math.Min(100, score)
}

func cashComment(steps []StepResult, year string) string {
	cr := getStepValue(steps, 15, year, "cashRatio")
	ocf := getStepValue(steps, 15, year, "operatingCF")
	if cr >= 100 && ocf > 0 {
		return "现金流质量优秀，经营现金流充沛"
	} else if ocf > 0 {
		return "经营现金流为正，但净利润含金量有提升空间"
	} else if ocf < 0 {
		return "经营现金流为负，现金流压力需关注"
	}
	return "现金流数据不足"
}

func debtScore(steps []StepResult, year string) float64 {
	dr := getStepValue(steps, 3, year, "debtRatio")
	diff := getStepValue(steps, 3, year, "cashDebtDiff")
	score := 50.0
	if dr <= 40 {
		score += 25
	} else if dr <= 60 {
		score += 15
	} else if dr <= 70 {
		score += 5
	}
	if diff >= 0 {
		score += 25
	} else if diff > -1e9 {
		score += 10
	}
	return math.Min(100, score)
}

func debtComment(steps []StepResult, year string) string {
	dr := getStepValue(steps, 3, year, "debtRatio")
	diff := getStepValue(steps, 3, year, "cashDebtDiff")
	if dr <= 40 && diff >= 0 {
		return "偿债能力优秀，负债率低且现金充裕"
	} else if dr <= 60 && diff >= 0 {
		return "偿债能力良好，结构相对安全"
	} else if dr <= 60 {
		return "偿债能力一般，准货币资金未能完全覆盖有息负债"
	} else if dr <= 70 {
		return "偿债压力较大，负债率偏高"
	}
	return "偿债风险高，需密切关注"
}

func valuationScore(quote *QuoteData) float64 {
	if quote == nil {
		return 0
	}
	score := 50.0
	if quote.PE > 0 {
		if quote.PE < 15 {
			score += 25
		} else if quote.PE < 25 {
			score += 15
		} else if quote.PE < 40 {
			score += 5
		} else {
			score -= 15
		}
	} else {
		score -= 15
	}
	if quote.PB > 0 {
		if quote.PB < 2 {
			score += 25
		} else if quote.PB < 3 {
			score += 15
		} else if quote.PB < 5 {
			score += 5
		} else {
			score -= 15
		}
	} else {
		score -= 15
	}
	return math.Max(0, math.Min(100, score))
}

func valuationComment(quote *QuoteData) string {
	if quote == nil || (quote.PE <= 0 && quote.PB <= 0) {
		return "暂无估值数据"
	}
	s := valuationScore(quote)
	if s >= 80 {
		return "估值较低，具备安全边际"
	} else if s >= 60 {
		return "估值处于合理区间"
	} else if s >= 40 {
		return "估值偏高，需关注性价比"
	}
	return "估值过高，注意风险"
}

func reverseScore(steps []StepResult, year string, score *YearScore) float64 {
	s := 85.0
	if score != nil {
		s -= (100 - score.RawScore) * 0.4
	}
	risks := extractRisks(steps, year)
	s -= float64(len(risks)) * 5
	return math.Max(0, math.Min(100, s))
}

func reverseComment(steps []StepResult, year string, score *YearScore) string {
	s := reverseScore(steps, year, score)
	if s >= 75 {
		return "风险可控，负面因素较少"
	} else if s >= 60 {
		return "存在一定风险，需关注短板"
	} else if s >= 40 {
		return "风险点较多，谨慎对待"
	}
	return "风险较高，建议回避"
}

func buffettScore(steps []StepResult, year string, score *YearScore) float64 {
	roe := getStepValue(steps, 16, year, "roe")
	gm := getStepValue(steps, 10, year, "grossMargin")
	cr := getStepValue(steps, 15, year, "cashRatio")
	dr := getStepValue(steps, 3, year, "debtRatio")
	ia := getStepValue(steps, 7, year, "ratio")

	s := 5.0
	if roe >= 15 {
		s += 1.5
	}
	if gm >= 40 {
		s += 1
	}
	if getStepValue(steps, 8, year, "AScore") < 50 {
		s += 1
	}
	if cr >= 100 {
		s += 0.5
	}
	if dr <= 60 {
		s += 0.5
	}
	if ia <= 10 {
		s += 0.5
	}
	return math.Min(10, s)
}

func buffettComment(steps []StepResult, year string, score *YearScore) string {
	s := buffettScore(steps, year, score)
	if s >= 8 {
		return "基本满足投资检查标准"
	} else if s >= 6 {
		return "勉强及格，部分维度待提升"
	} else if s >= 4 {
		return "不满足标准，需等待改善"
	}
	return "明显偏离标准，建议回避"
}

func positiveFactors(steps []StepResult, year string) []string {
	var ps []string
	if g := getStepValue(steps, 9, year, "growthRate"); g >= 10 {
		ps = append(ps, fmt.Sprintf("营收保持 %.2f%% 增长，业务仍在扩张", g))
	}
	if pg := getStepValue(steps, 16, year, "profitGrowth"); pg >= 10 {
		ps = append(ps, fmt.Sprintf("净利润增长 %.2f%%，盈利能力改善", pg))
	}
	if roe := getStepValue(steps, 16, year, "roe"); roe >= 15 {
		ps = append(ps, fmt.Sprintf("ROE %.2f%%，资本回报能力优秀", roe))
	}
	if gm := getStepValue(steps, 10, year, "grossMargin"); gm >= 40 {
		ps = append(ps, fmt.Sprintf("毛利率 %.2f%%，护城河较深", gm))
	}
	if dr := getStepValue(steps, 3, year, "debtRatio"); dr <= 40 {
		ps = append(ps, fmt.Sprintf("资产负债率 %.2f%%，财务结构稳健", dr))
	}
	if cr := getStepValue(steps, 15, year, "cashRatio"); cr >= 100 {
		ps = append(ps, fmt.Sprintf("净利润现金含量 %.2f%%，盈利质量高", cr))
	}
	if ascore := getStepValue(steps, 8, year, "AScore"); ascore < 50 {
		ps = append(ps, fmt.Sprintf("A-Score %.1f，综合财务风险可控", ascore))
	}
	return ps
}

func getStepValue(steps []StepResult, stepNum int, year, key string) float64 {
	for _, s := range steps {
		if s.StepNum != stepNum {
			continue
		}
		yd, ok := s.YearlyData[year]
		if !ok {
			return 0
		}
		return anyToFloat64(yd[key])
	}
	return 0
}

func countCategoryPass(steps []StepResult, stepNums []int, year string) (int, int) {
	pass := 0
	total := 0
	stepMap := make(map[int]bool)
	for _, n := range stepNums {
		stepMap[n] = true
	}
	for _, s := range steps {
		if !stepMap[s.StepNum] {
			continue
		}
		p, ok := s.Pass[year]
		if !ok {
			continue
		}
		total++
		if p {
			pass++
		}
	}
	return pass, total
}

func extractRisks(steps []StepResult, year string) []RiskItem {
	var risks []RiskItem
	for _, s := range steps {
		p, ok := s.Pass[year]
		if !ok || p {
			continue
		}
		switch s.StepNum {
		case 3:
			dr := getStepValue(steps, 3, year, "debtRatio")
			diff := getStepValue(steps, 3, year, "cashDebtDiff")
			if dr > 60 {
				risks = append(risks, RiskItem{"偿债风险", fmt.Sprintf("资产负债率%.1f%%", dr), "🔴 高", "负债率超过60%警戒线"})
			} else if diff < 0 {
				risks = append(risks, RiskItem{"偿债风险", fmt.Sprintf("准货币资金缺口%.1f亿", -diff/1e8), "🟠 中高", "现金未能覆盖有息负债"})
			}
		case 4:
			diff := getStepValue(steps, 4, year, "diff")
			risks = append(risks, RiskItem{"产业链地位", fmt.Sprintf("两头吃差额%.1f亿", diff/1e8), "🟠 中高", "对上下游议价能力偏弱"})
		case 5:
			ratio := getStepValue(steps, 5, year, "ratio")
			if ratio > 20 {
				risks = append(risks, RiskItem{"回款风险", fmt.Sprintf("应收账款占比%.1f%%", ratio), "🔴 高", "回款压力大，销售回款慢"})
			} else {
				risks = append(risks, RiskItem{"回款风险", fmt.Sprintf("应收账款占比%.1f%%", ratio), "🟠 中高", "应收占比偏高"})
			}
		case 6:
			ratio := getStepValue(steps, 6, year, "ratio")
			if ratio > 40 {
				risks = append(risks, RiskItem{"资产结构", fmt.Sprintf("固定资产占比%.1f%%", ratio), "🟠 中高", "重资产模式，维持成本高"})
			}
		case 7:
			ratio := getStepValue(steps, 7, year, "ratio")
			risks = append(risks, RiskItem{"主业专注", fmt.Sprintf("投资类资产占比%.1f%%", ratio), "🟠 中高", "主业专注度不足"})
		case 8:
			ascore := getStepValue(steps, 8, year, "AScore")
			if ascore >= 60 {
				severity := "🟠 中高"
				if ascore >= 70 {
					severity = "🔴 高"
				}
				risks = append(risks, RiskItem{"财务造假/偿债风险", fmt.Sprintf("A-Score %.1f", ascore), severity, "存在财报操纵或偿债压力嫌疑，建议深入核查"})
			}
		case 9:
			g := getStepValue(steps, 9, year, "growthRate")
			if g < 0 {
				risks = append(risks, RiskItem{"成长风险", fmt.Sprintf("营收增长%.1f%%", g), "🔴 高", "营业收入出现负增长"})
			} else if g < 10 {
				risks = append(risks, RiskItem{"成长风险", fmt.Sprintf("营收增长%.1f%%", g), "🟠 中高", "营收增长未达10%理想水平"})
			}
		case 10:
			gm := getStepValue(steps, 10, year, "grossMargin")
			if gm < 20 {
				risks = append(risks, RiskItem{"盈利质量", fmt.Sprintf("毛利率%.1f%%", gm), "🔴 高", "毛利率偏低，产品竞争力弱"})
			} else if gm < 40 {
				risks = append(risks, RiskItem{"盈利质量", fmt.Sprintf("毛利率%.1f%%", gm), "🟠 中高", "毛利率未达高毛利标准"})
			}
		case 16:
			roe := getStepValue(steps, 16, year, "roe")
			if roe < 15 {
				risks = append(risks, RiskItem{"资本回报", fmt.Sprintf("ROE %.1f%%", roe), "🟠 中高", "ROE低于15%理想水平"})
			}
		}
	}
	return risks
}

func writeMetricRow(b *strings.Builder, name string, latestVal, prevVal float64, unit string, div float64) {
	lv, pv := latestVal, prevVal
	if div != 1 {
		lv /= div
		pv /= div
	}
	change := "-"
	if pv != 0 {
		pct := (latestVal - prevVal) / math.Abs(prevVal) * 100
		emoji := "➡️"
		if pct > 0 {
			emoji = "📈"
		} else if pct < 0 {
			emoji = "📉"
		}
		change = fmt.Sprintf("%s %.2f%%", emoji, pct)
	}
	assess := "🟡 持平"
	if latestVal > prevVal {
		assess = "🟢 上升"
	} else if latestVal < prevVal {
		assess = "🔴 下降"
	}
	if name == "资产负债率" || name == "期间费用率/毛利率" {
		if latestVal < prevVal {
			assess = "🟢 优化"
		} else if latestVal > prevVal {
			assess = "🔴 恶化"
		}
	}
	if prevVal == 0 {
		assess = "-"
		change = "-"
	}
	b.WriteString(fmt.Sprintf("| **%s** | %.2f%s | %.2f%s | %s | %s |\n", name, lv, unit, pv, unit, change, assess))
}

func fmtVal(v float64, unit string) string {
	if v == 0 {
		return "-"
	}
	if unit == "%" {
		return fmt.Sprintf("%.2f%%", v)
	}
	return fmt.Sprintf("%.3f", v)
}

// infoTooltipHTML 已废弃，不再在 Markdown 中输出 HTML（前端交互组件）
func infoTooltipHTML(title, body string) string {
	return ""
}

func infoIcon(title, body string) string {
	return ""
}

// traceTrigger 溯源问号图标（前端交互用，导出 Markdown 时不需要）
func traceTrigger(stepNums ...int) string {
	return ""
}

func yoyFmt(cur, prev float64) string {
	if prev == 0 {
		return "-"
	}
	pct := (cur - prev) / math.Abs(prev) * 100
	return fmt.Sprintf("%.2f%%", pct)
}

func yoyEmoji(cur, prev float64) string {
	if prev == 0 {
		return "-"
	}
	pct := (cur - prev) / math.Abs(prev) * 100
	if pct > 0 {
		return "🟢 增长"
	} else if pct < 0 {
		return "🔴 下降"
	}
	return "➡️ 持平"
}

func roeEmoji(v float64) string {
	if v >= 15 {
		return "🟢 优秀"
	} else if v >= 10 {
		return "🟡 一般"
	} else if v > 0 {
		return "🔴 偏弱"
	}
	return "🔴 极差"
}

func gmEmoji(v float64) string {
	if v >= 40 {
		return "🟢 高毛利"
	} else if v >= 30 {
		return "🟡 中等"
	} else if v >= 20 {
		return "🔴 偏低"
	}
	return "🔴 过低"
}

func drEmoji(v float64) string {
	if v <= 40 {
		return "🟢 安全"
	} else if v <= 60 {
		return "🟡 适中"
	} else if v <= 70 {
		return "🔴 偏高"
	}
	return "🔴 危险"
}

func asEmoji(v float64) string {
	if v < 50 {
		return "🟢 安全"
	} else if v < 60 {
		return "🟡 关注"
	}
	return "🔴 风险"
}

func moatComment(gm, roe float64) string {
	if gm >= 40 && roe >= 15 {
		return "高毛利+高ROE，护城河较深"
	} else if gm >= 30 {
		return "毛利率尚可，竞争壁垒中等"
	}
	return "毛利率偏低，护城河待验证"
}

func auditCommentAScore(ascore float64) string {
	if ascore < 50 {
		return "，审计质量可信"
	} else if ascore < 60 {
		return "，需关注"
	}
	return "，风险较高"
}

func mapScore(val, good, bad, max float64) float64 {
	if val >= good {
		return max
	}
	if val <= bad {
		return 1
	}
	return 1 + (max-1)*(val-bad)/(good-bad)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func turnoverAssessment(tr float64) string {
	if tr >= 7 {
		return "非常活跃"
	}
	if tr >= 3 {
		return "活跃"
	}
	if tr >= 1 {
		return "正常"
	}
	return "低迷"
}

func volumeRatioAssessment(vr float64) string {
	if vr >= 2 {
		return "放量"
	}
	if vr >= 0.8 {
		return "正常"
	}
	return "缩量"
}

func gapDirection(gap float64) string {
	if gap > 0 {
		return "高开运行"
	}
	return "低开运行"
}

func writeComparableTrendComment(b *strings.Builder, steps []StepResult, comp *ComparableAnalysis) {
	if len(comp.CommonYears) < 2 {
		return
	}
	latest := comp.CommonYears[0]
	oldest := comp.CommonYears[len(comp.CommonYears)-1]

	latestAvg := comp.YearlyTrends[0].Average
	oldestAvg := comp.YearlyTrends[len(comp.YearlyTrends)-1].Average

	latestROE := getStepValue(steps, 16, latest, "roe")
	oldestROE := getStepValue(steps, 16, oldest, "roe")
	roeGap := latestROE - latestAvg.ROE

	if latestROE > oldestROE && latestAvg.ROE < oldestAvg.ROE {
		b.WriteString("- **ROE 走势优异**：公司 ROE 呈上升趋势，而可比均值在下降，竞争优势在扩大。\n")
	} else if latestROE < oldestROE && latestAvg.ROE > oldestAvg.ROE {
		b.WriteString("- **ROE 走势承压**：公司 ROE 下滑，而可比均值在提升，相对竞争力在减弱。\n")
	} else if latestROE > oldestROE {
		b.WriteString("- **ROE 持续改善**：公司与可比均值同步或独立改善。\n")
	} else if latestROE < oldestROE {
		b.WriteString("- **ROE 有所回落**：公司 ROE 较历史高点下降，需关注盈利能力变化。\n")
	}

	if roeGap >= 5 {
		b.WriteString(fmt.Sprintf("- **ROE 领先可比均值 %.2f 个百分点**，资本回报能力在可比公司中处于优势地位。\n", roeGap))
	} else if roeGap <= -5 {
		b.WriteString(fmt.Sprintf("- **ROE 落后可比均值 %.2f 个百分点**，资本回报能力相对偏弱。\n", -roeGap))
	}

	latestGM := getStepValue(steps, 10, latest, "grossMargin")
	oldestGM := getStepValue(steps, 10, oldest, "grossMargin")
	if latestGM > oldestGM+3 {
		b.WriteString("- **毛利率持续提升**：定价权或成本控制能力在改善。\n")
	} else if latestGM < oldestGM-3 {
		b.WriteString("- **毛利率有所下滑**：产品竞争力或行业竞争格局可能趋紧。\n")
	}

	latestDR := getStepValue(steps, 3, latest, "debtRatio")
	oldestDR := getStepValue(steps, 3, oldest, "debtRatio")
	if latestDR < oldestDR-5 {
		b.WriteString("- **负债率明显下降**：财务结构在优化，偿债安全性提升。\n")
	} else if latestDR > oldestDR+5 {
		b.WriteString("- **负债率明显上升**：杠杆扩张较快，需关注偿债压力。\n")
	}

	latestCR := getStepValue(steps, 15, latest, "cashRatio")
	oldestCR := getStepValue(steps, 15, oldest, "cashRatio")
	if latestCR > oldestCR+10 {
		b.WriteString("- **现金流质量改善**：盈利含金量在提升。\n")
	} else if latestCR < oldestCR-10 {
		b.WriteString("- **现金流质量下滑**：净利润中的现金比例在下降。\n")
	}
}

func ascoreComment(v float64) string {
	if v >= 70 {
		return "综合财务风险较高，建议深入核查"
	}
	if v >= 60 {
		return "综合财务风险中等，需保持关注"
	}
	if v >= 40 {
		return "综合财务风险可控"
	}
	return "综合财务风险低，基本面相对稳健"
}

func ascoreBadge(v float64) string {
	if v >= 70 {
		return "🔴 高风险（A-Score ≥ 70）"
	}
	if v >= 60 {
		return "🟡 中风险（A-Score 60-70）"
	}
	if v >= 40 {
		return "🟢 低风险（A-Score 40-60）"
	}
	return "🟢 安全（A-Score < 40）"
}

// scoreBandPoint 固定档位锚点：threshold 是指标原值（如 ROE=15 表示 15%），score 是 0-100 评分
type scoreBandPoint struct {
	threshold float64
	score     float64
}

// scoreByBand 对单指标做分段线性插值打分。
// points 须按 threshold 升序；value 落在两个相邻点之间时按线性插值，
// 落在首/末点之外时返回首/末点的 score（即上下封顶）。
func scoreByBand(value float64, points []scoreBandPoint) float64 {
	if len(points) == 0 {
		return 50
	}
	if value <= points[0].threshold {
		return points[0].score
	}
	if value >= points[len(points)-1].threshold {
		return points[len(points)-1].score
	}
	for i := 1; i < len(points); i++ {
		if value <= points[i].threshold {
			lo := points[i-1]
			hi := points[i]
			t := (value - lo.threshold) / (hi.threshold - lo.threshold)
			return lo.score + t*(hi.score-lo.score)
		}
	}
	return points[len(points)-1].score
}

// 各指标的固定档位锚点（A 股通用，不区分行业）。
// 锚点取值与 ExtractHighlightsAndRisks / ascoreBadge 保持一致，让"得分"和"亮点/风险"语义对得上。
var (
	// ROE 正向（≥15% 优秀）
	bandROE = []scoreBandPoint{{-100, 0}, {0, 20}, {5, 40}, {10, 60}, {15, 80}, {20, 95}, {30, 100}}
	// 毛利率 正向（≥40% 高毛利）
	bandGM = []scoreBandPoint{{0, 0}, {10, 30}, {20, 50}, {30, 70}, {40, 85}, {60, 100}}
	// 营收增长 正向（≥10% 稳健，<0 承压）
	bandGrowth = []scoreBandPoint{{-30, 0}, {-10, 20}, {0, 40}, {10, 65}, {20, 85}, {40, 100}}
	// 负债率 反向（≤40% 稳健，>60% 偏大）
	bandDebt = []scoreBandPoint{{0, 100}, {30, 95}, {40, 85}, {50, 70}, {60, 50}, {70, 30}, {80, 10}, {95, 0}}
	// 现金含量（经营现金流/净利润，%）正向
	bandCash = []scoreBandPoint{{-50, 0}, {0, 20}, {50, 40}, {100, 70}, {150, 90}, {200, 100}}
	// A-Score 反向（<40 安全、<60 低风险、<70 中风险、≥70 高风险）
	bandAScore = []scoreBandPoint{{0, 100}, {30, 90}, {40, 80}, {50, 65}, {60, 50}, {70, 30}, {80, 10}, {90, 0}}
)

func clampActivity(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// MedianActivityScore 计算可比公司列表中有效活跃度的中位数，无有效样本时返回 50。
func MedianActivityScore(list []*ComparableMetrics) float64 {
	var vals []float64
	for _, m := range list {
		if m.ActivityScore >= 0 {
			vals = append(vals, m.ActivityScore)
		}
	}
	if len(vals) == 0 {
		return 50
	}
	for i := 0; i < len(vals); i++ {
		for j := i + 1; j < len(vals); j++ {
			if vals[i] > vals[j] {
				vals[i], vals[j] = vals[j], vals[i]
			}
		}
	}
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

// CalcComparableScore 综合得分（固定档位法）。
// 每个指标按 bandXxx 锚点做分段线性映射到 0-100，再按权重加权求和：
//
//	ROE 25% / 毛利率 20% / 营收增长 15% / 负债率 10%(反向) / 现金含量 10% /
//	A-Score 10%(反向) / 活跃度 10%
//
// 历史踩坑：原来用 Min-Max within-pool，加一两家可比公司就能把 A、B 的相对排名翻转
// （违反 IIA）。固定档位让得分只依赖该公司自身指标，与可比池组成无关，跨池可比。
// 入参 all 现在用不到，保留是为了调用方不用改签名。
func CalcComparableScore(m *ComparableMetrics, all []*ComparableMetrics, medianActivity float64) float64 {
	_ = all
	act := m.ActivityScore
	if act < 0 {
		act = medianActivity
	}
	return scoreByBand(m.ROE, bandROE)*0.25 +
		scoreByBand(m.GrossMargin, bandGM)*0.20 +
		scoreByBand(m.RevenueGrowth, bandGrowth)*0.15 +
		scoreByBand(m.DebtRatio, bandDebt)*0.10 +
		scoreByBand(m.CashRatio, bandCash)*0.10 +
		scoreByBand(m.AScore, bandAScore)*0.10 +
		clampActivity(act)*0.10
}

// 半年涨幅档位：-100% → 0 分，0% → 60 分，50% → 95 分，100% → 100 分
var bandHalfYearChange = []scoreBandPoint{{-100, 0}, {-50, 20}, {-20, 40}, {0, 60}, {20, 80}, {50, 95}, {100, 100}}

// CalcGroupComparisonScore 组内对比综合得分（含半年涨幅）。
// 权重：ROE 25% / 毛利率 20% / 营收增长 10% / 负债率 10%(反向) / 现金含量 7.5% /
// A-Score 10%(反向) / 活跃度 10% / 半年涨幅 7.5%
func CalcGroupComparisonScore(m *ComparableMetrics, medianActivity, halfYearChange float64) float64 {
	act := m.ActivityScore
	if act < 0 {
		act = medianActivity
	}
	return scoreByBand(m.ROE, bandROE)*0.25 +
		scoreByBand(m.GrossMargin, bandGM)*0.20 +
		scoreByBand(m.RevenueGrowth, bandGrowth)*0.10 +
		scoreByBand(m.DebtRatio, bandDebt)*0.10 +
		scoreByBand(m.CashRatio, bandCash)*0.075 +
		scoreByBand(m.AScore, bandAScore)*0.10 +
		clampActivity(act)*0.10 +
		scoreByBand(halfYearChange, bandHalfYearChange)*0.075
}

// HighlightRisk 亮点与风险摘要
type HighlightRisk struct {
	Highlights []string `json:"highlights"`
	Risks      []string `json:"risks"`
}

// ExtractHighlightsAndRisks 从分析步骤中提取亮点与风险摘要
func ExtractHighlightsAndRisks(steps []StepResult, years []string) HighlightRisk {
	if len(years) == 0 {
		return HighlightRisk{Highlights: []string{}, Risks: []string{}}
	}
	latest := years[0]

	roe := getStepValue(steps, 16, latest, "roe")
	gm := getStepValue(steps, 10, latest, "grossMargin")
	growth := getStepValue(steps, 9, latest, "growthRate")
	pg := getStepValue(steps, 16, latest, "profitGrowth")
	ascore := getStepValue(steps, 8, latest, "AScore")
	dr := getStepValue(steps, 3, latest, "debtRatio")
	cr := getStepValue(steps, 15, latest, "cashRatio")

	var highlights, risks []string

	if roe >= 15 {
		highlights = append(highlights, "ROE 优秀，资本回报能力强")
	} else {
		risks = append(risks, "ROE 低于 15%，资本回报能力有待提升")
	}

	if gm >= 40 {
		highlights = append(highlights, "高毛利率，定价权稳固")
	} else {
		risks = append(risks, "毛利率未达 40%，产品竞争力一般")
	}

	if dr <= 40 {
		highlights = append(highlights, "低负债率，财务结构稳健")
	} else if dr > 60 {
		risks = append(risks, "负债率超过 60%，偿债压力偏大")
	}

	if ascore < 40 {
		highlights = append(highlights, "A-Score 安全，财务质量良好")
	} else if ascore < 60 {
		highlights = append(highlights, "A-Score 低风险，财务质量可控")
	} else if ascore < 70 {
		risks = append(risks, "A-Score 中风险，需关注财务健康度")
	} else {
		risks = append(risks, "A-Score 高风险，建议谨慎")
	}

	if growth >= 10 {
		highlights = append(highlights, "营收稳健增长")
	} else if growth < 0 {
		risks = append(risks, "营收负增长，成长性承压")
	}

	if pg >= 10 {
		highlights = append(highlights, "净利润持续增长")
	} else if pg < 0 {
		risks = append(risks, "净利润下滑，盈利能力减弱")
	}

	if cr >= 100 {
		highlights = append(highlights, "经营现金流充沛，盈利质量高")
	} else if cr > 0 {
		risks = append(risks, "现金流含金量不足")
	}

	return HighlightRisk{Highlights: highlights, Risks: risks}
}

// writeDataQualityWarnings 在报告中显示数据质量警告
func riskEmoji(level string) string {
	switch level {
	case "high":
		return "🔴"
	case "medium":
		return "🟡"
	default:
		return "🟢"
	}
}

// writeAuditOpinion 在报告中展示审计意见
func findStepResult(steps []StepResult, num int) *StepResult {
	for i := range steps {
		if steps[i].StepNum == num {
			return &steps[i]
		}
	}
	return nil
}

// writeModuleQuarterly 季度滚动预警模块
