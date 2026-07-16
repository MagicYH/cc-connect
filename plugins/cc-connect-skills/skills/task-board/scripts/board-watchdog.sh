#!/usr/bin/env bash
# board-watchdog.sh —— 看板防停滞巡检（纯脚本、无 LLM）。以 Boss 身份经系统 crontab 周期运行，
# 把催办消息直接推到**各任务自己的工作群**里 @ 责任 bot——被唤醒的会话因此天然在工作群中，
# 回复与 workspace 都落在正确的群（取代旧的"每角色一条 LLM 自查 cron 锚定在固定群"方案）。
#   待办  超过 STALL_TODO_MIN 分钟未认领            → @对应角色 bot 催办
#   待办  子任务含「待发起人<open_id>确认」          → @发起人本人催确认（不打扰 bot）
#   进行中 心跳（缺省取认领/创建时间）超过 STALL_HB_MIN → @对应角色 bot 催回收/继续
#   阻塞                                            → @team-leader 催改派
# 前提：Boss bot app 必须在每个工作群里（board-init-project.sh 建群时已拉入）。
# 用法：CC_PROJECT=boss ./board-watchdog.sh    （crontab 安装见 board-setup-cron.sh）
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
SEND="$(dirname "$0")/board-send.sh"
STALL_TODO_MIN="${STALL_TODO_MIN:-10}"
STALL_HB_MIN="${STALL_HB_MIN:-15}"
NOW_TS=$(date +%s)

age_min(){ # <"YYYY-MM-DD HH:MM:SS"> → 距今分钟数；解析失败输出 -1
  local ts
  ts=$(date -d "$1" +%s 2>/dev/null) || { echo -1; return; }
  echo $(( (NOW_TS - ts) / 60 ))
}

push(){ # <chat> <open_id> <text> —— 单群失败不中断巡检
  "$SEND" "$1" "$2" "$3" >/dev/null || echo "WARN: push failed chat=$1" >&2
}

list_rows | jq -r '.data as $d | $d.record_id_list | to_entries[] | .key as $k | ($d.data[$k]) as $r
  | def fv(n): ($d.fields | index(n)) as $i | $r[$i]
      | if type=="array" then (.[0]//"" | if type=="object" then (.id // .name // "") else . end)
        elif .==null then "" else . end;
  [.value, fv("状态"), fv("角色"), fv("工作群"), fv("创建时间"), fv("心跳时间"), fv("认领时间"),
   (fv("主任务")|gsub("[\t\n]";" ")), (fv("子任务")|gsub("[\t\n]";" "))] | @tsv' |
while IFS=$'\t' read -r rid st role chat ctime hbtime cltime mt sub; do
  [ -n "$chat" ] || continue
  rolename="$role"
  case "$role" in *"("*")"*) rolename="${role##*(}"; rolename="${rolename%)*}";; esac
  ov="BOT_OPENID_${rolename//-/_}"
  case "$st" in
    待办)
      if [[ "$sub" =~ 待发起人(ou_[a-z0-9]+) ]]; then
        push "$chat" "${BASH_REMATCH[1]}" "设计待确认：「$sub」（主任务 $mt），请回复 team-leader 确认或给出修改意见"
        echo "REMIND_INITIATOR $rid"
        continue
      fi
      AGE=$(age_min "${ctime:-}")
      [ "$AGE" -ge "$STALL_TODO_MIN" ] || continue
      [ -n "${!ov:-}" ] || continue
      push "$chat" "${!ov}" "看板催办：待办任务「$sub」（主任务 $mt）已 ${AGE} 分钟未认领，请用 task-board 技能处理"
      echo "PUSH_TODO $rid $rolename"
      ;;
    进行中)
      LAST="${hbtime:-}"; [ -n "$LAST" ] || LAST="${cltime:-}"; [ -n "$LAST" ] || LAST="${ctime:-}"
      AGE=$(age_min "$LAST")
      [ "$AGE" -ge "$STALL_HB_MIN" ] || continue
      [ -n "${!ov:-}" ] || continue
      push "$chat" "${!ov}" "看板催办：进行中任务「$sub」（主任务 $mt）心跳已 ${AGE} 分钟无更新，请用 task-board 技能回收或继续处理"
      echo "PUSH_STALE $rid $rolename"
      ;;
    阻塞)
      [ -n "${BOT_OPENID_team_leader:-}" ] || continue
      push "$chat" "$BOT_OPENID_team_leader" "看板催办：任务「$sub」（主任务 $mt）处于阻塞，请查看产出备注并改派或处理"
      echo "PUSH_BLOCKED $rid"
      ;;
  esac
done
echo "WATCHDOG_DONE"
