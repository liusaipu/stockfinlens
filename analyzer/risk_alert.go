package analyzer

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// BuildRiskAlertSummary 从财报分析结果中构建风险警示摘要
// sensitivity: 敏感度级别，影响阈值
func BuildRiskAlertSummary(steps []StepResult, extras map[string]float64, years []string, external *ExternalRiskData, sensitivity SensitivityLevel) *RiskAlertSummary {
	if len(years) == 0 {
		return &RiskAlertSummary{Level: "low", PrimaryMsg: "暂无足够数据进行风险评估"}
	}

	latest := years[0]
	var prev string
	if len(years) > 1 {
		prev = years[1]
	}

	// 获取敏感度阈值
	thresholds := getSensitivityThresholds(sensitivity)

	// Helper: 从 steps 中按 stepNum 查找
	findStep := func(num int) *StepResult {
		for i := range steps {
			if steps[i].StepNum == num {
				return &steps[i]
			}
		}
		return nil
	}

	// Helper: 读取某一步某年份的 float 值
	getFloat := func(step *StepResult, year, key string) float64 {
		if step == nil || step.YearlyData == nil {
			return 0
		}
		yd, ok := step.YearlyData[year]
		if !ok {
			return 0
		}
		v, ok := yd[key]
		if !ok {
			return 0
		}
		if vf, ok2 := v.(float64); ok2 {
			return vf
		}
		return 0
	}

	// Helper: 读取某一步某年份的 string 值
	getString := func(step *StepResult, year, key string) string {
		if step == nil || step.YearlyData == nil {
			return ""
		}
		yd, ok := step.YearlyData[year]
		if !ok {
			return ""
		}
		v, ok := yd[key]
		if !ok {
			return ""
		}
		if vs, ok2 := v.(string); ok2 {
			return vs
		}
		return ""
	}

	summary := &RiskAlertSummary{
		Level:   "low",
		Flags:   []RiskAlertFlag{},
		OneVeto: false,
	}

	// ========== 一票否决检查 ==========

	// 1. 审计意见非标
	step1 := findStep(1)
	if step1 != nil {
		auditOpinion := getString(step1, latest, "opinion")
		// 只有当审计意见有明确值且为非标时才触发（排除占位符/未查询状态）
		if auditOpinion != "" && auditOpinion != "标准无保留意见" && auditOpinion != "请查询年报确认" && auditOpinion != "待确认" {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "audit_nonstandard",
				Name:    "审计意见非标",
				Level:   "high",
				Source:  "step1",
				Format:  fmt.Sprintf("审计意见: %s", auditOpinion),
				Details: []string{fmt.Sprintf("审计意见: %s", auditOpinion), "非标审计意见意味着财务报表可能存在重大不确定性或错误"},
			})
			summary.OneVeto = true
		}
	}

	// 2. 审计机构变更（外部数据）
	// 四层判断：
	// - 正常轮换（招标选聘/连续服务多年/无分歧等）+ 非年报前 → info 提示，不触发风险
	// - 政策合规更换（如国企8年强制轮换）+ 非年报前 → info 提示，不触发风险
	// - 被动更换（原事务所被处罚/禁入等）+ 非年报前 → info 提示，不触发风险
	// - 年报披露期内更换 或 明确异常（辞任/解聘/意见分歧）→ 高风险 + 一票否决
	// - 正常轮换但发生在年报期内 → 中风险（提示注意时机，但不一票否决）
	// - 其他情况（原因模糊、未说明具体原因）→ 中风险，不触发一票否决
	if external != nil && external.AuditorChanged {
		var highRiskChanges []AuditorChangeDetail
		var mediumRiskChanges []AuditorChangeDetail
		var infoChanges []AuditorChangeDetail

		for _, cd := range external.AuditorChangeDetails {
			if cd.IsNormalRotation && !cd.IsBeforeAnnualReport {
				// 正常轮换 + 非年报前 = 看似异常实则正常
				infoChanges = append(infoChanges, cd)
			} else if cd.IsPolicyCompliance && !cd.IsBeforeAnnualReport {
				// 政策合规 + 非年报前 = 合规轮换
				infoChanges = append(infoChanges, cd)
			} else if cd.IsPassiveChange && !cd.IsBeforeAnnualReport {
				// 被动更换（原事务所被处罚/禁入等）+ 非年报前 = 非公司自身问题
				infoChanges = append(infoChanges, cd)
			} else if cd.IsBeforeAnnualReport || cd.IsAbnormal {
				// 年报披露期内更换 或 明确异常信号（辞任/解聘/意见分歧等）
				highRiskChanges = append(highRiskChanges, cd)
			} else if cd.IsNormalRotation && cd.IsBeforeAnnualReport {
				// 正常轮换但发生在年报披露期内，时机敏感，降为中风险提示
				mediumRiskChanges = append(mediumRiskChanges, cd)
			} else {
				// 其他情况：原因模糊、未说明具体原因，属可疑但不确定
				mediumRiskChanges = append(mediumRiskChanges, cd)
			}
		}

		// 处理高风险更换（一票否决）
		if len(highRiskChanges) > 0 {
			details := []string{}
			for i, cd := range highRiskChanges {
				if len(highRiskChanges) == 1 {
					details = append(details, fmt.Sprintf("变更公告日期: %s", cd.Date))
				} else {
					details = append(details, fmt.Sprintf("变更公告%d日期: %s", i+1, cd.Date))
				}
				if cd.OldAuditor != "" {
					details = append(details, fmt.Sprintf("  更换前审计机构: %s", cd.OldAuditor))
				}
				if cd.NewAuditor != "" {
					details = append(details, fmt.Sprintf("  更换后审计机构: %s", cd.NewAuditor))
				}
				if cd.IsBeforeAnnualReport {
					details = append(details, fmt.Sprintf("  ⚠️ 对应年报截止日: %s（变更公告发布于年报披露期内）", cd.AnnualReportDeadline))
				} else {
					details = append(details, fmt.Sprintf("  对应年报截止日: %s", cd.AnnualReportDeadline))
				}
				details = append(details, fmt.Sprintf("  变更原因: %s", cd.Reason))
				if cd.IsAbnormal {
					details = append(details, "  ⚠️ 该变更属于异常信号（辞任/解聘/意见分歧等）")
				}
			}
			if len(infoChanges) > 0 {
				details = append(details, fmt.Sprintf("（另有 %d 次正常轮换/被动更换已排除）", len(infoChanges)))
			}
			details = append(details, "⚠️ 年报披露期内更换审计机构或异常辞任，通常是掩盖财务问题的强烈信号")
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "auditor_changed",
				Name:    "审计机构异常更换",
				Level:   "high",
				Source:  "external",
				Format:  "近3年异常更换审计机构",
				Details: details,
			})
			summary.OneVeto = true
		}

		// 处理中风险更换（可疑但不确定，不触发一票否决）
		if len(mediumRiskChanges) > 0 && len(highRiskChanges) == 0 {
			details := []string{}
			for i, cd := range mediumRiskChanges {
				if len(mediumRiskChanges) == 1 {
					details = append(details, fmt.Sprintf("变更公告日期: %s", cd.Date))
				} else {
					details = append(details, fmt.Sprintf("变更公告%d日期: %s", i+1, cd.Date))
				}
				if cd.OldAuditor != "" {
					details = append(details, fmt.Sprintf("  更换前审计机构: %s", cd.OldAuditor))
				}
				if cd.NewAuditor != "" {
					details = append(details, fmt.Sprintf("  更换后审计机构: %s", cd.NewAuditor))
				}
				details = append(details, fmt.Sprintf("  对应年报截止日: %s", cd.AnnualReportDeadline))
				details = append(details, fmt.Sprintf("  变更原因: %s", cd.Reason))
			}
			if len(infoChanges) > 0 {
				details = append(details, fmt.Sprintf("（另有 %d 次正常轮换/被动更换已排除）", len(infoChanges)))
			}
			details = append(details, "⚠️ 该审计机构更换未说明明确原因，建议关注后续年报审计意见及更换细节")
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "auditor_changed_suspicious",
				Name:    "审计机构更换原因不明",
				Level:   "medium",
				Source:  "external",
				Format:  "近3年更换审计机构（原因不明）",
				Details: details,
			})
		}

		// 检测频繁更换：半年内（180天）多次更换审计机构，即使单次看似正常也需重点关注
		if len(external.AuditorChangeDetails) >= 2 {
			sortedChanges := make([]AuditorChangeDetail, len(external.AuditorChangeDetails))
			copy(sortedChanges, external.AuditorChangeDetails)
			sort.Slice(sortedChanges, func(i, j int) bool {
				return sortedChanges[i].Date < sortedChanges[j].Date
			})
			for i := 1; i < len(sortedChanges); i++ {
				days := daysBetween(sortedChanges[i-1].Date, sortedChanges[i].Date)
				if days >= 0 && days <= 180 {
					details := []string{}
					details = append(details, fmt.Sprintf("近3年内共发生 %d 次审计机构变更", len(external.AuditorChangeDetails)))
					details = append(details, fmt.Sprintf("%s 与 %s 两次变更间隔仅 %d 天（≤180天）", sortedChanges[i-1].Date, sortedChanges[i].Date, days))
					if sortedChanges[i-1].OldAuditor != "" || sortedChanges[i-1].NewAuditor != "" {
						details = append(details, fmt.Sprintf("  前次: %s → %s", sortedChanges[i-1].OldAuditor, sortedChanges[i-1].NewAuditor))
					}
					if sortedChanges[i].OldAuditor != "" || sortedChanges[i].NewAuditor != "" {
						details = append(details, fmt.Sprintf("  后次: %s → %s", sortedChanges[i].OldAuditor, sortedChanges[i].NewAuditor))
					}
					details = append(details, "⚠️ 半年内多次更换审计机构是极不寻常的信号，即使单次变更原因看似正常，频繁更换通常意味着公司与审计机构之间存在未披露的重大分歧或财务问题")
					summary.Flags = append(summary.Flags, RiskAlertFlag{
						Code:    "auditor_changed_frequent",
						Name:    "审计机构频繁更换",
						Level:   "high",
						Source:  "external",
						Format:  fmt.Sprintf("%d 天内更换 %d 次", days, len(external.AuditorChangeDetails)),
						Details: details,
					})
					summary.OneVeto = true
					break
				}
			}
		}

		// 处理正常轮换/政策合规/被动更换（信息提示，不触发风险，但展示在报告中）
		if len(infoChanges) > 0 && len(highRiskChanges) == 0 {
			details := []string{}
			var noteParts []string
			for _, cd := range infoChanges {
				details = append(details, fmt.Sprintf("%s: %s → %s（%s）", cd.Date, cd.OldAuditor, cd.NewAuditor, cd.Reason))
				if cd.IsNormalRotation {
					noteParts = append(noteParts, fmt.Sprintf("%s 的更换公告标题中出现「招标/选聘/连续服务」等正常轮换关键词", cd.Date))
				} else if cd.IsPolicyCompliance {
					noteParts = append(noteParts, fmt.Sprintf("%s 为政策合规轮换（如服务年限届满）", cd.Date))
				} else if cd.IsPassiveChange {
					noteParts = append(noteParts, fmt.Sprintf("%s 为被动更换（原事务所被处罚等，非公司自身问题）", cd.Date))
				}
			}
			note := "该审计机构更换看似异常，实则属于正常轮换。"
			if len(noteParts) > 0 {
				note += strings.Join(noteParts, "；") + "。"
			}
			note += " 更换不在年报披露期内，且未发现辞任、解聘、意见分歧等异常信号，因此不列入风险警示。"
			details = append(details, "✓ 该变更为正常轮换/政策合规更换/被动更换，不属于公司自身风险信号")
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "auditor_rotation",
				Name:    "审计机构正常轮换",
				Level:   "info",
				Source:  "external",
				Format:  "审计机构正常轮换",
				Details: details,
				Note:    note,
			})
		}
	}

	// 3. 核心财务负责人频繁更换（外部数据）
	if external != nil && external.ExecChanged {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:    "exec_changed",
			Name:    "核心财务负责人频繁更换",
			Level:   "high",
			Source:  "external",
			Format:  fmt.Sprintf("近1年财务/审计负责人变动 %d 次", external.ExecChangeCount),
			Details: external.ExecHistory,
		})
		summary.OneVeto = true
	}

	// 4. 资金占用/违规担保诉讼（外部数据）
	if external != nil && external.HasLitigation {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:    "litigation",
			Name:    "资金占用/违规担保/诉讼",
			Level:   "high",
			Source:  "external",
			Format:  fmt.Sprintf("存在 %d 起高风险公告", external.LitigationCount),
			Details: external.LitigationHistory,
		})
		summary.OneVeto = true
	}

	// 5. 印章失控传闻（外部数据 - 舆情）
	if external != nil && external.SealControlRumor {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:    "seal_rumor",
			Name:    "印章失控传闻",
			Level:   "high",
			Source:  "external",
			Format:  "舆情监测到印章失控相关传闻",
			Details: []string{"印章失控通常意味着公司治理存在严重问题，可能导致合同欺诈、资金挪用等风险"},
		})
		summary.OneVeto = true
	}

	// step8 在多个检查中使用（A-Score、M-Score、DSRI、DEPI 等）
	step8 := findStep(8)

	// 6. 经营现金流连续亏损
	step15 := findStep(15)
	if step15 != nil {
		ocfLatest := getFloat(step15, latest, "operatingCashFlow")
		ocfPrev := getFloat(step15, prev, "operatingCashFlow")
		if ocfLatest < 0 && ocfPrev < 0 {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "ocf_consecutive_negative",
				Name:    "经营现金流连续亏损",
				Level:   "high",
				Source:  "step15",
				Format:  "经营活动现金流连续2年为负",
				Details: []string{fmt.Sprintf("%s 经营现金流: %.0f", latest, ocfLatest), fmt.Sprintf("%s 经营现金流: %.0f", prev, ocfPrev), "连续2年经营现金流为负，说明主营业务无法产生正向现金"},
			})
			summary.OneVeto = true
		}
	}

	// 8. 负债率极高
	step3 := findStep(3)
	if step3 != nil {
		debtRatio := getFloat(step3, latest, "debtRatio")
		if debtRatio > thresholds.DebtRatioExtreme {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "debt_ratio_extreme",
				Name:    "资产负债率极高",
				Value:   debtRatio,
				Level:   "high",
				Source:  "step3",
				Format:  fmt.Sprintf("资产负债率 %.1f%%（> %.0f%%）", debtRatio, thresholds.DebtRatioExtreme),
				Details: []string{fmt.Sprintf("资产负债率 %.2f%%，超过 %.0f%% 警戒线", debtRatio, thresholds.DebtRatioExtreme), "高负债率意味着偿债压力大，一旦融资环境收紧可能引发流动性危机"},
			})
			summary.OneVeto = true
		}
	}

	// 9. 营收断崖式下跌
	step9 := findStep(9)
	if step9 != nil {
		revGrowth := getFloat(step9, latest, "growthRate")
		if revGrowth < thresholds.RevenueCollapse {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "revenue_collapse",
				Name:    "营收断崖式下跌",
				Value:   revGrowth,
				Level:   "high",
				Source:  "step9",
				Format:  fmt.Sprintf("营收同比 %.1f%%（< %.0f%%）", revGrowth, thresholds.RevenueCollapse),
				Details: []string{fmt.Sprintf("营收同比增长率 %.2f%%，低于 %.0f%% 警戒线", revGrowth, thresholds.RevenueCollapse), "营收大幅下滑通常预示行业景气度下行或公司竞争力衰退"},
			})
			summary.OneVeto = true
		}
	}

	// 10. 大股东高比例质押
	pledgeRatio := 0.0
	if v, ok := extras["pledge_ratio"]; ok {
		pledgeRatio = v
	}
	if pledgeRatio > thresholds.PledgeExtreme {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:    "pledge_extreme",
			Name:    "大股东高比例质押",
			Value:   pledgeRatio,
			Level:   "high",
			Source:  "crawler",
			Format:  fmt.Sprintf("股权质押 %.0f%%（> %.0f%%）", pledgeRatio, thresholds.PledgeExtreme),
			Details: []string{fmt.Sprintf("大股东股权质押比例 %.0f%%，超过 %.0f%% 警戒线", pledgeRatio, thresholds.PledgeExtreme), "高比例质押意味着大股东资金链紧张，股价下跌可能触发强制平仓"},
		})
		summary.OneVeto = true
	}

	// 11. 一年内多次监管问询
	inquiryCount := 0.0
	if v, ok := extras["inquiry_count_1y"]; ok {
		inquiryCount = v
	}
	if inquiryCount >= thresholds.InquiryExtreme {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:    "inquiry_extreme",
			Name:    "一年内多次监管问询",
			Value:   inquiryCount,
			Level:   "high",
			Source:  "crawler",
			Format:  fmt.Sprintf("近1年被监管问询 %.0f 次", inquiryCount),
			Details: []string{fmt.Sprintf("近1年被监管问询 %.0f 次，超过 %.0f 次警戒线", inquiryCount, thresholds.InquiryExtreme), "频繁被监管问询通常说明信息披露存在问题或财务数据异常"},
		})
		summary.OneVeto = true
	}

	// 12. 毛利率为负
	step10 := findStep(10)
	if step10 != nil {
		gm := getFloat(step10, latest, "grossMargin")
		if gm < 0 {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "gm_negative",
				Name:    "毛利率为负",
				Value:   gm,
				Level:   "high",
				Source:  "step10",
				Format:  fmt.Sprintf("毛利率 %.1f%%（为负）", gm*100),
				Details: []string{fmt.Sprintf("毛利率 %.2f%%，为负值", gm*100), "毛利率为负意味着产品售价低于成本，主营业务不具备盈利能力"},
			})
			summary.OneVeto = true
		}
	}

	// ========== 二级风险检查 ==========
	// 直接触发：单条即加入 flags（严重风险信号）
	// 累积计数：加入 accumulatedFlags 但不影响等级判定，累积 3 条以上才触发中风险
	accumulatedFlags := []RiskAlertFlag{}

	// 1. A-Score 偏高
	if step8 != nil {
		ascore := getFloat(step8, latest, "AScore")
		summary.Score = ascore
		if ascore >= thresholds.AScoreHigh {
			// A-Score ≥ 70：直接触发
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:    "ascore_high",
				Name:    "A-Score 综合风险偏高",
				Value:   ascore,
				Level:   "high",
				Source:  "step8",
				Format:  fmt.Sprintf("A-Score %.1f", ascore),
				Details: []string{fmt.Sprintf("A-Score %.2f，超过 %.0f 分高风险线", ascore, thresholds.AScoreHigh), "A-Score 综合评分越高，财务操纵和造假风险越大"},
			})
		} else if ascore >= thresholds.AScoreMedium {
			// A-Score 60-69：累积计数
			accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
				Code:   "ascore_medium",
				Name:   "A-Score 中等",
				Value:  ascore,
				Level:  "medium",
				Source: "step8",
				Format: fmt.Sprintf("A-Score %.1f", ascore),
			})
		}
	}

	// 2. M-Score 造假嫌疑
	if step8 != nil {
		mscore := getFloat(step8, latest, "MScore")
		if mscore > thresholds.MScoreSuspect {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:   "mscore_suspect",
				Name:   "M-Score 造假嫌疑",
				Value:  mscore,
				Level:  "medium",
				Source: "step8",
				Format: fmt.Sprintf("M-Score %.2f（> %.2f）", mscore, thresholds.MScoreSuspect),
			})
		}
	}

	// 3. 应收账款异常
	// 逻辑：应收增速显著高于营收增速，且营收为正增长时尤为可疑；
	// 或营收下滑但应收大增（回款困难信号）
	if step8 != nil {
		dsri := getFloat(step8, latest, "DSRI")
		arGrowth := 0.0
		revGrowth := 0.0
		if step9 != nil {
			arGrowth = getFloat(step9, latest, "arGrowth")
			revGrowth = getFloat(step9, latest, "growthRate")
		}
		// 条件1：营收正增长时，应收增速 > 营收增速*1.5 且应收增速 > 20%
		condition1 := revGrowth > 0 && arGrowth > revGrowth*1.5 && arGrowth > 20
		// 条件2：营收下滑时，应收增速 > 30%（回款困难）
		condition2 := revGrowth <= 0 && arGrowth > 30
		if dsri > 1.0 && (condition1 || condition2) {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:   "ar_abnormal",
				Name:   "应收账款异常",
				Value:  dsri,
				Level:  "medium",
				Source: "step8",
				Format: fmt.Sprintf("应收增速 %.1f%% 远超营收增速 %.1f%%", arGrowth*100, revGrowth*100),
			})
		}
	}

	// 4. 毛利率大幅下滑（累积计数）
	if step10 != nil && prev != "" {
		gmLatest := getFloat(step10, latest, "grossMargin")
		gmPrev := getFloat(step10, prev, "grossMargin")
		if gmPrev > 0 && gmLatest < gmPrev {
			// steps.go 中 grossMargin 已存储为百分比（如 60.32 表示 60.32%）
			// 因此下降幅度直接相减即可，无需再乘 100
			decline := gmPrev - gmLatest
			if decline > thresholds.GMDecline {
				accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
					Code:   "gm_decline",
					Name:   "毛利率大幅下滑",
					Value:  decline,
					Level:  "medium",
					Source: "step10",
					Format: fmt.Sprintf("毛利率下降 %.1f 百分点", decline),
				})
			}
		}
	}

	// 5. ROE 偏低（累积计数）
	step16 := findStep(16)
	if step16 != nil {
		roe := getFloat(step16, latest, "roe")
		if roe < thresholds.ROELow {
			accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
				Code:   "roe_low",
				Name:   "ROE 偏低",
				Value:  roe,
				Level:  "medium",
				Source: "step16",
				Format: fmt.Sprintf("ROE %.1f%%（< %.0f%%）", roe, thresholds.ROELow),
			})
		}
	}

	// 6. 净利润现金含量不足
	if step15 != nil {
		cashRatio := getFloat(step15, latest, "cashRatio")
		if cashRatio < thresholds.CashRatioCritical && cashRatio > 0 {
			// < 0.3：直接触发（high）
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:   "cash_ratio_critical",
				Name:   "净利润现金含量严重不足",
				Value:  cashRatio,
				Level:  "high",
				Source: "step15",
				Format: fmt.Sprintf("经营现金流/净利润 %.1f%%（< 30%%）", cashRatio),
			})
		} else if cashRatio < thresholds.CashRatioLow && cashRatio > 0 {
			// 0.3-0.8：累积计数
			accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
				Code:   "cash_ratio_low",
				Name:   "净利润现金含量不足",
				Value:  cashRatio,
				Level:  "medium",
				Source: "step15",
				Format: fmt.Sprintf("经营现金流/净利润 %.1f%%", cashRatio),
			})
		}
	}

	// 7. 存货 + 应收占比高（累积计数）
	step5 := findStep(5)
	step11 := findStep(11)
	step3Data := findStep(3)
	if step5 != nil && step11 != nil && step3Data != nil {
		ar := getFloat(step5, latest, "receivableRatio")
		inventory := getFloat(step11, latest, "inventoryRatio")
		totalAsset := getFloat(step3Data, latest, "totalAssets")
		if totalAsset > 0 {
			arInventoryRatio := (ar + inventory) / totalAsset * 100
			if arInventoryRatio > thresholds.ARInventoryHigh {
				accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
					Code:   "ar_inventory_high",
					Name:   "存货+应收占比偏高",
					Value:  arInventoryRatio,
					Level:  "medium",
					Source: "step5/step11",
					Format: fmt.Sprintf("应收+存货占总资产 %.1f%%", arInventoryRatio),
				})
			}
		}
	}

	// 8. 大股东减持（直接触发）
	reductionCount := 0.0
	if v, ok := extras["reduction_count_1y"]; ok {
		reductionCount = v
	}
	if reductionCount >= 1 {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:   "reduction",
			Name:   "大股东减持",
			Value:  reductionCount,
			Level:  "medium",
			Source: "crawler",
			Format: fmt.Sprintf("近1年减持公告 %.0f 次", reductionCount),
		})
	}

	// 9. 股权质押偏高（直接触发）
	if pledgeRatio > thresholds.PledgeMedium && pledgeRatio <= thresholds.PledgeExtreme {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:   "pledge_high",
			Name:   "股权质押比例偏高",
			Value:  pledgeRatio,
			Level:  "medium",
			Source: "crawler",
			Format: fmt.Sprintf("股权质押 %.0f%%", pledgeRatio),
		})
	}

	// 10. 营收增长停滞（累积计数）
	if step9 != nil {
		revGrowth := getFloat(step9, latest, "growthRate")
		if revGrowth < thresholds.RevenueStagnant && revGrowth >= thresholds.RevenueCollapse {
			accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
				Code:   "revenue_stagnant",
				Name:   "营收增长停滞",
				Value:  revGrowth,
				Level:  "medium",
				Source: "step9",
				Format: fmt.Sprintf("营收同比 %.1f%%", revGrowth),
			})
		}
	}

	// 11. 负债率偏高（累积计数）
	if step3 != nil {
		debtRatio := getFloat(step3, latest, "debtRatio")
		if debtRatio > thresholds.DebtRatioMedium && debtRatio <= thresholds.DebtRatioExtreme {
			accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
				Code:   "debt_ratio_medium",
				Name:   "负债率偏高",
				Value:  debtRatio,
				Level:  "medium",
				Source: "step3",
				Format: fmt.Sprintf("资产负债率 %.1f%%", debtRatio),
			})
		}
	}

	// 12. 商誉占比高（累积计数）
	step7 := findStep(7)
	if step7 != nil && step3Data != nil {
		goodwill := getFloat(step7, latest, "goodwill")
		equity := getFloat(step3Data, latest, "equity")
		if equity > 0 {
			goodwillRatio := goodwill / equity * 100
			if goodwillRatio > thresholds.GoodwillHigh {
				accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
					Code:   "goodwill_high",
					Name:   "商誉占比偏高",
					Value:  goodwillRatio,
					Level:  "medium",
					Source: "step7",
					Format: fmt.Sprintf("商誉/净资产 %.1f%%", goodwillRatio),
				})
			}
		}
	}

	// 13. 折旧费用异常（DEPI 指标）（直接触发）
	if step8 != nil {
		depi := getFloat(step8, latest, "DEPI")
		if depi > thresholds.DEPISuspect {
			summary.Flags = append(summary.Flags, RiskAlertFlag{
				Code:   "depi_suspect",
				Name:   "折旧费用异常（可能拉长折旧年限）",
				Value:  depi,
				Level:  "medium",
				Source: "step8",
				Format: fmt.Sprintf("DEPI %.2f（> %.2f）", depi, thresholds.DEPISuspect),
			})
		}
	}

	// 14. 内部人大额减持（外部数据）（直接触发）
	if external != nil && external.HasInternalSell {
		summary.Flags = append(summary.Flags, RiskAlertFlag{
			Code:   "internal_sell",
			Name:   "内部人大额减持",
			Level:  "medium",
			Source: "external",
			Format: "近半年有大额减持动作",
		})
	}

	// 15. 内部人增持（正面信号，不作为风险，但可用于抵消）
	if external != nil && external.HasInternalBuy {
		// 正面信号，记录但不加入 risk flags
		// 可在后续版本中用于风险评分调整
	}

	// ========== 综合判定风险等级 ==========

	highCount := 0
	mediumCount := 0
	for _, f := range summary.Flags {
		if f.Level == "high" {
			highCount++
		} else if f.Level == "medium" {
			mediumCount++
		}
	}

	// 4.1 普通对外担保（外部数据 - 中风险累积）
	if external != nil && external.HasGuarantee && !external.HasHighRiskGuarantee && !external.HasFundOccupation {
		accumulatedFlags = append(accumulatedFlags, RiskAlertFlag{
			Code:   "guarantee_medium",
			Name:   "对外担保",
			Level:  "medium",
			Source: "external",
			Format: "存在对外担保相关公告",
		})
	}

	// 累积判定：直接触发的 medium + 累积计数 ≥ 3 才升级为中风险
	totalMedium := mediumCount + len(accumulatedFlags)

	if summary.OneVeto || highCount >= 1 {
		summary.Level = "high"
		// 混合等级：同时存在 high 和 medium 时显示"中高风险"
		if totalMedium > 0 {
			summary.PrimaryMsg = fmt.Sprintf("该股票存在 %d 项中高风险信号", highCount+totalMedium)
		} else {
			summary.PrimaryMsg = fmt.Sprintf("该股票存在 %d 项高风险信号", highCount)
		}
		// 高风险时，累积项也加入 flags 供展示
		summary.Flags = append(summary.Flags, accumulatedFlags...)
	} else if totalMedium >= 3 {
		summary.Level = "medium"
		summary.PrimaryMsg = fmt.Sprintf("该股票存在 %d 项中风险信号", totalMedium)
		// 中风险时，把累积项加入 flags 以便报告列出具体是哪几项
		summary.Flags = append(summary.Flags, accumulatedFlags...)
	} else if totalMedium >= 1 {
		// 1-2 项中风险信号：显示为 low，不加入 flags（避免干扰）
		summary.Level = "low"
		if len(summary.Flags) > 0 {
			summary.PrimaryMsg = fmt.Sprintf("未发现重大风险信号（有 %d 项关注点）", totalMedium)
		} else {
			summary.PrimaryMsg = "未发现重大风险信号"
		}
	} else {
		summary.Level = "low"
		summary.PrimaryMsg = "🟢 未发现重大风险信号"
	}

	// 13. 审计意见非标（一票否决）
	step1 = findStep(1)
	if step1 != nil {
		opinion := getString(step1, latest, "opinion")
		auditor := getString(step1, latest, "auditor")
		isStandardVal := step1.YearlyData[latest]["isStandard"]
		var isStandard bool
		if b, ok := isStandardVal.(bool); ok {
			isStandard = b
		}
		if opinion != "" && opinion != "请查询年报确认" {
			if !isStandard {
				// 保留意见 / 无法表示意见 / 否定意见 / 带强调事项段
				level := "high"
				if strings.Contains(opinion, "强调事项") {
					level = "medium"
				}
				summary.Flags = append(summary.Flags, RiskAlertFlag{
					Code:    "audit_nonstandard",
					Name:    "审计意见非标",
					Value:   0,
					Level:   level,
					Source:  "step1",
					Format:  fmt.Sprintf("%s（%s）", opinion, auditor),
					Details: []string{"非标审计意见通常意味着财务报表存在重大不确定性或错报风险"},
				})
				if level == "high" {
					summary.OneVeto = true
				}
			}
		}
	}

	return summary
}

