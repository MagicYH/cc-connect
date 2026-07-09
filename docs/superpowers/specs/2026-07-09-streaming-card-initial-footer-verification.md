# Streaming Card Initial Footer Verification

> Source spec: ./2026-07-09-streaming-card-initial-footer-design.md
> Used by: superpowers:writing-plans (TDD coverage) and post-implementation smoke testing.

## Environment & Access

| Item | Value | How to Obtain |
|---|---|---|
| Target environment | Local Go test environment in `/data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer` | `git -C /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer status --short --branch` |
| Database | Not used | This feature is pure card composition/rendering; no database access is required. |
| Config namespace | Not used | Existing in-process engine toggles are set in Go tests; no TCC/config service is required. |
| Config key | Not used | Existing in-process engine toggles are set in Go tests; no external config key is required. |
| Service endpoint | Not used | Verification uses Go unit tests and does not call Feishu APIs. |
| Test accounts | Not used | Verification uses stub `AgentSession` and platform renderer tests; no Feishu user account is required. |
| Required env vars | None | Run tests from the repository worktree with the normal Go toolchain. |

A stranger can run every scenario with repository access and `go test`; no external authentication is required.

## Public Operations

### Run Focused Footer Tests

Purpose: Run only the tests that should be added or updated for this feature after implementation.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir' -count=1
```

### Run Full Relevant Package Tests

Purpose: Confirm the core engine and Feishu renderer packages still pass after the focused tests pass.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core ./platform/feishu -count=1
```

## Acceptance Criteria

- [ ] AC-1: Initial Feishu streaming rich cards show footer metadata immediately when model, workdir, or session values are available. (covers spec §Goal, §Data Flow)
- [ ] AC-2: Streaming footer is recomputed on later in-progress rich-card rebuilds, so a session id that becomes available after the first update can appear before finalization. (covers spec §Data Flow, §Risks and Mitigations)
- [ ] AC-3: Final Feishu cards keep the existing completion footer behavior and do not duplicate the initial footer. (covers spec §Scope, §Current Behavior)
- [ ] AC-4: Existing footer toggles continue to apply: master footer off disables all rich-card footer output, context/model visibility follows `showContextIndicator`, and workdir/session visibility follows `showWorkdirIndicator`. (covers spec §Rules)
- [ ] AC-5: Missing metadata does not produce malformed output, empty labels, or extra separators. (covers spec §Error Handling)
- [ ] AC-6: Feishu renders multi-line status footer text as dim notation footer blocks below the body. (covers spec §Feishu renderer)

## Test Scenarios

### Scenario S1: Streaming footer composes model and cwd/session immediately

**Verifies:** AC-1, AC-4

**Execution:** AI-autonomous

**Preconditions:**
- The repository worktree is `/data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer`.
- A Go unit test exists in `core/engine_test.go` for `composeRichStatusFooter(streaming=true, ...)` and the initial footer helper it delegates to.
- The test engine has `replyFooterEnabled=true`, `showContextIndicator=true`, and `showWorkdirIndicator=true`.
- The stub agent/session exposes model `claude-test-model`, workdir `/tmp/cc-connect-fixture`, and current session id `sess-stream-123`.

**Steps:**
1. Run the focused footer tests.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir' -count=1
   ```
   → If this fails, **stop** — the unit-level behavior is not verified.
2. Inspect the S1 test assertion in `core/engine_test.go`.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   grep -n "claude-test-model\|sess-stream-123\|cwd:" core/engine_test.go
   ```
   → If the grep returns no lines, **stop** — the test does not assert the requested initial metadata.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| model | string | claude-test-model | concrete | Stub value used only in local Go tests. |
| workdir | string, absolute path | /tmp/cc-connect-fixture | concrete | Stub value; no real directory creation required if the helper only formats strings. |
| session_id | string | sess-stream-123 | concrete | Stub `AgentSession.CurrentSessionID()` value. |
| reply_footer_enabled | boolean | true | concrete | Set on the test `Engine`. |
| show_context_indicator | boolean | true | concrete | Set on the test `Engine`. |
| show_workdir_indicator | boolean | true | concrete | Set on the test `Engine`. |

