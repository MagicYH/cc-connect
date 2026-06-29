package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func newTestSubscription(id, project, chatID string) *Subscription {
	return &Subscription{
		ID:         id,
		Project:    project,
		ChatID:     chatID,
		Platform:   "feishu",
		SessionKey: "feishu:" + chatID,
		Prompt:     "check alerts",
		Interval:   "*/5 * * * *",
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

func TestSubscriptionStore_UpdateAnchor_PruneProcessedIDs(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-prune", "proj1", "chat1")
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Create 150 IDs
	ids := make([]string, 150)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%d", i)
	}
	if err := store.UpdateAnchor("sub-prune", "anchor-big", ids); err != nil {
		t.Fatal(err)
	}

	got := store.Get("sub-prune")
	if len(got.ProcessedIDs) != 100 {
		t.Errorf("ProcessedIDs length = %d, want 100 (pruned from 150)", len(got.ProcessedIDs))
	}
	// Should keep the last 100 (id50..id149)
	if got.ProcessedIDs[0] != "id50" {
		t.Errorf("first ProcessedID = %q, want %q", got.ProcessedIDs[0], "id50")
	}
	if got.ProcessedIDs[99] != "id149" {
		t.Errorf("last ProcessedID = %q, want %q", got.ProcessedIDs[99], "id149")
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
		"interval": "*/10 * * * *",
		"enabled":  false,
	}); err != nil {
		t.Fatal(err)
	}

	got := store.Get("sub-upd")
	if got.Prompt != "updated prompt" {
		t.Errorf("prompt = %q, want %q", got.Prompt, "updated prompt")
	}
	if got.Interval != "*/10 * * * *" {
		t.Errorf("interval = %q, want %q", got.Interval, "*/10 * * * *")
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
		{MessageID: "m1", Content: "【告警】CPU超过90%", IsBot: true},
		{MessageID: "m2", Content: "【恢复】CPU已正常", IsBot: true},
		{MessageID: "m3", Content: "今日天气不错", IsBot: false},
		{MessageID: "m4", Content: "Bot消息", IsBot: true},
	}
	filterRe := regexp.MustCompile("(?i)告警")
	excludeRe := regexp.MustCompile("(?i)恢复")
	matched, err := filterMessages(msgs, filterRe, excludeRe, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 || matched[0].MessageID != "m1" {
		t.Errorf("filterMessages = %v, want 1 match with m1", matched)
	}
}

func TestSubscriptionFilter_CaseInsensitive(t *testing.T) {
	sub := &Subscription{Filter: "warning", ExcludeFilter: "Recovered"}
	if err := sub.compileFilters(); err != nil {
		t.Fatal(err)
	}
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "WARNING: CPU high", IsBot: true},
		{MessageID: "m2", Content: "warning: disk full", IsBot: true},
		{MessageID: "m3", Content: "warning: recovered", IsBot: true},
		{MessageID: "m4", Content: "normal info", IsBot: true},
	}
	matched, err := filterMessages(msgs, sub.filterRe, sub.excludeFilterRe, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 2 {
		t.Errorf("filterMessages case-insensitive = %d matches, want 2 (m1, m2)", len(matched))
	}
	for _, m := range matched {
		if m.MessageID == "m3" {
			t.Error("m3 should be excluded by case-insensitive 'Recovered' filter")
		}
	}
}

func TestSubscriptionFilterEmpty(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "消息1", IsBot: false},
		{MessageID: "m2", Content: "消息2", IsBot: true},
	}
	matched, err := filterMessages(msgs, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	// Only IsBot=true messages are kept
	if len(matched) != 1 || matched[0].MessageID != "m2" {
		t.Errorf("filterMessages empty = %d, want 1 (only bot messages)", len(matched))
	}
}

func TestSubscriptionFilterDashWildcard(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "告警消息", IsBot: true},
		{MessageID: "m2", Content: "normal message", IsBot: false},
	}
	// "-" filter is not compiled to regex (compileFilters skips it), so both
	// filter and exclude are nil — only IsBot check applies
	matched, err := filterMessages(msgs, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 || matched[0].MessageID != "m1" {
		t.Errorf("filterMessages dash = %d, want 1 (only bot messages)", len(matched))
	}
}

