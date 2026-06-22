# Streaming Multi-Slot Rich Card Design

## Overview

Refactor cc-connect's Feishu tool call cards from full-card rebuild on every event to a multi-slot streaming skeleton architecture (modeled after happyclaw). Each panel has an independent `element_id` and its inner markdown is patched via `cardElement.content()`, eliminating full-card JSON rebuilds on tool events.

Additionally, tool return values are displayed inline in the tools timeline panel (truncated to 120 chars), not just input parameters.

**Verified constraint**: `cardElement.content()` CAN target markdown elements nested inside `collapsible_panel` — confirmed by happyclaw's production usage. Element IDs are global within the card. However, toggling `collapsible_panel.expanded` mid-stream via `cardElement.update()` is unreliable (Feishu's streaming_mode sometimes rejects structural rewrites). Panel expanded state must be set at card creation time and only changed during finalization (after streaming mode is disabled).

## Motivation

**Current problem**: Every `EventToolUse` / `EventToolResult` triggers a complete card JSON rebuild (~28KB), followed by a full-card API update. This causes:

- Visible flicker when panels are reconstructed
- Higher API payload and latency
- Throttling risk from frequent full-card updates
- No tool return value display — users can only see tool inputs

**Desired state**: Incremental slot-level updates — only the changed panel's markdown content is patched, no structural rebuild needed. Tool return values appear inline in the timeline.

## Architecture

### Card Skeleton (Streaming Mode)

```
Header (blue/green/red + status text)
Config: streaming_mode=true, update_multi=true, enable_forward_interaction=true
Body:
  1. status_banner   (markdown, element_id: "status_banner")     — phase banner, always visible
  2. thinking_panel  (collapsible_panel, element_id: "thinking_panel", expanded: true)
     └ thinking_md   (markdown, element_id: "thinking_md")      — thinking text
  3. tools_panel     (collapsible_panel, element_id: "tools_panel", expanded: false)
     └ tools_md      (markdown, element_id: "tools_md")         — tool timeline
  4. main_text       (markdown, element_id: "main_text")        — main reply (typewriter target)
  5. hr separator
  6. footer_note     (markdown, element_id: "footer_note")      — status footer
```

Panel expanded states are set at creation time and NOT changed during streaming. The thinking panel starts expanded (so the user sees thinking content as it arrives); the tools panel starts collapsed (user expands manually if interested). Both panels collapse when the card transitions to completed mode (after streaming mode is disabled).

### Card Skeleton (Completed Mode)

```
Header (violet "已完成" / orange "已中断" / red "出错")
Config: streaming_mode=false, update_multi=true, enable_forward_interaction=true
Body:
  1. main reply markdown
  2. hr separator
  3. tools_panel     (collapsible_panel, expanded: false) — aggregated tool counts
  4. thinking_panel  (collapsible_panel, expanded: false) — full thinking text
  5. footer metadata (duration, tokens, etc.)
```

### Flush Controller

The flush controller lives in `platform/feishu/` (Feishu-specific, not in core). It manages two flush tracks:

| Track | Interval | Target Slots | Trigger |
|-------|----------|-------------|---------|
| Text flush | 600ms / 30-char delta | `main_text` | Text streaming |
| Aux flush | 1500ms | `status_banner`, `thinking_md`, `tools_md`, `footer_note` | Tool/thinking events |

**Serialization**: Both tracks share a single serialized dispatch channel per card. The text track has higher priority (dispatched first when both fire simultaneously). All API calls are serialized through this channel to maintain monotonic sequence order.

**Dedup**: Each slot's last-rendered markdown is cached (fnv64 hash). Unchanged slots are skipped in the flush cycle.

**Event-driven flush**: Tool events and thinking events reset the aux flush timer (do not wait for the next tick). This ensures timely status banner updates.

## Interface Changes

### New: StreamingRichCardSupporter

`StreamingRichCardSupporter` does NOT embed `RichCardSupporter`. They are independent interfaces. The engine checks `StreamingRichCardSupporter` first; if unavailable, checks `RichCardSupporter`. This avoids forcing platforms to maintain two rendering implementations.

