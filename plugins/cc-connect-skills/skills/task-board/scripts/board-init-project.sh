#!/usr/bin/env bash
# board-init-project.sh <项目名> <需求描述> [工作目录]
# 一键初始化新项目（供 Boss/TL agent 调用）：
#   建群(拉齐角色bot+看板写入者+发起人) → 写Projects行(初始化中) → 建工作目录(git init)
#   → 逐个 @bot 绑定 workspace → 轮询绑定生效 → 置进行中 → @team-leader 起步
# 输出最后一行: PROJECT_READY <chat_id>
# 依赖 ~/.cc-connect/board.env 提供:
#   BOT_APPID_<role> / BOT_OPENID_<role>（team_leader/developer/tester/reviewer）
#   BOARD_WRITER_OPENID（lark-cli 登录用户，必须入群才能写 Group 字段）
#   PROJECTS_BASE_DIR（默认工作目录根）；可选 INITIATOR_OPENID（发起人）
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
NAME="${1:?usage: board-init-project.sh <项目名> <需求> [工作目录]}"
REQ="${2:?need 需求描述}"
ROLES=(team_leader developer tester reviewer)
ROLE_COUNT=${#ROLES[@]}
CONFIG_FILE="${CC_CONNECT_CONFIG:-$HOME/.cc-connect/config.toml}"
for r in "${ROLES[@]}"; do
  av="BOT_APPID_$r"; ov="BOT_OPENID_$r"
  [ -n "${!av:-}" ] && [ -n "${!ov:-}" ] || { echo "FATAL: board.env 缺 $av/$ov" >&2; exit 1; }
done
: "${BOARD_WRITER_OPENID:?board.env 缺 BOARD_WRITER_OPENID}"
WORKDIR="${3:-${PROJECTS_BASE_DIR:?board.env 缺 PROJECTS_BASE_DIR}/$NAME}"

config_value(){ python3 - "$CONFIG_FILE" "$1" "$2" <<'PYEOF'
import re, sys
cfg, project, field = sys.argv[1:]
text = open(cfg, encoding="utf-8").read()
blocks = re.split(r'(?m)^\s*\[\[projects\]\]\s*$', text)[1:]
for b in blocks:
    m = re.search(r'(?m)^\s*name\s*=\s*"([^"]+)"', b)
    if not m or m.group(1) != project:
        continue
    platform_blocks = re.split(r'(?m)^\s*\[\[projects\.platforms\]\]\s*$', b)[1:]
    if not platform_blocks:
        break
    platform = platform_blocks[0]
    if field == "platform":
        pm = re.search(r'(?m)^\s*type\s*=\s*"([^"]+)"', platform)
        if pm:
            print(pm.group(1))
    else:
        fm = re.search(r'(?m)^\s*' + re.escape(field) + r'\s*=\s*"([^"]+)"', platform)
        if fm:
            print(fm.group(1))
    break
PYEOF
}

top_config_value(){ python3 - "$CONFIG_FILE" "$1" <<'PYEOF'
import re, sys
cfg, field = sys.argv[1:]
text = open(cfg, encoding="utf-8").read()
top = re.split(r'(?m)^\s*\[\[projects\]\]\s*$', text, maxsplit=1)[0]
m = re.search(r'(?m)^\s*' + re.escape(field) + r'\s*=\s*"([^"]*)"', top)
if m:
    print(m.group(1))
PYEOF
}

# 1) 建群：角色 bot + 调用者自己的 bot app（否则后续 board-send 报 230002 不在群）+ 看板写入者(+发起人)
APPIDS="$BOT_APPID_team_leader,$BOT_APPID_developer,$BOT_APPID_tester,$BOT_APPID_reviewer"
CALLER_APPID_VAR="BOT_APPID_${ROLE//-/_}"
CALLER_APPID="${!CALLER_APPID_VAR:-}"
[ -n "$CALLER_APPID" ] || CALLER_APPID="$(config_value "$ROLE" app_id)"
if [ -n "$CALLER_APPID" ] && [[ ",$APPIDS," != *",$CALLER_APPID,"* ]]; then APPIDS="$APPIDS,$CALLER_APPID"; fi
USERS="$BOARD_WRITER_OPENID"
[ -n "${INITIATOR_OPENID:-}" ] && [ "$INITIATOR_OPENID" != "$BOARD_WRITER_OPENID" ] && USERS="$USERS,$INITIATOR_OPENID"
CHAT=$(lark-cli im +chat-create --name "$NAME" --bots "$APPIDS" --users "$USERS" --as user | jq -r '.data.chat_id // .data.chat.chat_id // empty')
[ -n "$CHAT" ] || { echo "FATAL: 建群失败" >&2; exit 1; }
echo "chat=$CHAT"

# 2) Projects 行（初始化中；工作群为 Group 字段——写入者已在群，可写）
PJSON=$(jq -nc --arg n "$NAME" --arg c "$CHAT" --arg q "$REQ" --arg init "${INITIATOR_OPENID:-$BOARD_WRITER_OPENID}" \
  '{"主任务名":$n,"项目状态":"初始化中","工作群":[{"id":$c}],"需求描述":$q,"参与角色":"team-leader,developer,tester,reviewer","发起人":$init}')
PR=$(lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --json "$PJSON" --as user)
PRID=$(echo "$PR" | jq -r '.data.record.record_id_list[0] // empty')
[ -n "$PRID" ] || { echo "FATAL: 项目行写入失败: $PR" >&2; exit 1; }
echo "project_row=$PRID"

mark_failed(){
  lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --record-id "$PRID" \
    --json '{"项目状态":"初始化失败"}' --as user >/dev/null || true
}
trap 'mark_failed' ERR

# 3) 工作目录
mkdir -p "$WORKDIR"; ( cd "$WORKDIR" && git init -q 2>/dev/null || true )
echo "workdir=$WORKDIR"

# 4) 逐个 @bot 绑定 workspace（board-send 以 CC_PROJECT 身份发；bot 处理需数秒）
SEND="$(dirname "$0")/board-send.sh"
for r in "${ROLES[@]}"; do
  ov="BOT_OPENID_$r"
  "$SEND" "$CHAT" "${!ov}" "/workspace init '$WORKDIR'" >/dev/null
  sleep 2
done

# 5) 轮询绑定生效（workspace_bindings.json，最多 150s）
DATA_DIR="$(top_config_value data_dir)"
[ -n "$DATA_DIR" ] || DATA_DIR="$HOME/.cc-connect"
WB="$DATA_DIR/workspace_bindings.json"
PLATFORM="$(config_value "$ROLE" platform)"
[ -n "$PLATFORM" ] || PLATFORM="feishu"
CHANNEL_KEY="$PLATFORM:$CHAT"
for i in $(seq 1 30); do
  N=$(jq --arg c "$CHANNEL_KEY" --arg w "$WORKDIR" '
    . as $root
    | ["team-leader","developer","tester","reviewer"]
    | map("project:" + .)
    | map(select(($root[.][$c].workspace // "") == $w))
    | length
  ' "$WB" 2>/dev/null || echo 0)
  [ "$N" -ge "$ROLE_COUNT" ] && break
  sleep 5
done
if [ "${N:-0}" -lt "$ROLE_COUNT" ]; then
  echo "FATAL: 仅 $N/$ROLE_COUNT 个 bot 完成 workspace 绑定，项目初始化失败" >&2
  mark_failed
  exit 1
fi

trap - ERR
# 6) 置进行中
lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --record-id "$PRID" \
  --json '{"项目状态":"进行中"}' --as user >/dev/null

# 7) @team-leader 起步
"$SEND" "$CHAT" "$BOT_OPENID_team_leader" "新项目「${NAME}」，需求=${REQ}。工作群与看板已就绪、workspace 已绑定。请按 task-board 技能拆解首个可开工子任务（主任务=${NAME}, 工作群=${CHAT}）并派发。" >/dev/null
echo "PROJECT_READY $CHAT"
