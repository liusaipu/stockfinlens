package analyzer

import (
	"fmt"
	"strings"
)

// QuarterlyAlertItem 单条季度预警项
type QuarterlyAlertItem struct {
	Period      string  `json:"period"`
	Metric      string  `json:"metric"`
	Current     float64 `json:"current"`
	Previous    float64 `json:"previous"`
	ChangePct   float64 `json:"changePct"`
	Level       string  `json:"level"` // "warning" / "danger"
	Description string  `json:"description"`
	CompareType string  `json:"compareType"` // "环比" / "同比"
}

// QuarterlyAlert 季度滚动预警摘要
type QuarterlyAlert struct {
	HasData bool                 `json:"hasData"`
	Items   []QuarterlyAlertItem `json:"items"`
}

// BuildQuarterlyAlert 构建季度滚动预警（含环比+同比）
func BuildQuarterlyAlert(data *FinancialData) *QuarterlyAlert {
	alert := &QuarterlyAlert{}
	if data == nil || len(data.Quarters) == 0 {
		return alert
	}

	// 提取季度数据（含年报，Q1/Q2/Q3/Q4）
	quarters := filterQuarters(data.Quarters)
	if len(quarters) < 2 {
		return alert
	}

	alert.HasData = true

	// ========== 环比检测（相邻单季）==========
	for _, cur := range quarters {
		prev := previousQuarterPeriod(cur)
		if prev == "" || !hasPeriod(quarters, prev) {
			continue
		}
		alert.addQoQChecks(data, cur, prev)
	}

	// ========== 同比检测（去年同期单季）==========
	for _, cur := range quarters {
		yoy := getYoYPeriod(cur)
		if yoy == "" || !hasPeriod(quarters, yoy) {
			continue
		}
		alert.addYoYChecks(data, cur, yoy)
	}

	return alert
}

// addQoQChecks 添加环比检测项（基于单季还原值）
func (alert *QuarterlyAlert) addQoQChecks(data *FinancialData, cur, prev string) {
	// 营收环比
	curRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", cur)
	prevRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", prev)
	if prevRev > 0 {
		revChange := (curRev - prevRev) / prevRev
		if revChange < -0.20 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "营业收入", Current: curRev, Previous: prevRev,
				ChangePct: revChange, Level: "danger",
				Description: fmt.Sprintf("营收环比下滑 %.1f%%", -revChange*100),
				CompareType: "环比",
			})
		} else if revChange < -0.05 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "营业收入", Current: curRev, Previous: prevRev,
				ChangePct: revChange, Level: "warning",
				Description: fmt.Sprintf("营收环比下滑 %.1f%%", -revChange*100),
				CompareType: "环比",
			})
		}
	}

	// 净利润环比
	curNP := singleQuarterValue(data, data.IncomeStatement, "净利润", cur)
	prevNP := singleQuarterValue(data, data.IncomeStatement, "净利润", prev)
	if prevNP != 0 {
		npChange := (curNP - prevNP) / abs(prevNP)
		if npChange < -0.30 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "净利润", Current: curNP, Previous: prevNP,
				ChangePct: npChange, Level: "danger",
				Description: fmt.Sprintf("净利润环比下滑 %.1f%%", -npChange*100),
				CompareType: "环比",
			})
		} else if npChange < -0.10 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "净利润", Current: curNP, Previous: prevNP,
				ChangePct: npChange, Level: "warning",
				Description: fmt.Sprintf("净利润环比下滑 %.1f%%", -npChange*100),
				CompareType: "环比",
			})
		}
	}

	// 毛利率环比变化（基于单季营收/成本）
	curRevSQ := singleQuarterValue(data, data.IncomeStatement, "营业收入", cur)
	prevRevSQ := singleQuarterValue(data, data.IncomeStatement, "营业收入", prev)
	curCostSQ := singleQuarterValue(data, data.IncomeStatement, "营业成本", cur)
	prevCostSQ := singleQuarterValue(data, data.IncomeStatement, "营业成本", prev)
	if curRevSQ > 0 && prevRevSQ > 0 {
		curGM := (curRevSQ - curCostSQ) / curRevSQ
		prevGM := (prevRevSQ - prevCostSQ) / prevRevSQ
		gmChange := curGM - prevGM
		if gmChange < -0.03 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "毛利率", Current: curGM, Previous: prevGM,
				ChangePct: gmChange, Level: "warning",
				Description: fmt.Sprintf("毛利率环比下降 %.1f%%", -gmChange*100),
				CompareType: "环比",
			})
		}
	}

	// 经营现金流环比（现金流量表也是累计 YTD，需还原为单季）
	curOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", cur)
	prevOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", prev)
	if prevOCF != 0 {
		ocfChange := (curOCF - prevOCF) / abs(prevOCF)
		if curOCF < 0 && prevOCF > 0 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "经营现金流", Current: curOCF, Previous: prevOCF,
				ChangePct: ocfChange, Level: "danger",
				Description: "经营现金流由正转负",
				CompareType: "环比",
			})
		}
	}
}

