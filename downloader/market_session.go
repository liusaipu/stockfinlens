package downloader

import (
	"sync"
	"time"
)

// shanghaiLocation 上海时区（中国大陆股市时区）。失败时退化到 UTC+8 固定偏移。
var (
	shanghaiLocOnce sync.Once
	shanghaiLoc     *time.Location
)

func shanghaiLocation() *time.Location {
	shanghaiLocOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err == nil {
			shanghaiLoc = loc
		} else {
			shanghaiLoc = time.FixedZone("CST", 8*3600)
		}
	})
	return shanghaiLoc
}

// IsTradingHours 是否处于 A 股交易时段（不处理法定节假日）
// 交易时段：周一~周五，09:30-11:30 或 13:00-15:00 上海时间
func IsTradingHours(now time.Time) bool {
	t := now.In(shanghaiLocation())
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	mins := t.Hour()*60 + t.Minute()
	const (
		morningOpen    = 9*60 + 30
		morningClose   = 11*60 + 30
		afternoonOpen  = 13 * 60
		afternoonClose = 15 * 60
	)
	return (mins >= morningOpen && mins < morningClose) ||
		(mins >= afternoonOpen && mins < afternoonClose)
}

// NextPreOpen 返回下一次「开盘前 5 分钟」（即 09:25）的上海时间
// 用作非交易时段的缓存过期点：盘前 5 分钟内重新拉数据，确保开盘瞬间能立刻看到新数据
func NextPreOpen(now time.Time) time.Time {
	t := now.In(shanghaiLocation())
	preOpen := time.Date(t.Year(), t.Month(), t.Day(), 9, 25, 0, 0, shanghaiLocation())
	if !t.Before(preOpen) {
		// 已过今天 9:25，推到明天
		preOpen = preOpen.Add(24 * time.Hour)
	}
	// 跳过周末
	for {
		wd := preOpen.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			break
		}
		preOpen = preOpen.Add(24 * time.Hour)
	}
	return preOpen
}
