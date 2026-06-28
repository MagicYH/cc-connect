# 订阅回复 Thread 化与过滤规则修复 Deployment

> Source spec: ./2026-06-28-subscription-thread-reply-filter-fix-design.md
> Cross-reference (for v2): ./2026-06-28-subscription-thread-reply-filter-fix-verification.md
>
> **Two-Phase Document**
> - Phase 1 (v1): authored during brainstorming via writing-deployment-plan. Describes intent.
> - Phase 2 (v2): completed during finishing-a-development-branch. Adds executable runbook.

---

## Phase 1 · Release Intent (v1)

### 1.1 Release Scope
- Affected services/components: `cc-connect` (single Go binary)
- Includes frontend release: yes — WebUI i18n placeholder updates in `web/src/i18n/locales/*.json`
- Includes data migration: no — existing subscription data is compatible (filter strings are valid regex)
- Includes config change: no — no new config.toml fields required

### 1.2 Infrastructure Touchpoints
- DB migration: none — subscriptions persisted as JSON files, no schema change
- Env vars: none
- TCC config: none
- New dependencies: none — `regexp` is stdlib
- Capacity: no — no change in request volume or memory footprint

### 1.3 Deployment Order
| Step | Action | Must precede |
|---|---|---|
| 1 | Build Go binary with `make build` | step 2 |
| 2 | Restart daemon with `cc-connect daemon restart` | — |

Single-step deployment. No ordering dependencies beyond build-then-restart.

### 1.4 Risk Assessment
- R1: `makeSessionKey` behavior change for `threadIsolation=false` — thread messages (with `root_id`) now route to thread session instead of base session. Mitigation: only affects messages already inside threads; non-thread messages unchanged. Intentional per design spec §5 compat notes.
- R2: `filterMessages` behavior change — now excludes human messages and self-bot messages. Existing subscriptions that relied on matching human messages will silently stop matching them. Mitigation: subscriptions are designed for bot alert messages; human messages have the @mention channel. Low likelihood of impact.
- R3: Regex semantics change for filter/exclude_filter — special characters (`.`、`*`、`+`) gain regex meaning. Users with literal-dot filters like `error.txt` will match `errorXtxt` instead. Mitigation: `compileFilters()` validates at creation time; invalid regex is rejected with clear error message. Document regex behavior in help text.
- R4: `/sub` alias removal — users currently using `/sub` will get "unknown command". Mitigation: `/subscribe` is the canonical command; `/sub` was a short alias. Low impact.

### 1.5 Rollback Strategy (high-level)
- Rollback trigger: subscription replies stop appearing in threads; or filter rules cause all messages to be dropped; or `/sub` users report breakage
- Rollback scope: code only — `git revert` the commit(s) and rebuild
- Data reversibility: full — no data migration, subscription JSON files unchanged

### 1.6 Observability Plan
- Key metrics: subscription scan success rate, filter match count, thread reply success count
- Key logs: `subscription: filter results` (filter stats), `subscription: built thread reply context` (thread creation), `subscription: reply_in_thread not supported, falling back` (fallback triggered)
- Observation windows: 5 min / 30 min post-deploy — verify first scan cycle completes and replies land in threads

### 1.7 Communication
- Notify: none required — this is a personal dev tool, not a shared service
- Timing: N/A
- Channels: N/A

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