// getSensitivityThresholds 根据敏感度返回阈值配置
func getSensitivityThresholds(s SensitivityLevel) *riskThresholds {
	switch s {
	case SensitivityStrict:
		return &riskThresholds{
			DebtRatioExtreme:  70,
			DebtRatioMedium:   50,
			RevenueCollapse:   -20,
			RevenueStagnant:   10,
			PledgeExtreme:     80,
			PledgeMedium:      50,
			InquiryExtreme:    2,
			AScoreHigh:        60,
			AScoreMedium:      50,
			MScoreSuspect:     -1.78,
			GMDecline:         3,
			ROELow:            12,
			CashRatioLow:      80,
			CashRatioCritical: 30,
			ARInventoryHigh:   25,
			GoodwillHigh:      20,
			DEPISuspect:       1.1,
		}
	case SensitivityLoose:
		return &riskThresholds{
			DebtRatioExtreme:  85,
			DebtRatioMedium:   65,
			RevenueCollapse:   -40,
			RevenueStagnant:   0,
			PledgeExtreme:     80,
			PledgeMedium:      55,
			InquiryExtreme:    5,
			AScoreHigh:        70,
			AScoreMedium:      60,
			MScoreSuspect:     -1.5,
			GMDecline:         8,
			ROELow:            8,
			CashRatioLow:      60,
			CashRatioCritical: 20,
			ARInventoryHigh:   40,
			GoodwillHigh:      50,
			DEPISuspect:       1.2,
		}
	default: // standard
		return &riskThresholds{
			DebtRatioExtreme:  85,
			DebtRatioMedium:   70,
			RevenueCollapse:   -30,
			RevenueStagnant:   0,
			PledgeExtreme:     80,
			PledgeMedium:      50,
			InquiryExtreme:    3,
			AScoreHigh:        70,
			AScoreMedium:      60,
			MScoreSuspect:     -1.78,
			GMDecline:         10,
			ROELow:            5,
			CashRatioLow:      80,
			CashRatioCritical: 30,
			ARInventoryHigh:   40,
			GoodwillHigh:      50,
			DEPISuspect:       1.1,
		}
	}
}

