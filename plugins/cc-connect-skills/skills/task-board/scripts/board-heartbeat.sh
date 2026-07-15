#!/usr/bin/env bash
# board-heartbeat.sh <record_id> <my_nonce> —— fenced 心跳刷新：令牌不匹配即报 FENCED（任务已被接管，立即放弃）
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-heartbeat.sh <record_id> <my_nonce>}"; TOK="${2:?need nonce}"
J=$(rec_get "$RID")
[ "$(rec_field "$J" "认领令牌")" = "$TOK" ] || { echo "FENCED"; exit 2; }
rec_upsert "$RID" "$(jq -nc --arg t "$(NOW)" '{"心跳时间":$t}')" >/dev/null
echo OK
