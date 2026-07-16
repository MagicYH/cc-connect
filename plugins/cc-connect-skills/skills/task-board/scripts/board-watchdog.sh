#!/usr/bin/env bash
# board-watchdog.sh —— 看板防停滞巡检（纯脚本、无 LLM）。以 Boss 身份经系统 crontab 周期运行。
#
# 核心思想（2026-07 改版）：干活的 bot 很难可靠地"定期发心跳"，故**心跳由 Boss 从群消息活动推导**——
# 扫每个仍有未完成任务的工作群近 24h 消息，取**该角色 bot（按 app_id 过滤）**最后一条消息时间，
# 写回该任务的 `心跳时间`。真正在群里干活/汇报的 bot 因此不会被误判超时；真的卡死的才会被催。
# 催办消息直接推到**各任务自己的工作群** @ 责任 bot——被唤醒的会话天然在工作群中（回复/workspace 正确）。
#   待办  超过 STALL_TODO_MIN 分钟未开始            → @对应角色 bot：先用异步 subagent 认领再动手
#   待办  子任务含「待发起人<open_id>确认」          → @发起人本人催确认（不打扰 bot）
#   进行中 先写心跳=该角色最后群消息时间；据此若超过 STALL_HB_MIN → @对应角色 bot 催收口/继续
#   阻塞                                            → @team-leader 催改派
# 前提：①Boss bot app 在每个工作群里（board-init-project.sh 建群已拉入）②board.env 配 BOT_APPID_<role>。
# 用法：CC_PROJECT=boss ./board-watchdog.sh    （crontab 安装见 board-setup-cron.sh）
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
SEND="$(dirname "$0")/board-send.sh"
STALL_TODO_MIN="${STALL_TODO_MIN:-10}"
STALL_HB_MIN="${STALL_HB_MIN:-15}"
MSG_WINDOW_H="${MSG_WINDOW_H:-24}"          # 群消息回看窗口（小时）
NOW_TS=$(date +%s)
START_ISO=$(date -d "$MSG_WINDOW_H hours ago" "+%Y-%m-%dT%H:%M:%S")
MSG_CACHE=$(mktemp -d)
trap 'rm -rf "$MSG_CACHE"' EXIT

age_min(){ # <"YYYY-MM-DD HH:MM:SS"> → 距今分钟数；解析失败输出 -1
  local ts
  ts=$(date -d "$1" +%s 2>/dev/null) || { echo -1; return; }
  echo $(( (NOW_TS - ts) / 60 ))
}

push(){ # <chat> <open_id> <text> —— 单群失败不中断巡检
  "$SEND" "$1" "$2" "$3" >/dev/null || echo "WARN: push failed chat=$1" >&2
}

# chat_msgs <chat> —— 取该群近 MSG_WINDOW_H 小时消息 JSON（每群只拉一次，缓存到临时文件）。
# 只需第 1 页：--sort desc 保证最新 50 条含最近一条消息，心跳只关心"最后一条"，无需翻页。
chat_msgs(){
  local chat="$1" f="$MSG_CACHE/$1.json"
  if [ ! -f "$f" ]; then
    lark-cli im +chat-messages-list --chat-id "$chat" --start "$START_ISO" \
      --sort desc --page-size 50 --as user 2>/dev/null | _json > "$f" || echo '{}' > "$f"
  fi
  echo "$f"
}

# last_role_msg <chat> <app_id> —— 该角色 bot 在群里最后一条消息时间，规整成 "YYYY-MM-DD HH:MM:SS"；无则空。
# 按 sender.id==app_id 过滤：bot 消息 sender 为 app_id（cli_xxx），必须排除 Boss 自己的催办消息与他人消息。
last_role_msg(){
  local raw
  raw=$(jq -r --arg app "$2" '[.data.messages[]? | select(.sender.id==$app) | .create_time] | max // ""' "$(chat_msgs "$1")" 2>/dev/null) || return
  [ -n "$raw" ] || { echo ""; return; }
  date -d "$raw" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || echo ""
}

# resolve_role（角色键/裸 bot 名/label → 规范角色键，解析不出为空）来自 board-lib；此处兜住脏「角色」值，
# 实在解析不出的行由下方 case 打 SKIP_UNRESOLVED（不静默跳过卡死任务）。

