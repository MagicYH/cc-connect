---
name: task-management
description: Use when creating a task that needs Feishu group chat plus Bitable tracking, or when setting up or checking hourly progress tracking via cc-connect cron
---

# Task Management

Manage tasks via Feishu group chat + Bitable, with hourly progress tracking.

## Route First

Load only the workflow you need. The skill base directory is `/home/chenhao.magic/Project/Source/Github/cc-connect/plugins/cc-connect-skills/skills/task-management`; read the routed file from that directory before acting.

| User intent | Read next |
|---|---|
| Create a new task, create/bind a workspace group, or write the Bitable task record | [create-task.md](create-task.md) |
| Set up hourly progress tracking, send progress checks, or update progress from replies | [progress-check.md](progress-check.md) |

## Prerequisites

- `bytedcli` installed and authenticated (`bytedcli feishu login`)
- `lark-cli` installed and authenticated
- `cc-connect` available for cron scheduling
- `CC_SESSION_KEY` env var available

## Default Rule

Always read the Bitable field list before writing records. Never guess field names or write formats.

```bash
bytedcli --json feishu bitable field list --app-token <APP_TOKEN> --table-id <TABLE_ID> 2>/dev/null
```

## Common Mistakes

| Mistake | Fix |
|---|---|
| Loading both workflows by default | Read only the routed file for the requested operation |
| Guessing Bitable field names | Always run `bitable field list` first |
| Plain text @mention | Use Feishu post format with `at` tag |
| Using CronCreate for Feishu cron | Use `cc-connect cron add` for recurring Feishu tasks |
| Hard-coding chat_id in cron prompt | Read chat_ids from Bitable at runtime |
