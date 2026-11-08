package updater

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

// TestCompareVersion 验证版本号比对逻辑
func TestCompareVersion(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.3.33", "1.3.33", 0},
		{"1.3.32", "1.3.33", -1},
		{"1.3.33", "1.3.32", 1},
		{"1.2.0", "1.10.0", -1},
		{"2.0.0", "1.99.99", 1},
		{"0.0.0", "0.0.1", -1},
		{"", "1.0.0", -1},
		{"1.0.0", "", 1},
	}

	for _, tt := range tests {
		got := compareVersion(tt.v1, tt.v2)
		if got != tt.want {
			t.Errorf("compareVersion(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

// TestParseVersion 验证版本号解析
func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.3.33", [3]int{1, 3, 33}},
		{"2.0.0", [3]int{2, 0, 0}},
		{"1.10", [3]int{1, 10, 0}},
		{"3", [3]int{3, 0, 0}},
		{"", [3]int{0, 0, 0}},
		{"1.a.3", [3]int{1, 0, 3}},
	}

	for _, tt := range tests {
		got := parseVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseVersion(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestFormatPublishedAt 验证发布时间格式化
func TestFormatPublishedAt(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-01-15T08:30:00Z", "2024-01-15"},
		{"2024-12-31T23:59:59+08:00", "2024-12-31"},
		{"invalid", "invalid"},
		{"", ""},
	}

	for _, tt := range tests {
		got := formatPublishedAt(tt.input)
		if got != tt.want {
			t.Errorf("formatPublishedAt(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestMatchPlatformAsset 验证平台 asset 匹配（macOS 优先 universal，其次 arm64）
func TestMatchPlatformAsset(t *testing.T) {
	assets := []githubAsset{
		{Name: "stockfinlens-macos-universal-v1.3.33.dmg", BrowserDownloadURL: "https://example.com/mac.dmg"},
		{Name: "stockfinlens-windows-amd64-v1.3.33.zip", BrowserDownloadURL: "https://example.com/win.zip"},
		{Name: "source-code.zip", BrowserDownloadURL: "https://example.com/src.zip"},
	}

	url, name := matchPlatformAsset(assets)

	// 根据当前平台验证
	switch runtime.GOOS {
	case "windows":
		if url != "https://example.com/win.zip" {
			t.Errorf("Windows 应匹配 win.zip, got %q", url)
		}
		if name != "stockfinlens-windows-amd64-v1.3.33.zip" {
			t.Errorf("Windows 资产名错误, got %q", name)
		}
	case "darwin":
		if url != "https://example.com/mac.dmg" {
			t.Errorf("macOS 应匹配 mac.dmg, got %q", url)
		}
		if name != "stockfinlens-macos-universal-v1.3.33.dmg" {
			t.Errorf("macOS 资产名错误, got %q", name)
		}
	default:
		if url != "" || name != "" {
			t.Errorf("其他平台应返回空, got url=%q name=%q", url, name)
		}
	}

	// 验证 universal 不存在时回退到 arm64
	if runtime.GOOS == "darwin" {
		arm64Assets := []githubAsset{
			{Name: "stockfinlens-macos-arm64-v1.3.33.dmg", BrowserDownloadURL: "https://example.com/mac-arm64.dmg"},
			{Name: "stockfinlens-windows-amd64-v1.3.33.zip", BrowserDownloadURL: "https://example.com/win.zip"},
		}
		url, name := matchPlatformAsset(arm64Assets)
		if url != "https://example.com/mac-arm64.dmg" {
			t.Errorf("macOS 无 universal 时应回退 arm64, got %q", url)
		}
		if name != "stockfinlens-macos-arm64-v1.3.33.dmg" {
			t.Errorf("macOS arm64 资产名错误, got %q", name)
		}
	}
}

// TestBuildDownloadSources 验证多源下载 URL 构建
func TestBuildDownloadSources(t *testing.T) {
	original := "https://github.com/liusaipu/stockfinlens/releases/download/v1.3.33/stockfinlens-windows-amd64-v1.3.33.zip"
	sources := buildDownloadSources(original, "v1.3.33", "stockfinlens-windows-amd64-v1.3.33.zip")

	if len(sources) != 2 {
		t.Fatalf("应有 2 个下载源, 实际 %d", len(sources))
	}

	// 第一个应为 gh-proxy.com 加速镜像
	if sources[0] != "https://gh-proxy.com/"+original {
		t.Errorf("第一个源应为 gh-proxy.com 镜像, got %q", sources[0])
	}

	// 第二个应为原始 GitHub URL
	if sources[1] != original {
		t.Errorf("第二个源应为原始 URL, got %q", sources[1])
	}
}

// TestTruncateURL 验证 URL 截断
func TestTruncateURL(t *testing.T) {
	short := "https://example.com/short"
	if truncateURL(short) != short {
		t.Error("短 URL 不应被截断")
	}

	long := "https://github.com/liusaipu/stockfinlens/releases/download/v1.3.33/stockfinlens-windows-amd64-v1.3.33.zip"
	truncated := truncateURL(long)
	if len(truncated) > 80 {
		t.Errorf("截断后 URL 长度应 <= 80, 实际 %d: %q", len(truncated), truncated)
	}
	if truncated == long {
		t.Error("长 URL 应被截断")
	}
}

// TestDownloadUpdateCancel 验证主动取消下载返回 ErrDownloadCanceled
func TestDownloadUpdateCancel(t *testing.T) {
	// 慢速流式响应服务器：每 50ms 写 1KB，总 1MB（足够触发取消）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		chunk := make([]byte, 1024)
		for i := 0; i < 1024; i++ {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	dir := t.TempDir()
	_, err := DownloadUpdate(ctx, server.URL+"/pkg.zip", "v0.0.1", dir, nil)
	if !errors.Is(err, ErrDownloadCanceled) {
		t.Fatalf("应返回 ErrDownloadCanceled, 实际: %v", err)
	}
}
