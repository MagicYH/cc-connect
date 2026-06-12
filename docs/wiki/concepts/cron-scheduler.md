# CronScheduler

Recurring job scheduler using `robfig/cron/v3`. Executes prompts or shell commands on cron schedules. Per-job session mode (reuse/new_per_run), silent/mute modes, permission overrides, exec timeout. Persists jobs via [CronStore](../entities/cron-store.md).

**Implementation:** `core/cron.go:423` (`NewCronScheduler`)
**Config:** `[[cron]]` array in `config.toml`

Cross-references: [TimerScheduler](timer-scheduler.md), [HookRunner](hook-runner.md)
