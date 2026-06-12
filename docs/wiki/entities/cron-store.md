# CronStore

Persists cron job definitions to `<data_dir>/crons/crons.json`. Survives restarts. Used by [CronScheduler](cron-scheduler.md).

**Implementation:** `core/cron.go` (`CronStore`, `CronScheduler`)
**Initialized:** `core/cron.go:123`

Cross-references: [TimerStore](timer-store.md), [HookRunner](hook-runner.md)