func TestSubscriptionFilter_ProcessedIDsDedup(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "alert 1", IsBot: true},
		{MessageID: "m2", Content: "alert 2", IsBot: true},
		{MessageID: "m3", Content: "alert 3", IsBot: true},
	}
	matched, err := filterMessages(msgs, nil, nil, []string{"m1", "m3"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 || matched[0].MessageID != "m2" {
		t.Errorf("filterMessages with processedIDs = %v, want 1 match with m2", matched)
	}
}

func TestBuildPrompt(t *testing.T) {
	sub := &Subscription{Prompt: "排查：{{content}}"}
	result := sub.BuildPrompt("CPU超过90%")
	if result != "排查：CPU超过90%" {
		t.Errorf("BuildPrompt = %q, want %q", result, "排查：CPU超过90%")
	}
}

// ---------------------------------------------------------------------------
// SubscriptionManager tests
// ---------------------------------------------------------------------------

// stubEngine is a minimal Engine that records ExecuteSubscriptionScan calls.
type stubEngine struct {
	mu       sync.Mutex
	calls    []string // subscription IDs that were scanned
	err      error    // error to return from ExecuteSubscriptionScan
	delay    time.Duration
	sessions map[string]*AgentSession
}

func (se *stubEngine) ExecuteSubscriptionScan(sub *Subscription, botID string) error {
	se.mu.Lock()
	se.calls = append(se.calls, sub.ID)
	err := se.err
	delay := se.delay
	se.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	return err
}

func newStubEngine() *stubEngine {
	return &stubEngine{
		sessions: make(map[string]*AgentSession),
	}
}

func newTestManager(t *testing.T) (*SubscriptionManager, *stubEngine) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)
	eng := newStubEngine()
	sm.RegisterEngine("proj1", &Engine{})
	return sm, eng
}

func TestSubscriptionManager_AddSubscription_ValidInterval(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub1",
		Project:   "proj1",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "*/5 * * * *",
		Enabled:   false,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription with valid interval: %v", err)
	}
}

func TestSubscriptionManager_AddSubscription_InvalidInterval(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub2",
		Project:   "proj1",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "not-a-cron",
		Enabled:   true,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sm.AddSubscription(sub); err == nil {
		t.Error("AddSubscription with invalid interval should return error")
	}
}

func TestSubscriptionManager_RemoveSubscription(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub-rm",
		Project:   "proj1",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "*/5 * * * *",
		Enabled:   true,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatal(err)
	}

	if err := sm.RemoveSubscription("sub-rm"); err != nil {
		t.Fatalf("RemoveSubscription: %v", err)
	}
	if store.Get("sub-rm") != nil {
		t.Error("subscription should be gone after RemoveSubscription")
	}

	// Remove nonexistent
	if err := sm.RemoveSubscription("nope"); err == nil {
		t.Error("RemoveSubscription on nonexistent should return error")
	}
}

func TestSubscriptionManager_EnableDisableSubscription(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub-en",
		Project:   "proj1",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "*/5 * * * *",
		Enabled:   false,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatal(err)
	}

	// Enable
	if err := sm.EnableSubscription("sub-en"); err != nil {
		t.Fatalf("EnableSubscription: %v", err)
	}
	got := store.Get("sub-en")
	if !got.Enabled {
		t.Error("subscription should be enabled after EnableSubscription")
	}
	// Should have a cron entry
	sm.mu.RLock()
	_, hasEntry := sm.entries["sub-en"]
	sm.mu.RUnlock()
	if !hasEntry {
		t.Error("enabled subscription should have a cron entry")
	}

	// Disable
	if err := sm.DisableSubscription("sub-en"); err != nil {
		t.Fatalf("DisableSubscription: %v", err)
	}
	got = store.Get("sub-en")
	if got.Enabled {
		t.Error("subscription should be disabled after DisableSubscription")
	}
	sm.mu.RLock()
	_, hasEntry = sm.entries["sub-en"]
	sm.mu.RUnlock()
	if hasEntry {
		t.Error("disabled subscription should not have a cron entry")
	}

	// Enable nonexistent
	if err := sm.EnableSubscription("nope"); err == nil {
		t.Error("EnableSubscription on nonexistent should return error")
	}
}

func TestSubscriptionManager_RegisterEngine(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	eng := &Engine{}
	sm.RegisterEngine("myproject", eng)

	sm.mu.RLock()
	got, ok := sm.engines["myproject"]
	sm.mu.RUnlock()
	if !ok || got != eng {
		t.Error("RegisterEngine did not register the engine correctly")
	}
}

func TestSubscriptionManager_ExecuteScan_MarkRun(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	// Build a real engine with a scanner platform
	p := &stubScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		messages: []ScannedMessage{
			{MessageID: "m1", Content: "alert", IsBot: true, CreatedAt: time.Now()},
		},
		rcCtx: "reply-ctx",
	}
	eng := NewEngine("proj1", &stubAgent{}, []Platform{p}, "", LangEnglish)
	eng.SetSubscriptionManager(sm)
	sm.RegisterEngine("proj1", eng)

	sub := &Subscription{
		ID:          "sub-scan",
		Project:     "proj1",
		ChatID:      "chat1",
		Platform:    "feishu",
		SessionKey:  "feishu:chat1:bot",
		Filter:      "alert",
		Interval:    "*/5 * * * *",
		Enabled:     true,
		Prompt:      "{{content}}",
		TimeoutMins: 1, // 1-minute timeout so executeScan doesn't block forever
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatal(err)
	}

	// Cancel the engine context shortly after the scan starts so
	// processInteractiveMessageWith exits cleanly (stub agent never responds).
	go func() {
		time.Sleep(200 * time.Millisecond)
		eng.Stop()
	}()

	sm.executeScan("sub-scan")

	got := store.Get("sub-scan")
	if got.LastRun.IsZero() {
		t.Error("LastRun should be set after executeScan")
	}
}

