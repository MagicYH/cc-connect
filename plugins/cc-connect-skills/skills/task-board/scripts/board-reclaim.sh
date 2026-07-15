#!/usr/bin/env bash
# board-reclaim.sh <record_id> —— 回收心跳超时的进行中任务：换新 nonce+回读校验（同认领锁）
# 成功: "RECLAIMED <nonce>" exit 0；失败: "NOT_STALE"/"LOST" exit 1
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-reclaim.sh <record_id>}"
M="${RECLAIM_MINUTES:-15}"
CUTOFF=$(date -d "-${M} min" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || date -v-${M}M "+%Y-%m-%d %H:%M:%S")
J=$(rec_get "$RID")
ST=$(rec_field "$J" "状态"); HB=$(rec_field "$J" "心跳时间")
{ [ "$ST" = "进行中" ] && [ -n "$HB" ] && [ "$HB" \< "$CUTOFF" ]; } || { echo "NOT_STALE(状态=$ST 心跳=$HB)"; exit 1; }
NONCE="${ROLE}-reclaim-$(date +%s%N)-$$"
rec_upsert "$RID" "$(jq -nc --arg n "$NONCE" --arg r "$ROLE" --arg t "$(NOW)" '{"认领人":$r,"认领令牌":$n,"心跳时间":$t}')" >/dev/null
sleep 0.$((RANDOM%15+5))
J2=$(rec_get "$RID")
[ "$(rec_field "$J2" "认领令牌")" = "$NONCE" ] && { echo "RECLAIMED $NONCE"; exit 0; } || { echo "LOST"; exit 1; }
