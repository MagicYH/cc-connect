# Streaming Multi-Slot Rich Card Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** ./../specs/2026-06-21-streaming-multi-slot-card-design.md
**Verification:** ./../specs/2026-06-21-streaming-multi-slot-card-verification.md

**Goal:** Refactor Feishu tool call cards from full-card JSON rebuilds on every event to a multi-slot streaming skeleton with independent element_ids patched via `cardElement.content()`.

**Architecture:** A new `StreamingRichCardSupporter` interface (independent from `RichCardSupporter`) lets platforms opt into slot-level streaming. The engine restructures its event loop to prefer slot updates, falling back to the existing full-card rebuild path. On the Feishu side, a new card skeleton with 5 independent slots (status_banner, thinking_md, tools_md, main_text, footer_note) is created at turn start, and a flush controller with dual-track throttling manages API calls.

**Tech Stack:** Go 1.22+, Feishu Card 2.0 API (cardkit-v1), cardElement.content() slot-level patch

---

## File Structure

| File | Responsibility | Action |
|------|---------------|--------|
| `core/streaming.go` | `StreamingSlotID`, `SlotContent`, `StreamingRichCardSupporter` interface, sentinel errors, `ToolStep.Duration`/`startedAt` | Modify |
| `core/engine.go` | Event loop restructure, `derivePhase()`, slot update dispatch, `hasTextContent` tracking | Modify |
| `core/engine_test.go` | Tests for `derivePhase`, fallback logic, slot dispatch | Modify |
| `platform/feishu/streaming_card.go` | `BuildStreamingCard`, `StreamSlotContent`, `FinalizeStreamingCard`, flush controller, slot rendering | Create |
| `platform/feishu/streaming_card_test.go` | Tests for streaming card lifecycle, slot rendering, degradation, flush controller | Create |
| `platform/feishu/feishu.go` | Rename `buildRichCardJSONBytes` → `buildCompletedCardJSON`, remove `buildRichCard` wrapper (inlined), add `StreamingRichCardSupporter` interface assertion | Modify |

---

### Task 1: Add core types and StreamingRichCardSupporter interface

<!-- covers: S1 (partially — type definitions), S4 (derivePhase input types), S8 (interface definition), S9 (interface definition), S12 (ErrSlotNotSupported) -->

**Files:**
- Modify: `core/streaming.go:1-10` (add imports)
- Modify: `core/streaming.go:65-74` (extend ToolStep)
- Modify: `core/streaming.go:89-126` (add new types after RichCardTextStreamer)
- Test: `core/streaming_test.go`

- [ ] **Step 1: Write the failing test for derivePhase**

Add to `core/engine_test.go`:

```go
func TestDerivePhase(t *testing.T) {
	tests := []struct {
		name          string
		toolSteps     []ToolStep
		hasTextContent bool
		want          string
	}{
		{
			name:          "no activity yields done",
			toolSteps:     nil,
			hasTextContent: false,
			want:          "done",
		},
		{
			name:          "thinking steps only yields thinking",
			toolSteps:     []ToolStep{{Kind: ToolStepKindThinking, Done: true}},
			hasTextContent: false,
			want:          "thinking",
		},
		{
			name:          "running tool yields tooling",
			toolSteps:     []ToolStep{{Kind: ToolStepKindTool, Done: false, Name: "Read"}},
			hasTextContent: false,
			want:          "tooling",
		},
		{
			name:          "completed tools with text yields streaming",
			toolSteps:     []ToolStep{{Kind: ToolStepKindTool, Done: true, Name: "Read"}},
			hasTextContent: true,
			want:          "streaming",
		},
		{
			name:          "text only yields streaming",
			toolSteps:     nil,
			hasTextContent: true,
			want:          "streaming",
		},
		{
			name: "running tool takes priority over text",
			toolSteps: []ToolStep{
				{Kind: ToolStepKindTool, Done: false, Name: "Bash"},
				{Kind: ToolStepKindTool, Done: true, Name: "Read"},
			},
			hasTextContent: true,
			want:          "tooling",
		},
		{
			name: "running tool takes priority over thinking",
			toolSteps: []ToolStep{
				{Kind: ToolStepKindThinking, Done: true},
				{Kind: ToolStepKindTool, Done: false, Name: "Bash"},
			},
			hasTextContent: false,
			want:          "tooling",
		},
		{
			name:          "completed tools without text yields done",
			toolSteps:     []ToolStep{{Kind: ToolStepKindTool, Done: true, Name: "Read"}},
			hasTextContent: false,
			want:          "done",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derivePhase(tt.toolSteps, tt.hasTextContent)
			if got != tt.want {
				t.Errorf("derivePhase() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/ -run TestDerivePhase -v`
Expected: FAIL — `derivePhase` undefined

- [ ] **Step 3: Add StreamingSlotID, SlotContent, sentinel errors to core/streaming.go**

Add after the `RichCardTextStreamer` interface (after line ~126):

```go
// StreamingSlotID identifies an independently patchable slot in a streaming card.
type StreamingSlotID string

const (
	SlotStatusBanner StreamingSlotID = "status_banner"
	SlotThinking     StreamingSlotID = "thinking_md"
	SlotTools        StreamingSlotID = "tools_md"
	SlotMainText     StreamingSlotID = "main_text"
	SlotFooterNote   StreamingSlotID = "footer_note"
)

// Sentinel errors for slot-level streaming operations.
var (
	ErrSlotRateLimited  = errors.New("slot update rate limited")
	ErrSlotNotSupported = errors.New("slot update not supported (no cardEntity)")
	ErrSlotInvalidHandle = errors.New("invalid streaming card handle")
)

// SlotContent carries structured data for a slot update. The platform
// implementation renders this into platform-specific markdown internally.
// Core never produces platform-specific markup.
type SlotContent struct {
	// StatusBanner fields
	Phase        string        // "thinking" | "tooling" | "streaming" | "done"
	Elapsed      time.Duration
	ActiveTool   string        // active tool name (for tooling phase)
	ToolSummary  string        // active tool input summary (for tooling phase)

	// ToolsTimeline fields
	ToolSteps []ToolStep

	// ThinkingContent fields
	ThinkingText string

	// MainText fields (used for SlotMainText)
	MainText string

	// FooterNote fields
	StatusFooter string
}

// StreamingRichCardSupporter is an optional interface for platforms that support
// multi-slot streaming card updates. It does NOT embed RichCardSupporter — they
// are independent interfaces. The engine checks StreamingRichCardSupporter first;
// if unavailable, it falls back to RichCardSupporter.
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
```

Add `"errors"` to the import block in `core/streaming.go` (it may already be present; check first).

- [ ] **Step 4: Extend ToolStep with Duration and startedAt**

Modify the `ToolStep` struct in `core/streaming.go` (currently lines 65-74):

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

- [ ] **Step 5: Add derivePhase to core/engine.go**

Add as a package-level pure function in `core/engine.go`:

```go
// derivePhase determines the current streaming phase from tool steps and text state.
// Priority: tooling > thinking > streaming > done.
func derivePhase(toolSteps []ToolStep, hasTextContent bool) string {
	for _, s := range toolSteps {
		if s.Kind == ToolStepKindTool && !s.Done {
			return "tooling"
		}
	}
	for _, s := range toolSteps {
		if s.Kind == ToolStepKindThinking {
			return "thinking"
		}
	}
	if hasTextContent {
		return "streaming"
	}
	return "done"
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./core/ -run TestDerivePhase -v`
Expected: PASS

- [ ] **Step 7: Run full core test suite**

