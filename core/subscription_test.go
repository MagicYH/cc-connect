package core

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestSubscription(id, project, chatID string) *Subscription {
	return &Subscription{
		ID:         id,
		Project:    project,
		ChatID:     chatID,
		Platform:   "feishu",
		SessionKey: "feishu:" + chatID,
		Prompt:     "check alerts",
		Interval:   "5m",
		Enabled:    true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func TestSubscriptionStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Add
	sub := newTestSubscription("sub1", "proj1", "chat1")
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Get
	got := store.Get("sub1")
	if got == nil {
		t.Fatal("Get(sub1) returned nil")
	}
	if got.Project != "proj1" || got.ChatID != "chat1" {
		t.Errorf("Get(sub1) = project=%q chatID=%q, want proj1/chat1", got.Project, got.ChatID)
	}

	// Get nonexistent
	if store.Get("nope") != nil {
		t.Error("Get(nope) should return nil")
	}

	// Update
	if err := store.Update("sub1", map[string]any{"prompt": "new prompt"}); err != nil {
		t.Fatal(err)
	}
	got = store.Get("sub1")
	if got.Prompt != "new prompt" {
		t.Errorf("after Update, prompt = %q, want %q", got.Prompt, "new prompt")
	}

	// Remove
	if err := store.Remove("sub1"); err != nil {
		t.Fatal(err)
	}
	if store.Get("sub1") != nil {
		t.Error("Get(sub1) should return nil after Remove")
	}
}

func TestSubscriptionStore_UniquenessConstraint(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub1 := newTestSubscription("sub1", "proj1", "chat1")
	if err := store.Add(sub1); err != nil {
		t.Fatal(err)
	}

	// Same Project+ChatID, different ID — should fail
	sub2 := newTestSubscription("sub2", "proj1", "chat1")
	if err := store.Add(sub2); err == nil {
		t.Error("Add with duplicate Project+ChatID should return error")
	}

	// Different ChatID — should succeed
	sub3 := newTestSubscription("sub3", "proj1", "chat2")
	if err := store.Add(sub3); err != nil {
		t.Fatal(err)
	}

	// Different Project — should succeed
	sub4 := newTestSubscription("sub4", "proj2", "chat1")
	if err := store.Add(sub4); err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptionStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-persist", "proj1", "chat1")
	sub.Prompt = "persist me"
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Reload from same directory
	store2, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	got := store2.Get("sub-persist")
	if got == nil {
		t.Fatal("subscription not found after reload")
	}
	if got.Prompt != "persist me" {
		t.Errorf("after reload, prompt = %q, want %q", got.Prompt, "persist me")
	}
}

func TestSubscriptionStore_UpdateAnchor(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-anchor", "proj1", "chat1")
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	if err := store.UpdateAnchor("sub-anchor", "anchor-123", []string{"id1", "id2"}); err != nil {
		t.Fatal(err)
	}

	got := store.Get("sub-anchor")
	if got.Anchor != "anchor-123" {
		t.Errorf("Anchor = %q, want %q", got.Anchor, "anchor-123")
	}
	if len(got.ProcessedIDs) != 2 || got.ProcessedIDs[0] != "id1" || got.ProcessedIDs[1] != "id2" {
		t.Errorf("ProcessedIDs = %v, want [id1 id2]", got.ProcessedIDs)
	}

	// Nonexistent subscription
	if err := store.UpdateAnchor("nope", "x", nil); err == nil {
		t.Error("UpdateAnchor on nonexistent ID should return error")
	}
}

func TestSubscriptionStore_MarkRun_Success(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-mr", "proj1", "chat1")
	sub.ConsecutiveErrors = 5
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	before := time.Now()
	if err := store.MarkRun("sub-mr", "", false); err != nil {
		t.Fatal(err)
	}
	after := time.Now()

	got := store.Get("sub-mr")
	if got.LastRun.IsZero() {
		t.Error("LastRun should be set after MarkRun")
	}
	if got.LastRun.Before(before) || got.LastRun.After(after) {
		t.Error("LastRun should be between before and after MarkRun call")
	}
	if got.LastError != "" {
		t.Errorf("LastError = %q on success, want empty", got.LastError)
	}
	if got.ConsecutiveErrors != 0 {
		t.Errorf("ConsecutiveErrors = %d on success, want 0", got.ConsecutiveErrors)
	}
}

func TestSubscriptionStore_MarkRun_PermanentError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-pe", "proj1", "chat1")
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Permanent error increments ConsecutiveErrors
	if err := store.MarkRun("sub-pe", "bad thing", true); err != nil {
		t.Fatal(err)
	}
	got := store.Get("sub-pe")
	if got.ConsecutiveErrors != 1 {
		t.Errorf("ConsecutiveErrors = %d, want 1", got.ConsecutiveErrors)
	}
	if got.LastError != "bad thing" {
		t.Errorf("LastError = %q, want %q", got.LastError, "bad thing")
	}
}

