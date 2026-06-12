# Engineering Constraints

## Error Handling

### Error Wrapping Convention

The project exclusively uses `fmt.Errorf("context: %w", err)` for error wrapping. No third-party error libraries (pkg/errors, etc.) are used. The `%w` verb preserves the error chain for `errors.Is`/`errors.As`. This pattern appears ~162 times in `agent/` and ~120 times in `core/` (non-test files).

Representative examples:
- `agent/copilot/jsonrpc.go`: `fmt.Errorf("marshal: %w", err)`
- `agent/codex/session.go`: `fmt.Errorf("codexSession: stdout pipe: %w", err)`
- `agent/acp/session.go`: `fmt.Errorf("acp: initialize: %w", err)`
- `platform/wecom/wecom.go`: WeCom API calls return `fmt.Errorf("wecom: <operation> failed: %d %s", result.ErrCode, result.ErrMsg)` (flat, no `%w` -- external API errors are not wrapped)

### Sentinel Errors

Standard Go sentinel errors created with `errors.New()`, used with `errors.Is()` for programmatic branching:

- `core/interfaces.go:19` -- `ErrNotSupported` (optional capability not implemented)
- `core/engine.go:65` -- `ErrAttachmentSendDisabled`
- `core/cron.go:51-52` -- `ErrCronJobNotFound`, `ErrCronProjectNotFound`
- `platform/wecom/websocket.go:47` -- `errWSAckTimeout`
- `platform/feishu/feishu.go:4507-4508` -- `errFeishuCardRateLimited`, `errFeishuCardTableLimit`

### Custom Error Types

- `platform/feishu/feishu.go:4511-4551` -- `feishuCardAPIError` struct with `API`, `Code`, `SubCode`, `Msg` fields. Implements `Error()` and `Is()` for `errors.Is` matching against sentinel errors. Factory `classifyFeishuCardAPIError()` parses sub-codes via regex.
- `agent/copilot/jsonrpc.go:37-46` -- `jsonRPCError` struct with `Code int`, `Message string`, `Data any`. Standard JSON-RPC 2.0 error codes.
- `agent/acp/rpc.go:20-30` -- `rpcErrPayload` struct with `Code int`, `Message string`. Used in newline-delimited JSON-RPC transport.
- `platform/telegram/telegram.go:73-90` -- `retryLoopError` wrapping a `retryCause` enum and inner `error`. Implements `Error()` and `Unwrap()` for chain traversal.

### Engine-Level Error-to-User Mapping

- `core/engine.go:4089-4096, 5313-5329` -- `agentErrorHandlers` table maps substring matches on error messages to i18n message keys. Currently one entry: `{"Session not found", MsgSessionNotFound}`. Unmatched errors use the `MsgError` template.

### Management API Error Responses

- `core/management.go:356-380` -- Unified JSON error envelope:
  - `mgmtJSON(w, status, data)` returns `{"ok": true, "data": ...}`
  - `mgmtError(w, status, msg)` returns `{"ok": false, "error": msg}`
  - `mgmtOK(w, msg)` returns `{"ok": true, "data": {"message": msg}}`

### Hook-Based Error Observability

- `core/hooks.go:16-27, 62-74` -- `HookEventError` ("error") event type. Engine emits `HookEvent{Event: HookEventError, Error: errMsg}` to configured shell or HTTP hook handlers for external error observability.

### Platform Lifecycle Error Recovery

- `core/engine.go:2120-2130` -- `AsyncRecoverablePlatform` interface: `OnPlatformReady(p)` and `OnPlatformUnavailable(p, err)`. Engine logs errors and marks platform as unavailable. No proactive user notification on platform failure; `cleanupInteractiveState` and `notifyDroppedQueuedMessages` handle informing users about dropped queued messages.

### Panic Handling

No global HTTP panic recovery middleware exists. Panic patterns:
- `agent/tmux/session.go:171` -- `defer func() { _ = recover() }()` silently recovers panics from channel send during session teardown.
- `core/cron.go:739`, `core/timer.go:440` -- `panic(fmt.Errorf(...))` for "should never happen" conditions (crypto/rand failure).
- `core/runas_check.go:68-69` -- Explicit "never panics" contract: `PreflightRunAsUser` accumulates errors into `result.Fatal`/`result.Warn` slices instead of returning or panicking.

## Performance Invariants

### Incoming Message Rate Limiting (Sliding Window)

