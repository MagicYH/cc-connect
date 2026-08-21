#!/usr/bin/env bash
# board-complete-project.sh <主任务名> —— Projects 行置已完成 + 完成时间
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
MT="${1:?usage: board-complete-project.sh <主任务名>}"
: "${TBL_PROJECTS:?TBL_PROJECTS not set}"

J=$(list_table_rows "$TBL_PROJECTS")
MATCHES=()
while IFS= read -r _line; do [ -n "$_line" ] && MATCHES+=("$_line"); done < <(echo "$J" | jq -r --arg mt "$MT" '.data as $d | (($d.fields | index("主任务名")) // error("PROJECT_FIELD_MISSING 主任务名")) as $i | $d.record_id_list | to_entries[] | .key as $k | ($d.data[$k]) as $row | select(($row[$i] // "")==$mt) | .value')
[ "${#MATCHES[@]}" -gt 0 ] || { echo "PROJECT_NOT_FOUND $MT" >&2; exit 1; }
[ "${#MATCHES[@]}" -eq 1 ] || { echo "PROJECT_AMBIGUOUS $MT ${MATCHES[*]}" >&2; exit 1; }
RID="${MATCHES[0]}"

# 任务最终结束前，清理 agent 间通信的中间产物（工作区里的 .board/，gitignore、不进 git）。
# 从项目行的「工作群」解析出本角色绑定的 workspace，删除其 .board/。尽力而为——解析不到/删除失败
# 均不阻断收尾（仅告警），保证项目行照常置完成。
CHAT=$(echo "$J" | jq -r --arg rid "$RID" '
  .data as $d
  | ($d.fields | index("工作群")) as $ci
  | if $ci == null then "" else
      ($d.record_id_list | index($rid)) as $ri
      | if $ri == null then "" else
          ($d.data[$ri][$ci]
           | if type=="array" then (.[0].id // .[0] // "")
             elif type=="object" then (.id // "")
             elif .==null then "" else . end)
        end
    end' 2>/dev/null || true)
if [ -n "$CHAT" ]; then
  WS=$(resolve_workspace "$CHAT")
  if [[ "$WS" = /* ]] && [ -d "$WS" ]; then
    WS_REAL=$(cd -- "$WS" && pwd -P)
    if [ -n "$WS_REAL" ] && [ "$WS_REAL" != "/" ] && [ -d "$WS_REAL/.board" ]; then
      rm -rf -- "$WS_REAL/.board" && echo "cleaned_artifacts=$WS_REAL/.board" || echo "WARN: 清理中间产物失败: $WS_REAL/.board" >&2
    fi
  fi
fi

lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --record-id "$RID" \
  --json "$(jq -nc --arg t "$(NOW)" '{"项目状态":"已完成","完成时间":$t}')" --as user >/dev/null

echo "PROJECT_DONE $RID"
