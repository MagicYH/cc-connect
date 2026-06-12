# HookRunner

Event-driven hook executor. Runs shell commands or HTTP POSTs on lifecycle events (`message.received`, `message.sent`, `session.started`, `session.ended`, `cron.triggered`, `timer.triggered`, `permission.requested`, `error`, `*`). Async by default with configurable timeout and `CC_HOOK_*` env vars.

**Implementation:** `core/hooks.go` (`HookRunner`)
**Config:** `[[hooks]]` array in `config.toml`

Cross-references: [CronScheduler](cron-scheduler.md), [TimerScheduler](timer-scheduler.md)
