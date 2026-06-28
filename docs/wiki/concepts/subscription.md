# Subscription

Periodically scan platform messages matching a filter, inject them into agent sessions for processing. Opt-in per project via `subscriptions_enabled = true`.

## How It Works

1. **Scan cycle**: At each cron interval, the subscription manager calls the platform's `MessageScanner` to list recent messages in the target chat
2. **Filter**: Messages are filtered by keyword (must contain) and exclude keyword (must not contain); bot messages and already-processed IDs are skipped
3. **Inject**: Matched messages are injected into agent sessions as side sessions, with the subscription prompt (default: `{{content}}`) replacing `{{content}}` with the original message text
4. **Anchor**: After each scan, the latest message timestamp is saved as an anchor; the next scan only fetches messages after this point

## Slash Commands

| Command | Description |
|---------|-------------|
| `/subscribe <filter> [-e\|--exclude <exclude>] [-p\|--prompt <prompt>] [-i\|--interval <cron>]` | Create a subscription (default subcommand) |
| `/sub list` | List subscriptions for current chat |
| `/sub list all` | List all subscriptions in the project |
| `/sub show <id>` | Show subscription details |
| `/sub enable <id>` | Enable a subscription (admin only) |
| `/sub disable <id>` | Disable a subscription (admin only) |
| `/sub del <id>` | Delete a subscription (admin only) |

- Use `"-"` as filter to match all messages
- Use `{{content}}` in prompt to reference the matched message text
- Default interval: `*/5 * * * *` (every 5 minutes)
- Default prompt: `{{content}}`
- One subscription per chat (per project); duplicate `(project, chat_id)` pairs are rejected

## Example

```
/sub 报警 -e 测试 -p "分析这条报警并给出建议: {{content}}" -i */10 * * * *
```

Creates a subscription that scans every 10 minutes for messages containing "报警" but not "测试", injecting the prompt "分析这条报警并给出建议: <message>".

## Management API

All endpoints under `/api/v1/subscription`:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/subscription` | List all (optional `?project=<name>`) |
| `POST` | `/subscription` | Create |
| `GET` | `/subscription/{id}` | Get by ID |
| `PATCH` | `/subscription/{id}` | Partial update |
| `DELETE` | `/subscription/{id}` | Delete |
| `POST` | `/subscription/{id}/enable` | Enable + schedule |
| `POST` | `/subscription/{id}/disable` | Disable + unschedule |
| `POST` | `/subscription/{id}/run` | Trigger immediate scan (async, returns 202) |

Returns 503 if `subscriptions_enabled` is not set.

## Data Model

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Auto-generated UUID |
| `project` | string | Owning project |
| `chat_id` | string | Target chat |
| `filter` | string | Keyword to match (case-insensitive) |
| `exclude_filter` | string | Keyword to exclude (case-insensitive) |
| `prompt` | string | Template with `{{content}}` |
| `interval` | string | Cron expression |
| `anchor` | string | RFC3339Nano timestamp of last scanned message |
| `enabled` | bool | Whether the subscription is active |
| `consecutive_errors` | int | Auto-disabled when >= 10 |
| `concurrency_limit` | int | Max parallel agent sessions (default 5) |
| `processed_ids` | []string | Last 100 processed message IDs (dedup) |

## Error Handling

- Consecutive permanent errors (permission/not-exist/token/unauthorized/forbidden) increment `consecutive_errors`
- At 10 consecutive permanent errors, the subscription auto-disables and unschedules
- Transient errors are recorded but do not increment the counter
- Use `/sub enable <id>` to re-enable after fixing the root cause

## Configuration

```toml
# In config.toml, per project:
[[projects]]
  name = "my-project"
  subscriptions_enabled = true   # Required; defaults to false
```

## WebUI

The admin web interface includes a Subscription List page at `/subscriptions` with full CRUD: card grid view, add/edit modal with interval presets (5m/15m/30m/1h/2h/6h/12h/24h), hover actions (run/enable/disable/delete).

## Implementation

- **Manager**: `core/subscription.go` — `SubscriptionManager` with cron scheduler, `SubscriptionStore` with JSON file persistence
- **Execution**: `core/engine.go` — `ExecuteSubscriptionScan` resolves platform, lists messages, filters, injects into agent sessions
- **Commands**: `core/subscription_cmd.go` — slash command handler
- **API**: `core/management.go` — REST endpoints
- **WebUI**: `web/src/pages/Subscriptions/SubscriptionList.tsx`
- **Platform interface**: `core/interfaces.go` — `MessageScanner`, `ThreadReplyContextBuilder`

## Cross-References

- [Slash Commands](slash-commands.md) — full command reference
- [Cron Scheduler](cron-scheduler.md) — the underlying cron engine
- [File-Based Persistence](file-based-persistence.md) — JSON store pattern
- [Capability Interface](capability-interface.md) — `MessageScanner` and `ThreadReplyContextBuilder`
- [RBAC](rbac.md) — admin-only commands
