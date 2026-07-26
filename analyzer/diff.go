package analyzer

import (
	"fmt"
	"math"
	"time"
)

// ComputeAnalysisDiff 计算两次分析报告之间的差异
// previous 可为 nil（首次分析），此时返回的 diff.HasPrevious = false
func ComputeAnalysisDiff(current, previous *AnalysisReport) *AnalysisDiff {
	diff := &AnalysisDiff{
		CurrentTime: time.Now().Format("2006-01-02 15:04"),
	}
	if current == nil {
		return diff
	}

	if previous == nil {
		return diff
	}

	diff.HasPrevious = true
	// 上次分析时间取自快照中持久化的生成时间；老版本快照没有该字段时由调用方回填
	diff.PreviousTime = previous.GeneratedAt

	// 评分变化
	if current.Score != nil && previous.Score != nil {
		curScore := current.Score["weighted"]
		prevScore := previous.Score["weighted"]
		diff.ScoreChange = curScore - prevScore
		diff.CurrentGrade = current.OverallGrade
		diff.PreviousGrade = previous.OverallGrade
		diff.GradeChanged = current.OverallGrade != previous.OverallGrade
	}

	// 风险 flags 变化
	if current.RiskAlert != nil && previous.RiskAlert != nil {
		curFlags := flagMap(current.RiskAlert.Flags)
		prevFlags := flagMap(previous.RiskAlert.Flags)

		for code, f := range curFlags {
			if _, ok := prevFlags[code]; !ok {
				diff.NewFlags = append(diff.NewFlags, f)
			} else {
				diff.PersistentFlags = append(diff.PersistentFlags, f)
			}
		}
		for code, f := range prevFlags {
			if _, ok := curFlags[code]; !ok {
				diff.ResolvedFlags = append(diff.ResolvedFlags, f)
			}
		}
	}

	// 关键指标变化
	diff.KeyMetricChanges = computeMetricChanges(current, previous)

	return diff
}

// flagMap 将风险 flag 列表转为以 Code 为 key 的 map
func flagMap(flags []RiskAlertFlag) map[string]RiskAlertFlag {
	m := make(map[string]RiskAlertFlag, len(flags))
	for _, f := range flags {
		m[f.Code] = f
	}
	return m
}

// computeMetricChanges 计算关键财务指标的变化
func computeMetricChanges(current, previous *AnalysisReport) []MetricChange {
	var changes []MetricChange
	if len(current.StepResults) == 0 || len(previous.StepResults) == 0 {
		return changes
	}

	curYear := ""
	if len(current.Years) > 0 {
		curYear = current.Years[0]
	}
	prevYear := ""
	if len(previous.Years) > 0 {
		prevYear = previous.Years[0]
	}
	if curYear == "" || prevYear == "" {
		return changes
	}

	// 定义要追踪的关键指标：stepNum -> key -> displayName
	metrics := []struct {
		stepNum int
		key     string
		name    string
		isPct   bool // 是否为百分比（变化用 pct 点表示）
		thresh  float64
	}{
		{8, "AScore", "A-Score风险分", false, 5},
		{10, "grossMargin", "毛利率", true, 0.03},
		{16, "roe", "ROE", true, 0.015},
		{9, "revenueGrowth", "营收增速", true, 0.05},
		{15, "cashFlowQuality", "经营现金流/净利润", false, 0.15},
		{3, "debtRatio", "资产负债率", true, 0.03},
	}

	for _, m := range metrics {
		curVal := getStepValueSafe(current.StepResults, m.stepNum, curYear, m.key)
		prevVal := getStepValueSafe(previous.StepResults, m.stepNum, prevYear, m.key)
		if math.IsNaN(curVal) || math.IsNaN(prevVal) {
			continue
		}
		delta := curVal - prevVal
		var deltaPct float64
		if m.isPct {
			deltaPct = delta // 百分比指标直接返回差值（如 0.03 = 3pct）
		} else if prevVal != 0 {
			deltaPct = delta / math.Abs(prevVal)
		}
		sig := math.Abs(delta) >= m.thresh
		changes = append(changes, MetricChange{
			Name:        m.name,
			Previous:    prevVal,
			Current:     curVal,
			Delta:       delta,
			DeltaPct:    deltaPct,
			Significant: sig,
		})
	}

	return changes
}

// getStepValueSafe 从 StepResults 中提取指定 step/year/key 的 float64 值
// 与 report.go 的 getStepValue 不同：缺失时返回 NaN 而非 0，以便调用方区分
func getStepValueSafe(steps []StepResult, stepNum int, year, key string) float64 {
	for _, step := range steps {
		if step.StepNum != stepNum {
			continue
		}
		yd, ok := step.YearlyData[year]
		if !ok {
			return math.NaN()
		}
		v, ok := yd[key]
		if !ok {
			return math.NaN()
		}
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		case int64:
			return float64(val)
		case string:
			// 尝试解析百分比字符串如 "15.2%"
			var f float64
			if _, err := fmt.Sscanf(val, "%f%%", &f); err == nil {
				return f / 100
			}
		}
	}
	return math.NaN()
}
