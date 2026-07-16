#!/usr/bin/env bash
# board-lib.sh —— task-board 脚本共享库。被其它 board-*.sh source，不直接执行。
# 配置来源（优先级）：环境变量 BOARD_BASE/TBL_TASKS/TBL_PROJECTS > ~/.cc-connect/board.env
set -euo pipefail
BOARD_ENV="${BOARD_ENV:-$HOME/.cc-connect/board.env}"
[ -f "$BOARD_ENV" ] && source "$BOARD_ENV"
: "${BOARD_BASE:?BOARD_BASE not set (write ~/.cc-connect/board.env or export it)}"
: "${TBL_TASKS:?TBL_TASKS not set}"
ROLE="${CC_PROJECT:?CC_PROJECT not set (must run inside a cc-connect agent session)}"
# 角色显示名（含 Bot 名，如 Beta (team-leader)）：board.env 里配 BOT_LABEL_<role下划线>；未配则等于 ROLE
_lv="BOT_LABEL_${ROLE//-/_}"; ROLE_LABEL="${!_lv:-$ROLE}"
# role_label <角色名> → 该角色的显示名（供 new-task 指派他人时用）
role_label(){ local v="BOT_LABEL_${1//-/_}"; echo "${!v:-$1}"; }
NOW(){ date "+%Y-%m-%d %H:%M:%S"; }

# _json —— 剥掉 lark-cli 偶发打在 stdout 前面的非 JSON 横幅（如版本更新提示），
#          只保留自首个 { 或 [ 起的内容（实测横幅会让下游 jq 报 Invalid numeric literal）
_json(){ sed -n '/^[[{]/,$p'; }

# rec_get <record_id>  → JSON 到 stdout
rec_get(){ lark-cli base +record-get --base-token "$BOARD_BASE" --table-id "$TBL_TASKS" --record-id "$1" --format json --as user | _json; }

# rec_field <record_get_json> <字段名> → 值（select 数组取第一项；群字段对象取 .name；空值输出空串）
rec_field(){ echo "$1" | jq -r --arg f "$2" '(.data.fields | index($f)) as $i | .data.data[0][$i] | if type=="array" then (.[0]//"" | if type=="object" then (.name // .id // "") else . end) elif .==null then "" else . end'; }
# rec_chat <record_get_json> <字段名> → 群 chatID（兼容 Group 字段对象数组与纯文本）
rec_chat(){ echo "$1" | jq -r --arg f "$2" '(.data.fields | index($f)) as $i | .data.data[0][$i] | if type=="array" then (.[0]//"" | if type=="object" then (.id // "") else . end) elif .==null then "" else . end'; }

# rec_upsert <record_id|-> <fields_json>  —— 写入；1254291 并发冲突时退避重试 3 次
rec_upsert(){
  local rid="$1" json="$2" i out
  for i in 1 2 3; do
    if [ "$rid" = "-" ]; then
      out=$(lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_TASKS" --json "$json" --as user 2>&1) && { echo "$out" | _json; return 0; }
    else
      out=$(lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_TASKS" --record-id "$rid" --json "$json" --as user 2>&1) && { echo "$out" | _json; return 0; }
    fi
    echo "$out" | grep -q 1254291 || { echo "$out" >&2; return 1; }
    sleep 0.$((RANDOM%9+1))
  done
  echo "UPSERT_RETRY_EXHAUSTED" >&2; return 1
}

# list_rows → 全表 JSON（含 .data.fields 与 .data.data[] 与 .data.record_id_list[]）
list_rows(){ lark-cli base +record-list --base-token "$BOARD_BASE" --table-id "$TBL_TASKS" --format json --as user | _json; }
