#!/usr/bin/env bash
# board-setup.sh [看板名] —— 一次性建看板：Base + Projects/Tasks 两表，写 ~/.cc-connect/board.env
# 适配 lark-cli v1.0.33+：type 用字符串；select 选项用平级 "options" 键（"property" 会报 Unrecognized key）
# ⚠️ 建表请求失败会留半成品表（先建表后加字段）；报"同名已存在"时先 +table-delete 清残留再重跑
set -euo pipefail
NAME="${1:-Agent任务看板}"
OUT=$(lark-cli base +base-create --name "$NAME" --as user)
BOARD_BASE=$(echo "$OUT" | jq -r '.data.base.base_token // empty')
[ -n "$BOARD_BASE" ] || { echo "FATAL: base create failed: $OUT" >&2; exit 1; }
echo "BASE=$BOARD_BASE"

PF='[{"field_name":"主任务名","type":"text"},{"field_name":"项目状态","type":"select","options":[{"name":"初始化中"},{"name":"进行中"},{"name":"已完成"},{"name":"已归档"},{"name":"初始化失败"}]},{"field_name":"群chatID","type":"group_chat"},{"field_name":"需求描述","type":"text"},{"field_name":"参与角色","type":"text"},{"field_name":"发起人","type":"text"},{"field_name":"创建时间","type":"created_at"},{"field_name":"完成时间","type":"datetime"}]'
TBL_PROJECTS=$(lark-cli base +table-create --base-token "$BOARD_BASE" --name Projects --fields "$PF" --as user | jq -r '.data.table.id // empty')
[ -n "$TBL_PROJECTS" ] || { echo "FATAL: Projects table failed（同名残留？先 +table-delete）" >&2; exit 1; }

TF='[{"field_name":"子任务","type":"text"},{"field_name":"主任务","type":"text"},{"field_name":"群chatID","type":"group_chat"},{"field_name":"角色","type":"select","options":[{"name":"team-leader(Beta)"},{"name":"developer(Delta)"},{"name":"tester(Zero)"},{"name":"reviewer(Gamma)"}]},{"field_name":"状态","type":"select","options":[{"name":"待办"},{"name":"进行中"},{"name":"完成"},{"name":"阻塞"}]},{"field_name":"认领人","type":"text"},{"field_name":"认领令牌","type":"text"},{"field_name":"认领时间","type":"datetime"},{"field_name":"心跳时间","type":"datetime"},{"field_name":"来源任务","type":"text"},{"field_name":"创建人","type":"text"},{"field_name":"产出备注","type":"text"},{"field_name":"创建时间","type":"created_at"},{"field_name":"完成时间","type":"datetime"}]'
TBL_TASKS=$(lark-cli base +table-create --base-token "$BOARD_BASE" --name Tasks --fields "$TF" --as user | jq -r '.data.table.id // empty')
[ -n "$TBL_TASKS" ] || { echo "FATAL: Tasks table failed" >&2; exit 1; }

mkdir -p ~/.cc-connect
cat > ~/.cc-connect/board.env <<EOV
BOARD_BASE=$BOARD_BASE
TBL_PROJECTS=$TBL_PROJECTS
TBL_TASKS=$TBL_TASKS
# 角色显示名映射（改成你实际的 Bot 飞书应用名；脚本据此写「角色/认领人」字段）
BOT_LABEL_team_leader="team-leader(Beta)"
BOT_LABEL_developer="developer(Delta)"
BOT_LABEL_tester="tester(Zero)"
BOT_LABEL_reviewer="reviewer(Gamma)"
EOV
echo "written ~/.cc-connect/board.env:"
cat ~/.cc-connect/board.env
echo "看板 URL: https://bytedance.my.larkoffice.com/base/$BOARD_BASE"
