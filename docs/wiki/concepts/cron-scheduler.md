# CronScheduler

Recurring job scheduler using `robfig/cron/v3`. Executes prompts or shell commands on cron schedules. Per-job session mode (reuse/new_per_run), silent/mute modes, permission overrides, exec timeout. Persists jobs via [CronStore](../entities/cron-store.md).

**Implementation:** `core/cron.go:423` (`NewCronScheduler`)
**Config:** `[[cron]]` array in `config.toml`

注意：`cron add` 必须显式 `--session-key`（`core/cron.go:96` 强制）；锚点会话决定 cron 跑在哪个 workspace——锚点选择的劫持陷阱见 [Cron Self-Check](multi-agent-collaboration/cron-self-check.md)。

Cross-references: [TimerScheduler](timer-scheduler.md), [HookRunner](hook-runner.md), [Cron Self-Check](multi-agent-collaboration/cron-self-check.md)
