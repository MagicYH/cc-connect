package feishu

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, ""},
		{100 * time.Millisecond, "100ms"},
		{500 * time.Millisecond, "500ms"},
		{time.Second, "1.0s"},
		{2300 * time.Millisecond, "2.3s"},
		{30 * time.Second, "30.0s"},
		{90 * time.Second, "1m30s"},
		{2*time.Minute + 15*time.Second, "2m15s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestRenderStatusBanner(t *testing.T) {
	tests := []struct {
		name    string
		content core.SlotContent
		want    []string // substrings that must appear
		dont    []string // substrings that must NOT appear
	}{
		{
			name:    "thinking phase",
			content: core.SlotContent{Phase: core.PhaseThinking, Elapsed: 12 * time.Second},
			want:    []string{"💭", "**思考中**", "12.0s"},
		},
		{
			name:    "tooling phase with active tool",
			content: core.SlotContent{Phase: core.PhaseTooling, Elapsed: 5 * time.Second, ActiveTool: "Read", ToolSummary: "path: core/engine.go"},
			want:    []string{"🔧", "**工具调用中**", "`Read`", "5.0s"},
		},
		{
			name:    "tooling phase without active tool",
			content: core.SlotContent{Phase: core.PhaseTooling, Elapsed: 3 * time.Second},
			want:    []string{"🔧", "**工具调用中**", "3.0s"},
			dont:    []string{"`"},
		},
		{
			name:    "streaming phase",
			content: core.SlotContent{Phase: core.PhaseStreaming, Elapsed: 15 * time.Second},
			want:    []string{"✍️", "**生成中**", "15.0s"},
		},
		{
			name:    "done phase",
			content: core.SlotContent{Phase: core.PhaseDone, Elapsed: 23 * time.Second},
			want:    []string{"✅", "**完成**", "23.0s"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderStatusBanner(tt.content)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("renderStatusBanner() = %q, want substring %q", got, w)
				}
			}
			for _, d := range tt.dont {
				if strings.Contains(got, d) {
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
			want: []string{"运行", "`Run command`", "2.3s"}, // buildToolDisplay maps "Bash" → "Run command"
		},
		{
			name: "completed tool with result",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: core/engine.go", Status: "complete", Done: true, Result: "32 lines, 1200 bytes", Duration: 500 * time.Millisecond},
			}},
			want: []string{"完成", "`Read`", "↳", "32 lines, 1200 bytes"},
		},
		{
			name: "error tool",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindTool, Name: "Bash", Summary: "cmd: fail", Status: "error", Done: true, Duration: 100 * time.Millisecond},
			}},
			want: []string{"出错", "`Run command`"}, // buildToolDisplay maps "Bash" → "Run command"
		},
		{
			name: "mixed running and completed tools",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: a.go", Status: "complete", Done: true, Duration: time.Second},
				{Kind: core.ToolStepKindTool, Name: "Bash", Summary: "cmd: test", Status: "running", Done: false, Duration: 2 * time.Second},
			}},
			want: []string{"运行", "完成", "`Run command`", "`Read`"},
		},
		{
			name: "thinking steps filtered out",
			content: core.SlotContent{ToolSteps: []core.ToolStep{
				{Kind: core.ToolStepKindThinking, Name: "thinking", Status: "complete", Done: true, Duration: time.Second},
				{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: a.go", Status: "complete", Done: true, Duration: time.Second},
			}},
			want: []string{"`Read`"},
			dont: []string{"thinking"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderToolsTimeline(tt.content)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("renderToolsTimeline() = %q, want substring %q", got, w)
				}
			}
			for _, d := range tt.dont {
				if strings.Contains(got, d) {
					t.Errorf("renderToolsTimeline() = %q, should NOT contain %q", got, d)
				}
			}
		})
	}
}

func TestRenderToolsTimelineResultTruncation(t *testing.T) {
	longResult := strings.Repeat("x", 200)
	content := core.SlotContent{ToolSteps: []core.ToolStep{
		{Kind: core.ToolStepKindTool, Name: "Bash", Status: "complete", Done: true, Result: longResult, Duration: time.Second},
	}}
	got := renderToolsTimeline(content)
	if strings.Contains(got, strings.Repeat("x", 121)) {
		t.Errorf("result should be truncated to 120 chars")
	}
}

