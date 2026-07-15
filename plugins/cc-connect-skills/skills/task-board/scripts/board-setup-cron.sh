#!/usr/bin/env bash
# board-setup-cron.sh <锚点sessionKey> [bot列表,逗号分隔] [cron表达式]
# 为每个角色 bot 注册看板自查 cron。boss 由人触发，无需 cron。
# 锚点 sessionKey 格式 feishu:<chatID>。⚠️ 两条铁律：
#   1. 锚点群绑定的 workspace 里不得有其它任务管理类 skill（会劫持自查）；
#   2. 锚点群勿用生产群（agent 过程卡片会发进去）。
set -euo pipefail
SK_ANCHOR="${1:?usage: board-setup-cron.sh <feishu:chatID> [bots] [cron]}"
BOTS="${2:-team-leader,developer,tester,reviewer}"
CRON="${3:-*/30 * * * *}"
source "${BOARD_ENV:-$HOME/.cc-connect/board.env}"
PROMPT="看板自查：使用 task-board 技能，运行 board-my-todos.sh 并按技能工作循环处理你的全部任务（认领待办/回收超时/完成后派发）。无任务则直接结束。看板 BASE=$BOARD_BASE Tasks=$TBL_TASKS"
IFS=',' read -ra ARR <<< "$BOTS"
for P in "${ARR[@]}"; do
  cc-connect cron add -p "$P" -s "$SK_ANCHOR" --cron "$CRON" --prompt "$PROMPT" --desc "看板自查-$P" --session-mode new-per-run --silent
done
cc-connect cron list
