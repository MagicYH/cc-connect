# Subscription Auto-Investigation Deployment

> Source spec: ./2026-06-26-subscription-auto-investigation-design.md
> Cross-reference (for v2): ./2026-06-26-subscription-auto-investigation-verification.md
>
> **Two-Phase Document**
> - Phase 1 (v1): authored during brainstorming via writing-deployment-plan. Describes intent.
> - Phase 2 (v2): completed during finishing-a-development-branch. Adds executable runbook.

---

## Phase 1 · Release Intent (v1)

### 1.1 Release Scope
- Affected services/components: `cc-connect` binary, `web/` frontend (WebUI)
- Includes frontend release: yes — new `/subscriptions` page in WebUI
- Includes data migration: no — new `data_dir/subscriptions/` directory, created on first use
- Includes config change: yes — new `subscriptions_enabled` flag in `config.toml` (default: `true`)

### 1.2 Infrastructure Touchpoints
- New data directory: `data_dir/subscriptions/` with `jobs.json` and `logs/` subdirectory — auto-created on first subscription creation
- Config: `subscriptions_enabled` boolean flag in `config.toml`, default `true` — when `false`, all subscription functionality is disabled (source: design spec §Feature Flag)
- Feishu API: `Im.Message.List` API used for scanning group messages — no new app permissions required; the bot already has `im:message` scope for reading messages
- No new env vars, DB migrations, or external dependencies

### 1.3 Deployment Order
| Step | Action | Must precede |
|---|---|---|
| 1 | Build cc-connect binary (includes new `core/subscription.go`, `core/subscription_cmd.go`) | daemon restart |
| 2 | Build WebUI (`make web`) | daemon restart |
| 3 | Restart cc-connect daemon (`cc-connect daemon restart`) | — |

### 1.4 Risk Assessment
- R1: **Feishu API rate limiting** — Subscription scans call `Im.Message.List` every 2 minutes per subscription. With multiple subscriptions, this could hit Feishu's API rate limits (typically 5 req/s). Mitigation: platform-level rate limiter shared across all subscriptions; exponential backoff on 429 responses (source: design spec §Platform Interface Extensions).
- R2: **Agent resource exhaustion** — Each matched alarm starts a new agent session (`NewSideSession`). An alarm burst (100+ messages in 2 minutes) could spawn many concurrent sessions. Mitigation: `ConcurrencyLimit` (default 5) caps total active sessions per subscription; `TimeoutMins` (default 30) prevents session leaks.
- R3: **Duplicate message processing on crash** — If the daemon crashes after injecting messages but before persisting the updated anchor, messages may be re-injected on restart. Mitigation: `ProcessedIDs` dedup buffer prevents re-injection of already-processed messages (source: design spec §Deduplication).
- R4: **WebUI stale data** — `ChatName` is stored at subscription creation time and may become stale if the group is renamed. Mitigation: acceptable trade-off; ChatID is the authoritative identifier.

### 1.5 Rollback Strategy (high-level)
- **Rollback trigger**: Subscription feature causes excessive Feishu API calls, agent resource exhaustion, or unexpected message processing
- **Primary rollback**: Set `subscriptions_enabled = false` in `config.toml` and restart daemon — all subscription scanning stops immediately, existing subscriptions remain in `jobs.json` but are not scheduled
- **Full rollback**: Deploy previous cc-connect binary — subscription code is entirely removed; `data_dir/subscriptions/` files are inert (not read by old binary) and can be deleted manually
- **Data reversibility**: No data migration was performed; `data_dir/subscriptions/` can be safely deleted. No Feishu-side state was mutated beyond sending thread replies.

### 1.6 Observability Plan
- Key metrics: subscription scan count, messages matched per scan, active session count per subscription, ConsecutiveErrors per subscription
- Key logs: `subscription: scan started`, `subscription: scan completed`, `subscription: scan skipped`, `subscription: auto-disabled`, `subscription: poison message skipped`
- Observation windows: 15 min (verify first scan completes), 1h (verify steady-state scanning), 24h (verify no auto-disables or resource issues)

### 1.7 Communication
- Notify: team members using the bot in Feishu groups
- Timing: pre-deploy (announce new feature), post-deploy (confirm working)
- Channels: Feishu group

---

## Phase 2 · Release Runbook (v2)
> Filled in during finishing-a-development-branch after implementation.

### 2.1 Pre-Deploy Checklist
> To be filled during finishing-a-development-branch Step 1.5.

### 2.2 Executable Steps
> To be filled during finishing-a-development-branch Step 1.5.

### 2.3 Post-Deploy Verification
> To be filled during finishing-a-development-branch Step 1.5.

### 2.4 Concrete Rollback Commands
> To be filled during finishing-a-development-branch Step 1.5.

### 2.5 v1 Revisions Log
> To be filled during finishing-a-development-branch Step 1.5.
