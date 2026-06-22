package feishu

import (
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
