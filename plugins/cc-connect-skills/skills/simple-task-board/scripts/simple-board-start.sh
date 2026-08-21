#!/usr/bin/env bash
# simple-board-start.sh <群名> <任务描述> <绝对工作目录> [角色列表]
# 一次性启动轻量协作群：建群 → 拉 team-leader(+可选角色) → route workspace → @team-leader 发任务。
set -euo pipefail

NAME="${1:?usage: simple-board-start.sh <群名> <任务描述> <绝对工作目录> [角色列表]}"
TASK="${2:?need 任务描述}"
WORKDIR="${3:?need 绝对工作目录}"
ROLES_RAW="${4:-}"

valid_name(){ [[ "$1" =~ ^[A-Za-z0-9_-]+$ ]]; }
validate_route_path(){
  local p="$1"
  case "$p" in *[[:cntrl:]]*) echo "FATAL: 工作目录包含控制字符" >&2; exit 1;; esac
  [[ "$p" != *"'"* ]] || { echo "FATAL: 工作目录不能包含单引号" >&2; exit 1; }
}
_json(){ sed -n '/^{/,$p'; }
load_board_env(){
  local file="$1" line key value
  [ -f "$file" ] || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    key="${BASH_REMATCH[1]}"
    value="${BASH_REMATCH[2]}"
    case "$key" in
      BOT_APPID_*|BOT_OPENID_*|BOARD_WRITER_OPENID|INITIATOR_OPENID|SIMPLE_TASKS_BASE_TOKEN|SIMPLE_TASKS_TABLE_ID)
        if [[ "$value" == \"*\" && "$value" == *\" ]]; then
          value="${value:1:${#value}-2}"
        elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
          value="${value:1:${#value}-2}"
        fi
        printf -v "$key" '%s' "$value"
        export "$key"
        ;;
    esac
  done < "$file"
}

BOARD_ENV="${BOARD_ENV:-$HOME/.cc-connect/board.env}"
load_board_env "$BOARD_ENV"
: "${SIMPLE_TASKS_BASE_TOKEN:?SIMPLE_TASKS_BASE_TOKEN not set (write ~/.cc-connect/board.env or export it)}"
: "${SIMPLE_TASKS_TABLE_ID:?SIMPLE_TASKS_TABLE_ID not set (write ~/.cc-connect/board.env or export it)}"

valid_name "${CC_PROJECT:?CC_PROJECT not set}" || { echo "FATAL: invalid CC_PROJECT: $CC_PROJECT" >&2; exit 1; }
[ -n "${TASK//[[:space:]]/}" ] || { echo "FATAL: 任务为空" >&2; exit 1; }
[[ "$TASK" != @* ]] || { echo "FATAL: @file task input is not supported; pass task text directly" >&2; exit 1; }
[[ "$WORKDIR" = /* ]] || { echo "FATAL: 工作目录必须是绝对路径: $WORKDIR" >&2; exit 1; }
validate_route_path "$WORKDIR"
[ -d "$WORKDIR" ] || { echo "FATAL: 工作目录不存在: $WORKDIR" >&2; exit 1; }
WORKDIR=$(cd -- "$WORKDIR" && pwd -P)
validate_route_path "$WORKDIR"

role_var(){ echo "${1//-/_}"; }
role_appid(){ local v="BOT_APPID_$(role_var "$1")"; echo "${!v:-}"; }
role_openid(){ local v="BOT_OPENID_$(role_var "$1")"; echo "${!v:-}"; }
require_role(){
  local r="$1" app open
  valid_name "$r" || { echo "FATAL: invalid role: $r" >&2; exit 1; }
  app=$(role_appid "$r")
  open=$(role_openid "$r")
  [ -n "$app" ] && [ -n "$open" ] || { echo "FATAL: unknown role or missing BOT_APPID/BOT_OPENID for $r" >&2; exit 1; }
}
add_role(){
  local r="$1" seen
  [ -n "$r" ] || return 0
  r="${r//_/-}"
  require_role "$r"
  for seen in "${ROLES[@]}"; do [ "$seen" = "$r" ] && return 0; done
  ROLES+=("$r")
}
append_csv_unique(){
  local current="$1" item="$2"
  [ -n "$item" ] || { echo "$current"; return; }
  [[ ",$current," == *",$item,"* ]] && { echo "$current"; return; }
  [ -n "$current" ] && echo "$current,$item" || echo "$item"
}
quote_route_path(){
  local p="$1"
  printf "'%s'" "${p//\'/\'\\\'\'}"
}
caller_appid_from_config(){
  local cfg="${CC_CONNECT_CONFIG:-$HOME/.cc-connect/config.toml}" project="$1"
  [ -f "$cfg" ] || return 0
  python3 - "$cfg" "$project" <<'PYEOF'
import re, sys
cfg, project = sys.argv[1:]
try:
    text = open(cfg, encoding="utf-8").read()
except OSError:
    sys.exit(0)
for block in re.split(r'(?m)^\s*\[\[projects\]\]\s*$', text)[1:]:
    name = re.search(r'(?m)^\s*name\s*=\s*"([^"]+)"', block)
    if not name or name.group(1) != project:
        continue
    app_id = re.search(r'(?m)^\s*app_id\s*=\s*"([^"]+)"', block)
    if app_id:
        print(app_id.group(1))
    break
PYEOF
}
record_simple_task(){
  local chat="$1" roles="$2" initiator payload
  initiator="${INITIATOR_OPENID:-${BOARD_WRITER_OPENID:-}}"
  payload=$(jq -nc \
    --arg name "$NAME" \
    --arg status "进行中" \
    --arg chat "$chat" \
    --arg task "$TASK" \
    --arg roles "$roles" \
    --arg initiator "$initiator" \
    '{"主任务名":$name,"项目状态":$status,"工作群":[{"id":$chat}],"需求描述":$task,"参与角色":$roles,"发起人":$initiator}')
  if ! lark-cli base +record-upsert \
    --base-token "$SIMPLE_TASKS_BASE_TOKEN" \
    --table-id "$SIMPLE_TASKS_TABLE_ID" \
    --json "$payload" \
    --as user >/dev/null 2>/dev/null; then
    echo "FATAL: Simple Tasks record write failed" >&2
    return 1
  fi
}

ROLES=()
add_role "team-leader"
if [ -n "$ROLES_RAW" ]; then
  NORMALIZED=${ROLES_RAW//,/ }
  read -r -a REQUESTED_ROLES <<< "$NORMALIZED"
  for r in "${REQUESTED_ROLES[@]}"; do
    add_role "$r"
  done
fi

APPIDS=""
for r in "${ROLES[@]}"; do
  APPIDS=$(append_csv_unique "$APPIDS" "$(role_appid "$r")")
done
[ -n "${BOT_APPID_boss:-}" ] && APPIDS=$(append_csv_unique "$APPIDS" "$BOT_APPID_boss")
CALLER_APPID_VAR="BOT_APPID_${CC_PROJECT//-/_}"
CALLER_APPID="${!CALLER_APPID_VAR:-}"
[ -n "$CALLER_APPID" ] || CALLER_APPID=$(caller_appid_from_config "$CC_PROJECT")
[ -n "$CALLER_APPID" ] || { echo "FATAL: caller bot app_id not found for $CC_PROJECT" >&2; exit 1; }
APPIDS=$(append_csv_unique "$APPIDS" "$CALLER_APPID")

USERS="${BOARD_WRITER_OPENID:-}"
if [ -n "${INITIATOR_OPENID:-}" ]; then
  USERS=$(append_csv_unique "$USERS" "$INITIATOR_OPENID")
fi

CREATE_ARGS=(im +chat-create --name "$NAME" --bots "$APPIDS" --as user)
if [ -n "$USERS" ]; then
  CREATE_ARGS+=(--users "$USERS")
fi
CHAT=$(lark-cli "${CREATE_ARGS[@]}" | _json | jq -r '.data.chat_id // .data.chat.chat_id // empty')
[ -n "$CHAT" ] || { echo "FATAL: 建群失败" >&2; exit 1; }
echo "chat=$CHAT"

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd -P)
SEND="$SCRIPT_DIR/board-send.sh"
if [ ! -x "$SEND" ]; then
  SEND="$SCRIPT_DIR/../../task-board/scripts/board-send.sh"
fi
[ -x "$SEND" ] || { echo "FATAL: board-send.sh not found" >&2; exit 1; }
ROUTE_PATH=$(quote_route_path "$WORKDIR")
for r in "${ROLES[@]}"; do
  "$SEND" "$CHAT" "$(role_openid "$r")" "/workspace route $ROUTE_PATH" >/dev/null
done

ROLE_LIST=$(IFS=,; echo "${ROLES[*]}")
"$SEND" "$CHAT" "$(role_openid team-leader)" "新任务「${NAME}」

任务目标：
${TASK}

工作目录：${WORKDIR}
参与角色：${ROLE_LIST}

请先切换到上述工作目录，确认上下文，然后组织完成这项工作。可选角色已在群内待命；默认由 team-leader 负责拆解、协调与最终交付。" >/dev/null

record_simple_task "$CHAT" "$ROLE_LIST"

echo "SIMPLE_BOARD_READY $CHAT"
