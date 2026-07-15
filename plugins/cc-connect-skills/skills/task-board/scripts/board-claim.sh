#!/usr/bin/env bash
# board-claim.sh <record_id> —— 令牌锁认领：读→校验待办→写nonce→抖动→回读校验
# 成功: 输出 "CLAIMED <nonce>" exit 0；失败: "LOST"/"NOT_TODO" exit 1
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-claim.sh <record_id>}"
J=$(rec_get "$RID")
ST=$(rec_field "$J" "状态")
[ "$ST" = "待办" ] || { echo "NOT_TODO($ST)"; exit 1; }
NONCE="${ROLE}-$(date +%s%N)-$$"
rec_upsert "$RID" "$(jq -nc --arg n "$NONCE" --arg r "$ROLE" --arg t "$(NOW)" '{"状态":"进行中","认领人":$r,"认领令牌":$n,"认领时间":$t,"心跳时间":$t}')" >/dev/null
sleep 0.$((RANDOM%15+5))   # 抖动 0.5~2s
J2=$(rec_get "$RID")
[ "$(rec_field "$J2" "认领令牌")" = "$NONCE" ] && { echo "CLAIMED $NONCE"; exit 0; } || { echo "LOST"; exit 1; }
