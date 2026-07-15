#!/usr/bin/env bash
# board-my-todos.sh —— 列出：①指派给我的待办 ②我角色下心跳超时(默认15分钟)可回收的进行中
# 输出 TSV：record_id  类别(TODO|RECLAIM)  子任务  主任务  工作群
# 角色匹配兼容纯角色名与"角色(Bot名)"标签；工作群 兼容 Group 字段(取 .id)与文本
set -euo pipefail
source "$(dirname "$0")/board-lib.sh"
M="${RECLAIM_MINUTES:-15}"
CUTOFF=$(date -d "-${M} min" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || date -v-${M}M "+%Y-%m-%d %H:%M:%S")
list_rows | jq -r --arg role "$ROLE" --arg cutoff "$CUTOFF" '
  .data.fields as $f
  | [(.data.data | to_entries[]), (.data.record_id_list | to_entries[])] | group_by(.key) | map({row: .[0].value, rid: .[1].value})
  | .[] | .rid as $rid | .row as $r
  | def fv(name): ($f | index(name)) as $i | $r[$i] | if type=="array" then (.[0]//"" | if type=="object" then (.name // .id // "") else . end) elif .==null then "" else . end;
  def fchat: ($f | index("工作群")) as $i | $r[$i] | if type=="array" then (.[0]//"" | if type=="object" then (.id // "") else . end) elif .==null then "" else . end;
  select(fv("角色")==$role or (fv("角色") | startswith($role+"(")))
  | if fv("状态")=="待办" then [$rid,"TODO",fv("子任务"),fv("主任务"),fchat]
    elif fv("状态")=="进行中" and fv("心跳时间") != "" and fv("心跳时间") < $cutoff then [$rid,"RECLAIM",fv("子任务"),fv("主任务"),fchat]
    else empty end
  | @tsv'