# 分隔符用 \x1f（unit separator）而非 TAB：TAB 属 IFS 空白，read 会把连续 TAB 折叠，
# 空字段（如待办行的心跳/认领时间）会导致后续字段左移（实测催办消息变成「」）。
list_rows | jq -r '.data as $d | $d.record_id_list | to_entries[] | .key as $k | ($d.data[$k]) as $r
  | def fv(n): ($d.fields | index(n)) as $i | $r[$i]
      | if type=="array" then (.[0]//"" | if type=="object" then (.id // .name // "") else . end)
        elif .==null then "" else . end;
  [.value, fv("状态"), fv("角色"), fv("工作群"), fv("创建时间"), fv("心跳时间"), fv("认领时间"),
   (fv("主任务")|gsub("[\n]";" ")), (fv("子任务")|gsub("[\n]";" "))] | join("\u001f")' |
while IFS=$'\x1f' read -r rid st role chat ctime hbtime cltime mt sub; do
  [ -n "$chat" ] || continue
  role_key=$(resolve_role "$role")     # 规范角色键（兼容 label / 角色键 / 裸 bot 名）
  ov="BOT_OPENID_${role_key//-/_}"     # @唤醒用的 open_id
  av="BOT_APPID_${role_key//-/_}"      # 群消息发送者匹配用的 app_id
  case "$st" in
    待办)
      if [[ "$sub" =~ 待发起人(ou_[a-z0-9]+) ]]; then
        push "$chat" "${BASH_REMATCH[1]}" "设计待确认：「$sub」（主任务 $mt），请回复 team-leader 确认或给出修改意见"
        echo "REMIND_INITIATOR $rid"
        continue
      fi
      AGE=$(age_min "${ctime:-}")
      [ "$AGE" -ge "$STALL_TODO_MIN" ] || continue
      if [ -z "$role_key" ] || [ -z "${!ov:-}" ]; then echo "SKIP_UNRESOLVED $rid 待办 角色=[$role]"; continue; fi
      push "$chat" "${!ov}" "看板催办：待办任务「$sub」（主任务 $mt）已 ${AGE} 分钟未开始。开始前请先用**异步 subagent** 执行 task-board 技能认领（board-claim）并置进行中，再动手。"
      echo "PUSH_TODO $rid $role_key"
      ;;
    进行中)
      # 心跳由 Boss 推导：取该角色最后群消息时间，比库里心跳新就写回
      LASTMSG=""
      [ -n "${!av:-}" ] && LASTMSG=$(last_role_msg "$chat" "${!av}")
      if [ -n "$LASTMSG" ] && { [ -z "$hbtime" ] || [[ "$LASTMSG" > "$hbtime" ]]; }; then
        rec_upsert "$rid" "$(jq -nc --arg t "$LASTMSG" '{"心跳时间":$t}')" >/dev/null \
          || echo "WARN: 心跳写入失败 rid=$rid" >&2
        hbtime="$LASTMSG"
        echo "HB_WRITE $rid $role_key $LASTMSG"
      fi
      # 据最新可用时间判超时：群消息 > 库心跳 > 认领 > 创建
      LAST="$LASTMSG"; [ -n "$LAST" ] || LAST="${hbtime:-}"; [ -n "$LAST" ] || LAST="${cltime:-}"; [ -n "$LAST" ] || LAST="${ctime:-}"
      AGE=$(age_min "$LAST")
      [ "$AGE" -ge "$STALL_HB_MIN" ] || continue
      if [ -z "$role_key" ] || [ -z "${!ov:-}" ]; then echo "SKIP_UNRESOLVED $rid 进行中 角色=[$role]"; continue; fi
      push "$chat" "${!ov}" "看板催办：进行中任务「$sub」（主任务 $mt）已 ${AGE} 分钟无群消息更新。若已完成，请把工作纪要落到文档，再用**异步 subagent** 执行 task-board 技能 board-done 更新状态与产物、指派下一个任务；若未完成请继续推进。"
      echo "PUSH_STALE $rid $role_key"
      ;;
    阻塞)
      [ -n "${BOT_OPENID_team_leader:-}" ] || continue
      push "$chat" "$BOT_OPENID_team_leader" "看板催办：任务「$sub」（主任务 $mt）处于阻塞，请查看产出备注并改派或处理"
      echo "PUSH_BLOCKED $rid"
      ;;
  esac
done
echo "WATCHDOG_DONE"
