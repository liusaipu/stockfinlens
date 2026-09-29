package downloader

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// balanceSheetMap HSF10 资产负债表字段 -> 标准中文名
var balanceSheetMap = map[string]string{
	"TOTAL_ASSETS":                 "资产合计",
	"TOTAL_LIABILITIES":            "负债合计",
	"TOTAL_EQUITY":                 "所有者权益合计",
	"MONETARYFUNDS":                "货币资金",
	"TRADE_FINASSET_NOTFVTPL":      "交易性金融资产",
	"NOTES_RECEIVABLES":            "应收票据",
	"ACCOUNTS_RECE":                "应收账款",
	"PREPAYMENT":                   "预付款项",
	"CONTRACT_ASSET":               "合同资产",
	"INVENTORY":                    "存货",
	"TOTAL_CURRENT_ASSETS":         "流动资产合计",
	"FIXED_ASSET":                  "固定资产",
	"CIP":                          "在建工程",
	"CONSTRUCT_MATERIAL":           "工程物资",
	"INTANGIBLE_ASSET":             "无形资产",
	"GOODWILL":                     "商誉",
	"LT_EQUITY_INVEST":             "长期股权投资",
	"OTHER_EQUITY_INVEST":          "其他权益工具投资",
	"OTHER_NONCURRENT_FINASSET":    "其他非流动金融资产",
	"AVAILABLE_SALE_FINASSET":      "可供出售金融资产",
	"HELD_MATURITY_INVEST":         "持有至到期投资",
	"INVEST_REALESTATE":            "投资性房地产",
	"TOTAL_NONCURRENT_ASSETS":      "非流动资产合计",
	"SHORT_LOAN":                   "短期借款",
	"NONCURRENT_LIAB_DUE_IN1Y":     "一年内到期的非流动负债",
	"LONG_LOAN":                    "长期借款",
	"BONDS_PAYABLE":                "应付债券",
	"LONG_PAYABLE":                 "长期应付款",
	"NOTES_PAYABLE":                "应付票据",
	"ACCOUNTS_PAYABLE":             "应付账款",
	"ADVANCE_RECEIVABLES":          "预收款项",
	"CONTRACT_LIABILITIES":         "合同负债",
	"SALARY_PAYABLE":               "应付职工薪酬",
	"TAX_PAYABLE":                  "应交税费",
	"TOTAL_CURRENT_LIABILITIES":    "流动负债合计",
	"TOTAL_NONCURRENT_LIABILITIES": "非流动负债合计",
	"DEFER_TAX_ASSET":              "递延所得税资产",
	"DEFER_TAX_LIABILITIES":        "递延所得税负债",
	"SHARE_CAPITAL":                "实收资本（或股本）",
	"CAPITAL_RESERVE":              "资本公积",
	"SURPLUS_RESERVE":              "盈余公积",
	"RETAINED_EARNINGS":            "未分配利润",
	"UNASSIGN_RPOFIT":              "未分配利润",
	"PARENT_EQUITY_BALANCE":        "归属于母公司所有者权益合计",
	"TOTAL_PARENT_EQUITY":          "归属于母公司所有者权益合计",
	"MINORITY_EQUITY":              "少数股东权益",
}

