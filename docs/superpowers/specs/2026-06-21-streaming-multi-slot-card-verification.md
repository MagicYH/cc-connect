# Streaming Multi-Slot Rich Card Verification

> Source spec: ./2026-06-21-streaming-multi-slot-card-design.md
> Used by: superpowers:writing-plans (TDD coverage) and post-implementation smoke testing.

## Environment & Access

| Item | Value | How to Obtain |
|---|---|---|
| Target environment | Local dev machine (Go build + test) | `go test ./...` from repo root |
| Repository | `/data00/home/chenhao.magic/Project/Source/Github/cc-connect` | Already checked out on branch `shadow` |
| Go version | Go 1.22+ | `go version` |
| Feishu test bot | Configured in `config.toml` under `[platform.feishu]` | Local config file; bot credentials from Feishu Open Platform |
| Feishu cardEntity API | `https://open.feishu.cn/open-apis/cardkit/v1/` | Feishu Open Platform docs; requires `app_id` + `app_secret` |
| Test chat ID | Configured in `config.toml` under `[platform.feishu.chat_ids]` | Feishu group chat for testing |
| Required env vars | `FEISHU_APP_ID`: Feishu app ID; `FEISHU_APP_SECRET`: Feishu app secret | Feishu Open Platform → app credentials |

## Public Operations

### Run all tests

Purpose: Execute full test suite to verify no regressions.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./...
```

### Run specific package tests with verbose output

Purpose: Targeted test execution during development.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./core/ -v -run TestDerivePhase
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -v -run TestStreamingCard
```

### Build the binary

