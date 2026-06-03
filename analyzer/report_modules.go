package analyzer

import (
	"fmt"
	"math"
	"strings"
)

func writeModule1(b *strings.Builder, symbol string, steps []StepResult, latest, prev string, score *YearScore, quote *QuoteData, technical *TechnicalData, activity *ActivityData, ml *MLPredictionData, riskAlert *RiskAlertSummary) {
	b.WriteString("# 模块1: 执行摘要\n\n")

	writeEightIndicatorsHighlight(b, steps, latest)

	// 1.0 审计意见
	writeAuditOpinion(b, steps, latest, riskAlert)

	b.WriteString("## 1.1 多维度评分汇总\n\n")
	b.WriteString("| 评估维度 | 评级 | 得分 | 关键结论 |\n")
	b.WriteString("|----------|------|------|----------|\n")
	if score != nil {
		b.WriteString(fmt.Sprintf("| **财务健康度综合评分** | %s | **%.0f/100** | %s |\n", scoreToStars(score.RawScore), score.RawScore, gradeComment(score.RawScore)))
	}
	b.WriteString(fmt.Sprintf("| **成长能力** | %s | %.0f/100 | %s |\n", scoreToStars(growthScore(steps, latest)), growthScore(steps, latest), growthComment(steps, latest)))
	b.WriteString(fmt.Sprintf("| **盈利能力** | %s | %.0f/100 | %s |\n", scoreToStars(profitScore(steps, latest)), profitScore(steps, latest), profitComment(steps, latest)))
	b.WriteString(fmt.Sprintf("| **现金流质量** | %s | %.0f/100 | %s |\n", scoreToStars(cashScore(steps, latest)), cashScore(steps, latest), cashComment(steps, latest)))
	b.WriteString(fmt.Sprintf("| **偿债安全** | %s | %.0f/100 | %s |\n", scoreToStars(debtScore(steps, latest)), debtScore(steps, latest), debtComment(steps, latest)))
	vs := valuationScore(quote)
	vc := valuationComment(quote)
	if quote != nil && (quote.PE > 0 || quote.PB > 0) {
		b.WriteString(fmt.Sprintf("| **实时估值** | %s | %.0f/100 | %s |\n", scoreToStars(vs), vs, vc))
	} else {
		b.WriteString(fmt.Sprintf("| **实时估值** | - | -/100 | %s |\n", vc))
	}
	if technical != nil && technical.Score > 0 {
		b.WriteString(fmt.Sprintf("| **技术形态** | %s | %.0f/100 | %s |\n", scoreToStars(technical.Score), technical.Score, technical.Comment))
	} else {
		b.WriteString(fmt.Sprintf("| **技术形态** | - | -/100 | 待接入技术分析数据 |\n"))
	}
	if activity != nil && activity.Score > 0 {
		b.WriteString(fmt.Sprintf("| **交易活跃度** | %s | %.0f/100 | %s |\n", formatActivityStars(activity.Stars), activity.Score, activity.Comment))
	} else {
		b.WriteString(fmt.Sprintf("| **交易活跃度** | - | -/100 | 数据不足 |\n"))
	}

	if ml != nil && ml.Summary != nil && ml.Summary.HasData {
		sum := ml.Summary
		var predText string
		if sum.RangeHigh > 0 && sum.RangeLow >= 0 {
			predText = fmt.Sprintf("%s +%.0f%%~+%.0f%%", mlDirectionCN(sum.Direction), sum.RangeLow, sum.RangeHigh)
		} else if sum.RangeHigh <= 0 && sum.RangeLow < 0 {
			predText = fmt.Sprintf("%s %.0f%%~%.0f%%", mlDirectionCN(sum.Direction), math.Abs(sum.RangeHigh), math.Abs(sum.RangeLow))
		} else {
			predText = fmt.Sprintf("%s %.0f%%~+%.0f%%", mlDirectionCN(sum.Direction), sum.RangeLow, sum.RangeHigh)
		}
		b.WriteString(fmt.Sprintf("| **ML预测** | - | -/30 | %s |\n", predText))
	} else {
		b.WriteString(fmt.Sprintf("| **ML预测** | - | -/30 | 待接入机器学习模型 |\n"))
	}
	b.WriteString(fmt.Sprintf("| **逆向检查** | %s | %.0f/100 | %s |\n", scoreToStars(reverseScore(steps, latest, score)), reverseScore(steps, latest, score), reverseComment(steps, latest, score)))
	b.WriteString(fmt.Sprintf("| **投资检查清单** | %s | %.1f/10 | %s |\n", scoreToStars(buffettScore(steps, latest, score)*10), buffettScore(steps, latest, score), buffettComment(steps, latest, score)))
	if score != nil {
		weighted := score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
		b.WriteString(fmt.Sprintf("| **综合建议** | **%s** | **%.0f/100** | %s |\n", investmentGrade(weighted), weighted, strategyAdvice(weighted)))
	}
	b.WriteString("\n")
	b.WriteString("\n")

	b.WriteString("## 1.2 综合评级与建议\n\n")
	if score != nil {
		weighted := score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
		b.WriteString(fmt.Sprintf("**综合评分**: %.0f/100 %s  \n", weighted, scoreToStars(weighted)))
		b.WriteString(fmt.Sprintf("**投资评级**: **%s**  \n", investmentGrade(weighted)))
		b.WriteString(fmt.Sprintf("**建议仓位**: %s  \n", positionAdvice(weighted)))
		b.WriteString(fmt.Sprintf("**操作策略**: %s  \n\n", strategyAdvice(weighted)))
		b.WriteString("> **一句话建议**: ")
		b.WriteString(oneSentenceAdvice(symbol, weighted, steps, latest))
		b.WriteString("\n\n")
	}
	b.WriteString("---\n\n")
}

// ========== 模块2: 换手率深度分析 ==========
func writeModule2(b *strings.Builder, quote *QuoteData) {
	b.WriteString("# 模块2: 换手率深度分析\n\n")

	if quote == nil || quote.TurnoverRate == 0 {
		b.WriteString("> **说明**: 当前暂无实时换手率数据。请在网络畅通时重新选中股票获取行情。\n\n")
		b.WriteString("---\n\n")
		return
	}

	tr := quote.TurnoverRate
	vol := quote.Volume
	toa := quote.TurnoverAmount
	vr := quote.VolumeRatio

	b.WriteString("## 2.1 实时换手与成交指标\n\n")
	b.WriteString("| 指标 | 数值 | 评估 |\n")
	b.WriteString("|------|------|------|\n")
	b.WriteString(fmt.Sprintf("| **换手率** | %.2f%% | %s |\n", tr, turnoverAssessment(tr)))
	b.WriteString(fmt.Sprintf("| **成交量** | %.0f 手 | - |\n", vol))
	b.WriteString(fmt.Sprintf("| **成交额** | %.0f 万元 | - |\n", toa/10000))
	if vr > 0 {
		b.WriteString(fmt.Sprintf("| **量比** | %.2f | %s |\n", vr, volumeRatioAssessment(vr)))
	}
	b.WriteString("\n")

	b.WriteString("## 2.2 流动性评级\n\n")
	if tr < 1 {
		b.WriteString("- **流动性偏低**：换手率低于 1%，交投清淡，大额买卖可能对价格产生较大冲击。\n")
	} else if tr < 3 {
		b.WriteString("- **流动性正常**：换手率在 1%~3% 区间，交易活跃度适中，流动性风险可控。\n")
	} else if tr < 7 {
		b.WriteString("- **流动性活跃**：换手率在 3%~7% 区间，市场关注度较高，买卖盘相对充裕。\n")
	} else {
		b.WriteString("- **流动性非常活跃**：换手率超过 7%，交投极度活跃，需警惕短期波动放大。\n")
	}
	b.WriteString("\n---\n\n")
}

// ========== 模块3: 公司基本面分析 ==========
func writeModule3(b *strings.Builder, steps []StepResult, years []string, latest, prev string, quarterlyAlert *QuarterlyAlert, ttmMetrics *TTMMetrics) {
	b.WriteString("# 模块3: 公司基本面分析\n\n")

	b.WriteString("## 3.1 全年核心财务数据" + traceTrigger(3, 9, 10, 15, 16) + "\n\n")
	b.WriteString(fmt.Sprintf("| 指标 | %s | %s | 同比 | 评估 |\n", latest, prev))
	b.WriteString("|------|--------|------|------|------|\n")
	writeMetricRow(b, "营业收入", getStepValue(steps, 9, latest, "revenue"), getStepValue(steps, 9, prev, "revenue"), "亿元", 1e8)
	writeMetricRow(b, "归母净利润", getStepValue(steps, 16, latest, "profit"), getStepValue(steps, 16, prev, "profit"), "亿元", 1e8)
	writeMetricRow(b, "ROE", getStepValue(steps, 16, latest, "roe"), getStepValue(steps, 16, prev, "roe"), "%", 1)
	writeMetricRow(b, "毛利率", getStepValue(steps, 10, latest, "grossMargin"), getStepValue(steps, 10, prev, "grossMargin"), "%", 1)
	writeMetricRow(b, "资产负债率", getStepValue(steps, 3, latest, "debtRatio"), getStepValue(steps, 3, prev, "debtRatio"), "%", 1)
	writeMetricRow(b, "经营现金流净额", getStepValue(steps, 15, latest, "operatingCF"), getStepValue(steps, 15, prev, "operatingCF"), "亿元", 1e8)
	b.WriteString("\n")

	b.WriteString("## 3.2 核心财务指标趋势（近5年）" + traceTrigger(3, 9, 10, 15, 16) + "\n\n")
	b.WriteString("| 年度 | ROE | 毛利率 | 资产负债率 | 营收增长率 | 净利润现金含量 | M-Score |\n")
	b.WriteString("|------|-----|--------|------------|------------|----------------|---------|\n")
	for i := 0; i < len(years) && i < 5; i++ {
		year := years[i]
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
			year,
			fmtVal(getStepValue(steps, 16, year, "roe"), "%"),
			fmtVal(getStepValue(steps, 10, year, "grossMargin"), "%"),
			fmtVal(getStepValue(steps, 3, year, "debtRatio"), "%"),
			fmtVal(getStepValue(steps, 9, year, "growthRate"), "%"),
			fmtVal(getStepValue(steps, 15, year, "cashRatio"), "%"),
			fmtVal(getStepValue(steps, 8, year, "MScore"), ""),
		))
	}
	b.WriteString("\n")
	b.WriteString("> **解读**: 持续观察ROE和毛利率的趋势变化，若连续下滑需警惕竞争力衰退；资产负债率稳定或下降为加分项。M-Score已纳入A-Score综合风险体系，A-Score≥60时建议重点核查财报真实性与偿债能力。\n\n")

	// 财务趋势图表（与中栏"财务趋势"弹窗保持一致）
	b.WriteString("```chart-financial-trend\n```\n\n")

	// 3.3 季度滚动与 TTM 透视
	if quarterlyAlert != nil && quarterlyAlert.HasData {
		writeModuleQuarterly(b, quarterlyAlert)
	}
	if ttmMetrics != nil && ttmMetrics.HasData {
		writeModuleTTM(b, ttmMetrics)
	}

	b.WriteString(fmt.Sprintf("## 3.5 财务指标逐项解读（%s）\n\n", latest))
	categories := []struct {
		name  string
		steps []int
	}{
		{"会计与资产质量", []int{2, 5, 6, 7, 8}},
		{"偿债与营运安全", []int{3, 4, 11, 17}},
		{"盈利能力", []int{10, 12, 13, 14, 16}},
		{"现金流与分红", []int{15, 18}},
		{"成长能力", []int{9}},
	}
	b.WriteString("| 维度 | 达标数/总数 | 状态 |\n")
	b.WriteString("|------|-------------|------|\n")
	for _, cat := range categories {
		pass, total := countCategoryPass(steps, cat.steps, latest)
		status := "🟢 健康"
		if pass < total {
			if float64(pass)/float64(total) < 0.6 {
				status = "🔴 偏弱"
			} else {
				status = "🟡 一般"
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %d/%d | %s |\n", cat.name, pass, total, status))
	}
	b.WriteString("\n")

	b.WriteString("## 3.6 核心风险点\n\n")
	risks := extractRisks(steps, latest)
	if len(risks) == 0 {
		b.WriteString("未发现重大风险，财务整体可控。\n")
	} else {
		b.WriteString("| 风险类别 | 风险描述 | 严重程度 |\n")
		b.WriteString("|----------|----------|----------|\n")
		for _, r := range risks {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.Category, r.Indicator, r.Severity))
		}
	}
	b.WriteString("\n---\n\n")
}