func TestSubscriptionManager_ExecuteScan_EngineNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub-noeng",
		Project:   "proj_missing",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "*/5 * * * *",
		Enabled:   true,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatal(err)
	}

	sm.executeScan("sub-noeng")

	got := store.Get("sub-noeng")
	if got.LastError == "" {
		t.Error("LastError should be set when engine not found")
	}
	if got.ConsecutiveErrors != 1 {
		t.Errorf("ConsecutiveErrors = %d, want 1", got.ConsecutiveErrors)
	}
}

func TestSubscriptionManager_ExecuteScan_ConcurrencyGuard(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub-conc",
		Project:   "proj1",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "*/5 * * * *",
		Enabled:   true,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatal(err)
	}

	// Simulate an in-progress scan
	sm.running.Store("sub-conc", true)

	// executeScan should skip
	sm.executeScan("sub-conc")

	// The running flag should still be there (we didn't delete it)
	if _, ok := sm.running.Load("sub-conc"); !ok {
		t.Error("running flag should still be set since scan was skipped")
	}

	// Clean up
	sm.running.Delete("sub-conc")
}

func TestSubscriptionManager_ExecuteScan_AutoDisableUnschedule(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	// Use a project with no engine registered — this triggers the engine-not-found
	// MarkRun path in executeScan, which calls unscheduleIfDisabled.
	sub := &Subscription{
		ID:                "sub-auto",
		Project:           "proj_missing",
		ChatID:            "chat1",
		Platform:          "feishu",
		Interval:          "*/5 * * * *",
		Enabled:           true,
		Prompt:            "test",
		ConsecutiveErrors: 9,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatal(err)
	}

	// Should have a cron entry
	sm.mu.RLock()
	_, hasEntry := sm.entries["sub-auto"]
	sm.mu.RUnlock()
	if !hasEntry {
		t.Fatal("subscription should have a cron entry before auto-disable")
	}

	// executeScan will hit engine-not-found, call MarkRun with isPermanent=true,
	// pushing ConsecutiveErrors to 10, which auto-disables, then unscheduleIfDisabled
	// should remove the cron entry.
	sm.executeScan("sub-auto")

	got := store.Get("sub-auto")
	if got.Enabled {
		t.Error("subscription should be auto-disabled after 10th consecutive error")
	}

	sm.mu.RLock()
	_, hasEntry = sm.entries["sub-auto"]
	sm.mu.RUnlock()
	if hasEntry {
		t.Error("auto-disabled subscription should have cron entry removed")
	}
}

func TestSubscriptionManager_Start(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub-start",
		Project:   "proj1",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "*/5 * * * *",
		Enabled:   true,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	if err := sm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sm.Stop()

	sm.mu.RLock()
	_, hasEntry := sm.entries["sub-start"]
	sm.mu.RUnlock()
	if !hasEntry {
		t.Error("enabled subscription should be scheduled after Start")
	}
}

func TestSubscriptionManager_Store(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)
	if sm.Store() != store {
		t.Error("Store() should return the underlying store")
	}
}

func TestSubscriptionManager_AppendLog(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	entry := LogEntry{
		SubscriptionID: "sub-log",
		MessageID:      "msg1",
		ChatID:         "chat1",
		Status:         "sent",
		CreatedAt:      time.Now(),
	}
	if err := sm.AppendLog(entry); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}

	// Verify the log file exists
	logPath := filepath.Join(sm.logsDir, "sub-log.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file should exist at %s: %v", logPath, err)
	}
}

func TestSubscriptionManager_ExecuteScan_DisabledSub(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	sub := &Subscription{
		ID:        "sub-dis",
		Project:   "proj1",
		ChatID:    "chat1",
		Platform:  "feishu",
		Interval:  "*/5 * * * *",
		Enabled:   false,
		Prompt:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatal(err)
	}

	sm.executeScan("sub-dis")

	got := store.Get("sub-dis")
	if !got.LastRun.IsZero() {
		t.Error("disabled subscription should not have LastRun set from executeScan")
	}
}

func TestSubscriptionManager_ExecuteScan_NonexistentSub(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	// Should not panic
	sm.executeScan("nonexistent")
}