func TestSubscriptionStore_MarkRun_TransientError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-te", "proj1", "chat1")
	sub.ConsecutiveErrors = 3
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Transient error should NOT increment ConsecutiveErrors but should record LastError
	if err := store.MarkRun("sub-te", "timeout", false); err != nil {
		t.Fatal(err)
	}
	got := store.Get("sub-te")
	if got.ConsecutiveErrors != 3 {
		t.Errorf("ConsecutiveErrors = %d on transient error, want 3 (unchanged)", got.ConsecutiveErrors)
	}
	if got.LastError != "timeout" {
		t.Errorf("LastError = %q, want %q", got.LastError, "timeout")
	}
}

func TestSubscriptionStore_MarkRun_AutoDisable(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-ad", "proj1", "chat1")
	sub.Enabled = true
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Accumulate 9 permanent errors
	for i := 0; i < 9; i++ {
		if err := store.MarkRun("sub-ad", "error", true); err != nil {
			t.Fatal(err)
		}
	}
	got := store.Get("sub-ad")
	if got.ConsecutiveErrors != 9 {
		t.Fatalf("ConsecutiveErrors = %d after 9 errors, want 9", got.ConsecutiveErrors)
	}
	if !got.Enabled {
		t.Error("subscription should still be enabled after 9 errors")
	}

	// 10th permanent error auto-disables
	if err := store.MarkRun("sub-ad", "error", true); err != nil {
		t.Fatal(err)
	}
	got = store.Get("sub-ad")
	if got.ConsecutiveErrors != 10 {
		t.Errorf("ConsecutiveErrors = %d, want 10", got.ConsecutiveErrors)
	}
	if got.Enabled {
		t.Error("subscription should be auto-disabled after 10 consecutive errors")
	}
}

func TestSubscriptionStore_MarkRun_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.MarkRun("nope", "", false); err == nil {
		t.Error("MarkRun on nonexistent ID should return error")
	}
}

