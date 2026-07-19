package downloader

import (
	"testing"
)

// TestDetectEastMoneyOffset 验证偏移检测逻辑
func TestDetectEastMoneyOffset(t *testing.T) {
	cases := []struct {
		name     string
		first    string
		expected int
	}{
		{
			name:     "标准格式（fields2生效，开盘含小数）",
			first:    "2024-01-02,123.45,125.00,126.00,122.50,10000,1234500.00,2.84,1.21,1.50,0.80",
			expected: 0,
		},
		{
			name:     "标准格式（开盘为整数但收盘含小数）",
			first:    "2024-01-02,100,101,102,99,50000,5000000.00,3.00,1.00,1.00,1.00,1.20",
			expected: 0,
		},
		{
			name:     "偏移格式（fields2未生效，parts[1]为天数计数）",
			first:    "2024-01-02,1,123.45,125.00,122.50,126.00,10000,1234500.00,2.84,1.21,1.50,0.80",
			expected: 1,
		},
		{
			name:     "字段不足",
			first:    "2024-01-02,123.45,125.00",
			expected: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := detectEastMoneyOffset(c.first)
			if got != c.expected {
				t.Errorf("detectEastMoneyOffset(%q) = %d, want %d", c.first, got, c.expected)
			}
		})
	}
}

// TestParseEastMoneyKlinesStandardFormat 验证标准格式（offset=0）解析正确
func TestParseEastMoneyKlinesStandardFormat(t *testing.T) {
	// 原始数据来自真实API返回：日期,开盘,收盘,最高,最低,成交量,成交额,振幅,涨跌幅,涨跌额,换手率
	lines := []string{
		"2014-01-23,1.38,2.36,2.36,1.38,23664,66310791.00,163.33,293.33,1.76,11.49",
		"2014-01-24,2.36,2.93,2.93,2.36,13180,41043703.00,24.15,24.15,0.57,6.40",
	}

	result := parseEastMoneyKlines(lines)
	if len(result) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(result))
	}

	first := result[0]
	if first.Open != 1.38 {
		t.Errorf("Open = %.2f, want 1.38", first.Open)
	}
	if first.Close != 2.36 {
		t.Errorf("Close = %.2f, want 2.36", first.Close)
	}
	// 关键校验：标准格式中 parts[3]=最高(2.36), parts[4]=最低(1.38)
	if first.High != 2.36 {
		t.Errorf("High = %.2f, want 2.36", first.High)
	}
	if first.Low != 1.38 {
		t.Errorf("Low = %.2f, want 1.38", first.Low)
	}
	if first.Volume != 23664 {
		t.Errorf("Volume = %.0f, want 23664", first.Volume)
	}
	if first.TurnoverRate != 11.49 {
		t.Errorf("TurnoverRate = %.2f, want 11.49", first.TurnoverRate)
	}

	second := result[1]
	if second.High != 2.93 {
		t.Errorf("High = %.2f, want 2.93", second.High)
	}
	if second.Low != 2.36 {
		t.Errorf("Low = %.2f, want 2.36", second.Low)
	}
}

// TestParseEastMoneyKlinesOffsetFormat 验证偏移格式（offset=1）解析正确
func TestParseEastMoneyKlinesOffsetFormat(t *testing.T) {
	// 偏移格式：日期,天数计数,开盘,收盘,最低,最高,成交量,成交额,振幅,涨跌幅,涨跌额,换手率
	lines := []string{
		"2024-01-02,1,123.45,125.00,122.50,126.00,10000,1234500.00,2.84,1.21,1.50,0.80",
	}

	result := parseEastMoneyKlines(lines)
	if len(result) != 1 {
		t.Fatalf("expected 1 kline, got %d", len(result))
	}

	k := result[0]
	if k.Open != 123.45 {
		t.Errorf("Open = %.2f, want 123.45", k.Open)
	}
	if k.Close != 125.00 {
		t.Errorf("Close = %.2f, want 125.00", k.Close)
	}
	// 偏移格式中 parts[4]=最低(122.50), parts[5]=最高(126.00)
	if k.Low != 122.50 {
		t.Errorf("Low = %.2f, want 122.50", k.Low)
	}
	if k.High != 126.00 {
		t.Errorf("High = %.2f, want 126.00", k.High)
	}
}

// TestParseSinaIndexKlines 验证新浪 jsonp 包裹的指数日线解析正确
func TestParseSinaIndexKlines(t *testing.T) {
	// 原始格式来自真实API返回（bj899050）：jsonp 前缀 + var _k=([...]);
	body := []byte(`/*<script>location.href='//sina.com';</script>*/
var _k=([{"day":"2022-05-05","open":"1000.585","high":"1036.860","low":"993.928","close":"1031.315","volume":"49876843"},{"day":"2022-05-06","open":"1031.000","high":"1040.000","low":"1010.500","close":"1020.250","volume":"30000000"}]);`)

	result, err := parseSinaIndexKlines(body)
	if err != nil {
		t.Fatalf("parseSinaIndexKlines error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(result))
	}

	first := result[0]
	if first.Time != "2022-05-05" {
		t.Errorf("Time = %s, want 2022-05-05", first.Time)
	}
	if first.Open != 1000.585 {
		t.Errorf("Open = %.3f, want 1000.585", first.Open)
	}
	if first.Close != 1031.315 {
		t.Errorf("Close = %.3f, want 1031.315", first.Close)
	}
	if first.High != 1036.860 {
		t.Errorf("High = %.3f, want 1036.860", first.High)
	}
	if first.Low != 993.928 {
		t.Errorf("Low = %.3f, want 993.928", first.Low)
	}
	if first.Volume != 49876843 {
		t.Errorf("Volume = %.0f, want 49876843", first.Volume)
	}
}

// TestParseSinaIndexKlinesBadInput 验证异常输入返回错误而非 panic
func TestParseSinaIndexKlinesBadInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"空响应", ""},
		{"无jsonp包裹", `{"day":"2022-05-05"}`},
		{"null数据", `var _k=(null);`},
		{"全为异常价格", `var _k=([{"day":"2022-05-05","open":"0","high":"0","low":"0","close":"0","volume":"0"}]);`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parseSinaIndexKlines([]byte(c.body)); err == nil {
				t.Errorf("expected error for %s, got nil", c.name)
			}
		})
	}
}
