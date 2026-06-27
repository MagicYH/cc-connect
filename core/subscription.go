package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Subscription represents a recurring scan of a messaging thread for new items.
type Subscription struct {
	ID                string    `json:"id"`
	Project           string    `json:"project"`
	ChatID            string    `json:"chat_id"`
	ChatName          string    `json:"chat_name,omitempty"`
	Platform          string    `json:"platform"`
	SessionKey        string    `json:"session_key"`
	Filter            string    `json:"filter,omitempty"`
	ExcludeFilter     string    `json:"exclude_filter,omitempty"`
	Prompt            string    `json:"prompt"`
	Anchor            string    `json:"anchor,omitempty"`
	Interval          string    `json:"interval"`
	ConcurrencyLimit  int       `json:"concurrency_limit"`
	TimeoutMins       int       `json:"timeout_mins"`
	Enabled           bool      `json:"enabled"`
	LastRun           time.Time `json:"last_run,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	ConsecutiveErrors int       `json:"consecutive_errors,omitempty"`
	ProcessedIDs      []string  `json:"processed_ids,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

var ErrSubscriptionNotFound = errors.New("subscription not found")

var ErrSubscriptionDuplicate = errors.New("subscription already exists for this project and chat")

// GenerateSubscriptionID creates a 16-hex-char unique ID (8 random bytes → hex).
func GenerateSubscriptionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("generate subscription id: %w", err))
	}
	return hex.EncodeToString(b)
}

// SubscriptionStore persists subscriptions to a JSON file.
type SubscriptionStore struct {
	path string
	mu   sync.Mutex
	subs []*Subscription
}

// NewSubscriptionStore creates the data directory, loads existing subscriptions, and returns the store.
func NewSubscriptionStore(dataDir string) (*SubscriptionStore, error) {
	dir := filepath.Join(dataDir, "subscriptions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("subscription: create data dir: %w", err)
	}
	path := filepath.Join(dir, "jobs.json")
	s := &SubscriptionStore{path: path}
	s.load()
	return s, nil
}

func (s *SubscriptionStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &s.subs); err != nil {
		slog.Error("subscription: failed to load jobs", "path", s.path, "error", err)
	}
}

func (s *SubscriptionStore) save() error {
	data, err := json.MarshalIndent(s.subs, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(s.path, data, 0o644)
}

// Add inserts a new subscription. Returns ErrSubscriptionDuplicate if a
// subscription with the same (Project, ChatID) pair already exists.
func (s *SubscriptionStore) Add(sub *Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.subs {
		if existing.Project == sub.Project && existing.ChatID == sub.ChatID {
			return ErrSubscriptionDuplicate
		}
	}
	s.subs = append(s.subs, sub)
	return s.save()
}

// Get returns the subscription with the given ID, or nil.
func (s *SubscriptionStore) Get(id string) *Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		if sub.ID == id {
			return sub
		}
	}
	return nil
}

// Remove deletes the subscription with the given ID.
// Returns ErrSubscriptionNotFound if the ID does not exist.
func (s *SubscriptionStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subs {
		if sub.ID == id {
			s.subs = append(s.subs[:i], s.subs[i+1:]...)
			if err := s.save(); err != nil {
				slog.Warn("subscription: failed to save after remove", "error", err)
			}
			return nil
		}
	}
	return ErrSubscriptionNotFound
}

// ListByProject returns all subscriptions for the given project.
func (s *SubscriptionStore) ListByProject(project string) []*Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Subscription
	for _, sub := range s.subs {
		if sub.Project == project {
			out = append(out, sub)
		}
	}
	return out
}

// ListByChatID returns all subscriptions for the given chat ID.
func (s *SubscriptionStore) ListByChatID(chatID string) []*Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Subscription
	for _, sub := range s.subs {
		if sub.ChatID == chatID {
			out = append(out, sub)
		}
	}
	return out
}

// ListAll returns a copy of all subscriptions.
func (s *SubscriptionStore) ListAll() []*Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Subscription, len(s.subs))
	copy(out, s.subs)
	return out
}

