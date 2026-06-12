# Engineering Constraints

[Raw scan](../raw/engineering-constraints.md)

## Error Handling

All error wrapping uses `fmt.Errorf("context: %w", err)` (~162 in agent/, ~120 in core/). No third-party error libraries. Sentinel errors (`errors.New` + `errors.Is`) for programmatic branching: `ErrNotSupported`, `ErrAttachmentSendDisabled`, `ErrCronJobNotFound`, `ErrCronProjectNotFound`. Custom error types for Feishu card API, JSON-RPC, and retry logic. Engine maps error substrings to i18n keys via `agentErrorHandlers`. Management API uses unified JSON envelope (`mgmtJSON`/`mgmtError`/`mgmtOK`). Hook-based error observability via `HookEventError`. No global HTTP panic recovery.

## Performance Invariants

- **Incoming rate limiting**: sliding-window per key (`core/ratelimit.go`)
- **Outgoing rate limiting**: token-bucket per platform (`core/outgoing_ratelimit.go`)
- **Role-based rate limiting**: per-role limits with global fallback (`core/user_roles.go`)
- **Streaming throttling**: `streamPreview` with 1500ms interval, 30-char min delta, 2000-char cap
- **Message dedup**: `MessageDedup` with 60s TTL, startup-replay guard
- **Caching**: `sync.RWMutex` (presets, runAs) and `sync.Map` (platform user/chat caches)
- **Atomic writes**: temp-file + rename pattern for all persistent state
- **Concurrency**: `Session.TryLock()`, `CompareAndSetAgentSessionID()`, `sync.Once` for cleanup, `sync.WaitGroup` for goroutine lifecycle, `atomic.Value`/`atomic.Bool` for lock-free state

## Security

- **Redaction**: `RedactEnv()`, `RedactArgs()`, `RedactToken()`, `redactInlineSecrets()` mask secrets in logs and output
- **Timing-safe comparison**: all token/auth checks use `crypto/subtle.ConstantTimeCompare`
- **CORS**: configurable origin allowlist on management and bridge servers
- **Encryption**: AES-256-CBC for WeCom and WPS Xiezuo callbacks; CDN media decryption for WeChat
- **Webhook timeout**: 5-minute cap on shell commands
- **Token generation**: `crypto/rand` with timestamp fallback

## Authentication

- Management API: Bearer token or `?token=` param; empty = unauthenticated mode
- Webhook: three header/param options; empty = open
- Bridge: three header/param options; `insecure` flag for local dev
- WeCom: SHA1 signature verification
- WPS Xiezuo: HMAC-SHA256 signature
- MAX: webhook secret header/param
- Unix socket: 0600 file permissions (no token)
- All platforms require credentials at init; fail if missing

## Authorization

- **Admin allowlist** (`admin_from`): fail-closed; empty = deny all privileged commands
- **User allowlist** (`allow_from`): fail-open; empty = allow all
- **RBAC**: `UserRoleManager` with per-role disabled commands and rate limits
- **Privileged command gating**: `isAdmin()` check + audit logging
- **Disabled commands**: configurable per-project, `"*"` disables all
- **Banned words**: case-insensitive content filter before processing
- **Tool authorization**: `ToolAuthorizer` interface for dynamic tool access
- **Unix user isolation**: `run_as_user` spawns via `sudo -n -iu`

## Stable Contracts

- Unix socket API: unversioned endpoints (`/send`, `/sessions`, `/cron/*`, `/timer/*`, `/relay/*`)
- Bridge WebSocket: `/bridge/ws` with typed protocol messages, `v: 1` payload versioning
- Management API: `/api/v1/` prefix with constant-time token auth
- Instance lock: `syscall.Flock` with `LOCK_EX|LOCK_NB`
- Retry patterns: exponential backoff + jitter across all platform adapters
- Graceful shutdown: ordered teardown with 5s HTTP timeouts, 120s agent session stop, self-restart via `syscall.Exec`
