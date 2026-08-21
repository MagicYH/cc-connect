---
name: simple-task-board
description: Use when Boss needs to start a lightweight Feishu/Lark work group for one task, record it in the Simple Tasks Bitable for lookup, and avoid cron reminders, watchdog polling, or the full task-board workflow.
---

# Simple Task Board

Start a one-off collaboration group: create a Feishu/Lark chat, include `team-leader` by default, include only explicitly requested optional roles, route everyone to the workspace, send the task to `team-leader`, then write one lookup row to the `Simple Tasks` Bitable table.

## When to use

Use this for requests like:

- “Boss 建一个群，让 team leader 和 reviewer 做这个任务”
- “指定任务目标和 workspace，拉 TL，其他角色按需加入”
- “不要轮询催办，只建群派任务，并把记录放到 Simple Tasks 表里方便以后找”

Do not use this for shared task-board rows, claim tokens, cross-role handoff tracking, watchdog, cron, or progress polling; use `task-board` for that.

## Command

From this skill directory:

```bash
scripts/simple-board-start.sh "<群名>" "<任务描述>" "<绝对工作目录>" "[可选角色列表]"
```

Examples:

```bash
scripts/simple-board-start.sh "修复登录问题" "修复登录失败并验证" "/data00/work/login-fix"
scripts/simple-board-start.sh "评审支付改动" "评审支付改动并输出风险结论" "/data00/work/pay" "reviewer"
scripts/simple-board-start.sh "多角色任务" "完成开发和评审" "/data00/work/feature" "developer,reviewer"
```

`team-leader` is always included. Optional roles may be comma- or space-separated and must exist in `~/.cc-connect/board.env` as matching `BOT_APPID_<role>` and `BOT_OPENID_<role>` variables. Role names accept hyphen or underscore.

## Requirements

- `CC_PROJECT` is set by cc-connect.
- `~/.cc-connect/board.env` defines at least:
  - `BOT_APPID_team_leader`
  - `BOT_OPENID_team_leader`
- Each optional role defines matching `BOT_APPID_*` and `BOT_OPENID_*`.
- The caller bot app ID is available as `BOT_APPID_<CC_PROJECT>` in `board.env` or as `app_id` in `~/.cc-connect/config.toml`.
- `SIMPLE_TASKS_BASE_TOKEN` and `SIMPLE_TASKS_TABLE_ID` are set in `board.env` or the environment.
- `lark-cli`, `jq`, `python3`, and `task-board/scripts/board-send.sh` are available.
- The caller has permission to write the configured `Simple Tasks` Bitable table.
- Workspace path is absolute, already exists, and does not contain apostrophes.

## Behavior contract

1. Validate task text, existing workspace, and role configuration.
2. Create a group with Boss/current bot when configured, `team-leader`, and requested optional roles only.
3. Send `/workspace route '<absolute-workspace>'` to every included role.
4. Send the task kickoff only to `team-leader`.
5. Write one `Simple Tasks` Bitable row with status `进行中`, the created group, task text, included roles, and initiator.
6. Print `SIMPLE_BOARD_READY <chat_id>` only after the Bitable row is written successfully.

## Non-goals

This skill intentionally does not:

- write the full task-board `Tasks` table or create claim/handoff rows
- create cron/timer jobs
- run watchdog polling
- claim/reclaim/heartbeat tasks
- route all known roles unless requested
- auto-dispatch work to reviewer/developer/tester

## Bitable lookup row

On success, the script writes one row to the `Simple Tasks` table:

| Field | Value |
|---|---|
| `主任务名` | group/task name argument |
| `项目状态` | `进行中` |
| `工作群` | created chat ID |
| `需求描述` | task description argument |
| `参与角色` | `team-leader` plus explicitly requested optional roles |
| `发起人` | `INITIATOR_OPENID`, falling back to `BOARD_WRITER_OPENID` |

If this write fails, the script exits non-zero and does not print `SIMPLE_BOARD_READY`. It does not start any timer, cron, watchdog, or polling process.

## Common mistakes

| Mistake | Fix |
|---|---|
| Using `task-board/scripts/board-init-project.sh` | That starts the full Bitable workflow and all fixed roles; use this skill instead. |
| Passing `reviewer` and expecting developer/tester too | Only requested optional roles are included. Add them explicitly. |
| Omitting workspace creation | Create or choose the workspace first; this skill only routes an existing path. |
| Sending the task to every role | The kickoff task goes only to `team-leader`; other roles wait in the group. |
