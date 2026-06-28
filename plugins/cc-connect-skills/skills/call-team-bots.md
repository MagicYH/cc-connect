---
name: call-team-bots
description: Use when adding team bots to a Feishu group chat, binding teammates to a workspace, or sending @mention messages to bots via lark-cli
---

# Call Team Bots

Add team bots to a Feishu group chat and direct them to bind or route to a workspace, using lark-cli with proper authentication and @mention formatting.

## Prerequisites

- `lark-cli` installed
- `CC_SESSION_KEY` environment variable available (set by cc-connect)
- `~/.cc-connect/config.toml` exists with feishu platform configs
- Bot app_id/app_secret available in config for token acquisition

## Required Input

**The user MUST provide the workspace name before proceeding.** If not provided, ask the user:
- "Which workspace should the teammates bind to?"

## Steps

### 1. Extract chat_id and parse team bots

```bash
CHAT_ID=$(echo "$CC_SESSION_KEY" | cut -d: -f2)
```

`CC_SESSION_KEY` format: `feishu:<chat_id>:<user_id>`. If missing, ask user for chat_id.

Parse team bot app_ids from `~/.cc-connect/config.toml`. Find all projects that share the same `base_dir` (multi-workspace mode) or `work_dir` (normal mode) AND same mode as the current project, **including the current project itself**.

```python
python3 << 'PYEOF'
import tomllib, os, json

config_path = os.environ.get('CC_DATA_DIR', os.path.expanduser('~/.cc-connect')) + '/config.toml'
with open(config_path, 'rb') as f:
    cfg = tomllib.load(f)

current = os.environ.get('CC_PROJECT', '')

current_dir = None
current_mode = None
for p in cfg.get('projects', []):
    if p.get('name') == current:
        current_mode = p.get('mode', '')
        if current_mode == 'multi-workspace':
            current_dir = p.get('base_dir', '')
        else:
            current_dir = p.get('agent', {}).get('options', {}).get('work_dir', '')
        break

if not current_dir:
    print(json.dumps({'error': f'Project {current!r} not found or has no workspace dir'}))
    exit(1)

app_ids = []
team_members = []
for p in cfg.get('projects', []):
    name = p.get('name', '')
    p_mode = p.get('mode', '')
    if p_mode == 'multi-workspace':
        p_dir = p.get('base_dir', '')
    else:
        p_dir = p.get('agent', {}).get('options', {}).get('work_dir', '')
    if p_dir == current_dir and p_mode == current_mode:
        for plat in p.get('platforms', []):
            if plat.get('type') == 'feishu':
                aid = plat.get('options', {}).get('app_id', '')
                if aid:
                    app_ids.append(aid)
                    team_members.append({'name': name, 'app_id': aid})

print(json.dumps({'team_members': team_members, 'app_ids': app_ids}))
PYEOF
```

### 2. Authenticate lark-cli

**Critical**: The default lark-cli profile may not be the current project's bot. Before calling chat member APIs, ensure the correct profile is active.

If the current project's bot is not the active profile:

```bash
# Add profile for current project's bot (read app_secret from config)
echo "APP_SECRET" | lark-cli profile add --name PROJECT_NAME --app-id APP_ID --app-secret-stdin --use

# Login with IM permissions using device auth flow
lark-cli auth login --domain im --recommend --no-wait --json
```

This returns a `verification_url` and `device_code`. Show the URL to the user (verbatim, in a code block). After user confirms authorization:

```bash
lark-cli auth login --device-code "DEVICE_CODE"
```

### 3. Add bots to the group chat

Use `lark-cli im chat.members create` with `--as user` (not `--as bot`, which often lacks `im:chat.members:write_only` scope). **Exclude the current project's own bot from the add request** — it is already in the chat.

```bash
# Use app_ids from step 1, minus the current project's own app_id
lark-cli im chat.members create \
  --as user \
  --params "{\"chat_id\":\"$CHAT_ID\",\"member_id_type\":\"app_id\",\"succeed_type\":1}" \
  --data '{"id_list":["cli_xxx","cli_yyy"]}'
```

`succeed_type=1` skips unavailable IDs gracefully. Max 5 bots per request; split if more.

### 4. Send @mention messages to bots in group chat

**Important**: Plain text messages with `@bot_name` do NOT trigger bot message receipt. Must use Feishu rich text (post) format with proper `at` tag.

First, get bot open_ids in the chat:

```bash
lark-cli im chat.members bots "$CHAT_ID" --as bot --params "{\"chat_id\":\"$CHAT_ID\",\"member_id_type\":\"open_id\"}"
```

Then send a post message with @mention for each bot:

```bash
TOKEN=$(python3 -c "
import tomllib, os, json, urllib.request
config_path = os.environ.get('CC_DATA_DIR', os.path.expanduser('~/.cc-connect')) + '/config.toml'
with open(config_path, 'rb') as f:
    cfg = tomllib.load(f)
for p in cfg.get('projects', []):
    if p.get('name') == 'CURRENT_PROJECT':
        for plat in p.get('platforms', []):
            if plat.get('type') == 'feishu':
                req = urllib.request.Request(
                    'https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal',
                    data=json.dumps({'app_id': plat['options']['app_id'], 'app_secret': plat['options']['app_secret']}).encode(),
                    headers={'Content-Type': 'application/json'}
                )
                with urllib.request.urlopen(req) as resp:
                    print(json.loads(resp.read())['tenant_access_token'])
")

# Build and send rich text message with @mention
python3 -c "
import json, urllib.request

token = '$TOKEN'
chat_id = '$CHAT_ID'
bot_open_id = 'BOT_OPEN_ID'
# Workspace commands (use the one the user specified):
#   /workspace bind WORKSPACE_NAME
#   /workspace route /absolute/path/to/project
message_text = ' /workspace bind WORKSPACE_NAME'

content = json.dumps({
    'zh_cn': {
        'title': '',
        'content': [[
            {'tag': 'at', 'user_id': bot_open_id},
            {'tag': 'text', 'text': message_text}
        ]]
    }
})

body = json.dumps({
    'receive_id': chat_id,
    'msg_type': 'post',
    'content': content
})

req = urllib.request.Request(
    'https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id',
    data=body.encode(),
    headers={'Authorization': f'Bearer {token}', 'Content-Type': 'application/json'}
)
with urllib.request.urlopen(req) as resp:
    print(json.loads(resp.read()).get('code', 'error'))
"
```

### 5. Report results

After execution, report:
- Which bots were added to the group
- Which bots received @mention messages
- Any failures and their error codes

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Using `--as bot` for chat.members.create | Use `--as user` — bot scope often lacks `im:chat.members:write_only` |
| Plain text @mention | Bots ignore plain text @. Must use post (rich text) with `at` tag |
| Wrong lark-cli profile active | Check `lark-cli profile list`, switch with `lark-cli profile use NAME` |
| Passing chat_id as positional arg in old lark-cli | Use `--params` with `chat_id` field instead |
| lark-cli path parameter errors (old versions) | Use `--params '{"chat_id":"..."}'` or upgrade lark-cli |

## Error Handling

- `CC_SESSION_KEY` missing: ask user for chat_id
- `99991672` (scope not enabled): switch to `--as user` or login with `--domain im`
- `232011` (operator not in chat): add calling bot to chat first, then retry
- `1063002` (permission denied on doc): user lacks manage permission on the doc
- `1063003` (invalid operation on doc): member already has the permission
- lark-cli old version path param bug: pass `chat_id` via `--params` instead of positional arg
