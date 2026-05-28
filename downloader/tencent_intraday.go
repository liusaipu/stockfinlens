package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FetchIntradayFromTencent 从腾讯财经拉取分时数据
// 当东方财富 trends2 接口被反爬规则拦截（EOF）时作为 fallback 数据源
// 接口示例：https://web.ifzq.gtimg.cn/appstock/app/minute/query?code=sh600000
func FetchIntradayFromTencent(ctx context.Context, market, code string) (*IntradayData, error) {
	var tencentCode string
	switch strings.ToUpper(market) {
	case "SH":
		tencentCode = "sh" + code
	case "SZ":
		tencentCode = "sz" + code
	case "HK":
		tencentCode = "hk" + code
	default:
		tencentCode = "sz" + code
	}

	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/minute/query?code=%s", tencentCode)
	body, err := httpGetWithReferer(ctx, url, "https://stockapp.finance.qq.com/")
	if err != nil {
		return nil, fmt.Errorf("腾讯分时数据请求失败: %w", err)
	}

	// 腾讯响应实际结构（实测 web.ifzq.gtimg.cn 2026-05）：
	//   data.{code}.data 只有 {date, data:[...]}, 没有 prec
	//   昨收要从 data.{code}.qt.{code}[4] 拿（qt 数组是腾讯标准 quote 格式）
	//   每行只有 4 字段："HHMM price totalVol totalAmount"，**没有均价**，要自己算
	var resp struct {
		Code int `json:"code"`
		Data map[string]struct {
			Data struct {
				Date string   `json:"date"`
				Data []string `json:"data"`
			} `json:"data"`
			Qt map[string][]string `json:"qt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("腾讯分时数据解析失败: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("腾讯分时数据返回错误码 %d", resp.Code)
	}
	section, ok := resp.Data[tencentCode]
	if !ok || len(section.Data.Data) == 0 {
		return nil, fmt.Errorf("腾讯分时数据为空")
	}

	// 昨收：qt[4]（[0]=未知 [1]=name [2]=code [3]=current [4]=prev_close ...）
	var prevClose float64
	if qtArr, ok := section.Qt[tencentCode]; ok && len(qtArr) > 4 {
		prevClose, _ = strconv.ParseFloat(qtArr[4], 64)
	}
	if prevClose <= 0 {
		return nil, fmt.Errorf("腾讯分时数据缺少昨收价")
	}

	out := &IntradayData{
		PrevClose: prevClose,
		Points:    make([]IntradayPoint, 0, len(section.Data.Data)),
	}

	// 单分钟量/额 = 当前累积 - 上一根累积；均价 = 累积金额 / 累积成交股数
	var lastTotalVol, lastTotalAmt float64
	first := true
	for _, line := range section.Data.Data {
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		rawTime := parts[0]
		if len(rawTime) != 4 {
			continue
		}
		hm := rawTime[:2] + ":" + rawTime[2:]
		price, _ := strconv.ParseFloat(parts[1], 64)
		totalVol, _ := strconv.ParseFloat(parts[2], 64)
		totalAmt, _ := strconv.ParseFloat(parts[3], 64)

		var minuteVol, minuteAmt float64
		if first {
			minuteVol = totalVol
			minuteAmt = totalAmt
			first = false
		} else {
			minuteVol = totalVol - lastTotalVol
			minuteAmt = totalAmt - lastTotalAmt
			if minuteVol < 0 {
				minuteVol = 0
			}
			if minuteAmt < 0 {
				minuteAmt = 0
			}
		}
		lastTotalVol = totalVol
		lastTotalAmt = totalAmt

		// 均价 = 累积成交额 / 累积成交股数（totalVol 单位是"手"，× 100 → 股）
		var avg float64
		if totalVol > 0 {
			avg = totalAmt / (totalVol * 100.0)
		}

		out.Points = append(out.Points, IntradayPoint{
			Time:   hm,
			Price:  price,
			AvgPx:  avg,
			Volume: minuteVol * 100.0, // "手" → "股"，与东财源对齐
			Amount: minuteAmt,
		})
	}

	// 腾讯日期格式 "20241127" → "2024-11-27"
	if d := section.Data.Date; len(d) == 8 {
		out.Date = d[:4] + "-" + d[4:6] + "-" + d[6:]
	}
	out.IsRealtime = out.Date == time.Now().In(shanghaiLocation()).Format("2006-01-02")
	out.LastUpdated = time.Now().Format(time.RFC3339)
	return out, nil
}
