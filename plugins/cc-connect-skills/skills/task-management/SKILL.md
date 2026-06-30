---
name: task-management
description: Use when creating a task that needs Feishu group chat plus Bitable tracking, or when setting up or checking hourly progress tracking via cc-connect cron
---

# Task Management

Manage tasks via Feishu group chat + Bitable, with hourly progress tracking.

## Prerequisites

- `bytedcli` installed and authenticated (`bytedcli feishu login`)
- `lark-cli` installed and authenticated
- `cc-connect` available for cron scheduling
- `CC_SESSION_KEY` env var available

## Workflow

### 1. Create Group and Add Members

**REQUIRED SUB-SKILL:** Use cc-connect-skills:create-workspace-group to create the Feishu group, add all team bots + user, and send workspace bind commands.

If the user does not specify a project directory, the default is `/home/{user}/Project/Source/Bytedance` (get `{user}` via `whoami`). This is passed as an absolute path, so the workspace command will be `/workspace route /home/{user}/Project/Source/Bytedance`.

After group creation, you have the `chat_id` for Bitable and cron setup.

### 2. Read Bitable Schema First

**ALWAYS read field list before writing.** Never guess field names or formats.

```bash
bytedcli --json feishu bitable field list --app-token <APP_TOKEN> --table-id <TABLE_ID> 2>/dev/null
```

Key field types and their write formats:

| Bitable Type | ui_type | Write format |
|---|---|---|
| Text (1) | Text | `"value"` |
| SingleSelect (3) | SingleSelect | `"option_name"` (exact string) |
| DateTime (5) | DateTime | `unix_timestamp_ms` (integer) |
| SingleLink (18) | SingleLink | `["<record_id>"]` (array of record_id strings) |
| GroupChat (23) | GroupChat | `[{"id":"<chat_id>"}]` (array of objects with `id` field) |

### 3. Write to Bitable

**Main task table**:

```bash
bytedcli --json feishu bitable record create \
  --app-token "<APP_TOKEN>" \
  --table-id "<MAIN_TABLE_ID>" \
  --body-json "{\"fields\":{\"Task Name\":\"<task_name>\",\"Description\":\"<desc>\",\"workspace\":\"<workspace>\",\"Task Group\":[{\"id\":\"<chat_id>\"}],\"Status\":\"Queue\",\"Create Time\":$(date +%s)000}}" 2>/dev/null
```

**Subtask table** (link to main task via `Main Task` field with record_id):

```bash
bytedcli --json feishu bitable record create \
  --app-token "<APP_TOKEN>" \
  --table-id "<SUBTASK_TABLE_ID>" \
  --body-json "{\"fields\":{\"Task Name\":\"<subtask_name>\",\"Status\":\"Queue\",\"Main Task\":[\"<main_record_id>\"],\"Create Time\":$(date +%s)000}}" 2>/dev/null
```

### 4. Share Related Documents in Group Chat

**After creating Bitable records, share all related documents (requirements, design docs, etc.) in the task group chat with @team-leader.** Team members cannot act on tasks without knowing what to build. @team-leader ensures they get notified.

Use boss bot token to send a post with `at` tag for team-leader, document link, and a brief summary:

```python
content = json.dumps({
    'zh_cn': {
        'title': '<task_name> - 需求文档',
        'content': [
            [{'tag': 'at', 'user_id': TEAM_LEADER_OPEN_ID}, {'tag': 'text', 'text': ' 请查收相关文档：'}],
            [{'tag': 'a', 'text': '文档链接', 'href': '<doc_url>'}],
            [{'tag': 'text', 'text': '\n\n需求概要：<brief_summary>'}]
        ]
    }
})
# Send via boss bot token
```

**If the task has a Related Documents field in Bitable, always send those links to the group with @team-leader.**

### 5. Set Up Hourly Progress Tracking

**Before creating a cron, always check if one already exists:**

**Before creating a cron, always check if one already exists:**

```bash
cc-connect cron list
```

Look for a job with a description matching "Task progress check" or similar. If one already exists, do NOT create a duplicate.

**Only if no existing cron, create one:**

```bash
cc-connect cron add --cron "7 * * * *" \
  --prompt "Execute the task-management progress tracking routine: 1) Read the Bitable main task table (app-token: <APP_TOKEN>, table-id: <MAIN_TABLE_ID>) and filter for records where Status is NOT Finished (i.e. Queue, Running, or Pause). 2) For each unfinished task, find its Task Group (chat_id), then in that group chat send a Feishu post (rich text) message with @team-leader asking for progress. MUST use post format with at tag — plain text @mention does NOT work for bots. 3) After receiving a response from team-leader, update the Bitable record's Status and Update Time fields accordingly. Bitable subtask table for subtask updates: <SUBTASK_TABLE_ID>." \
  --desc "Task progress check"
```

