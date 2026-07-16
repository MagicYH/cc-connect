#!/usr/bin/env bash
# board-setup-cron.sh [boss项目名] [cron表达式] —— 安装看板防停滞巡检定时任务（cc-connect cron，webui 可见）。
# 巡检 = board-watchdog.sh 以 Boss 身份周期运行（纯脚本无 LLM），催办直接推到各任务的工作群 @ 责任 bot，
# 被唤醒的会话天然在工作群里（回复与 workspace 均正确）。
#
# 用 cc-connect cron 的 `--exec`（直跑 shell、不拉 LLM 会话）注册，好处：在 cc-connect webui 里可见/可管
# （exec 立即触发 / edit / info 看 last_run/last_error / del），比 OS crontab 更好排查。经 board-watchdog-cron.sh
# 包装以设定 Boss 身份并落日志。
# ⚠️ 旧方案（每角色一条 LLM 自查 cron 锚定固定群 / 或裸 OS crontab）已废弃。残留请清理：
#    - cc-connect 侧：cc-connect cron list → cc-connect cron del <id> 删掉描述含"看板自查/Task progress check"的
#    - OS 侧：crontab -e 删掉含 board-watchdog.sh 的行
# 前提：board.env 配 BOSS_SESSION_KEY（Boss 会话，形如 feishu:oc_xxx；--exec 只作归属标签，命令独立运行）。
set -euo pipefail
BOSS="${1:-boss}"
CRON="${2:-*/10 * * * *}"   # 每 10 分钟：心跳由 Boss 从群消息推导，粒度太粗会晚发现卡死的 bot
DIR="$(cd "$(dirname "$0")" && pwd)"
WRAPPER="$DIR/board-watchdog-cron.sh"
DESC="看板防停滞巡检(board-watchdog)"
BOARD_ENV="${BOARD_ENV:-$HOME/.cc-connect/board.env}"
[ -f "$BOARD_ENV" ] && source "$BOARD_ENV"

: "${BOSS_SESSION_KEY:?board.env 缺 BOSS_SESSION_KEY（Boss 会话，形如 feishu:oc_xxx）——见 cc-connect sessions list 取 boss 会话的群}"
[ -x "$WRAPPER" ] || chmod +x "$WRAPPER"
mkdir -p "$HOME/.cc-connect/logs"

if cc-connect cron list 2>/dev/null | grep -qF "$DESC"; then
  echo "已存在「$DESC」cc-connect cron，跳过（如需改周期：cc-connect cron edit <id>）"
else
  cc-connect cron add --cron "$CRON" --exec "$WRAPPER" --desc "$DESC" \
    -p "$BOSS" --session-key "$BOSS_SESSION_KEY" --silent --timeout-mins 5
fi
echo "--- 当前 cc-connect cron ---"
cc-connect cron list 2>/dev/null | grep -F "$DESC" || true
# 提示清理废弃残留
OLD_OS=$(crontab -l 2>/dev/null | grep "board-watchdog" || true)
[ -n "$OLD_OS" ] && { echo "⚠️ 检测到废弃的 OS crontab watchdog 行，请 crontab -e 删除："; echo "$OLD_OS"; }
OLD_CC=$(cc-connect cron list 2>/dev/null | grep -iE "看板自查|Task progress check" || true)
[ -n "$OLD_CC" ] && { echo "⚠️ 检测到废弃的旧 LLM 自查 cron，请 cc-connect cron del 删除："; echo "$OLD_CC"; }
