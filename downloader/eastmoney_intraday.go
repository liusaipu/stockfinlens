package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// IntradayPoint 一根分时数据点
type IntradayPoint struct {
	Time   string  `json:"time"`   // "HH:MM"
	Price  float64 `json:"price"`  // 当前价（即该 1min bar 的 close）
	AvgPx  float64 `json:"avgPx"`  // 均价
	Volume float64 `json:"volume"` // 成交量（股，已从"手"换算）
	Amount float64 `json:"amount"` // 成交额（元）
}

// IntradayData 单只股票的当日/上一交易日分时数据
type IntradayData struct {
	Date        string          `json:"date"`      // 该数据所属交易日 "YYYY-MM-DD"
	PrevClose   float64         `json:"prevClose"` // 昨收，画水平参考线 + 算涨跌幅
	Points      []IntradayPoint `json:"points"`
	IsRealtime  bool            `json:"isRealtime"`  // true=当日盘中实时，false=历史/盘后
	LastUpdated string          `json:"lastUpdated"` // RFC3339，最后一个数据点的 wall time
}

// FetchIntradayFromEastMoney 拉取东方财富 trends2 分时接口
// 盘中：返回今日截至当前的实时数据；盘后/非交易日：返回最近一个交易日的完整 240 点数据
func FetchIntradayFromEastMoney(ctx context.Context, market, code string) (*IntradayData, error) {
	var secid string
	switch strings.ToUpper(market) {
	case "SH":
		secid = "1." + code
	case "SZ":
		secid = "0." + code
	case "HK":
		secid = "116." + code
	default:
		secid = "0." + code
	}

	// trends2 接口被服务端严格校验：fields1 必须是连续的 f1..f14（少一个或跳号会直接 close 连接 → EOF），
	// 且 trends2 需要专用 ut=f057cbcbce2a86e2866ab8877db1d059（与 kline 的 ut 不同；混用会被 reject）。
	// fields2: f51 时间 / f52 开 / f53 收 / f54 高 / f55 低 / f56 成交量(手) / f57 成交额 / f58 均价
	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/trends2/get?fields1=f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13,f14&fields2=f51,f52,f53,f54,f55,f56,f57,f58&ut=f057cbcbce2a86e2866ab8877db1d059&ndays=1&iscr=0&iscca=0&secid=%s",
		secid,
	)

	// trends2 接口的 EOF 是 path-specific 反爬，不是网络抖动 → 重试救不回来。
	// 用 1 次尝试快速失败，让上层立刻退到腾讯，避免 ~9s 的指数退避总耗时。
	body, err := httpGetWithRetry(ctx, url, "https://quote.eastmoney.com/", 1)
	if err != nil {
		return nil, fmt.Errorf("分时数据请求失败: %w", err)
	}

	var resp struct {
		RC   int `json:"rc"`
		Data *struct {
			Code     string   `json:"code"`
			Name     string   `json:"name"`
			PreClose float64  `json:"preClose"`
			Trends   []string `json:"trends"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("分时数据解析失败: %w", err)
	}
	if resp.Data == nil || len(resp.Data.Trends) == 0 {
		return nil, fmt.Errorf("分时数据为空")
	}

	out := &IntradayData{
		PrevClose: resp.Data.PreClose,
		Points:    make([]IntradayPoint, 0, len(resp.Data.Trends)),
	}

	for _, line := range resp.Data.Trends {
		parts := strings.Split(line, ",")
		if len(parts) < 8 {
			continue
		}
		// parts: [0]time "YYYY-MM-DD HH:MM" [1]open [2]close [3]high [4]low [5]vol(手) [6]amount [7]avg
		full := strings.TrimSpace(parts[0])
		// Date 来自第一个 point
		if out.Date == "" {
			if len(full) >= 10 {
				out.Date = full[:10]
			}
		}
		hm := ""
		if len(full) >= 16 {
			hm = full[11:16]
		}
		out.Points = append(out.Points, IntradayPoint{
			Time:   hm,
			Price:  parseStrFloat(parts[2]),
			AvgPx:  parseStrFloat(parts[7]),
			Volume: parseStrFloat(parts[5]) * 100.0, // 手 → 股
			Amount: parseStrFloat(parts[6]),
		})
	}

	// 判定是否实时：最后一个点的日期 == 今天，则为当日实时（盘中/盘后均可能 true）
	out.IsRealtime = out.Date == time.Now().In(shanghaiLocation()).Format("2006-01-02")
	out.LastUpdated = time.Now().Format(time.RFC3339)

	return out, nil
}

// parseStrFloat 在 eastmoney.go 已定义；这里仅占位以便单测时若拆分包能独立 build
var _ = strconv.ParseFloat