func TestSubscriptionManager_NewSubscriptionStore_WrappedError(t *testing.T) {
	// We can't easily test MkdirAll failure with a temp dir, but we can at
	// least verify that NewSubscriptionStore works normally.
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatalf("NewSubscriptionStore: %v", err)
	}
	if store == nil {
		t.Error("store should not be nil")
	}
}

func TestSubscriptionManager_Update_VariableShadowing(t *testing.T) {
	// This test verifies that the Update method correctly handles the
	// variable shadowing fix (val instead of s).
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	sub := newTestSubscription("sub-shadow", "proj1", "chat1")
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Update project field — this used to shadow the receiver 's'
	if err := store.Update("sub-shadow", map[string]any{"project": "proj2"}); err != nil {
		t.Fatal(err)
	}
	got := store.Get("sub-shadow")
	if got.Project != "proj2" {
		t.Errorf("project = %q, want %q", got.Project, "proj2")
	}

	// Update chat_id field too
	if err := store.Update("sub-shadow", map[string]any{"chat_id": "chat2"}); err != nil {
		t.Fatal(err)
	}
	got = store.Get("sub-shadow")
	if got.ChatID != "chat2" {
		t.Errorf("chat_id = %q, want %q", got.ChatID, "chat2")
	}
}

// Verify cron.ParseStandard is used for interval validation
func TestCronParseStandard(t *testing.T) {
	valid := []string{"*/5 * * * *", "0 * * * *", "0 9 * * 1-5"}
	for _, expr := range valid {
		if _, err := cron.ParseStandard(expr); err != nil {
			t.Errorf("ParseStandard(%q) should succeed: %v", expr, err)
		}
	}

	invalid := []string{"not-a-cron", "60 * * * *"}
	for _, expr := range invalid {
		if _, err := cron.ParseStandard(expr); err == nil {
			t.Errorf("ParseStandard(%q) should fail", expr)
		}
	}
}

// ---------------------------------------------------------------------------
// ExecuteSubscriptionScan engine integration tests
// ---------------------------------------------------------------------------

// stubScannerPlatform implements Platform + MessageScanner + ReplyContextReconstructor
type stubScannerPlatform struct {
	stubPlatformEngine
	messages    []ScannedMessage
	nextToken   string
	listErr     error
	listCalls   int
	rcCtx       any
	rcErr       error
	threadCtx   any
	threadErr   error
	threadCalls int
}

func (p *stubScannerPlatform) ListMessages(_ context.Context, _ string, _ ListMessagesOptions) ([]ScannedMessage, string, error) {
	p.listCalls++
	if p.listErr != nil {
		return nil, "", p.listErr
	}
	return p.messages, p.nextToken, nil
}

func (p *stubScannerPlatform) ReconstructReplyCtx(_ string) (any, error) {
	if p.rcErr != nil {
		return nil, p.rcErr
	}
	return p.rcCtx, nil
}

func (p *stubScannerPlatform) BuildThreadReplyCtx(_ string, _ string, _ string) (any, string, error) {
	p.threadCalls++
	if p.threadErr != nil {
		return nil, "", p.threadErr
	}
	return p.threadCtx, "", nil
}

func TestExecuteSubscriptionScan_PlatformNotFound(t *testing.T) {
	e := newTestEngine()
	sub := &Subscription{
		ID:         "sub1",
		Project:    "test",
		ChatID:     "chat1",
		Platform:   "nonexistent",
		SessionKey: "nonexistent:chat1:bot",
		Filter:     "",
		Prompt:     "{{content}}",
	}
	err := e.ExecuteSubscriptionScan(sub, "")
	if err == nil {
		t.Error("expected error when platform not found")
	}
}

func TestExecuteSubscriptionScan_PlatformNotScanner(t *testing.T) {
	p := &stubPlatformEngine{n: "noscan"}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	sub := &Subscription{
		ID:         "sub1",
		Project:    "test",
		ChatID:     "chat1",
		Platform:   "noscan",
		SessionKey: "noscan:chat1:bot",
		Filter:     "",
		Prompt:     "{{content}}",
	}
	err := e.ExecuteSubscriptionScan(sub, "")
	if err == nil {
		t.Error("expected error when platform does not implement MessageScanner")
	}
}

func TestExecuteSubscriptionScan_ListMessagesError(t *testing.T) {
	p := &stubScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		listErr:            fmt.Errorf("API error: unauthorized"),
	}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	sub := &Subscription{
		ID:         "sub1",
		Project:    "test",
		ChatID:     "chat1",
		Platform:   "feishu",
		SessionKey: "feishu:chat1:bot",
		Filter:     "",
		Prompt:     "{{content}}",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	err := e.ExecuteSubscriptionScan(sub, "")
	if err == nil {
		t.Error("expected error when ListMessages fails")
	}
	if !strings.Contains(err.Error(), "list messages") {
		t.Errorf("error should mention list messages, got: %v", err)
	}
}

