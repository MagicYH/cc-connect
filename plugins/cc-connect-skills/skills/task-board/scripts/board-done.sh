#!/usr/bin/env bash
# board-done.sh <record_id> <my_nonce> <产出备注> ( --next <角色> <子任务> | --last )
# fenced 置完成 + 强制交代下一步（防"完成后忘记派发"——实测最高频的断链点）：
#   --next <角色> <子任务> ：自动建后继任务行（同主任务/同工作群，来源=本行）并 @ 唤醒该角色
#   --last                ：本主任务已无后续工作——若已无 待办/进行中 行，自动建「收尾验收」行给 team-leader 并 @ 它
#                          （调用者自己就是 team-leader 时仅置完成，不再自我派发）
# 不带二选一参数直接报错退出。
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-done.sh <record_id> <my_nonce> <产出> ( --next <角色> <子任务> | --last )}"
TOK="${2:?need nonce}"; NOTE="${3:?need 产出备注}"; MODE="${4:-}"
case "$MODE" in
  --next) NEXT_ROLE="${5:?--next 需要 <角色>}"; NEXT_SUB="${6:?--next 需要 <子任务>}";;
  --last) ;;
  *) echo "ERROR: 必须指定 --next <角色> <子任务> 或 --last（完成必须交代下一步）" >&2; exit 3;;
esac
J=$(rec_get "$RID")
[ "$(rec_field "$J" "认领令牌")" = "$TOK" ] || { echo "FENCED"; exit 2; }
MT=$(rec_field "$J" "主任务"); CHAT=$(rec_chat "$J" "工作群")
rec_upsert "$RID" "$(jq -nc --arg n "$NOTE" --arg t "$(NOW)" '{"状态":"完成","产出备注":$n,"完成时间":$t}')" >/dev/null
echo DONE
SEND="$(dirname "$0")/board-send.sh"; NEWT="$(dirname "$0")/board-new-task.sh"
if [ "$MODE" = "--next" ]; then
  NRID=$("$NEWT" "$MT" "$CHAT" "$NEXT_ROLE" "$NEXT_SUB" "$RID")
  ov="BOT_OPENID_${NEXT_ROLE//-/_}"
  [ -n "${!ov:-}" ] && "$SEND" "$CHAT" "${!ov}" "看板有新任务（主任务 $MT）：$NEXT_SUB，请用 task-board 技能处理" >/dev/null
  echo "NEXT $NRID"
else
  # --last：检查主任务下是否还有 待办/进行中（除本行外）
  LEFT=$(list_rows | jq -r --arg mt "$MT" --arg rid "$RID" '.data as $d | [$d.record_id_list | to_entries[] | select(.value != $rid) | .key as $k | ($d.data[$k]) as $r | (($d.fields | index("主任务")) as $i | $r[$i] // "") as $m | (($d.fields | index("状态")) as $j | $r[$j] | if type=="array" then (.[0]//"") else (.//"") end) as $s | select($m==$mt and ($s=="待办" or $s=="进行中"))] | length')
  if [ "$LEFT" -eq 0 ] && [ "$ROLE" != "team-leader" ]; then
    NRID=$("$NEWT" "$MT" "$CHAT" "team-leader" "收尾验收：$MT 全部子任务已完成，请验收并置项目已完成" "$RID")
    [ -n "${BOT_OPENID_team_leader:-}" ] && "$SEND" "$CHAT" "$BOT_OPENID_team_leader" "看板有收尾验收任务（主任务 $MT），请用 task-board 技能处理" >/dev/null
    echo "CLOSEOUT $NRID"
  else
    echo "LAST ok(剩余未完行=$LEFT)"
  fi
fi
