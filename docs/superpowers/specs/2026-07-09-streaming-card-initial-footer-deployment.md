# Streaming Card Initial Footer Deployment

> Source spec: ./2026-07-09-streaming-card-initial-footer-design.md
> Cross-reference (for v2): ./2026-07-09-streaming-card-initial-footer-verification.md
>
> **Two-Phase Document**
> - Phase 1 (v1): authored during brainstorming via writing-deployment-plan. Describes intent.
> - Phase 2 (v2): completed during finishing-a-development-branch. Adds executable runbook.

---

## Phase 1 · Release Intent (v1)

### 1.1 Release Scope
- Affected services/components: cc-connect Go backend `core.Engine` rich-card footer composition and Feishu rich-card rendering tests.
- Includes frontend release: no.
- Includes data migration: no.
- Includes config change: no.
- Includes external API behavior change: no new API calls; existing streaming card updates carry non-empty footer content.

### 1.2 Infrastructure Touchpoints
- DB migration: none. Source: design §Scope excludes data changes and design §Error Handling states no new network operations.
- Env vars: none. Source: design §Scope excludes new user-facing configuration.
- TCC config: none. Source: design §Scope excludes new configuration.
- New dependencies: none. Source: design §Components reuses existing `composeRichStatusFooter`, `replyFooterWorkDir`, and Feishu `BuildRichCard` paths.
- Capacity: no scaling required. Source: design §Data Flow reuses existing rich-card rebuilds and does not add Feishu patch calls.

### 1.3 Deployment Order
| Step | Action | Must precede |
|---|---|---|
| 1 | Merge cc-connect backend code and tests | service release |
| 2 | Release cc-connect service binary through the existing service release path | post-release observation |
| 3 | Observe Feishu streaming card behavior and service health | completion |

### 1.4 Risk Assessment
- R1: Initial footer could expose malformed text when metadata is missing — mitigation: unit tests cover missing fields, empty labels, and separator cleanup.
- R2: Final footer could regress while adding streaming footer behavior — mitigation: non-streaming `composeRichStatusFooter(false, ...)` regression coverage keeps final footer unchanged.
- R3: Feishu card layout could render footer as body content instead of dim notation — mitigation: Feishu renderer tests assert `text_size: notation`, `text_color: grey`, and footer placement after an `hr` separator.
- R4: Session id could be unavailable on the first streaming update — mitigation: streaming footer is recomputed on later rebuilds and omits session until `CurrentSessionID()` becomes non-empty.

### 1.5 Rollback Strategy (high-level)
- Rollback trigger conditions: Feishu rich-card send/update failures increase, streaming cards render malformed footer content, or final footer content regresses after release.
- Rollback scope: code rollback only; no config, database, or data rollback is required.
- Rollback action: revert the cc-connect service release to the previous binary version through the existing service rollback path.
- Data reversibility: no data changes are made, so rollback has no data-reversal component.

### 1.6 Observability Plan
- Key metrics: cc-connect message send/update error rate, Feishu card patch/send failure logs, process error logs, and user-visible streaming card rendering reports.
- Key logs/alerts: warnings or errors around rich-card build/send/update failures, Feishu API failures, and unexpected empty/malformed footer reports.
- Observation windows: 15 minutes after release for send/update failures; 1 hour after release for user-visible card rendering feedback.

### 1.7 Communication
- Notify: cc-connect maintainers and users watching Feishu streaming card behavior.
- Timing: post-merge before service release, then post-release if any rollback trigger appears.
- Channels: existing cc-connect development or release coordination channel.

---

## Phase 2 · Release Runbook (v2)
> To be filled during finishing-a-development-branch Step 1.5.

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
