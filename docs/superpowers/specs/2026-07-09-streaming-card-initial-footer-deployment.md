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

### 2.1 Pre-Deploy Checklist
- [x] Local focused verification passed on branch `feat/streaming-card-initial-footer`.
- [x] Local full relevant package tests passed on branch `feat/streaming-card-initial-footer`.
- [x] Post-implementation correctness, quality, and security review completed with no high or medium issues remaining.
- [ ] Pull request or merge request created and CI green.
- [ ] Release owner confirms the existing cc-connect service release path for the target environment.

### 2.2 Executable Steps

#### Step 1: Confirm branch state
**Execution:** AI-autonomous
```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
git status --short --branch
git log --oneline --max-count=10
```
Expected: branch is `feat/streaming-card-initial-footer`; working tree is clean; commits include `b467593 fix: populate initial footer in streaming card skeleton` and `111c065 feat: show initial footer on streaming cards`.

#### Step 2: Run focused verification
**Execution:** AI-autonomous
```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir|TestProcessInteractiveEvents_BuildStreamingCardReceivesInitialFooter|TestBuildStreamingCardSkeletonIncludesInitialFooter' -count=1
```
Expected: both `github.com/chenhg5/cc-connect/core` and `github.com/chenhg5/cc-connect/platform/feishu` report `ok`.

#### Step 3: Run full relevant package tests
**Execution:** AI-autonomous
```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core ./platform/feishu -count=1
```
Expected: both `github.com/chenhg5/cc-connect/core` and `github.com/chenhg5/cc-connect/platform/feishu` report `ok`.

#### Step 4: Release through existing cc-connect service pipeline
**Execution:** Human-assisted — release owner selects the target environment and runs the established cc-connect service release pipeline after the branch is merged and CI is green.
No repository-local deploy command exists for this change; this feature has no database, config, dependency, or external API deployment step.

### 2.3 Post-Deploy Verification
Run these verification scenarios from `./2026-07-09-streaming-card-initial-footer-verification.md` against the released build:
- S1: Streaming footer composes model and cwd/session immediately.
- S2: Streaming footer recomputes when session id appears later.
- S3: Footer toggles preserve existing visibility semantics.
- S4: Final footer behavior remains unchanged.
- S5: Feishu renderer emits multi-line footer as notation blocks.

For a local smoke check before release, run:
```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir|TestProcessInteractiveEvents_BuildStreamingCardReceivesInitialFooter|TestBuildStreamingCardSkeletonIncludesInitialFooter' -count=1
```
Expected: command exits 0 and both packages report `ok`.

### 2.4 Concrete Rollback Commands

#### Roll back code changes before merge
**Execution:** AI-autonomous
```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
git revert --no-edit b467593 d791768 45f307c 111c065
```
Expected: git creates revert commits that remove the streaming footer implementation and tests while leaving planning docs intact.

#### Roll back after merge
**Execution:** AI-autonomous for git revert; Human-assisted for service release through the established cc-connect service pipeline.
```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
git revert --no-edit b467593 d791768 45f307c 111c065
go test ./core ./platform/feishu -count=1
```
Expected: revert commits are created and tests pass locally. After the revert branch is merged, the release owner redeploys cc-connect through the existing service release pipeline.

### 2.5 v1 Revisions Log
| Section | v1 description | v2 revision | Reason |
|---|---|---|---|
| 1.1 Release Scope | Affected components listed `core.Engine` and Feishu rich-card rendering tests | Added `StreamingRichCardSupporter` / Feishu multi-slot skeleton interface path as part of implementation scope | Final review found the primary Feishu multi-slot streaming card path also needed initial footer propagation. |
| 1.2 Infrastructure Touchpoints | No new API calls or extra Feishu patch calls | Confirmed unchanged: initial footer is passed into the existing initial card skeleton, not patched by a new API call | The implementation preserves the no-extra-network-operation constraint. |
| 1.4 Risk Assessment | R4 said session id may appear on later rebuilds | Added direct initial skeleton coverage for `BuildStreamingCard` so the first card can contain available metadata immediately | The actual Feishu path sends a multi-slot skeleton first; the final implementation seeds that skeleton directly. |