Purpose: Verify compilation with selective build tags.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go build ./cmd/cc-connect
```

## Acceptance Criteria

- [ ] AC-1: Streaming card skeleton is created with correct element_ids and config flags (`streaming_mode=true`, `update_multi=true`) (covers spec §Architecture - Card Skeleton)
- [ ] AC-2: `StreamSlotContent` patches individual slots via `cardElement.content()` without rebuilding the full card (covers spec §Interface Changes - StreamingRichCardSupporter)
- [ ] AC-3: Tool return values are displayed inline in the tools timeline, truncated to 120 chars (covers spec §Panel Rendering Details - Tools Timeline)
- [ ] AC-4: Status banner reflects the correct phase (thinking/tooling/streaming/done) with elapsed time (covers spec §Panel Rendering Details - Status Banner)
- [ ] AC-5: `FinalizeStreamingCard` produces a completed card with collapsed panels, aggregated tool counts, and correct header color (covers spec §Architecture - Card Skeleton Completed Mode)
- [ ] AC-6: API degradation chain: Level 0 (slot patch) → Level 1 (full card update) → Level 2 (Im.Message.Patch), with one-way degradation per turn (covers spec §API Degradation Chain)
- [ ] AC-7: Flush controller dedupes unchanged slots and serializes API calls with text-priority dispatch (covers spec §Flush Controller)
- [ ] AC-8: `StreamingRichCardSupporter` does NOT embed `RichCardSupporter`; engine falls back to `RichCardSupporter` when streaming is unavailable (covers spec §Interface Changes, §Backward Compatibility)
- [ ] AC-9: `StreamSlotContent(SlotMainText)` replaces `StreamRichCardText` for streaming-capable platforms; the two never coexist (covers spec §Backward Compatibility)
- [ ] AC-10: `derivePhase` correctly derives phase from `toolSteps` and `hasTextContent` (covers spec §Engine Event Loop Changes)
- [ ] AC-11: `ToolStep.Duration` is computed from `startedAt` set on `EventToolUse` (covers spec §ToolStep Extension)
- [ ] AC-12: Platforms not implementing `StreamingRichCardSupporter` continue using `RichCardSupporter` with zero behavior change (covers spec §Backward Compatibility)

## Test Scenarios

### Scenario S1: Streaming card skeleton creation

**Verifies:** AC-1

**Execution:** AI-autonomous

**Preconditions:**
- Repository compiles: `go build ./...` succeeds
- `StreamingRichCardSupporter` interface is defined in `core/streaming.go`

**Steps:**
1. Write a unit test that calls `BuildStreamingCard(ctx, chatID, CardStatusThinking, "Thinking")` on the Feishu platform implementation.
   ```bash
   go test ./platform/feishu/ -v -run TestBuildStreamingCard
   ```
2. Assert the returned card JSON contains:
   - `"streaming_mode": true` in config
   - `"update_multi": true` in config
   - `element_id: "status_banner"` as a markdown element
   - `element_id: "thinking_panel"` as a collapsible_panel with `expanded: true`
   - `element_id: "thinking_md"` as a markdown nested inside thinking_panel
   - `element_id: "tools_panel"` as a collapsible_panel with `expanded: false`
   - `element_id: "tools_md"` as a markdown nested inside tools_panel
   - `element_id: "main_text"` as a markdown element
   - `element_id: "footer_note"` as a markdown element
3. Assert the returned handle is non-nil and of type `*feishuPreviewHandle` with a non-empty `cardID`.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| status | CardStatus | CardStatusThinking | concrete | initial status for card creation |
| title | string | "Thinking" | concrete | initial title |
| chatID | string | "test_chat_123" | concrete | test chat ID |

**Expected Results:**
- (must) Card JSON parses without error and contains all 7 element_ids listed above
- (must) `streaming_mode` is `true` in card config
- (must) `thinking_panel` has `expanded: true`, `tools_panel` has `expanded: false`
- (must) Handle is non-nil with non-empty `cardID`

**Failure Handling:**
- Card JSON missing element_ids: verify `BuildStreamingCard` builds the skeleton correctly — check the JSON construction function
- Handle has empty `cardID`: cardEntity creation may have failed — check mock/stub for cardkit API

---

### Scenario S2: Slot-level content patching

**Verifies:** AC-2, AC-4

**Execution:** AI-autonomous

**Preconditions:**
- `BuildStreamingCard` returns a valid handle (Scenario S1 passes)
- Mock/stub for `cardElement.content()` API is set up

**Steps:**
1. Create a streaming card via `BuildStreamingCard`.
2. Call `StreamSlotContent(ctx, handle, SlotStatusBanner, SlotContent{Phase: "tooling", Elapsed: 3*time.Second, ActiveTool: "Read", ToolSummary: "path: core/engine.go"})`.
3. Assert the `cardElement.content()` API was called with `element_id: "status_banner"` and content containing `🔧` and `Read`.
4. Call `StreamSlotContent(ctx, handle, SlotTools, SlotContent{ToolSteps: []ToolStep{{Kind: ToolStepKindTool, Name: "Read", Summary: "path: core/engine.go", Status: "running", Done: false}}})`.
5. Assert the `cardElement.content()` API was called with `element_id: "tools_md"` and content containing `<text_tag color='blue'>运行</text_tag>` and `Read`.
6. Assert no full-card rebuild API (`updateCardEntity` or `patchCardMessage`) was called.

```bash
go test ./platform/feishu/ -v -run TestStreamSlotContent
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| phase | string | "tooling" | concrete | status banner phase |
| elapsed | time.Duration | 3s | concrete | elapsed time for display |
| activeTool | string | "Read" | concrete | tool name for tooling phase banner |
| toolSummary | string | "path: core/engine.go" | concrete | tool input summary |

**Expected Results:**
- (must) `cardElement.content()` called exactly once for `status_banner` with correct element_id
- (must) `cardElement.content()` called exactly once for `tools_md` with correct element_id
- (must) Status banner content contains `🔧` and `Read` and `3s` (or `3.0s`)
- (must) Tools timeline content contains `运行` tag and `Read`
- (must) No full-card rebuild API calls (`updateCardEntity`, `patchCardMessage`) were made

**Failure Handling:**
- Full-card rebuild was called instead of slot patch: check that handle has non-empty `cardID` and `StreamSlotContent` routes to `cardElement.content()` path
- Slot content missing expected text: check rendering functions (`renderStatusBanner`, `renderToolsTimeline`)

---

### Scenario S3: Tool return value display in timeline

**Verifies:** AC-3

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card is active (handle has valid cardID)
- A tool has completed (EventToolResult received)

**Steps:**
1. Create streaming card and add a completed tool step:
   ```go
   steps := []ToolStep{
       {Kind: ToolStepKindTool, Name: "Read", Summary: "path: core/engine.go", Status: "complete", Done: true, Result: "32 lines, 1200 bytes of Go code with engine event loop logic", Duration: 500 * time.Millisecond},
   }
   ```