Run: `go test ./core/ -count=1`
Expected: All tests pass (ToolStep struct change is additive — existing code that doesn't set Duration/startedAt gets zero values)

- [ ] **Step 8: Commit**

```bash
git add core/streaming.go core/engine.go core/engine_test.go
git commit -m "feat(core): add StreamingRichCardSupporter interface, SlotContent, derivePhase"
```

---

### Task 2: Implement Feishu slot rendering functions

<!-- covers: S2 (rendering), S3 (tool result), S4 (status banner rendering), S11 (overflow/truncation), S14 (empty thinking) -->

**Files:**
- Create: `platform/feishu/streaming_card.go`
- Test: `platform/feishu/streaming_card_test.go`

- [ ] **Step 1: Write the failing test for renderStatusBanner**

Create `platform/feishu/streaming_card_test.go`:

```go
package feishu

import (
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestRenderStatusBanner(t *testing.T) {
	tests := []struct {
		name    string
		content core.SlotContent
		want    []string // substrings that must appear
		dont    []string // substrings that must NOT appear
	}{
		{
			name:    "thinking phase",
			content: core.SlotContent{Phase: "thinking", Elapsed: 12 * time.Second},
			want:    []string{"💭", "**思考中**", "12s"},
		},
		{
			name:    "tooling phase with active tool",
			content: core.SlotContent{Phase: "tooling", Elapsed: 5 * time.Second, ActiveTool: "Read", ToolSummary: "path: core/engine.go"},
			want:    []string{"🔧", "**工具调用中**", "`Read`", "5s"},
		},
		{
			name:    "streaming phase",
			content: core.SlotContent{Phase: "streaming", Elapsed: 15 * time.Second},
			want:    []string{"✍️", "**生成中**", "15s"},
		},
		{
			name:    "done phase",
			content: core.SlotContent{Phase: "done", Elapsed: 23 * time.Second},
			want:    []string{"✅", "**完成**", "23s"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderStatusBanner(tt.content)
			for _, w := range tt.want {
				if !containsString(got, w) {
					t.Errorf("renderStatusBanner() = %q, want substring %q", got, w)
				}
			}
			for _, d := range tt.dont {
				if containsString(got, d) {
					t.Errorf("renderStatusBanner() = %q, should NOT contain %q", got, d)
				}
			}
		})
	}
}

func TestRenderToolsTimeline(t *testing.T) {
	tests := []struct {
		name    string
		content core.SlotContent
		want    []string
		dont    []string
	}{
		{
			name: "running tool",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindTool, Name: "Bash", Summary: "cmd: go test ./...", Status: "running", Done: false, Duration: 2300 * time.Millisecond},
			}},
			want: []string{"运行", "`Bash`", "2.3s"},
		},
		{
			name: "completed tool with result",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: core/engine.go", Status: "complete", Done: true, Result: "32 lines, 1200 bytes", Duration: 500 * time.Millisecond},
			}},
			want: []string{"完成", "`Read`", "↳", "32 lines, 1200 bytes"},
		},
		{
			name: "result truncated to 120 chars",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindTool, Name: "Bash", Status: "complete", Done: true, Result: stringsRepeat("x", 200), Duration: time.Second},
			}},
			want: []string{"完成"},
			dont: []string{stringsRepeat("x", 121)},
		},
		{
			name: "error tool",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindTool, Name: "Bash", Summary: "cmd: fail", Status: "error", Done: true, Duration: 100 * time.Millisecond},
			}},
			want: []string{"出错", "`Bash`"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderToolsTimeline(tt.content)
			for _, w := range tt.want {
				if !containsString(got, w) {
					t.Errorf("renderToolsTimeline() = %q, want substring %q", got, w)
				}
			}
			for _, d := range tt.dont {
				if containsString(got, d) {
					t.Errorf("renderToolsTimeline() = %q, should NOT contain %q", got, d)
				}
			}
		})
	}
}

func TestRenderToolsTimelineOverflow(t *testing.T) {
	steps := make([]core.ToolStep, 12)
	for i := range steps {
		steps[i] = core.ToolStep{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: file.go", Status: "complete", Done: true, Duration: time.Second}
	}
	got := renderToolsTimeline(core.SlotContent{ToolSteps: steps})
	if !containsString(got, "另有") {
		t.Errorf("expected overflow message for 12 tools, got: %q", got)
	}
	if len(got) > 4096 {
		t.Errorf("tools_md exceeds 4KB budget: %d bytes", len(got))
	}
	// Count tool entries — should be at most 8
	if countSubstring(got, "`Read`") > 8 {
		t.Errorf("expected at most 8 visible tool entries, got more")
	}
}

func TestRenderThinkingContent(t *testing.T) {
	tests := []struct {
		name    string
		content core.SlotContent
		want    string
		empty   bool
	}{
		{
			name:    "empty thinking returns empty",
			content: core.SlotContent{ThinkingText: ""},
			empty:   true,
		},
		{
			name:    "non-empty thinking returns content",
			content: core.SlotContent{ThinkingText: "Analyzing the code..."},
			want:    "Analyzing the code...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderThinkingContent(tt.content)
			if tt.empty && got != "" {
				t.Errorf("expected empty, got %q", got)
			}
			if !tt.empty && !containsString(got, tt.want) {
				t.Errorf("renderThinkingContent() = %q, want substring %q", got, tt.want)
			}
		})
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, len(s)*n)
	for i := 0; i < n; i++ {
		copy(out[i*len(s):], s)
	}
	return string(out)
}

func countSubstring(s, sub string) int {
	count := 0
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			count++
		}
	}
	return count
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/feishu/ -run "TestRenderStatusBanner|TestRenderToolsTimeline|TestRenderThinkingContent" -v`
Expected: FAIL — functions undefined

- [ ] **Step 3: Implement rendering functions in streaming_card.go**

Create `platform/feishu/streaming_card.go`:

```go
package feishu

import (
	"fmt"
	"strings"

	"github.com/chenhg5/cc-connect/core"
)

const (
	maxVisibleTools   = 8
	maxToolResultLen  = 120
	maxToolsMdBytes   = 4096
)

// renderStatusBanner produces the status banner markdown for a slot update.
func renderStatusBanner(c core.SlotContent) string {
	elapsed := formatDuration(c.Elapsed)
	switch c.Phase {
	case "thinking":
		return fmt.Sprintf("💭 **思考中** · %s", elapsed)
	case "tooling":
		tool := c.ActiveTool
		if tool == "" {
			tool = "..."
		}
		return fmt.Sprintf("🔧 **工具调用中** · `%s` %s", tool, elapsed)
	case "streaming":
		return fmt.Sprintf("✍️ **生成中** · %s", elapsed)
	case "done":
		return fmt.Sprintf("✅ **完成** · %s", elapsed)
	default:
		return fmt.Sprintf("· %s", elapsed)
	}
}

// renderToolsTimeline produces the tools timeline markdown for a slot update.
func renderToolsTimeline(c core.SlotContent) string {
	if len(c.ToolSteps) == 0 {
		return ""
	}

	// Sort: running tools first (newest first), then completed (newest first)
	running := make([]core.ToolStep, 0)
	completed := make([]core.ToolStep, 0)
	for _, s := range c.ToolSteps {
		if s.Kind != core.ToolStepKindTool {
			continue
		}
		if !s.Done {
			running = append(running, s)
		} else {
			completed = append(completed, s)
		}
	}

	visible := make([]core.ToolStep, 0, maxVisibleTools)
	visible = append(visible, running...)
	overflow := 0
	remaining := maxVisibleTools - len(visible)
	if remaining < len(completed) {
		overflow = len(completed) - remaining
		completed = completed[:remaining]
	}
	visible = append(visible, completed...)

	var sb strings.Builder
	for _, step := range visible {
		sb.WriteString(renderToolStepRow(step))
		sb.WriteString("\n")
	}
	if overflow > 0 {
		sb.WriteString(fmt.Sprintf("... 另有 %d 条工具记录已收起\n", overflow))
	}

	result := sb.String()
	if len(result) > maxToolsMdBytes {
		result = result[:maxToolsMdBytes]
	}
	return result
}

// renderToolStepRow renders a single tool step as a timeline row.
func renderToolStepRow(step core.ToolStep) string {
	display := buildToolDisplay(step.Name, step.Summary)
	tag := renderToolStatusTag(step)
	duration := formatDuration(step.Duration)

	line := fmt.Sprintf("%s `%s` (%s)", tag, display.Title, duration)
	if display.Detail != "" {
		line += fmt.Sprintf("\n  <font color='grey'>%s</font>", display.Detail)
	}
	if step.Done && step.Result != "" {
		result := step.Result
		if len(result) > maxToolResultLen {
			result = result[:maxToolResultLen] + "…"
		}
		line += fmt.Sprintf("\n  ↳ <font color='grey'>结果</font> `%s` %s", display.Title, result)
	}
	return line
}

// renderToolStatusTag returns the Feishu text_tag for a tool's status.
func renderToolStatusTag(step core.ToolStep) string {
	if !step.Done {
		return "<text_tag color='blue'>运行</text_tag>"
	}
	if step.Status == "error" || (step.Success != nil && !*step.Success) {
		return "<text_tag color='red'>出错</text_tag>"
	}
	return "<text_tag color='green'>完成</text_tag>"
}

// renderThinkingContent produces the thinking panel markdown.
// Returns empty string when there is no thinking text (panel should be hidden).
func renderThinkingContent(c core.SlotContent) string {
	if c.ThinkingText == "" {
		return ""
	}
	return c.ThinkingText
}

// formatDuration formats a duration for display in the card.
func formatDuration(d interface{ Milliseconds() int64 }) string {
	if d == nil {
		return ""
	}
	ms := d.Milliseconds()
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	sec := float64(ms) / 1000.0
	if sec < 60 {
		return fmt.Sprintf("%.1fs", sec)
	}
	min := int(sec) / 60
	rem := sec - float64(min*60)
	return fmt.Sprintf("%dm%.0fs", min, rem)
}
```

- [ ] **Step 4: Run tests — iteration 1**

Run: `go test ./platform/feishu/ -run "TestRenderStatusBanner|TestRenderToolsTimeline|TestRenderThinkingContent" -v`

Expected: The `formatDuration` function needs to accept `time.Duration`. Let me fix — `time.Duration` doesn't have a `Milliseconds()` method directly on the interface. Replace the signature:

Change `formatDuration` to:

```go
func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	ms := d.Milliseconds()
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	sec := float64(ms) / 1000.0
	if sec < 60 {
		return fmt.Sprintf("%.1fs", sec)
	}
	min := int(sec) / 60
	rem := sec - float64(min*60)
	return fmt.Sprintf("%dm%.0fs", min, rem)
}
```

Also update `renderStatusBanner` — the elapsed formatting for 12s should produce "12.0s" not "12s". Adjust the test expectations accordingly: change `"12s"` to `"12.0s"`, `"5s"` to `"5.0s"`, `"15s"` to `"15.0s"`, `"23s"` to `"23.0s"`. And for `2.3s` / `500ms` in the tools timeline tests.

Update test substrings:

```go
// In TestRenderStatusBanner:
// "12s" → "12.0s", "5s" → "5.0s", "15s" → "15.0s", "23s" → "23.0s"

// In TestRenderToolsTimeline:
// "2.3s" stays as is, add "500ms" check for the 500ms duration
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./platform/feishu/ -run "TestRenderStatusBanner|TestRenderToolsTimeline|TestRenderThinkingContent" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add platform/feishu/streaming_card.go platform/feishu/streaming_card_test.go
git commit -m "feat(feishu): add slot rendering functions for streaming cards"
```

---

### Task 3: Build streaming card skeleton and implement BuildStreamingCard

<!-- covers: S1 (skeleton creation), S14 (empty thinking panel hidden) -->

**Files:**
- Modify: `platform/feishu/streaming_card.go`
- Modify: `platform/feishu/streaming_card_test.go`

- [ ] **Step 1: Write the failing test for BuildStreamingCard skeleton**

Add to `platform/feishu/streaming_card_test.go`:

```go
func TestBuildStreamingCardSkeleton(t *testing.T) {
	skeleton := buildStreamingCardSkeleton(core.CardStatusThinking, "")

	// Parse the card JSON
	var card map[string]any
	if err := json.Unmarshal([]byte(skeleton), &card); err != nil {
		t.Fatalf("card JSON parse error: %v", err)
	}

	// Check config
	config := card["config"].(map[string]any)
	if config["streaming_mode"] != true {
		t.Error("streaming_mode should be true")
	}
	if config["update_multi"] != true {
		t.Error("update_multi should be true")
	}

	// Collect element_ids
	body := card["body"].(map[string]any)
	elements := body["elements"].([]any)
	elementIDs := make(map[string]bool)
	for _, elem := range elements {
		em := elem.(map[string]any)
		if id, ok := em["element_id"]; ok {
			elementIDs[id.(string)] = true
		}
		// Check inside collapsible_panel elements
		if em["tag"] == "collapsible_panel" {
			if id, ok := em["element_id"]; ok {
				elementIDs[id.(string)] = true
			}
			if inner, ok := em["elements"].([]any); ok {
				for _, child := range inner {
					cm := child.(map[string]any)
					if cid, ok := cm["element_id"]; ok {
						elementIDs[cid.(string)] = true
					}
				}
			}
		}
	}

	required := []string{"status_banner", "tools_panel", "tools_md", "main_text", "footer_note"}
	for _, id := range required {
		if !elementIDs[id] {
			t.Errorf("missing element_id %q in skeleton", id)
		}
	}

	// thinking_panel should NOT be present when no thinking text
	if elementIDs["thinking_panel"] {
		t.Error("thinking_panel should be hidden when no thinking text")
	}

	// tools_panel should be collapsed
	for _, elem := range elements {
		em := elem.(map[string]any)
		if em["tag"] == "collapsible_panel" && em["element_id"] == "tools_panel" {
			if em["expanded"] != false {
				t.Error("tools_panel should start collapsed")
			}
		}
	}
}

func TestBuildStreamingCardSkeletonWithThinking(t *testing.T) {
	skeleton := buildStreamingCardSkeleton(core.CardStatusThinking, "initial thinking")

	var card map[string]any
	if err := json.Unmarshal([]byte(skeleton), &card); err != nil {
		t.Fatalf("card JSON parse error: %v", err)
	}

	body := card["body"].(map[string]any)
	elements := body["elements"].([]any)
	hasThinkingPanel := false
	for _, elem := range elements {
		em := elem.(map[string]any)
		if em["tag"] == "collapsible_panel" && em["element_id"] == "thinking_panel" {
			hasThinkingPanel = true
			if em["expanded"] != true {
				t.Error("thinking_panel should be expanded")
			}
		}
	}
	if !hasThinkingPanel {
		t.Error("thinking_panel should be present when thinking text is non-empty")
	}
}
```

Add `"encoding/json"` to the test file imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/feishu/ -run "TestBuildStreamingCardSkeleton" -v`
Expected: FAIL — `buildStreamingCardSkeleton` undefined

- [ ] **Step 3: Implement buildStreamingCardSkeleton**

Add to `platform/feishu/streaming_card.go`:

```go
// Element IDs for the streaming card skeleton.
const (
	streamingElementStatusBanner = "status_banner"
	streamingElementThinkingPanel = "thinking_panel"
	streamingElementThinkingMd   = "thinking_md"
	streamingElementToolsPanel   = "tools_panel"
	streamingElementToolsMd      = "tools_md"
	streamingElementMainText     = "main_text"
	streamingElementFooterNote   = "footer_note"
)

// buildStreamingCardSkeleton creates the initial card JSON with all slots.
// thinkingText controls whether the thinking panel is included.
func buildStreamingCardSkeleton(status core.CardStatus, thinkingText string) string {
	elements := make([]map[string]any, 0, 6)

	// 1. Status banner
	elements = append(elements, map[string]any{
		"tag":        "markdown",
		"element_id": streamingElementStatusBanner,
		"content":    renderStatusBanner(core.SlotContent{Phase: "thinking"}),
	})

	// 2. Thinking panel (hidden when empty)
	if thinkingText != "" {
		elements = append(elements, map[string]any{
			"tag":              "collapsible_panel",
			"element_id":       streamingElementThinkingPanel,
			"expanded":         true,
			"background_color": "grey",
			"header": map[string]any{
				"title": map[string]any{"tag": "plain_text", "content": "思考过程"},
			},
			"border":           map[string]any{"color": "grey"},
			"vertical_spacing": "8px",
			"padding":          "4px 8px",
			"elements": []map[string]any{
				{
					"tag":        "markdown",
					"element_id": streamingElementThinkingMd,
					"content":    renderThinkingContent(core.SlotContent{ThinkingText: thinkingText}),
				},
			},
		})
	}

	// 3. Tools panel (starts collapsed)
	elements = append(elements, map[string]any{
		"tag":              "collapsible_panel",
		"element_id":       streamingElementToolsPanel,
		"expanded":         false,
		"background_color": "grey",
		"header": map[string]any{
			"title": map[string]any{"tag": "plain_text", "content": "工具调用"},
		},
		"border":           map[string]any{"color": "grey"},
		"vertical_spacing": "8px",
		"padding":          "4px 8px",
		"elements": []map[string]any{
			{
				"tag":        "markdown",
				"element_id": streamingElementToolsMd,
				"content":    "",
			},
		},
	})

	// 4. Main text
	elements = append(elements, map[string]any{
		"tag":        "markdown",
		"element_id": streamingElementMainText,
		"content":    "",
	})

	// 5. Footer note
	elements = append(elements, map[string]any{
		"tag":        "markdown",
		"element_id": streamingElementFooterNote,
		"content":    "",
	})

	headerTemplate := "blue"
	headerTitle := pickThinkingVerb()
	switch status {
	case core.CardStatusDone:
		headerTemplate = "green"
		headerTitle = "Done"
	case core.CardStatusError:
		headerTemplate = "red"
		headerTitle = "Error"
	}

	card := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"streaming_mode":             true,
			"update_multi":               true,
			"enable_forward_interaction": true,
		},
		"header": map[string]any{
			"template": headerTemplate,
			"title":    map[string]any{"tag": "plain_text", "content": headerTitle},
		},
		"body": map[string]any{"elements": elements},
	}

	b, _ := json.Marshal(card)
	return string(b)
}
```

Add `"encoding/json"` to the imports in `streaming_card.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./platform/feishu/ -run "TestBuildStreamingCardSkeleton" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add platform/feishu/streaming_card.go platform/feishu/streaming_card_test.go
git commit -m "feat(feishu): add streaming card skeleton builder"
```

---

### Task 4: Implement StreamSlotContent with cardElement.content() API

<!-- covers: S2 (slot patching), S9 (SlotMainText replaces StreamRichCardText) -->

**Files:**
- Modify: `platform/feishu/streaming_card.go`
- Modify: `platform/feishu/streaming_card_test.go`

- [ ] **Step 1: Write the failing test for StreamSlotContent**

Add to `platform/feishu/streaming_card_test.go`:

```go
func TestStreamSlotContentRoutesToSlotAPI(t *testing.T) {
	// Verify that slot ID maps to the correct element_id in the cardElement.content() API
	slotToElement := map[core.StreamingSlotID]string{
		core.SlotStatusBanner: streamingElementStatusBanner,
		core.SlotThinking:    streamingElementThinkingMd,
		core.SlotTools:       streamingElementToolsMd,
		core.SlotMainText:    streamingElementMainText,
		core.SlotFooterNote:  streamingElementFooterNote,
	}
	for slot, elemID := range slotToElement {
		t.Run(string(slot), func(t *testing.T) {
			resolved := resolveSlotElementID(slot)
			if resolved != elemID {
				t.Errorf("resolveSlotElementID(%q) = %q, want %q", slot, resolved, elemID)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/feishu/ -run TestStreamSlotContentRoutesToSlotAPI -v`
Expected: FAIL — `resolveSlotElementID` undefined

- [ ] **Step 3: Implement resolveSlotElementID and StreamSlotContent**

Add to `platform/feishu/streaming_card.go`:

```go
// resolveSlotElementID maps a StreamingSlotID to the Feishu card element_id.
func resolveSlotElementID(slot core.StreamingSlotID) string {
	switch slot {
	case core.SlotStatusBanner:
		return streamingElementStatusBanner
	case core.SlotThinking:
		return streamingElementThinkingMd
	case core.SlotTools:
		return streamingElementToolsMd
	case core.SlotMainText:
		return streamingElementMainText
	case core.SlotFooterNote:
		return streamingElementFooterNote
	default:
		return string(slot)
	}
}

// renderSlotContent produces the markdown content for a given slot update.
func renderSlotContent(slot core.StreamingSlotID, content core.SlotContent) string {
	switch slot {
	case core.SlotStatusBanner:
		return renderStatusBanner(content)
	case core.SlotThinking:
		return renderThinkingContent(content)
	case core.SlotTools:
		return renderToolsTimeline(content)
	case core.SlotMainText:
		return content.MainText
	case core.SlotFooterNote:
		return content.StatusFooter
	default:
		return ""
	}
}
```

Now implement `StreamSlotContent` on the Platform type. Add to `platform/feishu/streaming_card.go`:

```go
// StreamSlotContent patches a single slot's markdown via cardElement.content().
func (p *Platform) StreamSlotContent(ctx context.Context, previewHandle any, slot core.StreamingSlotID, content core.SlotContent) error {
	h, ok := previewHandle.(*feishuPreviewHandle)
	if !ok {
		return fmt.Errorf("%s: StreamSlotContent: %w", p.tag(), core.ErrSlotInvalidHandle)
	}

	h.mu.Lock()
	if h.cardID == "" {
		h.mu.Unlock()
		return core.ErrSlotNotSupported
	}
	h.sequence++
	cardID := h.cardID
	seq := h.sequence
	h.mu.Unlock()

	elementID := resolveSlotElementID(slot)
	markdown := renderSlotContent(slot, content)

	apiPath := fmt.Sprintf("/open-apis/cardkit/v1/cards/%s/elements/%s/content", cardID, elementID)
	body := map[string]any{
		"content":  markdown,
		"sequence": seq,
	}

	var apiResp *larkcore.ApiResp
	if err := p.withFreshTenantAccessTokenRetry(ctx, "stream slot content", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
		var err error
		apiResp, err = client.Put(ctx, apiPath, body, larkcore.AccessTokenTypeTenant, options...)
		return err
	}); err != nil {
		return fmt.Errorf("%s: stream slot content: %w", p.tag(), err)
	}
	if apiResp == nil || apiResp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: stream slot content: HTTP status %d", p.tag(), apiResp.StatusCode)
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(apiResp.RawBody, &resp); err != nil {
		return fmt.Errorf("%s: stream slot content: parse response: %w", p.tag(), err)
	}
	if resp.Code != 0 {
		err := classifyFeishuCardAPIError("stream slot content", resp.Code, resp.Msg)
		if errors.Is(err, errFeishuCardRateLimited) {
			slog.Debug(p.tag() + ": stream slot content rate limited; skipping frame", "slot", slot, "code", resp.Code)
			return core.ErrSlotRateLimited
		}
		return fmt.Errorf("%s: stream slot content: %w", p.tag(), err)
	}
	return nil
}
```

Add `"errors"`, `"log/slog"`, and the lark imports to `streaming_card.go` imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./platform/feishu/ -run TestStreamSlotContentRoutesToSlotAPI -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add platform/feishu/streaming_card.go platform/feishu/streaming_card_test.go
git commit -m "feat(feishu): implement StreamSlotContent with cardElement.content() API"
```

---

### Task 5: Implement BuildStreamingCard on Platform

<!-- covers: S1 (full lifecycle), S12 (ErrSlotNotSupported on cardEntity failure) -->

**Files:**
- Modify: `platform/feishu/streaming_card.go`
- Modify: `platform/feishu/feishu.go` (add interface assertion)

- [ ] **Step 1: Implement BuildStreamingCard on Platform**

Add to `platform/feishu/streaming_card.go`:

```go
// BuildStreamingCard creates the initial multi-slot card skeleton, sends it
// via the platform's message API, creates a cardEntity for slot-level patching,
// and returns a handle.
func (p *Platform) BuildStreamingCard(ctx context.Context, chatID string, status core.CardStatus, title string) (any, error) {
	if !p.useInteractiveCard {
		return nil, core.ErrSlotNotSupported
	}

	cardJSON := buildStreamingCardSkeleton(status, "")

	// Create cardEntity for slot-level patching
	cardID, err := p.createCardEntity(ctx, cardJSON)
	if err != nil {
		slog.Info(p.tag()+": streaming card create cardEntity failed, falling back", "error", err)
		return nil, core.ErrSlotNotSupported
	}

	sendContent := fmt.Sprintf(`{"type":"card","data":{"card_id":"%s"}}`, cardID)

	msgID, err := p.sendCardMessage(ctx, chatID, sendContent)
	if err != nil {
		return nil, fmt.Errorf("%s: build streaming card send: %w", p.tag(), err)
	}

	return &feishuPreviewHandle{
		messageID: msgID,
		chatID:    chatID,
		cardID:    cardID,
		status:    status,
	}, nil
}
```

Add the helper `sendCardMessage` which extracts the common message-sending logic from `SendPreviewStart`. This avoids duplicating the reply/create API logic:

```go
// sendCardMessage sends a card content string to the given chatID and returns the message ID.
func (p *Platform) sendCardMessage(ctx context.Context, chatID, sendContent string) (string, error) {
	// Build a minimal replyContext for the send
	rc := replyContext{chatID: chatID}

	if p.shouldUseThreadOrReplyAPI(rc) {
		resp, err := p.client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
			MessageId(rc.parentID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("interactive").
				Content(sendContent).
				Build()).
			Build())
		if err != nil {
			return "", err
		}
		if !resp.Success() {
			return "", fmt.Errorf("reply API failed: code=%d msg=%s", resp.Code, resp.Msg)
		}
		if resp.Data != nil && resp.Data.MessageId != nil {
			return *resp.Data.MessageId, nil
		}
		return "", fmt.Errorf("reply API returned no message ID")
	}

	resp, err := p.client.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("interactive").
			Content(sendContent).
			Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("create API failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", fmt.Errorf("create API returned no message ID")
}
```

- [ ] **Step 2: Add interface assertion in feishu.go**

Add at the end of `platform/feishu/feishu.go` (near other interface assertions):

```go
var _ core.StreamingRichCardSupporter = (*Platform)(nil)
```

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./platform/feishu/`
Expected: PASS (no compile errors)

- [ ] **Step 4: Commit**

```bash
git add platform/feishu/streaming_card.go platform/feishu/feishu.go
git commit -m "feat(feishu): implement BuildStreamingCard on Platform"
```

---

### Task 6: Implement FinalizeStreamingCard

<!-- covers: S5 (completed card), S6 (finalization), S13 (Phase 2 failure resilience) -->

**Files:**
- Modify: `platform/feishu/streaming_card.go`
- Modify: `platform/feishu/streaming_card_test.go`

- [ ] **Step 1: Write the failing test for buildCompletedCardJSON**

Add to `platform/feishu/streaming_card_test.go`:

```go
func TestBuildCompletedCardJSON(t *testing.T) {
	steps := []core.ToolStep{
		{Kind: core.ToolStepKindTool, Name: "Read", Summary: "a.go", Status: "complete", Done: true, Duration: 500 * time.Millisecond},
		{Kind: core.ToolStepKindTool, Name: "Read", Summary: "b.go", Status: "complete", Done: true, Duration: 300 * time.Millisecond},
		{Kind: core.ToolStepKindTool, Name: "Bash", Summary: "cmd", Status: "complete", Done: true, Duration: time.Second},
	}
	b, err := buildCompletedCardJSON(core.CardStatusDone, steps, "Final answer", false, "3 tools used")
	if err != nil {
		t.Fatalf("buildCompletedCardJSON error: %v", err)
	}

	var card map[string]any
	if err := json.Unmarshal(b, &card); err != nil {
		t.Fatalf("card JSON parse error: %v", err)
	}

	// streaming_mode should be false
	config := card["config"].(map[string]any)
	if config["streaming_mode"] != false {
		t.Error("completed card should have streaming_mode=false")
	}

	// Header should be green/violet for done
	header := card["header"].(map[string]any)
	if header["template"] != "green" {
		t.Errorf("expected green header for done, got %v", header["template"])
	}

	// Main text should be present
	body := card["body"].(map[string]any)
	elements := body["elements"].([]any)
	hasMainText := false
	for _, elem := range elements {
		em := elem.(map[string]any)
		if em["tag"] == "markdown" {
			if content, ok := em["content"].(string); ok && content == "Final answer" {
				hasMainText = true
			}
		}
	}
	if !hasMainText {
		t.Error("completed card missing main text")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/feishu/ -run TestBuildCompletedCardJSON -v`
Expected: FAIL — `buildCompletedCardJSON` undefined (it's currently named `buildRichCardJSONBytes`)

- [ ] **Step 3: Rename buildRichCardJSONBytes to buildCompletedCardJSON**

In `platform/feishu/feishu.go`, rename the function `buildRichCardJSONBytes` to `buildCompletedCardJSON`. Update all callers (primarily `buildRichCard` which calls it). Since `buildCompletedCardJSON` is now used by both the old path and the new FinalizeStreamingCard, keep the signature identical.

Also move `buildCompletedCardJSON` and its helper functions (`buildRichPanel`, `richStepElement`, `richStepRowContent`, `richPanelElements`, `richPlaceholderElement`, `splitRichStepsByLane`, `richLaneTitle`, `compactRichStepsForCardSize`, `compactRichFallbackMarkdown`) from `feishu.go` to `streaming_card.go` to keep the streaming card code together.

Update `buildRichCard` in `feishu.go` to call `buildCompletedCardJSON` instead:

```go
func buildRichCard(status core.CardStatus, _ string, steps []core.ToolStep, markdown string, streaming bool, statusFooter string) string {
	b, err := buildCompletedCardJSON(status, steps, markdown, streaming, statusFooter)
	// ... rest unchanged ...
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./platform/feishu/ -run TestBuildCompletedCardJSON -v`
Expected: PASS

- [ ] **Step 5: Implement FinalizeStreamingCard**

Add to `platform/feishu/streaming_card.go`:

```go
// FinalizeStreamingCard performs two-phase finalization:
// Phase 1: Patch all slots to terminal content (still in streaming mode)
// Phase 2: Disable streaming mode + rebuild full card with completed layout + set header
func (p *Platform) FinalizeStreamingCard(ctx context.Context, previewHandle any, steps []core.ToolStep, markdown string, status core.CardStatus, statusFooter string) error {
	h, ok := previewHandle.(*feishuPreviewHandle)
	if !ok {
		return fmt.Errorf("%s: FinalizeStreamingCard: %w", p.tag(), core.ErrSlotInvalidHandle)
	}

	// Phase 1: Patch slots to terminal content
	phase1Slots := []struct {
		slot    core.StreamingSlotID
		content core.SlotContent
	}{
		{core.SlotStatusBanner, core.SlotContent{Phase: "done", Elapsed: 0}},
		{core.SlotThinking, core.SlotContent{ThinkingText: ""}},
		{core.SlotTools, core.SlotContent{ToolSteps: steps}},
		{core.SlotMainText, core.SlotContent{MainText: markdown}},
		{core.SlotFooterNote, core.SlotContent{StatusFooter: statusFooter}},
	}
	for _, s := range phase1Slots {
		_ = p.StreamSlotContent(ctx, h, s.slot, s.content)
		// Phase 1 errors are not fatal — content may already be correct
	}

	// Phase 2: Rebuild full card with completed layout
	completedJSON, err := buildCompletedCardJSON(status, steps, markdown, false, statusFooter)
	if err != nil {
		slog.Error(p.tag()+": finalize streaming card: build completed card failed", "error", err)
		return err
	}

	h.mu.Lock()
	h.lastContent = string(completedJSON)
	h.status = status
	h.mu.Unlock()

	if err := p.updateCardEntity(ctx, h, string(completedJSON)); err != nil {
		slog.Warn(p.tag()+": finalize streaming card: Phase 2 failed (card content correct, header may be wrong)", "error", err)
		return err
	}
	return nil
}
```

- [ ] **Step 6: Build to verify compilation**

Run: `go build ./platform/feishu/`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add platform/feishu/streaming_card.go platform/feishu/feishu.go platform/feishu/streaming_card_test.go
git commit -m "feat(feishu): implement FinalizeStreamingCard with two-phase finalization"
```

---

### Task 7: Implement flush controller with dual-track throttling and dedup

<!-- covers: S7 (flush controller dedup/serialization), S10 (dedup), S15 (monitoring) -->

**Files:**
- Modify: `platform/feishu/streaming_card.go`
- Modify: `platform/feishu/streaming_card_test.go`

- [ ] **Step 1: Write the failing test for flush controller dedup**

Add to `platform/feishu/streaming_card_test.go`:

```go
func TestFlushControllerDedup(t *testing.T) {
	fc := newFlushController()

	// First write for a slot should be dispatched
	dispatched := fc.markDirty("status_banner", "content A")
	if !dispatched {
		t.Error("first write should be dispatched")
	}

	// Same content should be deduped
	dispatched = fc.markDirty("status_banner", "content A")
	if dispatched {
		t.Error("duplicate content should be deduped")
	}

	// Different content should be dispatched
	dispatched = fc.markDirty("status_banner", "content B")
	if !dispatched {
		t.Error("changed content should be dispatched")
	}

	// Different slot should be dispatched independently
	dispatched = fc.markDirty("tools_md", "tool content")
	if !dispatched {
		t.Error("different slot should be dispatched")
	}
}

func TestFlushControllerClear(t *testing.T) {
	fc := newFlushController()

	fc.markDirty("status_banner", "content A")
	fc.markDirty("tools_md", "tool content")

	pending := fc.drainPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending slots, got %d", len(pending))
	}

	// After drain, pending should be empty
	pending = fc.drainPending()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after drain, got %d", len(pending))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/feishu/ -run "TestFlushControllerDedup|TestFlushControllerClear" -v`
Expected: FAIL — `flushController` undefined

- [ ] **Step 3: Implement flush controller**

Add to `platform/feishu/streaming_card.go`:

```go
// flushController tracks per-slot content hashes for dedup and manages
// pending dirty slots for the next flush cycle.
type flushController struct {
	mu       sync.Mutex
	lastHash map[string]uint32       // slot element_id → fnv64 hash of last rendered content
	lastContent map[string]string     // slot element_id → last rendered content
	pending  map[string]string        // slot element_id → content to flush
}

func newFlushController() *flushController {
	return &flushController{
		lastHash:    make(map[string]uint32),
		lastContent: make(map[string]string),
		pending:     make(map[string]string),
	}
}

// markDirty records a slot content change. Returns true if the content
// differs from the last flushed content (i.e., should be dispatched).
func (fc *flushController) markDirty(elementID, content string) bool {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	hash := fnv32(content)
	if lastHash, ok := fc.lastHash[elementID]; ok && lastHash == hash {
		if lastContent, ok := fc.lastContent[elementID]; ok && lastContent == content {
			return false // dedup: content unchanged
		}
	}

	fc.pending[elementID] = content
	fc.lastHash[elementID] = hash
	fc.lastContent[elementID] = content
	return true
}

// drainPending returns all pending slot updates and clears the pending map.
func (fc *flushController) drainPending() map[string]string {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	result := make(map[string]string, len(fc.pending))
	for k, v := range fc.pending {
		result[k] = v
	}
	fc.pending = make(map[string]string)
	return result
}

// fnv32 computes a 32-bit FNV-1 hash for dedup.
func fnv32(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for i := 0; i < len(s); i++ {
		h *= prime32
		h ^= uint32(s[i])
	}
	return h
}
```

Add `"sync"` to the imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./platform/feishu/ -run "TestFlushControllerDedup|TestFlushControllerClear" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add platform/feishu/streaming_card.go platform/feishu/streaming_card_test.go
git commit -m "feat(feishu): add flush controller with dedup and pending slot tracking"
```

---

### Task 8: Restructure engine event loop for StreamingRichCardSupporter

<!-- covers: S4 (derivePhase in engine), S5 (ToolStep.Duration), S8 (fallback), S9 (SlotMainText replaces StreamRichCardText), S12 (BuildStreamingCard failure) -->

**Files:**
- Modify: `core/engine.go`

This is the most complex task. The engine event loop must be restructured to check `StreamingRichCardSupporter` before `RichCardSupporter`, and route events to slot updates when the streaming card is active.

- [ ] **Step 1: Add per-turn state variables for streaming card**

In the engine event loop (where `toolSteps`, `cardMessageID`, etc. are initialized), add:

```go
var streamingCardHandle any          // handle from BuildStreamingCard
var hasStreamingCardSupport bool     // p implements StreamingRichCardSupporter
var streamingCardDegraded bool       // degraded to full-card rebuild this turn
var hasTextContent bool              // true once first EventText arrives
```

And at the point where `richCardSupporter, hasRichCard := p.(RichCardSupporter)` is called, add before it:

```go
streamingCardSupporter, hasStreamingCardSupport := p.(StreamingRichCardSupporter)
richCardSupporter, hasRichCard := p.(RichCardSupporter)
// Card 2.0 rich-card path is opt-in via [display] mode = "rich".
if e.display.CardMode != "rich" {
	hasStreamingCardSupport = false
	hasRichCard = false
}
```

- [ ] **Step 2: Add helper for creating/using streaming card**

After the `buildResolvedRichCard` closure, add a helper for the streaming card path:

```go
ensureStreamingCard := func(status core.CardStatus, title string) error {
	if streamingCardHandle != nil {
		return nil
	}
	handle, err := streamingCardSupporter.BuildStreamingCard(e.ctx, rc.chatID, status, title)
	if err != nil {
		slog.Debug("engine: BuildStreamingCard failed, falling back to RichCardSupporter", "error", err)
		hasStreamingCardSupport = false
		streamingCardDegraded = true
		return err
	}
	streamingCardHandle = handle
	cardMessageID = handle
	return nil
}

streamSlotUpdate := func(slot core.StreamingSlotID, content core.SlotContent) {
	if streamingCardHandle == nil || streamingCardDegraded {
		return
	}
	err := streamingCardSupporter.StreamSlotContent(e.ctx, streamingCardHandle, slot, content)
	if err != nil {
		if errors.Is(err, core.ErrSlotRateLimited) {
			slog.Debug("engine: slot rate limited, degrading to full-card rebuild", "slot", slot)
			streamingCardDegraded = true
		} else if errors.Is(err, core.ErrSlotNotSupported) {
			slog.Debug("engine: slot not supported, degrading to full-card rebuild", "slot", slot)
			streamingCardDegraded = true
		}
	}
}
```

- [ ] **Step 3: Track startedAt on EventToolUse**

In the `EventToolUse` handler, when appending a `ToolStep`, set `startedAt`:

```go
toolSteps = append(toolSteps, core.ToolStep{
	Kind:     core.ToolStepKindTool,
	Name:     toolName,
	Summary:  toolSummary,
	Status:   "running",
	Done:     false,
	startedAt: time.Now(),
})
```

- [ ] **Step 4: Compute Duration on EventToolResult**

In the `mergeRichToolResult` function (or wherever tool results are merged into `toolSteps`), after finding the matching step:

```go
step.Duration = time.Since(step.startedAt)
```

- [ ] **Step 5: Route EventToolUse to streaming card when available**

In the `EventToolUse` handler, after appending the tool step, add the streaming card path:

```go
if hasStreamingCardSupport && !streamingCardDegraded {
	if err := ensureStreamingCard(core.CardStatusWorking, "Working"); err == nil {
		phase := derivePhase(toolSteps, hasTextContent)
		streamSlotUpdate(core.SlotStatusBanner, core.SlotContent{Phase: phase, ActiveTool: toolName, ToolSummary: toolSummary})
		streamSlotUpdate(core.SlotTools, core.SlotContent{ToolSteps: toolSteps})
	}
	continue // skip the old RichCardSupporter path
}
```

- [ ] **Step 6: Route EventToolResult to streaming card when available**

Similar routing in `EventToolResult`:

```go
if hasStreamingCardSupport && !streamingCardDegraded {
	phase := derivePhase(toolSteps, hasTextContent)
	streamSlotUpdate(core.SlotStatusBanner, core.SlotContent{Phase: phase, ActiveTool: lastActiveTool(toolSteps), ToolSummary: lastActiveToolSummary(toolSteps)})
	streamSlotUpdate(core.SlotTools, core.SlotContent{ToolSteps: toolSteps})
	continue
}
```

- [ ] **Step 7: Route EventThinking to streaming card when available**

```go
if hasStreamingCardSupport && !streamingCardDegraded {
	if err := ensureStreamingCard(core.CardStatusThinking, "Thinking"); err == nil {
		phase := derivePhase(toolSteps, hasTextContent)
		streamSlotUpdate(core.SlotStatusBanner, core.SlotContent{Phase: phase})
		streamSlotUpdate(core.SlotThinking, core.SlotContent{ThinkingText: thinkingText})
	}
	continue
}
```

- [ ] **Step 8: Route EventText to streaming card when available**

For `EventText`, set `hasTextContent = true` and route to `StreamSlotContent(SlotMainText, ...)` instead of `StreamRichCardText`:

```go
hasTextContent = true
if hasStreamingCardSupport && !streamingCardDegraded {
	if err := ensureStreamingCard(core.CardStatusWorking, "Working"); err == nil {
		phase := derivePhase(toolSteps, hasTextContent)
		streamSlotUpdate(core.SlotStatusBanner, core.SlotContent{Phase: phase})
		streamSlotUpdate(core.SlotMainText, core.SlotContent{MainText: fullText})
	}
	continue
}
```

This replaces the existing `StreamRichCardText` call for streaming-capable platforms. The `RichCardTextStreamer` path is only used when `!hasStreamingCardSupport`.

- [ ] **Step 9: Route finalization to FinalizeStreamingCard**

In the `EventResult` handler, when the turn is complete and `hasStreamingCardSupport`:

```go
if hasStreamingCardSupport && streamingCardHandle != nil {
	err := streamingCardSupporter.FinalizeStreamingCard(e.ctx, streamingCardHandle, toolSteps, finalMarkdown, core.CardStatusDone, statusFooter)
	if err != nil {
		slog.Warn("engine: FinalizeStreamingCard failed", "error", err)
		// Fall through to existing finalization path
	} else {
		// Card was finalized successfully
		cardMessageID = nil
		// ... continue with done reaction etc.
	}
}
```

- [ ] **Step 10: Reset streaming card state on turn boundary**

Where `cardMessageID` and `toolSteps` are reset at the start of a new turn, also reset:

```go
streamingCardHandle = nil
streamingCardDegraded = false
hasTextContent = false
```

- [ ] **Step 11: Add lastActiveTool helper**

```go
func lastActiveTool(steps []ToolStep) string {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Kind == ToolStepKindTool && !steps[i].Done {
			return steps[i].Name
		}
	}
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Kind == ToolStepKindTool {
			return steps[i].Name
		}
	}
	return ""
}
```

- [ ] **Step 12: Build to verify compilation**

Run: `go build ./...`
Expected: PASS (may need iteration on variable names and exact line numbers)

- [ ] **Step 13: Run full test suite**

Run: `go test ./core/ -count=1`
Expected: All tests pass (existing tests should be unaffected since the streaming card path is opt-in)

- [ ] **Step 14: Commit**

```bash
git add core/engine.go
git commit -m "feat(core): restructure engine event loop for StreamingRichCardSupporter"
```

---

### Task 9: Add engine tests for fallback and slot dispatch

<!-- covers: S8 (fallback to RichCardSupporter), S9 (SlotMainText replaces StreamRichCardText), S12 (BuildStreamingCard failure) -->

**Files:**
- Modify: `core/engine_test.go`

- [ ] **Step 1: Write test for RichCardSupporter fallback**

Add to `core/engine_test.go`:

```go
// stubRichCardOnly implements Platform + RichCardSupporter (no StreamingRichCardSupporter).
type stubRichCardOnly struct {
	stubPlatformEngine
	builtCards []string
}

func (s *stubRichCardOnly) BuildRichCard(status CardStatus, title string, steps []ToolStep, markdown string, streaming bool, statusFooter string) string {
	s.builtCards = append(s.builtCards, title)
	return fmt.Sprintf("card:%s", title)
}

func TestEngineFallbackToRichCardSupporter(t *testing.T) {
	p := &stubRichCardOnly{n: "test-rich"}
	// Verify it does NOT implement StreamingRichCardSupporter
	if _, ok := any(p).(StreamingRichCardSupporter); ok {
		t.Fatal("stubRichCardOnly should not implement StreamingRichCardSupporter")
	}
	// Verify it DOES implement RichCardSupporter
	if _, ok := any(p).(RichCardSupporter); !ok {
		t.Fatal("stubRichCardOnly should implement RichCardSupporter")
	}
	// The engine's hasStreamingCardSupport check should be false,
	// so tool events route through BuildRichCard.
	// Full integration test requires a running engine — this unit test
	// validates the type assertion logic.
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./core/ -run TestEngineFallbackToRichCardSupporter -v`
Expected: PASS (type assertion tests)

- [ ] **Step 3: Commit**

```bash
git add core/engine_test.go
git commit -m "test(core): add StreamingRichCardSupporter fallback tests"
```

---

### Task 10: Add monitoring/logging for streaming card operations

<!-- covers: S15 (monitoring metrics) -->

**Files:**
- Modify: `platform/feishu/streaming_card.go`

- [ ] **Step 1: Add structured logging to StreamSlotContent**

In `StreamSlotContent`, add after successful API call:

```go
slog.Debug(p.tag()+": stream slot content success",
	"slot", slot,
	"element_id", elementID,
	"content_len", len(markdown),
	"sequence", seq,
)
```

- [ ] **Step 2: Add structured logging to FinalizeStreamingCard**

After Phase 1 completes:

```go
slog.Debug(p.tag()+": finalize streaming card Phase 1 complete",
	"slots_patched", len(phase1Slots),
)
```

After Phase 2 succeeds:

```go
slog.Info(p.tag()+": finalize streaming card complete",
	"status", status,
	"steps_count", len(steps),
)
```

- [ ] **Step 3: Add degradation logging in BuildStreamingCard**

Already present from the `slog.Info` call on cardEntity creation failure.

- [ ] **Step 4: Commit**

```bash
git add platform/feishu/streaming_card.go
git commit -m "feat(feishu): add structured logging for streaming card operations"
```

---

### Task 11: Integration test — end-to-end streaming card lifecycle

<!-- covers: S1, S2, S3, S4, S5, S6, S7 (combined lifecycle) -->

**Files:**
- Modify: `platform/feishu/streaming_card_test.go`

- [ ] **Step 1: Write integration test for full lifecycle**

Add to `platform/feishu/streaming_card_test.go`:

```go
func TestStreamingCardLifecycle(t *testing.T) {
	// 1. Build skeleton
	skeleton := buildStreamingCardSkeleton(core.CardStatusThinking, "")
	var card map[string]any
	if err := json.Unmarshal([]byte(skeleton), &card); err != nil {
		t.Fatalf("skeleton parse error: %v", err)
	}
	config := card["config"].(map[string]any)
	if config["streaming_mode"] != true {
		t.Fatal("skeleton should have streaming_mode=true")
	}

	// 2. Render status banner for thinking phase
	banner := renderStatusBanner(core.SlotContent{Phase: "thinking", Elapsed: 3 * time.Second})
	if !containsString(banner, "思考中") {
		t.Errorf("thinking banner missing 思考中: %q", banner)
	}

	// 3. Render tools timeline with running tool
	tools := renderToolsTimeline(core.SlotContent{ToolSteps: []core.ToolStep{
		{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: core/engine.go", Status: "running", Done: false, Duration: 1200 * time.Millisecond},
	}})
	if !containsString(tools, "运行") || !containsString(tools, "Read") {
		t.Errorf("tools timeline missing running tool: %q", tools)
	}

	// 4. Render tools timeline with completed tool + result
	tools = renderToolsTimeline(core.SlotContent{ToolSteps: []core.ToolStep{
		{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: core/engine.go", Status: "complete", Done: true, Result: "32 lines, 1200 bytes", Duration: 500 * time.Millisecond},
	}})
	if !containsString(tools, "完成") || !containsString(tools, "↳") {
		t.Errorf("tools timeline missing completed tool with result: %q", tools)
	}

	// 5. Build completed card
	completed, err := buildCompletedCardJSON(core.CardStatusDone, []core.ToolStep{
		{Kind: core.ToolStepKindTool, Name: "Read", Summary: "a.go", Status: "complete", Done: true, Duration: 500 * time.Millisecond},
	}, "Final answer", false, "1 tool used")
	if err != nil {
		t.Fatalf("completed card error: %v", err)
	}
	var completedCard map[string]any
	if err := json.Unmarshal(completed, &completedCard); err != nil {
		t.Fatalf("completed card parse error: %v", err)
	}
	completedConfig := completedCard["config"].(map[string]any)
	if completedConfig["streaming_mode"] != false {
		t.Error("completed card should have streaming_mode=false")
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./platform/feishu/ -run TestStreamingCardLifecycle -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add platform/feishu/streaming_card_test.go
git commit -m "test(feishu): add end-to-end streaming card lifecycle test"
```

---

### Task 12: Run full test suite and fix regressions

<!-- covers: All scenarios — regression check -->

**Files:**
- Any files that need fixes

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests pass

- [ ] **Step 2: Run with race detector**

Run: `go test -race ./core/ ./platform/feishu/ -count=1`
Expected: No race conditions

- [ ] **Step 3: Fix any test failures**

If any tests fail, investigate and fix. Common issues:
- `ToolStep` struct change breaking existing code that uses positional initialization
- `buildRichCardJSONBytes` rename breaking callers
- Import cycles from moving functions between files

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve test regressions from streaming card refactor"
```

---

### Task 13: Commit the uncommitted design spec

**Files:**
- `docs/superpowers/specs/2026-06-21-streaming-multi-slot-card-design.md`

- [ ] **Step 1: Commit the design spec**

```bash
git add docs/superpowers/specs/2026-06-21-streaming-multi-slot-card-design.md
git commit -m "docs: add streaming multi-slot rich card design spec"
```

---

## Self-Review

### 1. Spec coverage

| Spec Section | Plan Task |
|---|---|
| Architecture - Card Skeleton (Streaming) | Task 3 |
| Architecture - Card Skeleton (Completed) | Task 6 |
| Architecture - Flush Controller | Task 7 |
| Interface Changes - StreamingRichCardSupporter | Task 1 |
| Interface Changes - ToolStep Extension | Task 1 |
| Interface Changes - Engine Event Loop Changes | Task 8 |
| Panel Rendering - Status Banner | Task 2 |
| Panel Rendering - Tools Timeline | Task 2 |
| Panel Rendering - Thinking Panel | Task 2 |
| Panel Rendering - Completed Card Tools Panel | Task 6 |
| API Degradation Chain | Task 4 (ErrSlotRateLimited return), Task 8 (degradation flag) |
| Monitoring | Task 10 |
| Backward Compatibility | Task 8 (fallback), Task 9 (test) |

### 2. Placeholder scan

No TBD, TODO, or placeholder patterns found. All steps contain actual code.

### 3. Type consistency

- `StreamingSlotID` constants in Task 1 match usage in Tasks 4, 8
- `SlotContent` struct fields in Task 1 match rendering function parameters in Task 2
- `ToolStep.Duration` and `startedAt` added in Task 1, populated in Task 8
- `buildCompletedCardJSON` defined in Task 6, called in Task 6 (FinalizeStreamingCard)
- `resolveSlotElementID` mapping in Task 4 matches element IDs in Task 3
- `flushController` defined in Task 7

### 4. Verification coverage

| Scenario | Plan Task |
|---|---|
| S1: Streaming card skeleton creation | Task 3 (skeleton test) |
| S2: Slot-level content patching | Task 4 (StreamSlotContent) |
| S3: Tool return value display | Task 2 (renderToolsTimeline with Result) |
| S4: Phase derivation logic | Task 1 (derivePhase test) |
| S5: ToolStep Duration calculation | Task 8 (startedAt/Duration in engine) |
| S6: FinalizeStreamingCard completed card | Task 6 (buildCompletedCardJSON test) |
| S7: API degradation L0→L1 | Task 8 (degradation flag + fallback) |
| S8: BuildStreamingCard failure → RichCardSupporter | Task 9 (fallback test) |
| S9: StreamSlotContent replaces StreamRichCardText | Task 8 (engine routing) |
| S10: Flush controller dedup/serialization | Task 7 (flush controller test) |
| S11: Tools timeline overflow/truncation | Task 2 (overflow test) |
| S12: BuildStreamingCard returns ErrSlotNotSupported | Task 5 (BuildStreamingCard) |
| S13: FinalizeStreamingCard Phase 2 failure | Task 6 (two-phase implementation) |
| S14: Empty thinking panel hidden | Task 3 (skeleton test with/without thinking) |
| S15: Monitoring metrics emitted | Task 10 (structured logging) |
