// Package updater 处理自动更新检测与下载
package updater

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	githubAPIURL    = "https://api.github.com/repos/liusaipu/stockfinlens/releases/latest"
	apiTimeout      = 10 * time.Second
	downloadTimeout = 60 * time.Second
)

// ProxyConfig 网络代理配置
type ProxyConfig struct {
	Enabled  bool
	URL      string
	Username string
	Password string
}

var proxyConfig ProxyConfig

// SetProxyConfig 设置更新模块使用的代理
func SetProxyConfig(cfg ProxyConfig) {
	proxyConfig = cfg
}

// createHTTPClient 创建带代理支持的 HTTP 客户端
func createHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	if proxyConfig.Enabled && proxyConfig.URL != "" {
		proxyURL, err := url.Parse(proxyConfig.URL)
		if err == nil {
			if proxyConfig.Username != "" {
				proxyURL.User = url.UserPassword(proxyConfig.Username, proxyConfig.Password)
			}
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// UpdateInfo 更新信息
type UpdateInfo struct {
	HasUpdate   bool   `json:"hasUpdate"`
	CurrentVer  string `json:"currentVer"`
	LatestVer   string `json:"latestVer"`
	ReleaseName string `json:"releaseName"`
	ReleaseNote string `json:"releaseNote"`
	PublishedAt string `json:"publishedAt"`
	AssetURL    string `json:"assetURL"` // 原始 GitHub asset URL（用于 mirror 拼接）
	HTMLURL     string `json:"htmlURL"`  // release 页面
}

// githubRelease GitHub API 响应结构
type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate 检查是否有新版本。
// 优先直连 GitHub API，失败时回退到 gh-proxy.com 镜像，以应对代理 IP 被限流等情况。
func CheckUpdate(currentVersion string) (*UpdateInfo, error) {
	sources := []string{githubAPIURL, "https://gh-proxy.com/" + githubAPIURL}
	var lastErr error
	for i, src := range sources {
		fmt.Printf("[Updater] 尝试检查更新源 %d/%d...\n", i+1, len(sources))
		info, err := checkUpdateFromURL(currentVersion, src)
		if err == nil {
			return info, nil
		}
		lastErr = err
		fmt.Printf("[Updater] 源 %d 失败: %v\n", i+1, err)
	}
	return nil, fmt.Errorf("所有更新检查源均失败: %w", lastErr)
}

func checkUpdateFromURL(currentVersion, apiURL string) (*UpdateInfo, error) {
	client := createHTTPClient(apiTimeout)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "StockFinLens-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回状态码 %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析 GitHub 响应失败: %w", err)
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	assetURL, assetName := matchPlatformAsset(release.Assets)

	info := &UpdateInfo{
		CurrentVer:  currentVersion,
		LatestVer:   latestVer,
		ReleaseName: release.Name,
		ReleaseNote: release.Body,
		PublishedAt: formatPublishedAt(release.PublishedAt),
		AssetURL:    assetURL,
		HTMLURL:     release.HTMLURL,
	}

	info.HasUpdate = compareVersion(currentVersion, latestVer) < 0
	if !info.HasUpdate {
		info.AssetURL = ""
	}

	// 调试日志
	fmt.Printf("[Updater] 当前版本: %s, 最新版本: %s, 有更新: %v, asset: %s\n",
		currentVersion, latestVer, info.HasUpdate, assetName)

	return info, nil
}

// matchPlatformAsset 根据当前平台匹配对应的 release asset。
// macOS 优先 universal，其次 arm64；Windows 优先 amd64。
func matchPlatformAsset(assets []githubAsset) (url, name string) {
	var wantSuffixes []string
	switch runtime.GOOS {
	case "windows":
		wantSuffixes = []string{"windows-amd64-"}
	case "darwin":
		wantSuffixes = []string{"macos-universal-", "macos-arm64-"}
	default:
		return "", ""
	}

	for _, suffix := range wantSuffixes {
		for _, a := range assets {
			if strings.Contains(a.Name, suffix) {
				return a.BrowserDownloadURL, a.Name
			}
		}
	}
	return "", ""
}

// DownloadUpdate 下载更新包，返回本地文件路径
// progressFn 可选，用于报告下载进度 (0-100)
func DownloadUpdate(assetURL, tag string, updateDir string, progressFn func(percent int)) (string, error) {
	if assetURL == "" {
		return "", fmt.Errorf("没有可用的下载链接")
	}

	// 清理并创建更新目录
	_ = os.RemoveAll(updateDir)
	if err := os.MkdirAll(updateDir, 0755); err != nil {
		return "", fmt.Errorf("创建更新目录失败: %w", err)
	}

	// 获取文件名
	assetName := filepath.Base(assetURL)
	localPath := filepath.Join(updateDir, assetName)

	// 多源下载
	sources := buildDownloadSources(assetURL, tag, assetName)
	var lastErr error
	for i, src := range sources {
		fmt.Printf("[Updater] 尝试下载源 %d/%d: %s...\n", i+1, len(sources), truncateURL(src))
		path, err := downloadFile(src, localPath, downloadTimeout, progressFn)
		if err == nil {
			fmt.Printf("[Updater] 下载成功: %s\n", path)
			return path, nil
		}
		lastErr = err
		fmt.Printf("[Updater] 源 %d 失败: %v\n", i+1, err)
	}

	return "", fmt.Errorf("所有下载源均失败: %w", lastErr)
}

// buildDownloadSources 构建多源下载列表
func buildDownloadSources(originalURL, tag, assetName string) []string {
	var sources []string
	// 主源：gh-proxy.com 加速镜像
	sources = append(sources, "https://gh-proxy.com/"+originalURL)
	// 备用：GitHub 直连
	sources = append(sources, originalURL)
	return sources
}

// downloadFile 从 URL 下载文件到本地路径
func downloadFile(url, localPath string, timeout time.Duration, progressFn func(int)) (string, error) {
	client := createHTTPClient(timeout)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "StockFinLens-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		// 未知大小，直接复制
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return "", err
		}
		return localPath, nil
	}

	// 带进度追踪的复制
	var written int64
	buf := make([]byte, 32*1024)
	lastPercent := -1
	for {
		nr, rerr := resp.Body.Read(buf)
		if nr > 0 {
			nw, werr := out.Write(buf[:nr])
			if werr != nil {
				return "", werr
			}
			written += int64(nw)
			if progressFn != nil && totalSize > 0 {
				percent := int(float64(written) * 100 / float64(totalSize))
				if percent != lastPercent && percent%5 == 0 {
					progressFn(percent)
					lastPercent = percent
				}
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return "", rerr
		}
	}

	if progressFn != nil {
		progressFn(100)
	}
	return localPath, nil
}

// ExtractZip 解压 zip 文件到指定目录
func ExtractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		// 防止 zip slip 攻击
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("非法 zip 路径: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// compareVersion 比较两个版本号，返回 -1/0/1
// 版本号格式: "1.3.33"
func compareVersion(v1, v2 string) int {
	p1 := parseVersion(v1)
	p2 := parseVersion(v2)
	for i := 0; i < 3; i++ {
		if p1[i] < p2[i] {
			return -1
		}
		if p1[i] > p2[i] {
			return 1
		}
	}
	return 0
}

// parseVersion 解析版本号字符串为三段整数
func parseVersion(v string) [3]int {
	var parts [3]int
	segs := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(segs); i++ {
		n, _ := strconv.Atoi(segs[i])
		parts[i] = n
	}
	return parts
}

// formatPublishedAt 格式化发布时间
func formatPublishedAt(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Format("2006-01-02")
}

// truncateURL 截断 URL 用于日志显示
func truncateURL(url string) string {
	if len(url) > 80 {
		return url[:40] + "..." + url[len(url)-30:]
	}
	return url
}
