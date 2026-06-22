package feishu

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chenhg5/cc-connect/core"
)

// formatDuration produces a human-friendly elapsed-time string.
//
//	0        → ""
//	< 1s     → "123ms"
//	< 60s    → "1.5s"
//	>= 60s   → "2m30s"
func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

// renderStatusBanner produces status banner markdown for a streaming card slot.
func renderStatusBanner(c core.SlotContent) string {
	elapsed := formatDuration(c.Elapsed)
	switch c.Phase {
	case core.PhaseThinking:
		return fmt.Sprintf("💭 **思考中** · %s", elapsed)
	case core.PhaseTooling:
		if c.ActiveTool != "" {
			return fmt.Sprintf("🔧 **工具调用中** · `%s` %s", c.ActiveTool, elapsed)
		}
		return fmt.Sprintf("🔧 **工具调用中** · %s", elapsed)
	case core.PhaseStreaming:
		return fmt.Sprintf("✍️ **生成中** · %s", elapsed)
	case core.PhaseDone:
		return fmt.Sprintf("✅ **完成** · %s", elapsed)
	default:
		return ""
	}
}

const (
	maxVisibleTools  = 8
	maxToolResultLen = 120
	maxToolsMDSize   = 4096
	overflowMsg      = "另有 %d 条工具记录已收起"
)

// renderToolsTimeline produces a tools timeline markdown for a streaming card slot.
// Running tools are listed first (newest first), then completed (newest first).
// Max 8 visible entries; overflow summarized. Total output capped at 4KB.
func renderToolsTimeline(c core.SlotContent) string {
	if len(c.ToolSteps) == 0 {
		return ""
	}

	// Partition into running and completed, preserving newest-first order
	// (ToolSteps is already append-order; reverse to get newest-first).
	var running, completed []core.ToolStep
	for i := len(c.ToolSteps) - 1; i >= 0; i-- {
		s := c.ToolSteps[i]
		if s.Kind == core.ToolStepKindThinking {
			continue
		}
		if !s.Done {
			running = append(running, s)
		} else {
			completed = append(completed, s)
		}
	}

	// Merge: running first, then completed
	ordered := append(running[:min(len(running), maxVisibleTools)], completed...)

	var sb strings.Builder
	visible := 0
	for _, step := range ordered {
		if visible >= maxVisibleTools {
			break
		}
		visible++

		display := buildToolDisplay(step.Name, step.Summary)

		switch {
		case !step.Done:
			sb.WriteString(fmt.Sprintf("<text_tag color='blue'>运行</text_tag> `%s` (%s)\n", display.Title, formatDuration(step.Duration)))
		case step.Status == "error":
			sb.WriteString(fmt.Sprintf("<text_tag color='red'>出错</text_tag> `%s`\n", display.Title))
		default:
			sb.WriteString(fmt.Sprintf("<text_tag color='green'>完成</text_tag> `%s`\n", display.Title))
		}

		if display.Detail != "" {
			sb.WriteString(fmt.Sprintf("  <font color='grey'>%s</font>\n", display.Detail))
		}

		if step.Done && step.Result != "" {
			result := step.Result
			if len([]rune(result)) > maxToolResultLen {
				result = string([]rune(result)[:maxToolResultLen]) + "…"
			}
			sb.WriteString(fmt.Sprintf("  ↳ <font color='grey'>结果</font> %s\n", result))
		}
	}

	// Overflow message
	remaining := len(c.ToolSteps) - visible
	if remaining > 0 {
		sb.WriteString(fmt.Sprintf(overflowMsg+"\n", remaining))
	}

	md := sb.String()
	// Trim trailing newline
	md = strings.TrimRight(md, "\n")

	// Cap at maxToolsMDSize bytes, truncating at a valid UTF-8 boundary
	if len(md) > maxToolsMDSize {
		end := maxToolsMDSize
		for end > 0 && !utf8.RuneStart(md[end]) {
			end--
		}
		md = md[:end]
	}

	return md
}

// renderThinkingContent returns the thinking text, or empty string if none (panel hidden).
func renderThinkingContent(c core.SlotContent) string {
	return c.ThinkingText
}
