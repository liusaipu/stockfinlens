package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ========== 东方财富个股资金流向 ==========

// eastMoneyMoneyflowResp 东财资金流向接口响应结构
type eastMoneyMoneyflowResp struct {
	Rc   int   `json:"rc"`
	Rt   int   `json:"rt"`
	Svr  int64 `json:"svr"`
	Lt   int   `json:"lt"`
	Full int   `json:"full"`
	Data struct {
		Code   string   `json:"code"`
		Market int      `json:"market"`
		Name   string   `json:"name"`
		Klines []string `json:"klines"`
	} `json:"data"`
}

// fetchMoneyflowFromEastMoney 从东方财富获取个股历史资金流向
// secid 格式: 0.code(深圳), 1.code(上海)
// 返回数据按日期升序排列，需反转
func fetchMoneyflowFromEastMoney(ctx context.Context, market, code, startDate, endDate string) ([]SFLMoneyflowItem, error) {
	if strings.ToUpper(market) == "HK" {
		return nil, fmt.Errorf("东方财富资金流向接口暂不支持港股")
	}
	secid := toEastMoneySecid(market, code)
	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get?"+
			"secid=%s&lmt=0&klt=101&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56&"+
			"ut=b2884a393a59ad64002292a3e90d46a5",
		secid)

	body, err := httpGetEastMoney(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("东财资金流向请求失败: %w", err)
	}

	var result eastMoneyMoneyflowResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("东财资金流向解析失败: %w", err)
	}

	if result.Rc != 0 || len(result.Data.Klines) == 0 {
		return nil, fmt.Errorf("东财资金流向无数据: rc=%d, klines=%d", result.Rc, len(result.Data.Klines))
	}

	// 解析 klines，过滤日期范围
	items := make([]SFLMoneyflowItem, 0, len(result.Data.Klines))
	for _, line := range result.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}

		// 日期格式 YYYY-MM-DD → YYYYMMDD
		rawDate := strings.TrimSpace(parts[0])
		tradeDate := strings.ReplaceAll(rawDate, "-", "")
		if tradeDate < startDate || tradeDate > endDate {
			continue
		}

		// 东财返回的字段: 日期,主力净流入,小单净流入,中单净流入,大单净流入,超大单净流入
		// 注意东财的"主力"=超大+大，"散户"=中+小
		netMain := parseFloatSafe(parts[1]) // 主力净流入 (超大+大)
		netSm := parseFloatSafe(parts[2])   // 小单净流入
		netMd := parseFloatSafe(parts[3])   // 中单净流入
		netLg := parseFloatSafe(parts[4])   // 大单净流入
		netElg := parseFloatSafe(parts[5])  // 超大单净流入

		// 主力净流入 = 超大单净流入 + 大单净流入，做一下校验
		if netMain == 0 && (netLg != 0 || netElg != 0) {
			netMain = netLg + netElg
		}

		// 转换为 SFLMoneyflowItem 格式（只保留净流入关系即可，Buy-Sell = Net）
		items = append(items, SFLMoneyflowItem{
			TsCode:        toTsCode(market, code),
			TradeDate:     tradeDate,
			BuySmAmount:   maxFloat(netSm, 0),
			SellSmAmount:  maxFloat(-netSm, 0),
			BuyMdAmount:   maxFloat(netMd, 0),
			SellMdAmount:  maxFloat(-netMd, 0),
			BuyLgAmount:   maxFloat(netLg, 0),
			SellLgAmount:  maxFloat(-netLg, 0),
			BuyElgAmount:  maxFloat(netElg, 0),
			SellElgAmount: maxFloat(-netElg, 0),
			NetMfAmount:   netMain,
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("东财资金流向在 %s~%s 范围内无数据", startDate, endDate)
	}

	// 东财返回按日期升序，反转成降序（最新在前）
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	fmt.Printf("[EastMoney.FetchMoneyflow] %s %s~%s => %d records\n", secid, startDate, endDate, len(items))
	return items, nil
}

// toEastMoneySecid 将 market+code 转为东财 secid 格式
// 深圳: 0.code, 上海: 1.code, 港股: 116.code(未验证)
func toEastMoneySecid(market, code string) string {
	m := strings.ToUpper(market)
	switch m {
	case "SZ":
		return "0." + code
	case "SH":
		return "1." + code
	case "HK":
		return "116." + code
	default:
		return "0." + code
	}
}

func parseFloatSafe(s string) float64 {
	s = strings.TrimSpace(s)
	// 去除千分位逗号（东财部分接口返回数值可能带逗号，如 "1,234.56"）
	s = strings.ReplaceAll(s, ",", "")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
