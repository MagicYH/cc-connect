package feishu

import (
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

// Compile-time interface compliance checks.
func TestSubscriptionInterfaceCompliance(t *testing.T) {
	var _ core.MessageScanner = (*Platform)(nil)
	var _ core.ThreadReplyContextBuilder = (*Platform)(nil)
}

func TestBuildThreadReplyCtx(t *testing.T) {
	p := &Platform{platformName: "feishu"}

	result, threadKey, err := p.BuildThreadReplyCtx("feishu:oc_123:bot_abc", "oc_123", "om_456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rc, ok := result.(replyContext)
	if !ok {
		t.Fatalf("expected replyContext, got %T", result)
	}

	if rc.chatID != "oc_123" {
		t.Errorf("chatID: got %q, want %q", rc.chatID, "oc_123")
	}
	if rc.messageID != "om_456" {
		t.Errorf("messageID: got %q, want %q", rc.messageID, "om_456")
	}
	if rc.sessionKey != "feishu:oc_123:root:om_456" {
		t.Errorf("sessionKey: got %q, want %q", rc.sessionKey, "feishu:oc_123:root:om_456")
	}
	if !rc.forceThreadReply {
		t.Errorf("forceThreadReply: got false, want true")
	}
	if threadKey != "feishu:oc_123:root:om_456" {
		t.Errorf("threadSessionKey: got %q, want %q", threadKey, "feishu:oc_123:root:om_456")
	}
}

func TestExtractPlainText(t *testing.T) {
	tests := []struct {
		name    string
		msgType string
		content string
		want    string
	}{
		{
			name:    "text message",
			msgType: "text",
			content: `{"text":"hello world"}`,
			want:    "hello world",
		},
		{
			name:    "post message delegates to extractPostPlainText",
			msgType: "post",
			content: `{"zh_cn":{"title":"Hello","content":[[{"tag":"text","text":"line1"}]]}}`,
			want:    "Hello\nline1",
		},
		{
			name:    "unknown message type returns raw content",
			msgType: "image",
			content: `{"image_key":"img1"}`,
			want:    `{"image_key":"img1"}`,
		},
		{
			name:    "empty text returns raw content",
			msgType: "text",
			content: `{"text":""}`,
			want:    `{"text":""}`,
		},
		{
			name:    "interactive delegates to extractInteractiveCardText",
			msgType: "interactive",
			content: `{"config":{},"elements":[{"tag":"div","text":{"tag":"plain_text","content":"card text"}}]}`,
			want:    "card text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPlainText(tt.msgType, tt.content)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
