#!/usr/bin/env bash
# board-new-task.sh <主任务> <群chatID> <角色> <子任务> [来源record_id] —— 建待办行（创建人=自己）
# 角色自动映射为"角色(Bot名)"标签（board.env 的 BOT_LABEL_*）；群chatID 写入 Group 字段格式。
# 幂等保护：已存在 同主任务+同角色+同子任务 且 待办/进行中 → 返回 "DUP <rid>" 不重复建。
# ⚠️ 输出 rid 即创建成功凭证；**不要**再查表复核、不要重试——bitable 读后写有秒级延迟，
#    复核会看不到刚建的行，导致重复建行（实测踩坑）。
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
MT="${1:?usage: board-new-task.sh <主任务> <群chatID> <角色> <子任务> [来源rid]}"
CHAT="${2:?need chatID}"; TGT_RAW="${3:?need 角色}"; SUB="${4:?need 子任务}"; SRC="${5:-}"
TGT=$(role_label "$TGT_RAW")
EXIST=$(list_rows | jq -r --arg mt "$MT" --arg r "$TGT_RAW" --arg s "$SUB" '.data as $d | $d.record_id_list | to_entries[] | .key as $k | ($d.data[$k]) as $row | def fv(n): ($d.fields | index(n)) as $i | $row[$i] | if type=="array" then (.[0]//"" | if type=="object" then (.name // .id // "") else . end) elif .==null then "" else . end; select(fv("主任务")==$mt and (fv("角色")==$r or (fv("角色")|startswith($r+"("))) and fv("子任务")==$s and (fv("状态")=="待办" or fv("状态")=="进行中")) | .value' | head -1)
[ -n "$EXIST" ] && { echo "DUP $EXIST"; exit 0; }
OUT=$(rec_upsert "-" "$(jq -nc --arg mt "$MT" --arg c "$CHAT" --arg r "$TGT" --arg s "$SUB" --arg src "$SRC" --arg me "$ROLE_LABEL" '{"主任务":$mt,"群chatID":[{"id":$c}],"角色":$r,"子任务":$s,"状态":"待办","来源任务":$src,"创建人":$me}')")
RID=$(echo "$OUT" | jq -r '.data.record.record_id_list[0] // .data.record_id_list[0] // empty')
[ -n "$RID" ] || { echo "CREATE_FAILED: $OUT" >&2; exit 1; }
echo "$RID"
