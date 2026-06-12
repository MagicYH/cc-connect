# Bypass Permissions

Auto-approve mode (alias "yolo") that resolves every permission request without human review. Enabled via `bypassPermissions` config or `ModeOverride` on cron/timer jobs.

Risks: combined with cron, allows autonomous unreviewed tool use (shell commands, file writes, network access). Prompt-based cron jobs (non-admin) can set this mode. Root bypass downgrade exists only for Claude Code agent, not others.

Project relation: `core/engine.go`, `core/cron.go`, `agent/claudecode/session.go`.

Cross-references: [timer-scheduler](../entities/timer-scheduler.md), [pending-permission](../entities/pending-permission.md)
