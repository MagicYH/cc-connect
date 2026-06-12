# Risky Areas

## Concurrency

### Mutex ordering and dual-lock acquisition

`PruneDuplicateSessions` (`core/session.go:872-884`) acquires `sm.mu.Lock()`, then iterates sessions acquiring `keep.mu.Lock()` and `old.mu.Lock()` simultaneously. Lock ordering is deterministic (keep first, old second) and no other code path acquires two `Session.mu` locks simultaneously, so no deadlock exists today. However, if a future change adds dual-session locking in reverse order, deadlock would result.

`SessionManager.Save()` (`core/session.go:594-625`) acquires `sm.mu.RLock` then `s.mu.Lock`. Any writer holding `sm.mu.Lock()` that also tried to acquire `s.mu.Lock()` would deadlock against a concurrent `Save()`. No such writer exists today, but the lock ordering contract is implicit and undocumented.

### Fragile explicit unlock in processInteractiveMessageWith

`core/engine.go:3196-3206` -- `session.Unlock()` is NOT deferred; it is called explicitly in the drain loop while holding `state.mu` to close a race window between "queue is empty" and "session unlocked". A fallback `defer` covers early-return paths. This was the fix for commit 992b82e ("session busy lockup"). Any future early-return path that forgets to unlock will cause a session lockup. **Severity: Medium** -- fragile, easily broken by future changes.

### TOCTOU gap in handlePendingPermission

`core/engine.go:2966-2987` -- `handlePendingPermission` first locks `e.interactiveMu`, reads the state, unlocks, then separately locks `state.mu`. The gap between releasing `interactiveMu` and acquiring `state.mu` means the state could be cleaned up by a concurrent goroutine. Handled by nil checks and `sync.Once` on `pending.resolve()`, but the TOCTOU gap is a design smell. **Severity: Low**.

### Channel leak on agent session Close timeout

`agent/iflow/session.go:898-903` and `agent/antigravity/session.go:468-473` -- when `wg.Wait()` times out during `Close()`, the events channel is never closed. The consumer (engine event loop) may block forever on `<-s.events`. The same class of bug was fixed for kimi, pi, and acp (commit 7c42f89) but iflow and antigravity still have the leak. **Severity: Medium**.

### Codex deferred-close goroutine window

`agent/codex/session.go:865-876` -- on timeout, `cs.events` is closed via a background goroutine (`go func() { <-done; cs.closeOnce.Do(...) }()`), creating a window where the events channel is still writable after `Close()` returns. If the engine sends after `Close()` returns but before the goroutine finishes, a send-on-closed-channel panic could occur. **Severity: Medium**.

### Bridge fire-and-forget goroutines

`core/bridge.go:1006,1028` -- `dispatchAsMessage` and `dispatchAsPermissionResponse` launch untracked goroutines (`go ref.platform.handler(...)`) with no context propagation. If the engine is shutting down, these goroutines may attempt to call `handleMessage` on a stopped engine. No data corruption risk, but cleanup ordering is not guaranteed. **Severity: Low**.

### Unprotected codexSession.pendingMsgs

`agent/codex/session.go:46,329,389-415,489` -- `pendingMsgs` is a `[]string` with no mutex protection. Safe today because only the `readLoop` goroutine accesses it, but the single-goroutine ownership invariant is not enforced by the type system. **Severity: Low**.

## Authentication & Authorization

### Permission callbacks matched by session key, not RequestID

`core/engine.go:2961-3095` -- When a user taps "Allow"/"Deny" on a platform card, `handlePendingPermission` resolves whichever permission request is currently pending for that session key, regardless of whether the callback corresponds to a different permission prompt. A user tapping "Allow" on a stale card for tool A could inadvertently approve a later permission request for tool B. Commit e166ca4 acknowledges this: "Request-ID validation is deferred as a follow-up." **Severity: Medium**.

### Web admin token sent via IM chat in plaintext

`core/engine.go:15556`, `core/i18n.go:3066-3077` -- The `/web setup` command generates a management API token and sends it via the IM platform reply. The token grants full control over project settings (admin_from, disabled_commands, mode, agent type). Any user in the chat channel can see it and gain admin access. **Severity: High**.

### Bridge token accepted via URL query parameter

`core/bridge.go:1271-1272` -- `authenticate` reads the token from `r.URL.Query().Get("token")`. URL query parameters are logged in web server access logs, proxy logs, and browser history. While the comparison uses `subtle.ConstantTimeCompare`, the transport mechanism leaks the token into logs. **Severity: Medium**.

