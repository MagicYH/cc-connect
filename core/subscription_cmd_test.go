package core

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// spyPlatform records replies for test assertions.
type spyPlatform struct {
	stubPlatformEngine
	mu      sync.Mutex
	replies []string
}

func (s *spyPlatform) Reply(_ context.Context, _ any, content string) error {
	s.mu.Lock()
	s.replies = append(s.replies, content)
	s.mu.Unlock()
	return nil
}

func (s *spyPlatform) Send(_ context.Context, _ any, content string) error {
	return s.Reply(context.Background(), nil, content)
}

func (s *spyPlatform) lastReply() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.replies) == 0 {
		return ""
	}
	return s.replies[len(s.replies)-1]
}

func newSubscribeTestEngine(t *testing.T) (*Engine, *spyPlatform, *SubscriptionManager) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir)
	p := &spyPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetSubscriptionManager(sm)
	return e, p, sm
}

func testSubMsg(userID string) *Message {
	return &Message{
		Content:   "/subscribe",
		UserID:    userID,
		ChatName:  "test-chat",
		SessionKey: "feishu:oc_chat123:user456",
	}
}

func TestCmdSubscribe_NotAvailable(t *testing.T) {
	p := &spyPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	// No subscriptionManager set — should report not available
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, nil)

	if !strings.Contains(p.lastReply(), "not available") {
		t.Errorf("expected not available message, got: %s", p.lastReply())
	}
}

func TestCmdSubscribe_Help(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, nil)

	if !strings.Contains(p.lastReply(), "subscribe") || !strings.Contains(p.lastReply(), "Create") {
		t.Errorf("expected help message, got: %s", p.lastReply())
	}
}

func TestCmdSubscribe_Add(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, []string{"error"})

	reply := p.lastReply()
	if !strings.Contains(reply, "created") && !strings.Contains(reply, "ID:") {
		t.Errorf("expected created message, got: %s", reply)
	}

	subs := sm.Store().ListByProject("test")
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	s := subs[0]
	if s.Filter != "error" {
		t.Errorf("expected filter 'error', got %q", s.Filter)
	}
	if s.Prompt != "{{content}}" {
		t.Errorf("expected default prompt '{{content}}', got %q", s.Prompt)
	}
	if s.Interval != "*/5 * * * *" {
		t.Errorf("expected default interval, got %q", s.Interval)
	}
	if s.ChatID != "oc_chat123" {
		t.Errorf("expected chatID 'oc_chat123', got %q", s.ChatID)
	}
	if s.Platform != "feishu" {
		t.Errorf("expected platform 'feishu', got %q", s.Platform)
	}
}

func TestCmdSubscribe_AddWithFlags(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, []string{"error", "--exclude", "debug", "--prompt", "check this", "--interval", "*/10 * * * *"})

	subs := sm.Store().ListByProject("test")
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	s := subs[0]
	if s.Filter != "error" {
		t.Errorf("expected filter 'error', got %q", s.Filter)
	}
	if s.ExcludeFilter != "debug" {
		t.Errorf("expected exclude 'debug', got %q", s.ExcludeFilter)
	}
	if s.Prompt != "check this" {
		t.Errorf("expected prompt 'check this', got %q", s.Prompt)
	}
	if s.Interval != "*/10 * * * *" {
		t.Errorf("expected interval '*/10 * * * *', got %q", s.Interval)
	}

	reply := p.lastReply()
	if !strings.Contains(reply, "debug") {
		t.Errorf("expected reply to contain 'debug', got: %s", reply)
	}
}

func TestCmdSubscribe_AddShortFlags(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, []string{"warn", "-e", "noise", "-p", "review", "-i", "0 * * * *"})

	subs := sm.Store().ListByProject("test")
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	s := subs[0]
	if s.ExcludeFilter != "noise" {
		t.Errorf("expected exclude 'noise', got %q", s.ExcludeFilter)
	}
	if s.Prompt != "review" {
		t.Errorf("expected prompt 'review', got %q", s.Prompt)
	}
	if s.Interval != "0 * * * *" {
		t.Errorf("expected interval '0 * * * *', got %q", s.Interval)
	}
}

func TestCmdSubscribe_AddDuplicate(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	// Add first
	e.cmdSubscribe(p, msg, []string{"error"})
	// Try to add again in same chat
	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()
	e.cmdSubscribe(p, msg, []string{"other"})

	reply := p.lastReply()
	if !strings.Contains(reply, "already") {
		t.Errorf("expected already-exists message, got: %s", reply)
	}
}

func TestCmdSubscribe_AddInvalidInterval(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, []string{"error", "--interval", "invalid"})

	reply := p.lastReply()
	if !strings.Contains(reply, "Error") && !strings.Contains(reply, "invalid") {
		t.Errorf("expected error message for invalid interval, got: %s", reply)
	}
}

func TestCmdSubscribe_List(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	// Add a subscription first
	e.cmdSubscribe(p, msg, []string{"error"})
	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	// List for current chat
	e.cmdSubscribe(p, msg, []string{"list"})

	reply := p.lastReply()
	if !strings.Contains(reply, "error") {
		t.Errorf("expected list to contain 'error', got: %s", reply)
	}
	if !strings.Contains(reply, "✅") {
		t.Errorf("expected enabled icon, got: %s", reply)
	}
}

func TestCmdSubscribe_ListEmpty(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, []string{"list"})

	reply := p.lastReply()
	if reply == "" {
		t.Error("expected non-empty reply for empty list")
	}
}