// addYoYChecks 添加同比检测项（基于单季还原值）
func (alert *QuarterlyAlert) addYoYChecks(data *FinancialData, cur, yoy string) {
	// 营收同比
	curRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", cur)
	yoyRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", yoy)
	if yoyRev > 0 {
		revChange := (curRev - yoyRev) / yoyRev
		if revChange < -0.30 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "营业收入", Current: curRev, Previous: yoyRev,
				ChangePct: revChange, Level: "danger",
				Description: fmt.Sprintf("营收同比下滑 %.1f%%", -revChange*100),
				CompareType: "同比",
			})
		} else if revChange < -0.15 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "营业收入", Current: curRev, Previous: yoyRev,
				ChangePct: revChange, Level: "warning",
				Description: fmt.Sprintf("营收同比下滑 %.1f%%", -revChange*100),
				CompareType: "同比",
			})
		}
	}

	// 净利润同比
	curNP := singleQuarterValue(data, data.IncomeStatement, "净利润", cur)
	yoyNP := singleQuarterValue(data, data.IncomeStatement, "净利润", yoy)
	if yoyNP != 0 {
		npChange := (curNP - yoyNP) / abs(yoyNP)
		if npChange < -0.50 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "净利润", Current: curNP, Previous: yoyNP,
				ChangePct: npChange, Level: "danger",
				Description: fmt.Sprintf("净利润同比下滑 %.1f%%", -npChange*100),
				CompareType: "同比",
			})
		} else if npChange < -0.25 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "净利润", Current: curNP, Previous: yoyNP,
				ChangePct: npChange, Level: "warning",
				Description: fmt.Sprintf("净利润同比下滑 %.1f%%", -npChange*100),
				CompareType: "同比",
			})
		}
	}

	// 毛利率同比变化（百分点，基于单季营收/成本）
	curRevSQ := singleQuarterValue(data, data.IncomeStatement, "营业收入", cur)
	yoyRevSQ := singleQuarterValue(data, data.IncomeStatement, "营业收入", yoy)
	curCostSQ := singleQuarterValue(data, data.IncomeStatement, "营业成本", cur)
	yoyCostSQ := singleQuarterValue(data, data.IncomeStatement, "营业成本", yoy)
	if curRevSQ > 0 && yoyRevSQ > 0 {
		curGM := (curRevSQ - curCostSQ) / curRevSQ
		yoyGM := (yoyRevSQ - yoyCostSQ) / yoyRevSQ
		gmChange := curGM - yoyGM
		if gmChange < -0.05 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "毛利率", Current: curGM, Previous: yoyGM,
				ChangePct: gmChange, Level: "danger",
				Description: fmt.Sprintf("毛利率同比下降 %.1f%%", -gmChange*100),
				CompareType: "同比",
			})
		} else if gmChange < -0.03 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "毛利率", Current: curGM, Previous: yoyGM,
				ChangePct: gmChange, Level: "warning",
				Description: fmt.Sprintf("毛利率同比下降 %.1f%%", -gmChange*100),
				CompareType: "同比",
			})
		}
	}

	// 经营现金流同比（现金流量表累计 YTD 还原为单季）
	curOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", cur)
	yoyOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", yoy)
	if yoyOCF != 0 {
		ocfChange := (curOCF - yoyOCF) / abs(yoyOCF)
		if curOCF < 0 && yoyOCF > 0 {
			alert.Items = append(alert.Items, QuarterlyAlertItem{
				Period: cur, Metric: "经营现金流", Current: curOCF, Previous: yoyOCF,
				ChangePct: ocfChange, Level: "danger",
				Description: "经营现金流同比由正转负",
				CompareType: "同比",
			})
		}
	}
}

// getYoYPeriod 获取去年同期期间
func getYoYPeriod(period string) string {
	parts := strings.Split(period, "-")
	if len(parts) != 3 {
		return ""
	}
	year := 0
	if _, err := fmt.Sscanf(parts[0], "%d", &year); err != nil || year < 2000 {
		return ""
	}
	return fmt.Sprintf("%d-%s-%s", year-1, parts[1], parts[2])
}

// filterQuarters 从所有期间中过滤出季度末日期（含年报 12-31）
func filterQuarters(periods []string) []string {
	var quarters []string
	for _, p := range periods {
		if len(p) != 10 {
			continue
		}
		switch {
		case strings.HasSuffix(p, "-03-31"),
			strings.HasSuffix(p, "-06-30"),
			strings.HasSuffix(p, "-09-30"),
			strings.HasSuffix(p, "-12-31"):
			quarters = append(quarters, p)
		}
	}
	return quarters
}

// previousQuarterPeriod 返回相邻的上一个季度
func previousQuarterPeriod(period string) string {
	parts := strings.Split(period, "-")
	if len(parts) != 3 {
		return ""
	}
	year := 0
	if _, err := fmt.Sscanf(parts[0], "%d", &year); err != nil || year < 2000 {
		return ""
	}
	month := parts[1]
	switch month {
	case "03":
		return fmt.Sprintf("%d-12-31", year-1)
	case "06":
		return fmt.Sprintf("%d-03-31", year)
	case "09":
		return fmt.Sprintf("%d-06-30", year)
	case "12":
		return fmt.Sprintf("%d-09-30", year)
	}
	return ""
}

// singleQuarterValue 把累计 YTD 值还原为单季度值
func singleQuarterValue(data *FinancialData, dataMap map[string]map[string]float64, account, period string) float64 {
	cumulative := data.GetValueOrZero(dataMap, account, period)
	if cumulative == 0 {
		return 0
	}
	// Q1 累计即单季
	if strings.HasSuffix(period, "-03-31") {
		return cumulative
	}
	prev := previousQuarterPeriod(period)
	if prev == "" {
		return cumulative
	}
	prevCum := data.GetValueOrZero(dataMap, account, prev)
	if prevCum == 0 {
		return cumulative
	}
	return cumulative - prevCum
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
