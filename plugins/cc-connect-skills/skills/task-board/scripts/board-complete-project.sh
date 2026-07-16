#!/usr/bin/env bash
# board-complete-project.sh <主任务名> —— Projects 行置已完成 + 完成时间
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
MT="${1:?usage: board-complete-project.sh <主任务名>}"
: "${TBL_PROJECTS:?TBL_PROJECTS not set}"

J=$(lark-cli base +record-list --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --format json --as user)
MATCHES=()
while IFS= read -r _line; do [ -n "$_line" ] && MATCHES+=("$_line"); done < <(echo "$J" | jq -r --arg mt "$MT" '.data as $d | (($d.fields | index("主任务名")) // error("PROJECT_FIELD_MISSING 主任务名")) as $i | $d.record_id_list | to_entries[] | .key as $k | ($d.data[$k]) as $row | select(($row[$i] // "")==$mt) | .value')
[ "${#MATCHES[@]}" -gt 0 ] || { echo "PROJECT_NOT_FOUND $MT" >&2; exit 1; }
[ "${#MATCHES[@]}" -eq 1 ] || { echo "PROJECT_AMBIGUOUS $MT ${MATCHES[*]}" >&2; exit 1; }
RID="${MATCHES[0]}"

lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --record-id "$RID" \
  --json "$(jq -nc --arg t "$(NOW)" '{"项目状态":"已完成","完成时间":$t}')" --as user >/dev/null

echo "PROJECT_DONE $RID"
