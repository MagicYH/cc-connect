# Progress Check

Use this when setting up hourly progress tracking, sending progress-check messages, or updating task progress after team-leader replies.

## Set Up Hourly Progress Tracking

**Before creating a cron, always check if one already exists:**

```bash
cc-connect cron list
```

Look for a job with a description matching "Task progress check" or similar. If one already exists, do NOT create a duplicate.

**Only if no existing cron, create one:**

```bash
cc-connect cron add --cron "7 * * * *" \
  --prompt "1) Read the Bitable main task table (app-token: <APP_TOKEN>, table-id: <MAIN_TABLE_ID>) and filter for records where Status is NOT Finished (Queue, Running, or Pause). 2) For each unfinished task, find its Task Group (chat_id), then send a Feishu post (rich text) message with @team-leader asking for progress. MUST use post format with at tag — plain text @mention does NOT work for bots. Tell team-leader in the message: 'This message is from Boss. You MUST @Boss in your reply — Boss is a bot and cannot receive messages without @mention.' Include task name and current Bitable status, and ask team-leader to reply in format: a) task xxx progress yyy, update status to Running; b) task xxx completed, update status to Finished. 3) After receiving a response from team-leader, update the Bitable record's Status and Update Time fields. Subtask table for subtask updates: <SUBTASK_TABLE_ID>." \
  --desc "Task progress check"
```

**The cron prompt MUST include**:
- Bitable app-token and both table IDs (main + subtask)
- The instruction to read Bitable and filter for unfinished tasks
- The requirement to use post format with `at` tag for @mention
- The instruction to tell team-leader the message is from Boss and must @Boss in reply (Boss is a bot, cannot receive messages without @mention)
- The instruction to update Status and Update Time after receiving replies

**The cron prompt must NOT include**:
- Hard-coded chat_ids (they come from Bitable at runtime)
- Hard-coded record_ids (they come from Bitable at runtime)

## Send Progress Check Messages

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

## Update Progress in Bitable

After receiving progress update from team-leader:

```bash
bytedcli --json feishu bitable record update \
  --app-token "<APP_TOKEN>" \
  --table-id "<TABLE_ID>" \
  --record-id "<RECORD_ID>" \
  --body-json "{\"fields\":{\"Status\":\"Running\",\"Update Time\":$(date +%s)000}}" 2>/dev/null
```
