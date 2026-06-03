package analyzer

import (
	"fmt"
	"sort"
)

// Position 单只股票持仓
type Position struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	CostPrice    float64 `json:"costPrice"`    // 成本价
	Shares       float64 `json:"shares"`       // 持股数量
	CurrentPrice float64 `json:"currentPrice"` // 当前价
	MarketValue  float64 `json:"marketValue"`  // 市值
	Profit       float64 `json:"profit"`       // 盈亏金额
	ProfitPct    float64 `json:"profitPct"`    // 盈亏比例
	Weight       float64 `json:"weight"`       // 仓位占比（%）
	StopLoss     float64 `json:"stopLoss"`     // 止损价
	TakeProfit   float64 `json:"takeProfit"`   // 止盈价
	Industry     string  `json:"industry"`     // 行业
}

// Portfolio 投资组合
type Portfolio struct {
	Positions        []Position         `json:"positions"`
	TotalCost        float64            `json:"totalCost"`
	TotalValue       float64            `json:"totalValue"`
	TotalProfit      float64            `json:"totalProfit"`
	TotalProfitPct   float64            `json:"totalProfitPct"`
	IndustryExposure map[string]float64 `json:"industryExposure"` // 行业暴露（行业 -> 权重和）
	RiskAlerts       []PortfolioAlert   `json:"riskAlerts"`
}

// PortfolioAlert 组合层面的预警
type PortfolioAlert struct {
	Level         string `json:"level"` // high / medium / low
	Type          string `json:"type"`  // concentration / stop_loss / take_profit / rebalance
	Message       string `json:"message"`
	RelatedSymbol string `json:"relatedSymbol,omitempty"`
}

// BuildPortfolioAnalysis 构建组合分析
func BuildPortfolioAnalysis(positions []Position) *Portfolio {
	p := &Portfolio{
		Positions:        positions,
		IndustryExposure: make(map[string]float64),
	}

	if len(positions) == 0 {
		return p
	}

	for i := range positions {
		pos := &positions[i]
		pos.MarketValue = pos.CurrentPrice * pos.Shares
		pos.Profit = (pos.CurrentPrice - pos.CostPrice) * pos.Shares
		if pos.CostPrice > 0 {
			pos.ProfitPct = (pos.CurrentPrice - pos.CostPrice) / pos.CostPrice
		}
		p.TotalCost += pos.CostPrice * pos.Shares
		p.TotalValue += pos.MarketValue
	}

	if p.TotalCost > 0 {
		p.TotalProfit = p.TotalValue - p.TotalCost
		p.TotalProfitPct = p.TotalProfit / p.TotalCost
	}

	// 计算权重
	for i := range positions {
		if p.TotalValue > 0 {
			positions[i].Weight = positions[i].MarketValue / p.TotalValue * 100
		}
		if positions[i].Industry != "" {
			p.IndustryExposure[positions[i].Industry] += positions[i].Weight
		}
	}

	p.RiskAlerts = analyzePortfolioRisk(positions, p.TotalValue)
	return p
}

// analyzePortfolioRisk 分析组合风险并生成预警
func analyzePortfolioRisk(positions []Position, totalValue float64) []PortfolioAlert {
	var alerts []PortfolioAlert
	if len(positions) == 0 {
		return alerts
	}

	// 1. 集中度检查：单只股票权重超过 30%
	for _, pos := range positions {
		if pos.Weight > 30 {
			alerts = append(alerts, PortfolioAlert{
				Level:         "high",
				Type:          "concentration",
				Message:       fmt.Sprintf("%s 仓位占比 %.1f%%，超过 30%% 集中度警戒线", pos.Symbol, pos.Weight),
				RelatedSymbol: pos.Symbol,
			})
		} else if pos.Weight > 20 {
			alerts = append(alerts, PortfolioAlert{
				Level:         "medium",
				Type:          "concentration",
				Message:       fmt.Sprintf("%s 仓位占比 %.1f%%，建议分散", pos.Symbol, pos.Weight),
				RelatedSymbol: pos.Symbol,
			})
		}
	}

	// 2. 止损/止盈检查
	for _, pos := range positions {
		if pos.StopLoss > 0 && pos.CurrentPrice <= pos.StopLoss {
			alerts = append(alerts, PortfolioAlert{
				Level:         "high",
				Type:          "stop_loss",
				Message:       fmt.Sprintf("%s 已触及止损价 %.2f（当前 %.2f）", pos.Symbol, pos.StopLoss, pos.CurrentPrice),
				RelatedSymbol: pos.Symbol,
			})
		}
		if pos.TakeProfit > 0 && pos.CurrentPrice >= pos.TakeProfit {
			alerts = append(alerts, PortfolioAlert{
				Level:         "medium",
				Type:          "take_profit",
				Message:       fmt.Sprintf("%s 已触及止盈价 %.2f（当前 %.2f），建议分批减仓", pos.Symbol, pos.TakeProfit, pos.CurrentPrice),
				RelatedSymbol: pos.Symbol,
			})
		}
	}

	// 3. 再平衡建议：如果最高权重与最低权重的差距超过 20pct
	sorted := make([]Position, len(positions))
	copy(sorted, positions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Weight > sorted[j].Weight
	})
	if len(sorted) >= 2 && sorted[0].Weight-sorted[len(sorted)-1].Weight > 20 {
		alerts = append(alerts, PortfolioAlert{
			Level: "medium",
			Type:  "rebalance",
			Message: fmt.Sprintf("仓位分化明显：最高 %.1f%% (%s) vs 最低 %.1f%% (%s)，建议再平衡",
				sorted[0].Weight, sorted[0].Symbol,
				sorted[len(sorted)-1].Weight, sorted[len(sorted)-1].Symbol),
		})
	}

	return alerts
}
