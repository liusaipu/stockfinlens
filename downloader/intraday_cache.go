package downloader

import (
	"sync"
	"time"
)

// IntradayCache 分时数据内存缓存
// TTL 策略：
//   - 交易时段：60s（盘中数据每分钟会刷新）
//   - 非交易时段：缓存到下一个交易日 09:25（盘前 5 分钟），中间不重复请求
type IntradayCache struct {
	mu    sync.RWMutex
	items map[string]intradayCacheItem
}

type intradayCacheItem struct {
	data    *IntradayData
	expires time.Time
}

func NewIntradayCache() *IntradayCache {
	return &IntradayCache{items: make(map[string]intradayCacheItem)}
}

// Get 返回有效缓存数据，或 nil
func (c *IntradayCache) Get(symbol string) *IntradayData {
	c.mu.RLock()
	defer c.mu.RUnlock()
	it, ok := c.items[symbol]
	if !ok {
		return nil
	}
	if time.Now().After(it.expires) {
		return nil
	}
	return it.data
}

// Put 写入缓存，根据当前是否交易时段自动计算过期时间
func (c *IntradayCache) Put(symbol string, data *IntradayData) {
	if data == nil {
		return
	}
	now := time.Now()
	var expires time.Time
	if IsTradingHours(now) {
		expires = now.Add(60 * time.Second)
	} else {
		expires = NextPreOpen(now)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[symbol] = intradayCacheItem{data: data, expires: expires}
}

// Clear 清空缓存（测试或异常恢复时用）
func (c *IntradayCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]intradayCacheItem)
}
