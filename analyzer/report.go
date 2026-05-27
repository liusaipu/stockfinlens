package analyzer

import (
	"fmt"
	"strings"
)

// GenerateMarkdown 生成增强版 Markdown 格式的投资分析报告（14模块标准框架）
func GenerateMarkdown(symbol string, years []string, steps []StepResult, scores map[string]*YearScore, comp *ComparableAnalysis, industry *IndustryComparison, quote *QuoteData, sentiment *SentimentData, policy *PolicyMatchData, technical *TechnicalData, activity *ActivityData, moneyflow *MoneyflowData, ml *MLPredictionData, rim *RIMData, riskAlert *RiskAlertSummary, qualityWarnings []string, diff *AnalysisDiff, quarterlyAlert *QuarterlyAlert, ttmMetrics *TTMMetrics) string {
	if len(years) == 0 {
		return "# 无数据\n\n未找到可用的财务数据。"
	}
	latest := years[0]
	prev := ""
	if len(years) > 1 {
		prev = years[1]
	}
	latestScore := scores[latest]

	var b strings.Builder

	// ==================== 封面 ====================
	b.WriteString(fmt.Sprintf("# %s 深度投资分析报告\n\n", symbol))
	b.WriteString(fmt.Sprintf("**股票代码**: %s  \n", symbol))
	b.WriteString("**分析框架**: 财报透视分析框架  \n")
	b.WriteString(fmt.Sprintf("**最新报告期**: %s  \n", latest))
	b.WriteString(fmt.Sprintf("**数据基础**: 基于 %d 年财务报表数据（%s ~ %s）\n\n", len(years), latest, years[len(years)-1]))
	b.WriteString("---\n\n")

	// ==================== 数据质量警告（新增）====================
	if len(qualityWarnings) > 0 {
		writeDataQualityWarnings(&b, qualityWarnings)
	}

	// ==================== 风险警示横幅（新增）====================
	if riskAlert != nil {
		writeRiskAlertBanner(&b, riskAlert)
	}

	// ==================== 重大风险提示 ====================
	// 当风险警示已覆盖主要风险时，跳过重大风险提示模块，避免内容重叠
	if riskAlert == nil || riskAlert.Level == "low" {
		writeMajorRisks(&b, symbol, steps, latest, prev, latestScore)
	}

	// ==================== 模块1: 执行摘要 ====================
	writeModule1(&b, symbol, steps, latest, prev, latestScore, quote, technical, activity, ml, riskAlert)

	// ==================== 模块1.3: 与上次分析对比 ====================
	if diff != nil && diff.HasPrevious {
		writeModuleDiff(&b, diff)
	}

	// ==================== 模块2: 换手率深度分析 ====================
	writeModule2(&b, quote)

	// ==================== 模块3: 公司基本面分析 ====================
	writeModule3(&b, steps, years, latest, prev, quarterlyAlert, ttmMetrics)

	// ==================== 模块4: 行业横向对比分析 ====================
	activityScore := -1.0
	if activity != nil {
		activityScore = activity.Score
	}
	writeModule4(&b, steps, latest, comp, industry, activityScore)

	// ==================== 模块5: 十五五政策匹配度评估 ====================
	writeModule5(&b, policy)

	// ==================== 模块6: 剩余收益模型估值(RIM) ====================
	writeModule6(&b, steps, latest, quote, rim)

	// ==================== 模块7: A-Score 综合风险画像 ====================
	writeAScoreProfile(&b, steps, years, latest, comp)

	// ==================== 模块8: 技术面分析 ====================
	writeModule7(&b, quote, technical, activity, moneyflow)

	// ==================== 模块9: ML机器学习预测 ====================
	writeModule8(&b, steps, latest, prev, ml)

	// ==================== 模块10: 智能选股7大条件 ====================
	writeModule9(&b, steps, latest, prev)

	// ==================== 模块11: 逆向思维检查 ====================
	writeModule10(&b, steps, latest, latestScore)

	// ==================== 模块12: 投资检查清单 ====================
	writeModule11(&b, steps, latest, latestScore)

	// ==================== 模块13: 社交媒体情绪监控 ====================
	writeModule12(&b, sentiment)

	// ==================== 模块14: 综合投资建议 ====================
	writeModule13(&b, symbol, steps, latest, latestScore, quote, rim, technical, ml, sentiment)

	// ==================== 模块15: 结论与附录 ====================
	writeModule14(&b, symbol, steps, years, latest, latestScore, sentiment)

	b.WriteString("---\n\n")
	b.WriteString("*报告生成时间：基于最新导入的财务数据*\n")

	return b.String()
}

