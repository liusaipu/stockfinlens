//go:build darwin

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// findAppBundlePath 从当前可执行文件路径向上推导出 .app 根路径。
// 典型路径形如 /Applications/stockfinlens.app/Contents/MacOS/stockfinlens
func findAppBundlePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		real = exePath
	}
	cur := real
	for i := 0; i < 6; i++ {
		cur = filepath.Dir(cur)
		if strings.HasSuffix(cur, ".app") {
			return cur, nil
		}
		if cur == "/" {
			break
		}
	}
	return "", fmt.Errorf("未在 %s 的上层找到 .app 目录", real)
}

// applyScript 是静默自更新脚本。
// 参数：$1=DMG_PATH  $2=APP_PATH  $3=MOUNT_POINT  $4=WAIT_PID  $5=LOG_PATH
const applyScript = `#!/bin/bash
# StockFinLens macOS 自动更新脚本（由主程序生成并以独立会话启动）

DMG_PATH="$1"
APP_PATH="$2"
MOUNT_POINT="$3"
WAIT_PID="$4"
LOG_PATH="$5"

exec >> "$LOG_PATH" 2>&1
echo ""
echo "[$(date)] === StockFinLens 自动更新开始 ==="
echo "DMG:       $DMG_PATH"
echo "App:       $APP_PATH"
echo "Mount:     $MOUNT_POINT"
echo "Wait PID:  $WAIT_PID"

# 0) 防御：拒绝替换运行自 DMG 的应用
case "$APP_PATH" in
  /Volumes/*)
    echo "ERROR: 应用运行自 DMG，无法原地替换。请先拖入 Applications。"
    exit 1
    ;;
esac

# 1) 等待主程序退出（最多 30s）
for i in $(seq 1 60); do
  if ! kill -0 "$WAIT_PID" 2>/dev/null; then
    echo "主程序已退出"
    break
  fi
  sleep 0.5
done

# 2) 挂载 DMG
mkdir -p "$MOUNT_POINT"
if ! hdiutil attach -nobrowse -mountpoint "$MOUNT_POINT" "$DMG_PATH"; then
  echo "ERROR: 挂载 DMG 失败"
  exit 1
fi

# 3) 在挂载点查找新版本 .app
NEW_APP=""
for cand in "$MOUNT_POINT"/*.app; do
  if [ -d "$cand" ]; then
    NEW_APP="$cand"
    break
  fi
done
if [ -z "$NEW_APP" ]; then
  echo "ERROR: DMG 中未找到 .app"
  hdiutil detach "$MOUNT_POINT" -force || true
  exit 1
fi
echo "新版本: $NEW_APP"

# 4) ditto 拷贝到旁路位置，再做原子 mv 替换（先备份旧版本以便回滚）
NEW_TMP_APP="${APP_PATH}.new"
BACKUP_APP="${APP_PATH}.old"
rm -rf "$NEW_TMP_APP" "$BACKUP_APP"

if ! ditto "$NEW_APP" "$NEW_TMP_APP"; then
  echo "ERROR: 拷贝新版本失败（可能权限不足）"
  hdiutil detach "$MOUNT_POINT" -force || true
  rm -rf "$NEW_TMP_APP"
  exit 1
fi

if ! mv "$APP_PATH" "$BACKUP_APP" 2>/dev/null; then
  echo "ERROR: 备份旧版本失败（可能权限不足，需手动安装）"
  hdiutil detach "$MOUNT_POINT" -force || true
  rm -rf "$NEW_TMP_APP"
  exit 1
fi

if ! mv "$NEW_TMP_APP" "$APP_PATH"; then
  echo "ERROR: 安装新版本失败，正在回滚..."
  mv "$BACKUP_APP" "$APP_PATH" 2>/dev/null || true
  hdiutil detach "$MOUNT_POINT" -force || true
  exit 1
fi

# 5) 清除隔离属性，避免 Gatekeeper 二次拦截
xattr -dr com.apple.quarantine "$APP_PATH" 2>/dev/null || true

# 6) 卸载 DMG
hdiutil detach "$MOUNT_POINT" -force || true

# 7) 删除备份
rm -rf "$BACKUP_APP"

# 8) 重启应用
echo "重启应用..."
open "$APP_PATH"

# 9) 清理 dmg、挂载点
sleep 2
rm -f "$DMG_PATH"
rmdir "$MOUNT_POINT" 2>/dev/null || true
echo "[$(date)] === 更新完成 ==="
`

// ApplyUpdate 在 macOS 上执行静默自更新：
//   1. 把 helper 脚本写到下载目录
//   2. 以独立会话 (setsid) 启动脚本，脱离当前进程
//   3. 1.5s 后退出当前进程；脚本检测到主进程已退出后开始替换
//   4. 脚本：挂载 DMG → ditto 拷贝 → 原子 mv 替换 → xattr 清隔离 → 卸载 → open 重启
//
// 失败回滚：先备份为 .old，新版安装失败时把 .old 移回原位。
// 日志位置：与 DMG 同目录的 apply_update.log。
func ApplyUpdate(dmgPath string) error {
	appPath, err := findAppBundlePath()
	if err != nil {
		return fmt.Errorf("定位应用 bundle 失败: %w", err)
	}

	updateDir := filepath.Dir(dmgPath)
	scriptPath := filepath.Join(updateDir, "apply_update.sh")
	logPath := filepath.Join(updateDir, "apply_update.log")
	mountPoint := filepath.Join(updateDir, "mount")

	if err := os.WriteFile(scriptPath, []byte(applyScript), 0755); err != nil {
		return fmt.Errorf("写入更新脚本失败: %w", err)
	}

	pid := os.Getpid()
	cmd := exec.Command("/bin/bash", scriptPath, dmgPath, appPath, mountPoint, fmt.Sprint(pid), logPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动更新脚本失败: %w", err)
	}

	go func() {
		time.Sleep(1500 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}
