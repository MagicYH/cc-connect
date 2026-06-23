package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/chenhg5/cc-connect/core"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
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
	maxToolResultLen = 300 // max runes for tool result in streaming timeline
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

// renderThinkingContent returns the thinking text for the thinking slot, or empty string if none.
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

	b, err := json.Marshal(card)
	if err != nil {
		slog.Error("feishu: marshal streaming card skeleton", "error", err)
		return "{}"
	}
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
		slog.Warn("feishu: unknown StreamingSlotID", "slot", slot)
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

// StreamSlotContent patches a single slot's markdown via cardElement.content().
// Returns ErrSlotRateLimited on rate limit (engine triggers degradation).
// Returns ErrSlotNotSupported if cardEntity is unavailable.
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
	if elementID == "" {
		return fmt.Errorf("%s: StreamSlotContent: unknown slot %q", p.tag(), slot)
	}
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
			slog.Debug(p.tag()+": stream slot content rate limited; skipping frame", "slot", slot, "code", resp.Code)
			return core.ErrSlotRateLimited
		}
		return fmt.Errorf("%s: stream slot content: %w", p.tag(), err)
	}

	slog.Debug(
		p.tag()+": stream slot content success",
		"slot", slot,
		"element_id", elementID,
		"content_len", len(markdown),
		"sequence", seq,
	)
	return nil
}

// BuildStreamingCard creates the initial multi-slot card skeleton, creates a
// cardEntity for slot-level patching, sends the card as a new message, and
// returns a handle.
func (p *Platform) BuildStreamingCard(ctx context.Context, chatID string, status core.CardStatus, title string) (any, error) {
	if !p.useInteractiveCard {
		return nil, core.ErrSlotNotSupported
	}

	cardJSON := buildStreamingCardSkeleton(status, "")

	cardID, err := p.createCardEntity(ctx, cardJSON)
	if err != nil {
		slog.Info(p.tag()+": streaming card create cardEntity failed, falling back", "error", err)
		return nil, core.ErrSlotNotSupported
	}

	sendContent := fmt.Sprintf(`{"type":"card","data":{"card_id":"%s"}}`, cardID)

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeInteractive).
			Content(sendContent).
			Build()).
		Build()

	var resp *larkim.CreateMessageResp
	if err := p.withTransientRetry(ctx, "build streaming card", func() error {
		return p.withFreshTenantAccessTokenRetry(ctx, "build streaming card", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Im.Message.Create(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: build streaming card: %w", p.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: build streaming card code=%d msg=%s", p.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	}); err != nil {
		return nil, err
	}

	msgID := ""
	if resp.Data != nil && resp.Data.MessageId != nil {
		msgID = *resp.Data.MessageId
	}
	if msgID == "" {
		return nil, fmt.Errorf("%s: build streaming card: no message ID returned", p.tag())
	}

	return &feishuPreviewHandle{
		messageID: msgID,
		chatID:    chatID,
		cardID:    cardID,
		status:    status,
	}, nil
}

// flushController tracks per-slot content hashes for dedup and manages
// pending dirty slots for the next flush cycle.
type flushController struct {
	mu          sync.Mutex
	lastHash    map[string]uint32 // slot element_id → fnv32 hash of last rendered content
	lastContent map[string]string // slot element_id → last rendered content
	pending     map[string]string // slot element_id → content to flush
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
			return false
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
		{core.SlotStatusBanner, core.SlotContent{Phase: core.PhaseDone}},
		{core.SlotThinking, core.SlotContent{ThinkingText: ""}},
		{core.SlotTools, core.SlotContent{ToolSteps: steps}},
		{core.SlotMainText, core.SlotContent{MainText: markdown}},
		{core.SlotFooterNote, core.SlotContent{StatusFooter: statusFooter}},
	}
	for _, s := range phase1Slots {
		_ = p.StreamSlotContent(ctx, h, s.slot, s.content)
		// Phase 1 errors are not fatal — content may already be correct
	}
	slog.Debug(p.tag()+": finalize streaming card Phase 1 complete", "slots_patched", len(phase1Slots))

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
