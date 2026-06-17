package analyzer

import (
	"fmt"
	"math"
	"strings"
)

// QuarterlyAlertItem 单条季度预警项
type QuarterlyAlertItem struct {
	Period          string  `json:"period"`
	PreviousPeriod  string  `json:"previousPeriod"`
	Metric          string  `json:"metric"`
	Current         float64 `json:"current"`
	Previous        float64 `json:"previous"`
	ChangePct       float64 `json:"changePct"`
	Level           string  `json:"level"` // "warning" / "danger"
	Description     string  `json:"description"`
	CompareType     string  `json:"compareType"` // "环比" / "同比"
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

	// 只取最近一期做滚动预警
	cur := quarters[0]

	// 环比检测（相邻单季）
	prev := previousQuarterPeriod(cur)
	if prev != "" && hasPeriod(quarters, prev) {
		alert.addQoQChecks(data, cur, prev)
	}

	// 同比检测（去年同期单季）
	yoy := getYoYPeriod(cur)
	if yoy != "" && hasPeriod(quarters, yoy) {
		alert.addYoYChecks(data, cur, yoy)
	}

	return alert
}

// addQoQChecks 添加环比检测项（基于单季还原值，始终生成条目）
func (alert *QuarterlyAlert) addQoQChecks(data *FinancialData, cur, prev string) {
	// 营收环比
	curRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", cur)
	prevRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", prev)
	if prevRev > 0 {
		revChange := (curRev - prevRev) / prevRev
		level, desc := classifyChange("营业收入", revChange, true)
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: prev, Metric: "营业收入", Current: curRev, Previous: prevRev,
			ChangePct: revChange, Level: level, Description: desc, CompareType: "环比",
		})
	}

	// 净利润环比
	curNP := singleQuarterValue(data, data.IncomeStatement, "净利润", cur)
	prevNP := singleQuarterValue(data, data.IncomeStatement, "净利润", prev)
	if prevNP != 0 {
		npChange := (curNP - prevNP) / abs(prevNP)
		level, desc := classifyChange("净利润", npChange, true)
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: prev, Metric: "净利润", Current: curNP, Previous: prevNP,
			ChangePct: npChange, Level: level, Description: desc, CompareType: "环比",
		})
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
		level, desc := classifyChange("毛利率", gmChange, true)
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: prev, Metric: "毛利率", Current: curGM, Previous: prevGM,
			ChangePct: gmChange, Level: level, Description: desc, CompareType: "环比",
		})
	}

	// 经营现金流环比（现金流量表也是累计 YTD，需还原为单季）
	curOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", cur)
	prevOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", prev)
	if prevOCF != 0 {
		ocfChange := (curOCF - prevOCF) / abs(prevOCF)
		level, desc := "normal", "经营现金流环比正常"
		if curOCF < 0 && prevOCF > 0 {
			level, desc = "danger", "经营现金流由正转负"
		}
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: prev, Metric: "经营现金流", Current: curOCF, Previous: prevOCF,
			ChangePct: ocfChange, Level: level, Description: desc, CompareType: "环比",
		})
	}
}

// addYoYChecks 添加同比检测项（基于单季还原值，始终生成条目）
func (alert *QuarterlyAlert) addYoYChecks(data *FinancialData, cur, yoy string) {
	// 营收同比
	curRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", cur)
	yoyRev := singleQuarterValue(data, data.IncomeStatement, "营业收入", yoy)
	if yoyRev > 0 {
		revChange := (curRev - yoyRev) / yoyRev
		level, desc := classifyChange("营业收入", revChange, false)
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: yoy, Metric: "营业收入", Current: curRev, Previous: yoyRev,
			ChangePct: revChange, Level: level, Description: desc, CompareType: "同比",
		})
	}

	// 净利润同比
	curNP := singleQuarterValue(data, data.IncomeStatement, "净利润", cur)
	yoyNP := singleQuarterValue(data, data.IncomeStatement, "净利润", yoy)
	if yoyNP != 0 {
		npChange := (curNP - yoyNP) / abs(yoyNP)
		level, desc := classifyChange("净利润", npChange, false)
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: yoy, Metric: "净利润", Current: curNP, Previous: yoyNP,
			ChangePct: npChange, Level: level, Description: desc, CompareType: "同比",
		})
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
		level, desc := classifyChange("毛利率", gmChange, false)
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: yoy, Metric: "毛利率", Current: curGM, Previous: yoyGM,
			ChangePct: gmChange, Level: level, Description: desc, CompareType: "同比",
		})
	}

	// 经营现金流同比（现金流量表累计 YTD 还原为单季）
	curOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", cur)
	yoyOCF := singleQuarterValue(data, data.CashFlow, "经营活动产生的现金流量净额", yoy)
	if yoyOCF != 0 {
		ocfChange := (curOCF - yoyOCF) / abs(yoyOCF)
		level, desc := "normal", "经营现金流同比正常"
		if curOCF < 0 && yoyOCF > 0 {
			level, desc = "danger", "经营现金流同比由正转负"
		}
		alert.Items = append(alert.Items, QuarterlyAlertItem{
			Period: cur, PreviousPeriod: yoy, Metric: "经营现金流", Current: curOCF, Previous: yoyOCF,
			ChangePct: ocfChange, Level: level, Description: desc, CompareType: "同比",
		})
	}
}

// classifyChange 根据变化率返回风险等级和描述（始终返回，不再过滤）
func classifyChange(metric string, changePct float64, isQoQ bool) (level string, desc string) {
	prefix := "同比"
	if isQoQ {
		prefix = "环比"
	}

	// 毛利率用百分点描述
	if metric == "毛利率" {
		changePP := changePct * 100
		switch {
		case changePct < -0.05:
			return "danger", fmt.Sprintf("毛利率%s下降 %.1fpp", prefix, -changePP)
		case changePct < -0.03:
			return "warning", fmt.Sprintf("毛利率%s下降 %.1fpp", prefix, -changePP)
		case changePct > 0.05:
			return "normal", fmt.Sprintf("毛利率%s上升 %.1fpp", prefix, changePP)
		default:
			return "normal", fmt.Sprintf("毛利率%s变化 %.1fpp", prefix, changePP)
		}
	}

	// 营收/净利润用百分比描述
	changePercent := changePct * 100
	sign := "变化"
	if changePct > 0 {
		sign = "增长"
	} else if changePct < 0 {
		sign = "下滑"
	}

	// 环比/同比的危险、警告阈值不同
	var dangerThreshold, warningThreshold float64
	switch metric {
	case "营业收入":
		if isQoQ {
			dangerThreshold, warningThreshold = -0.20, -0.05
		} else {
			dangerThreshold, warningThreshold = -0.30, -0.15
		}
	case "净利润":
		if isQoQ {
			dangerThreshold, warningThreshold = -0.30, -0.10
		} else {
			dangerThreshold, warningThreshold = -0.50, -0.25
		}
	}

	switch {
	case changePct <= dangerThreshold:
		return "danger", fmt.Sprintf("%s%s %.1f%%", metric, sign, math.Abs(changePercent))
	case changePct <= warningThreshold:
		return "warning", fmt.Sprintf("%s%s %.1f%%", metric, sign, math.Abs(changePercent))
	default:
		return "normal", fmt.Sprintf("%s%s %.1f%%", metric, sign, math.Abs(changePercent))
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
