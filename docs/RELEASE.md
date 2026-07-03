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
6. **构建发布包**:
   - Windows: `.\build-windows.ps1 package`
   - macOS: `./build-release.sh mac`
7. **创建 GitHub Release**: 上传 ZIP/DMG，将 `CHANGELOG.md` 对应章节粘贴到 Release Notes，并附上 `test-results/latest_summary.md` 测试报告。
8. **版本号递增**: 发布后立刻将 `wails.json` 版本号 bump 到下一个未发布版本（如 `1.4.1`）。