// Update modifies the specified fields on a subscription and sets UpdatedAt.
// Returns ErrSubscriptionNotFound if the ID does not exist.
// Read-only fields (id, created_at, last_run, last_error, consecutive_errors,
// processed_ids, anchor) cannot be modified and will cause an error.
func (s *SubscriptionStore) Update(id string, fields map[string]any) error {
	readOnlyFields := map[string]bool{
		"id": true, "created_at": true, "last_run": true, "last_error": true,
		"consecutive_errors": true, "processed_ids": true, "anchor": true,
	}
	for field := range fields {
		if readOnlyFields[field] {
			return fmt.Errorf("field %q is read-only", field)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		if sub.ID == id {
			newProject := sub.Project
			newChatID := sub.ChatID
			if v, ok := fields["project"]; ok {
				if val, ok := v.(string); ok {
					newProject = val
				}
			}
			if v, ok := fields["chat_id"]; ok {
				if val, ok := v.(string); ok {
					newChatID = val
				}
			}
			if newProject != sub.Project || newChatID != sub.ChatID {
				for _, existing := range s.subs {
					if existing.ID != id && existing.Project == newProject && existing.ChatID == newChatID {
						return ErrSubscriptionDuplicate
					}
				}
			}
			for field, value := range fields {
				if err := updateSubscriptionField(sub, field, value); err != nil {
					return fmt.Errorf("update field %q: %w", field, err)
				}
			}
			sub.UpdatedAt = time.Now()
			if err := s.save(); err != nil {
				slog.Warn("subscription: failed to save after update", "error", err)
			}
			return nil
		}
	}
	return ErrSubscriptionNotFound
}

// updateSubscriptionField sets a single field on a Subscription by name.
func updateSubscriptionField(sub *Subscription, field string, value any) error {
	switch field {
	case "project":
		if v, ok := value.(string); ok {
			sub.Project = v
			return nil
		}
	case "chat_id":
		if v, ok := value.(string); ok {
			sub.ChatID = v
			return nil
		}
	case "chat_name":
		if v, ok := value.(string); ok {
			sub.ChatName = v
			return nil
		}
	case "platform":
		if v, ok := value.(string); ok {
			sub.Platform = v
			return nil
		}
	case "session_key":
		if v, ok := value.(string); ok {
			sub.SessionKey = v
			return nil
		}
	case "filter":
		if v, ok := value.(string); ok {
			sub.Filter = v
			return nil
		}
	case "exclude_filter":
		if v, ok := value.(string); ok {
			sub.ExcludeFilter = v
			return nil
		}
	case "prompt":
		if v, ok := value.(string); ok {
			sub.Prompt = v
			return nil
		}
	case "interval":
		if v, ok := value.(string); ok {
			sub.Interval = v
			return nil
		}
	case "concurrency_limit":
		if v, ok := value.(float64); ok {
			sub.ConcurrencyLimit = int(v)
			return nil
		}
		if v, ok := value.(int); ok {
			sub.ConcurrencyLimit = v
			return nil
		}
	case "timeout_mins":
		if v, ok := value.(float64); ok {
			sub.TimeoutMins = int(v)
			return nil
		}
		if v, ok := value.(int); ok {
			sub.TimeoutMins = v
			return nil
		}
	case "enabled":
		if v, ok := value.(bool); ok {
			sub.Enabled = v
			return nil
		}
	}
	return fmt.Errorf("unknown or invalid field: %s", field)
}

// UpdateAnchor atomically updates the anchor and processed IDs for a subscription.
func (s *SubscriptionStore) UpdateAnchor(id string, anchor string, processedIDs []string) error {
	// Prune to max 100 entries
	const maxProcessedIDs = 100
	if len(processedIDs) > maxProcessedIDs {
		processedIDs = processedIDs[len(processedIDs)-maxProcessedIDs:]
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		if sub.ID == id {
			sub.Anchor = anchor
			sub.ProcessedIDs = append([]string(nil), processedIDs...)
			sub.UpdatedAt = time.Now()
			if err := s.save(); err != nil {
				slog.Warn("subscription: failed to save after update anchor", "error", err)
			}
			return nil
		}
	}
	return ErrSubscriptionNotFound
}

// MarkRun updates the run status of a subscription.
//   - On success (lastErr == ""): resets ConsecutiveErrors to 0 and clears LastError.
//   - On transient error (isPermanent == false): records LastError but does not increment ConsecutiveErrors.
//   - On permanent error (isPermanent == true): increments ConsecutiveErrors and records LastError.
//     Auto-disables the subscription when ConsecutiveErrors reaches 10.
func (s *SubscriptionStore) MarkRun(id string, lastErr string, isPermanent bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		if sub.ID == id {
			sub.LastRun = time.Now()
			if lastErr == "" {
				sub.LastError = ""
				sub.ConsecutiveErrors = 0
			} else {
				sub.LastError = lastErr
				if isPermanent {
					sub.ConsecutiveErrors++
					if sub.ConsecutiveErrors >= 10 {
						sub.Enabled = false
					}
				}
			}
			if err := s.save(); err != nil {
				slog.Warn("subscription: failed to save after mark run", "error", err)
			}
			return nil
		}
	}
	return ErrSubscriptionNotFound
}

