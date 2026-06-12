# Timer Scheduler

One-shot delayed task system in `core/timer.go`. Uses `time.AfterFunc` for scheduling.

Risks: timer jobs execute even after engine shutdown (goroutine completes unimpeded); timer timeout marks job done but the agent session continues running, producing orphaned processing and potential duplicate output.

Cross-references: [bypass-permissions](../concepts/bypass-permissions.md), [session-manager](./session-manager.md)
