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
for r in "${ROLES[@]}"; do
  av="BOT_APPID_$r"; ov="BOT_OPENID_$r"
  [ -n "${!av:-}" ] && [ -n "${!ov:-}" ] || { echo "FATAL: board.env 缺 $av/$ov" >&2; exit 1; }
done
: "${BOARD_WRITER_OPENID:?board.env 缺 BOARD_WRITER_OPENID}"
WORKDIR="${3:-${PROJECTS_BASE_DIR:?board.env 缺 PROJECTS_BASE_DIR}/$NAME}"

# 1) 建群：角色 bot + 看板写入者(+发起人)
APPIDS="$BOT_APPID_team_leader,$BOT_APPID_developer,$BOT_APPID_tester,$BOT_APPID_reviewer"
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

# 3) 工作目录
mkdir -p "$WORKDIR"; ( cd "$WORKDIR" && git init -q 2>/dev/null || true )
echo "workdir=$WORKDIR"

# 4) 逐个 @bot 绑定 workspace（board-send 以 CC_PROJECT 身份发；bot 处理需数秒）
SEND="$(dirname "$0")/board-send.sh"
for r in "${ROLES[@]}"; do
  ov="BOT_OPENID_$r"
  "$SEND" "$CHAT" "${!ov}" "/workspace init $WORKDIR" >/dev/null
  sleep 2
done

# 5) 轮询绑定生效（workspace_bindings.json，最多 150s）
WB="$HOME/.cc-connect/workspace_bindings.json"
for i in $(seq 1 30); do
  N=$(jq --arg c "feishu:$CHAT" '[to_entries[] | select(.key | startswith("project:")) | select(.value | has($c))] | length' "$WB" 2>/dev/null || echo 0)
  [ "$N" -ge 4 ] && break
  sleep 5
done
[ "${N:-0}" -ge 4 ] || { echo "WARN: 仅 $N/4 个 bot 完成 workspace 绑定（可稍后在群里补发 /workspace init）" >&2; }

# 6) 置进行中
lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --record-id "$PRID" \
  --json '{"项目状态":"进行中"}' --as user >/dev/null

# 7) @team-leader 起步
"$SEND" "$CHAT" "$BOT_OPENID_team_leader" "新项目「$NAME」，需求=$REQ。工作群与看板已就绪、workspace 已绑定。请按 task-board 技能拆解首个可开工子任务（主任务=$NAME, 工作群=$CHAT）并派发。" >/dev/null
echo "PROJECT_READY $CHAT"
