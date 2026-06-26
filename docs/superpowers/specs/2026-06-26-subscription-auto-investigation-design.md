# Subscription: Group Message Auto-Investigation

## Problem

When alarm cards appear in Feishu group chats, engineers need to manually investigate each alert. This is slow and error-prone. We need a mechanism for bots to periodically scan group messages, detect alarm cards (but not recovery cards), and automatically trigger agent investigation with replies threaded under the original card.

## Solution

Introduce a **Subscription** entity — a per-bot, per-group subscription that periodically pulls new messages, filters them, and injects matched messages into the agent for processing. Replies are threaded under the original message.

## Data Model

```
Subscription {
    ID                  string    // auto-generated unique identifier (16-hex-char, e.g. "a1b2c3d4e5f67890")
    Project             string    // owning project (bot identity)
    ChatID              string    // monitored group chat ID
    ChatName            string    // group name at creation time (may become stale if group is renamed)
    Platform            string    // platform type (feishu/telegram/...)
    SessionKey          string    // resolved session key for injected messages, format: "{platform}:{chatID}:{botUserID}"
    Filter              string    // inclusion filter: substring match (case-insensitive), empty = match all
    ExcludeFilter       string    // exclusion filter: substring match (case-insensitive), empty = no exclusion
    Prompt              string    // agent prompt template, {{content}} replaced with message text
    Anchor              string    // opaque anchor string; interpreted by the platform's MessageScanner
    Interval            string    // cron expression, default "*/2 * * * *"
    ConcurrencyLimit    int       // max total active sessions for this subscription, default 5
    TimeoutMins         int       // max duration per agent session, default 30, 0 = unlimited
    Enabled             bool
    LastRun             time.Time
    LastError           string
    ConsecutiveErrors   int       // auto-disable after 10 consecutive permanent failures
    ProcessedIDs        []string  // recently injected message IDs (dedup buffer, max 100 entries)
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

### Uniqueness constraint

`(Project, ChatID)` must be unique. Each bot can subscribe to a group at most once. Multiple bots can independently subscribe to the same group.

### Filter logic

- Filter empty + ExcludeFilter empty → match all messages
- Filter non-empty → message title/content must contain Filter as substring (case-insensitive) to be included
- ExcludeFilter non-empty → message title/content containing ExcludeFilter as substring (case-insensitive) is excluded
- For card messages, the platform's `MessageScanner` implementation must return extracted readable text in `ScannedMessage.Content` (e.g., Feishu implementation extracts card text internally before returning)

### Prompt template

`{{content}}` is replaced with the matched message's text content. Default prompt: `"{{content}}"`.

### Anchor

- The `Anchor` field is an opaque string interpreted by the platform's `MessageScanner` implementation
- Feishu implementation uses Feishu message IDs as anchor values; the API call converts the anchor to a timestamp internally, then uses the anchor for client-side dedup
- On first creation, Anchor is empty → default to current timestamp to avoid flooding with historical messages
- Runtime fallback: if `Anchor` is empty, the `MessageScanner` uses the current time as the starting point for this scan only; the persisted `Anchor` is never mutated based on runtime heuristics

### Deduplication

- `ProcessedIDs` tracks message IDs that were successfully injected but whose anchor has not yet advanced past them
- On re-scan (after partial failure or crash), messages with IDs in `ProcessedIDs` are skipped
- Entries older than the scan interval are pruned on each scan
- Max 100 entries; oldest entries pruned first

### Auto-disable

- `ConsecutiveErrors` is incremented only for **permanent** errors (e.g., bot removed from group, token revoked)
- **Transient** errors (e.g., 429 rate limit, network timeout) do NOT increment `ConsecutiveErrors`; the scan is simply retried on the next trigger
- After 10 consecutive permanent errors, the subscription is auto-disabled
- On any successful scan, `ConsecutiveErrors` resets to 0
- When auto-disabled: emit `slog.Error("subscription: auto-disabled", "subscription_id", id, "auto_disabled", true, "consecutive_errors", count, "last_error", lastErr)` and attempt to send a notification to the chat with `MsgSubAutoDisabled` i18n message

## Execution Flow

```
Cron trigger (robfig/cron)
  │
  ▼
1. Acquire subscription mutex; skip if previous scan still running (log warning)
  │
  ▼
