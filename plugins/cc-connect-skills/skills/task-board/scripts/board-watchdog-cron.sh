#!/usr/bin/env bash
# board-watchdog-cron.sh —— cc-connect cron `--exec` 入口。
# 为什么要它：watchdog 走 cc-connect cron（webui 可见/可管：exec/edit/info/del）而非 OS crontab；
# cc-connect cron 的 `--exec` 直跑 shell（不拉 LLM 会话、无"锚定固定群"问题），但不保证注入 CC_PROJECT，
# 且命令里带 `cd`/环境前缀会踩引号坑——故用这个包装：设定 Boss 身份 + 落日志，再调 board-watchdog.sh。
cd "$(dirname "$0")" || exit 1
CC_PROJECT="${CC_PROJECT:-boss}" ./board-watchdog.sh >> "$HOME/.cc-connect/logs/board-watchdog.log" 2>&1
