#!/usr/bin/env bash
# board-project-context.sh <task_record_id> —— 输出当前任务的项目需求、任务链和上游产出，供角色开工前阅读。
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
RID="${1:?usage: board-project-context.sh <task_record_id>}"
: "${TBL_PROJECTS:?TBL_PROJECTS not set}"

TASK=$(rec_get "$RID")
MT=$(rec_field "$TASK" "主任务")
SUB=$(rec_field "$TASK" "子任务")
CHAT=$(rec_chat "$TASK" "工作群")
SRC=$(rec_field "$TASK" "来源任务")
ROLE_FIELD=$(rec_field "$TASK" "角色")
STATUS=$(rec_field "$TASK" "状态")

MATCHES=""
OFFSET=0
while :; do
  PROJECTS=$(lark-cli base +record-list --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --limit 200 --offset "$OFFSET" --format json --as user | _json)
  PAGE_COUNT=$(echo "$PROJECTS" | jq -r '(.data.record_id_list // []) | length')
  PAGE_MATCHES=$(echo "$PROJECTS" | jq -r --arg mt "$MT" '
    .data as $d
    | $d.record_id_list
    | to_entries[]
    | .key as $k
    | ($d.data[$k]) as $row
    | def fv(n): ($d.fields | index(n)) as $i | $row[$i] | if type=="array" then (.[0]//"" | if type=="object" then (.name // .id // "") else . end) elif .==null then "" else . end;
    select(fv("主任务名") == $mt)
    | [.value, fv("项目状态"), fv("需求描述"), fv("发起人"), fv("参与角色"), fv("工作群")]
    | @tsv')
  if [ -n "$PAGE_MATCHES" ]; then
    MATCHES="${MATCHES}${MATCHES:+$'\n'}${PAGE_MATCHES}"
  fi
  [ "$PAGE_COUNT" -gt 0 ] || break
  OFFSET=$((OFFSET + 200))
done
COUNT=$(printf '%s\n' "$MATCHES" | sed '/^$/d' | wc -l | tr -d ' ')
if [ "$COUNT" -eq 0 ]; then
  echo "PROJECT_NOT_FOUND $MT" >&2
  exit 2
fi
if [ "$COUNT" -gt 1 ]; then
  IDS=$(printf '%s\n' "$MATCHES" | cut -f1 | xargs)
  echo "PROJECT_AMBIGUOUS $MT $IDS" >&2
  exit 3
fi
IFS=$'\t' read -r PRID PSTATUS REQ INIT ROLES PCHAT <<<"$MATCHES"

printf '# Task Board Context\n\n'
printf '主任务：%s\n' "$MT"
printf '项目记录：%s\n' "$PRID"
printf '项目状态：%s\n' "$PSTATUS"
printf '当前任务：%s\n' "$SUB"
printf '当前任务记录：%s\n' "$RID"
printf '当前任务状态：%s\n' "$STATUS"
printf '当前角色：%s\n' "$ROLE_FIELD"
printf '工作群：%s\n' "${CHAT:-$PCHAT}"
printf '发起人：%s\n' "$INIT"
printf '参与角色：%s\n' "$ROLES"
printf '\n## 需求描述\n\n%s\n' "$REQ"

if [ -n "$SRC" ]; then
  printf '\n## 来源任务\n\n'
  printf '来源任务：%s\n' "$SRC"
  if SRC_JSON=$(rec_get "$SRC" 2>/dev/null); then
    printf '来源子任务：%s\n' "$(rec_field "$SRC_JSON" "子任务")"
    printf '来源角色：%s\n' "$(rec_field "$SRC_JSON" "角色")"
    printf '上游产出：%s\n' "$(rec_field "$SRC_JSON" "产出备注")"
  fi
fi