func TestExecuteSubscriptionScan_NoMatchingMessages(t *testing.T) {
	p := &stubScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		messages: []ScannedMessage{
			{MessageID: "m1", Content: "normal message", IsBot: false},
			{MessageID: "m2", Content: "another message", IsBot: false},
		},
	}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	sub := &Subscription{
		ID:         "sub1",
		Project:    "test",
		ChatID:     "chat1",
		Platform:   "feishu",
		SessionKey: "feishu:chat1:bot",
		Filter:     "告警",
		Prompt:     "{{content}}",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	err := e.ExecuteSubscriptionScan(sub, "")
	if err != nil {
		t.Errorf("expected nil when no messages match, got: %v", err)
	}
}

func TestExecuteSubscriptionScan_MatchingMessages(t *testing.T) {
	p := &stubScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		messages: []ScannedMessage{
			{MessageID: "m1", Content: "【告警】CPU超90%", IsBot: true, CreatedAt: time.Now()},
			{MessageID: "m2", Content: "【恢复】CPU正常", IsBot: true, CreatedAt: time.Now()},
			{MessageID: "m3", Content: "【告警】内存超80%", IsBot: true, CreatedAt: time.Now()},
		},
		rcCtx:     "reply-ctx-base",
		threadCtx: "thread-ctx-1",
	}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)
	e.SetSubscriptionManager(sm)

	sub := &Subscription{
		ID:            "sub1",
		Project:       "test",
		ChatID:        "chat1",
		Platform:      "feishu",
		SessionKey:    "feishu:chat1:bot",
		Filter:        "告警",
		ExcludeFilter: "恢复",
		Prompt:        "排查：{{content}}",
		Interval:      "*/5 * * * *",
		Enabled:       true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	// Cancel the engine context after a short delay so processInteractiveMessageWith exits
	go func() {
		time.Sleep(200 * time.Millisecond)
		e.Stop()
	}()

	err = e.ExecuteSubscriptionScan(sub, "")
	if err != nil {
		t.Fatalf("ExecuteSubscriptionScan: %v", err)
	}

	// Verify ThreadReplyContextBuilder was called for each matched message
	if p.threadCalls != 2 {
		t.Errorf("BuildThreadReplyCtx called %d times, want 2 (2 matching messages)", p.threadCalls)
	}

	// Verify anchor was updated
	got := store.Get("sub1")
	if got == nil {
		t.Fatal("subscription should exist after scan")
	}
	if got.Anchor == "" {
		t.Error("anchor should be updated after scan with messages")
	}
}

func TestExecuteSubscriptionScan_Pagination(t *testing.T) {
	page1 := []ScannedMessage{
		{MessageID: "m1", Content: "alert 1", IsBot: true, CreatedAt: time.Now()},
	}
	page2 := []ScannedMessage{
		{MessageID: "m2", Content: "alert 2", IsBot: true, CreatedAt: time.Now()},
	}

	// Use a paginating scanner that returns pages one at a time
	scanner := &paginatingScanner{
		stubScannerPlatform: &stubScannerPlatform{
			stubPlatformEngine: stubPlatformEngine{n: "feishu"},
			rcCtx:              "reply-ctx",
		},
		pages:  [][]ScannedMessage{page1, page2},
		tokens: []string{"page2token", ""},
	}

	e := NewEngine("test", &stubAgent{}, []Platform{scanner}, "", LangEnglish)

	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)
	e.SetSubscriptionManager(sm)

	sub := &Subscription{
		ID:         "sub1",
		Project:    "test",
		ChatID:     "chat1",
		Platform:   "feishu",
		SessionKey: "feishu:chat1:bot",
		Filter:     "alert",
		Prompt:     "{{content}}",
		Interval:   "*/5 * * * *",
		Enabled:    true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		e.Stop()
	}()

	err = e.ExecuteSubscriptionScan(sub, "")
	if err != nil {
		t.Fatalf("ExecuteSubscriptionScan: %v", err)
	}

	// Verify pagination actually happened (2 calls to ListMessages)
	if scanner.callIdx != 2 {
		t.Errorf("ListMessages called %d times, want 2 (pagination)", scanner.callIdx)
	}

	got := store.Get("sub1")
	if len(got.ProcessedIDs) < 2 {
		t.Errorf("ProcessedIDs = %v, want at least 2", got.ProcessedIDs)
	}
}

