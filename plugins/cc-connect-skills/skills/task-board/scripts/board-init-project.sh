#!/usr/bin/env bash
# board-init-project.sh <项目名> <需求描述> [工作目录]
#   <需求描述> 可为文本，或 @<文件路径>（读文件全文；长/多行需求走文件，避免命令行截断或被 LLM 精简）。
#   [工作目录] 可为新建目录，也可为**已有 git 仓库**路径（幂等，不动其工作树/历史）。
# 一键初始化新项目（供 Boss/TL agent 调用）：
#   建群(拉齐角色bot+看板写入者+发起人) → 写Projects行(初始化中) → 建/复用工作目录(git init 幂等)
#   → 逐个 @bot 用 /workspace route 绑定该绝对目录 → 轮询绑定生效 → 置进行中 → @team-leader 起步
# 输出最后一行: PROJECT_READY <chat_id>
# 依赖 ~/.cc-connect/board.env 提供:
#   BOT_APPID_<role> / BOT_OPENID_<role>（team_leader/developer/tester/reviewer）
#   BOARD_WRITER_OPENID（lark-cli 登录用户，必须入群才能写 Group 字段）
#   PROJECTS_BASE_DIR（默认工作目录根）；可选 INITIATOR_OPENID（发起人）
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
NAME="${1:?usage: board-init-project.sh <项目名> <需求> [工作目录]}"
REQ="${2:?need 需求描述}"
# 需求可传 @<文件路径>：读文件全文作为需求。长/多行需求若经命令行参数传，易被 Boss(LLM) 精简成
# 一句话、或被 shell 转义/截断坏；走文件可无损承载用户原文（Boss 用 Write 写原文到文件后传 @path）。
case "$REQ" in
  @?*) REQ_FILE="${REQ#@}"; [ -f "$REQ_FILE" ] || { echo "FATAL: 需求文件不存在: $REQ_FILE" >&2; exit 1; }; REQ=$(cat "$REQ_FILE");;