### Token response bodies in error messages not redacted

`platform/qqbot/qqbot.go:582` -- Token response body included verbatim: `fmt.Errorf("token request returned %d: %s", resp.StatusCode, raw)`. `platform/dingtalk/dingtalk.go:574` -- error chain could include access tokens. `core/redact.go` provides `RedactEnv`/`RedactArgs` for CLI arguments but does not cover HTTP API response bodies from OAuth/token endpoints. **Severity: Medium**.

### RedactEnv sensitive-key list is incomplete

`core/redact.go:8-10` -- `sensitiveKeys` is `["KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL"]`. Misses `AWS_ACCESS_KEY_ID`, `PRIVATE_KEY`, `AUTH`, `COOKIE`, `SESSION`, or provider-specific keys like `DASHSCOPE_API_KEY`. Any env var not matching those five substrings is logged in cleartext. **Severity: Low-Medium**.

### bypassPermissions + cron allows autonomous unreviewed tool use

`core/engine.go:4748-4755`, `core/cron.go:105` -- The `bypassPermissions` mode (alias "yolo") auto-approves every permission request. Combined with cron/timer jobs that set `ModeOverride: "bypassPermissions"`, scheduled tasks can execute any tool (shell commands, file writes, network access) without human review. This is intentional but creates risk for misconfigured cron jobs. **Severity: Medium**.

### Non-admin prompt-based cron jobs can use bypassPermissions mode

`core/engine.go:12672`, `core/cron.go:105` -- `cmdCronAddExec` (shell-based cron) requires `isAdmin`, but `cmdCronAdd` (prompt-based cron) does not check admin privileges. Both allow setting `Mode: "bypassPermissions"`. A non-admin user with access to the management API could create a prompt-based cron job that runs in bypassPermissions mode. **Severity: Medium**.

### Root bypass downgrade only for Claude Code, not all agents

`agent/claudecode/session.go:96-102` -- When running as root (EUID 0), Claude Code downgrades `bypassPermissions` to "auto" mode. Other agents (Copilot, Gemini, Kimi, etc.) have no root check. If cc-connect runs as root with a non-Claude-Code agent in YOLO mode, the bypass flag passes directly. **Severity: Low-Medium**.

### allow_from defaults to permit-all

`core/message.go:39-44` -- `AllowList` returns `true` when `allowFrom` is empty or `"*"`, meaning all users are permitted by default. In contrast, `isAdmin` returns `false` when `adminFrom` is empty (fail-closed). A fresh install without explicit `allow_from` configuration is open to all users. **Severity: Medium**.

### Bridge insecure mode has no production guard

`core/bridge.go:159-184` -- `NewBridgeServerInsecure` creates a bridge server that allows all requests without a token when `bs.insecure` is true and `bs.token` is empty. There is no mechanism to prevent production use of this mode. A configuration error setting `insecure=true` without a token exposes the bridge fully. **Severity: Medium**.

### Bridge web-admin uses single shared identity

`core/bridge.go:1000-1028` -- `dispatchAsPermissionResponse` and `dispatchAsMessage` set `UserID: "web-admin"` and `UserName: "Web Admin"` on all synthesized messages. Any bridge user who knows the token can send permission responses on behalf of "web-admin" with no per-user authorization. **Severity: Low-Medium**.

### Feishu card action values not validated for origin

`platform/feishu/feishu.go:660-688` -- Card action values (`perm:allow`, `perm:deny`, etc.) are parsed from `event.Event.Action.Value` with prefix checks only. There is no validation that the action value originates from a card that cc-connect generated. A crafted card action from a malicious Feishu app integration could synthesize a `perm:allow` callback. **Severity: Low-Medium**.

## Session Lifecycle

### Session ID validation is opt-in per agent (cross-project leak)

`core/interfaces.go:196-210`, `core/engine.go:3568-3576` -- The `SessionIDValidator` interface is optional. If an agent does not implement it, the engine proceeds to `--resume` with whatever `AgentSessionID` is stored, which could belong to a different project sharing the same workspace directory. Cross-project session leakage means a user could resume a conversation from a completely different project, potentially exposing sensitive data or executing actions in the wrong context. **Severity: High**.

### Platform disconnect does not tear down active sessions

