#!/usr/bin/env bash
# board-lib.sh —— task-board 脚本共享库。被其它 board-*.sh source，不直接执行。
# 配置来源（优先级）：环境变量 BOARD_BASE/TBL_TASKS/TBL_PROJECTS > ~/.cc-connect/board.env
set -euo pipefail
BOARD_ENV="${BOARD_ENV:-$HOME/.cc-connect/board.env}"
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
      BOARD_*|TBL_*|BOT_APPID_*|BOT_OPENID_*|BOT_LABEL_*|BOARD_WRITER_OPENID|INITIATOR_OPENID|TASK_TIMEOUT_MINUTES)
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
load_board_env "$BOARD_ENV"
: "${BOARD_BASE:?BOARD_BASE not set (write ~/.cc-connect/board.env or export it)}"
: "${TBL_TASKS:?TBL_TASKS not set}"
ROLE="${CC_PROJECT:?CC_PROJECT not set (must run inside a cc-connect agent session)}"
# 角色显示名（含 Bot 名，如 Beta (team-leader)）：board.env 里配 BOT_LABEL_<role下划线>；未配则等于 ROLE
_lv="BOT_LABEL_${ROLE//-/_}"; ROLE_LABEL="${!_lv:-$ROLE}"

# resolve_role <角色原值> → 规范角色键(连字符，如 team-leader / reviewer)；无法识别输出空。
# 兼容三种写法：完整 label「Beta (team-leader)」、角色键「team-leader」、裸 bot 名「Gamma」。
# 实测坑：派单方（含人类用 @Gamma）拿 bot 显示名当角色 → 旧 role_label 原样透传脏值入库，
# 派发/watchdog 都路由不到。这里统一归一；写入侧(role_label)据此把脏值挡在门外。
KNOWN_ROLES=$(compgen -v | sed -n 's/^BOT_OPENID_//p')   # 从 board.env 的 BOT_OPENID_* 动态推已知角色
resolve_role(){
  local raw="$1" cand r label name
  case "$raw" in *"("*")"*) cand="${raw##*(}"; cand="${cand%)*}";; *) cand="$raw";; esac
  local uv="BOT_OPENID_${cand//-/_}"
  [ -n "${!uv:-}" ] && { echo "${cand//_/-}"; return; }   # 已是角色键（或 label 内层就是角色键）
  for r in $KNOWN_ROLES; do                               # 否则当裸 bot 名，比对各角色 BOT_LABEL 前导名/全 label
    label="BOT_LABEL_${r}"; label="${!label:-}"; [ -n "$label" ] || continue
    name="${label%% (*}"
    { [ "$cand" = "$label" ] || [ "$cand" = "$name" ]; } && { echo "${r//_/-}"; return; }
  done
  echo ""
}

# role_label <角色原值> → 规范显示 label（"Beta (team-leader)"），new-task 指派他人写「角色」字段用。
# **归一化 + 严格**：角色键/裸bot名/label 一律归到规范 label；彻底认不出的报错退出（不再静默透传脏值）。
role_label(){
  local rk; rk=$(resolve_role "$1")
  [ -n "$rk" ] || { echo "ERROR: 无法识别角色「$1」——请用角色键 team-leader/developer/tester/reviewer（不是 Bot 显示名）" >&2; return 3; }
  local v="BOT_LABEL_${rk//-/_}"; echo "${!v:-$rk}"
}
NOW(){ date "+%Y-%m-%d %H:%M:%S"; }

# cc_data_dir → cc-connect 数据目录（config.toml 顶层 data_dir，展开 ~/$VAR；缺省 ~/.cc-connect）。
# 与 board-init-project.sh 定位 workspace_bindings.json 的口径一致。config 缺失/无 python 时静默回退默认。
cc_data_dir(){
  local cfg="${CC_CONNECT_CONFIG:-$HOME/.cc-connect/config.toml}" dd=""
  if [ -f "$cfg" ] && command -v python3 >/dev/null 2>&1; then
    dd=$(python3 - "$cfg" <<'PY' 2>/dev/null || true
import os,re,sys
try: t=open(sys.argv[1],encoding="utf-8").read()
except OSError: t=""
top=re.split(r'(?m)^\s*\[\[projects\]\]\s*$',t,1)[0]
m=re.search(r'(?m)^\s*data_dir\s*=\s*"([^"]*)"',top)
print(os.path.expanduser(os.path.expandvars(m.group(1))) if m and m.group(1) else "")
PY
)
  fi
  [ -n "$dd" ] && echo "$dd" || echo "$HOME/.cc-connect"
}

# resolve_workspace <chatID> → 当前角色($ROLE)在此群绑定的 workspace 物理路径；未绑定/无文件输出空串。
resolve_workspace(){
  local chat="$1" wb
  [ -n "$chat" ] || return 0
  wb="$(cc_data_dir)/workspace_bindings.json"
  [ -f "$wb" ] || return 0
  jq -r --arg role "project:$ROLE" --arg c "feishu:$chat" --arg lc "lark:$chat" \
    '((.[$role][$c].workspace // .[$role][$lc].workspace) // "")' "$wb" 2>/dev/null || true
}

# _json —— 剥掉 lark-cli 偶发打在 stdout 前面的非 JSON 横幅（如 `[lark-cli] [WARN] proxy detected...`
#          或版本更新提示）。只从**第一行以 { 开头**处起——这些 base/im 命令顶层都返回对象 `{...}`，
#          绝不返回裸数组；而横幅行以 `[` 开头（`[lark-cli]…`），旧的 `/^[[{]/` 会把横幅当成 JSON 起点
#          放进下游 jq → `Invalid numeric literal at line 1, column 10`（列 10 正是 `[lark-cli]` 的 `]`）。
_json(){ sed -n '/^{/,$p'; }

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

# list_table_rows <table_id> → 全表 JSON（含 .data.fields 与 .data.data[] 与 .data.record_id_list[]）
list_table_rows(){
  local table="$1" offset=0 page pages="" has_more count
  while :; do
    page=$(lark-cli base +record-list --base-token "$BOARD_BASE" --table-id "$table" --format json --as user --offset "$offset" | _json)
    pages="$pages$page
"
    has_more=$(echo "$page" | jq -r '.data.has_more // false')
    [ "$has_more" = "true" ] || break
    count=$(echo "$page" | jq -r '.data.record_id_list | length')
    [ "$count" -gt 0 ] || count=100
    offset=$((offset + count))
  done
  printf '%s' "$pages" | jq -s '
    reduce .[] as $p (null;
      if . == null then $p
      else
        .data.data += ($p.data.data // [])
        | .data.record_id_list += ($p.data.record_id_list // [])
        | .data.has_more = ($p.data.has_more // false)
      end
    )'
}

list_rows(){ list_table_rows "$TBL_TASKS"; }
