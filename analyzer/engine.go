package analyzer

import (
	"fmt"
	"strings"
	"time"
)

// AnalysisOptions 控制 RunAnalysis 集成哪些附加数据。零值合法（=纯财务透视）。
// 新增维度时只需在此 struct 加字段，无需新增 RunAnalysisWithXxx 包装函数。
type AnalysisOptions struct {
	Comparable  *ComparableAnalysis
	Quote       *QuoteData
	Sentiment   *SentimentData
	Policy      *PolicyMatchData
	Technical   *TechnicalData
	Activity    *ActivityData
	Moneyflow   *MoneyflowData
	ML          *MLPredictionData
	RIM         *RIMData
	Extras      map[string]float64
	External    *ExternalRiskData
	Sensitivity SensitivityLevel // 零值视为 SensitivityStandard
	CompanyName string           // 公司名称，用于报告标题；为空时回退到 symbol
}

// RunAnalysis 执行完整的财报透视分析，返回报告。
// 零值 opts 等价于纯财务透视（不集成行情/舆情/ML/可比公司等附加数据）。
func RunAnalysis(baseDir, symbol string, opts AnalysisOptions) (*AnalysisReport, error) {
	// Sensitivity 零值兜底（SensitivityLevel 是 string，零值为 ""）
	if opts.Sensitivity == "" {
		opts.Sensitivity = SensitivityStandard
	}

	data, err := LoadFinancialData(baseDir, symbol)
	if err != nil {
		return nil, fmt.Errorf("load financial data: %w", err)
	}
	if len(data.Years) == 0 {
		return nil, fmt.Errorf("no financial data available for %s", symbol)
	}
	// 将传入的 extras 回填到 data.Extras，供 step8RiskAnalysis 等使用
	if len(opts.Extras) > 0 {
		if data.Extras == nil {
			data.Extras = make(map[string]float64)
		}
		for k, v := range opts.Extras {
			data.Extras[k] = v
		}
	}

	steps := []StepResult{
		step1Audit(data),
		step2AssetScale(data),
		step3Solvency(data),
		step4CompetitivePosition(data),
		step5Receivables(data),
		step6FixedAssets(data),
		step7InvestmentAssets(data),
		step8RiskAnalysis(data),
		step9RevenueGrowth(data),
		step10GrossMargin(data),
		step11OperationEfficiency(data),
		step12CostControl(data),
		step13ExpenseRatio(data),
		step14CoreProfit(data),
		step15CashFlowQuality(data),
		step16ROE(data),
		step17CAPEX(data),
		step18Dividend(data),
	}

	scores := Evaluate(data, steps)

	passSummary := make(map[string][]PassItem)
	for _, step := range steps {
		for _, year := range data.Years {
			p, ok := step.Pass[year]
			var val any
			if yd, ok2 := step.YearlyData[year]; ok2 {
				// 尝试找一个数值型 key 作为展示值
				for k, v := range yd {
					if k != "status" && k != "competitiveness" && k != "risk" && k != "companyType" && k != "focus" && k != "control" && k != "innovation" && k != "salesDifficulty" && k != "profitability" && k != "quality" && k != "assessment" && k != "sustainability" && k != "note" && k != "fraudRisk" {
						val = v
						break
					}
				}
			}
			if !ok {
				p = true
			}
			passSummary[year] = append(passSummary[year], PassItem{
				Year:   year,
				Passed: p,
				Value:  val,
			})
		}
	}

	scoreMap := make(map[string]float64)
	overallGrade := ""
	if len(data.Years) > 0 {
		latest := data.Years[0]
		if s, ok := scores[latest]; ok {
			scoreMap[latest] = s.RawScore
			overallGrade = s.Grade
		}
	}

	// 生成 ML 综合预测摘要（融入 A-Score）
	if opts.ML != nil {
		ascore := getAScore(steps[7].YearlyData, data.Years)
		opts.ML.Summary = BuildMLSummary(opts.ML, opts.Technical, opts.Activity, opts.Sentiment, ascore)
	}

	// 行业均值对比
	var industry *IndustryComparison
	if opts.Policy != nil && opts.Policy.Industry != "" {
		industry = CompareWithIndustry(opts.Policy.Industry, steps, data.Years[0])
	}

	// 构建风险警示摘要
	riskAlert := BuildRiskAlertSummary(steps, opts.Extras, data.Years, opts.External, opts.Sensitivity)

	// 季度滚动预警 + TTM
	quarterlyAlert := BuildQuarterlyAlert(data)
	ttmMetrics := BuildTTMMetrics(data)

	companyName := opts.CompanyName
	if companyName == "" {
		companyName = symbol
	}
	md := GenerateMarkdown(companyName, symbol, data.Years, steps, scores, opts.Comparable, industry, opts.Quote, opts.Sentiment, opts.Policy, opts.Technical, opts.Activity, opts.Moneyflow, opts.ML, opts.RIM, riskAlert, data.QualityWarnings, nil, quarterlyAlert, ttmMetrics)

	hr := ExtractHighlightsAndRisks(steps, data.Years)

	report := &AnalysisReport{
		Symbol:          symbol,
		CompanyName:     companyName,
		GeneratedAt:     time.Now().Format("2006-01-02 15:04"),
		Years:           data.Years,
		StepResults:     steps,
		PassSummary:     passSummary,
		Score:           scoreMap,
		ScoreDetails:    scores,
		OverallGrade:    overallGrade,
		MarkdownContent: md,
		RIM:             opts.RIM,
		Highlights:      hr.Highlights,
		Risks:           hr.Risks,
		RiskAlert:       riskAlert,
		QualityWarnings: data.QualityWarnings,
		QuarterlyAlert:  quarterlyAlert,
		TTMMetrics:      ttmMetrics,
	}
	if opts.Sentiment != nil {
		report.InteractQAs = opts.Sentiment.InteractQAs
	}
	return report, nil
}

// RegenerateModule4Only 仅重新生成模块4（行业横向对比分析）
// 用于可比公司数据更新后，只更新报告中的模块4部分，不重新获取网络数据
func RegenerateModule4Only(baseDir, symbol string, comp *ComparableAnalysis, industry *IndustryComparison) (string, error) {
	data, err := LoadFinancialData(baseDir, symbol)
	if err != nil {
		return "", fmt.Errorf("load financial data: %w", err)
	}
	if len(data.Years) == 0 {
		return "", fmt.Errorf("no financial data available for %s", symbol)
	}

	steps := []StepResult{
		step1Audit(data),
		step2AssetScale(data),
		step3Solvency(data),
		step4CompetitivePosition(data),
		step5Receivables(data),
		step6FixedAssets(data),
		step7InvestmentAssets(data),
		step8RiskAnalysis(data),
		step9RevenueGrowth(data),
		step10GrossMargin(data),
		step11OperationEfficiency(data),
		step12CostControl(data),
		step13ExpenseRatio(data),
		step14CoreProfit(data),
		step15CashFlowQuality(data),
		step16ROE(data),
		step17CAPEX(data),
		step18Dividend(data),
	}

	activityScore := loadActivityScore(baseDir, symbol)

	var b strings.Builder
	writeModule4(&b, steps, data.Years[0], comp, industry, activityScore)
	return b.String(), nil
}
