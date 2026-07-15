#!/usr/bin/env bash
set -euo pipefail
# board-send —— bot 用自己的 app 身份向群发文本消息（可带 @），用于看板派发/求助唤醒。
# 用法: board-send <chat_id> <at_open_id|-> <text...>
#   at_open_id 传 "-" 表示不 @ 任何人。
# 身份: 从 $CC_PROJECT 定位自己是哪个 bot，在 ~/.cc-connect/config.toml 里读取该 bot 的
#   app_id/app_secret（agent 不经手密钥），换 tenant_access_token（内存态、不落盘）。
# 设计动机: 避免所有 bot 共用 lark-cli 认证 store（会互踩），且发送者=bot 自己（在群即可发）。

CHAT_ID=${1:?usage: board-send <chat_id> <at_open_id|-> <text...>}
AT_ID=${2:?usage: board-send <chat_id> <at_open_id|-> <text...>}
shift 2
TEXT="$*"
[ -n "$TEXT" ] || { echo "ERROR: empty text" >&2; exit 1; }

PROJECT=${CC_PROJECT:?ERROR: CC_PROJECT not set (must run inside a cc-connect agent session)}
CFG="${CC_CONNECT_CONFIG:-$HOME/.cc-connect/config.toml}"

# 从 config.toml 提取本 project 的 feishu app_id/app_secret（第一个 feishu platform）
read -r APP_ID APP_SECRET < <(python3 - "$CFG" "$PROJECT" <<'PYEOF'
import re, sys
cfg, want = sys.argv[1], sys.argv[2]
text = open(cfg, encoding="utf-8").read()
# 切出 [[projects]] 块
blocks = re.split(r'(?m)^\[\[projects\]\]', text)[1:]
for b in blocks:
    m = re.search(r'(?m)^\s*name\s*=\s*"([^"]+)"', b)
    if not m or m.group(1) != want:
        continue
    aid = re.search(r'(?m)^\s*app_id\s*=\s*"([^"]+)"', b)
    sec = re.search(r'(?m)^\s*app_secret\s*=\s*"([^"]+)"', b)
    if aid and sec:
        print(aid.group(1), sec.group(1))
        sys.exit(0)
sys.exit(f"ERROR: project {want} feishu app credentials not found in {cfg}")
PYEOF
)

# 1) 换 tenant_access_token（不落盘）
TOKEN=$(curl -s -X POST "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal" \
  -H "Content-Type: application/json" \
  -d "{\"app_id\":\"$APP_ID\",\"app_secret\":\"$APP_SECRET\"}" | python3 -c 'import json,sys;d=json.load(sys.stdin);assert d.get("code")==0, d;print(d["tenant_access_token"])')

# 2) 组装 content（text 消息里的 <at user_id="ou_x"></at> 会触发对方 bot 事件）
if [ "$AT_ID" != "-" ]; then
  BODY_TEXT="<at user_id=\"$AT_ID\"></at> $TEXT"
else
  BODY_TEXT="$TEXT"
fi
CONTENT=$(python3 -c 'import json,sys;print(json.dumps({"text":sys.argv[1]},ensure_ascii=False))' "$BODY_TEXT")
PAYLOAD=$(python3 -c 'import json,sys;print(json.dumps({"receive_id":sys.argv[1],"msg_type":"text","content":sys.argv[2]},ensure_ascii=False))' "$CHAT_ID" "$CONTENT")

# 3) 发送
RESP=$(curl -s -X POST "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$PAYLOAD")
echo "$RESP" | python3 -c 'import json,sys
d=json.load(sys.stdin)
if d.get("code")==0:
    print("OK", d["data"]["message_id"])
else:
    print("SEND_FAILED", d.get("code"), d.get("msg"), file=sys.stderr); sys.exit(1)'
