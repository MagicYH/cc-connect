#!/usr/bin/env bash
# board-done.sh <record_id> <my_nonce> <产出备注> —— fenced 置完成+写产出与完成时间
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-done.sh <record_id> <my_nonce> <产出备注>}"; TOK="${2:?need nonce}"; shift 2
NOTE="$*"
J=$(rec_get "$RID")
[ "$(rec_field "$J" "认领令牌")" = "$TOK" ] || { echo "FENCED"; exit 2; }
rec_upsert "$RID" "$(jq -nc --arg n "$NOTE" --arg t "$(NOW)" '{"状态":"完成","产出备注":$n,"完成时间":$t}')" >/dev/null
echo DONE