func TestExecuteSubscriptionScan_ProcessedIDsDedup(t *testing.T) {
	p := &stubScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		messages: []ScannedMessage{
			{MessageID: "m1", Content: "alert A", IsBot: true, CreatedAt: time.Now()},
			{MessageID: "m2", Content: "alert B", IsBot: true, CreatedAt: time.Now()},
		},
		rcCtx: "reply-ctx",
	}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)
	e.SetSubscriptionManager(sm)

	sub := &Subscription{
		ID:           "sub1",
		Project:      "test",
		ChatID:       "chat1",
		Platform:     "feishu",
		SessionKey:   "feishu:chat1:bot",
		Filter:       "alert",
		Prompt:       "{{content}}",
		Interval:     "*/5 * * * *",
		Enabled:      true,
		ProcessedIDs: []string{"m1"}, // m1 already processed
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := store.Add(sub); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		e.Stop()
	}()

	err = e.ExecuteSubscriptionScan(sub, "")
	if err != nil {
		t.Fatalf("ExecuteSubscriptionScan: %v", err)
	}

	// Only m2 should be newly injected (m1 already in ProcessedIDs)
	got := store.Get("sub1")
	found := false
	for _, id := range got.ProcessedIDs {
		if id == "m2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("m2 should be in ProcessedIDs after scan")
	}
}

func TestExecuteSubscriptionScan_FallbackToReconstructReplyCtx(t *testing.T) {
	p := &stubScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		messages: []ScannedMessage{
			{MessageID: "m1", Content: "alert", IsBot: true, CreatedAt: time.Now()},
		},
		rcCtx:     "reconstructed-ctx",
		threadErr: fmt.Errorf("not supported"),
	}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)

	sub := &Subscription{
		ID:         "sub1",
		Project:    "test",
		ChatID:     "chat1",
		Platform:   "feishu",
		SessionKey: "feishu:chat1:bot",
		Filter:     "alert",
		Prompt:     "{{content}}",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		e.Stop()
	}()
	err := e.ExecuteSubscriptionScan(sub, "")
	if err != nil {
		t.Fatalf("ExecuteSubscriptionScan: %v", err)
	}
	// Should have fallen back to ReconstructReplyCtx
}

func TestIsPermanentError(t *testing.T) {
	tests := []struct {
		errMsg    string
		permanent bool
	}{
		{"permission denied", true},
		{"chat does not exist", true},
		{"invalid token", true},
		{"unauthorized access", true},
		{"forbidden: bot removed", true},
		{"rate limit exceeded", false},
		{"timeout waiting for response", false},
		{"connection reset by peer", false},
	}
	for _, tt := range tests {
		got := isPermanentError(errors.New(tt.errMsg))
		if got != tt.permanent {
			t.Errorf("isPermanentError(%q) = %v, want %v", tt.errMsg, got, tt.permanent)
		}
	}
	// nil error should not be permanent
	if isPermanentError(nil) {
		t.Error("isPermanentError(nil) = true, want false")
	}
}

// paginatingScanner is a test helper that simulates paginated ListMessages.
type paginatingScanner struct {
	*stubScannerPlatform
	pages   [][]ScannedMessage
	tokens  []string
	callIdx int
}

func (s *paginatingScanner) ListMessages(_ context.Context, _ string, _ ListMessagesOptions) ([]ScannedMessage, string, error) {
	if s.callIdx >= len(s.pages) {
		return nil, "", nil
	}
	msgs := s.pages[s.callIdx]
	token := ""
	if s.callIdx < len(s.tokens) {
		token = s.tokens[s.callIdx]
	}
	s.callIdx++
	return msgs, token, nil
}

// ---------------------------------------------------------------------------
// Integration tests — full scan lifecycle
// ---------------------------------------------------------------------------

// integrationScannerPlatform implements Platform + MessageScanner +
// ReplyContextReconstructor + ThreadReplyContextBuilder for integration tests.
// It records all calls for assertion.
type integrationScannerPlatform struct {
	stubPlatformEngine
	mu          sync.Mutex
	messages    []ScannedMessage
	listCalls   []string // chatIDs passed to ListMessages
	rcCtx       any
	threadCtx   any
	threadCalls []string // messageIDs passed to BuildThreadReplyCtx
}

func (p *integrationScannerPlatform) ListMessages(_ context.Context, chatID string, _ ListMessagesOptions) ([]ScannedMessage, string, error) {
	p.mu.Lock()
	p.listCalls = append(p.listCalls, chatID)
	p.mu.Unlock()
	return p.messages, "", nil
}

func (p *integrationScannerPlatform) ReconstructReplyCtx(_ string) (any, error) {
	return p.rcCtx, nil
}

func (p *integrationScannerPlatform) BuildThreadReplyCtx(_ string, _ string, messageID string) (any, string, error) {
	p.mu.Lock()
	p.threadCalls = append(p.threadCalls, messageID)
	p.mu.Unlock()
	return p.threadCtx, "", nil
}

