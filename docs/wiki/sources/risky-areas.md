# Risky Areas — Source Summary

[Raw scan](../raw/risky-areas.md)

## Concurrency Risks

**Mutex ordering** — `SessionManager.Save()` acquires `sm.mu.RLock` then `s.mu.Lock`; a writer holding `sm.mu.Lock` plus `s.mu.Lock` would deadlock. The ordering contract is implicit and undocumented. `PruneDuplicateSessions` acquires two `Session.mu` locks deterministically but a future reversal would deadlock.

**Fragile explicit unlock** — `processInteractiveMessageWith` (`core/engine.go:3196-3206`) calls `session.Unlock()` explicitly instead of deferring. Early-return paths that forget to unlock cause session lockups. This was the fix for commit 992b82e.

**TOCTOU gap** — `handlePendingPermission` releases `interactiveMu` before acquiring `state.mu`, allowing concurrent cleanup. Mitigated by nil checks and `sync.Once`, but the gap is a design smell.

**Channel leaks** — `agent/iflow` and `agent/antigravity` never close the events channel on `Close()` timeout, causing the engine event loop to block forever. Same bug class already fixed for kimi/pi/acp (commit 7c42f89).

**Codex deferred close** — `agent/codex/session.go:865-876` closes `cs.events` via a background goroutine after `Close()` returns, creating a send-on-closed-channel panic window.

**Untracked goroutines** — `core/bridge.go` dispatches untracked goroutines with no context propagation; cleanup ordering not guaranteed during shutdown.

**Unprotected `pendingMsgs`** — `agent/codex/session.go` uses a `[]string` with no mutex; safe only under single-goroutine ownership invariant.

## Authentication & Authorization Risks

**Permission callbacks matched by session key, not RequestID** — Tapping "Allow" on a stale card for tool A could approve a later request for tool B. Request-ID validation deferred.

**Web admin token sent via IM** — `/web setup` generates and sends an admin token through the chat platform. Anyone in the channel gains full admin access. **Severity: High.**

**Bridge token in URL query param** — `core/bridge.go:1271` reads the token from `r.URL.Query()`, leaking it into logs/proxy histories despite `subtle.ConstantTimeCompare`.

**Incomplete redaction** — `core/redact.go` sensitive-key list (`KEY`, `TOKEN`, `SECRET`, `PASSWORD`, `CREDENTIAL`) misses `AWS_ACCESS_KEY_ID`, `PRIVATE_KEY`, `AUTH`, `COOKIE`, `SESSION`, provider-specific keys. Error messages in qqbot/dingtalk include token response bodies.

**bypassPermissions + cron** — Scheduled tasks with `ModeOverride: "bypassPermissions"` execute any tool without human review. Prompt-based cron jobs (non-admin) can also set this mode.

**Root bypass only for Claude Code** — Other agents have no root-check downgrade for bypassPermissions.

**allow_from defaults to permit-all** — Empty `allowFrom` allows all users (fail-open), while `adminFrom` defaults to fail-closed.

**Bridge insecure mode** — No production guard prevents deploying with `insecure=true` and empty token.

**Feishu card action origin** — No validation that card action values originate from cc-connect-generated cards.

## Session Lifecycle Risks

**Session ID validation opt-in** — Agents not implementing `SessionIDValidator` can resume sessions from a different project sharing the same workspace. **Severity: High.**

**Platform disconnect leaves stale state** — `OnPlatformUnavailable` flips a boolean but does not clean up `interactiveState` or close agent sessions. Ghost messages or hangs on reconnection.

**Close timeout abandons process** — 130-second ceiling logs "abandoning" without waiting; orphaned subprocesses (especially MCP servers) survive.

**InteractiveState cleanup race** — `stopInteractiveSessionWithOptions` does not pass `expected` to `cleanupInteractiveState`, so a concurrent new turn can have its state destroyed.

## Process Management Risks

**Permission request hangs indefinitely** — If the platform disconnects after a permission prompt is sent, the event loop goroutine blocks on `<-pending.Resolved` forever. No automatic timeout. **Severity: High.**

**Detached grandchildren** — Process group kill misses daemon grandchildren started by MCP servers.

**No health monitoring between turns** — `unsolicitedReader` blocks on `Events()` forever if the agent process hangs without exiting. `eventIdleTimeout` only checked during foreground turns.

## Input Validation Risks

**Shell injection via cron/timer** — Commands passed verbatim to `sh -c`. Admin-only but unsanitized.

**Path traversal scope** — `resolveCustomCommand` correctly validates, but other file-read paths (`observer.go`, `reference_show.go`) lack traversal checks.

**Plain text "allow"** — Typing "allow" in a message could accidentally resolve a pending permission.

## Multi-Workspace & Timer Risks

**Workspace binding TOCTOU** — Multiple cc-connect instances sharing a bindings file can race between lookup and use.

**Auto-binding by convention** — Directory names matching channel names create implicit routing without confirmation.

**Timer outlives shutdown** — `time.AfterFunc` fires even after engine stop; `executeJob` completes unimpeded if already running.

**Timer timeout orphans session** — Timer marks job done but the agent session continues processing, causing duplicate output.

## Rate Limiting & Dedup

**Slash commands bypass queue** — `/shell` and admin commands create sessions without per-session busy-queue logic.

**Dedup TTL eviction O(n)** — `IsDuplicate` iterates the full map per call under write lock.

**Empty message IDs bypass dedup** — Platforms with unreliable ID generation cause duplicate processing.
