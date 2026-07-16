#!/usr/bin/env bash
# board-setup-cron.sh [boss项目名] [cron表达式] —— 安装看板防停滞巡检 crontab。
# 巡检 = board-watchdog.sh 以 Boss 身份周期运行（纯脚本无 LLM），催办消息直接推到
# 各任务的工作群 @ 责任 bot，被唤醒的会话天然在工作群里（回复与 workspace 均正确）。
# ⚠️ 旧方案（每角色一条 cc-connect LLM 自查 cron 锚定固定群）已废弃：会话锚在固定群导致
#    回复落错群、workspace 错绑。如存在旧 cron（描述"看板自查-*"），用 cc-connect cron 删除。
set -euo pipefail
BOSS="${1:-boss}"
CRON="${2:-*/30 * * * *}"
DIR="$(cd "$(dirname "$0")" && pwd)"
LOG="$HOME/.cc-connect/logs/board-watchdog.log"
LINE="$CRON cd $DIR && CC_PROJECT=$BOSS ./board-watchdog.sh >> $LOG 2>&1"
mkdir -p "$(dirname "$LOG")"
if crontab -l 2>/dev/null | grep -qF "board-watchdog.sh"; then
  echo "已存在 board-watchdog crontab，跳过（如需改周期请手动 crontab -e）"
else
  ( crontab -l 2>/dev/null; echo "$LINE" ) | crontab -
  echo "已安装: $LINE"
fi
OLD=$(cc-connect cron list 2>/dev/null | grep "看板自查-" || true)
[ -n "$OLD" ] && { echo "检测到废弃的旧自查 cron，请删除："; echo "$OLD"; }
crontab -l | grep board-watchdog