// ========== 模块4: 行业横向对比分析 ==========
func writeModule4(b *strings.Builder, steps []StepResult, latest string, comp *ComparableAnalysis, industry *IndustryComparison, activityScore float64) {
	b.WriteString("# 模块4: 行业横向对比分析\n\n")

	// 行业均值对比（新增）
	if industry != nil && industry.HasData {
		b.WriteString("## 4.0 行业均值对比\n\n")
		b.WriteString("| 指标 | 当前公司 | 行业均值 | 差异 | 说明 |\n")
		b.WriteString("|------|----------|----------|------|------|\n")

		roe := getStepValue(steps, 16, latest, "roe")
		gm := getStepValue(steps, 10, latest, "grossMargin")
		growth := getStepValue(steps, 9, latest, "growthRate")
		debt := getStepValue(steps, 3, latest, "debtRatio")

		roeEmoji := "➡️"
		if industry.ROEPercentile >= 75 {
			roeEmoji = "🟢"
		} else if industry.ROEPercentile <= 25 {
			roeEmoji = "🔴"
		}

		b.WriteString(fmt.Sprintf("| **ROE** | %.2f%% | %s | %+.2f%% | %s 行业百分位: %.0f%% |\n",
			roe, industry.Industry, industry.GMDiff, roeEmoji, industry.ROEPercentile))
		b.WriteString(fmt.Sprintf("| **毛利率** | %.2f%% | 行业均值 | %+.2f%% | %s |\n",
			gm, industry.GMDiff, getDiffEmoji(industry.GMDiff)))
		b.WriteString(fmt.Sprintf("| **营收增长** | %.2f%% | 行业均值 | %+.2f%% | %s |\n",
			growth, industry.GrowthDiff, getDiffEmoji(industry.GrowthDiff)))
		b.WriteString(fmt.Sprintf("| **负债率** | %.2f%% | 行业均值 | %+.2f%% | %s |\n",
			debt, industry.DebtDiff, getDiffEmoji(-industry.DebtDiff)))
		b.WriteString("\n")

		b.WriteString(fmt.Sprintf("> **行业对比总结**: %s\n\n", industry.Summary))
	}

	if comp == nil || !comp.HasData || len(comp.Metrics) == 0 {
		b.WriteString("> **说明**: 当前未配置可比公司，或可比公司数据尚未下载。请在股票详情页的\"可比公司\"面板中添加 3~5 家对标公司并下载其财报数据。\n\n")
		b.WriteString("---\n\n")
		return
	}

	target := &ComparableMetrics{
		Symbol:        "当前公司",
		ROE:           getStepValue(steps, 16, latest, "roe"),
		GrossMargin:   getStepValue(steps, 10, latest, "grossMargin"),
		RevenueGrowth: getStepValue(steps, 9, latest, "growthRate"),
		DebtRatio:     getStepValue(steps, 3, latest, "debtRatio"),
		CashRatio:     getStepValue(steps, 15, latest, "cashRatio"),
		AScore:        getStepValue(steps, 8, latest, "AScore"),
		ActivityScore: activityScore,
	}

	b.WriteString(fmt.Sprintf("## 4.1 可比公司关键指标对比（%s）", latest) + traceTrigger(3, 9, 10, 15, 16) + "\n\n")
	b.WriteString("| 指标 | 当前公司 | 可比均值 | 最高 | 最低 | 排名百分位 |\n")
	b.WriteString("|------|----------|----------|------|------|------------|\n")
	b.WriteString(fmt.Sprintf("| **ROE** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.ROE, comp.Average.ROE, comp.Max.ROE, comp.Min.ROE, RankPercentile(comp.Metrics, target, "roe")))
	b.WriteString(fmt.Sprintf("| **毛利率** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.GrossMargin, comp.Average.GrossMargin, comp.Max.GrossMargin, comp.Min.GrossMargin, RankPercentile(comp.Metrics, target, "grossMargin")))
	b.WriteString(fmt.Sprintf("| **营收增长率** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.RevenueGrowth, comp.Average.RevenueGrowth, comp.Max.RevenueGrowth, comp.Min.RevenueGrowth, RankPercentile(comp.Metrics, target, "revenueGrowth")))
	b.WriteString(fmt.Sprintf("| **资产负债率** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.DebtRatio, comp.Average.DebtRatio, comp.Max.DebtRatio, comp.Min.DebtRatio, RankPercentile(comp.Metrics, target, "debtRatio")))
	b.WriteString(fmt.Sprintf("| **净利润现金含量** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.0f%% |\n",
		target.CashRatio, comp.Average.CashRatio, comp.Max.CashRatio, comp.Min.CashRatio, RankPercentile(comp.Metrics, target, "cashRatio")))
	b.WriteString(fmt.Sprintf("| **A-Score** | %.1f | %.1f | %.1f | %.1f | %.0f%% |\n",
		target.AScore, comp.Average.AScore, comp.Max.AScore, comp.Min.AScore, RankPercentile(comp.Metrics, target, "aScore")))
	if target.ActivityScore >= 0 || comp.Average.ActivityScore >= 0 {
		avgAct := comp.Average.ActivityScore
		if avgAct < 0 {
			avgAct = 0
		}
		b.WriteString(fmt.Sprintf("| **活跃度** | %.0f | %.0f | %.0f | %.0f | %.0f%% |\n",
			math.Max(0, target.ActivityScore), avgAct, math.Max(0, comp.Max.ActivityScore), math.Max(0, comp.Min.ActivityScore), RankPercentile(comp.Metrics, target, "activityScore")))
	}
	b.WriteString("\n")

	// 4.2 可比公司明细（含加权评分、排序、蓝色亮条）
	all := make([]*ComparableMetrics, 0, len(comp.Metrics)+1)
	all = append(all, target)
	for _, m := range comp.Metrics {
		all = append(all, m)
	}

	// 计算缺失活跃度的中位数替代值
	medianActivity := medianActivityScore(all)

	// 按综合得分排序
	type scored struct {
		*ComparableMetrics
		score float64
		rank  int
	}
	scoredList := make([]scored, 0, len(all))
	for _, m := range all {
		scoredList = append(scoredList, scored{
			ComparableMetrics: m,
			score:             calcComparableScore(m, all, medianActivity),
		})
	}
	// 降序
	for i := 0; i < len(scoredList); i++ {
		for j := i + 1; j < len(scoredList); j++ {
			if scoredList[i].score < scoredList[j].score {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}
	for i := range scoredList {
		scoredList[i].rank = i + 1
	}

	// 找到当前公司的排名
	targetRank := 1
	for _, s := range scoredList {
		if s.Symbol == "当前公司" {
			targetRank = s.rank
			break
		}
	}
	total := len(scoredList)
	percentile := float64(targetRank-1) / float64(total) * 100 // 越小越好

	var advice string
	if targetRank == 1 {
		advice = "当前公司综合评分最高，建议重点关注/持有 🥇"
	} else if percentile < 30 {
		advice = "当前公司质地优良，建议持有"
	} else if percentile < 60 {
		advice = "当前公司表现中等，可继续持有观察"
	} else {
		advice = "当前公司相对可比公司存在明显短板，建议谨慎"
	}

	// 检查是否有缺失活跃度的可比公司
	hasMissingActivity := false
	for _, s := range scoredList {
		if s.ActivityScore < 0 {
			hasMissingActivity = true
			break
		}
	}

	b.WriteString("## 4.2 可比公司明细\n\n")
	b.WriteString("> **综合得分计算方式**：每个指标按 A 股通用固定档位映射到 0-100 分（ROE≥15%、毛利率≥40%、营收增长≥10%、负债率≤40%、A-Score<40 视为优秀），再按以下权重加权求和：ROE 25%、毛利率 20%、营收增长 15%、现金含量 10%、负债率 10%（反向，越低越好）、A-Score 10%（反向，越低越好）、活跃度 10%。**得分仅依赖该公司自身指标，与可比池组成无关，加减可比公司不会改变任何一家的得分。** 缺失活跃度时使用可比池有效样本的中位数替代，标记为 *。\n\n")
	if hasMissingActivity {
		b.WriteString("> 部分可比公司活跃度使用样本中位数替代，[获取真实活跃度](#fetch-activity)\n\n")
	}
	b.WriteString("| 排名 | 公司 | ROE | 毛利率 | 营收增长 | 负债率 | 现金含量 | A-Score | 活跃度 | 综合得分 |\n")
	b.WriteString("|------|------|-----|--------|----------|--------|----------|---------|--------|----------|\n")
	for _, s := range scoredList {
		displayName := s.Name
		if displayName == "" {
			displayName = s.Symbol
		}
		if s.Symbol == "当前公司" {
			displayName = "**当前公司**"
		}
		actStr := "-"
		if s.ActivityScore >= 0 {
			actStr = fmt.Sprintf("%.0f", s.ActivityScore)
		} else if medianActivity > 0 {
			actStr = fmt.Sprintf("%.0f*", medianActivity)
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.1f | %s | %.1f |\n",
			s.rank, displayName, s.ROE, s.GrossMargin, s.RevenueGrowth, s.DebtRatio, s.CashRatio, s.AScore, actStr, s.score))
	}
	avgActStr := "-"
	if comp.Average.ActivityScore >= 0 {
		avgActStr = fmt.Sprintf("%.0f", comp.Average.ActivityScore)
	}
	b.WriteString(fmt.Sprintf("| — | **平均值** | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.1f | %s | — |\n",
		comp.Average.ROE, comp.Average.GrossMargin, comp.Average.RevenueGrowth, comp.Average.DebtRatio, comp.Average.CashRatio, comp.Average.AScore, avgActStr))
	b.WriteString("\n")

	b.WriteString("> **解读**: 排名百分位表示当前公司在可比公司中的相对位置（越高越好，负债率与 A-Score 为反向指标）。活跃度带 `*` 表示使用样本中位数替代（该可比公司暂无本地缓存数据）。综合得分基于 ROE(25%)、毛利率(20%)、营收增长(15%)、现金含量(10%)、负债率(10%)、A-Score(10%)、活跃度(10%) 加权计算。\n\n")

	b.WriteString(fmt.Sprintf("> **💡 综合评分排名**（满分100）\n"))
	b.WriteString(fmt.Sprintf("> 当前公司在 **%d** 家可比公司中排名第 **%d**，综合得分 **%.1f**。\n", total, targetRank, scoredList[targetRank-1].score))
	b.WriteString(fmt.Sprintf("> %s\n\n", advice))
	// 多年度趋势对比
	if len(comp.YearlyTrends) >= 2 && len(comp.CommonYears) >= 2 {
		b.WriteString("## 4.3 多年度趋势对比（当前公司 vs 可比均值）\n\n")
		b.WriteString("| 年份 | ROE(公司/均值) | 毛利率(公司/均值) | 负债率(公司/均值) | 现金含量(公司/均值) |\n")
		b.WriteString("|------|----------------|-------------------|-------------------|---------------------|\n")
		for _, yt := range comp.YearlyTrends {
			ty := getStepValue(steps, 16, yt.Year, "roe")
			tgm := getStepValue(steps, 10, yt.Year, "grossMargin")
			tdr := getStepValue(steps, 3, yt.Year, "debtRatio")
			tcr := getStepValue(steps, 15, yt.Year, "cashRatio")
			b.WriteString(fmt.Sprintf("| **%s** | %.2f%% / %.2f%% | %.2f%% / %.2f%% | %.2f%% / %.2f%% | %.2f%% / %.2f%% |\n",
				yt.Year, ty, yt.Average.ROE, tgm, yt.Average.GrossMargin, tdr, yt.Average.DebtRatio, tcr, yt.Average.CashRatio))
		}
		b.WriteString("\n")

		b.WriteString("### 趋势简评\n\n")
		writeComparableTrendComment(b, steps, comp)
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
}

// WriteModule4Only 仅生成模块4内容（用于增量更新）
func WriteModule4Only(b *strings.Builder, steps []StepResult, latest string, comp *ComparableAnalysis, industry *IndustryComparison, activityScore float64) {
	writeModule4(b, steps, latest, comp, industry, activityScore)
}

// ========== 模块5: 十五五政策匹配度评估 ==========
func writeModule5(b *strings.Builder, policy *PolicyMatchData) {
	b.WriteString("# 模块5: 十五五政策匹配度评估\n\n")

	if policy == nil || policy.Industry == "" {
		b.WriteString("> **说明**: 当前暂无政策匹配数据。请在网络畅通时重新选中股票获取基本资料。\n\n")
		b.WriteString("---\n\n")
		return
	}

	b.WriteString("## 5.1 政策匹配概览\n\n")
	b.WriteString(fmt.Sprintf("| 评估维度 | 结果 |\n"))
	b.WriteString(fmt.Sprintf("|----------|------|\n"))
	b.WriteString(fmt.Sprintf("| **所属行业** | %s |\n", policy.Industry))
	b.WriteString(fmt.Sprintf("| **匹配评级** | %s |\n", policy.MatchLevel))
	b.WriteString(fmt.Sprintf("| **政策评分** | %d / 100 |\n", policy.Score))
	b.WriteString("\n")

	// 按 Level 分组（1-5），同档位合并到一行，由高到低排列
	groups := make(map[int][]string)
	for _, p := range policy.Policies {
		lv := p.Level
		if lv < 1 {
			lv = 1
		}
		if lv > 5 {
			lv = 5
		}
		groups[lv] = append(groups[lv], p.Name)
	}

	b.WriteString("## 5.2 重点政策方向\n\n")
	for lv := 5; lv >= 1; lv-- {
		names, ok := groups[lv]
		if !ok || len(names) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("- `%s` **%s**\n", policySignalText(lv), strings.Join(names, "、")))
	}
	b.WriteString("\n")

	b.WriteString("## 5.3 解读摘要\n\n")
	b.WriteString(fmt.Sprintf("> %s\n\n", policy.Summary))

	b.WriteString("---\n\n")
}

func writeModule6(b *strings.Builder, steps []StepResult, latest string, quote *QuoteData, rim *RIMData) {
	b.WriteString("# 模块6: 剩余收益模型估值(RIM)\n\n")

	roe := getStepValue(steps, 16, latest, "roe")

	b.WriteString(fmt.Sprintf("## 6.1 模型参数（基于 %s 年报）", latest) + traceTrigger(16) + "\n\n")
	b.WriteString("| 参数 | 符号 | 取值 | 说明 |\n")
	b.WriteString("|------|------|------|------|\n")
	b.WriteString(fmt.Sprintf("| **ROE** | ROE | %.2f%% | 年报数据 |\n", roe))

	hasRIM := rim != nil && rim.HasData && rim.Result != nil
	var bps0, ke, gTerminal, currentPrice float64

	if hasRIM {
		bps0 = rim.Params.BPS0
		ke = rim.Params.KE
		gTerminal = rim.Params.GTerminal
		currentPrice = rim.Params.CurrentPrice
		b.WriteString(fmt.Sprintf("| **每股净资产** | BPS | %.2f元 | %s |\n", bps0, rimSourceDesc(rim, quote)))
		b.WriteString(fmt.Sprintf("| **资本成本** | kE | %.2f%% | CAPM(Rf=%.2f%%, Beta=%.2f, Rm-Rf=%.2f%%) |\n", ke*100, rim.Rf*100, rim.Beta, rim.RmRf*100))
		b.WriteString(fmt.Sprintf("| **永续增长率** | g | %.1f%% | 稳态假设 |\n", gTerminal*100))
		if currentPrice > 0 {
			b.WriteString(fmt.Sprintf("| **当前股价** | P | %.2f元 | 实时行情 |\n", currentPrice))
		} else {
			b.WriteString("| **当前股价** | P | - | 待接入实时行情 |\n")
		}
		// 显示预测期 EPS
		if len(rim.Result.Details) > 0 {
			var epsLine string
			for i, d := range rim.Result.Details {
				if i > 0 {
					epsLine += ", "
				}
				yearLabel := fmt.Sprintf("第%d年", d.Year)
				if d.CalendarYear > 0 {
					yearLabel = fmt.Sprintf("%d年", d.CalendarYear)
				}
				epsLine += fmt.Sprintf("%s %.2f", yearLabel, d.EPS)
			}
			b.WriteString(fmt.Sprintf("| **预测期 EPS** | - | %s | 机构一致预期 |\n", epsLine))
		}
	} else {
		b.WriteString("| **每股净资产** | BPS | 待计算 | 需接入实时行情与财报 |\n")
		b.WriteString("| **资本成本** | kE | 7.0% | 假设值 |\n")
		b.WriteString("| **永续增长率** | g | 3.0% | 假设值 |\n")
		b.WriteString("| **当前股价** | P | - | 待接入实时行情 |\n")
	}
	b.WriteString("\n")

	b.WriteString("## 6.2 估值情景\n\n")
	if hasRIM {
		b.WriteString("| 情景 | ROE假设 | 内在价值(元) | 相对现价 | 评级 |\n")
		b.WriteString("|------|---------|-------------|----------|------|\n")
		b.WriteString(fmt.Sprintf("| 悲观 | %.2f%% | %.2f | %+.1f%% | %s |\n", rim.Result.Pessimistic.ROE, rim.Result.Pessimistic.Value, rim.Result.Pessimistic.DiffPct, rim.Result.Pessimistic.Grade))
		b.WriteString(fmt.Sprintf("| 基准 | %.2f%% | %.2f | %+.1f%% | %s |\n", rim.Result.Baseline.ROE, rim.Result.Baseline.Value, rim.Result.Baseline.DiffPct, rim.Result.Baseline.Grade))
		b.WriteString(fmt.Sprintf("| 乐观 | %.2f%% | %.2f | %+.1f%% | %s |\n", rim.Result.Optimistic.ROE, rim.Result.Optimistic.Value, rim.Result.Optimistic.DiffPct, rim.Result.Optimistic.Grade))
		b.WriteString("\n")
	} else {
		b.WriteString("| 情景 | ROE假设 | 内在价值/净资产 | 评级 |\n")
		b.WriteString("|------|---------|----------------|------|\n")
		b.WriteString("| 悲观 | ROE-3pp | 约1.2-1.5x PB | 谨慎 |\n")
		b.WriteString("| 基准 | 维持当前 | 约1.5-2.0x PB | 中性 |\n")
		b.WriteString("| 乐观 | ROE+3pp | 约2.0-2.5x PB | 积极 |\n")
		b.WriteString("\n")
	}

	// 多期明细
	if hasRIM && len(rim.Result.Details) > 0 {
		b.WriteString("## 6.3 多期计算明细\n\n")
		b.WriteString("| 年度 | EPS(元) | DPS(元) | BPS(元) | RE(元) | 折现率 | RE现值(元) |\n")
		b.WriteString("|------|---------|---------|---------|--------|--------|------------|\n")
		runningBPS := 0.0
		if rim != nil {
			runningBPS = rim.Params.BPS0
		}
		for _, d := range rim.Result.Details {
			runningBPS = runningBPS + d.EPS - d.DPS
			yearLabel := fmt.Sprintf("第%d年", d.Year)
			if d.CalendarYear > 0 {
				yearLabel = fmt.Sprintf("%d年", d.CalendarYear)
			}
			b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.2f | %.4f | %.4f | %.4f |\n", yearLabel, d.EPS, d.DPS, runningBPS, d.RE, d.Discount, d.PVRE))
		}
		b.WriteString(fmt.Sprintf("| **RE现值之和** | - | - | - | - | - | **%.4f** |\n", rim.Result.SumPVRE))
		b.WriteString(fmt.Sprintf("| **持续价值 CV** | - | - | - | %.4f | - | **%.4f** |\n", rim.Result.CV, rim.Result.PVCV))
		b.WriteString(fmt.Sprintf("| **每股价值** | - | - | - | - | - | **%.2f** |\n", rim.Result.Value))
		b.WriteString("\n")
		if currentPrice > 0 {
			b.WriteString(fmt.Sprintf("> **多期模型估算每股价值**: %.2f 元/股，相对当前股价 %.2f 元 **%+.1f%%**。\n\n", rim.Result.Value, currentPrice, rim.Result.Upside))
		} else {
			b.WriteString(fmt.Sprintf("> **多期模型估算每股价值**: %.2f 元/股（未接入实时股价，无法计算涨幅）。\n\n", rim.Result.Value))
		}
	}

	b.WriteString("> **解读**: RIM 估值的核心在于 ROE 能否持续高于资本成本。")
	if hasRIM && currentPrice > 0 {
		diff := rim.Result.Upside
		if diff >= 20 {
			b.WriteString(fmt.Sprintf("当前多期模型估算每股价值 %.2f 元显著高于市价 %.2f 元，存在约 %.0f%% 的潜在上行空间。", rim.Result.Value, currentPrice, diff))
		} else if diff >= 0 {
			b.WriteString(fmt.Sprintf("当前多期模型估算每股价值 %.2f 元略高于市价 %.2f 元，上行空间约 %.0f%%，安全边际一般。", rim.Result.Value, currentPrice, diff))
		} else {
			b.WriteString(fmt.Sprintf("当前多期模型估算每股价值 %.2f 元低于市价 %.2f 元，当前价格可能已反映乐观预期。", rim.Result.Value, currentPrice))
		}
	} else {
		if roe >= 15 {
			b.WriteString(fmt.Sprintf("当前 ROE %.2f%% 高于一般资本成本，具备创造价值的能力。", roe))
		} else if roe > 7 {
			b.WriteString(fmt.Sprintf("当前 ROE %.2f%% 略高于资本成本，但安全边际不足。", roe))
		} else {
			b.WriteString(fmt.Sprintf("当前 ROE %.2f%% 低于资本成本，长期可能侵蚀股东价值。", roe))
		}
	}
	b.WriteString("\n\n---\n\n")
}

func writeModule7(b *strings.Builder, quote *QuoteData, technical *TechnicalData, activity *ActivityData, moneyflow *MoneyflowData) {
	b.WriteString("# 模块8: 技术面分析\n\n")

	if quote == nil || quote.CurrentPrice == 0 {
		b.WriteString("> **说明**: 当前暂无实时行情数据，无法生成技术面分析。请在网络畅通时重新选中股票获取行情。\n\n")
		b.WriteString("---\n\n")
		return
	}

	cp := quote.CurrentPrice
	high := quote.High
	low := quote.Low
	open := quote.Open
	prev := quote.PreviousClose
	tr := quote.TurnoverRate
	amp := quote.Amplitude

	b.WriteString("## 8.1 日内价格位置\n\n")
	if high > low {
		pos := (cp - low) / (high - low) * 100
		b.WriteString(fmt.Sprintf("- 当前价格处于今日高低点区间的 **%.1f%%** 位置", pos))
		if pos > 70 {
			b.WriteString("，接近日内高点，多头力量较强。\n")
		} else if pos < 30 {
			b.WriteString("，接近日内低点，空头压力较大。\n")
		} else {
			b.WriteString("，位于中间区域，多空博弈均衡。\n")
		}
	}
	if open > 0 && prev > 0 {
		gap := (open - prev) / prev * 100
		if math.Abs(gap) > 1 {
			b.WriteString(fmt.Sprintf("- 今日开盘跳空 %.2f%%，%s\n", gap, gapDirection(gap)))
		}
	}
	b.WriteString("\n")

	b.WriteString("## 8.2 量价关系简评\n\n")
	// 优先使用近5日平均换手/振幅判断，避免单日异常导致误判
	avgTr := tr
	avgAmp := amp
	if activity != nil {
		if activity.TurnoverDensity > 0 {
			avgTr = activity.TurnoverDensity
		}
		if activity.AvgAmplitude5 > 0 {
			avgAmp = activity.AvgAmplitude5
		}
	}
	if avgTr >= 3 && avgAmp >= 3 {
		b.WriteString("- **高换手高振幅**：交投活跃，资金博弈激烈，短期趋势可能延续。\n")
	} else if avgTr >= 3 && avgAmp < 3 {
		b.WriteString("- **高换手低振幅**：筹码交换充分但价格波动有限，可能是蓄势或出货信号。\n")
	} else if avgTr < 1 && avgAmp >= 3 {
		b.WriteString("- **低换手高振幅**：流动性不足导致价格易受大单影响，波动具有偶然性。\n")
	} else {
		b.WriteString("- **低换手低振幅**：交投清淡，趋势惯性较强，突破需放量确认。\n")
	}

	b.WriteString("\n## 8.3 短期技术倾向\n\n")
	score := 0
	if quote.ChangePercent > 0 {
		score++
	}
	if cp > open {
		score++
	}
	if high > prev && low > prev*0.97 {
		score++
	}
	if tr > 1 {
		score++
	}

	switch score {
	case 4:
		b.WriteString("**偏多**:  price、开盘、高低点及换手均呈现积极信号。\n")
	case 3:
		b.WriteString("**略偏多**: 大部分日内指标偏向积极，但有一处偏弱。\n")
	case 2:
		b.WriteString("**中性**: 多空信号交织，短期方向不明。\n")
	case 1:
		b.WriteString("**略偏空**: 大部分日内指标偏弱，仅有一处积极信号。\n")
	default:
		b.WriteString("**偏空**: price、开盘、高低点及换手均呈现弱势信号。\n")
	}

	// 新增：基于历史K线的技术指标分析
	if technical != nil && technical.Score > 0 {
		b.WriteString("\n## 8.4 历史K线技术指标分析\n\n")
		b.WriteString(fmt.Sprintf("| 指标 | 状态 | 说明 |\n"))
		b.WriteString(fmt.Sprintf("|------|------|------|\n"))
		b.WriteString(fmt.Sprintf("| 技术评分 | %s | %.0f/100（%s） |\n", scoreToStars(technical.Score), technical.Score, technical.Grade))
		b.WriteString(fmt.Sprintf("| 趋势方向 | %s | %s |\n", technical.Trend, technical.MAStatus))
		b.WriteString(fmt.Sprintf("| MACD | %s | 动能信号 |\n", technical.MACDStatus))
		b.WriteString(fmt.Sprintf("| RSI(14) | %s | 超买超卖状态 |\n", technical.RSIStatus))
		b.WriteString(fmt.Sprintf("| 布林带 | %s | 价格相对波动区间 |\n", technical.BollingerStatus))
		b.WriteString(fmt.Sprintf("| 量价关系 | %s | 成交量配合程度 |\n", technical.VolumeStatus))
		b.WriteString(fmt.Sprintf("| 技术形态 | %s | %s |\n", technical.Pattern, technical.SupportResistance))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("> **综合结论**: %s\n\n", technical.Comment))

		b.WriteString("### 技术指标联动分析图（K线+成交量+MACD+RSI+布林带）\n\n")
		b.WriteString("```chart-unified\n```\n\n")
	}

	if activity != nil && activity.Score > 0 && activity.PotentialHint != "" {
		b.WriteString("\n## 8.5 交易活跃度潜力提示\n\n")
		b.WriteString(fmt.Sprintf("> **活跃度评级**: %s（%.0f分）\n\n", formatActivityStars(activity.Stars), activity.Score))
		b.WriteString(fmt.Sprintf("> 💡 **提示**: %s\n\n", activity.PotentialHint))
	}

	// 新增：资金流向分析
	if moneyflow != nil && moneyflow.HasData && len(moneyflow.Items) > 0 {
		b.WriteString("\n## 8.6 资金流向分析\n\n")
		b.WriteString(fmt.Sprintf("> **%s**\n\n", moneyflow.Summary))
		b.WriteString("| 日期 | 主力净流入 | 超大单 | 大单 | 中单 | 小单 |\n")
		b.WriteString("|------|-----------|--------|------|------|------|\n")
		for _, item := range moneyflow.Items {
			dateStr := item.Date
			if len(dateStr) == 8 {
				dateStr = dateStr[:4] + "-" + dateStr[4:6] + "-" + dateStr[6:]
			}
			b.WriteString(fmt.Sprintf("| %s | %s%.2f亿 | %s%.2f亿 | %s%.2f亿 | %s%.2f亿 | %s%.2f亿 |\n",
				dateStr,
				sign(item.MainInflow), math.Abs(item.MainInflow)/1e4,
				sign(item.ElgNetAmount), math.Abs(item.ElgNetAmount)/1e4,
				sign(item.LgNetAmount), math.Abs(item.LgNetAmount)/1e4,
				sign(item.MdNetAmount), math.Abs(item.MdNetAmount)/1e4,
				sign(item.SmNetAmount), math.Abs(item.SmNetAmount)/1e4,
			))
		}
		b.WriteString("\n")
		// 资金流向综合判断
		var inflowDays, outflowDays int
		var totalMain float64
		for _, item := range moneyflow.Items {
			totalMain += item.MainInflow
			if item.MainInflow > 0 {
				inflowDays++
			} else if item.MainInflow < 0 {
				outflowDays++
			}
		}
		if inflowDays > outflowDays {
			b.WriteString(fmt.Sprintf("- **主力资金整体呈流入态势**：近%d日中%d日净流入，累计主力净流入 %.2f 亿，表明机构资金对该股关注度较高。\n", len(moneyflow.Items), inflowDays, totalMain/1e4))
		} else if outflowDays > inflowDays {
			b.WriteString(fmt.Sprintf("- **主力资金整体呈流出态势**：近%d日中%d日净流出，累计主力净流出 %.2f 亿，需警惕机构资金撤离风险。\n", len(moneyflow.Items), outflowDays, -totalMain/1e4))
		} else {
			b.WriteString(fmt.Sprintf("- **主力资金分歧较大**：近%d日流入与流出天数持平，累计主力净流入 %.2f 亿，资金博弈激烈。\n", len(moneyflow.Items), totalMain/1e4))
		}
		// 散户与机构对比
		var totalSm, totalMd float64
		for _, item := range moneyflow.Items {
			totalSm += item.SmNetAmount
			totalMd += item.MdNetAmount
		}
		if totalSm > 0 && totalMain < 0 {
			b.WriteString("- **散户接盘迹象**：小单持续流入而主力净流出，可能存在散户接盘、主力出货的风险信号。\n")
		} else if totalSm < 0 && totalMain > 0 {
			b.WriteString("- **机构吸筹特征**：小单持续流出而主力净流入，可能是机构在低位吸筹、散户恐慌抛售。\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("\n---\n\n")
}

// ========== 模块9: ML机器学习预测（ONNX 双引擎：Engine-A 情绪+价格 / Engine-B 财务 BiLSTM） ==========
func writeModule8(b *strings.Builder, steps []StepResult, latest, prev string, ml *MLPredictionData) {
	b.WriteString("# 模块9: ML机器学习预测\n\n")

	hasML := ml != nil && (ml.Financial != nil || ml.Sentiment != nil || ml.EngineD != nil)
	if !hasML {
		b.WriteString("> **说明**: 当前版本暂无机器学习模型，以下基于财务趋势做简易方向推断。\n\n")
		// 如果 Python 推理返回了明确的错误（如缺少依赖包），直接显示
		if ml != nil && ml.MLError != "" {
			b.WriteString("> ⚠️ **Python 推理失败**: ")
			b.WriteString(ml.MLError)
			b.WriteString("\n\n")
			if strings.Contains(ml.MLError, "pip3 install") {
				b.WriteString("> 💻 **解决方法**: 打开终端（Terminal）并运行以下命令安装依赖：\n")
				b.WriteString("> ```bash\n")
				b.WriteString("> pip3 install onnxruntime scikit-learn numpy\n")
				b.WriteString("> ```\n")
				b.WriteString("> 安装完成后重新启动应用即可。\n\n")
			}
		} else {
			b.WriteString("> 💡 **排查建议**: 如已确认模型文件存在但仍显示此提示，请检查「设置 → 数据 → 🔍 检测 Python 环境」，确保 onnxruntime / scikit-learn / numpy 已正确安装。\n\n")
		}
	} else {
		b.WriteString("> **说明**: 以下预测结果由 ONNX 多引擎模型生成。\n\n")
	}

	// 置信等级标注
	if ml != nil && ml.Confidence != "" {
		confLabel := ""
		confDesc := ""
		switch ml.Confidence {
		case "high":
			confLabel = "🟢 高置信"
			confDesc = "模型输入数据充分，预测结果可信度较高"
		case "medium":
			confLabel = "🟡 中置信"
			confDesc = "部分模型输入使用默认值填充，预测结果仅供参考"
		case "low":
			confLabel = "🔴 低置信"
			confDesc = "模型输入数据缺失较多，预测结果可靠性有限，请勿作为决策依据"
		}
		if confLabel != "" {
			b.WriteString(fmt.Sprintf("> **预测置信等级**: %s — %s\n\n", confLabel, confDesc))
		}
	}

	// 双引擎融合总结（2-4周预测）
	if ml != nil && ml.Summary != nil && ml.Summary.HasData {
		sum := ml.Summary
		if sum.RangeHigh > 0 && sum.RangeLow >= 0 {
			b.WriteString(fmt.Sprintf("> **未来 2-4 周预测**: %s，预计涨幅 **+%.0f%% ~ +%.0f%%**。%s\n\n", sum.Direction, sum.RangeLow, sum.RangeHigh, sum.Reason))
		} else if sum.RangeHigh <= 0 && sum.RangeLow < 0 {
			b.WriteString(fmt.Sprintf("> **未来 2-4 周预测**: %s，预计跌幅 **%.0f%% ~ %.0f%%**。%s\n\n", sum.Direction, math.Abs(sum.RangeHigh), math.Abs(sum.RangeLow), sum.Reason))
		} else {
			b.WriteString(fmt.Sprintf("> **未来 2-4 周预测**: %s，预计波动区间 **%.0f%% ~ +%.0f%%**。%s\n\n", sum.Direction, sum.RangeLow, sum.RangeHigh, sum.Reason))
		}
	}

	// 引擎B：财务LSTM - 始终显示10.1章节
	b.WriteString("## 9.1 Engine-B 财务趋势预测（BiLSTM+Self-Attention）\n\n")
	if ml != nil && ml.Financial != nil {
		fp := ml.Financial
		b.WriteString(fmt.Sprintf("| 指标 | 预测方向 | 概率 | 说明 |\n"))
		b.WriteString("|------|----------|------|------|\n")
		b.WriteString(fmt.Sprintf("| ROE 趋势 | %s | %.1f%% | 基于最近 8 年度财务序列 |\n", mlDirectionCN(fp.ROEDirection), fp.ROEProb*100))
		b.WriteString(fmt.Sprintf("| 营收趋势 | %s | %.1f%% | 营收增长方向预测 |\n", mlDirectionCN(fp.RevenueDirection), fp.RevenueProb*100))
		b.WriteString(fmt.Sprintf("| M-Score 趋势 | %s | %.1f%% | 财报质量变化方向 |\n", mlDirectionCN(fp.MScoreDirection), fp.MScoreProb*100))
		b.WriteString(fmt.Sprintf("| 财务健康分 | %.2f / 10 | — | 综合健康度评分 |\n\n", fp.HealthScore))
	} else {
		b.WriteString("> **数据缺失**: Engine-B 模型暂不可用。可能原因：\n")
		b.WriteString("> - ONNX 模型文件未加载（需要 `financial_lstm.onnx` + `scaler.pkl`）\n")
		b.WriteString("> - 或历史财务数据不足（需要至少 1 年财报数据）\n\n")
	}

	// 引擎A：舆情+价格 - 始终显示10.2章节
	b.WriteString("## 9.2 Engine-A 市场情绪与价格融合预测（Cross-Attention）\n\n")
	if ml != nil && ml.Sentiment != nil {
		sp := ml.Sentiment
		b.WriteString(fmt.Sprintf("| 指标 | 预测结果 | 概率 | 说明 |\n"))
		b.WriteString("|------|----------|------|------|\n")
		b.WriteString(fmt.Sprintf("| 次日走势 | %s | %.1f%% | 上涨/持平/下跌 |\n", mlDirectionCN(sp.MovementLabel), sp.MovementProb*100))
		b.WriteString(fmt.Sprintf("| 异动概率 | %.2f%% | — | 价格异常波动预警 |\n\n", sp.AnomalyProb*100))
	} else {
		b.WriteString("> **数据缺失**: Engine-A 模型暂不可用。可能原因：\n")
		b.WriteString("> - ONNX 模型文件未加载（需要 `sentiment_price_fusion.onnx`）\n")
		b.WriteString("> - 或日 K 线数据不足（需要至少 16 个交易日）\n\n")
	}

	// 引擎D：风险预警 - 始终显示10.3章节
	b.WriteString("## 9.3 Engine-D 风险预警模型（GradientBoosting）\n\n")
	if ml != nil && ml.EngineD != nil {
		dp := ml.EngineD

		// 风险等级颜色标识
		riskEmoji := "🟢"
		if dp.RiskLevel == "中风险" {
			riskEmoji = "🟡"
		} else if dp.RiskLevel == "高风险" {
			riskEmoji = "🔴"
		}

		modelStatus := "✅ 模型"
		if !dp.ModelLoaded {
			modelStatus = "⚠️ 规则"
		}

		b.WriteString(fmt.Sprintf("| 指标 | 结果 | 说明 |\n"))
		b.WriteString("|------|------|------|\n")
		b.WriteString(fmt.Sprintf("| 风险评级 | %s %s | 基于%s评估 |\n", riskEmoji, dp.RiskLevel, modelStatus))
		b.WriteString(fmt.Sprintf("| 风险概率 | %.1f%% | 财务造假/退市风险概率 |\n", dp.RiskProb*100))

		if len(dp.TopFactors) > 0 {
			factorsStr := strings.Join(dp.TopFactors, ", ")
			b.WriteString(fmt.Sprintf("| 主要风险因子 | %s | 影响最大的特征 |\n", factorsStr))
		}
		b.WriteString("\n")

		// 风险提示
		if dp.RiskLabel == 1 {
			b.WriteString("> ⚠️ **风险提示**: Engine-D 模型识别到潜在风险信号，建议进一步审慎评估。\n\n")
		} else {
			b.WriteString("> ✅ **风险评估**: 当前无显著风险信号，财务状况相对健康。\n\n")
		}
	} else {
		b.WriteString("> **数据缺失**: Engine-D 模型暂不可用。可能原因：\n")
		b.WriteString("> - 模型文件未加载（需要 `engine_d_model.pkl`）\n")
		b.WriteString("> - 或财务数据加载失败\n\n")
	}

	// 如果没有 ML，保留原来的简易推断
	if !hasML {
		b.WriteString("## 9.4 负向因子（基于财务指标的简易推断）\n\n")
		var neg, pos []string
		if g := getStepValue(steps, 9, latest, "growthRate"); g < 10 {
			neg = append(neg, fmt.Sprintf("- 营收增长率 %.2f%%，低于理想水平", g))
		}
		if pg := getStepValue(steps, 16, latest, "profitGrowth"); pg < 10 {
			neg = append(neg, fmt.Sprintf("- 净利润增长率 %.2f%%，低于理想水平", pg))
		}
		if roe := getStepValue(steps, 16, latest, "roe"); roe < 15 {
			neg = append(neg, fmt.Sprintf("- ROE %.2f%%，资本回报能力偏弱", roe))
		}
		if gm := getStepValue(steps, 10, latest, "grossMargin"); gm < 40 {
			neg = append(neg, fmt.Sprintf("- 毛利率 %.2f%%，产品竞争力未达高毛利标准", gm))
		}
		if ascore := getStepValue(steps, 8, latest, "AScore"); ascore >= 60 {
			neg = append(neg, fmt.Sprintf("- A-Score %.1f，综合财务风险需关注", ascore))
		}
		if dr := getStepValue(steps, 3, latest, "debtRatio"); dr > 60 {
			neg = append(neg, fmt.Sprintf("- 资产负债率 %.2f%%，偿债压力偏大", dr))
		}
		if len(neg) == 0 {
			neg = append(neg, "- 暂无显著负向因子")
		}
		for _, s := range neg {
			b.WriteString(s + "\n")
		}
		b.WriteString("\n")

		b.WriteString("## 9.4 正向因子\n\n")
		if g := getStepValue(steps, 9, latest, "growthRate"); g >= 10 {
			pos = append(pos, fmt.Sprintf("- 营收增长率 %.2f%%，保持稳健增长", g))
		}
		if pg := getStepValue(steps, 16, latest, "profitGrowth"); pg >= 10 {
			pos = append(pos, fmt.Sprintf("- 净利润增长率 %.2f%%，盈利能力持续改善", pg))
		}
		if roe := getStepValue(steps, 16, latest, "roe"); roe >= 15 {
			pos = append(pos, fmt.Sprintf("- ROE %.2f%%，资本回报能力良好", roe))
		}
		if gm := getStepValue(steps, 10, latest, "grossMargin"); gm >= 40 {
			pos = append(pos, fmt.Sprintf("- 毛利率 %.2f%%，具备较强定价权", gm))
		}
		if dr := getStepValue(steps, 3, latest, "debtRatio"); dr <= 40 {
			pos = append(pos, fmt.Sprintf("- 资产负债率 %.2f%%，财务结构稳健", dr))
		}
		if cr := getStepValue(steps, 15, latest, "cashRatio"); cr >= 100 {
			pos = append(pos, fmt.Sprintf("- 净利润现金含量 %.2f%%，盈利质量高", cr))
		}
		if ascore := getStepValue(steps, 8, latest, "AScore"); ascore < 50 {
			pos = append(pos, fmt.Sprintf("- A-Score %.1f，综合财务风险可控", ascore))
		}
		if len(pos) == 0 {
			pos = append(pos, "- 暂无显著正向因子")
		}
		for _, s := range pos {
			b.WriteString(s + "\n")
		}
		b.WriteString("\n")

		b.WriteString("## 9.5 简易预测结论\n\n")
		score := 50.0
		score -= float64(len(neg)) * 8
		score += float64(len(pos)) * 8
		score = math.Max(0, math.Min(100, score))
		b.WriteString(fmt.Sprintf("**财务趋势评分**: %.0f/100（基于正负向因子简易加权）\n\n", score))
	}

	b.WriteString("---\n\n")
}

// ========== 模块10: 智能选股7大条件 ==========
func writeModule9(b *strings.Builder, steps []StepResult, latest, prev string) {
	b.WriteString("# 模块10: 智能选股7大条件\n\n")
	b.WriteString("## 10.1 条件检查表" + traceTrigger(3, 9, 10, 15, 16) + "\n\n")

	roe := getStepValue(steps, 16, latest, "roe")
	gm := getStepValue(steps, 10, latest, "grossMargin")
	growth := getStepValue(steps, 9, latest, "growthRate")
	dr := getStepValue(steps, 3, latest, "debtRatio")
	cashDiff := getStepValue(steps, 3, latest, "cashDebtDiff")
	cr := getStepValue(steps, 15, latest, "cashRatio")

	conditions := []struct {
		name   string
		std    string
		value  string
		pass   bool
		points int
		max    int
	}{
		{"① ROE ≥ 15%", "≥15%", fmt.Sprintf("%.2f%%", roe), roe >= 15, 20, 20},
		{"② 毛利率 ≥ 40%", "≥40%", fmt.Sprintf("%.2f%%", gm), gm >= 40, 15, 15},
		{"③ 营收增长 ≥ 10%", "≥10%", fmt.Sprintf("%.2f%%", growth), growth >= 10, 15, 15},
		{"④ A-Score < 60", "<60", fmt.Sprintf("%.1f", getStepValue(steps, 8, latest, "AScore")), getStepValue(steps, 8, latest, "AScore") < 60, 15, 15},
		{"⑤ 资产负债率 ≤ 60%", "≤60%", fmt.Sprintf("%.2f%%", dr), dr <= 60, 10, 10},
		{"⑥ 净利润现金含量 ≥ 100%", "≥100%", fmt.Sprintf("%.2f%%", cr), cr >= 100, 15, 15},
		{"⑦ 准货币资金-有息负债 ≥ 0", "≥0", fmt.Sprintf("%.2f亿", cashDiff/1e8), cashDiff >= 0, 10, 10},
	}

	b.WriteString("| 条件 | 标准 | 实际值 | 是否满足 | 得分 |\n")
	b.WriteString("|------|------|--------|----------|------|\n")
	totalScore := 0
	passCount := 0
	for _, c := range conditions {
		passStr := "❌"
		if c.pass {
			passStr = "✅"
			passCount++
			totalScore += c.points
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d/%d |\n", c.name, c.std, c.value, passStr, totalScore, c.max))
	}
	b.WriteString(fmt.Sprintf("| **总分** | - | - | **%d/%d项** | **%d/100** |\n", passCount, len(conditions), totalScore))
	b.WriteString("\n")

	if passCount >= 5 {
		b.WriteString("**选股评级**: ✅ **符合**核心买入条件\n\n")
	} else if passCount >= 3 {
		b.WriteString("**选股评级**: 🟡 **部分符合**，需观察短板改善\n\n")
	} else {
		b.WriteString("**选股评级**: ❌ **不符合**买入条件\n\n")
	}
	b.WriteString("---\n\n")
}

// ========== 模块11: 逆向思维检查 ==========
func writeModule10(b *strings.Builder, steps []StepResult, latest string, score *YearScore) {
	b.WriteString("# 模块11: 逆向思维检查\n\n")

	b.WriteString("## 11.1 逆向三问\n\n")

	b.WriteString("### 问1: 市场忽略了什么负面因素？\n\n")
	risks := extractRisks(steps, latest)
	if len(risks) == 0 {
		b.WriteString("- 暂未发现重大被忽略的负面因素。\n")
	} else {
		for _, r := range risks {
			b.WriteString(fmt.Sprintf("- **%s**: %s（%s）\n", r.Category, r.Indicator, r.Desc))
		}
	}
	b.WriteString("\n")

	b.WriteString("### 问2: 悲观时忽略了什么积极因素？\n\n")
	var positives []string
	if g := getStepValue(steps, 9, latest, "growthRate"); g >= 10 {
		positives = append(positives, fmt.Sprintf("- 营收保持 %.2f%% 增长，业务仍在扩张", g))
	}
	if roe := getStepValue(steps, 16, latest, "roe"); roe >= 15 {
		positives = append(positives, fmt.Sprintf("- ROE %.2f%%，长期资本回报能力依然优秀", roe))
	}
	if gm := getStepValue(steps, 10, latest, "grossMargin"); gm >= 40 {
		positives = append(positives, fmt.Sprintf("- 毛利率 %.2f%%，护城河较深", gm))
	}
	if dr := getStepValue(steps, 3, latest, "debtRatio"); dr <= 40 {
		positives = append(positives, fmt.Sprintf("- 资产负债率 %.2f%%，财务结构健康", dr))
	}
	if cr := getStepValue(steps, 15, latest, "cashRatio"); cr >= 100 {
		positives = append(positives, fmt.Sprintf("- 经营现金流充沛，净利润含金量 %.2f%%", cr))
	}
	if ascore := getStepValue(steps, 8, latest, "AScore"); ascore < 50 {
		positives = append(positives, fmt.Sprintf("- A-Score %.1f，财报质量可信", ascore))
	}
	if len(positives) == 0 {
		positives = append(positives, "- 当前财务数据中积极因素不明显，需等待基本面改善信号。")
	}
	for _, p := range positives {
		b.WriteString(p + "\n")
	}
	b.WriteString("\n")

	b.WriteString("### 问3: 什么情况下这笔交易会成功？\n\n")
	b.WriteString("**反转触发器**:\n")
	b.WriteString("1. ROE 持续回升并稳定在 15% 以上\n")
	b.WriteString("2. 毛利率止跌回升，定价权修复\n")
	b.WriteString("3. 经营现金流持续覆盖净利润\n")
	b.WriteString("4. A-Score 回落至 60 以下\n")
	b.WriteString("\n")

	b.WriteString("## 11.2 逆向检查评分\n\n")
	revScore := reverseScore(steps, latest, score)
	b.WriteString(fmt.Sprintf("**基础分**: 85分  \n"))
	b.WriteString(fmt.Sprintf("**扣分合计**: %.0f分  \n", math.Max(0, 85-revScore)))
	b.WriteString(fmt.Sprintf("**最终评分**: %.0f/100  \n", revScore))
	if revScore >= 70 {
		b.WriteString("**风险评级**: 🟢 **中等偏低风险**\n\n")
	} else if revScore >= 50 {
		b.WriteString("**风险评级**: 🟠 **中等风险**\n\n")
	} else {
		b.WriteString("**风险评级**: 🔴 **中高风险**\n\n")
	}
	b.WriteString("---\n\n")
}

// ========== 模块12: 投资检查清单 ==========
func writeModule11(b *strings.Builder, steps []StepResult, latest string, score *YearScore) {
	b.WriteString("# 模块12: 投资检查清单\n\n")
	b.WriteString("## 12.1 7项核心检查" + traceTrigger(3, 7, 10, 15, 16, 18) + "\n\n")

	roe := getStepValue(steps, 16, latest, "roe")
	gm := getStepValue(steps, 10, latest, "grossMargin")
	growth := getStepValue(steps, 9, latest, "growthRate")
	pg := getStepValue(steps, 16, latest, "profitGrowth")
	dr := getStepValue(steps, 3, latest, "debtRatio")
	cr := getStepValue(steps, 15, latest, "cashRatio")
	divRatio := getStepValue(steps, 18, latest, "ratio")

	checks := []struct {
		dim    string
		weight string
		item   string
		score  float64
		desc   string
	}{
		{"护城河", "15%", "竞争优势（毛利率/ROE）", mapScore(gm, 40, 20, 5), moatComment(gm, roe)},
		{"能力圈", "10%", "业务可理解（主业专注度）", mapScore(getStepValue(steps, 7, latest, "ratio"), 10, 30, 5), "投资类资产占比反映主业专注度"},
		{"安全边际", "20%", "估值有折扣（暂缺股价）", 3, "未接入实时股价，无法计算安全边际"},
		{"长期价值", "10%", "持续经营能力（分红/现金流）", mapScore(divRatio, 45, 70, 5), func() string {
			if divRatio == 0 {
				return "分红数据缺失或未实施现金分红"
			}
			return fmt.Sprintf("分红占比 %.1f%%，分红可持续性待观察", divRatio)
		}()},
		{"管理层", "10%", "诚信可靠（A-Score审计质量）", mapScore(100-getStepValue(steps, 8, latest, "AScore"), 40, 1.0, 5), fmt.Sprintf("A-Score=%.1f%s", getStepValue(steps, 8, latest, "AScore"), auditCommentAScore(getStepValue(steps, 8, latest, "AScore")))},
		{"财务稳健", "20%", "现金流健康/负债率低", (mapScore(dr, 60, 80, 5) + mapScore(cr, 100, 50, 5)) / 2, fmt.Sprintf("负债率%.1f%%，现金含量%.1f%%", dr, cr)},
		{"供需格局", "15%", "成长空间（营收增长率）", mapScore(growth, 20, 0, 5), growthComment(steps, latest)},
	}

	b.WriteString("| 维度 | 权重 | 检查项 | 评分 | 说明 |\n")
	b.WriteString("|------|------|--------|------|------|\n")
	total := 0.0
	for _, c := range checks {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %.1f/5 | %s |\n", c.dim, c.weight, c.item, c.score, c.desc))
		total += c.score
	}
	b.WriteString(fmt.Sprintf("| **总分** | 100%% | - | **%.1f/10** | %s |\n", total/3.5, buffettComment(steps, latest, score)))
	b.WriteString("\n")

	b.WriteString("## 12.2 关键否决项\n\n")
	if roe < 15 {
		b.WriteString(fmt.Sprintf("- ❌ ROE>15%%可持续？**存疑**（当前%.2f%%）\n", roe))
	} else {
		b.WriteString(fmt.Sprintf("- ✅ ROE>15%%可持续？**通过**（当前%.2f%%）\n", roe))
	}
	if growth < 0 || pg < 0 {
		b.WriteString(fmt.Sprintf("- ❌ 业绩正增长？**否**（营收%.2f%%，净利润%.2f%%）\n", growth, pg))
	} else {
		b.WriteString(fmt.Sprintf("- ✅ 业绩正增长？**是**（营收%.2f%%，净利润%.2f%%）\n", growth, pg))
	}
	if ascore := getStepValue(steps, 8, latest, "AScore"); ascore >= 60 {
		b.WriteString(fmt.Sprintf("- ❌ 财报可信？**存疑**（A-Score %.1f）\n", ascore))
	} else {
		b.WriteString(fmt.Sprintf("- ✅ 财报可信？**通过**（A-Score %.1f）\n", ascore))
	}
	b.WriteString("\n---\n\n")
}

// ========== 模块13: 社交媒体情绪监控 ==========
func writeModule12(b *strings.Builder, sentiment *SentimentData) {
	b.WriteString("# 模块13: 社交媒体情绪监控\n\n")

	if sentiment == nil || !sentiment.HasData {
		b.WriteString("> **说明**: 当前暂无可用舆情数据（网络受限或该股票近期无相关研报/资讯）。\n\n")
		b.WriteString("---\n\n")
		return
	}

	// 14.1 情绪指标
	b.WriteString("## 13.1 情绪指标\n\n")
	b.WriteString("| 指标 | 数值 | 说明 |\n")
	b.WriteString("|------|------|------|\n")

	scoreEmoji := "🟡"
	scoreDesc := "中性"
	if sentiment.Score > 0.3 {
		scoreEmoji = "🟢"
		scoreDesc = "偏多"
	} else if sentiment.Score < -0.3 {
		scoreEmoji = "🔴"
		scoreDesc = "偏空"
	}
	b.WriteString(fmt.Sprintf("| %s 情绪得分 | %.2f | %s |\n", scoreEmoji, sentiment.Score, scoreDesc))
	b.WriteString(fmt.Sprintf("| 📊 热度指数 | %d 条 | 近一年相关研报/资讯数量 |\n", sentiment.HeatIndex))
	b.WriteString(fmt.Sprintf("| ✅ 利好信号 | %d 个 | 命中正面关键词次数 |\n", len(sentiment.PositiveWords)))
	b.WriteString(fmt.Sprintf("| ⚠️ 风险信号 | %d 个 | 命中负面关键词次数 |\n", len(sentiment.NegativeWords)))
	b.WriteString("\n")

	// 14.2 关键词云
	if len(sentiment.PositiveWords) > 0 || len(sentiment.NegativeWords) > 0 {
		b.WriteString("## 13.2 关键词云\n\n")
		if len(sentiment.PositiveWords) > 0 {
			b.WriteString("**正面关键词**：" + strings.Join(sentiment.PositiveWords, "、") + "\n\n")
		}
		if len(sentiment.NegativeWords) > 0 {
			b.WriteString("**负面关键词**：" + strings.Join(sentiment.NegativeWords, "、") + "\n\n")
		}
	}

	// 14.3 最新舆情摘要
	if len(sentiment.Summaries) > 0 {
		b.WriteString("## 13.3 最新舆情摘要\n\n")
		for _, s := range sentiment.Summaries {
			emoji := "🟡"
			if s.Sentiment > 0.3 {
				emoji = "🟢"
			} else if s.Sentiment < -0.3 {
				emoji = "🔴"
			}
			b.WriteString(fmt.Sprintf("- %s **%s**（%s，%s）\n", emoji, s.Title, s.Source, s.Date))
		}
		b.WriteString("\n")
	}

	// 13.4 互动平台近期问答
	b.WriteString("## 13.4 互动平台近期问答\n\n")
	if len(sentiment.InteractQAs) > 0 {
		sourceName := "巨潮互动易"
		switch sentiment.InteractQAs[0].Source {
		case "guba":
			sourceName = "东方财富股吧"
		case "sse":
			sourceName = "上证e互动"
		}
		b.WriteString(fmt.Sprintf("> 已获取 **%d** 条互动平台问答（来源：%s），最近更新时间：%s\n\n", len(sentiment.InteractQAs), sourceName, sentiment.InteractQAs[0].Date))
		b.WriteString("`INTERACT_QA_PANEL`\n\n")
	} else {
		b.WriteString("> 当前暂无互动平台问答数据。\n\n")
	}

	b.WriteString("---\n\n")
}

// ========== 模块14: 综合投资建议 ==========
func writeModule13(b *strings.Builder, symbol string, steps []StepResult, latest string, score *YearScore, quote *QuoteData, rim *RIMData, technical *TechnicalData, ml *MLPredictionData, sentiment *SentimentData) {
	b.WriteString("# 模块14: 综合投资建议\n\n")

	weighted := 0.0
	if score != nil {
		weighted = score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
	}

	b.WriteString("## 14.1 综合评分汇总\n\n")
	b.WriteString("| 模块 | 权重 | 得分 | 加权分 |\n")
	b.WriteString("|------|------|------|--------|\n")
	if score != nil {
		b.WriteString(fmt.Sprintf("| 财务健康度综合评分 | 30%% | %.0f/100 | %.1f |\n", score.RawScore, score.RawScore*0.30))
		b.WriteString(fmt.Sprintf("| 盈利能力 | 15%% | %.0f/100 | %.1f |\n", profitScore(steps, latest), profitScore(steps, latest)*0.15))
		b.WriteString(fmt.Sprintf("| 现金流质量 | 15%% | %.0f/100 | %.1f |\n", cashScore(steps, latest), cashScore(steps, latest)*0.15))
		b.WriteString(fmt.Sprintf("| 成长能力 | 15%% | %.0f/100 | %.1f |\n", growthScore(steps, latest), growthScore(steps, latest)*0.15))
		b.WriteString(fmt.Sprintf("| 偿债安全 | 10%% | %.0f/100 | %.1f |\n", debtScore(steps, latest), debtScore(steps, latest)*0.10))
		b.WriteString(fmt.Sprintf("| 逆向检查 | 10%% | %.0f/100 | %.1f |\n", reverseScore(steps, latest, score), reverseScore(steps, latest, score)*0.10))
		b.WriteString(fmt.Sprintf("| 投资检查清单 | 5%% | %.0f/100 | %.1f |\n", buffettScore(steps, latest, score)*10, buffettScore(steps, latest, score)))
		b.WriteString(fmt.Sprintf("| **总分** | 100%% | - | **%.0f/100** |\n", weighted))
	}
	b.WriteString("\n")

	ascore := getStepValue(steps, 8, latest, "AScore")
	entryRange, stopLoss, target := formatTradeLevels(quote, rim, technical, ml, ascore)

	b.WriteString("## 14.2 投资建议\n\n")
	b.WriteString("| 项目 | 建议 |\n")
	b.WriteString("|------|------|\n")
	b.WriteString(fmt.Sprintf("| **综合评级** | %s |\n", investmentGrade(weighted)))
	b.WriteString(fmt.Sprintf("| **综合评分** | %.0f/100 %s |\n", weighted, scoreToStars(weighted)))
	b.WriteString(fmt.Sprintf("| **操作建议** | %s |\n", strategyAdvice(weighted)))
	b.WriteString(fmt.Sprintf("| **建议仓位** | %s |\n", positionAdvice(weighted)))
	b.WriteString(fmt.Sprintf("| **入场区间** | %s |\n", entryRange))
	b.WriteString(fmt.Sprintf("| **止损位** | %s |\n", stopLoss))
	b.WriteString(fmt.Sprintf("| **目标位** | %s |\n", target))
	if sentiment != nil && sentiment.HasData {
		b.WriteString(fmt.Sprintf("| **舆情情绪** | %s |\n", sentimentSummary(sentiment)))
	}
	b.WriteString("\n")

	b.WriteString("## 14.3 操作策略\n\n")
	if weighted >= 80 {
		b.WriteString("**策略A：积极配置（推荐）**\n")
		b.WriteString("- 基本面健康，可逢低分批建仓\n")
		b.WriteString("- 建议仓位 5-8%，长期持有\n")
		b.WriteString("- 若回撤 10% 可加仓\n\n")
		b.WriteString("**策略B：持有者**\n")
		b.WriteString("- 继续持有，关注 ROE 和毛利率稳定性\n")
	} else if weighted >= 70 {
		b.WriteString("**策略A：逢低试探（推荐）**\n")
		b.WriteString("- 财务整体尚可，存在部分短板\n")
		b.WriteString("- 建议仓位 3-5%，分批试探\n")
		b.WriteString("- 等待 A-Score 或毛利率改善后再加仓\n\n")
		b.WriteString("**策略B：持有者**\n")
		b.WriteString("- 维持现有仓位，观察关键指标修复情况\n")
	} else if weighted >= 60 {
		b.WriteString("**策略A：观望等待（推荐）**\n")
		b.WriteString("- 财务存在明显短板，暂不建仓\n")
		b.WriteString("- 等待年报数据持续改善后再介入\n\n")
		b.WriteString("**策略B：左侧试探（激进）**\n")
		b.WriteString("- 若对行业长期前景有信心，可轻仓 1-3% 试探\n")
		b.WriteString("- 严格止损\n")
	} else {
		b.WriteString("**策略A：回避（推荐）**\n")
		b.WriteString("- 财务风险较高，建议回避\n")
		b.WriteString("- 等待风险释放、基本面反转后再考虑\n\n")
		b.WriteString("**策略B：持有者**\n")
		b.WriteString("- 建议减仓或设置严格止损\n")
	}
	if sentiment != nil && sentiment.HasData {
		b.WriteString("\n> **舆情提示**：")
		if sentiment.Score > 0.3 {
			b.WriteString(fmt.Sprintf("近期舆情整体偏正面（热度 %d 条），可作为辅助参考，但不宜单独作为决策依据。\n", sentiment.HeatIndex))
		} else if sentiment.Score < -0.3 {
			b.WriteString(fmt.Sprintf("近期舆情整体偏负面（热度 %d 条），建议保持谨慎，关注后续公告与风险释放。\n", sentiment.HeatIndex))
		} else {
			b.WriteString(fmt.Sprintf("近期舆情整体中性（热度 %d 条），建议以基本面判断为主，持续跟踪舆情变化。\n", sentiment.HeatIndex))
		}
	}
	b.WriteString("\n---\n\n")
}

// formatTradeLevels 根据实时行情、RIM 估值、技术面与 ML 预测动态计算交易水位
func writeModule14(b *strings.Builder, symbol string, steps []StepResult, years []string, latest string, score *YearScore, sentiment *SentimentData) {
	b.WriteString("# 模块15: 结论与附录\n\n")

	weighted := 0.0
	if score != nil {
		weighted = score.RawScore*0.30 + profitScore(steps, latest)*0.25 + cashScore(steps, latest)*0.20 + growthScore(steps, latest)*0.15 + debtScore(steps, latest)*0.10
	}

	b.WriteString("## 15.1 核心结论\n\n")
	b.WriteString("> **")
	b.WriteString(fmt.Sprintf("%s %s年报", symbol, latest))
	b.WriteString(fmt.Sprintf(" 综合评分 %.0f 分，评级 %s。", weighted, investmentGrade(weighted)))
	b.WriteString(oneSentenceAdvice(symbol, weighted, steps, latest))
	if sentiment != nil && sentiment.HasData {
		b.WriteString(fmt.Sprintf(" 舆情方面：%s。", sentimentSummary(sentiment)))
	}
	b.WriteString("**\n\n")

	b.WriteString("## 15.2 关键数据速查\n\n")
	b.WriteString(fmt.Sprintf("| 指标 | %s | 同比 | 评估 |\n", latest))
	b.WriteString("|------|--------|------|------|\n")
	rev := getStepValue(steps, 9, latest, "revenue")
	prevRev := getStepValue(steps, 9, years[minInt(1, len(years)-1)], "revenue")
	b.WriteString(fmt.Sprintf("| 营业总收入 | %.2f亿 | %s | %s |\n", rev/1e8, yoyFmt(rev, prevRev), yoyEmoji(rev, prevRev)))
	prof := getStepValue(steps, 16, latest, "profit")
	prevProf := getStepValue(steps, 16, years[minInt(1, len(years)-1)], "profit")
	b.WriteString(fmt.Sprintf("| 归母净利润 | %.2f亿 | %s | %s |\n", prof/1e8, yoyFmt(prof, prevProf), yoyEmoji(prof, prevProf)))
	b.WriteString(fmt.Sprintf("| 毛利率 | %.2f%% | - | %s |\n", getStepValue(steps, 10, latest, "grossMargin"), gmEmoji(getStepValue(steps, 10, latest, "grossMargin"))))
	b.WriteString(fmt.Sprintf("| 资产负债率 | %.2f%% | - | %s |\n", getStepValue(steps, 3, latest, "debtRatio"), drEmoji(getStepValue(steps, 3, latest, "debtRatio"))))
	b.WriteString(fmt.Sprintf("| A-Score | %.1f | - | %s |\n", getStepValue(steps, 8, latest, "AScore"), asEmoji(getStepValue(steps, 8, latest, "AScore"))))
	if score != nil {
		b.WriteString(fmt.Sprintf("| 财报健康分 | %.0f分（%s） | - | - |\n", score.RawScore, score.Grade))
	}
	b.WriteString("\n")

	b.WriteString("## 15.3 投资逻辑总结\n\n")
	b.WriteString("**负面因素**:\n")
	risks := extractRisks(steps, latest)
	if len(risks) == 0 {
		b.WriteString("1. 未发现显著财务风险点\n")
	} else {
		for i, r := range risks {
			b.WriteString(fmt.Sprintf("%d. %s：%s\n", i+1, r.Category, r.Desc))
		}
	}
	if sentiment != nil && sentiment.HasData && sentiment.Score < -0.3 {
		b.WriteString(fmt.Sprintf("%d. 舆情情绪：近期舆情整体偏负面（热度 %d 条），需关注市场风险。\n", len(risks)+1, sentiment.HeatIndex))
	}
	b.WriteString("\n**正面因素**:\n")
	positives := positiveFactors(steps, latest)
	if len(positives) == 0 {
		b.WriteString("1. 当前数据中积极因素不明显\n")
	} else {
		for i, p := range positives {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
		}
	}
	if sentiment != nil && sentiment.HasData && sentiment.Score > 0.3 {
		start := len(positives) + 1
		if len(positives) == 0 {
			start = 1
			b.WriteString(fmt.Sprintf("%d. 舆情情绪：近期舆情整体偏正面（热度 %d 条），市场关注度较好。\n", start, sentiment.HeatIndex))
		} else {
			b.WriteString(fmt.Sprintf("%d. 舆情情绪：近期舆情整体偏正面（热度 %d 条），市场关注度较好。\n", start, sentiment.HeatIndex))
		}
	}
	b.WriteString("\n")

	b.WriteString("## 15.4 免责声明\n\n")
	b.WriteString("本报告基于公开财务报表数据及财报透视分析模型生成，仅供参考，不构成任何投资建议。投资有风险，入市需谨慎。\n\n")
}

// ==================== 辅助函数 ====================

type RiskItem struct {
	Category  string
	Indicator string
	Severity  string
	Desc      string
}

func writeModuleDiff(b *strings.Builder, diff *AnalysisDiff) {
	if diff == nil || !diff.HasPrevious {
		return
	}

	b.WriteString("# 模块1.3: 与上次分析对比\n\n")
	b.WriteString(fmt.Sprintf("> 上次分析时间：%s | 本次分析时间：%s\n\n", diff.PreviousTime, diff.CurrentTime))

	// 评分变化
	if diff.GradeChanged {
		b.WriteString(fmt.Sprintf("**综合评分变化：%+.0f 分，等级由 %s 变为 %s**\n\n",
			diff.ScoreChange, diff.PreviousGrade, diff.CurrentGrade))
	} else if diff.ScoreChange != 0 {
		b.WriteString(fmt.Sprintf("**综合评分变化：%+.0f 分，等级维持 %s**\n\n", diff.ScoreChange, diff.CurrentGrade))
	}

	// 风险变化
	hasRiskChanges := len(diff.NewFlags) > 0 || len(diff.ResolvedFlags) > 0 || len(diff.PersistentFlags) > 0
	if hasRiskChanges {
		b.WriteString("### 风险变化\n\n")
		if len(diff.NewFlags) > 0 {
			b.WriteString(fmt.Sprintf("- 🆕 **新增风险（%d项）**\n", len(diff.NewFlags)))
			for _, f := range diff.NewFlags {
				b.WriteString(fmt.Sprintf("  - %s %s\n", riskEmoji(f.Level), f.Format))
			}
		}
		if len(diff.ResolvedFlags) > 0 {
			b.WriteString(fmt.Sprintf("- ✅ **解除风险（%d项）**\n", len(diff.ResolvedFlags)))
			for _, f := range diff.ResolvedFlags {
				b.WriteString(fmt.Sprintf("  - %s\n", f.Format))
			}
		}
		if len(diff.PersistentFlags) > 0 {
			b.WriteString(fmt.Sprintf("- ⏸️ **持续风险（%d项）**\n", len(diff.PersistentFlags)))
			for _, f := range diff.PersistentFlags {
				b.WriteString(fmt.Sprintf("  - %s %s\n", riskEmoji(f.Level), f.Format))
			}
		}
		b.WriteString("\n")
	}

	// 关键指标变化
	if len(diff.KeyMetricChanges) > 0 {
		b.WriteString("### 关键指标变化\n\n")
		b.WriteString("| 指标 | 上次 | 本次 | 变化 |\n")
		b.WriteString("|------|------|------|------|\n")
		for _, m := range diff.KeyMetricChanges {
			sign := ""
			if m.Significant {
				if m.Delta > 0 {
					sign = " 🟢"
				} else {
					sign = " 🔴"
				}
			}
			b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %+.2f%s |\n", m.Name, m.Previous, m.Current, m.Delta, sign))
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
}

func writeAuditOpinion(b *strings.Builder, steps []StepResult, latest string, riskAlert *RiskAlertSummary) {
	step1 := findStepResult(steps, 1)
	if step1 == nil || step1.YearlyData == nil {
		return
	}
	yd, ok := step1.YearlyData[latest]
	if !ok {
		return
	}
	opinion, _ := yd["opinion"].(string)
	auditor, _ := yd["auditor"].(string)
	isStandardVal := yd["isStandard"]
	var isStandard bool
	if b, ok := isStandardVal.(bool); ok {
		isStandard = b
	}
	if opinion == "" || opinion == "请查询年报确认" {
		return
	}

	b.WriteString("## 1.0 审计意见\n\n")
	b.WriteString("| 项目 | 内容 |\n")
	b.WriteString("|------|------|\n")
	if isStandard {
		b.WriteString(fmt.Sprintf("| 审计意见 | ✅ %s |\n", opinion))
	} else {
		b.WriteString(fmt.Sprintf("| 审计意见 | ⚠️ %s |\n", opinion))
	}
	b.WriteString(fmt.Sprintf("| 审计机构 | %s |\n", auditor))
	isTop10Val := yd["isTop10"]
	if isTop10, ok := isTop10Val.(bool); ok {
		if isTop10 {
			b.WriteString("| 事务所资质 | ✅ 属十大审计机构 |\n")
		} else {
			b.WriteString("| 事务所资质 | ⚠️ 非十大审计机构 |\n")
		}
	}
	b.WriteString("\n")

	// 审计机构正常轮换提示
	if riskAlert != nil {
		for _, f := range riskAlert.Flags {
			if f.Code == "auditor_rotation" && f.Level == "info" {
				b.WriteString("> ℹ️ **审计机构正常轮换提示**：")
				if f.Note != "" {
					b.WriteString(f.Note)
				} else {
					b.WriteString("该审计机构更换看似异常，实则属于正常轮换，不属于公司自身风险信号。")
				}
				b.WriteString("\n\n")
			}
		}
	}
}

// findStepResult 从 steps 中按 stepNum 查找
func writeModuleQuarterly(b *strings.Builder, alert *QuarterlyAlert) {
	if alert == nil || !alert.HasData || len(alert.Items) == 0 {
		return
	}
	b.WriteString("## 3.3 季度滚动预警（环比+同比）\n\n")
	b.WriteString("| 期间 | 对比类型 | 指标 | 当前值 | 对比基准 | 变化 | 状态 |\n")
	b.WriteString("|------|----------|------|--------|----------|------|------|\n")
	for _, item := range alert.Items {
		status := "🟡"
		if item.Level == "danger" {
			status = "🔴"
		}
		// 毛利率/净利率显示为百分点变化，其他显示为百分比变化
		changeStr := fmt.Sprintf("%+.1f%%", item.ChangePct*100)
		if item.Metric == "毛利率" {
			changeStr = fmt.Sprintf("%+.1fpp", item.ChangePct*100)
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %.2f | %.2f | %s | %s %s |\n",
			item.Period, item.CompareType, item.Metric, item.Current, item.Previous, changeStr, status, item.Description))
	}
	b.WriteString("\n")
}

// writeModuleTTM TTM 滚动指标模块
func writeModuleTTM(b *strings.Builder, metrics *TTMMetrics) {
	if metrics == nil || !metrics.HasData {
		return
	}
	b.WriteString("## 3.4 TTM（滚动12个月）数据\n\n")
	b.WriteString("> TTM = Trailing Twelve Months，即最近连续12个月的累计数据。对于A股，通常为最近4个季度财报数据累加，比单季度更能反映企业持续经营能力。\n\n")
	b.WriteString(metrics.FormatTTMReport())
}
