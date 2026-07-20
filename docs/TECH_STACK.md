# 技术栈与构建指南

> 人类与 AI 均可阅读的详细参考。若修改技术栈或构建流程，请同步更新本文件。

## 技术栈

| 层级 | 技术 | 版本/说明 |
|------|------|-----------|
| 桌面框架 | Wails v2 | `v2.12.0` |
| 后端 | Go | `1.25.0`（`go.mod` 硬性要求，不可降级） |
| 前端 | React + TypeScript + Vite | React `^18.2.0`、TypeScript `^5.0.0`、Vite `^5.0.0` |
| 图表 | Apache ECharts、lightweight-charts、recharts | `^6.0.0`、`^5.1.0`、`^2.10.0` |
| Markdown 渲染 | react-markdown + rehype/rehype 插件 | `^10.1.0` |
| ML 推理 | Python 3 + ONNX Runtime + scikit-learn + numpy | 运行时通过 `deps_manager.go` 自动检测 |
| 数据获取 | 东方财富 API、腾讯财经接口、StockFinLens Pro、Yahoo、akshare（港股）、同花顺 CSV/Excel 导入 |
| Excel/CSV | `github.com/xuri/excelize/v2`、标准库 `encoding/csv` | excelize `v2.10.1` |
| 通知 | `git.sr.ht/~jackmordaunt/go-toast/v2`（Windows Toast）| `v2.0.3` |

## 关键配置文件

| 文件 | 用途 |
|------|------|
| `wails.json` | Wails 应用配置：版本号、前端目录、构建命令、dev server URL (`http://localhost:5173`) |
| `go.mod` / `go.sum` | Go 依赖管理。核心依赖：`wails/v2 v2.12.0`、`excelize/v2 v2.10.1`、`go-toast/v2 v2.0.3`、`golang.org/x/text v0.35.0`、`golang.org/x/sync v0.20.0` |
| `frontend/package.json` | 前端依赖（注意：`version` 字段为 `0.1.0`，**不参与**应用版本同步） |
| `frontend/tsconfig.json` | TypeScript 主配置：`target: ES2020`、`jsx: react-jsx`、严格模式、`noUnusedLocals: true`、`noUnusedParameters: true`、`noFallthroughCasesInSwitch: true` |
| `frontend/tsconfig.node.json` | Vite 配置文件专用 TS 配置（`composite: true`），解决 Node 与浏览器环境类型冲突 |
| `frontend/vite.config.ts` | Vite 构建配置：`outDir: 'dist'`、`emptyOutDir: true`；构建期读取 `../wails.json` 注入 `__APP_VERSION__` |
| `requirements.txt` | Python 运行时依赖：`pandas>=2.0.0`、`numpy>=1.24.0`、`akshare>=1.12.0`、`requests>=2.31.0`、`openpyxl>=3.1.0`、`tqdm>=4.65.0` |
| `build-windows.ps1` | Windows 构建/打包/清理脚本（命令：`setup`、`dev`、`build`、`package`、`clean`），含环境检查 |
| `build-release.sh` | macOS/Windows 发布构建脚本（Bash，参数：`mac`、`windows`、`all`） |
| `.vscode/launch.json` | VS Code 调试配置：Wails Dev Mode、Debug Windows、Attach |
| `.vscode/tasks.json` | VS Code 任务：wails build/debug、frontend install/build、python venv、clean |
| `docs/BUILD.md` | 更详细的构建指南 |

## 构建与运行

### 环境要求

- **Go** >= `1.25.0`（`go.mod` 硬性指定，不可降级）
- **Node.js** >= 18
- **Python** 3.10+（运行时必需，用于 ML 推理与数据脚本）
- **Wails CLI** >= v2.12：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **GCC / MinGW-w64**（Windows 构建必需）

### 安装依赖

```bash
# Go 依赖
go mod tidy

# 前端依赖
cd frontend && npm install

# Python 依赖（运行时必需）
pip install -r requirements.txt
# 核心运行时额外依赖：onnxruntime, scikit-learn, numpy
```

### 常用命令

```bash
# 开发模式（热重载前端 + Go）
wails dev

# 构建生产版本（Windows，推荐用脚本）
.\build-windows.ps1 build

# 打包为 ZIP（包含 ml_models 和 scripts）
.\build-windows.ps1 package

# macOS 发布构建
./build-release.sh mac

# 运行全部 Go 测试
go test ./...

# 运行特定包测试
go test ./analyzer/...
go test ./downloader/...
go test ./updater/...

# 运行前端测试
cd frontend && npm test

# 统一回归测试（quick / full）
./scripts/run-regression.sh quick
./scripts/run-regression.sh full
```

### 构建注意事项

1. **版本号唯一来源**: `wails.json` 中的 `info.productVersion` 是应用版本的**唯一来源**。前端通过 `frontend/vite.config.ts` 在构建期读取并以 `define` 注入全局常量 `__APP_VERSION__`，`Settings.tsx` 直接引用该常量，**禁止重新硬编码**。构建脚本不再做一致性校验（版本自然一致）。当前版本为 `1.8.13`。
2. **前端 dist 重建**: 如果前端代码有变更，构建前必须确保 `frontend/dist` 是最新的。Wails `build` 在 `dist` 已存在时可能跳过前端构建，导致打包旧代码。`build-release.sh` 会强制先执行 `cd frontend && npm run build`。Windows 构建建议同样手动前置该步骤。
3. **打包产物必须包含**: `ml_models/` 和 `scripts/` 目录。Go 后端在运行时会从可执行文件同级目录查找这些路径。
4. **开发模式 vs 生产模式**: `main.go` 中 `readStockJSON()` 优先读取本地 `data/stocks.json`，打包后 fallback 到 `embed.FS`。
5. **跨平台构建标签**: `main`、`analyzer`、`downloader` 三个包均包含 `sysproc_windows.go`（`//go:build windows`）和 `sysproc_other.go`（`//go:build !windows`），用于隔离 Windows 特有的 `syscall.SysProcAttr{HideWindow: true}`，避免 macOS/Linux 编译失败。新增包若需调用 Python 子进程，应遵循同样模式。
6. **构建脚本硬编码路径**: `build-release.sh` 中 Wails CLI 路径硬编码为 `/Users/lobster/go/bin/wails`，在他人机器上构建时需确保路径存在或创建软链接。