2. Pull new messages from group (since Anchor)
   - Call MessageScanner.ListMessages() with ListMessagesOptions
   - Collect ALL pages before proceeding (do not inject on partial fetch)
   - If any page fails, abort the entire scan, leave anchor unchanged, log error
  │
  ▼
3. Filter messages
   - Skip messages where IsBot=true
   - Skip messages whose MessageID is in ProcessedIDs (dedup)
   - Apply Filter (inclusion)
   - Apply ExcludeFilter (exclusion)
   - For interactive cards → Content already contains extracted text (platform responsibility)
   - Log warning for card messages where Content is empty after extraction
  │
  ▼
4. Submit matched messages for processing
   - Check total active sessions for this subscription (not just this scan)
   - ConcurrencyLimit applies to TOTAL active sessions, not per-scan starts
   - If at limit, queue remaining messages for next scan; log count
   - Each message: build prompt (replace {{content}}), inject into engine via NewSideSession
   - replyCtx built via ThreadReplyContextBuilder interface (see Platform Interface Extension)
   - Track each submitted message ID in ProcessedIDs
  │
  ▼
5. Update Anchor and persist
   - Advance anchor to the last message in the fetched list (whether matched or not)
   - If some injections were queued (concurrency limit reached), anchor still advances — those messages are tracked in ProcessedIDs for dedup
   - Persist Subscription state (Anchor, ProcessedIDs, LastRun, ConsecutiveErrors) atomically
   - If any injection failed (non-queued), anchor stops before the first failed message
  │
  ▼
6. Log results
   - Scan summary: N new messages, M matched, K submitted, Q queued
   - Per-message: subscription_id, message_id, session_id, status
   - Anchor update: old → new
```

### Reply mode

All replies use **Reply in Thread** mode — agent responses are threaded under the original alarm card, keeping the investigation context tightly coupled with the alert.

### Session strategy

- Each matched message is injected via `NewSideSession` (independent session per message), matching the cron `session_mode=new_per_run` pattern
- The subscription manager tracks active session IDs per subscription
- When a session completes or times out, the engine notifies the subscription manager to release the ConcurrencyLimit slot (via callback or polling)
- Note: concurrent side sessions writing to the same workspace directory may not be safe for all agents. The ConcurrencyLimit default of 5 is conservative; users should validate agent behavior before increasing it

### Concurrency safety

- Each Subscription has a `sync.Mutex` keyed by subscription ID
- Only one scan per subscription runs at a time; if the previous scan is still running when the next trigger fires, skip the new scan and log a warning with `SkippedScans` count
- SubscriptionManager must remain in the `core/` package to avoid circular imports with Engine (same pattern as CronScheduler)

## Storage

```
data_dir/subscriptions/
  jobs.json          // subscription definitions + anchor state + ProcessedIDs
  logs/              // one file per subscription
    {id}.log         // append-only JSON lines, rotated by size (max 50 MB per file)