```go
// core/streaming.go

type StreamingSlotID string

const (
    SlotStatusBanner StreamingSlotID = "status_banner"
    SlotThinking     StreamingSlotID = "thinking_md"
    SlotTools        StreamingSlotID = "tools_md"
    SlotMainText     StreamingSlotID = "main_text"
    SlotFooterNote   StreamingSlotID = "footer_note"
)

// Sentinel errors for StreamSlotContent
var (
    ErrSlotRateLimited = errors.New("slot update rate limited")
    ErrSlotNotSupported = errors.New("slot update not supported (no cardEntity)")
    ErrSlotInvalidHandle = errors.New("invalid streaming card handle")
)

type StreamingRichCardSupporter interface {
    // BuildStreamingCard creates the initial multi-slot card skeleton, sends it
    // via the platform's PreviewStarter, and returns an opaque handle.
    // Returns ErrSlotNotSupported if cardEntity creation fails (engine falls back
    // to RichCardSupporter path).
    BuildStreamingCard(ctx context.Context, chatID string, status CardStatus, title string) (handle any, err error)

    // StreamSlotContent patches a single slot's markdown via cardElement.content().
    // The engine passes structured data via SlotContent; the platform renders it
    // into platform-specific markdown internally.
    // Returns ErrSlotRateLimited on rate limit (engine triggers degradation).
    // Returns ErrSlotNotSupported if cardEntity is unavailable.
    StreamSlotContent(ctx context.Context, handle any, slot StreamingSlotID, content SlotContent) error

    // FinalizeStreamingCard disables streaming mode, rebuilds the card as a
    // terminal (completed) card, and patches all slots to their terminal content
    // before switching the header to terminal status. Two-phase:
    //   Phase 1: Patch all slots to terminal content (still in streaming mode)
    //   Phase 2: Disable streaming mode + rebuild full card with completed layout + set header
    // If Phase 2 fails, the card content is still correct even if header shows wrong color.
    FinalizeStreamingCard(ctx context.Context, handle any, steps []ToolStep, markdown string, status CardStatus, statusFooter string) error
}

// SlotContent carries structured data for a slot update. The platform implementation
// is responsible for rendering this into platform-specific markdown (e.g. Feishu
// <text_tag>, <font> tags). Core never produces platform-specific markup.
type SlotContent struct {
    // StatusBanner fields
    Phase        string         // "thinking" | "tooling" | "streaming" | "done"
    Elapsed      time.Duration
    ActiveTool   string         // active tool name (for tooling phase)
    ToolSummary  string         // active tool input summary (for tooling phase)

    // ToolsTimeline fields
    ToolSteps []ToolStep

    // ThinkingContent fields
    ThinkingText string

    // MainText fields (used for SlotMainText)
    MainText string

    // FooterNote fields
    StatusFooter string
}
```

### ToolStep Extension

Add `Duration` and internal `startedAt` fields to `ToolStep`:

```go
type ToolStep struct {
    Kind      ToolStepKind
    Name      string
    Summary   string         // tool input summary
    Result    string         // tool return value (truncated)
    Status    string         // "running" | "complete" | "error"
    ExitCode  *int
    Success   *bool
    Done      bool
    Duration  time.Duration  // elapsed time (computed from startedAt on EventToolResult)
    startedAt time.Time      // internal: set on EventToolUse, used to compute Duration
}
```

Duration is calculated in the engine event loop: `startedAt` is set when `EventToolUse` arrives, and `Duration = time.Since(startedAt)` is computed when `EventToolResult` arrives.

### Engine Event Loop Changes

The engine event loop is restructured to prefer slot-level updates:

