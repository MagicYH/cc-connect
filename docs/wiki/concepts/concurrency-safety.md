# Concurrency Safety

The patterns used for thread-safe access to shared state across the codebase.

## Patterns

- `sync.RWMutex` for shared state on agent sessions and platform structs
- `defer mu.Unlock()` / `defer mu.RUnlock()` is standard
- `sync.Once` for one-time teardown (e.g. `closeOnce` in session close)
- `context.Context` as first argument to all blocking/cancellable methods
- Channel ownership documented; `Session.TryLock()`/`Session.Unlock()` for busy-state

## Project Relation

Agent sessions are accessed from multiple goroutines (engine event loop, permission handler, platform callbacks). The `I18n` struct also uses `sync.RWMutex` for concurrent translation lookups.

## Cross-References

- [agent-session-interface](../entities/agent-session-interface.md) — concurrency-safe session contract
- [i18n](../entities/i18n.md) — RWMutex-protected translations