func TestRenderToolsTimelineCJKTruncation(t *testing.T) {
	// Each CJK char is 3 bytes; byte-based truncation would split mid-rune.
	longResult := strings.Repeat("中", 200) // 600 bytes, 200 runes
	content := core.SlotContent{ToolSteps: []core.ToolStep{
		{Kind: core.ToolStepKindTool, Name: "Read", Status: "complete", Done: true, Result: longResult, Duration: time.Second},
	}}
	got := renderToolsTimeline(content)
	// Should have 120 CJK chars + "…", not garbled bytes
	if !strings.Contains(got, "…") {
		t.Errorf("CJK result should be truncated with ellipsis")
	}
	// 120 CJK chars = 360 bytes; verify no more than 120 runes of original
	visible := strings.Repeat("中", 121)
	if strings.Contains(got, visible) {
		t.Errorf("CJK result should be truncated to 120 runes, got more")
	}
}

func TestRenderToolsTimelineOverflow(t *testing.T) {
	steps := make([]core.ToolStep, 12)
	for i := range steps {
		steps[i] = core.ToolStep{Kind: core.ToolStepKindTool, Name: "Read", Summary: "path: file.go", Status: "complete", Done: true, Duration: time.Second}
	}
	got := renderToolsTimeline(core.SlotContent{ToolSteps: steps})
	if !strings.Contains(got, "另有") {
		t.Errorf("expected overflow message for 12 tools, got: %q", got)
	}
	if len(got) > 4096 {
		t.Errorf("tools_md exceeds 4KB budget: %d bytes", len(got))
	}
}

func TestRenderToolsTimelineEmpty(t *testing.T) {
	got := renderToolsTimeline(core.SlotContent{ToolSteps: nil})
	if got != "" {
		t.Errorf("empty tool steps should return empty string, got %q", got)
	}
}

func TestRenderThinkingContent(t *testing.T) {
	if got := renderThinkingContent(core.SlotContent{ThinkingText: ""}); got != "" {
		t.Errorf("empty thinking should return empty, got %q", got)
	}
	if got := renderThinkingContent(core.SlotContent{ThinkingText: "Analyzing..."}); !strings.Contains(got, "Analyzing") {
		t.Errorf("non-empty thinking should contain text, got %q", got)
	}
}

func TestBuildStreamingCardSkeleton(t *testing.T) {
	raw := buildStreamingCardSkeleton(core.CardStatusThinking, "")
	var card map[string]any
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Schema
	if card["schema"] != "2.0" {
		t.Errorf("expected schema 2.0, got %v", card["schema"])
	}

	// Config
	cfg, _ := card["config"].(map[string]any)
	if cfg["streaming_mode"] != true {
		t.Error("expected streaming_mode true")
	}
	if cfg["update_multi"] != true {
		t.Error("expected update_multi true")
	}

	// Header: thinking status → blue
	hdr, _ := card["header"].(map[string]any)
	if hdr["template"] != "blue" {
		t.Errorf("expected blue template for thinking, got %v", hdr["template"])
	}

	// Collect all element_ids in body order
	body, _ := card["body"].(map[string]any)
	elems, _ := body["elements"].([]any)
	var ids []string
	for _, e := range elems {
		em, _ := e.(map[string]any)
		if id, ok := em["element_id"]; ok {
			ids = append(ids, id.(string))
		}
		// Also check nested elements inside collapsible_panel
		if nested, ok := em["elements"]; ok {
			for _, n := range nested.([]any) {
				nm, _ := n.(map[string]any)
				if id, ok := nm["element_id"]; ok {
					ids = append(ids, id.(string))
				}
			}
		}
	}

	// Required element_ids must be present
	required := []string{
		streamingElementStatusBanner,
		streamingElementToolsMd,
		streamingElementMainText,
		streamingElementFooterNote,
	}
	for _, req := range required {
		found := false
		for _, id := range ids {
			if id == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing required element_id %q in card; found: %v", req, ids)
		}
	}

	// thinking_panel must be ABSENT when thinkingText is empty
	for _, id := range ids {
		if id == streamingElementThinkingPanel || id == streamingElementThinkingMd {
			t.Errorf("thinking panel element_id %q should not be present when thinkingText is empty", id)
		}
	}

	// Tools panel must be collapsed
	for _, e := range elems {
		em, _ := e.(map[string]any)
		if em["tag"] == "collapsible_panel" && em["element_id"] == streamingElementToolsPanel {
			if em["expanded"] == true {
				t.Error("tools panel should start collapsed")
			}
		}
	}
}

