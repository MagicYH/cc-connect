# Mutex Ordering

The lock acquisition order across the codebase. `SessionManager.Save()` acquires `sm.mu.RLock` then `s.mu.Lock`; `PruneDuplicateSessions` acquires `sm.mu.Lock` then iterated session `mu` locks. The ordering contract is implicit — no documentation enforces it. A future change that reverses the order on any code path would deadlock.

Project relation: affects `core/session.go` (SessionManager), `core/engine.go` (interactiveMu + state.mu), and agent session close paths.

Cross-references: [session-manager](../entities/session-manager.md), [interactive-state](../entities/interactive-state.md)