```go
// Pseudocode for event handling
if streamer, ok := p.(StreamingRichCardSupporter); ok && e.display.CardMode == "rich" {
    if previewHandle == nil {
        handle, err = streamer.BuildStreamingCard(ctx, chatID, status, title)
        if err != nil {
            // Fall back to RichCardSupporter path
            goto legacyRichCard
        }
    }
    // Update relevant slots with structured data — platform renders markdown
    streamer.StreamSlotContent(ctx, handle, SlotStatusBanner, SlotContent{Phase: derivePhase(toolSteps, hasTextContent), ...})
    streamer.StreamSlotContent(ctx, handle, SlotTools, SlotContent{ToolSteps: toolSteps})
    // ... etc
    // NOTE: StreamSlotContent(SlotMainText) REPLACES StreamRichCardText for
    // StreamingRichCardSupporter platforms. The two MUST NOT coexist — they
    // target the same element and share the sequence counter.
} else if richCard, ok := p.(RichCardSupporter); ok && e.display.CardMode == "rich" {
    // Existing full-card rebuild path (unchanged)
    // Also used as fallback when BuildStreamingCard returns ErrSlotNotSupported
}
```

Phase derivation: `derivePhase(toolSteps []ToolStep, hasTextContent bool) string`
- If any tool has `Done=false` → "tooling"
- Else if thinking steps exist and no tools → "thinking"
- Else if `hasTextContent` → "streaming"
- Else → "done"

`hasTextContent` is tracked in the engine: set to `true` when the first `EventText` arrives.

The actual markdown rendering (Feishu `<text_tag>`, `<font>`, etc.) lives in `platform/feishu/` — core never produces platform-specific markup.

## Panel Rendering Details (Feishu-specific)

All rendering functions below live in `platform/feishu/`. The engine passes structured `SlotContent` data; the Feishu platform implementation converts it to Feishu Card 2.0 markdown.

### Status Banner

Dynamic content based on current phase:

| Phase | Banner Text |
|-------|------------|
| Thinking | `💭 **思考中** · 12s` |
| Tooling | `🔧 **工具调用中** · \`Read\` (1.2s)` |
| Streaming | `✍️ **生成中** · 15s` |
| Done | `✅ **完成** · 23s` |

When thinking panel has no content yet (before first thinking token), the thinking panel is hidden entirely (empty collapsed panel is not rendered). It only appears once the first thinking text arrives.

### Tools Timeline (tools_md)

```
<text_tag color='blue'>运行</text_tag> `Bash` (2.3s)
  <font color='grey'>cmd: go test ./...</font>
<text_tag color='green'>完成</text_tag> `Read` (0.5s)
  <font color='grey'>path: core/engine.go</font>
  ↳ <font color='grey'>结果</font> `Read` 32 lines, 1200 bytes...
```

- Status tags: `blue` for running, `green` for complete, `red` for error
- Tool name in inline code; resolved via `buildToolDisplay()` for display name
- Elapsed time in parentheses
- Parameter summary: labeled key from tool descriptor's `ParamKeys`, truncated to 90 chars
- Return value: `↳ 结果 \`ToolName\` <truncated text, 120 chars>` on a new indented line
- Running tools sorted by start time (newest first), then completed tools sorted by completion time (newest first)
- Max 8 visible entries; overflow: `... 另有 N 条工具记录已收起`
- Per-slot content budget: max 4KB for `tools_md`. If exceeded, truncate oldest completed entries first.

### Thinking Panel

During streaming: `expanded: true`, content is thinking text blockquoted.

On completion: collapsed via full-card rebuild in `FinalizeStreamingCard` (after streaming mode is disabled).

### Completed Card Tools Panel

Aggregated counts only (no individual results):

```
🛠 工具调用 <number_tag>5</number_tag>
- `Read` <number_tag>2</number_tag>
- `Bash` <number_tag>1</number_tag>
- `Edit` <number_tag>1</number_tag>
- `Glob` <number_tag>1</number_tag>
```

Built by `buildCompletedCardJSON()` (renamed from `buildRichCardJSONBytes`), which is retained for the terminal card rebuild.

## API Degradation Chain

```
Level 0: cardElement.content() slot patch (preferred)
  ↓ on ErrSlotRateLimited from any slot
Level 1: cardkit-v1 full card update (fallback — rebuilds ALL slots at once)
  ↓ on cardkit API failure
Level 2: Im.Message.Patch (final fallback)
```