**The cron prompt MUST include**:
- Bitable app-token and both table IDs (main + subtask)
- The instruction to read Bitable and filter for unfinished tasks
- The requirement to use post format with `at` tag for @mention
- The instruction to update Status and Update Time after receiving replies

**The cron prompt must NOT include**:
- Hard-coded chat_ids (they come from Bitable at runtime)
- Hard-coded record_ids (they come from Bitable at runtime)

### 6. Send Progress Check Messages

**IMPORTANT**: Use the **boss bot** token to send progress messages, NOT the team-leader token. If team-leader sends the message, it's @mentioning itself — the notification won't trigger and the message may not be visible in the group.

```python
# Get boss bot tenant token
python3 -c "
import tomllib, os, json, urllib.request
config_path = os.environ.get('CC_DATA_DIR', os.path.expanduser('~/.cc-connect')) + '/config.toml'
with open(config_path, 'rb') as f:
    cfg = tomllib.load(f)
for p in cfg.get('projects', []):
    if p.get('name') == 'boss':
        for plat in p.get('platforms', []):
            if plat.get('type') == 'feishu':
                req = urllib.request.Request(
                    'https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal',
                    data=json.dumps({'app_id': plat['options']['app_id'], 'app_secret': plat['options']['app_secret']}).encode(),
                    headers={'Content-Type': 'application/json'}
                )
                with urllib.request.urlopen(req, timeout=10) as resp:
                    print(json.loads(resp.read())['tenant_access_token'])
"
```

Then send post (rich text) with `at` tag using the boss token:

```python
content = json.dumps({
    'zh_cn': {
        'title': '',
        'content': [[
            {'tag': 'at', 'user_id': TEAM_LEADER_OPEN_ID},
            {'tag': 'text', 'text': f' 请问任务「{task_name}」的最新进展如何？请回复当前状态（Queue/Running/Pause/Finished）。'}
        ]]
    }
})
body = json.dumps({
    'receive_id': chat_id,
    'msg_type': 'post',
    'content': content
})
# Use boss token for Authorization
req = urllib.request.Request(
    'https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id',
    data=body.encode(),
    headers={'Authorization': f'Bearer {boss_token}', 'Content-Type': 'application/json'}
)
```

### 7. Update Progress in Bitable

After receiving progress update from team-leader:

```bash
bytedcli --json feishu bitable record update \
  --app-token "<APP_TOKEN>" \
  --table-id "<TABLE_ID>" \
  --record-id "<RECORD_ID>" \
  --body-json "{\"fields\":{\"Status\":\"Running\",\"Update Time\":$(date +%s)000}}" 2>/dev/null
```

## Common Mistakes

| Mistake | Fix |
|---|---|
| Creating duplicate cron jobs | Always `cc-connect cron list` first; skip if already exists |
| Hard-coding chat_id in cron prompt | Read chat_ids from Bitable at runtime — tasks and groups change |
| Plain text @mention | **MUST use post format with `at` tag** — bots ignore plain text @ |
| Using app_id as open_id in @mention | `cli_xxx` ≠ `ou_xxx`. Get bot open_id via `lark-cli im chat.members bots` |
| Wrong SingleLink format | Use `["rec_xxx"]` (array of record_id strings), not `"rec_xxx"` or `{"record_ids":[...]}` |
| Wrong GroupChat format | Use `[{"id":"oc_xxx"}]` (array of objects with `id`), not `"oc_xxx"` or `[{"chat_id":"oc_xxx"}]` |
| Using CronCreate for Feishu cron | Use `cc-connect cron add` for recurring Feishu tasks |
| Guessing Bitable field names | Always `bitable field list` first |
| Not filtering for unfinished tasks | Only ask progress for Status != Finished |
| Using team-leader token to send progress messages | Use **boss bot** token — team-leader @itself won't trigger notification and message won't be visible |
| Not sharing related documents in group chat | After creating Bitable records, always send document links + summary to the group so team members know what to build |

## Field Reference

Default Bitable tables (may vary — always verify with `field list`):

**主任务表**: `tbluEzThmvfMShLx`
- Task Name (Text), Task Group (GroupChat → chat_id), Description (Text), workspace (Text), Related Documents (Text), Status (SingleSelect: Queue/Running/Finished/Pause), Create Time (DateTime), Update Time (DateTime)

**子任务表**: `tbly3SKBStBt2Drj`
- Same fields + Main Task (SingleLink → main record_id)
