package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/chenhg5/cc-connect/core"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// ListMessages retrieves message history from a Feishu chat using the
// Im.Message.List API. It implements core.MessageScanner.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (p *Platform) ListMessages(ctx context.Context, chatID string, opts core.ListMessagesOptions) ([]core.ScannedMessage, string, error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	builder := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		PageSize(pageSize).
		SortType(larkim.SortTypeListMessageByCreateTimeAsc)

	if !opts.Since.IsZero() {
		builder.StartTime(strconv.FormatInt(opts.Since.Unix(), 10))
	}
	if opts.PageToken != "" {
		builder.PageToken(opts.PageToken)
	}

	req := builder.Build()

	var resp *larkim.ListMessageResp
	if err := p.withTransientRetry(ctx, "list messages", func() error {
		return p.withFreshTenantAccessTokenRetry(ctx, "list messages", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			var err error
			resp, err = client.Im.Message.List(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: list messages api call: %w", p.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: list messages failed code=%d msg=%s", p.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	}); err != nil {
		return nil, "", err
	}

	if resp == nil || resp.Data == nil || len(resp.Data.Items) == 0 {
		return nil, "", nil
	}

	var messages []core.ScannedMessage
	for _, item := range resp.Data.Items {
		if item == nil {
			continue
		}
		sm := core.ScannedMessage{
			MessageID: stringValue(item.MessageId),
			ChatID:    stringValue(item.ChatId),
			MsgType:   stringValue(item.MsgType),
		}

		// Sender info
		if item.Sender != nil {
			sm.UserID = stringValue(item.Sender.Id)
			sm.IsBot = stringValue(item.Sender.SenderType) == "app"
		}

		// CreatedAt: Feishu returns millisecond timestamp as string
		if ct := stringValue(item.CreateTime); ct != "" {
			if ms, err := strconv.ParseInt(ct, 10, 64); err == nil {
				sm.CreatedAt = time.UnixMilli(ms)
			}
		}

		// Content extraction
		sm.IsCard = sm.MsgType == "interactive"
		if item.Body != nil && item.Body.Content != nil {
			content := *item.Body.Content
			if sm.IsCard {
				slog.Debug("subscription: raw card JSON", "msg_id", sm.MessageID, "card_json", truncateString(content, 2000))
				sm.Content = extractInteractiveCardText(content)
			} else {
				sm.Content = extractPlainText(sm.MsgType, content)
			}
		}

		messages = append(messages, sm)
	}

	var nextToken string
	if resp.Data.PageToken != nil {
		nextToken = stringValue(resp.Data.PageToken)
	}

	return messages, nextToken, nil
}

// BuildThreadReplyCtx constructs a replyContext targeting a specific message
// for reply-in-thread. It implements core.ThreadReplyContextBuilder.
func (p *Platform) BuildThreadReplyCtx(sessionKey string, chatID string, messageID string) (any, string, error) {
	threadSessionKey := fmt.Sprintf("%s:%s:root:%s", p.tag(), chatID, messageID)
	return replyContext{
		chatID:           chatID,
		messageID:        messageID,
		sessionKey:       threadSessionKey,
		forceThreadReply: true,
	}, threadSessionKey, nil
}

// extractPlainText extracts human-readable text from a Feishu message body
// based on the message type. For unhandled types it returns a placeholder.
func extractPlainText(msgType, content string) string {
	switch msgType {
	case "interactive":
		return extractInteractiveCardText(content)
	case "post":
		return extractPostPlainText(content)
	case "text":
		var body struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(content), &body) == nil && body.Text != "" {
			return body.Text
		}
		return content
	default:
		return content
	}
}