**Key rule**: On ANY slot-level failure (rate-limit or otherwise), immediately fall back to full-card rebuild for ALL slots — not just the failed one. This guarantees consistency. Once degraded to Level 1, stay at Level 1 for the rest of the turn (do not attempt Level 0 again).

If `BuildStreamingCard` returns `ErrSlotNotSupported` (cardEntity creation failed), the engine skips the streaming path entirely and uses the existing `RichCardSupporter` full-card rebuild path for the entire turn.

## Monitoring

Per-slot metrics logged via `slog` structured logging:

- Per-slot update counter: success / fail / degraded
- Flush cycle timing (text track / aux track)
- Dedup skip counter (how many slots were unchanged)
- Degradation level counter (L0 / L1 / L2)
- Slot content size (bytes) per slot

## Backward Compatibility

- Platforms not implementing `StreamingRichCardSupporter` continue using `RichCardSupporter` — zero change.
- `StreamingCardPlatform` (DingTalk-style) remains independent and unaffected.
- The `display.card_mode = "rich"` config key continues to work; new behavior is transparent.
- `StreamSlotContent(SlotMainText)` replaces `RichCardTextStreamer.StreamRichCardText` for platforms implementing `StreamingRichCardSupporter`. The engine must check `StreamingRichCardSupporter` first and only use `StreamRichCardText` when the platform does NOT implement the new interface.
- `buildRichCardJSONBytes()` is retained (renamed to `buildCompletedCardJSON()`) for the terminal card rebuild in `FinalizeStreamingCard`.

## Files Changed

| File | Change |
|------|--------|
| `core/streaming.go` | Add `StreamingSlotID`, `StreamingRichCardSupporter` interface (independent, NOT embedding `RichCardSupporter`), `SlotContent`, sentinel errors, `Duration`/`startedAt` on `ToolStep` |
| `core/engine.go` | Restructure event loop to prefer slot-level updates; add `derivePhase(toolSteps, hasTextContent)` pure function; pass structured `SlotContent` to platform; track `startedAt`/`Duration` on ToolStep; suppress `StreamRichCardText` when `StreamingRichCardSupporter` is active |
| `platform/feishu/feishu.go` | Implement `StreamingRichCardSupporter`; add `BuildStreamingCard()`, `StreamSlotContent()`, `FinalizeStreamingCard()`; add flush controller to `feishuPreviewHandle`; add `renderStatusBanner()`, `renderToolsTimeline()`, `renderThinkingContent()` rendering functions; rename `buildRichCardJSONBytes()` to `buildCompletedCardJSON()` |
| `core/engine_test.go` | Update tests for new event loop logic |
| `platform/feishu/feishu_test.go` | Add tests for slot rendering, degradation chain, timeline format |

## Implementation Order

1. Add `StreamingSlotID`, `StreamingRichCardSupporter` interface, `SlotContent`, sentinel errors to `core/streaming.go`
2. Add `Duration`/`startedAt` to `ToolStep` and populate in engine event loop
3. Add `derivePhase(toolSteps, hasTextContent)` in `core/engine.go`
4. Restructure engine event loop with `StreamingRichCardSupporter` priority (with fallback to `RichCardSupporter`)
5. Implement `BuildStreamingCard()` in `platform/feishu/feishu.go` — build skeleton JSON with config flags (`streaming_mode=true, update_multi=true`)
6. Implement `StreamSlotContent()` — render `SlotContent` to Feishu markdown, then `cardElement.content()` per slot
7. Implement `FinalizeStreamingCard()` — two-phase: patch terminal content then rebuild full card
8. Add flush controller (text 600ms, aux 1500ms) with serialized dispatch channel and slot dedup (fnv64)
9. Add API degradation chain (Level 0 → Level 1 on any slot failure, stay degraded for rest of turn)
10. Add i18n for all new user-facing strings
11. Add monitoring (per-slot counters, flush timing, dedup hits, degradation level)
12. Write tests