- `core/ratelimit.go` -- Per-key sliding-window rate limiter with `sync.Mutex`-protected bucket map. Tracks message timestamps per key; rejects requests exceeding `maxMessages` within a configurable window. Background `cleanupLoop` (5-minute ticker) removes stale buckets. Graceful stop via `stopCh` channel.

### Outgoing Message Rate Limiting (Token Bucket)

- `core/outgoing_ratelimit.go` -- Per-platform token bucket rate limiter. Configurable `MaxPerSecond` with per-platform overrides. `Wait()` blocks until a token is available, respecting `context.Context` cancellation. Buckets lazily created; burst defaults to `ceil(MaxPerSecond)`.

### Role-Based Rate Limiting

- `core/user_roles.go:134-153` -- `UserRoleManager.AllowRate()` resolves user's role and checks the per-role rate limiter. Falls back to global limiter if no role-specific limit exists.

### Streaming Preview Throttling

- `core/streaming.go` -- `streamPreview` struct throttles UI updates via `IntervalMs` (default 1500ms) and `MinDeltaChars` (default 30 chars). Uses `time.AfterFunc` for deferred flush scheduling; `sync.Mutex` protects state. Max preview length capped at `MaxChars` (default 2000 chars).

### Platform-Specific Progress Throttling

- `core/interfaces.go:~280` -- `ProgressUpdateThrottler` optional interface exposes `ProgressUpdateInterval()` for rate-limiting progress edits (e.g. Discord's ~5 edits/5s per channel constraint).

### Message Deduplication

- `core/dedup.go` -- `MessageDedup` uses `sync.Mutex`-protected map of seen message IDs with 60-second TTL (`dedupTTL`). `IsOldMessage()` rejects messages from before process startup (2-second grace) to prevent replay after restart.
- `platform/discord/discord.go` -- Two `sync.Map` instances (`seenMsgs`, `seenInteractions`) for lock-free dedup.
- `platform/wecom/wecom.go` -- `msgDedup` field tracks recently processed `MsgId` values.

### Caching Patterns

- `core/provider_presets.go` -- `presetsCache` with 6-hour TTL, `sync.RWMutex` protection. Stale cache served on fetch failure as fallback.
- `core/runas.go` -- 30-second TTL (`verifyCacheTTL`) verification cache keyed by `runAsUser`. Caches successful `runAs` verification to avoid ~100ms cost per spawn.
- Multiple platforms use `sync.Map` for lock-free concurrent caching of user/chat/channel/group names: `platform/feishu/feishu.go`, `platform/slack/slack.go`, `platform/discord/discord.go`, `platform/line/line.go`, `platform/wecom/wecom.go`.

### Atomic File Writes

- `core/atomicwrite.go` -- `AtomicWriteFile()` writes to temp file in same directory, syncs, then renames over target. Cleanups on failure. Used by session persistence, cron store, timer store, relay bindings, and directory history.

### HTTP Client Configuration

- `core/httpclient.go` -- Global `http.Client` with 30-second timeout shared across platform adapters.

### Concurrency Safety

- `core/session.go` -- `Session` uses `sync.Mutex` with `busy` boolean flag implementing `TryLock()` (non-blocking). Used pervasively across `core/engine.go` (15+ call sites) to prevent concurrent message processing on same session. Failed TryLock results in queuing or dropping.
- `core/session.go` -- `CompareAndSetAgentSessionID()` atomically sets session ID only if currently empty or holding sentinel `ContinueSession` value.
- `core/session.go` -- `SessionManager` uses `sync.RWMutex` for maps; individual `Session` objects have own `sync.Mutex`. Deep-copy snapshots during `saveLocked()`.
- `core/engine.go` -- Multiple mutexes for different concerns: `interactiveMu`, `aliasMu`, `bannedMu`, `userRolesMu`, `platformLifecycleMu`, `replyFooterMu`, `resolveOnce`.
- `core/bridge.go` -- Each `bridgeAdapter` has `writeMu sync.Mutex` for WebSocket write serialization, and `previewMu sync.Mutex` for preview request tracking.

### sync.Once for Idempotent Cleanup

- `agent/codex/session.go`, `agent/antigravity/session.go`, `agent/tmux/session.go`, `agent/codex/appserver_session.go` -- `closeOnce sync.Once` ensures session `Close()` is idempotent.
- `core/providerproxy.go` -- `once sync.Once` ensures provider resolution happens exactly once.
- `platform/wps-xiezuo/wpsxiezuo.go` -- `stopOnce sync.Once` ensures stop logic runs once.

### WaitGroup for Goroutine Lifecycle

- 10+ agent session files use `sync.WaitGroup` to track in-flight goroutines for clean shutdown: `agent/claudecode/session.go`, `agent/codex/session.go`, `agent/gemini/session.go`, etc.
- `core/engine.go` -- `sync.WaitGroup` tracks stdout/stderr pipe reader goroutines during command execution.

### Atomic Values for Lock-Free State

- `agent/kimi/session.go`, `agent/iflow/session.go`, `agent/cursor/session.go`, `agent/qoder/session.go`, `agent/copilot/session.go`, `agent/pi/session.go` -- `atomic.Value` for session ID, `atomic.Bool` for alive/sentOnce/turnActive/autoApprove flags.
- `agent/acp/rpc.go` -- `atomic.Int64` for RPC next-ID.

### Context.Context for Cancellation

- All agent sessions create per-session `context.WithCancel` from parent context.
- Pervasive `context.WithTimeout` usage with notable timeout values:
  - Doctor checks: 5-10s (`core/doctor.go`)
  - Hook execution: configurable, default 60s (`core/hooks.go`)
  - Webhook shell exec: 5 minutes (`core/webhook.go`)
  - Relay: 120s default (`core/relay.go`)
  - Platform API calls: 5-30s (varies)
  - Bridge server shutdown: 5s (`core/bridge.go`)

## Security Invariants

### Token/Secret Redaction

- `core/redact.go:7-33` -- `RedactEnv()` masks env var values whose keys contain "KEY", "TOKEN", "SECRET", "PASSWORD", or "CREDENTIAL" (case-insensitive) with `"***"`. Used when logging agent environment variables.
- `core/redact.go:37-68` -- `RedactArgs()` redacts values after sensitive CLI flags (`--api-key`, `--api_key`, `--apikey`, `--token`, `--secret`, `--password`, `-k`). Handles both `--flag=value` and `--flag value` formats. Applied across all agent session launch code paths.
- `core/message.go:47-54` -- `RedactToken()` replaces all occurrences of a token string with `[REDACTED]`. Used by Slack platform for logging URLs containing bot tokens.
- `platform/feishu/feishu.go:5110-5113` -- `redactInlineSecrets()` uses regex to replace `key=value` and `Authorization: Bearer <token>` patterns with `[redacted]` in user-facing content.

### Timing-Safe Token Comparison

All token/auth comparisons use `crypto/subtle.ConstantTimeCompare` to prevent timing attacks:
- `core/management.go` -- Bearer token auth
- `core/bridge.go` -- Bridge WebSocket/HTTP token auth
- `core/webhook.go` -- Webhook token auth
- `platform/max/max.go` -- MAX platform webhook secret

### CORS and Origin Checking

- `core/management.go:340-354` -- `setCORS()` checks request `Origin` against configured `corsOrigins` list. Supports wildcard `"*"`. Sets `Access-Control-Allow-Origin` to request origin (not wildcard), plus standard CORS headers.
- `core/bridge.go:673-713` -- `checkOrigin()` validates WebSocket origin. Insecure mode allows all; if CORS origins configured, matches against them; otherwise same-host required.

### Encryption for Platform Callbacks

- `platform/wecom/wecom.go:737-749+` -- AES-256-CBC for callback message encryption/decryption. `decodeAESKey()` decodes 43-char Base64 EncodingAESKey to 32 bytes. `decrypt()` performs AES-256-CBC with PKCS#7 unpadding.
- `platform/wps-xiezuo/wpsxiezuo.go:519-524+` -- AES-256-CBC with MD5-derived key for event data decryption.
- `platform/weixin/media_inbound.go:74-120` -- CDN media download and decryption via `downloadAndDecryptCDN()`.

### Webhook Shell Command Timeout

- `core/webhook.go:257-333` -- 5-minute timeout (`webhookShellTimeout`) on webhook-triggered shell commands, enforced via `context.WithTimeout`.

### Random Token Generation

- `core/api.go` -- `GenerateToken(n)` uses `crypto/rand` for cryptographically secure token generation. Falls back to timestamp-based token only if `rand.Read` fails.

## Authentication

### Management API Bearer Token

- `core/management.go:310-338` -- Bearer token + query param fallback. Accepts `Authorization: Bearer <token>` header or `?token=` query parameter. Empty token allows all requests (unauthenticated mode). Constant-time comparison.

### Webhook Server Token

- `core/webhook.go:154-178` -- Multi-header token auth: (1) `Authorization: Bearer <token>`, (2) `X-Webhook-Token` header, (3) `?token=` query param. Empty token allows all.

### Bridge Server Token

- `core/bridge.go:1258-1275` -- Multi-header token auth: (1) `Authorization: Bearer <token>`, (2) `X-Bridge-Token` header, (3) `?token=` query param. Has `insecure` mode flag for local development. Refuses to start without token if insecure is false.

### WeCom SHA1 Signature Verification

- `platform/wecom/wecom.go:728-735` -- `verifySignature()` sorts token+timestamp+nonce+encrypt, joins, hashes with SHA1, compares to expected signature. Used for callback URL verification and incoming message validation.

### WPS Xiezuo HMAC-SHA256 Signature Verification

- `platform/wps-xiezuo/wpsxiezuo.go:509-517` -- `verifyEventSignature()` computes HMAC-SHA256 of content string using app secret, compares with `hmac.Equal` (constant-time). Also signs outgoing WebSocket requests with `X-Kso-Authorization` header.

### MAX Platform Webhook Secret

- `platform/max/max.go:292-304` -- Compares `X-Max-Bot-Api-Secret` header (or `?s=` query param fallback) against configured webhook secret using `subtle.ConstantTimeCompare`.

### Unix Socket API (File Permission-Based)

- `core/api.go:44-63` -- Local-only access via Unix domain socket (`api.sock`) with 0600 permissions. No token-based auth; security relies on filesystem permissions.

### Platform Bot Token/Credential Requirements

All platform adapters require credentials at init time and return errors if missing:
- Telegram: bot token (`platform/telegram/telegram.go:139`)
- Slack: bot_token + app_token (`platform/slack/slack.go:47-53`)
- Discord: token (`platform/discord/discord.go:72`)
- DingTalk: client_id + client_secret (`platform/dingtalk/dingtalk.go:102`)
- LINE: channel_secret + channel_token (`platform/line/line.go:49`)
- QQBot: app ID + secret, obtains access tokens from OAuth2 endpoint (`platform/qqbot/qqbot.go:28-48`)

## Authorization

### Admin Allowlist (admin_from)

- `core/engine.go:966-1011` -- Comma-separated admin user ID allowlist. `isAdmin()` checks userID against `adminFrom` for privileged commands (`/shell`, `/show`, `/dir`, `/restart`, `/upgrade`, `/web`, `/diff`). Empty `adminFrom` = all privileged commands denied (fail-closed). `"*"` = all users. Warning logged at startup if not set.

### User Allowlist (allow_from)

- `core/message.go:56-70` -- `AllowList()` checks userID against comma-separated `allow_from` list per platform. Empty or `"*"` means allow all (fail-open).
- `core/message.go:37-45` -- `CheckAllowFrom()` logs warning at platform initialization if `allow_from` is empty, alerting operator that all users are permitted.

### Role-Based Access Control (RBAC)

- `core/user_roles.go` -- `UserRoleManager` maps user IDs to roles. Each `UserRole` has `DisabledCmds` (disabled command IDs, supports `"*"` wildcard) and optional `RateLimitCfg`. Resolution order: explicit user ID match -> default role -> wildcard role. Configurable via management API (`PATCH /api/v1/projects/{name}/users`).

### Privileged Command Gating

- `core/engine.go:5722-5728` -- Commands in `privilegedCommands` map require admin status (via `isAdmin()`). Blocked commands return "admin required" reply. Both blocked and executed commands logged with `slog.Info("audit: ...")`.

### Disabled Commands

- `core/engine.go:916-955` -- `resolveDisabledCmds()` resolves command names to a set. `"*"` disables all builtins. Checked at dispatch time; blocked commands result in "command disabled" reply.

### Banned Words Content Filter

- `core/engine.go:1013-1022, 2192-2200, 2552-2558` -- `SetBannedWords()` configures lowercased banned words. `matchBannedWord()` checks message content (case-insensitive). Messages containing banned words are blocked before processing.

### Tool Authorization (ToolAuthorizer)

- `core/interfaces.go:385-388` -- `ToolAuthorizer` interface allows agents to implement `AddAllowedTools(tools ...string) error` and `GetAllowedTools() []string`. Used via `/allow` command to grant tool access dynamically.

### Unix User Isolation (run_as_user)

- `core/runas.go` -- When `run_as_user` is configured, agent subprocess is spawned under a different Unix user via `sudo -n -iu`. Prefers sudo over setuid or su for security and environment correctness. Verification cached for 30 seconds.

## Stable Contracts

### API Endpoints (Unversioned)

- `core/api.go` -- Unix socket API endpoints: `/send`, `/sessions`, `/cron/add`, `/cron/list`, `/cron/info`, `/cron/edit`, `/cron/del`, `/cron/exec`, `/cron/run`, `/timer/add`, `/timer/list`, `/timer/info`, `/timer/del`, `/relay/send`, `/relay/bind`, `/relay/binding`. No `/v1/` or `/v2/` versioning.

### WebSocket Bridge Protocol

- `core/bridge.go` -- WebSocket path default `/bridge/ws`. REST endpoints at `/bridge/sessions` and `/bridge/sessions/`. Protocol messages use a `type` field (register, message, card_action, preview_ack, ping/pong). Versioned reconstruct-reply-context payload with `v: 1` field.

### Webhook Server

- `core/webhook.go` -- HTTP endpoint at configurable path (default `/hook`). Token-based auth via Bearer, `X-Webhook-Token`, or query param.

### Management HTTP API

- `core/management.go` -- Token-authenticated HTTP API at `/api/v1/` prefix. Uses `subtle.ConstantTimeCompare` for timing-safe auth checks.

### Instance Lock (File-Based Exclusive Lock)

- `cmd/cc-connect/instance_lock.go` -- `InstanceLock` uses `syscall.Flock` with `LOCK_EX|LOCK_NB` on per-config-path lock file. Writes PID for diagnostics. `Release()` truncates, unlocks, closes. `KillExistingInstance()` reads PID and sends `SIGKILL`. Separate Windows implementation at `instance_lock_windows.go`.

### Retry / Backoff Patterns

- `platform/feishu/feishu.go` -- `withTransientRetry()`: 3 retries max, 500ms initial delay, 5s max, exponential doubling + 25% jitter. Retries only on transient errors. `withFreshTenantAccessTokenRetry()`: single retry on expired token.
- `platform/wecom/websocket.go`, `platform/telegram/telegram.go`, `platform/qqbot/qqbot.go`, `platform/weixin/weixin.go`, `platform/wps-xiezuo/wpsxiezuo.go`, `platform/max/max.go` -- WebSocket reconnection backoff (1s initial, 30-60s max), context-aware.
- `platform/weixin/weixin.go` -- `sendChunkWithRetry()` and `sendSingleItemWithRetry()` retry once on `ret=-2` after refreshing `context_token`, 500ms delay.
- `platform/qqbot/qqbot.go` -- Single retry on 401 (token expiry), rebuilds request with fresh token.

### Graceful Shutdown Orchestration

- `cmd/cc-connect/main.go` -- Signal handler (SIGINT, SIGTERM) triggers ordered shutdown: management server, bridge server, webhook server, heartbeat scheduler, timer scheduler, cron scheduler, API server, engines, log closer, instance lock release. Supports self-restart via `RestartCh` with `syscall.Exec`.
- HTTP server shutdown: `http.Server.Shutdown(ctx)` with 5-second timeout contexts (`core/webhook.go`, `core/bridge.go`, `platform/max/max.go`).
- Agent session graceful stop: two-phase shutdown (graceful stop with 120s timeout, then SIGTERM escalation) (`agent/claudecode/session.go`).
- All platforms create `context.WithCancel` on start; `Stop()` calls cancel to signal all goroutines.

## Scan Warnings

- The Unix Socket API (`core/api.go`) has no authentication mechanism beyond filesystem permissions. If the socket file permissions are misconfigured, any local user can access the API.
- WeCom API error propagation (`platform/wecom/wecom.go`) uses flat `fmt.Errorf` without `%w` wrapping, losing the error chain for `errors.Is`/`errors.As`.
- No global HTTP panic recovery middleware exists for the management API or API server. A panic in a handler will propagate to Go's default net/http behavior (log + stack trace + 500).
- Bridge server `insecure` mode allows unauthenticated access; warning is logged but not blocked at runtime.
- `allow_from` defaults to fail-open (empty = allow all), while `admin_from` defaults to fail-closed (empty = deny all). This asymmetry could lead to misconfiguration.
- API endpoints are unversioned (no `/v1/`, `/v2/`), which could make backward-incompatible changes difficult.
