---
name: release
description: 安全发布 StockFinLens 新版本（防跳号、防忘 build、防 auto-update miss）。流程：列出近期 tags 让用户确认下一个版本号 → 跑测试 → 改 wails.json + CHANGELOG → commit/tag/push → build-release.sh → gh release create → curl 验证 /releases/latest。
---

# Release Skill

按以下步骤执行，**每一步出问题就停下来报告，不要自行决定跳过**。

## 0. 前置盘点（不可省略）

并行跑：
- `git status --short`（确认有未提交改动，识别是哪些文件）
- `git log --oneline origin/main..HEAD`（看本地领先远端多少）
- `git tag --sort=-v:refname | head -5`（看最近 5 个 tag，**这是防跳号的关键**）
- `cat wails.json | grep productVersion`（当前版本号）
- `gh release list --limit 3`（确认 GitHub 上 latest 是哪个，是否与 git tag 一致）

把这些一起报给用户，问"下一个版本号是什么？"——**不要自己决定**。让用户从近期 tag 推断 patch/minor/major bump 类型。

## 1. 跑测试

- `go test ./...`（必须全过）
- `cd frontend && npx tsc --noEmit && cd ..`（零错误）

任何一个失败就停，**不修测试，先报告给用户决定**。

## 2. 更新文档与版本号

- 改 `wails.json` 的 `info.productVersion`（这是单一源，前端 Vite 自动注入）
  - **Edit 工具要求同会话先 Read 过该文件**，否则报 "File has not been read yet"。改之前先 Read 一次 wails.json 再 Edit
  - 改完**必须立刻** `grep productVersion wails.json` 复核。并行批次里 Edit 失败的错误信息很容易混在其他结果里漏看，错过就会把 v1.5.0 的产物当成 v1.5.1 发出去（v1.5.1 实战中踩过一次）
- 在 `CHANGELOG.md` **顶部**插入新版本段，遵循已有的"新增/优化/修复"三段格式，每条引用具体文件路径
- 如果新增了**用户可见功能**，同步更新 `README.md`（不只是修 bug 时跳过）

## 3. commit + push + tag

- `git add` 具体文件（不用 `-A`，避免误提交 .env 等）
- `git commit -m "release: vX.Y.Z - <一句话总结>"`，commit body 列出主要变化，末尾加 `Co-Authored-By: Claude <noreply@anthropic.com>`
- `git push origin main`
- `git tag vX.Y.Z && git push origin vX.Y.Z`

## 4. 构建二进制

`./build-release.sh all` 在后台跑（耗时 5~15 分钟，用 `run_in_background: true` 配 `timeout: 1500000`）。等通知，不要 sleep poll。

构建完检查产物：`ls -lh build/bin/ | grep "v${VERSION}"`，应看到 DMG + ZIP 两个文件。

**关键**：脚本默默以当前 `wails.json` 的版本号命名产物，**不会**校验是否与 tag 一致。如果 step 2 漏改 wails.json，这里会出 `v(旧版本).dmg` 但脚本依然 exit 0。**必须用 `grep "v${VERSION}"` 而不是只看 `ls build/bin`**，文件名对不上立即停下来回到 step 2 修，别去 release。

## 5. 发布 GitHub Release

```bash
gh release create vX.Y.Z \
  --title "StockFinLens vX.Y.Z" \
  --notes "$(从 CHANGELOG.md 的新版本段拷贝并整理)" \
  build/bin/stockfinlens-macos-universal-vX.Y.Z.dmg \
  build/bin/stockfinlens-windows-amd64-vX.Y.Z.zip
```

## 6. 验证 auto-update 端点

```bash
curl -s -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/liusaipu/stockfinlens/releases/latest \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print('tag:', d.get('tag_name')); print('prerelease:', d.get('prerelease'))"
```

**必须**返回新 tag 且 `prerelease: False`，否则 v1.4.0 之类的旧版用户收不到更新提示（updater.go 走的是这个端点）。

## 7. 最终汇报

报告里包含：
- ✅ / ❌ 每一步状态
- Release URL（`https://github.com/liusaipu/stockfinlens/releases/tag/vX.Y.Z`）
- DMG + ZIP 文件大小（直观判断有没有打包失败导致体积异常）
- auto-update 验证结果（curl 返回的 tag）

## 反模式（不要做）

- ❌ 不要跳版本号（v1.3.40 直接到 v1.3.42 这种）。**回滚 release 需要 delete release + force delete tag，订阅者已经收到通知。**
- ❌ 不要在测试失败的情况下继续 release
- ❌ 不要在 release 完成后又改 `CHANGELOG.md` 的对应版本段（用户已经在 release notes 看到了，事后改是"修史"）
- ❌ 不要 `git push --force` 已发布的 tag
- ❌ 不要跳过第 6 步 curl 验证——auto-update 端点偶尔有缓存延迟，没验证过的 release 不能算完整发布