func TestBuildStreamingCardSkeletonWithThinking(t *testing.T) {
	raw := buildStreamingCardSkeleton(core.CardStatusThinking, "Analyzing code...")
	var card map[string]any
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	body, _ := card["body"].(map[string]any)
	elems, _ := body["elements"].([]any)

	// Collect all element_ids including nested
	var ids []string
	var thinkingPanel map[string]any
	for _, e := range elems {
		em, _ := e.(map[string]any)
		if id, ok := em["element_id"]; ok {
			ids = append(ids, id.(string))
		}
		if em["tag"] == "collapsible_panel" && em["element_id"] == streamingElementThinkingPanel {
			thinkingPanel = em
		}
		if nested, ok := em["elements"]; ok {
			for _, n := range nested.([]any) {
				nm, _ := n.(map[string]any)
				if id, ok := nm["element_id"]; ok {
					ids = append(ids, id.(string))
				}
			}
		}
	}

	// thinking_panel and thinking_md must be present
	foundPanel := false
	foundMd := false
	for _, id := range ids {
		if id == streamingElementThinkingPanel {
			foundPanel = true
		}
		if id == streamingElementThinkingMd {
			foundMd = true
		}
	}
	if !foundPanel {
		t.Error("thinking_panel element_id should be present when thinkingText is non-empty")
	}
	if !foundMd {
		t.Error("thinking_md element_id should be present when thinkingText is non-empty")
	}

	// Thinking panel must be expanded
	if thinkingPanel != nil && thinkingPanel["expanded"] != true {
		t.Error("thinking panel should be expanded when thinkingText is non-empty")
	}

	// Header should be blue for thinking status
	hdr, _ := card["header"].(map[string]any)
	if hdr["template"] != "blue" {
		t.Errorf("expected blue template for thinking, got %v", hdr["template"])
	}

	// Verify done status uses green
	rawDone := buildStreamingCardSkeleton(core.CardStatusDone, "")
	var cardDone map[string]any
	if err := json.Unmarshal([]byte(rawDone), &cardDone); err != nil {
		t.Fatalf("invalid JSON for done card: %v", err)
	}
	hdrDone, _ := cardDone["header"].(map[string]any)
	if hdrDone["template"] != "green" {
		t.Errorf("expected green template for done, got %v", hdrDone["template"])
	}

	// Verify error status uses red
	rawErr := buildStreamingCardSkeleton(core.CardStatusError, "")
	var cardErr map[string]any
	if err := json.Unmarshal([]byte(rawErr), &cardErr); err != nil {
		t.Fatalf("invalid JSON for error card: %v", err)
	}
	hdrErr, _ := cardErr["header"].(map[string]any)
	if hdrErr["template"] != "red" {
		t.Errorf("expected red template for error, got %v", hdrErr["template"])
	}
}

func TestStreamSlotContentRoutesToSlotAPI(t *testing.T) {
	// Verify each StreamingSlotID maps to the correct element_id.
	tests := []struct {
		slot core.StreamingSlotID
		want string
	}{
		{core.SlotStatusBanner, streamingElementStatusBanner},
		{core.SlotThinking, streamingElementThinkingMd},
		{core.SlotTools, streamingElementToolsMd},
		{core.SlotMainText, streamingElementMainText},
		{core.SlotFooterNote, streamingElementFooterNote},
	}
	for _, tt := range tests {
		got := resolveSlotElementID(tt.slot)
		if got != tt.want {
			t.Errorf("resolveSlotElementID(%q) = %q, want %q", tt.slot, got, tt.want)
		}
	}

	// Verify renderSlotContent routes correctly for each slot.
	// SlotStatusBanner → renderStatusBanner
	banner := renderSlotContent(core.SlotStatusBanner, core.SlotContent{Phase: core.PhaseThinking})
	if banner == "" {
		t.Error("renderSlotContent for SlotStatusBanner should produce non-empty output")
	}

	// SlotThinking → renderThinkingContent
	think := renderSlotContent(core.SlotThinking, core.SlotContent{ThinkingText: "Hello"})
	if think != "Hello" {
		t.Errorf("renderSlotContent for SlotThinking = %q, want %q", think, "Hello")
	}

	// SlotTools → renderToolsTimeline (empty steps → empty string)
	tools := renderSlotContent(core.SlotTools, core.SlotContent{ToolSteps: nil})
	if tools != "" {
		t.Errorf("renderSlotContent for SlotTools with empty steps = %q, want empty", tools)
	}

	// SlotMainText → content.MainText
	main := renderSlotContent(core.SlotMainText, core.SlotContent{MainText: "body text"})
	if main != "body text" {
		t.Errorf("renderSlotContent for SlotMainText = %q, want %q", main, "body text")
	}

	// SlotFooterNote → content.StatusFooter
	foot := renderSlotContent(core.SlotFooterNote, core.SlotContent{StatusFooter: "elapsed 5s"})
	if foot != "elapsed 5s" {
		t.Errorf("renderSlotContent for SlotFooterNote = %q, want %q", foot, "elapsed 5s")
	}
}
