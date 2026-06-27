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

	result, err := p.BuildThreadReplyCtx("oc_123", "om_456")
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
	wantKey := "feishu:oc_123"
	if rc.sessionKey != wantKey {
		t.Errorf("sessionKey: got %q, want %q", rc.sessionKey, wantKey)
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
			name:    "post message",
			msgType: "post",
			content: `{"content":[[{"tag":"text","text":"line1"},{"tag":"at","user_id":"u1"}],[{"tag":"text","text":"line2"}]]}`,
			want:    "line1\nline2",
		},
		{
			name:    "image message",
			msgType: "image",
			content: `{"image_key":"img1"}`,
			want:    "[image]",
		},
		{
			name:    "empty text",
			msgType: "text",
			content: `{"text":""}`,
			want:    "",
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