**Expected Results:**
- (must) Focused tests pass.
- (must) Streaming footer contains `model: claude-test-model`.
- (must) Streaming footer contains `cwd: /tmp/cc-connect-fixture · sess-stream-123` after path compaction rules are applied.
- (must) Streaming footer is non-empty while `streaming=true`.

**Failure Handling:**
- If only the model appears, verify `showWorkdirIndicator=true` and the test session returns `CurrentSessionID()`.
- If only cwd appears, verify the stub session id is non-empty and `replyFooterWorkDir` is reused.
- If the footer is empty, verify `replyFooterEnabled=true` and the streaming branch no longer returns early with `""`.

### Scenario S2: Streaming footer recomputes when session id appears later

**Verifies:** AC-2, AC-5

**Execution:** AI-autonomous

**Preconditions:**
- The repository worktree is `/data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer`.
- A mutable stub `AgentSession` can return an empty `CurrentSessionID()` for the first call and `sess-late-456` for a later call.
- The engine test has `replyFooterEnabled=true`, `showContextIndicator=true`, and `showWorkdirIndicator=true`.

**Steps:**
1. Run the focused footer tests.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir' -count=1
   ```
   → If this fails, **stop** — recomputation behavior is not verified.
2. Inspect the S2 test assertion in `core/engine_test.go`.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   grep -n "sess-late-456\|late session\|recompute" core/engine_test.go
   ```
   → If the grep returns no lines, **stop** — the test does not assert late session visibility.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| initial_session_id | string | empty string | concrete | First stub return value. |
| later_session_id | string | sess-late-456 | concrete | Later stub return value. |
| workdir | string, absolute path | /tmp/cc-connect-fixture | concrete | Stub value. |

**Expected Results:**
- (must) First streaming footer omits session without an empty `session:` label or trailing separator.
- (must) Later streaming footer includes `sess-late-456` without requiring a separate Feishu API call.
- (must) Both footer strings remain syntactically clean: no ` ·  · `, no trailing ` ·`, and no empty labels.

**Failure Handling:**
- If the later footer still omits the session id, verify the implementation recomputes from `session.CurrentSessionID()` on every streaming `composeRichStatusFooter` call instead of caching the first result.
- If malformed separators appear, verify the helper appends only non-empty segments.

### Scenario S3: Footer toggles preserve existing visibility semantics

**Verifies:** AC-4, AC-5

**Execution:** AI-autonomous

**Preconditions:**
- The repository worktree is `/data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer`.
- Core footer tests cover all three toggle combinations listed in Expected Results.

**Steps:**
1. Run focused footer tests.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir' -count=1
   ```
   → If this fails, **stop** — toggle semantics are not verified.
2. Inspect the toggle test assertions.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   grep -n "replyFooterEnabled\|showContextIndicator\|showWorkdirIndicator" core/engine_test.go
   ```
   → If the grep returns no lines, **stop** — the tests do not assert toggle behavior.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| reply_footer_enabled | boolean | false | concrete | Master-off subcase. |
| show_context_indicator | boolean | false | concrete | Model-hidden subcase. |
| show_workdir_indicator | boolean | false | concrete | Cwd/session-hidden subcase. |

**Expected Results:**
- (must) With `replyFooterEnabled=false`, streaming footer is empty even when model, cwd, and session are available.
- (must) With `showContextIndicator=false`, streaming footer does not include model text.
- (must) With `showWorkdirIndicator=false`, streaming footer does not include cwd or session text.
- (must) With `showContextIndicator=false` and `showWorkdirIndicator=true`, cwd/session may still appear because workdir visibility is independent of model visibility.

