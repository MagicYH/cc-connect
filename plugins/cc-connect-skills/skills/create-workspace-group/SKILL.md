---
name: create-workspace-group
description: Use when creating a new Feishu group chat for a work task, adding team bots to it, and binding them to a shared workspace directory
---

# Create Workspace Group

Create a Feishu group chat for a specific task, add team bots, and bind each bot to the correct workspace via `/workspace` commands.

## Prerequisites

- `lark-cli` installed and authenticated
- `CC_SESSION_KEY` environment variable available (set by cc-connect)
- `~/.cc-connect/config.toml` exists with feishu platform configs
- Project configured with `mode = "multi-workspace"` and `base_dir` set

## Required Input

Ask the user for these if not provided:

| Input | Required | Example |
|-------|----------|---------|
| Group name | Yes | "Feature X Development" |
| Project directory | Yes | "my-project" or "/absolute/path/to/project" |

**Group name**: Derive from the task description if the user doesn't specify one. Keep it concise (max 60 chars).

**Project directory**: This determines which `/workspace` command to use:
- Relative path (e.g. `my-project`) → `/workspace bind my-project` (resolves to `base_dir/my-project`)
- Absolute path starting with `/` (e.g. `/opt/some/project`) → `/workspace route /opt/some/project`

## Steps

### 1. Create the Feishu group

```bash
lark-cli im +chat-create --name "GROUP_NAME" --as bot --set-bot-manager
```

This creates a private group with the bot as manager. Add `--type public` for a public group if needed.

The response contains the new `chat_id`. Save it — all subsequent steps need it.

### 2. Parse team bots from config

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

### 3. Add ALL bots to the group — including the current project's own bot

**Critical**: You MUST add the current project's own bot to the group. Without it, the bot cannot send @mention messages (API returns error 230002 "Bot/User can NOT be out of the chat"). The current project's bot is the one whose `app_id` matches `CC_PROJECT` — do NOT exclude it.

**Option A**: Create the group with all bots from the start:

```bash
lark-cli im +chat-create --name "GROUP_NAME" --as bot --set-bot-manager --bots "cli_xxx,cli_yyy,cli_zzz"
```

**Option B**: Add bots to an existing group after creation:

```bash
lark-cli im chat.members create \
  --as user \
  --params "{\"chat_id\":\"$CHAT_ID\",\"member_id_type\":\"app_id\",\"succeed_type\":1}" \
  --data '{"id_list":["cli_xxx","cli_yyy","cli_zzz"]}'
```

Include ALL team bot app_ids — the current project's bot and all teammates. Use `--as user` (not `--as bot`) — bot scope often lacks `im:chat.members:write_only`. Max 5 bots per request; split if more.

### 4. Authenticate and get bot open_ids

Ensure the correct lark-cli profile is active, then get bot open_ids in the chat:

```bash
# Switch to current project's bot profile if needed
lark-cli profile use PROJECT_NAME

# Get bot open_ids in the new chat
lark-cli im chat.members bots "$CHAT_ID" --as bot --params "{\"chat_id\":\"$CHAT_ID\",\"member_id_type\":\"open_id\"}"
```

### 5. Send workspace commands to each bot

**Critical**: Plain text @mention does NOT trigger bot message receipt. Must use Feishu rich text (post) format with `at` tag.

Determine the correct workspace command based on the user-provided path:

```
Is the path absolute (starts with /)?
├── Yes → /workspace route /absolute/path/to/project
└── No  → /workspace bind relative-name
```

| Command | Syntax | When to use |
|---------|--------|-------------|
| `bind` | `/workspace bind <name>` | Relative path — resolves to `base_dir/<name>` |
| `route` | `/workspace route <path>` | Absolute path starting with `/` — binds directly |

Get a tenant access token and send the @mention message:

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

# Send rich text @mention with workspace command
python3 -c "
import json, urllib.request

token = '$TOKEN'
chat_id = '$CHAT_ID'
bot_open_id = 'BOT_OPEN_ID'

# Choose the right command:
#   message_text = ' /workspace bind my-project'       # for relative paths
#   message_text = ' /workspace route /opt/my-project'  # for absolute paths
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

Send one message per bot (each bot needs its own @mention).

### 6. Report results

After execution, report:
- Group name and chat_id created
- Which bots were added to the group
- Which bots received workspace commands and which command was sent
- Any failures and their error codes

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Using `/workspace bind` with absolute path | `bind` resolves relative to `base_dir`. Use `/workspace route` for absolute paths |
| Using `/workspace route` with relative path | `route` requires absolute path starting with `/`. Use `/workspace bind` for relative names |
| Plain text @mention | Bots ignore plain text @. Must use post (rich text) with `at` tag |
| Excluding current project's bot from the add request | You MUST add the current project's bot too — it needs to be in the group to send @mention messages. Error 230002 otherwise |
| Using `--as bot` for chat.members.create | Use `--as user` — bot scope often lacks `im:chat.members:write_only` |
| Using `open_id` field in `at` tag | Feishu requires `user_id` field (not `open_id`) in the `at` tag, even though the value is an open_id |
| Wrong lark-cli profile active | Check `lark-cli profile list`, switch with `lark-cli profile use NAME` |
| Forgetting `--set-bot-manager` on creation | Without this, bot can't manage the group it created |

## Error Handling

- `CC_SESSION_KEY` missing: ask user for chat_id
- `99991672` (scope not enabled): switch to `--as user` or login with `--domain im`
- `230002` (bot/user not in chat): add the current project's bot to the group first, then retry sending messages
- `232011` (operator not in chat): add calling bot to chat first, then retry
- lark-cli old version path param bug: pass `chat_id` via `--params` instead of positional arg