// ========== 风险警示横幅（新增）==========
func writeRiskAlertBanner(b *strings.Builder, alert *RiskAlertSummary) {
	if alert == nil || alert.Level == "low" {
		return
	}

	// 统计 high/medium 数量，混合时标题显示"中高风险"
	highCount := 0
	mediumCount := 0
	for _, f := range alert.Flags {
		if f.Level == "high" {
			highCount++
		} else if f.Level == "medium" {
			mediumCount++
		}
	}
	levelTitle := "🔴 高风险警示"
	if alert.Level == "medium" {
		levelTitle = "🟡 中风险警示"
	} else if alert.Level == "high" && mediumCount > 0 {
		levelTitle = "🔴 中高风险警示"
	}
	if alert.OneVeto {
		levelTitle += "（一票否决）"
	}

	// 标题
	totalFlags := len(alert.Flags)
	b.WriteString(fmt.Sprintf("### %s（共%d项）\n\n", levelTitle, totalFlags))

	// 风险项表格
	if len(alert.Flags) > 0 {
		b.WriteString("| 警示 | 风险指标 | 数值说明 | 等级 |\n")
		b.WriteString("|:----:|----------|----------|------|\n")
		for _, f := range alert.Flags {
			flagIcon := "🟡"
			flagLevel := "中风险"
			if f.Level == "high" {
				flagIcon = "🔴"
				flagLevel = "高风险"
			} else if f.Level == "info" {
				flagIcon = "ℹ️"
				flagLevel = "信息"
			}
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", flagIcon, f.Name, f.FormatFlagValue(), flagLevel))
		}
		b.WriteString("\n")
	}

	// 一票否决提示
	if alert.OneVeto {
		b.WriteString("> ⚠️ **一票否决**: 该股票触发了排雷检查清单中的铁律指标，建议深入核查后再做投资决策。\n\n")
	}

	b.WriteString("---\n\n")
}

// ========== 重大风险提示 ==========
func writeMajorRisks(b *strings.Builder, symbol string, steps []StepResult, latest, prev string, score *YearScore) {
	var risks []string
	ascore := getStepValue(steps, 8, latest, "AScore")
	if ascore >= 60 {
		if ascore >= 70 {
			risks = append(risks, fmt.Sprintf("**A-Score 偏高**: %.1f，综合财务风险较高 🔴**", ascore))
		} else {
			risks = append(risks, fmt.Sprintf("**A-Score 中等**: %.1f，需关注财务健康度 ⚠️**", ascore))
		}
	}
	growth := getStepValue(steps, 9, latest, "growthRate")
	if growth < 0 {
		risks = append(risks, fmt.Sprintf("**营收负增长**: %.2f%%，成长性承压 ❌**", growth))
	}
	pg := getStepValue(steps, 16, latest, "profitGrowth")
	if pg < -20 {
		risks = append(risks, fmt.Sprintf("**净利润大幅下滑**: %.2f%% ❌**", pg))
	} else if pg < 0 {
		risks = append(risks, fmt.Sprintf("**净利润负增长**: %.2f%% ❌**", pg))
	}
	roe := getStepValue(steps, 16, latest, "roe")
	if roe < 10 {
		risks = append(risks, fmt.Sprintf("**ROE 偏低**: %.2f%%，资本回报率不足 ❌**", roe))
	}
	gm := getStepValue(steps, 10, latest, "grossMargin")
	if gm < 20 {
		risks = append(risks, fmt.Sprintf("**毛利率过低**: %.2f%%，产品竞争力弱 ❌**", gm))
	}
	dr := getStepValue(steps, 3, latest, "debtRatio")
	if dr > 60 {
		risks = append(risks, fmt.Sprintf("**负债率过高**: %.2f%%，偿债压力大 🔴**", dr))
	}
	cashRatio := getStepValue(steps, 15, latest, "cashRatio")
	if cashRatio < 50 && cashRatio != 0 {
		risks = append(risks, fmt.Sprintf("**现金流质量偏弱**: 净利润现金含量 %.2f%% ⚠️**", cashRatio))
	}

	if len(risks) > 0 || (score != nil && score.RawScore < 70) {
		b.WriteString("# ⚠️ 风险提示\n\n")
		for _, r := range risks {
			b.WriteString(fmt.Sprintf("- %s\n", r))
		}
		if score != nil && score.RawScore < 70 {
			b.WriteString(fmt.Sprintf("- **综合评分偏低**: %.0f分（%s），财务健康度需关注\n", score.RawScore, score.Grade))
		}
		b.WriteString("\n---\n\n")
	}
}

