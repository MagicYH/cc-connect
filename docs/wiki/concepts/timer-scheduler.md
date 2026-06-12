# TimerScheduler

One-shot delayed task scheduler. Executes prompts or shell commands at a specific future time. Same features as CronScheduler; auto-cleanup after firing. Persists via [TimerStore](../entities/timer-store.md).

**Implementation:** `core/timer.go` (`TimerScheduler`)

Cross-references: [CronScheduler](cron-scheduler.md), [HookRunner](hook-runner.md)