// incomeStatementMap HSF10 利润表字段 -> 标准中文名
var incomeStatementMap = map[string]string{
	"TOTAL_OPERATE_INCOME":    "营业总收入",
	"OPERATE_INCOME":          "营业收入",
	"TOTAL_OPERATE_COST":      "营业总成本",
	"OPERATE_COST":            "营业成本",
	"OPERATE_TAX_ADD":         "税金及附加",
	"SALE_EXPENSE":            "销售费用",
	"MANAGE_EXPENSE":          "管理费用",
	"RESEARCH_EXPENSE":        "研发费用",
	"FINANCE_EXPENSE":         "财务费用",
	"FE_INTEREST_EXPENSE":     "其中：利息费用",
	"FE_INTEREST_INCOME":      "利息收入",
	"ASSET_IMPAIRMENT_LOSS":   "资产减值损失",
	"CREDIT_IMPAIRMENT_LOSS":  "信用减值损失",
	"INVEST_INCOME":           "投资收益",
	"INVEST_JOINT_INCOME":     "其中：对联营企业和合营企业的投资收益",
	"FAIRVALUE_CHANGE_INCOME": "公允价值变动收益",
	"ASSET_DISPOSAL_INCOME":   "资产处置收益",
	"OTHER_INCOME":            "其他收益",
	"OPERATE_PROFIT":          "营业利润",
	"NONBUSINESS_INCOME":      "营业外收入",
	"NONBUSINESS_EXPENSE":     "营业外支出",
	"TOTAL_PROFIT":            "利润总额",
	"INCOME_TAX":              "所得税费用",
	"NETPROFIT":               "净利润",
	"PARENT_NETPROFIT":        "归属于母公司所有者的净利润",
	"MINORITY_INTEREST":       "少数股东损益",
	"DEDUCT_PARENT_NETPROFIT": "扣除非经常性损益后的净利润",
	"BASIC_EPS":               "基本每股收益",
	"DILUTED_EPS":             "稀释每股收益",
	"OTHER_COMPRE_INCOME":     "其他综合收益",
	"TOTAL_COMPRE_INCOME":     "综合收益总额",
}

// cashFlowMap HSF10 现金流量表字段 -> 标准中文名
var cashFlowMap = map[string]string{
	"CCE_ADD":                 "现金及现金等价物净增加额",
	"NETCASH_OPERATE":         "经营活动产生的现金流量净额",
	"NETCASH_INVEST":          "投资活动产生的现金流量净额",
	"NETCASH_FINANCE":         "筹资活动产生的现金流量净额",
	"END_CCE":                 "期末现金及现金等价物余额",
	"BEGIN_CCE":               "期初现金及现金等价物余额",
	"SALES_SERVICES":          "销售商品、提供劳务收到的现金",
	"BUY_SERVICES":            "购买商品、接受劳务支付的现金",
	"PAY_STAFF_CASH":          "支付给职工以及为职工支付的现金",
	"PAY_TAX":                 "支付的各项税费",
	"PAY_OTHER_OPERATE":       "支付其他与经营活动有关的现金",
	"RECEIVE_INVEST_INCOME":   "取得投资收益收到的现金",
	"DISPOSAL_LONG_ASSET":     "处置固定资产无形资产和其他长期资产收回的现金净额",
	"CONSTRUCT_LONG_ASSET":    "购建固定资产无形资产和其他长期资产支付的现金",
	"INVEST_PAY_CASH":         "投资支付的现金",
	"SUBSIDIARY_PAY":          "取得子公司及其他营业单位支付的现金净额",
	"ACCEPT_INVEST_CASH":      "吸收投资收到的现金",
	"RECEIVE_LOAN_CASH":       "取得借款收到的现金",
	"PAY_DEBT_CASH":           "偿还债务支付的现金",
	"ASSIGN_DIVIDEND_PORFIT":  "分配股利、利润或偿付利息支付的现金",
	"SUBSIDIARY_PAY_DIVIDEND": "其中：子公司支付给少数股东的股利、利润",
	"PAY_OTHER_FINANCE":       "支付其他与筹资活动有关的现金",
	"RATE_CHANGE_EFFECT":      "汇率变动对现金及现金等价物的影响",
	"FA_IR_DEPR":              "固定资产折旧、油气资产折耗、生产性生物资产折旧",
	"OILGAS_BIOLOGY_DEPR":     "固定资产折旧、油气资产折耗、生产性生物资产折旧",
}