// ========== 目录 ==========
func writeTOC(b *strings.Builder) {
	b.WriteString("# 目录\n\n")
	b.WriteString("- [模块1: 执行摘要](#模块1-执行摘要)\n")
	b.WriteString("- [模块2: 换手率深度分析](#模块2-换手率深度分析)\n")
	b.WriteString("- [模块3: 公司基本面分析](#模块3-公司基本面分析)\n")
	b.WriteString("- [模块4: 行业横向对比分析](#模块4-行业横向对比分析)\n")
	b.WriteString("- [模块5: 十五五政策匹配度评估](#模块5-十五五政策匹配度评估)\n")
	b.WriteString("- [模块6: 剩余收益模型估值(RIM)](#模块6-剩余收益模型估值rim)\n")
	b.WriteString("- [模块7: A-Score 综合风险画像](#模块7-a-score-综合风险画像)\n")
	b.WriteString("- [模块8: 技术面分析](#模块8-技术面分析)\n")
	b.WriteString("- [模块9: ML机器学习预测](#模块9-ml机器学习预测)\n")
	b.WriteString("- [模块10: 智能选股7大条件](#模块10-智能选股7大条件)\n")
	b.WriteString("- [模块11: 逆向思维检查](#模块11-逆向思维检查)\n")
	b.WriteString("- [模块12: 投资检查清单](#模块12-投资检查清单)\n")
	b.WriteString("- [模块13: 社交媒体情绪监控](#模块13-社交媒体情绪监控)\n")
	b.WriteString("- [模块14: 综合投资建议](#模块14-综合投资建议)\n")
	b.WriteString("- [模块15: 结论与附录](#模块15-结论与附录)\n")
	b.WriteString("\n---\n\n")
}

