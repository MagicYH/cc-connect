# Session Lock

Concurrency control for `core/session.go` `Session` struct. Uses `sync.Mutex` with `busy` boolean flag implementing `TryLock()` (non-blocking). 15+ call sites in `core/engine.go` prevent concurrent message processing on same session. Failed TryLock results in queuing or dropping. Also provides `CompareAndSetAgentSessionID()` for atomic session ID assignment.

Cross-references: [graceful-shutdown](../concepts/graceful-shutdown.md)