**Failure Handling:**
- If model ignores `showContextIndicator`, route model composition through the same flag used by final rich footer line 2.
- If cwd/session ignores `showWorkdirIndicator`, route workdir/session composition through the same flag used by final rich footer line 3.
- If master footer off still emits any text, keep the existing `replyFooterEnabled` guard before the streaming helper.

### Scenario S4: Final footer behavior remains unchanged

**Verifies:** AC-3, AC-4

**Execution:** AI-autonomous

**Preconditions:**
- The repository worktree is `/data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer`.
- A regression test compares `composeRichStatusFooter(streaming=false, ...)` output after the change.

**Steps:**
1. Run the focused footer tests.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir' -count=1
   ```
   → If this fails, **stop** — final footer regression behavior is not verified.
2. Run full relevant package tests.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   go test ./core ./platform/feishu -count=1
   ```
   → If this fails, **stop** — package-level behavior regressed.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| streaming | boolean | false | concrete | Final footer composition path. |
| workdir | string, absolute path | /tmp/cc-connect-fixture | concrete | Stub value. |
| session_id | string | sess-final-789 | concrete | Stub value expected on final workdir line. |

**Expected Results:**
- (must) Final footer still includes elapsed time.
- (must) Final footer still includes existing token/context/model line when context data is available.
- (must) Final footer still includes the workdir/session line through existing `replyFooterWorkDir` behavior.
- (must) Final footer does not include the initial labels `model:` or `cwd:` because the non-streaming branch remains unchanged.
- (must) Full `./core` and `./platform/feishu` package tests pass.

**Failure Handling:**
- If final footer loses elapsed or token/context details, verify the non-streaming branch stayed unchanged.
- If final footer duplicates initial metadata, verify finalization still calls only `composeRichStatusFooter(streaming=false, ...)` and does not append streaming footer text separately.

### Scenario S5: Feishu renderer emits multi-line footer as notation blocks

**Verifies:** AC-6

**Execution:** AI-autonomous

**Preconditions:**
- The repository worktree is `/data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer`.
- A Feishu renderer test exists in `platform/feishu/platform_test.go` for a non-empty multi-line `statusFooter`.

**Steps:**
1. Run the Feishu renderer footer test.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   go test ./platform/feishu -run 'TestBuildRichCard.*Footer' -count=1
   ```
   → If this fails, **stop** — Feishu footer rendering is not verified.
2. Inspect the renderer test assertion.
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
   grep -n "text_size.*notation\|text_color.*grey\|statusFooter" platform/feishu/platform_test.go
   ```
   → If the grep returns no lines, **stop** — the renderer test does not assert footer styling.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| status_footer | string, contains escaped newline | `model: claude-test-model\ncwd: /tmp/cc-connect-fixture · sess-stream-123` | concrete | Test input passed to `BuildRichCard`. |
| streaming | boolean | true | concrete | Renderer should include footer even on streaming cards. |

**Expected Results:**
- (must) Rendered Feishu card JSON contains a footer markdown block for `model: claude-test-model`.
- (must) Rendered Feishu card JSON contains a footer markdown block for `cwd: /tmp/cc-connect-fixture · sess-stream-123` after sanitizer/path expectations.
- (must) Footer blocks use `text_size: notation` and `text_color: grey`.
- (must) Footer blocks appear after the body markdown, separated by an `hr` element.

**Failure Handling:**
- If only one footer line appears, verify renderer splits `statusFooter` by the newline separator (`\n`).
- If styling is missing, verify footer elements are created through the existing footer block path, not body markdown.
- If streaming cards omit footer, verify `BuildRichCard(..., streaming=true, statusFooter)` does not gate footer rendering on `streaming=false`.

## Coverage Matrix

| Acceptance Criterion | Covered by Scenario |
|---|---|
| AC-1 | S1 |
| AC-2 | S2 |
| AC-3 | S4 |
| AC-4 | S1, S3, S4 |
| AC-5 | S2, S3 |
| AC-6 | S5 |