// riskThresholds 风险阈值配置
type riskThresholds struct {
	ZScoreBankrupt    float64
	DebtRatioExtreme  float64
	DebtRatioMedium   float64
	RevenueCollapse   float64
	RevenueStagnant   float64
	PledgeExtreme     float64
	PledgeMedium      float64
	InquiryExtreme    float64
	AScoreHigh        float64
	AScoreMedium      float64
	MScoreSuspect     float64
	GMDecline         float64
	ROELow            float64
	CashRatioLow      float64
	CashRatioCritical float64
	ARInventoryHigh   float64
	GoodwillHigh      float64
	DEPISuspect       float64
}

// FormatFlagValue 格式化风险标记的数值
func (f *RiskAlertFlag) FormatFlagValue() string {
	if f.Format != "" {
		return f.Format
	}
	return f.Name
}

// daysBetween 计算两个日期之间的天数差（date1 在 date2 之前）
func daysBetween(date1, date2 string) int {
	d1, err1 := time.Parse("2006-01-02", date1)
	d2, err2 := time.Parse("2006-01-02", date2)
	if err1 != nil || err2 != nil {
		return -1
	}
	return int(d2.Sub(d1).Hours() / 24)
}

// monthsBefore 计算两个日期之间的大约月数差（date1 在 date2 之前）
func monthsBefore(date1, date2 string) int {
	// 简单按年月计算
	if len(date1) >= 7 && len(date2) >= 7 {
		y1, m1 := 0, 0
		y2, m2 := 0, 0
		fmt.Sscanf(date1[:7], "%d-%d", &y1, &m1)
		fmt.Sscanf(date2[:7], "%d-%d", &y2, &m2)
		return (y2-y1)*12 + (m2 - m1)
	}
	return 0
}