2. Call `StreamSlotContent(ctx, handle, SlotTools, SlotContent{ToolSteps: steps})`.
3. Assert rendered `tools_md` content contains `↳` (result indicator).
4. Assert result text contains `Read` (tool name) and is truncated (does not exceed 120 chars from the `Result` field).

```bash
go test ./platform/feishu/ -v -run TestToolsTimelineResult
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| result | string | "32 lines, 1200 bytes of Go code with engine event loop logic" | concrete | test return value |
| maxResultLen | integer | 120 | concrete | truncation limit per spec |

**Expected Results:**
- (must) Rendered `tools_md` contains `↳` followed by tool result text
- (must) Result text is truncated to at most 120 characters from the `Result` field
- (must) Completed tool entry has `<text_tag color='green'>完成</text_tag>` tag
- (must) Tool name appears in inline code format

**Failure Handling:**
- Result text not truncated: check `renderToolsTimeline` function for truncation logic
- `↳` not present: check that `ToolStep.Result` is non-empty and rendering handles it

---

### Scenario S4: Phase derivation logic

**Verifies:** AC-10

**Execution:** AI-autonomous

**Preconditions:**
- `derivePhase` function is defined in `core/engine.go`

**Steps:**
1. Test `derivePhase([]ToolStep{}, false)` → "done" (no activity)
2. Test `derivePhase([]ToolStep{{Kind: ToolStepKindThinking, Done: true}}, false)` → "thinking" (thinking steps, no tools, no text)
3. Test `derivePhase([]ToolStep{{Kind: ToolStepKindTool, Done: false, Name: "Read"}}, false)` → "tooling" (running tool)
4. Test `derivePhase([]ToolStep{{Kind: ToolStepKindTool, Done: true, Name: "Read"}}, true)` → "streaming" (completed tools + text streaming)
5. Test `derivePhase([]ToolStep{}, true)` → "streaming" (text streaming, no tool steps)

```bash
go test ./core/ -v -run TestDerivePhase
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| toolSteps | []ToolStep | (varies per sub-test) | concrete | see steps above |
| hasTextContent | bool | (varies per sub-test) | concrete | see steps above |

**Expected Results:**
- (must) Sub-test 1 returns "done"
- (must) Sub-test 2 returns "thinking"
- (must) Sub-test 3 returns "tooling" (running tool takes priority)
- (must) Sub-test 4 returns "streaming" (all tools done, text is streaming)
- (must) Sub-test 5 returns "streaming" (no tools, text is streaming)

**Failure Handling:**
- Incorrect phase: check priority ordering in `derivePhase` — tooling > thinking > streaming > done

---

### Scenario S5: ToolStep Duration calculation

**Verifies:** AC-11

**Execution:** AI-autonomous

**Preconditions:**
- Engine event loop tracks `startedAt` on `EventToolUse`

**Steps:**
1. Simulate `EventToolUse` for tool "Bash" — verify `ToolStep.startedAt` is set to current time and `Duration` is zero.
2. Simulate `EventToolResult` for tool "Bash" after 2.5s — verify `ToolStep.Duration` is approximately 2.5s (±100ms tolerance).
3. Verify `ToolStep.Done` is `true` after `EventToolResult`.

```bash
go test ./core/ -v -run TestToolStepDuration
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| toolName | string | "Bash" | concrete | test tool name |
| simulatedDelay | time.Duration | 2500ms | concrete | simulated time between use and result |

**Expected Results:**
- (must) After EventToolUse: `startedAt` is non-zero, `Duration` is zero, `Done` is false
- (must) After EventToolResult: `Duration` is within 100ms of 2.5s, `Done` is true

**Failure Handling:**
- Duration is zero after EventToolResult: check that engine computes `time.Since(startedAt)` and assigns it to `Duration`

---

### Scenario S6: FinalizeStreamingCard produces completed card

**Verifies:** AC-5

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card is active with thinking and tool steps
- Mock/stub for cardkit API and Im.Message.Patch is set up

**Steps:**
1. Create streaming card, add thinking steps and tool steps.
2. Call `FinalizeStreamingCard(ctx, handle, steps, "Final answer text", CardStatusDone, "3 tools used")`.
3. Assert the final card JSON contains:
   - `"streaming_mode": false` in config
   - Header with `template: "violet"` (done status)
   - Tools panel with `expanded: false` showing aggregated counts
   - Thinking panel with `expanded: false`
   - Main reply text present
4. Assert the cardkit `updateCardEntity` or `Im.Message.Patch` was called with the completed card JSON.

```bash
go test ./platform/feishu/ -v -run TestFinalizeStreamingCard
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| status | CardStatus | CardStatusDone | concrete | completion status |
| markdown | string | "Final answer text" | concrete | main reply content |
| statusFooter | string | "3 tools used" | concrete | footer text |