// ========== 8项核心指标高亮 ==========
func writeEightIndicatorsHighlight(b *strings.Builder, steps []StepResult, latest string) {
	indicators := []struct {
		name      string
		value     float64
		unit      string
		passed    bool
		operator  string
		threshold float64
	}{
		{"ROE", getStepValue(steps, 16, latest, "roe"), "%", getStepValue(steps, 16, latest, "roe") > 20, ">", 20},
		{"净利润现金比率", getStepValue(steps, 15, latest, "cashRatio"), "%", getStepValue(steps, 15, latest, "cashRatio") > 100, ">", 100},
		{"资产负债率", getStepValue(steps, 3, latest, "debtRatio"), "%", getStepValue(steps, 3, latest, "debtRatio") < 60, "<", 60},
		{"毛利率", getStepValue(steps, 10, latest, "grossMargin"), "%", getStepValue(steps, 10, latest, "grossMargin") > 40, ">", 40},
		{"营业利润率", getStepValue(steps, 14, latest, "coreProfitMargin"), "%", getStepValue(steps, 14, latest, "coreProfitMargin") > 20, ">", 20},
		{"营业收入增长率", getStepValue(steps, 9, latest, "growthRate"), "%", getStepValue(steps, 9, latest, "growthRate") > 10, ">", 10},
		{"固定资产比率", getStepValue(steps, 6, latest, "ratio"), "%", getStepValue(steps, 6, latest, "ratio") < 40, "<", 40},
		{"分红占经营现金流比", getStepValue(steps, 18, latest, "ratio"), "%", getStepValue(steps, 18, latest, "ratio") >= 20 && getStepValue(steps, 18, latest, "ratio") <= 70, "20%~", 70},
	}

	matchCount := 0
	for _, ind := range indicators {
		if ind.passed {
			matchCount++
		}
	}

	b.WriteString("## 核心指标一览\n\n")
	if matchCount >= 5 {
		b.WriteString("> 🏆 **核心指标亮点**：该企业 8 项核心财务指标中满足 **" + fmt.Sprintf("%d", matchCount) + " 项**，表现优异，值得重点关注。\n\n")
		b.WriteString("> **达标指标**：\n")
		for _, ind := range indicators {
			if ind.passed {
				b.WriteString(fmt.Sprintf("> - ✅ **%s**：%.2f%s（%s %.0f%s）\n", ind.name, ind.value, ind.unit, ind.operator, ind.threshold, ind.unit))
			}
		}
		if matchCount < 8 {
			b.WriteString("> \n")
			b.WriteString("> **未达标指标**：\n")
			for _, ind := range indicators {
				if !ind.passed {
					b.WriteString(fmt.Sprintf("> - ⚠️ **%s**：%.2f%s（%s %.0f%s）\n", ind.name, ind.value, ind.unit, ind.operator, ind.threshold, ind.unit))
				}
			}
		}
		b.WriteString("\n")
	} else {
		b.WriteString("| 指标 | 数值 | 阈值 | 是否达标 |\n")
		b.WriteString("|------|------|------|----------|\n")
		for _, ind := range indicators {
			status := "❌ 未达标"
			if ind.passed {
				status = "✅ 达标"
			}
			b.WriteString(fmt.Sprintf("| **%s** | %.2f%s | %s %.0f%s | %s |\n", ind.name, ind.value, ind.unit, ind.operator, ind.threshold, ind.unit, status))
		}
		b.WriteString(fmt.Sprintf("\n**达标比例**：%d / 8 项\n\n", matchCount))
	}

	// 添加 A-Score 风险评分
	ascore := getStepValue(steps, 8, latest, "AScore")
	b.WriteString("### A-Score 风险评分\n\n")
	b.WriteString(fmt.Sprintf("| 指标 | 数值 | 风险等级 | 说明 |\n"))
	b.WriteString("|------|------|----------|------|\n")
	if ascore > 0 {
		badge := ascoreBadge(ascore)
		comment := ascoreBrief(ascore)
		b.WriteString(fmt.Sprintf("| **A-Score** | **%.1f** | %s | %s |\n", ascore, badge, comment))
	} else {
		b.WriteString("| **A-Score** | — | — | 数据不足，无法计算 |\n")
	}
	b.WriteString("\n")
}

// ========== 模块1: 执行摘要 ==========
func writeDataQualityWarnings(b *strings.Builder, warnings []string) {
	if len(warnings) == 0 {
		return
	}

	var critical []string // 可能影响分析结果的问题
	var minor []string    // 数据源精度/舍入问题，不影响核心指标

	for _, w := range warnings {
		if strings.Contains(w, "数据源精度") || strings.Contains(w, "舍入") || strings.Contains(w, "不平衡") {
			minor = append(minor, w)
		} else {
			critical = append(critical, w)
		}
	}

	b.WriteString("## ⚠️ 数据质量提示\n\n")
	b.WriteString("> 财务数据来自东方财富 API（免费数据源），部分科目存在精度/舍入差异，属第三方数据源限制，非分析模型错误。\n\n")

	if len(critical) > 0 {
		b.WriteString("**以下问题可能影响分析准确性，建议关注：**\n\n")
		for _, w := range critical {
			b.WriteString(fmt.Sprintf("- 🔴 %s\n", w))
		}
		b.WriteString("\n")
	}

	if len(minor) > 0 {
		b.WriteString("**以下差异为数据源精度/舍入问题，不影响 ROE、毛利率、增长率等核心指标：**\n\n")
		for _, w := range minor {
			b.WriteString(fmt.Sprintf("- 🟡 %s\n", w))
		}
		b.WriteString("\n")
	}

	b.WriteString("> 💡 **建议**：如对本数据有疑虑，可通过「导入财报」功能手动导入同花顺/东方财富下载的原始 CSV/Excel 财报进行复核。\n\n")
	b.WriteString("---\n\n")
}

// writeModuleDiff 在报告中展示与上次分析的对比