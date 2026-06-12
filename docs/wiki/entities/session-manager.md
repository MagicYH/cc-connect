# SessionManager

Manages active session mappings (session_key -> session_id) per project and work_dir. Persists to `sessions.json` under the data directory.

**Implementation:** `core/session.go`
**Config:** `data_dir` in `config.toml`

Cross-references: [ProjectStateStore](project-state-store.md), [CronScheduler](cron-scheduler.md), [Mutex Ordering](../concepts/mutex-ordering.md), [InteractiveState](interactive-state.md), [PendingPermission](pending-permission.md), [agent-session-interface](agent-session-interface.md)

## Code Conventions

Created via `NewSessionManager(...)`. Provides `Session.TryLock()` / `Session.Unlock()` for session busy-state. Tracks active `AgentSession` instances and handles lifecycle (create, switch, delete, list).

## Risky Areas

- `Save()` acquires `sm.mu.RLock` then `s.mu.Lock`; any writer holding `sm.mu.Lock` plus `s.mu.Lock` would deadlock against it. Lock ordering contract is implicit.
- `PruneDuplicateSessions` acquires two `Session.mu` locks deterministically (keep then old). A future reversal would deadlock.
- Root bypass downgrade for `bypassPermissions` exists only for Claude Code agent, not others.
