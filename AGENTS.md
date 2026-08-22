<!-- AGENTS.md for stockfinlens / StockFinLens -->
<!-- 本文件面向 AI 编码助手。修改项目结构或构建流程后，请同步更新 docs/ 下对应文档。 -->

# Agent Guidelines for StockFinLens

> 本文件只保留 AI 编码助手必须遵守的核心约束。详细背景见：
> - 架构/数据流/缓存：`docs/ARCHITECTURE.md`
> - 技术栈与构建命令：`docs/TECH_STACK.md`
> - 测试策略：`docs/TESTING.md`
> - 发布流程：`docs/RELEASE.md`
> - 构建指南（人类阅读）：`docs/BUILD.md`
> - ML 设计：`docs/ML_PREDICTION_DESIGN.md`

## 项目核心约束

- **模块名**: `github.com/liusaipu/stockfinlens`
- **当前版本**: `1.8.25`（唯一来源：`wails.json` → `info.productVersion`）
- **本地数据目录**: `~/.config/stock-analyzer/`
- **股票代码格式**: A 股上海 `603501.SH`、深圳 `000001.SZ`、港股 `00700.HK`
- **注释语言**: 中文

## 修改代码前必读

### 版本号
- 应用版本号**唯一来源**是 `wails.json` 的 `info.productVersion`。
- 前端通过 Vite `define` 注入 `__APP_VERSION__`，**禁止**在 `frontend/src/` 硬编码版本号。

### Wails 绑定
- `app.go` 中首字母大写的方法会被前端调用，修改签名必须同步更新：
  - `frontend/src/App.tsx` 中的调用
  - `frontend/src/api/` 下对应的 facade 文件
- `frontend/wailsjs/go/main/App.d.ts` 与 `App.js` 由 `wails dev` 自动生成，**禁止手动编辑**。

### Python ↔ Go 集成
- 所有 Python 脚本通过 **stdin/stdout JSON** 与 Go 通信，禁止改动接口格式。
- 调用 Python 子进程必须：
  - 注入 `TQDM_DISABLE=1`、`PYTHONUNBUFFERED=1`
  - Windows 调用 `setHideWindow(cmd)`（已封装，按 build tag 隔离）
  - 新增包需遵循 `sysproc_windows.go` / `sysproc_other.go` 模式
- 路径解析必须兼容开发与打包双模式（见 `docs/ARCHITECTURE.md` → 路径解析策略）。

### 新增 Python 脚本
- 设置 `PYTHONIOENCODING=utf-8`。
- 屏蔽 `tqdm` 进度条，防止破坏 stdout JSON。
- 模型/脚本路径解析遵循 4 级回退：`os.Executable()` → `.app Resources` → `runtime.Caller(0)` → 系统 PATH。

### 前端规范
- `npm run build` 必须无类型错误。
- `tsconfig.json` 启用 `strict`、`noUnusedLocals`、`noUnusedParameters`、`noFallthroughCasesInSwitch`。
- ECharts 数据避免传 `null`，用 `'-'` 或 `undefined`。
- 状态管理集中在 `App.tsx`，不引入 Redux/Zustand。
- CSS 使用组件级 CSS 文件，不用 CSS-in-JS。
- 新增 Wails 事件必须在前端补 `EventsOn`/`EventsOff`。

### Go 规范
- 提交前运行 `go fmt ./...`。
- 错误处理：`fmt.Errorf("...: %w", err)`。
- 并发操作加 `recover()` 防止 panic 扩散。
- 新增依赖后运行 `go mod tidy`。

## 关键命令

```bash
# 开发
wails dev

# 测试
go test ./...
go test -short ./...
cd frontend && npm test
./scripts/run-regression.sh quick
./scripts/run-regression.sh full   # 发布前必须全绿
```

## 安全红线

- 禁止破坏 Wails 绑定签名。
- 禁止在源码中硬编码版本号（统一使用 `__APP_VERSION__`）。
- 禁止在 stdout JSON 通信脚本中输出进度条/调试日志。
- 禁止 Windows 上出现未隐藏窗口的 Python 子进程。
- 用户数据仅保存于 `~/.config/stock-analyzer/`，不涉云端。
- 不要自行 `git commit` / `git push` / 打标签，除非用户明确要求。

## 修改 AGENTS.md 本身

- 若变更核心约束，同步更新本文件。
- 若变更架构/构建/测试/发布细节，同步更新 `docs/` 下对应文件。
