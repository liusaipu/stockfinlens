package analyzer

import (
	"fmt"
	"strings"
)

// writeAScoreProfile 输出 A-Score 综合风险画像（模块7，位于 RIM 之后、技术面之前）
func writeAScoreProfile(b *strings.Builder, steps []StepResult, years []string, latest string, comp *ComparableAnalysis) {
	b.WriteString("# 模块7: A-Score 综合风险画像\n\n")
	b.WriteString("A-Score 是专为 A 股市场构建的多维度综合风险评分（0-100，越高越危险），融合 Beneish M-Score（财务造假风险）、Altman Z-Score（破产/偿债风险）、现金流偏离度、应收账款异常度、毛利率异常波动，以及股权质押、监管问询、大股东减持等非财务信号。\n\n")

	step8 := steps[7]
	yd, ok := step8.YearlyData[latest]
	if !ok || yd == nil {
		b.WriteString("> **说明**: 当前数据不足以计算 A-Score。\n\n")
		b.WriteString("---\n\n")
		return
	}
	if _, hasAScore := yd["AScore"]; !hasAScore {
		b.WriteString("> **说明**: 当前数据不足以计算 A-Score。\n\n")
		b.WriteString("---\n\n")
		return
	}

	ms := anyToFloat64(yd["MScore"])
	zs := anyToFloat64(yd["ZScore"])
	mRisk := anyToFloat64(yd["MRisk"])
	zRisk := anyToFloat64(yd["ZRisk"])
	cd := anyToFloat64(yd["CashDev"])
	ar := anyToFloat64(yd["ARRisk"])
	gm := anyToFloat64(yd["GMRisk"])
	crawler := anyToFloat64(yd["CrawlerRisk"])
	as := anyToFloat64(yd["AScore"])

	// 1. 总览横幅
	b.WriteString(fmt.Sprintf("> **%s** | **A-Score = %.1f** | %s\n\n", ascoreBadge(as), as, ascoreBrief(as)))

	// 2. 核心子指标概览（原模块3.5内容）
	b.WriteString("## 7.1 A-Score 核心子指标概览\n\n")
	b.WriteString("| 子指标 | 数值 | 风险说明 |\n")
	b.WriteString("|--------|------|----------|\n")
	b.WriteString(fmt.Sprintf("| **M-Score** | %.3f | Beneish 财务造假风险指标 |\n", ms))
	b.WriteString(fmt.Sprintf("| **Z-Score** | %.2f | Altman 破产风险评分 |\n", zs))
	b.WriteString(fmt.Sprintf("| **现金流风险分** | %.1f %s | 净利润与经营现金流背离程度（负值=现金流优于利润） |\n", cd, cashDevStatus(cd)))
	b.WriteString(fmt.Sprintf("| **应收账款异常度** | %.1f%% | 应收增速 vs 营收增速偏离 |\n", ar))
	b.WriteString(fmt.Sprintf("| **毛利率异常波动** | %.1f%% | 毛利率连续恶化信号 |\n", gm))
	b.WriteString(fmt.Sprintf("| **A-Score（综合）** | **%.1f** | **%s** |\n", as, ascoreComment(as)))
	b.WriteString("\n")

	// 3. 六维雷达分解
	b.WriteString("## 7.2 A-Score 六维风险分解\n\n")
	b.WriteString("| 维度 | 原始指标 | 风险分 | 权重 | 评估 |\n")
	b.WriteString("|------|----------|--------|------|------|\n")
	b.WriteString(fmt.Sprintf("| **M-Score（造假风险）** | %.3f | %.1f | 15%% | %s |\n", ms, mRisk, riskLevel(mRisk)))
	b.WriteString(fmt.Sprintf("| **Z-Score（破产风险）** | %.2f | %.1f | 20%% | %s |\n", zs, zRisk, riskLevel(zRisk)))
	b.WriteString(fmt.Sprintf("| **现金流风险分** | %.1f | %.1f | 20%% | %s |\n", cd, normalizeCashDev(cd), riskLevel(normalizeCashDev(cd))))
	b.WriteString(fmt.Sprintf("| **应收账款异常** | %.1f%% | %.1f | 15%% | %s |\n", ar, ar, riskLevel(ar)))
	b.WriteString(fmt.Sprintf("| **毛利率异常波动** | %.1f%% | %.1f | 10%% | %s |\n", gm, gm, riskLevel(gm)))
	b.WriteString(fmt.Sprintf("| **非财务信号** | — | %.1f | 20%% | %s |\n", crawler, riskLevel(crawler)))
	b.WriteString(fmt.Sprintf("| **A-Score（综合）** | — | **%.1f** | 100%% | **%s** |\n", as, riskLevel(as)))
	b.WriteString("\n")

	// 4. 历史趋势
	if len(years) >= 2 {
		b.WriteString("## 7.3 A-Score 历史趋势（近5年）\n\n")
		b.WriteString("| 年度 | A-Score | 趋势 | 状态 |\n")
		b.WriteString("|------|---------|------|------|\n")
		var prevAScore float64 = -1
		for i := 0; i < len(years) && i < 5; i++ {
			year := years[i]
			if yd2, ok := step8.YearlyData[year]; ok && yd2 != nil {
				if v, ok2 := yd2["AScore"].(float64); ok2 {
					trend := "—"
					if prevAScore >= 0 {
						diff := v - prevAScore
						if diff > 3 {
							trend = "↑ 上升"
						} else if diff < -3 {
							trend = "↓ 下降"
						} else {
							trend = "→ 持平"
						}
					}
					prevAScore = v
					b.WriteString(fmt.Sprintf("| %s | %.1f | %s | %s |\n", year, v, trend, ascoreBrief(v)))
				}
			}
		}
		b.WriteString("\n")
	}

	// 5. 同行业对比
	if comp != nil && comp.HasData && len(comp.Metrics) > 0 {
		var compAScores []float64
		for _, m := range comp.Metrics {
			if m.MScore != 0 {
				// 可比公司只有 M-Score，我们用 M-Score 估算一个简化 A-Score 做对比
				compAScores = append(compAScores, mapMScoreToRisk(m.MScore))
			}
		}
		if len(compAScores) > 0 {
			avg := 0.0
			for _, v := range compAScores {
				avg += v
			}
			avg /= float64(len(compAScores))
			b.WriteString("## 7.4 同行业 A-Score 参考（基于 M-Score 估算）\n\n")
			if len(compAScores) < 3 {
				// 样本太少时均值无统计意义，只给提示不下结论
				b.WriteString(fmt.Sprintf("> ⚠️ 可比样本不足（仅 %d 家，建议配置 3~5 家），同行业 A-Score 均值暂不具备参考价值，本节不下结论。\n\n", len(compAScores)))
			} else {
				b.WriteString("| 指标 | 当前公司 | 可比均值 | 差异 |\n")
				b.WriteString("|------|----------|----------|------|\n")
				diff := as - avg
				b.WriteString(fmt.Sprintf("| **A-Score** | %.1f | %.1f | %+.1f |\n", as, avg, diff))
				if diff > 10 {
					b.WriteString("> ⚠️ 当前公司 A-Score 显著高于可比均值，财务风险相对偏大。\n\n")
				} else if diff < -10 {
					b.WriteString("> ✅ 当前公司 A-Score 低于可比均值，财务风险相对可控。\n\n")
				} else {
					b.WriteString("> 当前公司 A-Score 与可比均值接近。\n\n")
				}
			}
		}
	}

	// 6. 调优建议
	b.WriteString("## 7.5 A-Score 调优建议\n\n")
	var tips []string
	if mRisk >= 60 {
		tips = append(tips, "M-Score 偏高：重点关注应收账款增速、收入确认政策及费用资本化情况，建议核查审计意见。")
	}
	if zRisk >= 60 {
		tips = append(tips, "Z-Score 偏低：偿债能力或营运资金存在压力，建议关注流动比率、速动比率及短期借款结构。")
	}
	if cd >= 40 {
		tips = append(tips, "现金流偏离度大：净利润现金含量不足，盈利质量存疑，警惕赊销驱动型增长。")
	}
	if ar >= 60 {
		tips = append(tips, "应收账款异常：应收增速显著高于营收增速，存在收入虚增或回款恶化风险。")
	}
	if gm >= 60 {
		tips = append(tips, "毛利率异常波动：毛利率连续下滑或急剧波动，需排查成本核算、关联交易定价及产品竞争力变化。")
	}
	if as >= 70 {
		tips = append(tips, "综合风险高：建议降低仓位或等待风险释放后再介入，优先排查上述高亮子项。")
	} else if as >= 50 && len(tips) == 0 {
		tips = append(tips, "A-Score 处于中等区间，无明显单项短板，建议持续跟踪现金流与应收变化。")
	} else if as < 40 && len(tips) == 0 {
		tips = append(tips, "A-Score 健康，财务风险可控，可更多关注成长性与估值匹配度。")
	}
	if len(tips) == 0 {
		tips = append(tips, "A-Score 整体可控，建议结合行业景气度与估值进一步决策。")
	}
	for _, t := range tips {
		b.WriteString(fmt.Sprintf("- %s\n", t))
	}
	b.WriteString("\n---\n\n")
}

func normalizeCashDev(cd float64) float64 {
	// 把 [-20,80] 映射到 [0,100] 用于展示
	v := cd + 20.0
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}

func cashDevStatus(cd float64) string {
	if cd < 0 {
		return "🟢 优秀"
	}
	if cd < 20 {
		return "🟢 健康"
	}
	if cd < 40 {
		return "🟡 关注"
	}
	if cd < 60 {
		return "🟡 偏高"
	}
	return "🔴 高风险"
}

func riskLevel(v float64) string {
	if v >= 70 {
		return "🔴 高"
	}
	if v >= 50 {
		return "🟡 中"
	}
	if v >= 30 {
		return "🟢 低"
	}
	return "🟢 很低"
}

func ascoreBrief(v float64) string {
	if v >= 70 {
		return "综合风险较高，建议深入核查"
	}
	if v >= 60 {
		return "综合风险中等，需保持关注"
	}
	if v >= 40 {
		return "综合风险可控"
	}
	return "综合风险低，基本面稳健"
}
