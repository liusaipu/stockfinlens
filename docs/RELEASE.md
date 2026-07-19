# 发布流程

> 从版本号更新到 GitHub Release 的完整步骤。发布前务必阅读 `docs/TESTING.md`。

1. **版本号更新**: 仅需修改 `wails.json` 的 `info.productVersion`（前端构建时自动注入）。
2. **更新 CHANGELOG.md**: 在顶部追加新版本说明。
3. **完整回归测试（硬性要求）**:
   ```bash
   ./scripts/run-regression.sh full
   ```
   - 必须 **全部通过** 才能继续发布
   - 结果自动保存到 `test-results/regression_full_YYYYMMDD_HHMMSS.log`
   - 失败时脚本返回非零 exit code 并阻止后续步骤
4. **提交并推送**: `git commit`, `git push origin main`
5. **打标签**: `git tag v1.4.0`, `git push origin v1.4.0`
6. **构建发布包**（每次 release 必须同时构建 macOS 与 Windows 双平台）：
   - **macOS**: `./build-release.sh mac`，产物为 `build/bin/stockfinlens-macos-universal-v{version}.dmg`
   - **Windows（在 Windows 主机上）**: `\.\build-windows.ps1 package`，产物为 `build/bin/stockfinlens-windows-amd64-v{version}.zip`
   - **Windows（在 macOS 上交叉编译）**: 若已安装 MinGW-w64（`brew install mingw-w64`），可直接执行：
     ```bash
     CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64 -clean
     cp -R ml_models scripts build/bin/
     cd build/bin && zip -r stockfinlens-windows-amd64-v$(grep -o '"productVersion": "[^"]*"' ../../wails.json | cut -d'"' -f4).zip stockfinlens.exe ml_models scripts
     ```
7. **创建 GitHub Release**: 上传 ZIP/DMG，将 `CHANGELOG.md` 对应章节粘贴到 Release Notes，并附上 `test-results/latest_summary.md` 测试报告。
8. **版本号递增**: 发布后立刻将 `wails.json` 版本号 bump 到下一个未发布版本（如 `1.4.1`）。
