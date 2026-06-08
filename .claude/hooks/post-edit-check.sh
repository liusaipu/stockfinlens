#!/bin/bash
# PostToolUse hook: 编辑文件后自动 type check
# - 改 .ts / .tsx → cd frontend && npx tsc --noEmit
# - 改 .go       → go vet ./...
# - 其他文件     → 静默通过
# 成功无输出，失败把错误前 20 行打到 stdout（Claude 会看到并自行修）
# exit 0 始终通过（PostToolUse 不阻塞工具，仅作信息回灌）

set +e

INPUT=$(cat)
FILE=$(printf '%s' "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('file_path',''))" 2>/dev/null)

[ -z "$FILE" ] && exit 0

PROJECT_ROOT="/Users/lobster/myprojects/stockfinlens"

case "$FILE" in
  *.ts|*.tsx)
    OUT=$(cd "$PROJECT_ROOT/frontend" 2>/dev/null && npx tsc --noEmit 2>&1)
    RC=$?
    if [ $RC -ne 0 ]; then
      echo "⚠️ TypeScript errors after editing $(basename "$FILE"):"
      echo "$OUT" | head -20
    fi
    ;;
  *.go)
    OUT=$(cd "$PROJECT_ROOT" 2>/dev/null && go vet ./... 2>&1)
    RC=$?
    if [ $RC -ne 0 ]; then
      echo "⚠️ go vet errors after editing $(basename "$FILE"):"
      echo "$OUT" | head -20
    fi
    ;;
esac

exit 0