// filterMessages applies Filter and ExcludeFilter to scanned messages, excluding
// bot messages and already-processed message IDs.
func filterMessages(msgs []ScannedMessage, filter string, excludeFilter string, processedIDs []string) []ScannedMessage {
	processedSet := make(map[string]struct{}, len(processedIDs))
	for _, id := range processedIDs {
		processedSet[id] = struct{}{}
	}

	var matched []ScannedMessage
	for _, msg := range msgs {
		if msg.IsBot {
			continue
		}
		if _, seen := processedSet[msg.MessageID]; seen {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(msg.Content), strings.ToLower(filter)) {
			continue
		}
		if excludeFilter != "" && strings.Contains(strings.ToLower(msg.Content), strings.ToLower(excludeFilter)) {
			continue
		}
		matched = append(matched, msg)
	}
	return matched
}

// BuildPrompt replaces {{content}} in the subscription's prompt template.
func (s *Subscription) BuildPrompt(content string) string {
	return strings.ReplaceAll(s.Prompt, "{{content}}", content)
}

// LogEntry records a subscription processing event.
type LogEntry struct {
	SubscriptionID string    `json:"subscription_id"`
	MessageID      string    `json:"message_id"`
	ChatID         string    `json:"chat_id"`
	Content        string    `json:"content,omitempty"`
	SessionID      string    `json:"session_id,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// SubscriptionManager manages subscription scheduling and execution.
type SubscriptionManager struct {
	store   *SubscriptionStore
	cron    *cron.Cron
	engines map[string]*Engine
	mu      sync.RWMutex
	entries map[string]cron.EntryID
	running sync.Map // map[string]bool — subscription ID → is running
	logsDir string
}

// NewSubscriptionManager creates a new SubscriptionManager with the given store and data directory.
func NewSubscriptionManager(store *SubscriptionStore, dataDir string) *SubscriptionManager {
	return &SubscriptionManager{
		store:   store,
		cron:    cron.New(),
		engines: make(map[string]*Engine),
		entries: make(map[string]cron.EntryID),
		logsDir: filepath.Join(dataDir, "subscriptions", "logs"),
	}
}

// RegisterEngine registers an engine for the given project name.
func (sm *SubscriptionManager) RegisterEngine(name string, e *Engine) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.engines[name] = e
}

// Start starts the cron scheduler and schedules all enabled subscriptions.
func (sm *SubscriptionManager) Start() error {
	for _, sub := range sm.store.ListAll() {
		if sub.Enabled {
			if err := sm.scheduleSubscription(sub); err != nil {
				slog.Warn("subscription: failed to schedule", "id", sub.ID, "error", err)
			}
		}
	}
	sm.cron.Start()
	slog.Info("subscription: manager started", "subscriptions", len(sm.store.ListAll()))
	return nil
}

// Stop stops the cron scheduler.
func (sm *SubscriptionManager) Stop() {
	sm.cron.Stop()
}

func (sm *SubscriptionManager) scheduleSubscription(sub *Subscription) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Remove existing schedule if any
	if old, ok := sm.entries[sub.ID]; ok {
		sm.cron.Remove(old)
	}

	entryID, err := sm.cron.AddFunc(sub.Interval, func() {
		sm.executeScan(sub.ID)
	})
	if err != nil {
		return err
	}
	sm.entries[sub.ID] = entryID
	return nil
}

// AddSubscription validates the interval, adds a new subscription, and schedules it if enabled.
func (sm *SubscriptionManager) AddSubscription(sub *Subscription) error {
	if _, err := cron.ParseStandard(sub.Interval); err != nil {
		return fmt.Errorf("invalid interval %q: %w", sub.Interval, err)
	}
	if err := sm.store.Add(sub); err != nil {
		return err
	}
	if sub.Enabled {
		return sm.scheduleSubscription(sub)
	}
	return nil
}

// RemoveSubscription removes a subscription and unschedules it.
func (sm *SubscriptionManager) RemoveSubscription(id string) error {
	sm.mu.Lock()
	if entryID, ok := sm.entries[id]; ok {
		sm.cron.Remove(entryID)
		delete(sm.entries, id)
	}
	sm.mu.Unlock()
	return sm.store.Remove(id)
}

// EnableSubscription enables a subscription and schedules it.
func (sm *SubscriptionManager) EnableSubscription(id string) error {
	if err := sm.store.Update(id, map[string]any{"enabled": true}); err != nil {
		return err
	}
	sub := sm.store.Get(id)
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	return sm.scheduleSubscription(sub)
}

// DisableSubscription disables a subscription and unschedules it.
func (sm *SubscriptionManager) DisableSubscription(id string) error {
	if err := sm.store.Update(id, map[string]any{"enabled": false}); err != nil {
		return err
	}
	sm.mu.Lock()
	if entryID, ok := sm.entries[id]; ok {
		sm.cron.Remove(entryID)
		delete(sm.entries, id)
	}
	sm.mu.Unlock()
	return nil
}

// Store returns the underlying SubscriptionStore.
func (sm *SubscriptionManager) Store() *SubscriptionStore {
	return sm.store
}

func (sm *SubscriptionManager) executeScan(subID string) {
	sub := sm.store.Get(subID)
	if sub == nil {
		return
	}
	if !sub.Enabled {
		return
	}

	// Skip if already running
	if _, loaded := sm.running.LoadOrStore(subID, true); loaded {
		slog.Warn("subscription: skipping scan, previous scan still running", "id", subID)
		return
	}
	defer sm.running.Delete(subID)

	sm.mu.RLock()
	engine, ok := sm.engines[sub.Project]
	sm.mu.RUnlock()
	if !ok {
		slog.Error("subscription: engine not found", "subscription_id", subID, "project", sub.Project)
		sm.store.MarkRun(subID, fmt.Sprintf("project %q not found", sub.Project), true)
		sm.unscheduleIfDisabled(subID)
		return
	}

	slog.Info("subscription: executing scan", "id", subID, "project", sub.Project)

	// Snapshot the subscription to avoid data race
	snapshot := *sub
	snapshot.ProcessedIDs = append([]string(nil), sub.ProcessedIDs...)

	done := make(chan error, 1)
	go func() {
		done <- engine.ExecuteSubscriptionScan(&snapshot)
	}()

	var err error
	if sub.TimeoutMins > 0 {
		timeout := time.Duration(sub.TimeoutMins) * time.Minute
		select {
		case err = <-done:
		case <-time.After(timeout):
			err = fmt.Errorf("scan timed out after %v", timeout)
		}
	} else {
		err = <-done
	}

	if err != nil {
		sm.store.MarkRun(subID, err.Error(), true)
		slog.Error("subscription: scan failed", "id", subID, "error", err)
	} else {
		sm.store.MarkRun(subID, "", false)
		slog.Info("subscription: scan completed", "id", subID)
	}

	sm.unscheduleIfDisabled(subID)
}

// unscheduleIfDisabled removes the cron entry for a subscription that was
// auto-disabled by MarkRun (ConsecutiveErrors >= 10).
func (sm *SubscriptionManager) unscheduleIfDisabled(subID string) {
	if updated := sm.store.Get(subID); updated != nil && !updated.Enabled {
		sm.mu.Lock()
		if entryID, ok := sm.entries[subID]; ok {
			sm.cron.Remove(entryID)
			delete(sm.entries, subID)
		}
		sm.mu.Unlock()
		slog.Warn("subscription: auto-disabled after consecutive errors", "id", subID)
	}
}

// AppendLog appends a log entry to the subscription's log file.
func (sm *SubscriptionManager) AppendLog(entry LogEntry) error {
	if err := os.MkdirAll(sm.logsDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(sm.logsDir, entry.SubscriptionID+".log")
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("subscription: marshal log entry: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, string(data))
	return err
}