**Expected Results:**
- (must) Final card has `streaming_mode: false`
- (must) Header template is "violet" for done status
- (must) Both panels have `expanded: false`
- (must) Tools panel shows aggregated counts (e.g., `Read 2`) not individual results
- (must) Main reply text is present in card body

**Failure Handling:**
- streaming_mode still true: check that FinalizeStreamingCard Phase 2 disables streaming mode
- Panels still expanded: check that completed card builder sets `expanded: false`

---

### Scenario S7: API degradation from Level 0 to Level 1

**Verifies:** AC-6

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card is active with valid handle and cardID
- Mock can simulate rate-limit error from `cardElement.content()`

**Steps:**
1. Create streaming card successfully (Level 0 working).
2. Simulate `StreamSlotContent` returning `ErrSlotRateLimited` on the `tools_md` slot.
3. Assert the engine immediately falls back to `UpdateMessage` with a full card JSON (Level 1).
4. Send another tool event and assert the engine stays at Level 1 (does not attempt Level 0 again).

```bash
go test ./platform/feishu/ -v -run TestDegradationL0toL1
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| slot | StreamingSlotID | SlotTools ("tools_md") | concrete | slot that triggers rate limit |
| error | error | ErrSlotRateLimited | concrete | simulated rate limit error |

**Expected Results:**
- (must) After ErrSlotRateLimited, `UpdateMessage` (full card rebuild) is called
- (must) Subsequent slot updates use `UpdateMessage` instead of `StreamSlotContent`
- (must) Degradation is one-way for the rest of the turn (no L1 → L0 recovery)

**Failure Handling:**
- Engine retries Level 0 after degradation: check that the degraded flag is set and persisted for the turn

---

### Scenario S8: BuildStreamingCard failure falls back to RichCardSupporter

**Verifies:** AC-8, AC-12

**Execution:** AI-autonomous

**Preconditions:**
- Mock platform that does NOT implement `StreamingRichCardSupporter`
- Mock platform that implements `RichCardSupporter`

**Steps:**
1. Test with a platform that only implements `RichCardSupporter` (not `StreamingRichCardSupporter`).
2. Send an `EventToolUse` event.
3. Assert `BuildRichCard` is called (not `BuildStreamingCard`).
4. Assert the card is built via the existing full-card rebuild path.

```bash
go test ./core/ -v -run TestFallbackToRichCardSupporter
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| platform | Platform | stubRichCardOnly | concrete | stub that implements only RichCardSupporter |

**Expected Results:**
- (must) Engine calls `BuildRichCard` on the platform
- (must) No `StreamingRichCardSupporter` methods are called
- (must) Tool events are handled via full-card rebuild path

**Failure Handling:**
- Engine panics or errors: check that the type assertion `p.(StreamingRichCardSupporter)` returns false and the else-branch for `RichCardSupporter` is taken

---

### Scenario S9: StreamSlotContent replaces StreamRichCardText

**Verifies:** AC-9

**Execution:** AI-autonomous

**Preconditions:**
- Platform implements `StreamingRichCardSupporter`
- Engine receives text streaming events

**Steps:**
1. Create streaming card via `BuildStreamingCard`.
2. Send `EventText` events to the engine.
3. Assert `StreamSlotContent(ctx, handle, SlotMainText, ...)` is called.
4. Assert `StreamRichCardText` is NOT called (even though the platform also implements `RichCardTextStreamer`).

