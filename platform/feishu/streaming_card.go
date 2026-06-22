package feishu

import (
	"encoding/json"
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

// Streaming card element IDs — each independently patchable via cardElement.content().
const (
	streamingElementStatusBanner  = "status_banner"
	streamingElementThinkingPanel = "thinking_panel"
	streamingElementThinkingMd    = "thinking_md"
	streamingElementToolsPanel    = "tools_panel"
	streamingElementToolsMd       = "tools_md"
	streamingElementMainText      = "main_text"
	streamingElementFooterNote    = "footer_note"
)

// buildStreamingCardSkeleton creates the initial multi-slot card JSON for
// Feishu streaming card mode. All slots have fixed element_ids so individual
// panels can be patched via cardElement.content() without rebuilding the card.
//
// thinkingText controls whether the thinking collapsible_panel is included;
// when non-empty the panel is present and expanded, when empty it is omitted.
func buildStreamingCardSkeleton(status core.CardStatus, thinkingText string) string {
	// Header template color and title follow status.
	headerTemplate := "blue"
	headerTitle := pickThinkingVerb()
	switch status {
	case core.CardStatusDone:
		headerTemplate = "green"
		headerTitle = "Done"
	case core.CardStatusError:
		headerTemplate = "red"
		headerTitle = "Error"
	case core.CardStatusThinking, core.CardStatusWorking:
		headerTemplate = "blue"
		headerTitle = pickThinkingVerb()
	}

	// Status banner — always present, patched throughout the session.
	bannerMd := renderStatusBanner(core.SlotContent{Phase: core.PhaseThinking})

	elements := []map[string]any{
		{
			"tag":        "markdown",
			"element_id": streamingElementStatusBanner,
			"content":    bannerMd,
		},
	}

	// Thinking panel — only included when thinkingText is non-empty.
	if thinkingText != "" {
		thinkingMd := renderThinkingContent(core.SlotContent{ThinkingText: thinkingText})
		elements = append(elements, map[string]any{
			"tag":              "collapsible_panel",
			"element_id":       streamingElementThinkingPanel,
			"expanded":         true,
			"background_color": "grey",
			"header": map[string]any{
				"title": map[string]any{"tag": "plain_text", "content": "Thinking"},
			},
			"border":           map[string]any{"color": "grey"},
			"vertical_spacing": "8px",
			"padding":          "4px 8px",
			"elements": []map[string]any{
				{
					"tag":        "markdown",
					"element_id": streamingElementThinkingMd,
					"content":    thinkingMd,
				},
			},
		})
	}

	// Tools panel — always present, starts collapsed.
	elements = append(elements, map[string]any{
		"tag":              "collapsible_panel",
		"element_id":       streamingElementToolsPanel,
		"expanded":         false,
		"background_color": "grey",
		"header": map[string]any{
			"title": map[string]any{"tag": "plain_text", "content": "Tools"},
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

	// Main text — initially empty, filled via streaming text updates.
	elements = append(elements, map[string]any{
		"tag":        "markdown",
		"element_id": streamingElementMainText,
		"content":    "",
	})

	// Footer note — initially empty, filled with status info at finalization.
	elements = append(elements, map[string]any{
		"tag":        "markdown",
		"element_id": streamingElementFooterNote,
		"content":    "",
		"text_size":  "notation",
		"text_color": "grey",
	})

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

// resolveSlotElementID maps a core StreamingSlotID to the Feishu element_id
// used in the streaming card skeleton.
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
		return ""
	}
}

// renderSlotContent renders slot-specific markdown content for a given slot.
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