func (p *integrationScannerPlatform) getListCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.listCalls))
	copy(out, p.listCalls)
	return out
}

func (p *integrationScannerPlatform) getThreadCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.threadCalls))
	copy(out, p.threadCalls)
	return out
}

// newIntegrationEnv creates a full test environment for subscription integration tests:
// temp dir, SubscriptionStore, SubscriptionManager, Engine with scanner platform.
func newIntegrationEnv(t *testing.T) (*SubscriptionManager, *Engine, *integrationScannerPlatform) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	p := &integrationScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		messages: []ScannedMessage{
			{MessageID: "m1", ChatID: "chat1", Content: "alert: CPU high", IsBot: true, CreatedAt: time.Now()},
			{MessageID: "m2", ChatID: "chat1", Content: "info: daily report", IsBot: false, CreatedAt: time.Now()},
			{MessageID: "m3", ChatID: "chat1", Content: "alert: memory low", IsBot: true, CreatedAt: time.Now()},
			{MessageID: "m4", ChatID: "chat1", Content: "bot message", IsBot: true, CreatedAt: time.Now()},
		},
		rcCtx:     "reply-ctx-1",
		threadCtx: "thread-ctx-1",
	}

	eng := NewEngine("proj1", &stubAgent{}, []Platform{p}, "", LangEnglish)
	eng.SetSubscriptionManager(sm)
	sm.RegisterEngine("proj1", eng)

	return sm, eng, p
}

// TestIntegration_SubscriptionFullScanCycle exercises the complete lifecycle:
// create → schedule → scan → verify flow → disable → re-enable → cleanup.
func TestIntegration_SubscriptionFullScanCycle(t *testing.T) {
	sm, eng, p := newIntegrationEnv(t)

	// 1. Create subscription with a filter
	sub := &Subscription{
		ID:               "sub-full",
		Project:          "proj1",
		ChatID:           "chat1",
		Platform:         "feishu",
		SessionKey:       "feishu:chat1:bot",
		Filter:           "alert",
		Prompt:           "investigate: {{content}}",
		Interval:         "*/5 * * * *",
		Enabled:          true,
		ConcurrencyLimit: 5,
		TimeoutMins:      1,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}

	// 2. Start the manager and verify cron entry exists
	if err := sm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sm.Stop()

	sm.mu.RLock()
	_, hasEntry := sm.entries["sub-full"]
	sm.mu.RUnlock()
	if !hasEntry {
		t.Error("enabled subscription should have a cron entry after Start")
	}

	// 3. Trigger scan via executeScan (synchronous, avoids goroutine timing)
	//    Cancel the engine after a short delay so processInteractiveMessageWith exits.
	go func() {
		time.Sleep(200 * time.Millisecond)
		eng.Stop()
	}()

	sm.executeScan("sub-full")

	// 4. Verify scan flow
	//    a. ListMessages was called with correct chatID
	listCalls := p.getListCalls()
	if len(listCalls) == 0 {
		t.Error("ListMessages should have been called")
	} else if listCalls[0] != "chat1" {
		t.Errorf("ListMessages called with chatID %q, want %q", listCalls[0], "chat1")
	}

	//    b. filterMessages was applied — only bot messages matching "alert" pass (m1, m3); m2 is human, m4 doesn't match filter
	threadCalls := p.getThreadCalls()
	if len(threadCalls) != 2 {
		t.Errorf("BuildThreadReplyCtx called %d times, want 2 (m1 and m3 match 'alert')", len(threadCalls))
	}

	//    c. Anchor was updated
	got := sm.Store().Get("sub-full")
	if got == nil {
		t.Fatal("subscription should exist after scan")
	}
	if got.Anchor == "" {
		t.Error("anchor should be updated after scan with messages")
	}

	//    d. ProcessedIDs contains matched message IDs
	hasM1 := false
	hasM3 := false
	for _, id := range got.ProcessedIDs {
		if id == "m1" {
			hasM1 = true
		}
		if id == "m3" {
			hasM3 = true
		}
	}
	if !hasM1 || !hasM3 {
		t.Errorf("ProcessedIDs should contain m1 and m3, got %v", got.ProcessedIDs)
	}

	//    e. LastRun was set
	if got.LastRun.IsZero() {
		t.Error("LastRun should be set after scan")
	}

	//    f. ConsecutiveErrors reset to 0 on success
	if got.ConsecutiveErrors != 0 {
		t.Errorf("ConsecutiveErrors = %d, want 0 after successful scan", got.ConsecutiveErrors)
	}

	// 5. Disable and verify cron entry removed
	if err := sm.DisableSubscription("sub-full"); err != nil {
		t.Fatalf("DisableSubscription: %v", err)
	}
	sm.mu.RLock()
	_, hasEntry = sm.entries["sub-full"]
	sm.mu.RUnlock()
	if hasEntry {
		t.Error("disabled subscription should not have a cron entry")
	}
	got = sm.Store().Get("sub-full")
	if got.Enabled {
		t.Error("subscription should be disabled in store")
	}

	// 6. Re-enable and verify cron entry re-added
	if err := sm.EnableSubscription("sub-full"); err != nil {
		t.Fatalf("EnableSubscription: %v", err)
	}
	sm.mu.RLock()
	_, hasEntry = sm.entries["sub-full"]
	sm.mu.RUnlock()
	if !hasEntry {
		t.Error("re-enabled subscription should have a cron entry")
	}
	got = sm.Store().Get("sub-full")
	if !got.Enabled {
		t.Error("subscription should be enabled in store after re-enable")
	}

	// 7. Cleanup — Stop is deferred, temp dir auto-cleaned by testing
}