`core/engine.go:2124-2172` -- `OnPlatformUnavailable` only flips a boolean flag (`platformReady[p] = false`). It does not clean up `interactiveState` entries, close agent sessions, or notify running turns. If a platform disconnects mid-session (e.g., Feishu WebSocket drops), the agent process keeps running. On reconnection, stale state in `interactiveStates` can cause ghost messages or hangs. **Severity: Medium**.

### Agent session close timeout abandons the process

`core/engine.go:3746-3777` -- When `closeAgentSessionWithTimeout` hits its 130-second ceiling, it logs "agent session close timed out, abandoning" and returns without waiting for the agent process to exit. The agent subprocess could remain alive (especially MCP server grandchildren), as acknowledged in `agent/claudecode/session.go:958`. **Severity: Medium**.

### InteractiveState cleanup races with new turn creation

`core/engine.go:3679-3737`, line 9341 -- `stopInteractiveSessionWithOptions` does NOT pass the `expected` parameter to `cleanupInteractiveState`, meaning a concurrent new turn that replaced the state between the delete and the agent-session close could have its state destroyed. The agent close goroutine runs concurrently with no coordination with the new turn's agent session. **Severity: Medium**.

## Process Management

### Permission request hangs indefinitely on platform disconnect

`core/engine.go:4781-4812` -- When a `control_request` (permission prompt) arrives, the engine creates a `pendingPermission` struct and blocks on `<-pending.Resolved`. The resolution happens when the user responds through the platform. If the platform disconnects between the permission prompt being sent and the user responding, the resolution never happens, and the event loop goroutine hangs forever. `cleanupInteractiveState` resolves pending permissions, but only when explicitly called (via `/stop` or idle reset). There is no automatic timeout for permission requests. **Severity: High**.

### Process group kill may miss detached grandchildren

`agent/claudecode/session.go:917-965`, `agent/codex/session.go:835-878` -- Both `Close()` methods attempt to kill the process group, but if a grandchild process detaches from the process group (e.g., a daemon started by an MCP server), it survives the group kill. The comment at lines 957-958 of `agent/claudecode/session.go` explicitly acknowledges this. **Severity: Medium**.

### No process health monitoring between turns

`agent/claudecode/session.go:279-346`, `core/engine.go:3862-3878` -- Between user turns, the `unsolicitedReader` goroutine reads from the agent's Events channel. If the agent process enters a hung state without exiting, the unsolicitedReader blocks on `Events()` forever. The `eventIdleTimeout` (default 2 hours) is checked only during foreground turn processing, not during the unsolicited reader phase. A hung agent process between turns wastes resources and causes stale state until the next user message or the 2-hour idle timeout. **Severity: Medium**.

## Input Validation

### Shell injection via cron/timer exec commands

`core/engine.go:12672-12698` (cmdCronAddExec), `core/engine.go:1598-1603` (executeTimerShell), `core/engine.go:6995-7007` (shellExecCommand) -- Shell commands from cron/timer jobs are passed directly to `sh -c <command>` without sanitization. While `cmdCronAddExec` requires admin privileges and this is intentional behavior, the command string is executed verbatim. **Severity: Medium** (admin-only, intentional).

### Feishu card action values not validated for origin

See Authentication & Authorization section above. Action values are parsed by prefix only with no origin validation. **Severity: Low-Medium**.

### Path traversal protection correct but narrow in scope

`core/command.go:107-116` -- `resolveCustomCommand` correctly validates resolved file paths stay within the agent directory using `strings.HasPrefix(absPath, absDir+string(filepath.Separator))`. However, other file read paths in the codebase (`core/observer.go:201`, `core/reference_show.go:177`) open files by internally-generated paths without traversal checks. **Severity: Low** -- user-facing path is properly protected.

### Plain text "allow" could false-positive resolve permissions

`core/engine.go:2977-2995` -- If a user types "allow" or "deny" as a plain text message (without `IsPermissionResponse`), it falls through to the normal handler and could resolve a pending permission. A user typing "allow me to explain" could accidentally approve a permission. **Severity: Low**.

## Multi-Workspace State

### Workspace binding file re-reads on every operation (TOCTOU risk)

`core/workspace_binding.go:193-229` -- `refreshLocked()` re-reads the bindings file from disk if the file's mtime or size has changed. Between `Lookup` returning a workspace and the engine using it, another process could `Unbind` it. In single-daemon deployments this is not an issue, but multiple cc-connect instances sharing the same bindings file could race. **Severity: Medium**.

### Auto-binding by convention creates implicit routing