func mergeBalanceSheet(target map[string]map[string]float64, src map[string]any, year string) {
	for k, stdName := range balanceSheetMap {
		v := extractFloat(src[k])
		if _, ok := target[stdName]; !ok {
			target[stdName] = make(map[string]float64)
		}
		target[stdName][year] = v
	}

	// 修复：东财新版 API 中 PARENT_EQUITY_BALANCE 常为 0，归母权益应使用 TOTAL_PARENT_EQUITY
	if v, ok := src["TOTAL_PARENT_EQUITY"]; ok {
		if totalParentEquity := extractFloat(v); math.Abs(totalParentEquity) > 1 {
			target["归属于母公司所有者权益合计"][year] = totalParentEquity
		}
	}

	// 修复：东财 API 中未分配利润字段为 UNASSIGN_RPOFIT，RETAINED_EARNINGS 可能为 0
	if v, ok := src["UNASSIGN_RPOFIT"]; ok {
		if unassignProfit := extractFloat(v); math.Abs(unassignProfit) > 1 {
			target["未分配利润"][year] = unassignProfit
		}
	}

	totalEquity := extractFloat(src["TOTAL_EQUITY"])
	minorityEquity := extractFloat(src["MINORITY_EQUITY"])
	parentEquity := target["归属于母公司所有者权益合计"][year]

	// 兜底：如果归母权益仍为 0 但总权益有值，用总权益 - 少数股东权益推导
	if math.Abs(parentEquity) < 1 && totalEquity != 0 {
		calculatedParent := totalEquity - minorityEquity
		if math.Abs(calculatedParent) > 1 {
			target["归属于母公司所有者权益合计"][year] = calculatedParent
		}
	}

	// 反向修复：TOTAL_EQUITY 为 0 但归母权益有值时
	if math.Abs(totalEquity) < 1 && math.Abs(parentEquity) > 1 {
		calculatedTotal := parentEquity + minorityEquity
		if math.Abs(calculatedTotal) > 1 {
			target["所有者权益合计"][year] = calculatedTotal
		}
	}

	// 额外计算：应付票据及应付账款 = 应付票据 + 应付账款
	// 应收票据及应收账款 = 应收票据 + 应收账款
	notesReceivable := extractFloat(src["NOTES_RECEIVABLES"])
	accountsRece := extractFloat(src["ACCOUNTS_RECE"])
	receivableTotal := notesReceivable + accountsRece
	if _, ok := target["应收票据及应收账款"]; !ok {
		target["应收票据及应收账款"] = make(map[string]float64)
	}
	target["应收票据及应收账款"][year] = receivableTotal

	notesPayable := extractFloat(src["NOTES_PAYABLE"])
	accountsPay := extractFloat(src["ACCOUNTS_PAYABLE"])
	payableTotal := notesPayable + accountsPay
	if _, ok := target["应付票据及应付账款"]; !ok {
		target["应付票据及应付账款"] = make(map[string]float64)
	}
	target["应付票据及应付账款"][year] = payableTotal
}

func mergeIncomeStatement(target map[string]map[string]float64, src map[string]any, year string) {
	for k, stdName := range incomeStatementMap {
		v := extractFloat(src[k])
		if _, ok := target[stdName]; !ok {
			target[stdName] = make(map[string]float64)
		}
		target[stdName][year] = v
	}
}

func mergeCashFlow(target map[string]map[string]float64, src map[string]any, year string) {
	for k, stdName := range cashFlowMap {
		v := extractFloat(src[k])
		if _, ok := target[stdName]; !ok {
			target[stdName] = make(map[string]float64)
		}
		// 对于折旧字段，如果已存在且当前值为0，不覆盖已有的非0值
		if (k == "FA_IR_DEPR" || k == "OILGAS_BIOLOGY_DEPR") && v == 0 && target[stdName][year] != 0 {
			continue
		}
		target[stdName][year] = v
	}
}

func extractFloat(v any) float64 {
	if v == nil {
		return 0
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
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		// 尝试 fmt.Sprint 后再解析
		f, _ := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		return f
	}
}
