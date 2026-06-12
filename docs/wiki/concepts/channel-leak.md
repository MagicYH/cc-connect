# Channel Leak

Bug class where an agent session's events channel is never closed on `Close()` timeout, causing the engine event loop to block on `<-s.events` forever. Already fixed for kimi/pi/acp (commit 7c42f89) but still present in iflow and antigravity.

Codex variant: events channel closed via deferred background goroutine, creating a send-on-closed-channel panic window after `Close()` returns.

Project relation: `agent/iflow/session.go`, `agent/antigravity/session.go`, `agent/codex/session.go`.

Cross-references: [session-manager](../entities/session-manager.md)
