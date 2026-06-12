# TimerStore

Persists one-shot delayed timer jobs to `<data_dir>/timers/timers.json`. Jobs auto-clean after firing. Used by [TimerScheduler](timer-scheduler.md).

**Implementation:** `core/timer.go` (`TimerStore`, `TimerScheduler`)
**Initialized:** `core/timer.go:97`

Cross-references: [CronStore](cron-store.md), [HookRunner](hook-runner.md)