```

- `jobs.json` uses `AtomicWriteFile` (same as CronStore) for crash-safe writes
- Log files are append-only (one JSON line per entry); no compaction on write
- On startup, if a log file exceeds 50 MB, truncate to last 1000 entries
- Each subscription's log is in a separate file to avoid write contention

## Startup Recovery

1. Engine loads `jobs.json` on startup
2. Register all Enabled=true subscriptions with `robfig/cron`
3. Subscriptions with empty Anchor default to current timestamp (avoid historical flood)
4. ProcessedIDs are loaded from `jobs.json` and used to deduplicate messages that were injected before a crash
5. Active session tracking: on startup, the subscription manager queries the engine for surviving sessions and reconciles its ConcurrencyLimit counters

## Creation & Management

### Slash commands (in group chat)

**AuthZ**: Only users in the project's `admin_from` list can create, modify, or delete subscriptions. Non-admin users can only use `/subscribe list` and `/subscribe show`. Reuse `MsgAdminRequired` for unauthorized attempts.

**Command syntax** (follows existing `/cron add` positional pattern):

```
/subscribe <filter> <exclude> [prompt...]
```

Where:
- First positional arg: filter keyword (use `"-"` to skip, meaning match all)
- Second positional arg: exclude keyword (use `"-"` to skip)
- Remaining args: prompt template (default: `"{{content}}"`)
- `interval` and `concurrency` are set via `/subscribe edit` or WebUI

Examples:
```
/subscribe 告警 恢复 排查以下报警：{{content}}
/subscribe - -                              // match all messages, default prompt
/subscribe error - 请检查这个错误：{{content}}
```

Management commands:
- `/subscribe list` — list subscriptions for current group
- `/subscribe list all` — list all subscriptions for the project
- `/subscribe show <id>` — show full details of a subscription
- `/subscribe edit <id> <field> <value>` — update a field (filter, exclude, prompt, interval, concurrency, timeout)
- `/subscribe enable <id>` — resume
- `/subscribe disable <id>` — pause
- `/subscribe del <id>` — delete (with confirmation)
- `/subscribe help` — show usage, explain `{{content}}` template

### Management API

```
POST   /api/v1/subscription              create
GET    /api/v1/subscription              list (query: ?chat_id= for filtering)
GET    /api/v1/subscription/{id}         detail
PATCH  /api/v1/subscription/{id}         update (field-by-field, including {"enabled": true/false})
DELETE /api/v1/subscription/{id}         delete
POST   /api/v1/subscription/{id}/trigger  manual scan trigger (like cron's exec)
```

- Uses singular resource name `/subscription` for consistency with existing `/cron` API
- No `toggle` endpoint — enable/disable via `PATCH` with `{"enabled": bool}`, matching cron's pattern
- AuthZ: reuses existing management API bearer-token auth (same as cron endpoints)

### WebUI

New `/subscriptions` route (component: `SubscriptionList.tsx`):
- **Creation modal**: group selector, filter, exclude filter, prompt template (with `{{content}}` hint in placeholder), interval picker (reuse CronPicker), concurrency limit, timeout
- **Table**: ID, group name, Filter, Exclude, Prompt, Interval, status, ConsecutiveErrors, last scan time, last error
- **Actions**: enable/disable, edit (modal), delete (with confirmation), trigger manual scan
- **Detail page**: subscription config + processing log entries

### i18n keys

All subscription user-facing strings use `core/i18n.go`. Required MsgKey constants:

- `MsgSubCreated` — "Subscription created (ID: {id})\nGroup: {chatName}\nFilter: {filter} | Exclude: {exclude}\nPrompt: {prompt}\nInterval: {interval}\nTip: Use {{content}} in prompt to reference the matched message."
- `MsgSubAlreadyExists` — "This group is already subscribed. Use /subscribe edit to modify."
- `MsgSubNotFound` — "Subscription not found: {id}"
- `MsgSubListTitle` — "Subscriptions for {chatName}:"
- `MsgSubListAllTitle` — "All subscriptions:"
- `MsgSubEnabled` — "Subscription {id} enabled"
- `MsgSubDisabled` — "Subscription {id} disabled"
- `MsgSubDeleted` — "Subscription {id} deleted"
- `MsgSubAutoDisabled` — "Subscription {id} auto-disabled after {count} consecutive errors. Last error: {error}. Use /subscribe enable {id} to re-enable."
- `MsgSubDelConfirm` — "Are you sure you want to delete subscription {id}?"
- `MsgSubShowFormat` — detailed subscription display format
- `MsgSubEditUsage` — "Usage: /subscribe edit <id> <field> <value>"
- `MsgSubUsage` — "Usage: /subscribe <filter> <exclude> [prompt...]"
- `MsgSubHelp` — detailed help text explaining all subcommands and {{content}}

Each key requires translations for EN, ZH, ZH-TW, JA, ES.

## Logging

### Structured log fields

```
// Scan trigger
slog.Info("subscription: scan started",
    "subscription_id", id, "chat_id", chatID, "project", project)

// Scan result
slog.Info("subscription: scan completed",
    "subscription_id", id, "chat_id", chatID, "project", project,
    "total", totalCount, "matched", matchCount, "submitted", submitCount, "queued", queuedCount)

// Scan skipped (previous still running)
slog.Warn("subscription: scan skipped, previous still running",
    "subscription_id", id, "skipped_scans", skippedCount)

// Message submitted
slog.Info("subscription: message submitted",
    "subscription_id", id, "message_id", msgID, "session_id", sessionID)

// Anchor update
slog.Info("subscription: anchor updated",
    "subscription_id", id, "old_anchor", oldAnchor, "new_anchor", newAnchor)

// Auto-disable
slog.Error("subscription: auto-disabled",
    "subscription_id", id, "auto_disabled", true, "consecutive_errors", count, "last_error", lastErr)

// Error
slog.Error("subscription: scan failed",
    "subscription_id", id, "chat_id", chatID, "project", project, "error", err, "error_type", errorType)
```

### Processing log entries

```
LogEntry {
    SubscriptionID    string
    MessageID         string
    ChatID            string
    Content           string    // matched message summary (first 200 chars)
    SessionID         string    // associated agent session
    Status            string    // "submitted" / "completed" / "failed" / "queued"
    CreatedAt         time.Time
}
```

Persisted to `data_dir/subscriptions/logs/{id}.log` (append-only JSON lines). Visible in WebUI subscription detail page.

## Architecture Placement

```
core/subscription.go          // Subscription struct, SubscriptionManager, scan logic
core/subscription_cmd.go      // cmdSubscribe() and subcommand handlers (Engine method)
web/src/pages/SubscriptionList.tsx  // WebUI management page
```

- `SubscriptionManager` is a standalone component following the `CronScheduler` pattern — it holds `map[string]*Engine` and calls `engine.ExecuteSubscriptionScan()` on scan triggers. The Engine holds a `*SubscriptionManager` field set via `SetSubscriptionManager()`.
- The `/subscribe` command is registered in the Engine's `builtinCommands` slice and `handleCommand` switch, following the same pattern as `/cron`. It is NOT in a `plugin_*.go` file.
- The subscription module depends on `core.Engine` for message injection and on `core.MessageScanner` (an optional interface defined in core) for message listing. Platforms implement `MessageScanner`; Feishu is the first implementation. The subscription module does not depend on `core.CronJob` — it only shares the `robfig/cron` scheduling library.
- SubscriptionManager must remain in the `core/` package to avoid circular imports with Engine (same pattern as CronScheduler).

## Platform Interface Extensions

### MessageScanner — message history retrieval

```go
type ListMessagesOptions struct {
    Since     time.Time // start time for message listing; always set by core
    PageSize  int       // messages per page, default 50
    PageToken string    // pagination token from previous call
}

type MessageScanner interface {
    ListMessages(ctx context.Context, chatID string, opts ListMessagesOptions) ([]ScannedMessage, string, error)
}
```

`ListMessages` returns `([]ScannedMessage, nextPageToken string, error)`. If `nextPageToken` is non-empty, call again with this token to get the next page. If empty, all messages have been returned.

The `Since` field is always a `time.Time` — core never passes platform-specific anchor types. Platforms that support ID-based paging (like Feishu) maintain their own anchor-to-timestamp mapping internally. The `Anchor` field on the Subscription struct is an opaque string that only the platform's `MessageScanner` interprets.

Platforms implement this to provide message history access. Feishu implementation uses `Im.Message.List` API with exponential backoff on rate-limit responses (429 status). A platform-level rate limiter shared across all subscriptions prevents API quota exhaustion.

`ScannedMessage` includes an `IsBot` field so the subscription filter can skip bot messages without needing to know the bot's user ID, and an `IsCard` field for logging when extraction fails:

```go
type ScannedMessage struct {
    MessageID string
    ChatID    string
    UserID    string
    IsBot     bool
    IsCard    bool      // true if message is an interactive card
    MsgType   string
    Content   string    // human-readable text; for card messages, platforms must extract text internally
    CreatedAt time.Time
}
```

### ThreadReplyContextBuilder — reply-in-thread support

```go
// ThreadReplyContextBuilder is an optional interface for platforms that can
// construct a reply context targeting a specific message for reply-in-thread.
type ThreadReplyContextBuilder interface {
    BuildThreadReplyCtx(chatID string, messageID string) (any, error)
}
```

The Feishu implementation returns a `replyContext` struct targeting the specified message ID. The `ExecuteSubscriptionScan` method uses this interface to construct per-message threaded reply contexts. If the platform does not implement this interface, fall back to `ReconstructReplyCtx(sessionKey)` (reply to chat root, not threaded).

## Feature Flag

A global `subscriptions_enabled` config flag (default: `true`). When `false`, no subscriptions are scheduled, the `/subscribe` command returns `MsgSubNotAvailable`, and the management API returns 404. This allows instant feature kill without code changes.

## Poison Message Handling

If a specific message consistently fails injection (e.g., due to engine error), it would block anchor advancement indefinitely. After 3 consecutive scan attempts where the same message fails, skip it, log a warning with `slog.Warn("subscription: poison message skipped", ...)`, and advance the anchor past it. This prevents a single bad message from stalling the entire subscription.
