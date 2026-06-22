# Streaming Multi-Slot Rich Card Deployment

> Source spec: ./2026-06-21-streaming-multi-slot-card-design.md
> Cross-reference (for v2): ./2026-06-21-streaming-multi-slot-card-verification.md
>
> **Two-Phase Document**
> - Phase 1 (v1): authored during brainstorming via writing-deployment-plan. Describes intent.
> - Phase 2 (v2): completed during finishing-a-development-branch. Adds executable runbook.

---

## Phase 1 · Release Intent (v1)

### 1.1 Release Scope
- Affected services/components: `cc-connect` binary (single Go binary, all platforms compiled in)
- Affected packages: `core/streaming.go`, `core/engine.go`, `platform/feishu/feishu.go`
- Includes frontend release: no
- Includes data migration: no
- Includes config change: no (existing `display.card_mode = "rich"` config key is reused; no new config keys needed)
- New dependencies: none

### 1.2 Infrastructure Touchpoints
- DB migration: none
- Env vars: none new (existing `FEISHU_APP_ID` / `FEISHU_APP_SECRET` continue to be used)
- TCC config: none
- New dependencies: none
- Capacity: no scaling changes needed — the multi-slot architecture reduces API payload size per update, so it actually lowers load

The only infrastructure change is behavioral: the Feishu platform adapter will use `cardElement.content()` slot-level patches instead of full-card rebuilds for tool events. This is a client-side API usage change, not a server-side infrastructure change.

### 1.3 Deployment Order

| Step | Action | Must precede |
|---|---|---|
| 1 | Merge `shadow` branch into `main` | Step 2 |
| 2 | Build cc-connect binary | Step 3 |
| 3 | Restart cc-connect daemon on target machine | — |

This is a single-binary deployment with no inter-service dependencies.

### 1.4 Risk Assessment

- **R1: Feishu cardElement.content() API incompatibility** — The slot-level patch API works for happyclaw (TypeScript) but has not been tested from a Go client. If the API behaves differently (e.g., element_id resolution inside `collapsible_panel` fails from Go SDK), all slot patches will fail. Mitigation: The degradation chain (L0→L1→L2) ensures fallback to full-card rebuild. If L0 fails on first use, the engine degrades to L1 for the rest of the turn, which is functionally equivalent to the current behavior.

- **R2: Sequence counter corruption from dual-track flush** — Text flush (600ms) and aux flush (1500ms) share a monotonic sequence counter. If the serialization channel has a bug, out-of-order updates could be rejected by Feishu. Mitigation: All API calls go through a single serialized dispatch channel per card. Text track has priority. If serialization fails, degradation to L1 (full rebuild) eliminates the multi-slot path.

- **R3: Regressed non-Feishu platforms** — Engine event loop restructuring could break `RichCardSupporter` or `StreamingCardPlatform` paths if the `if/else` dispatch is incorrect. Mitigation: The `StreamingRichCardSupporter` check happens first; if the platform does not implement it, the existing `RichCardSupporter` path is used unchanged. Unit tests with stub platforms (S8, S12 in verification) verify the fallback.

- **R4: cardEntity creation fails for some Feishu tenants** — `BuildStreamingCard` requires cardkit-v1 card entity creation, which may fail if the Feishu app lacks permissions or the tenant is restricted. Mitigation: `BuildStreamingCard` returns `ErrSlotNotSupported` on failure, and the engine falls back to `RichCardSupporter` for the entire turn. This is a graceful degradation identical to the current `StreamRichCardText` fallback.

### 1.5 Rollback Strategy (high-level)

- **Rollback trigger conditions**: Any of the following observed within 15 minutes of deploy:
  - Feishu card messages fail to render (blank cards or API errors logged)
  - Degradation to L1/L2 happening on >50% of turns (indicating L0 is not viable)
  - Non-Feishu platforms show regression (tool call cards not rendering)

- **Rollback scope**: Code only (git revert). No data or config to roll back.

- **Reversibility**: Fully reversible — revert the merge commit and rebuild/restart. The old `RichCardSupporter` code path is not deleted; `buildRichCardJSONBytes` is retained (renamed to `buildCompletedCardJSON`) and the full-card rebuild path remains functional.

### 1.6 Observability Plan

- **Key metrics** (from `slog` structured logs):
  - Slot update success/fail ratio per slot
  - Degradation level distribution (L0 vs L1 vs L2)
  - Flush cycle timing (text track / aux track)
  - Dedup skip rate (slots unchanged per flush)

- **Key logs/alerts**:
  - `slog.Error` on `StreamSlotContent` failures (ErrSlotRateLimited, ErrSlotNotSupported)
  - `slog.Warn` on degradation events (L0→L1, L0→L2)
  - `slog.Warn` on FinalizeStreamingCard Phase 2 failures

- **Observation windows**:
  - 15 min post-deploy: watch for degradation spikes or card rendering failures
  - 1 hour: confirm L0 success rate >90% (most turns stay at slot-level patch)
  - 24 hours: confirm no regressions on non-Feishu platforms

### 1.7 Communication

- **Notify**: cc-connect users (via the Feishu group where the bot operates)
- **Timing**: Post-deploy — announce that card rendering has been upgraded with streaming support
- **Channels**: Feishu group chat

---

## Phase 2 · Release Runbook (v2)

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
