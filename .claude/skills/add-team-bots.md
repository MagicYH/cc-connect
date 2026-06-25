---
name: add-team-bots
description: Add team bots from cc-connect config to the current Feishu group chat using lark-cli
---

Add all team bots from `~/.cc-connect/config.toml` into the current Feishu group chat.

## Prerequisites

- `lark-cli` installed and authenticated
- `CC_SESSION_KEY` environment variable available (set by cc-connect)
- `~/.cc-connect/config.toml` exists with feishu platform configs

## Steps

### 1. Extract chat_id from CC_SESSION_KEY

```bash
CHAT_ID=$(echo "$CC_SESSION_KEY" | cut -d: -f2)
```

The format is `feishu:<chat_id>:<user_id>`. If `CC_SESSION_KEY` is missing, ask the user for the chat_id.

### 2. Parse team bot app_ids from config

Use Python with `tomllib` to parse the config. Team membership is determined by shared `work_dir` — projects with the same working directory belong to the same team.

```bash
python3 << 'PYEOF'
import tomllib, os, json

config_path = os.environ.get('CC_DATA_DIR', os.path.expanduser('~/.cc-connect')) + '/config.toml'
with open(config_path, 'rb') as f:
    cfg = tomllib.load(f)

current = os.environ.get('CC_PROJECT', '')

# Find current project's work_dir
current_workdir = None
for p in cfg.get('projects', []):
    if p.get('name') == current:
        current_workdir = p.get('agent', {}).get('options', {}).get('work_dir', '')
        break

if not current_workdir:
    print(json.dumps({'error': f'Project {current!r} not found or has no work_dir'}))
    exit(1)

# Find team members: projects sharing the same work_dir
app_ids = []
team_members = []
for p in cfg.get('projects', []):
    workdir = p.get('agent', {}).get('options', {}).get('work_dir', '')
    if workdir == current_workdir:
        name = p.get('name', '')
        for plat in p.get('platforms', []):
            if plat.get('type') == 'feishu':
                aid = plat.get('options', {}).get('app_id', '')
                if aid:
                    app_ids.append(aid)
                    team_members.append({'name': name, 'app_id': aid})

print(json.dumps({'team_members': team_members, 'app_ids': app_ids}))
PYEOF
```

### 3. Add bots to the group chat

Use `lark-cli im chat.members create` with `member_id_type=app_id` and `succeed_type=1` (skip unavailable IDs gracefully).

**Important**: The calling bot must already be in the target chat. Use `--as bot`.

```bash
CHAT_ID=$(echo "$CC_SESSION_KEY" | cut -d: -f2)
# APP_IDS from step 2, e.g. ["cli_a956ed42217a5cd2","cli_aa99ba9120389cdb","cli_aa92ac4655fb5cdd","cli_a95db9947e22dcc4"]

lark-cli im chat.members create "$CHAT_ID" \
  --as bot \
  --params '{"member_id_type":"app_id","succeed_type":1}' \
  --data "{\"id_list\":[\"cli_xxx\",\"cli_yyy\"]}"
```

API limit: max 5 bots per request. If more than 5, split into batches.

### 4. Report results

After execution, report:
- Which bots were added successfully
- Which bots failed (already in group, not available, etc.)
- The chat_id for reference

## Error Handling

- `CC_SESSION_KEY` missing: ask user for chat_id
- Current project not in a team (unique `work_dir`): only its own bot will be found, which is already in the group. Inform the user.
- lark-cli errors for specific app_ids: report them, don't fail the whole operation
- Calling bot not in the chat: suggest adding at least one bot to the group first, then retry

## Example

User says: "把 team 的 bot 都拉到群里"

1. Extract `oc_4aa3e6499fbe690ce2f98e8b36b734a4` from `CC_SESSION_KEY`
2. Find `cc-connect` project's `work_dir`, find projects sharing the same `work_dir`
3. Collect their feishu `app_id`s
4. Call `lark-cli im chat.members create` to add them to the group