func TestSubscriptionStore_ListByProject(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	subs := []*Subscription{
		newTestSubscription("s1", "proj1", "chat1"),
		newTestSubscription("s2", "proj1", "chat2"),
		newTestSubscription("s3", "proj2", "chat3"),
	}
	for _, s := range subs {
		if err := store.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	list := store.ListByProject("proj1")
	if len(list) != 2 {
		t.Errorf("ListByProject(proj1) = %d, want 2", len(list))
	}

	list2 := store.ListByProject("proj2")
	if len(list2) != 1 {
		t.Errorf("ListByProject(proj2) = %d, want 1", len(list2))
	}

	if len(store.ListByProject("proj3")) != 0 {
		t.Error("ListByProject for unknown project should return empty")
	}
}

func TestSubscriptionStore_ListByChatID(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	subs := []*Subscription{
		newTestSubscription("s1", "proj1", "chat1"),
		newTestSubscription("s2", "proj2", "chat1"),
		newTestSubscription("s3", "proj1", "chat2"),
	}
	for _, s := range subs {
		if err := store.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	list := store.ListByChatID("chat1")
	if len(list) != 2 {
		t.Errorf("ListByChatID(chat1) = %d, want 2", len(list))
	}

	if len(store.ListByChatID("chat3")) != 0 {
		t.Error("ListByChatID for unknown chat should return empty")
	}
}

func TestSubscriptionStore_ListAll(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	subs := []*Subscription{
		newTestSubscription("s1", "proj1", "chat1"),
		newTestSubscription("s2", "proj2", "chat2"),
	}
	for _, s := range subs {
		if err := store.Add(s); err != nil {
			t.Fatal(err)
		}
	}

	all := store.ListAll()
	if len(all) != 2 {
		t.Errorf("ListAll = %d, want 2", len(all))
	}
}

func TestSubscriptionStore_Update(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-upd", "proj1", "chat1")
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}
	origUpdatedAt := sub.UpdatedAt

	if err := store.Update("sub-upd", map[string]any{
		"prompt":   "updated prompt",
		"interval": "10m",
		"enabled":  false,
	}); err != nil {
		t.Fatal(err)
	}

	got := store.Get("sub-upd")
	if got.Prompt != "updated prompt" {
		t.Errorf("prompt = %q, want %q", got.Prompt, "updated prompt")
	}
	if got.Interval != "10m" {
		t.Errorf("interval = %q, want %q", got.Interval, "10m")
	}
	if got.Enabled {
		t.Error("enabled should be false after update")
	}

	// UpdatedAt should have changed
	if !got.UpdatedAt.After(origUpdatedAt) {
		t.Error("UpdatedAt should be after original UpdatedAt")
	}
}

func TestSubscriptionStore_Update_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Update("nope", map[string]any{"prompt": "x"}); err == nil {
		t.Error("Update on nonexistent ID should return error")
	}
}

func TestSubscriptionStore_Remove_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Remove("nope"); err == nil {
		t.Error("Remove on nonexistent ID should return error")
	}
}

func TestSubscriptionStore_JobsPath(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(dir, "subscriptions", "jobs.json")
	if store.path != expected {
		t.Errorf("path = %q, want %q", store.path, expected)
	}
}

func TestGenerateSubscriptionID(t *testing.T) {
	id := GenerateSubscriptionID()
	if len(id) != 16 {
		t.Errorf("GenerateSubscriptionID() = %q (len %d), want 16 hex chars", id, len(id))
	}
	// Should be hex
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char %q in subscription ID", c)
			break
		}
	}

	// Should be unique
	id2 := GenerateSubscriptionID()
	if id == id2 {
		t.Error("two GenerateSubscriptionID calls returned the same ID")
	}
}

func TestSubscriptionFilter(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "【告警】CPU超过90%", IsBot: false},
		{MessageID: "m2", Content: "【恢复】CPU已正常", IsBot: false},
		{MessageID: "m3", Content: "今日天气不错", IsBot: false},
		{MessageID: "m4", Content: "Bot消息", IsBot: true},
	}
	sub := &Subscription{Filter: "告警", ExcludeFilter: "恢复"}
	matched := filterMessages(sub, msgs)
	if len(matched) != 1 || matched[0].MessageID != "m1" {
		t.Errorf("filterMessages = %v, want 1 match with m1", matched)
	}
}

func TestSubscriptionFilterEmpty(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "消息1", IsBot: false},
		{MessageID: "m2", Content: "消息2", IsBot: true},
	}
	sub := &Subscription{Filter: "", ExcludeFilter: ""}
	matched := filterMessages(sub, msgs)
	if len(matched) != 1 {
		t.Errorf("filterMessages empty = %d, want 1 (bot excluded)", len(matched))
	}
}

func TestBuildPrompt(t *testing.T) {
	sub := &Subscription{Prompt: "排查：{{content}}"}
	result := sub.BuildPrompt("CPU超过90%")
	if result != "排查：CPU超过90%" {
		t.Errorf("BuildPrompt = %q, want %q", result, "排查：CPU超过90%")
	}
}