`core/engine.go:15196-15205` -- If a directory named after the channel exists under `baseDir`, the engine auto-binds it without user confirmation. A directory name colliding with a channel name could route all messages for that channel to the wrong workspace. **Severity: Medium**.

### Workspace idle reaper can kill during turn-start window

`core/workspace_state.go:139-158`, `core/engine.go:607-641` -- `ReapIdle` skips workspaces with `activeTurns > 0`, but `activeTurns` is only incremented by `BeginTurn` called at line 3248. There is a window between workspace resolution (line 2598) and `BeginTurn` (line 3248) where the turn is conceptually active but not yet counted. **Severity: Low** -- window is microseconds, reaper runs every 60 seconds.

## Timer & Delayed Tasks

### Timer job executes after engine shutdown

`core/timer.go:383-435` -- `scheduleAt` uses `time.AfterFunc`, which fires in a new goroutine. If the engine has been stopped between scheduling and firing, `executeJob` still attempts to run. `TimerScheduler.Stop()` stops all timers, but if `executeJob` is already running when `Stop` is called, it completes unimpeded. Timer jobs can outlive the engine shutdown, potentially sending messages to a platform that is also shutting down. **Severity: Medium**.

### Timer job timeout leaves orphaned agent session

`core/timer.go:416-423` -- When a timer job times out, `executeJob` marks the job as fired with a timeout error. But the `engine.ExecuteTimerJob` goroutine continues running in the background. The agent session is still alive and processing, but the timer system considers it done, leading to orphaned processing and potential duplicate output. **Severity: Medium**.

## Dedup & Message Ordering

### Dedup TTL eviction is O(n) on every check

`core/dedup.go:24-44` -- `IsDuplicate` iterates the entire `seen` map to evict expired entries on every call, holding a write lock. Under high throughput, this is O(n) per message. **Severity: Low** -- dedup is per-platform-instance, TTL is only 60 seconds.

### Empty message IDs bypass dedup entirely

`core/dedup.go:25` -- If `msgID` is empty, `IsDuplicate` returns `false` (never a duplicate). Platforms with unreliable ID generation could cause duplicate processing. **Severity: Low** -- documented behavior.

### IsOldMessage grace period is only 2 seconds

`core/dedup.go:49-51` -- After a daemon restart, messages created within 2 seconds before startup are considered "old" and silently dropped. Clock skew or slow restarts could cause legitimate messages to be dropped. **Severity: Low**.

## Rate Limiting

### Slash commands bypass queuing rate-limit semantics

`core/engine.go:2618-2623,2632-2660` -- Commands like `/shell` or admin commands that invoke agent sessions bypass the message-queuing logic entirely. Rapid `/shell` commands each create a new interactive state and agent session without being subject to per-session busy-queue logic. **Severity: Medium**.

### Outgoing rate limiter can block indefinitely

`core/outgoing_ratelimit.go:101-132` -- The `Wait` method loops with timer-based waits until a token is available. If the context is not cancelled, this can block the calling goroutine for an unbounded time. **Severity: Low** -- by design, but callers must use proper contexts.

### UserID fallback to sessionKey is less strict

`core/engine.go:1040-1072` -- When `msg.UserID` is empty (anonymous users), the rate limiter falls back to `msg.SessionKey`, giving anonymous users separate rate buckets per session. A single user with multiple sessions could exceed the intended rate limit. **Severity: Low** -- documented limitation.

## Scan Warnings

- **Line numbers** in findings are approximate -- they reference the state of the code at the time of scanning and may drift with future commits.
- **Severity assessments** are based on code analysis, not runtime testing. Some Medium-severity findings (e.g., channel leaks on timeout) may be extremely rare in practice.
- **Session ID validation gap** (High severity) only affects agents that do not implement `SessionIDValidator`. Check which agents implement this interface to assess actual exposure.
- **Permission request hang** (High severity) requires a platform disconnect during an active permission prompt. The `/cancel` command and `reset_on_idle_mins` config provide partial mitigation.
- **Web admin token in chat** (High severity) is mitigated if the chat platform uses end-to-end encryption and the channel has restricted membership, but the design itself leaks the credential through the messaging layer.
- **iflow and antigravity channel leak** findings are the same class of bug that was already fixed for kimi/pi/acp (commit 7c42f89). These two agents may have been overlooked in that fix.
- The codebase has no traditional payment/order/refund/wallet/stock domain -- the risky areas identified are specific to this project's domain (messaging-AI bridge with permission flows and subprocess management).