esac
[ -n "${REQ//[[:space:]]/}" ] || { echo "FATAL: 需求为空" >&2; exit 1; }
ROLES=(team_leader developer tester reviewer)
ROLE_COUNT=${#ROLES[@]}
CONFIG_FILE="${CC_CONNECT_CONFIG:-$HOME/.cc-connect/config.toml}"
for r in "${ROLES[@]}"; do
  av="BOT_APPID_$r"; ov="BOT_OPENID_$r"
  [ -n "${!av:-}" ] && [ -n "${!ov:-}" ] || { echo "FATAL: board.env 缺 $av/$ov" >&2; exit 1; }
done
: "${BOARD_WRITER_OPENID:?board.env 缺 BOARD_WRITER_OPENID}"
WORKDIR="${3:-${PROJECTS_BASE_DIR:?board.env 缺 PROJECTS_BASE_DIR}/$NAME}"

CFG_VALUES=()
while IFS= read -r __cfg_line; do CFG_VALUES+=("$__cfg_line"); done < <(python3 - "$CONFIG_FILE" "$ROLE" <<'PYEOF'
import os, re, sys
cfg, project = sys.argv[1:]
try:
    text = open(cfg, encoding="utf-8").read()
except FileNotFoundError:
    text = ""
top = re.split(r'(?m)^\s*\[\[projects\]\]\s*$', text, maxsplit=1)[0]
dm = re.search(r'(?m)^\s*data_dir\s*=\s*"([^"]*)"', top)
data_dir = os.path.expanduser(os.path.expandvars(dm.group(1))) if dm else ""
app_id = ""
for b in re.split(r'(?m)^\s*\[\[projects\]\]\s*$', text)[1:]:
    m = re.search(r'(?m)^\s*name\s*=\s*"([^"]+)"', b)
    if m and m.group(1) == project:
        am = re.search(r'(?m)^\s*app_id\s*=\s*"([^"]+)"', b)
        app_id = am.group(1) if am else ""
        break
print(app_id)
print(data_dir)
PYEOF
)
CALLER_APPID_CONFIG="${CFG_VALUES[0]:-}"
CFG_DATA_DIR="${CFG_VALUES[1]:-}"

# 1) 建群：角色 bot + Boss（watchdog 巡检要在群里发催办）+ 调用者自己的 bot app（否则后续
#    board-send 报 230002 不在群）+ 看板写入者(+发起人)
APPIDS="$BOT_APPID_team_leader,$BOT_APPID_developer,$BOT_APPID_tester,$BOT_APPID_reviewer"
[ -n "${BOT_APPID_boss:-}" ] && [[ ",$APPIDS," != *",$BOT_APPID_boss,"* ]] && APPIDS="$APPIDS,$BOT_APPID_boss"
CALLER_APPID_VAR="BOT_APPID_${ROLE//-/_}"
CALLER_APPID="${!CALLER_APPID_VAR:-}"
[ -n "$CALLER_APPID" ] || CALLER_APPID="$CALLER_APPID_CONFIG"
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

# 3) 工作目录（/workspace route 要求绝对路径且不展开 ~/相对：先建目录，再归一为绝对**物理**路径）
#    必须 pwd -P 解析 symlink（如 dev-sg 的 /home→/data00/home）：cc-connect 的 route 会把路径归一成
#    物理路径存进 workspace_bindings；若这里用逻辑路径(pwd -L)，第 5 步轮询比对会因 symlink 差异误判未绑定。
mkdir -p "$WORKDIR"
WORKDIR=$(cd "$WORKDIR" && pwd -P)
( cd "$WORKDIR" && git init -q 2>/dev/null || true )
echo "workdir=$WORKDIR"

# 3b) 完整需求原文落 .board/（agent 间通信的中间产物：跟 feature 放一起、但不进 git、收尾自动清理）。
#     .board/.gitignore 用通配自忽略——整个 .board/ 对 git 隐形，既不污染交付仓库、也不动仓库自身 .gitignore
#     （已有仓库路由亦安全）。TL 从此读完整需求做设计，避免 @消息/字段被截断而丢意图。
mkdir -p "$WORKDIR/.board"
printf '*\n' > "$WORKDIR/.board/.gitignore"
printf '# 需求（发起人原文）\n\n%s\n' "$REQ" > "$WORKDIR/.board/requirement.md"
echo "requirement_doc=$WORKDIR/.board/requirement.md"

# 4) 逐个 @bot 绑定 workspace（board-send 以 CC_PROJECT 身份发；bot 处理需数秒）
SEND="$(dirname "$0")/board-send.sh"
WORKDIR_ARG="'${WORKDIR//\'/\'\\\'\'}'"
for r in "${ROLES[@]}"; do
  ov="BOT_OPENID_$r"
  "$SEND" "$CHAT" "${!ov}" "/workspace route $WORKDIR_ARG" >/dev/null
  sleep 2
done

# 5) 轮询绑定生效（workspace_bindings.json，最多 150s）
DATA_DIR="$CFG_DATA_DIR"
[ -n "$DATA_DIR" ] || DATA_DIR="$HOME/.cc-connect"
WB="$DATA_DIR/workspace_bindings.json"
for i in $(seq 1 30); do
  N=$(jq --arg c "feishu:$CHAT" --arg lc "lark:$CHAT" --arg w "$WORKDIR" '
    . as $root
    | ["team-leader","developer","tester","reviewer"]
    | map("project:" + .)
    | map(select((($root[.][$c].workspace // "") == $w) or (($root[.][$lc].workspace // "") == $w)))
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

# 6) 置进行中
lark-cli base +record-upsert --base-token "$BOARD_BASE" --table-id "$TBL_PROJECTS" --record-id "$PRID" \
  --json '{"项目状态":"进行中"}' --as user >/dev/null

# 7) @team-leader 起步（完整需求以 .board/requirement.md 为准；消息附原文做冗余，二者皆为完整原文）
"$SEND" "$CHAT" "$BOT_OPENID_team_leader" "新项目「${NAME}」。完整需求（发起人原文）已写入工作目录 .board/requirement.md，**以该文档全文为准，勿凭节选臆测或自行删减**。工作群与看板已就绪、workspace 已绑定，发起人=${INITIATOR_OPENID:-$BOARD_WRITER_OPENID}。请按 task-board 技能『项目启动·先分流』处理（主任务=${NAME}, 工作群=${CHAT}）：先通读 .board/requirement.md 全文；目标明确、无需设计/取舍/多角色拆解的建单/查询/整理/通知/简单操作可直接开始；需要协作但需求清楚则轻量设计后派发；复杂、含糊或有关键取舍时才走 brainstorming → writing-plans，并在需要确认时 @发起人。

需求原文：
${REQ}" >/dev/null
echo "PROJECT_READY $CHAT"
