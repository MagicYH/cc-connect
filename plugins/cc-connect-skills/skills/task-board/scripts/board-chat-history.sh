#!/usr/bin/env bash
# board-chat-history.sh <task_record_id> [limit|all] —— 从任务行反查本群消息并转成 Markdown，供角色交给 subagent 总结。
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-chat-history.sh <task_record_id> [limit|all]}"
LIMIT="${2:-50}"

TASK=$(rec_get "$RID")
CHAT=$(rec_chat "$TASK" "工作群")
[ -n "$CHAT" ] || { echo "CHAT_NOT_FOUND $RID" >&2; exit 2; }
TASK_ROLE=$(rec_field "$TASK" "角色")
STATUS=$(rec_field "$TASK" "状态")
if ! { [ "$TASK_ROLE" = "$ROLE" ] || [[ "$TASK_ROLE" == *"($ROLE)"* ]]; }; then
  echo "ROLE_MISMATCH $RID $TASK_ROLE" >&2
  exit 3
fi
[ "$STATUS" = "进行中" ] || { echo "TASK_NOT_IN_PROGRESS $RID $STATUS" >&2; exit 4; }

render_markdown(){
  python3 -c '
import json, sys

def text_of(msg):
    body = msg.get("body") or {}
    if isinstance(body, dict):
        for key in ("text", "content"):
            val = body.get(key)
            if isinstance(val, str) and val:
                return val
    val = msg.get("content")
    if isinstance(val, str) and val:
        try:
            data = json.loads(val)
            if isinstance(data, dict):
                return data.get("text") or data.get("content") or val
        except Exception:
            return val
    return ""

print("# Chat History")
print()
for line in sys.stdin:
    if not line.strip():
        continue
    data = json.loads(line)
    items = data.get("items") or data.get("data", {}).get("items") or data.get("data", {}).get("messages") or []
    for msg in items:
        sender = msg.get("sender") or {}
        name = sender.get("name") or sender.get("sender_name") or sender.get("id") or "unknown"
        ts = msg.get("create_time") or msg.get("update_time") or ""
        text = text_of(msg).replace("\n", " ").strip()
        if text:
            print(f"- {ts} {name}: {text}".strip())
'
}

if [ "$LIMIT" = "all" ]; then
  PAGE_TOKEN=""
  while :; do
    if [ -n "$PAGE_TOKEN" ]; then
      OUT=$(lark-cli im +chat-messages-list --chat-id "$CHAT" --page-size 50 --page-token "$PAGE_TOKEN" --as bot --format json)
    else
      OUT=$(lark-cli im +chat-messages-list --chat-id "$CHAT" --page-size 50 --as bot --format json)
    fi
    printf '%s\n' "$OUT" | jq -c '.'
    PAGE_TOKEN=$(printf '%s\n' "$OUT" | jq -r '(.page_token // .data.page_token // .next_page_token // .data.next_page_token // "")')
    HAS_MORE=$(printf '%s\n' "$OUT" | jq -r '(.has_more // .data.has_more // false)')
    [ "$HAS_MORE" = "true" ] && [ -n "$PAGE_TOKEN" ] || break
  done | render_markdown
else
  case "$LIMIT" in (*[!0-9]*|"") echo "INVALID_LIMIT $LIMIT" >&2; exit 5;; esac
  [ "$LIMIT" -le 50 ] || LIMIT=50
  lark-cli im +chat-messages-list --chat-id "$CHAT" --page-size "$LIMIT" --as bot --format json | render_markdown
fi