```bash
go test ./core/ -v -run TestMainTextSlotReplacesStreamRichCardText
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| slot | StreamingSlotID | SlotMainText ("main_text") | concrete | main text slot |
| mainText | string | "Hello, this is the answer." | concrete | test streaming text |

**Expected Results:**
- (must) `StreamSlotContent` is called with `SlotMainText` for text streaming
- (must) `StreamRichCardText` is NOT called
- (must) The `cardElement.content()` API targets `element_id: "main_text"`

**Failure Handling:**
- Both methods called: check engine logic — `StreamingRichCardSupporter` check must happen before `RichCardTextStreamer` check

---

### Scenario S10: Flush controller dedup and serialization

**Verifies:** AC-7

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card is active with valid handle
- Flush controller is implemented in `feishuPreviewHandle`

**Steps:**
1. Create streaming card.
2. Call `StreamSlotContent` for `SlotStatusBanner` with content A.
3. Call `StreamSlotContent` for `SlotStatusBanner` with content A again (same content).
4. Assert only one `cardElement.content()` API call was made for `status_banner` (dedup).
5. Call `StreamSlotContent` for `SlotStatusBanner` with content B (different).
6. Assert a second `cardElement.content()` API call was made (content changed).
7. Simulate concurrent text and aux flush — assert both are serialized (no concurrent API calls).

```bash
go test ./platform/feishu/ -v -run TestFlushControllerDedup
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| contentA | string | "💭 **思考中** · 5s" | concrete | first status banner content |
| contentB | string | "🔧 **工具调用中** · \`Read\` (1.2s)" | concrete | changed status banner content |

**Expected Results:**
- (must) Duplicate content A does not trigger a second API call
- (must) Changed content B triggers a new API call
- (must) Concurrent flush calls are serialized (no interleaving of API requests)

**Failure Handling:**
- Duplicate triggers API call: check fnv64 hash comparison in flush controller
- Concurrent calls not serialized: check dispatch channel implementation — should be single-consumer

---

### Scenario S11: Tools timeline with many tools (overflow and truncation)

**Verifies:** AC-3 (overflow handling)

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card is active

**Steps:**
1. Create 12 tool steps (4 running, 8 completed).
2. Call `StreamSlotContent(ctx, handle, SlotTools, SlotContent{ToolSteps: steps})`.
3. Assert rendered content shows at most 8 entries.
4. Assert overflow message appears: `另有 N 条工具记录已收起` where N >= 4.
5. Verify the 4KB content budget — rendered `tools_md` does not exceed 4096 bytes.

```bash
go test ./platform/feishu/ -v -run TestToolsTimelineOverflow
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| toolCount | integer | 12 | concrete | exceeds max 8 visible |
| maxVisible | integer | 8 | concrete | max entries per spec |
| maxBytes | integer | 4096 | concrete | per-slot content budget |

**Expected Results:**
- (must) At most 8 tool entries are rendered
- (must) Overflow message contains the count of hidden entries
- (must) Rendered `tools_md` content does not exceed 4096 bytes

**Failure Handling:**
- More than 8 entries rendered: check `renderToolsTimeline` truncation logic
- Content exceeds 4KB: check that oldest completed entries are truncated first

---

### Scenario S12: BuildStreamingCard returns ErrSlotNotSupported

**Verifies:** AC-6, AC-8

**Execution:** AI-autonomous

**Preconditions:**
- Mock simulates cardEntity creation failure (returns empty cardID)

**Steps:**
1. Call `BuildStreamingCard` on a mock where cardEntity creation fails.
2. Assert it returns `ErrSlotNotSupported`.
3. Assert the engine falls back to `RichCardSupporter.BuildRichCard` for the entire turn.
4. Send subsequent tool events and assert they all use the `RichCardSupporter` path.

```bash
go test ./core/ -v -run TestBuildStreamingCardNotSupported
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| error | error | ErrSlotNotSupported | concrete | cardEntity creation failure |

**Expected Results:**
- (must) `BuildStreamingCard` returns `ErrSlotNotSupported`
- (must) Engine falls back to `RichCardSupporter` path
- (must) Subsequent events in the same turn use `RichCardSupporter`, not `StreamingRichCardSupporter`

**Failure Handling:**
- Engine retries streaming after failure: check that the engine marks the turn as non-streaming and stays on the fallback path

---

### Scenario S13: FinalizeStreamingCard Phase 2 failure resilience