func TestCmdSubscribe_ListAll(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.cmdSubscribe(p, msg, []string{"error"})
	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	e.cmdSubscribe(p, msg, []string{"list", "all"})

	reply := p.lastReply()
	if !strings.Contains(reply, "error") {
		t.Errorf("expected list all to contain 'error', got: %s", reply)
	}
}

func TestCmdSubscribe_Show(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.cmdSubscribe(p, msg, []string{"error"})
	subs := sm.Store().ListByProject("test")
	subID := subs[0].ID

	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	e.cmdSubscribe(p, msg, []string{"show", subID})

	reply := p.lastReply()
	if !strings.Contains(reply, subID) {
		t.Errorf("expected show to contain ID %q, got: %s", subID, reply)
	}
	if !strings.Contains(reply, "error") {
		t.Errorf("expected show to contain filter, got: %s", reply)
	}
}

func TestCmdSubscribe_ShowNotFound(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")
	e.cmdSubscribe(p, msg, []string{"show", "nonexistent"})

	reply := p.lastReply()
	if !strings.Contains(reply, "not found") {
		t.Errorf("expected not found message, got: %s", reply)
	}
}

func TestCmdSubscribe_EnableDisable(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.cmdSubscribe(p, msg, []string{"error"})
	subs := sm.Store().ListByProject("test")
	subID := subs[0].ID

	// Set admin so enable/disable works
	e.userRolesMu.Lock()
	e.adminFrom = "*"
	e.userRolesMu.Unlock()

	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	// Disable
	e.cmdSubscribe(p, msg, []string{"disable", subID})
	reply := p.lastReply()
	if !strings.Contains(reply, "disabled") {
		t.Errorf("expected disabled message, got: %s", reply)
	}

	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	// Enable
	e.cmdSubscribe(p, msg, []string{"enable", subID})
	reply = p.lastReply()
	if !strings.Contains(reply, "enabled") {
		t.Errorf("expected enabled message, got: %s", reply)
	}
}

func TestCmdSubscribe_EnableDisable_AdminRequired(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.cmdSubscribe(p, msg, []string{"error"})
	subs := sm.Store().ListByProject("test")
	subID := subs[0].ID

	// No admin set — should be denied
	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	e.cmdSubscribe(p, msg, []string{"disable", subID})
	reply := p.lastReply()
	if !strings.Contains(reply, "admin") {
		t.Errorf("expected admin required message, got: %s", reply)
	}
}

func TestCmdSubscribe_EnableDisable_NotFound(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.userRolesMu.Lock()
	e.adminFrom = "*"
	e.userRolesMu.Unlock()

	e.cmdSubscribe(p, msg, []string{"enable", "nonexistent"})
	reply := p.lastReply()
	if !strings.Contains(reply, "not found") {
		t.Errorf("expected not found message, got: %s", reply)
	}
}

func TestCmdSubscribe_Remove(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.cmdSubscribe(p, msg, []string{"error"})
	subs := sm.Store().ListByProject("test")
	subID := subs[0].ID

	e.userRolesMu.Lock()
	e.adminFrom = "*"
	e.userRolesMu.Unlock()

	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	e.cmdSubscribe(p, msg, []string{"del", subID})
	reply := p.lastReply()
	if !strings.Contains(reply, "deleted") {
		t.Errorf("expected deleted message, got: %s", reply)
	}

	subs = sm.Store().ListByProject("test")
	if len(subs) != 0 {
		t.Errorf("expected 0 subscriptions after delete, got %d", len(subs))
	}
}

func TestCmdSubscribe_Remove_AdminRequired(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.cmdSubscribe(p, msg, []string{"error"})
	subs := sm.Store().ListByProject("test")
	subID := subs[0].ID

	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	e.cmdSubscribe(p, msg, []string{"rm", subID})
	reply := p.lastReply()
	if !strings.Contains(reply, "admin") {
		t.Errorf("expected admin required message, got: %s", reply)
	}
}

func TestCmdSubscribe_Remove_NotFound(t *testing.T) {
	e, p, _ := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.userRolesMu.Lock()
	e.adminFrom = "*"
	e.userRolesMu.Unlock()

	e.cmdSubscribe(p, msg, []string{"delete", "nonexistent"})
	reply := p.lastReply()
	if !strings.Contains(reply, "not found") {
		t.Errorf("expected not found message, got: %s", reply)
	}
}

func TestCmdSubscribe_DefaultAdd(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	// "/subscribe error" without "add" should still create a subscription
	e.cmdSubscribe(p, msg, []string{"error"})

	subs := sm.Store().ListByProject("test")
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Filter != "error" {
		t.Errorf("expected filter 'error', got %q", subs[0].Filter)
	}
}

func TestCmdSubscribe_RemoveAliases(t *testing.T) {
	e, p, sm := newSubscribeTestEngine(t)
	msg := testSubMsg("user1")

	e.userRolesMu.Lock()
	e.adminFrom = "*"
	e.userRolesMu.Unlock()

	e.cmdSubscribe(p, msg, []string{"error"})
	subs := sm.Store().ListByProject("test")
	subID := subs[0].ID

	p.mu.Lock()
	p.replies = nil
	p.mu.Unlock()

	// Test "remove" alias
	e.cmdSubscribe(p, msg, []string{"remove", subID})
	reply := p.lastReply()
	if !strings.Contains(reply, "deleted") {
		t.Errorf("expected deleted message via 'remove' alias, got: %s", reply)
	}
}