// TestIntegration_SubscriptionAutoDisable verifies that 10 consecutive
// permanent errors triggers auto-disable and cron entry removal.
func TestIntegration_SubscriptionAutoDisable(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	// Engine not registered for the project — executeScan will hit engine-not-found
	// which is classified as a permanent error, incrementing ConsecutiveErrors.
	sub := &Subscription{
		ID:                "sub-autodis",
		Project:           "proj1",
		ChatID:            "chat1",
		Platform:          "feishu",
		SessionKey:        "feishu:chat1:bot",
		Filter:            "alert",
		Prompt:            "test",
		Interval:          "*/5 * * * *",
		Enabled:           true,
		ConsecutiveErrors: 9, // one more error should trigger auto-disable
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}

	if err := sm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sm.Stop()

	// Verify cron entry exists before auto-disable
	sm.mu.RLock()
	_, hasEntry := sm.entries["sub-autodis"]
	sm.mu.RUnlock()
	if !hasEntry {
		t.Fatal("subscription should have a cron entry before auto-disable")
	}

	// Trigger scan — engine not found → permanent error → ConsecutiveErrors=10 → auto-disable
	sm.executeScan("sub-autodis")

	// Verify auto-disable happened
	got := store.Get("sub-autodis")
	if got.Enabled {
		t.Error("subscription should be auto-disabled after 10th consecutive error")
	}
	if got.ConsecutiveErrors != 10 {
		t.Errorf("ConsecutiveErrors = %d, want 10", got.ConsecutiveErrors)
	}

	// Verify cron entry removed
	sm.mu.RLock()
	_, hasEntry = sm.entries["sub-autodis"]
	sm.mu.RUnlock()
	if hasEntry {
		t.Error("auto-disabled subscription should have cron entry removed")
	}

	// Continue scanning should be a no-op (disabled)
	sm.executeScan("sub-autodis")
	got = store.Get("sub-autodis")
	if !got.LastRun.IsZero() && got.ConsecutiveErrors > 10 {
		t.Error("disabled subscription should not accumulate more errors from executeScan")
	}
}

// TestIntegration_SubscriptionConcurrencyGuard verifies that concurrent
// scans on the same subscription are skipped.
func TestIntegration_SubscriptionConcurrencyGuard(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSubscriptionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSubscriptionManager(store, dir, nil)

	// Use a scanner platform that introduces delay so the scan takes time
	p := &integrationScannerPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		messages: []ScannedMessage{
			{MessageID: "m1", ChatID: "chat1", Content: "alert", IsBot: true, CreatedAt: time.Now()},
		},
		rcCtx:     "reply-ctx",
		threadCtx: "thread-ctx",
	}

	eng := NewEngine("proj1", &stubAgent{}, []Platform{p}, "", LangEnglish)
	eng.SetSubscriptionManager(sm)
	sm.RegisterEngine("proj1", eng)

	sub := &Subscription{
		ID:          "sub-conc",
		Project:     "proj1",
		ChatID:      "chat1",
		Platform:    "feishu",
		SessionKey:  "feishu:chat1:bot",
		Filter:      "alert",
		Prompt:      "test",
		Interval:    "*/5 * * * *",
		Enabled:     true,
		TimeoutMins: 1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := sm.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}

	// Simulate an in-progress scan by setting the running flag
	sm.running.Store("sub-conc", true)

	// Try to execute a concurrent scan — should be skipped
	sm.executeScan("sub-conc")

	// Verify that ListMessages was NOT called (scan was skipped)
	listCalls := p.getListCalls()
	if len(listCalls) != 0 {
		t.Errorf("ListMessages should not be called when scan is skipped, got %d calls", len(listCalls))
	}

	// Verify the running flag is still set (we set it manually, not by executeScan)
	if _, ok := sm.running.Load("sub-conc"); !ok {
		t.Error("running flag should still be set since scan was skipped")
	}

	// Clean up
	sm.running.Delete("sub-conc")
}
