# Graceful Shutdown

Ordered teardown orchestrated in `cmd/cc-connect/main.go`: management server, bridge server, webhook server, heartbeat, timer, cron, API server, engines, log closer, instance lock. HTTP servers use 5s shutdown timeout. Agent sessions use two-phase stop (graceful + 120s timeout, then SIGTERM). Supports self-restart via `RestartCh` + `syscall.Exec`. All platforms use `context.WithCancel` for goroutine signaling.

Cross-references: [instance-lock](../entities/instance-lock.md), [session-lock](../entities/session-lock.md)
