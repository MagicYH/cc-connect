#!/usr/bin/env bash
# board-block.sh <record_id> <my_nonce> <原因> —— fenced 置阻塞+写原因（错派/卡住时用，之后应 @TL 求助）
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-block.sh <record_id> <my_nonce> <原因>}"; TOK="${2:?need nonce}"; shift 2
REASON="$*"
J=$(rec_get "$RID")
[ "$(rec_field "$J" "认领令牌")" = "$TOK" ] || { echo "FENCED"; exit 2; }
rec_upsert "$RID" "$(jq -nc --arg n "$REASON" '{"状态":"阻塞","产出备注":$n}')" >/dev/null
echo BLOCKED