**Verifies:** AC-5 (two-phase finalization)

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card is active
- Mock simulates Phase 2 (full card rebuild) failure

**Steps:**
1. Call `FinalizeStreamingCard`.
2. Simulate Phase 1 (slot content patching) succeeds but Phase 2 (streaming mode disable + card rebuild) fails.
3. Assert that slot content was already patched to terminal values before Phase 2.
4. Assert the error from Phase 2 is logged but not fatal — the card content is correct even if header color is wrong.

```bash
go test ./platform/feishu/ -v -run TestFinalizeStreamingCardPhase2Failure
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| phase2Error | error | errors.New("cardkit update failed") | concrete | simulated Phase 2 failure |

**Expected Results:**
- (must) Phase 1 slot patches were applied (terminal content visible)
- (must) Phase 2 failure is logged via `slog.Error` or `slog.Warn`
- (must) The function returns the Phase 2 error (caller can decide to retry)

**Failure Handling:**
- Phase 1 not applied: check that FinalizeStreamingCard applies Phase 1 before attempting Phase 2

---

### Scenario S14: Empty thinking panel hidden before first token

**Verifies:** AC-4 (empty-panel handling)

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card just created, no thinking text received yet

**Steps:**
1. Create streaming card with `CardStatusThinking`.
2. Assert the thinking panel is NOT present in the card JSON (hidden when empty).
3. Send first thinking text via `StreamSlotContent(ctx, handle, SlotThinking, SlotContent{ThinkingText: "Analyzing the code..."})`.
4. Assert the thinking panel now appears in the card with `expanded: true`.

```bash
go test ./platform/feishu/ -v -run TestThinkingPanelEmptyHandling
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| initialThinkingText | string | "" | concrete | empty — panel should be hidden |
| firstThinkingText | string | "Analyzing the code..." | concrete | first token — panel appears |

**Expected Results:**
- (must) Initial card skeleton does NOT contain `thinking_panel` when thinking text is empty
- (must) After first thinking text, `thinking_panel` appears with `expanded: true`

**Failure Handling:**
- Empty thinking panel is rendered: check `BuildStreamingCard` to skip empty panels; or check that the skeleton always includes the panel but with empty content and `expanded: false`

---

### Scenario S15: Monitoring metrics emitted

**Verifies:** AC-7 (observability)

**Execution:** AI-autonomous

**Preconditions:**
- Streaming card is active
- `slog` output can be captured in tests

**Steps:**
1. Create streaming card, send tool events, finalize.
2. Assert `slog` output contains structured log entries for:
   - Slot update success/fail counters
   - Flush cycle timing
   - Dedup skip counter
   - Degradation level (L0/L1/L2)

```bash
go test ./platform/feishu/ -v -run TestStreamingCardMonitoring
```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| expectedMetrics | array<string> | ["slot_update_success", "flush_cycle_ms", "dedup_skip", "degradation_level"] | concrete | metric key names to verify |

**Expected Results:**
- (must) Log output contains at least one entry with `slot_update_success` or equivalent structured field
- (must) Log output contains at least one entry with `flush_cycle_ms` or equivalent
- (must) Log output contains degradation level indicator when degradation occurs

**Failure Handling:**
- Missing metric: check that the monitoring code uses `slog.Info`/`slog.Warn` with structured fields matching the expected key names

## Coverage Matrix

| Acceptance Criterion | Covered by Scenario |
|---|---|
| AC-1: Streaming card skeleton creation | S1 |
| AC-2: Slot-level content patching | S2 |
| AC-3: Tool return values inline display | S3, S11 |
| AC-4: Status banner phase + elapsed time | S2, S14 |
| AC-5: FinalizeStreamingCard completed card | S6, S13 |
| AC-6: API degradation chain L0→L1→L2 | S7, S12 |
| AC-7: Flush controller dedup + serialization | S10, S15 |
| AC-8: StreamingRichCardSupporter independent from RichCardSupporter | S8, S12 |
| AC-9: StreamSlotContent replaces StreamRichCardText | S9 |
| AC-10: derivePhase logic | S4 |
| AC-11: ToolStep Duration calculation | S5 |
| AC-12: Backward compatibility with RichCardSupporter | S8 |
