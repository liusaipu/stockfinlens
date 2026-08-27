# Build Guide

> 详细构建指南。快速参考请查看项目根目录 [README.md](../README.md)。

---

## 环境要求

| 工具 | 最低版本 | 说明 |
|------|---------|------|
| Go | >= 1.25.0 | 后端与 Wails 框架 |
| Node.js | >= 18 | 前端构建 |
| Python | >= 3.10 | ML 推理与数据脚本 |
| Wails CLI | >= v2.12 | 跨平台桌面应用构建 |

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 安装 Python 依赖（必需）

```bash
pip install onnxruntime scikit-learn numpy
```

可选增强（用于完整数据获取功能）：

```bash
pip install akshare
```

---

## 开发构建

```bash
# 克隆项目
git clone https://github.com/liusaipu/stockfinlens.git
cd stockfinlens

# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式（热重载）
wails dev
```

开发模式下，前端会监听 `http://localhost:5173`，Wails 桥接自动生效。

---

## 生产构建

```bash
# 确保前端 dist 是最新的
cd frontend && npm run build && cd ..

# 构建当前平台的可执行文件
wails build

# 构建结果位于 build/bin/
```

### Windows 平台

在 Windows 上可直接使用提供的脚本：

```powershell
# PowerShell
.\build-windows.ps1
```

或跨平台脚本（需 WSL / Git Bash）：

```bash
bash build-release.sh
```

### macOS 平台

```bash
wails build -platform darwin/universal
```

---

## 发布前检查清单

详见 [AGENTS.md](../AGENTS.md) 中的 **Release Checklist**。核心要点：

1. **版本号同步**：`wails.json` 与 `frontend/src/Settings.tsx` 必须一致。
2. **重建前端**：`frontend/dist` 必须从 scratch 重新构建，避免嵌入旧代码。
3. **包含必需资源**：发布 ZIP 中必须包含 `ml_models/` 和 `scripts/` 目录。
4. **更新 CHANGELOG**：在 `CHANGELOG.md` 顶部追加新版本记录。
5. **打标签**：创建并推送 Git tag（如 `v1.3.22`）。

---

## 目录结构速览

```
├── analyzer/          # Go 分析引擎（财报透视、A-Score、ML 推理调用）
├── downloader/        # 数据下载层（东方财富、腾讯行情等）
├── frontend/          # React + TypeScript 前端
├── ml_models/         # ONNX 模型与 Python 推理脚本
├── scripts/           # Python 数据获取与更新脚本
├── docs/              # 设计文档与截图
├── build/             # 构建输出（由 Wails 生成）
├── app.go             # Wails App 定义
├── main.go            # 程序入口
└── wails.json         # Wails 配置（含版本号）
```

---

## 常见问题

**Q: 构建时提示 `frontend/dist` 不存在？**
> 先执行 `cd frontend && npm run build`，Wails 默认从 `frontend/dist` 读取前端资源。

**Q: Windows 上运行 exe 后 ML 预测报错？**
> 确保 `ml_models/` 与 `scripts/` 目录与可执行文件在同一目录下，或位于项目根目录下。

**Q: Python 依赖安装失败？**
> 使用 Python 3.10+。onnxruntime 对 Python 版本较敏感，建议使用虚拟环境：
> ```bash
> python -m venv .venv
> .venv\Scripts\activate  # Windows
> pip install -r requirements.txt
> ```
