#!/usr/bin/env bash
# board-new-task.sh <主任务> <群chatID> <角色> <子任务> [来源record_id] —— 建待办行（创建人=自己）
# 输出新行 record_id
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
MT="${1:?usage: board-new-task.sh <主任务> <群chatID> <角色> <子任务> [来源rid]}"
CHAT="${2:?need chatID}"; TGT="${3:?need 角色}"; SUB="${4:?need 子任务}"; SRC="${5:-}"
OUT=$(rec_upsert "-" "$(jq -nc --arg mt "$MT" --arg c "$CHAT" --arg r "$TGT" --arg s "$SUB" --arg src "$SRC" --arg me "$ROLE" '{"主任务":$mt,"群chatID":$c,"角色":$r,"子任务":$s,"状态":"待办","来源任务":$src,"创建人":$me}')")
echo "$OUT" | jq -r '.data.record.record_id_list[0] // .data.record_id_list[0] // empty'
