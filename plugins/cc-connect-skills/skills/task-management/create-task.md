# Create Task

Use this when the user asks to create a new task with a Feishu group and Bitable tracking.

## Inputs

Required:

- `workspace`: short workspace name such as `ws-dev-skills`, or an absolute path
- `task`: concise task title
- `content`: task details / requirements
- Bitable `app-token` and `table-id`

Optional:

- `group-name`: defaults to the task title, max 60 chars
- related document URLs to include in `content`

## Workflow

### 1. Create and bind the task group

**REQUIRED SUB-SKILL:** Use `cc-connect-skills:create-workspace-group`.

Pass the exact workspace from the user:

| Workspace input | Command sent by sub-skill |
|---|---|
| `ws-dev-skills` | `/workspace bind ws-dev-skills` |
| `/absolute/path` | `/workspace route /absolute/path` |
| not provided | `/workspace route /home/{user}/Project/Source/Bytedance` |

Save the returned `chat_id`. It is written to Bitable as the group chat field.

### 2. Summarize the task before writing

Write a compact Bitable `content` value before creating the record:

```text
workspace: <workspace>
任务内容：<one concise paragraph>
执行要点：
1. <step>
2. <step>
验收：<observable outcome>
```

Keep raw requirements in the chat if needed, but write the summarized version to Bitable.

### 3. Create the Bitable task record with the helper

The helper reads the field schema first, adapts to both known task table shapes, and writes only fields that exist.

Use the skill base directory printed when this skill loads:

```bash
SKILL_DIR="/home/chenhao.magic/Project/Source/Github/cc-connect/plugins/cc-connect-skills/skills/task-management"
python3 "$SKILL_DIR/create_task.py" \
  --app-token "$TASK_APP_TOKEN" \
  --table-id "tbll9qlspH9Ju0lE" \
  --workspace "ws-dev-skills" \
  --task "优化 wiki 相关技能" \
  --content "workspace: ws-dev-skills\n任务内容：优化 wiki 相关技能，录入前先查重；已有内容则更新正文和元信息，没有出现过再新增。\n执行要点：\n1. 检索现有 wiki 内容和关联关系。\n2. 更新已有条目的正文、时间、关联关系等元信息。\n3. 未命中时新增条目。\n验收：重复主题不会产生新条目，旧条目保持准确。" \
  --chat-id "oc_xxx"
```

Do not hardcode app tokens in the skill or script. Pass them as environment variables or CLI arguments.

### 4. Share related documents in the group

If the task has related documents, send them to the task group with `@team-leader` after creating the Bitable record. Team members need the document links before acting.

Use boss bot token and Feishu post format with an `at` tag; plain text mentions do not notify bots.

### 5. Progress tracking

Task creation does not create or modify the progress-check cron directly. For hourly progress tracking, load [progress-check.md](progress-check.md).

## Known Bitable Shapes

The helper supports these field names when present:

| Meaning | Field names |
|---|---|
| Task title | `Task`, `Task Name` |
| Details | `Relevant content`, `Description` |
| Workspace | `workspace` |
| Status | `Action` → `Not started`, or `Status` → `Queue` |
| Group chat | `Related Group`, `Task Group` |
| Create time | `Create Time`, `Create time` |

Group chat field format is always:

```json
[{"id":"oc_xxx"}]
```

## Common Mistakes

| Mistake | Fix |
|---|---|
| Guessing fields | Let `create_task.py` read field list first |
| Writing `chat_id` as a string | Use `[{'id': chat_id}]` via the helper |
| Creating a task without a group | Run `create-workspace-group` first unless the user gives an existing `chat_id` |
| Hardcoding app-token | Pass via env/argument only |
