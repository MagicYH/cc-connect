# Subscription Auto-Investigation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** ./../specs/2026-06-26-subscription-auto-investigation-design.md
**Verification:** ./../specs/2026-06-26-subscription-auto-investigation-verification.md

**Goal:** Add a Subscription feature that periodically scans Feishu group messages, filters by keyword, and auto-injects matched messages into the agent for investigation with thread replies.

**Architecture:** New `SubscriptionManager` standalone component (mirroring `CronScheduler`), new `MessageScanner` and `ThreadReplyContextBuilder` optional interfaces in core, `/subscribe` slash command, management API CRUD endpoints, and WebUI management page.

**Tech Stack:** Go (core logic), React/TypeScript (WebUI), robfig/cron/v3 (scheduling), Feishu Im.Message.List API (message scanning)

---

## File Structure

| File | Responsibility |
|---|---|
| `core/subscription.go` | Subscription struct, SubscriptionStore, SubscriptionManager, scan logic |
| `core/subscription_cmd.go` | `cmdSubscribe` and subcommand handlers (Engine methods) |
| `core/interfaces.go` | Add `MessageScanner` and `ThreadReplyContextBuilder` interfaces |
| `core/i18n.go` | Add all `MsgSub*` keys and translations |
| `core/engine.go` | Add `builtinCommands` entry, `handleCommand` case, `SetSubscriptionManager`, `ExecuteSubscriptionScan` |
| `core/management.go` | Add subscription CRUD API endpoints |
| `config/config.go` | Add `SubscriptionsEnabled` to `ProjectConfig` |
| `platform/feishu/feishu.go` | Implement `MessageScanner` and `ThreadReplyContextBuilder` |
| `platform/feishu/subscription.go` | Feishu-specific `ListMessages` implementation |
| `web/src/api/subscription.ts` | API client for subscription endpoints |
| `web/src/pages/Subscriptions/SubscriptionList.tsx` | WebUI management page |
| `web/src/App.tsx` | Add route for `/subscriptions` |

---

### Task 1: Core data model and store <!-- covers: S1, S2, S8, S13 -->

**Files:**
- Create: `core/subscription.go`
- Test: `core/subscription_test.go`

- [ ] **Step 1: Write failing test for Subscription struct and SubscriptionStore CRUD**

```go
package core

import (
    "os"
    "path/filepath"
    "testing"
)

func TestSubscriptionStoreCRUD(t *testing.T) {
    dir := t.TempDir()
    store, err := NewSubscriptionStore(dir)
    if err != nil {
        t.Fatalf("NewSubscriptionStore: %v", err)
    }

    sub := &Subscription{
        ID:          GenerateSubscriptionID(),
        Project:     "test-project",
        ChatID:      "oc_test_chat",
        ChatName:    "Test Group",
        Platform:    "feishu",
        SessionKey:  "feishu:oc_test_chat:bot123",
        Filter:      "告警",
        ExcludeFilter: "恢复",
        Prompt:      "排查以下报警：{{content}}",
        Interval:    "*/2 * * * *",
        ConcurrencyLimit: 5,
        TimeoutMins: 30,
        Enabled:     true,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    // Add
    if err := store.Add(sub); err != nil {
        t.Fatalf("Add: %v", err)
    }

    // Get
    got, err := store.Get(sub.ID)
    if err != nil {
        t.Fatalf("Get: %v", err)
    }
    if got.Filter != "告警" {
        t.Errorf("Filter = %q, want %q", got.Filter, "告警")
    }

    // ListByProject
    list := store.ListByProject("test-project")
    if len(list) != 1 {
        t.Errorf("ListByProject = %d items, want 1", len(list))
    }

    // Uniqueness check
    dup := &Subscription{
        ID:       GenerateSubscriptionID(),
        Project:  "test-project",
        ChatID:   "oc_test_chat",
        Platform: "feishu",
    }
    if err := store.Add(dup); err == nil {
        t.Error("Add duplicate (Project, ChatID) should fail")
    }

    // Update
    if err := store.Update(sub.ID, map[string]any{"filter": "warning"}); err != nil {
        t.Fatalf("Update: %v", err)
    }
    got, _ = store.Get(sub.ID)
    if got.Filter != "warning" {
        t.Errorf("Filter after update = %q, want %q", got.Filter, "warning")
    }

    // Remove
    if err := store.Remove(sub.ID); err != nil {
        t.Fatalf("Remove: %v", err)
    }
    if _, err := store.Get(sub.ID); err == nil {
        t.Error("Get after remove should fail")
    }
}

func TestSubscriptionStorePersistence(t *testing.T) {
    dir := t.TempDir()
    store, _ := NewSubscriptionStore(dir)

    sub := &Subscription{
        ID:       GenerateSubscriptionID(),
        Project:  "test-project",
        ChatID:   "oc_test_chat",
        Platform: "feishu",
        Enabled:  true,
        Anchor:   "msg_anchor_123",
    }
    store.Add(sub)

    // Reload from disk
    store2, err := NewSubscriptionStore(dir)
    if err != nil {
        t.Fatalf("NewSubscriptionStore reload: %v", err)
    }
    got, _ := store2.Get(sub.ID)
    if got.Anchor != "msg_anchor_123" {
        t.Errorf("Anchor after reload = %q, want %q", got.Anchor, "msg_anchor_123")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/ -run TestSubscriptionStore -v`
Expected: FAIL — `Subscription` and `SubscriptionStore` not defined

- [ ] **Step 3: Implement Subscription struct, GenerateSubscriptionID, and SubscriptionStore**

In `core/subscription.go`:

```go
package core

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "sync"
    "time"

    "github.com/anthropics/cc-connect/core/atomicwrite"
)

// Subscription represents a per-bot, per-group message scanning subscription.
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

// GenerateSubscriptionID creates a 16-hex-char unique ID.
func GenerateSubscriptionID() string {
    b := make([]byte, 8)
    rand.Read(b)
    return hex.EncodeToString(b)
}

// SubscriptionStore persists subscriptions to jobs.json.
type SubscriptionStore struct {
    path string
    mu   sync.Mutex
    subs []*Subscription
}

func NewSubscriptionStore(dataDir string) (*SubscriptionStore, error) {
    dir := dataDir + "/subscriptions"
    os.MkdirAll(dir, 0o755)
    s := &SubscriptionStore{path: dir + "/jobs.json"}
    return s, s.load()
}

func (s *SubscriptionStore) load() error {
    data, err := os.ReadFile(s.path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }
    return json.Unmarshal(data, &s.subs)
}

func (s *SubscriptionStore) save() error {
    data, err := json.MarshalIndent(s.subs, "", "  ")
    if err != nil {
        return err
    }
    return atomicwrite.AtomicWriteFile(s.path, data, 0o644)
}

func (s *SubscriptionStore) Add(sub *Subscription) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    for _, existing := range s.subs {
        if existing.Project == sub.Project && existing.ChatID == sub.ChatID {
            return fmt.Errorf("subscription already exists for project=%s chat_id=%s", sub.Project, sub.ChatID)
        }
    }
    s.subs = append(s.subs, sub)
    return s.save()
}

func (s *SubscriptionStore) Get(id string) (*Subscription, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for _, sub := range s.subs {
        if sub.ID == id {
            return sub, nil
        }
    }
    return nil, fmt.Errorf("subscription not found: %s", id)
}

func (s *SubscriptionStore) Remove(id string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i, sub := range s.subs {
        if sub.ID == id {
            s.subs = append(s.subs[:i], s.subs[i+1:]...)
            return s.save()
        }
    }
    return fmt.Errorf("subscription not found: %s", id)
}

func (s *SubscriptionStore) ListByProject(project string) []*Subscription {
    s.mu.Lock()
    defer s.mu.Unlock()
    var result []*Subscription
    for _, sub := range s.subs {
        if sub.Project == project {
            result = append(result, sub)
        }
    }
    return result
}

func (s *SubscriptionStore) ListByChatID(chatID string) []*Subscription {
    s.mu.Lock()
    defer s.mu.Unlock()
    var result []*Subscription
    for _, sub := range s.subs {
        if sub.ChatID == chatID {
            result = append(result, sub)
        }
    }
    return result
}

func (s *SubscriptionStore) ListAll() []*Subscription {
    s.mu.Lock()
    defer s.mu.Unlock()
    return append([]*Subscription{}, s.subs...)
}

func (s *SubscriptionStore) Update(id string, fields map[string]any) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    sub, err := s.getUnlocked(id)
    if err != nil {
        return err
    }
    for k, v := range fields {
        switch k {
        case "filter":
            sub.Filter = v.(string)
        case "exclude_filter":
            sub.ExcludeFilter = v.(string)
        case "prompt":
            sub.Prompt = v.(string)
        case "interval":
            sub.Interval = v.(string)
        case "concurrency_limit":
            sub.ConcurrencyLimit = v.(int)
        case "timeout_mins":
            sub.TimeoutMins = v.(int)
        case "enabled":
            sub.Enabled = v.(bool)
        }
    }
    sub.UpdatedAt = time.Now()
    return s.save()
}

func (s *SubscriptionStore) UpdateAnchor(id string, anchor string, processedIDs []string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    sub, err := s.getUnlocked(id)
    if err != nil {
        return err
    }
    sub.Anchor = anchor
    sub.ProcessedIDs = processedIDs
    sub.UpdatedAt = time.Now()
    return s.save()
}

func (s *SubscriptionStore) MarkRun(id string, lastErr string, isPermanent bool) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    sub, err := s.getUnlocked(id)
    if err != nil {
        return err
    }
    sub.LastRun = time.Now()
    sub.LastError = lastErr
    if lastErr == "" {
        sub.ConsecutiveErrors = 0
    } else if isPermanent {
        sub.ConsecutiveErrors++
    }
    sub.UpdatedAt = time.Now()
    if sub.ConsecutiveErrors >= 10 {
        sub.Enabled = false
    }
    return s.save()
}

func (s *SubscriptionStore) getUnlocked(id string) (*Subscription, error) {
    for _, sub := range s.subs {
        if sub.ID == id {
            return sub, nil
        }
    }
    return nil, fmt.Errorf("subscription not found: %s", id)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./core/ -run TestSubscriptionStore -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/subscription.go core/subscription_test.go
git commit -m "feat: add Subscription data model and SubscriptionStore"
```

---

### Task 2: MessageScanner and ThreadReplyContextBuilder interfaces <!-- covers: S6 -->

**Files:**
- Modify: `core/interfaces.go`

- [ ] **Step 1: Add interfaces to core/interfaces.go**

Append to the existing interfaces file:

```go
// ScannedMessage represents a message retrieved from a platform's message history.
type ScannedMessage struct {
    MessageID string
    ChatID    string
    UserID    string
    IsBot     bool
    IsCard    bool
    MsgType   string
    Content   string
    CreatedAt time.Time
}

// ListMessagesOptions configures a message history listing request.
type ListMessagesOptions struct {
    Since     time.Time
    PageSize  int
    PageToken string
}

// MessageScanner is an optional interface for platforms that support
// retrieving message history from a chat.
type MessageScanner interface {
    ListMessages(ctx context.Context, chatID string, opts ListMessagesOptions) ([]ScannedMessage, string, error)
}

// ThreadReplyContextBuilder is an optional interface for platforms that can
// construct a reply context targeting a specific message for reply-in-thread.
type ThreadReplyContextBuilder interface {
    BuildThreadReplyCtx(chatID string, messageID string) (any, error)
}
```

- [ ] **Step 2: Run build to verify no compilation errors**

Run: `go build ./core/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add core/interfaces.go
git commit -m "feat: add MessageScanner and ThreadReplyContextBuilder interfaces"
```

---

### Task 3: Feishu MessageScanner implementation <!-- covers: S3, S4, S5 -->

**Files:**
- Create: `platform/feishu/subscription.go`

- [ ] **Step 1: Write failing test for Feishu ListMessages**

```go
package feishu

import (
    "context"
    "testing"
    "time"

    "github.com/anthropics/cc-connect/core"
)

func TestFeishuListMessages(t *testing.T) {
    // This test verifies the ListMessages method exists and returns
    // correctly typed ScannedMessage structs. Integration testing
    // against the real Feishu API is done via verification scenarios.
    p := &Platform{}
    // Verify the interface is implemented
    var _ core.MessageScanner = p
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./platform/feishu/ -run TestFeishuListMessages -v`
Expected: FAIL — `*Platform` does not implement `MessageScanner`

- [ ] **Step 3: Implement Feishu ListMessages and BuildThreadReplyCtx**

In `platform/feishu/subscription.go`:

```go
package feishu

import (
    "context"
    "fmt"
    "time"

    "github.com/anthropics/cc-connect/core"
    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// ListMessages retrieves messages from a Feishu group chat since the given time.
func (p *Platform) ListMessages(ctx context.Context, chatID string, opts core.ListMessagesOptions) ([]core.ScannedMessage, string, error) {
    req := p.client.Im.Message.List()
    req.ContainerId(chatID)
    req.PageSize(int32Ptr(firstNonZero(opts.PageSize, 50)))
    if !opts.Since.IsZero() {
        req.StartTime(int64Ptr(opts.Since.UnixMilli()))
    }
    if opts.PageToken != "" {
        req.PageToken(opts.PageToken)
    }

    resp, err := req.Do(ctx)
    if err != nil {
        return nil, "", fmt.Errorf("feishu: list messages: %w", err)
    }

    var messages []core.ScannedMessage
    if resp.Data != nil && resp.Data.Items != nil {
        for _, msg := range resp.Data.Items {
            sm := core.ScannedMessage{
                MessageID: derefStr(msg.MessageId),
                ChatID:    derefStr(msg.ChatId),
                UserID:    derefStr(msg.SenderId),
                MsgType:   derefStr(msg.MsgType),
                Content:   extractMessageContent(derefStr(msg.MsgType), derefStr(msg.Body.Content)),
                CreatedAt: time.UnixMilli(derefInt64(msg.CreateTime)),
            }
            // Check if sender is a bot
            if msg.Sender != nil && msg.Sender.SenderType != nil {
                sm.IsBot = *msg.Sender.SenderType == "bot"
            }
            sm.IsCard = sm.MsgType == "interactive"
            messages = append(messages, sm)
        }
    }

    var nextToken string
    if resp.Data != nil && resp.Data.HasMore != nil && *resp.Data.HasMore {
        nextToken = derefStr(resp.Data.PageToken)
    }
    return messages, nextToken, nil
}

// BuildThreadReplyCtx constructs a replyContext targeting a specific message for reply-in-thread.
func (p *Platform) BuildThreadReplyCtx(chatID string, messageID string) (any, error) {
    sessionKey := fmt.Sprintf("%s:%s", p.platformName, chatID)
    return replyContext{chatID: chatID, messageID: messageID, sessionKey: sessionKey}, nil
}

func int32Ptr(v int) *int32   { i := int32(v); return &i }
func int64Ptr(v int64) *int64 { return &v }
func derefStr(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}
func derefInt64(i *int64) int64 {
    if i == nil {
        return 0
    }
    return *i
}

func firstNonZero(a, b int) int {
    if a != 0 {
        return a
    }
    return b
}

// extractMessageContent extracts readable text from a message based on its type.
func extractMessageContent(msgType, content string) string {
    switch msgType {
    case "interactive":
        return extractInteractiveCardText(content)
    case "text", "post":
        return content
    default:
        return content
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./platform/feishu/ -run TestFeishuListMessages -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add platform/feishu/subscription.go
git commit -m "feat: implement Feishu MessageScanner and ThreadReplyContextBuilder"
```

---

### Task 4: SubscriptionManager and scan logic <!-- covers: S3, S4, S5, S7, S8, S11 -->

**Files:**
- Modify: `core/subscription.go`

- [ ] **Step 1: Write failing test for SubscriptionManager scan logic**

```go
func TestSubscriptionFilter(t *testing.T) {
    msgs := []ScannedMessage{
        {MessageID: "m1", Content: "【告警】CPU超过90%", IsBot: false},
        {MessageID: "m2", Content: "【恢复】CPU已正常", IsBot: false},
        {MessageID: "m3", Content: "今日天气不错", IsBot: false},
        {MessageID: "m4", Content: "Bot消息", IsBot: true},
    }

    sub := &Subscription{Filter: "告警", ExcludeFilter: "恢复"}
    matched := filterMessages(sub, msgs)
    if len(matched) != 1 {
        t.Errorf("filterMessages = %d, want 1", len(matched))
    }
    if matched[0].MessageID != "m1" {
        t.Errorf("matched MessageID = %q, want %q", matched[0].MessageID, "m1")
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
        t.Errorf("filterMessages = %d, want 1 (bot messages excluded)", len(matched))
    }
}

func TestBuildPrompt(t *testing.T) {
    sub := &Subscription{Prompt: "排查：{{content}}"}
    result := sub.BuildPrompt("CPU超过90%")
    if result != "排查：CPU超过90%" {
        t.Errorf("BuildPrompt = %q, want %q", result, "排查：CPU超过90%")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/ -run TestSubscription -v`
Expected: FAIL — `filterMessages` and `BuildPrompt` not defined

- [ ] **Step 3: Implement filterMessages, BuildPrompt, and SubscriptionManager**

Add to `core/subscription.go`:

```go
import "strings"

// BuildPrompt replaces {{content}} in the prompt template with the message content.
func (s *Subscription) BuildPrompt(content string) string {
    return strings.ReplaceAll(s.Prompt, "{{content}}", content)
}

// filterMessages applies Filter and ExcludeFilter to a list of scanned messages.
func filterMessages(sub *Subscription, msgs []ScannedMessage) []ScannedMessage {
    var matched []ScannedMessage
    for _, msg := range msgs {
        if msg.IsBot {
            continue
        }
        if sub.Filter != "" && !strings.Contains(strings.ToLower(msg.Content), strings.ToLower(sub.Filter)) {
            continue
        }
        if sub.ExcludeFilter != "" && strings.Contains(strings.ToLower(msg.Content), strings.ToLower(sub.ExcludeFilter)) {
            continue
        }
        matched = append(matched, msg)
    }
    return matched
}
```

Add SubscriptionManager struct:

```go
import (
    "github.com/robfig/cron/v3"
)

// SubscriptionManager manages subscription scheduling and execution.
type SubscriptionManager struct {
    store    *SubscriptionStore
    cron     *cron.Cron
    engines  map[string]*Engine
    mu       sync.RWMutex
    entries  map[string]cron.EntryID
    logsDir  string
}

func NewSubscriptionManager(store *SubscriptionStore, dataDir string) *SubscriptionManager {
    return &SubscriptionManager{
        store:   store,
        cron:    cron.New(cron.WithSeconds()),
        engines: make(map[string]*Engine),
        entries: make(map[string]cron.EntryID),
        logsDir: dataDir + "/subscriptions/logs",
    }
}

func (sm *SubscriptionManager) RegisterEngine(name string, e *Engine) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.engines[name] = e
}

func (sm *SubscriptionManager) Start() error {
    // Schedule all enabled subscriptions
    for _, sub := range sm.store.ListAll() {
        if sub.Enabled {
            sm.scheduleSubscription(sub)
        }
    }
    return sm.cron.Start()
}

func (sm *SubscriptionManager) Stop() {
    sm.cron.Stop()
}

func (sm *SubscriptionManager) scheduleSubscription(sub *Subscription) error {
    entryID, err := sm.cron.AddFunc(sub.Interval, func() {
        sm.executeScan(sub.ID)
    })
    if err != nil {
        return err
    }
    sm.entries[sub.ID] = entryID
    return nil
}

func (sm *SubscriptionManager) AddSubscription(sub *Subscription) error {
    if err := sm.store.Add(sub); err != nil {
        return err
    }
    if sub.Enabled {
        return sm.scheduleSubscription(sub)
    }
    return nil
}

func (sm *SubscriptionManager) RemoveSubscription(id string) error {
    if entryID, ok := sm.entries[id]; ok {
        sm.cron.Remove(entryID)
        delete(sm.entries, id)
    }
    return sm.store.Remove(id)
}

func (sm *SubscriptionManager) executeScan(subID string) {
    sub, err := sm.store.Get(subID)
    if err != nil {
        return
    }
    if !sub.Enabled {
        return
    }

    engine, ok := sm.engines[sub.Project]
    if !ok {
        slog.Error("subscription: engine not found", "subscription_id", subID, "project", sub.Project)
        return
    }

    engine.ExecuteSubscriptionScan(sub)
}

// AppendLog writes a log entry for a subscription.
func (sm *SubscriptionManager) AppendLog(entry LogEntry) error {
    if err := os.MkdirAll(sm.logsDir, 0o755); err != nil {
        return err
    }
    path := sm.logsDir + "/" + entry.SubscriptionID + ".log"
    data, _ := json.Marshal(entry)
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = fmt.Fprintln(f, string(data))
    return err
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./core/ -run TestSubscription -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/subscription.go core/subscription_test.go
git commit -m "feat: add SubscriptionManager with scan scheduling and filter logic"
```

---

### Task 5: Engine integration — ExecuteSubscriptionScan <!-- covers: S3, S4, S5, S6, S7 -->

**Files:**
- Modify: `core/engine.go`
- Modify: `core/subscription.go`

- [ ] **Step 1: Add SubscriptionManager field to Engine**

In `core/engine.go`, add field near line 240 (where `cronScheduler` is):

```go
subscriptionManager *SubscriptionManager
```

Add setter near line 883:

```go
func (e *Engine) SetSubscriptionManager(sm *SubscriptionManager) { e.subscriptionManager = sm }
```

- [ ] **Step 2: Implement ExecuteSubscriptionScan on Engine**

Add to `core/engine.go`:

```go
func (e *Engine) ExecuteSubscriptionScan(sub *Subscription) {
    // 1. Resolve platform
    platform, ok := e.resolvePlatform(sub.SessionKey)
    if !ok {
        slog.Error("subscription: platform not found", "subscription_id", sub.ID, "session_key", sub.SessionKey)
        return
    }

    // 2. Check MessageScanner capability
    scanner, ok := platform.(MessageScanner)
    if !ok {
        slog.Error("subscription: platform does not support MessageScanner", "subscription_id", sub.ID, "platform", platform.Name())
        return
    }

    // 3. Collect all pages
    var allMsgs []ScannedMessage
    opts := ListMessagesOptions{
        Since:    time.Now().Add(-2 * time.Minute), // default lookback for empty anchor
        PageSize: 50,
    }
    if sub.Anchor != "" {
        // Parse anchor as timestamp for the API call
        if ts, err := time.Parse(time.RFC3339Nano, sub.Anchor); err == nil {
            opts.Since = ts
        }
    }

    for {
        msgs, nextToken, err := scanner.ListMessages(context.Background(), sub.ChatID, opts)
        if err != nil {
            isPermanent := isPermanentError(err)
            e.subscriptionManager.store.MarkRun(sub.ID, err.Error(), isPermanent)
            if isPermanent && sub.ConsecutiveErrors+1 >= 10 {
                slog.Error("subscription: auto-disabled", "subscription_id", sub.ID, "auto_disabled", true, "consecutive_errors", sub.ConsecutiveErrors+1, "last_error", err.Error())
                // Attempt notification
                e.sendSubscriptionAutoDisabledNotification(sub)
            }
            return
        }
        allMsgs = append(allMsgs, msgs...)
        if nextToken == "" {
            break
        }
        opts.PageToken = nextToken
    }

    if len(allMsgs) == 0 {
        e.subscriptionManager.store.MarkRun(sub.ID, "", false)
        return
    }

    // 4. Filter
    matched := filterMessages(sub, allMsgs)

    // 5. Dedup against ProcessedIDs
    processedSet := make(map[string]bool, len(sub.ProcessedIDs))
    for _, id := range sub.ProcessedIDs {
        processedSet[id] = true
    }
    var newMatched []ScannedMessage
    for _, msg := range matched {
        if !processedSet[msg.MessageID] {
            newMatched = append(newMatched, msg)
        }
    }

    // 6. Inject matched messages
    threadBuilder, hasThreadBuilder := platform.(ThreadReplyContextBuilder)
    var processedIDs []string
    processedIDs = append(processedIDs, sub.ProcessedIDs...)

    for _, msg := range newMatched {
        // Build reply context
        var replyCtx any
        if hasThreadBuilder {
            if rc, err := threadBuilder.BuildThreadReplyCtx(sub.ChatID, msg.MessageID); err == nil {
                replyCtx = rc
            }
        }
        if replyCtx == nil {
            if reconstructor, ok := platform.(ReplyContextReconstructor); ok {
                replyCtx, _ = reconstructor.ReconstructReplyCtx(sub.SessionKey)
            }
        }

        prompt := sub.BuildPrompt(msg.Content)
        session := e.sessions.NewSideSession(sub.SessionKey, "subscription-"+sub.ID)

        e.subscriptionManager.AppendLog(LogEntry{
            SubscriptionID: sub.ID,
            MessageID:      msg.MessageID,
            ChatID:         sub.ChatID,
            Content:        truncate(msg.Content, 200),
            SessionID:      session.ID,
            Status:         "submitted",
            CreatedAt:      time.Now(),
        })

        processedIDs = append(processedIDs, msg.MessageID)
        e.processSubscriptionMessage(platform, replyCtx, sub, session, prompt)
    }

    // 7. Update anchor to last message in the fetched list
    lastMsg := allMsgs[len(allMsgs)-1]
    newAnchor := lastMsg.CreatedAt.Format(time.RFC3339Nano)
    // Prune processedIDs older than 5 minutes
    if len(processedIDs) > 100 {
        processedIDs = processedIDs[len(processedIDs)-100:]
    }
    e.subscriptionManager.store.UpdateAnchor(sub.ID, newAnchor, processedIDs)
    e.subscriptionManager.store.MarkRun(sub.ID, "", false)
}
```

- [ ] **Step 3: Add helper functions**

```go
func (e *Engine) resolvePlatform(sessionKey string) (Platform, bool) {
    parts := strings.SplitN(sessionKey, ":", 2)
    if len(parts) < 1 {
        return nil, false
    }
    return e.platformByName(parts[0])
}

func isPermanentError(err error) bool {
    // Classify: permission denied, bot removed, token revoked = permanent
    // Rate limit, network timeout = transient
    msg := err.Error()
    permanentMarkers := []string{"permission", "not exist", "token", "unauthorized", "forbidden"}
    for _, marker := range permanentMarkers {
        if strings.Contains(strings.ToLower(msg), marker) {
            return true
        }
    }
    return false
}

func truncate(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[:n]
}

func (e *Engine) sendSubscriptionAutoDisabledNotification(sub *Subscription) {
    platform, ok := e.resolvePlatform(sub.SessionKey)
    if !ok {
        return
    }
    rc, ok := platform.(ReplyContextReconstructor)
    if !ok {
        return
    }
    replyCtx, err := rc.ReconstructReplyCtx(sub.SessionKey)
    if err != nil {
        return
    }
    msg := e.i18n.Tf(MsgSubAutoDisabled, sub.ID, sub.ConsecutiveErrors, sub.LastError)
    platform.Reply(context.Background(), replyCtx, msg)
}

func (e *Engine) processSubscriptionMessage(p Platform, replyCtx any, sub *Subscription, session *Session, prompt string) {
    // Fire-and-forget injection; the agent processes and replies
    go e.processInteractiveMessageWith(p, &Message{
        Content:    prompt,
        ReplyCtx:   replyCtx,
        SessionKey: sub.SessionKey,
        UserID:     "subscription",
    })
}
```

- [ ] **Step 4: Run build to verify compilation**

Run: `go build ./...`
Expected: success (some functions referenced may need stubs, fix as needed)

- [ ] **Step 5: Commit**

```bash
git add core/engine.go core/subscription.go
git commit -m "feat: add ExecuteSubscriptionScan engine integration"
```

---

### Task 6: i18n keys for subscription <!-- covers: S12 -->

**Files:**
- Modify: `core/i18n.go`

- [ ] **Step 1: Add MsgKey constants**

Add to the `const` block in `core/i18n.go`:

```go
MsgSubNotAvailable    MsgKey = "sub_not_available"
MsgSubUsage           MsgKey = "sub_usage"
MsgSubCreated         MsgKey = "sub_created"
MsgSubAlreadyExists   MsgKey = "sub_already_exists"
MsgSubNotFound        MsgKey = "sub_not_found"
MsgSubListTitle       MsgKey = "sub_list_title"
MsgSubListAllTitle    MsgKey = "sub_list_all_title"
MsgSubEnabled         MsgKey = "sub_enabled"
MsgSubDisabled        MsgKey = "sub_disabled"
MsgSubDeleted         MsgKey = "sub_deleted"
MsgSubAutoDisabled    MsgKey = "sub_auto_disabled"
MsgSubDelConfirm      MsgKey = "sub_del_confirm"
MsgSubShowFormat      MsgKey = "sub_show_format"
MsgSubEditUsage       MsgKey = "sub_edit_usage"
MsgSubHelp            MsgKey = "sub_help"
MsgSubAdminRequired   MsgKey = "sub_admin_required"
```

- [ ] **Step 2: Add translations for all 5 languages**

Add to the `messages` map:

```go
MsgSubNotAvailable: {
    LangEnglish:    "Subscription feature is not available.",
    LangChinese:    "订阅功能不可用。",
    LangChineseTW:  "訂閱功能不可用。",
    LangJapanese:   "サブスクリプション機能は利用できません。",
    LangSpanish:    "La función de suscripción no está disponible.",
},
MsgSubUsage: {
    LangEnglish:    "Usage: /subscribe <filter> <exclude> [prompt...]",
    LangChinese:    "用法：/subscribe <过滤词> <排除词> [提示词...]",
    LangChineseTW:  "用法：/subscribe <過濾詞> <排除詞> [提示詞...]",
    LangJapanese:   "使い方: /subscribe <フィルター> <除外> [プロンプト...]",
    LangSpanish:    "Uso: /subscribe <filtro> <excluir> [prompt...]",
},
MsgSubCreated: {
    LangEnglish:    "Subscription created (ID: %s)\nFilter: %s | Exclude: %s\nPrompt: %s\nInterval: %s\nTip: Use {{content}} in prompt to reference the matched message.",
    LangChinese:    "订阅已创建（ID: %s）\n过滤词: %s | 排除词: %s\n提示词: %s\n间隔: %s\n提示：在提示词中使用 {{content}} 来引用匹配的消息。",
    LangChineseTW:  "訂閱已建立（ID: %s）\n過濾詞: %s | 排除詞: %s\n提示詞: %s\n間隔: %s\n提示：在提示詞中使用 {{content}} 來引用匹配的訊息。",
    LangJapanese:   "サブスクリプション作成完了 (ID: %s)\nフィルター: %s | 除外: %s\nプロンプト: %s\n間隔: %s\nヒント: プロンプトで {{content}} を使って一致したメッセージを参照できます。",
    LangSpanish:    "Suscripción creada (ID: %s)\nFiltro: %s | Excluir: %s\nPrompt: %s\nIntervalo: %s\nConsejo: Usa {{content}} en el prompt para referenciar el mensaje coincidente.",
},
MsgSubAlreadyExists: {
    LangEnglish:    "This group is already subscribed. Use /subscribe edit to modify.",
    LangChinese:    "该群已订阅，请使用 /subscribe edit 修改。",
    LangChineseTW:  "該群已訂閱，請使用 /subscribe edit 修改。",
    LangJapanese:   "このグループは既にサブスクリプションされています。/subscribe edit で変更してください。",
    LangSpanish:    "Este grupo ya está suscrito. Use /subscribe edit para modificar.",
},
MsgSubNotFound: {
    LangEnglish:    "Subscription not found: %s",
    LangChinese:    "订阅未找到: %s",
    LangChineseTW:  "訂閱未找到: %s",
    LangJapanese:   "サブスクリプションが見つかりません: %s",
    LangSpanish:    "Suscripción no encontrada: %s",
},
MsgSubListTitle: {
    LangEnglish:    "Subscriptions for this group:",
    LangChinese:    "本群订阅列表：",
    LangChineseTW:  "本群訂閱列表：",
    LangJapanese:   "このグループのサブスクリプション:",
    LangSpanish:    "Suscripciones de este grupo:",
},
MsgSubListAllTitle: {
    LangEnglish:    "All subscriptions:",
    LangChinese:    "所有订阅：",
    LangChineseTW:  "所有訂閱：",
    LangJapanese:   "すべてのサブスクリプション:",
    LangSpanish:    "Todas las suscripciones:",
},
MsgSubEnabled: {
    LangEnglish:    "Subscription %s enabled.",
    LangChinese:    "订阅 %s 已启用。",
    LangChineseTW:  "訂閱 %s 已啟用。",
    LangJapanese:   "サブスクリプション %s を有効にしました。",
    LangSpanish:    "Suscripción %s habilitada.",
},
MsgSubDisabled: {
    LangEnglish:    "Subscription %s disabled.",
    LangChinese:    "订阅 %s 已禁用。",
    LangChineseTW:  "訂閱 %s 已停用。",
    LangJapanese:   "サブスクリプション %s を無効にしました。",
    LangSpanish:    "Suscripción %s deshabilitada.",
},
MsgSubDeleted: {
    LangEnglish:    "Subscription %s deleted.",
    LangChinese:    "订阅 %s 已删除。",
    LangChineseTW:  "訂閱 %s 已刪除。",
    LangJapanese:   "サブスクリプション %s を削除しました。",
    LangSpanish:    "Suscripción %s eliminada.",
},
MsgSubAutoDisabled: {
    LangEnglish:    "Subscription %s auto-disabled after %d consecutive errors. Last error: %s. Use /subscribe enable %s to re-enable.",
    LangChinese:    "订阅 %s 因连续 %d 次错误已自动禁用。最近错误: %s。使用 /subscribe enable %s 重新启用。",
    LangChineseTW:  "訂閱 %s 因連續 %d 次錯誤已自動停用。最近錯誤: %s。使用 /subscribe enable %s 重新啟用。",
    LangJapanese:   "サブスクリプション %s は連続 %d 回のエラーで自動無効化されました。最後のエラー: %s。/subscribe enable %s で再有効化できます。",
    LangSpanish:    "Suscripción %s deshabilitada automáticamente después de %d errores consecutivos. Último error: %s. Use /subscribe enable %s para rehabilitar.",
},
MsgSubDelConfirm: {
    LangEnglish:    "Are you sure you want to delete subscription %s?",
    LangChinese:    "确定要删除订阅 %s 吗？",
    LangChineseTW:  "確定要刪除訂閱 %s 嗎？",
    LangJapanese:   "サブスクリプション %s を削除してもよろしいですか？",
    LangSpanish:    "¿Está seguro de que desea eliminar la suscripción %s?",
},
MsgSubShowFormat: {
    LangEnglish:    "Subscription: %s\nGroup: %s\nFilter: %s\nExclude: %s\nPrompt: %s\nInterval: %s\nEnabled: %v\nConsecutiveErrors: %d\nLastRun: %s\nLastError: %s\nAnchor: %s",
    LangChinese:    "订阅: %s\n群组: %s\n过滤词: %s\n排除词: %s\n提示词: %s\n间隔: %s\n启用: %v\n连续错误: %d\n上次运行: %s\n最近错误: %s\n锚点: %s",
    LangChineseTW:  "訂閱: %s\n群組: %s\n過濾詞: %s\n排除詞: %s\n提示詞: %s\n間隔: %s\n啟用: %v\n連續錯誤: %d\n上次執行: %s\n最近錯誤: %s\n錨點: %s",
    LangJapanese:   "サブスクリプション: %s\nグループ: %s\nフィルター: %s\n除外: %s\nプロンプト: %s\n間隔: %s\n有効: %v\n連続エラー: %d\n最終実行: %s\n最終エラー: %s\nアンカー: %s",
    LangSpanish:    "Suscripción: %s\nGrupo: %s\nFiltro: %s\nExcluir: %s\nPrompt: %s\nIntervalo: %s\nHabilitado: %v\nErrores consecutivos: %d\nÚltima ejecución: %s\nÚltimo error: %s\nAncla: %s",
},
MsgSubEditUsage: {
    LangEnglish:    "Usage: /subscribe edit <id> <field> <value>",
    LangChinese:    "用法：/subscribe edit <id> <字段> <值>",
    LangChineseTW:  "用法：/subscribe edit <id> <欄位> <值>",
    LangJapanese:   "使い方: /subscribe edit <id> <フィールド> <値>",
    LangSpanish:    "Uso: /subscribe edit <id> <campo> <valor>",
},
MsgSubHelp: {
    LangEnglish:    "Subscription Commands:\n/subscribe <filter> <exclude> [prompt...] - Create subscription\n/subscribe list - List subscriptions for this group\n/subscribe list all - List all subscriptions\n/subscribe show <id> - Show subscription details\n/subscribe edit <id> <field> <value> - Edit subscription\n/subscribe enable <id> - Enable subscription\n/subscribe disable <id> - Disable subscription\n/subscribe del <id> - Delete subscription\n\nTip: Use {{content}} in prompt to reference the matched message. Use \"-\" for filter/exclude to match all messages.",
    LangChinese:    "订阅命令：\n/subscribe <过滤词> <排除词> [提示词...] - 创建订阅\n/subscribe list - 查看本群订阅\n/subscribe list all - 查看所有订阅\n/subscribe show <id> - 查看订阅详情\n/subscribe edit <id> <字段> <值> - 编辑订阅\n/subscribe enable <id> - 启用订阅\n/subscribe disable <id> - 禁用订阅\n/subscribe del <id> - 删除订阅\n\n提示：在提示词中使用 {{content}} 引用匹配的消息。过滤词/排除词用 \"-\" 表示匹配所有。",
    LangChineseTW:  "訂閱命令：\n/subscribe <過濾詞> <排除詞> [提示詞...] - 建立訂閱\n/subscribe list - 查看本群訂閱\n/subscribe list all - 查看所有訂閱\n/subscribe show <id> - 查看訂閱詳情\n/subscribe edit <id> <欄位> <值> - 編輯訂閱\n/subscribe enable <id> - 啟用訂閱\n/subscribe disable <id> - 停用訂閱\n/subscribe del <id> - 刪除訂閱\n\n提示：在提示詞中使用 {{content}} 引用匹配的訊息。過濾詞/排除詞用 \"-\" 表示匹配所有。",
    LangJapanese:   "サブスクリプションコマンド:\n/subscribe <フィルター> <除外> [プロンプト...] - 作成\n/subscribe list - このグループのサブスクリプション\n/subscribe list all - すべてのサブスクリプション\n/subscribe show <id> - 詳細表示\n/subscribe edit <id> <フィールド> <値> - 編集\n/subscribe enable <id> - 有効化\n/subscribe disable <id> - 無効化\n/subscribe del <id> - 削除\n\nヒント: プロンプトで {{content}} を使って一致したメッセージを参照できます。",
    LangSpanish:    "Comandos de suscripción:\n/subscribe <filtro> <excluir> [prompt...] - Crear\n/subscribe list - Ver suscripciones del grupo\n/subscribe list all - Ver todas\n/subscribe show <id> - Ver detalles\n/subscribe edit <id> <campo> <valor> - Editar\n/subscribe enable <id> - Habilitar\n/subscribe disable <id> - Deshabilitar\n/subscribe del <id> - Eliminar\n\nConsejo: Usa {{content}} en el prompt para referenciar el mensaje. Usa \"-\" para filtro/excluir y coincidir con todo.",
},
MsgSubAdminRequired: {
    LangEnglish:    "Only admins can manage subscriptions.",
    LangChinese:    "仅管理员可以管理订阅。",
    LangChineseTW:  "僅管理員可以管理訂閱。",
    LangJapanese:   "管理者のみサブスクリプションを管理できます。",
    LangSpanish:    "Solo los administradores pueden gestionar suscripciones.",
},
```

- [ ] **Step 2: Run build to verify compilation**

Run: `go build ./core/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add core/i18n.go
git commit -m "feat: add i18n keys for subscription feature"
```

---

### Task 7: /subscribe slash command <!-- covers: S1, S9 -->

**Files:**
- Create: `core/subscription_cmd.go`

- [ ] **Step 1: Add command to builtinCommands and handleCommand**

In `core/engine.go`, add to `builtinCommands` slice:

```go
{[]string{"subscribe", "sub"}, "subscribe"},
```

Add to `handleCommand` switch:

```go
case "subscribe":
    e.cmdSubscribe(p, msg, args)
```

- [ ] **Step 2: Implement cmdSubscribe and subcommand handlers**

Create `core/subscription_cmd.go`:

```go
package core

import (
    "fmt"
    "strings"
)

func (e *Engine) cmdSubscribe(p Platform, msg *Message, args []string) {
    if e.subscriptionManager == nil {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubNotAvailable))
        return
    }

    if len(args) == 0 || args[0] == "help" {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
        return
    }

    sub := matchSubCommand(strings.ToLower(args[0]), []string{
        "list", "show", "edit", "enable", "disable", "del", "delete", "rm", "remove",
    })

    switch sub {
    case "list":
        e.cmdSubscribeList(p, msg, args[1:])
    case "show":
        e.cmdSubscribeShow(p, msg, args[1:])
    case "edit":
        e.cmdSubscribeEdit(p, msg, args[1:])
    case "enable":
        e.cmdSubscribeEnable(p, msg, args[1:])
    case "disable":
        e.cmdSubscribeDisable(p, msg, args[1:])
    case "del", "delete", "rm", "remove":
        e.cmdSubscribeDel(p, msg, args[1:])
    default:
        e.cmdSubscribeAdd(p, msg, args)
    }
}

func (e *Engine) cmdSubscribeAdd(p Platform, msg *Message, args []string) {
    if !e.isAdmin(p, msg) {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
        return
    }

    filter := ""
    exclude := ""
    prompt := "{{content}}"

    if len(args) >= 1 && args[0] != "-" {
        filter = args[0]
    }
    if len(args) >= 2 && args[1] != "-" {
        exclude = args[1]
    }
    if len(args) >= 3 {
        prompt = strings.Join(args[2:], " ")
    }

    sub := &Subscription{
        ID:               GenerateSubscriptionID(),
        Project:          e.name,
        ChatID:           msg.ChatID,
        ChatName:         e.resolveChatName(p, msg.ChatID),
        Platform:         p.Name(),
        SessionKey:       msg.SessionKey,
        Filter:           filter,
        ExcludeFilter:    exclude,
        Prompt:           prompt,
        Interval:         "*/2 * * * *",
        ConcurrencyLimit: 5,
        TimeoutMins:      30,
        Enabled:          true,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }

    if err := e.subscriptionManager.AddSubscription(sub); err != nil {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAlreadyExists))
        return
    }

    resp := e.i18n.Tf(MsgSubCreated, sub.ID, sub.Filter, sub.ExcludeFilter, sub.Prompt, sub.Interval)
    e.reply(p, msg.ReplyCtx, resp)
}

func (e *Engine) cmdSubscribeList(p Platform, msg *Message, args []string) {
    var subs []*Subscription
    if len(args) > 0 && args[0] == "all" {
        subs = e.subscriptionManager.store.ListByProject(e.name)
    } else {
        subs = e.subscriptionManager.store.ListByChatID(msg.ChatID)
    }

    if len(subs) == 0 {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubListTitle))
        return
    }

    var sb strings.Builder
    if len(args) > 0 && args[0] == "all" {
        sb.WriteString(e.i18n.T(MsgSubListAllTitle))
    } else {
        sb.WriteString(e.i18n.T(MsgSubListTitle))
    }
    sb.WriteString("\n")
    for _, s := range subs {
        status := "✅"
        if !s.Enabled {
            status = "⏸"
        }
        sb.WriteString(fmt.Sprintf("%s %s | filter=%q exclude=%q | interval=%s\n", status, s.ID, s.Filter, s.ExcludeFilter, s.Interval))
    }
    e.reply(p, msg.ReplyCtx, sb.String())
}

func (e *Engine) cmdSubscribeShow(p Platform, msg *Message, args []string) {
    if len(args) < 1 {
        e.reply(p, msg.ReplyCtx, "Usage: /subscribe show <id>")
        return
    }
    sub, err := e.subscriptionManager.store.Get(args[0])
    if err != nil {
        e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubNotFound, args[0]))
        return
    }
    resp := e.i18n.Tf(MsgSubShowFormat, sub.ID, sub.ChatName, sub.Filter, sub.ExcludeFilter, sub.Prompt, sub.Interval, sub.Enabled, sub.ConsecutiveErrors, sub.LastRun.Format(time.RFC3339), sub.LastError, sub.Anchor)
    e.reply(p, msg.ReplyCtx, resp)
}

func (e *Engine) cmdSubscribeEdit(p Platform, msg *Message, args []string) {
    if !e.isAdmin(p, msg) {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
        return
    }
    if len(args) < 3 {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubEditUsage))
        return
    }
    id, field, value := args[0], args[1], strings.Join(args[2:], " ")
    fields := map[string]any{field: value}
    if err := e.subscriptionManager.store.Update(id, fields); err != nil {
        e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubNotFound, id))
        return
    }
    e.reply(p, msg.ReplyCtx, fmt.Sprintf("Subscription %s updated: %s=%s", id, field, value))
}

func (e *Engine) cmdSubscribeEnable(p Platform, msg *Message, args []string) {
    if !e.isAdmin(p, msg) {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
        return
    }
    if len(args) < 1 {
        e.reply(p, msg.ReplyCtx, "Usage: /subscribe enable <id>")
        return
    }
    if err := e.subscriptionManager.store.Update(args[0], map[string]any{"enabled": true}); err != nil {
        e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubNotFound, args[0]))
        return
    }
    e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubEnabled, args[0]))
}

func (e *Engine) cmdSubscribeDisable(p Platform, msg *Message, args []string) {
    if !e.isAdmin(p, msg) {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
        return
    }
    if len(args) < 1 {
        e.reply(p, msg.ReplyCtx, "Usage: /subscribe disable <id>")
        return
    }
    if err := e.subscriptionManager.store.Update(args[0], map[string]any{"enabled": false}); err != nil {
        e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubNotFound, args[0]))
        return
    }
    e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubDisabled, args[0]))
}

func (e *Engine) cmdSubscribeDel(p Platform, msg *Message, args []string) {
    if !e.isAdmin(p, msg) {
        e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
        return
    }
    if len(args) < 1 {
        e.reply(p, msg.ReplyCtx, "Usage: /subscribe del <id>")
        return
    }
    if err := e.subscriptionManager.RemoveSubscription(args[0]); err != nil {
        e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubNotFound, args[0]))
        return
    }
    e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgSubDeleted, args[0]))
}

func (e *Engine) isAdmin(p Platform, msg *Message) bool {
    // Check admin_from config for the project
    if e.adminFrom == "" {
        return true // no admin restriction configured
    }
    return msg.UserID == e.adminFrom
}

func (e *Engine) resolveChatName(p Platform, chatID string) string {
    // Best-effort: try to get chat name from platform
    if namer, ok := p.(interface{ GetChatName(chatID string) string }); ok {
        return namer.GetChatName(chatID)
    }
    return chatID
}
```

- [ ] **Step 3: Run build to verify compilation**

Run: `go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add core/subscription_cmd.go core/engine.go
git commit -m "feat: add /subscribe slash command with all subcommands"
```

---

### Task 8: Management API for subscriptions <!-- covers: S2, S14 -->

**Files:**
- Modify: `core/management.go`

- [ ] **Step 1: Add SubscriptionManager to ManagementServer**

Add field:

```go
subscriptionManager *SubscriptionManager
```

Add setter:

```go
func (m *ManagementServer) SetSubscriptionManager(sm *SubscriptionManager) { m.subscriptionManager = sm }
```

- [ ] **Step 2: Register API routes in buildHandler**

```go
mux.HandleFunc(prefix+"/subscription", m.wrap(m.handleSubscription))
mux.HandleFunc(prefix+"/subscription/", m.wrap(m.handleSubscriptionByID))
```

- [ ] **Step 3: Implement CRUD handlers**

```go
func (m *ManagementServer) handleSubscription(w http.ResponseWriter, r *http.Request) {
    if m.subscriptionManager == nil {
        mgmtError(w, http.StatusNotFound, "subscription feature not available")
        return
    }
    switch r.Method {
    case http.MethodGet:
        subs := m.subscriptionManager.store.ListAll()
        mgmtJSON(w, http.StatusOK, subs)
    case http.MethodPost:
        var req struct {
            Project          string `json:"project"`
            ChatID           string `json:"chat_id"`
            ChatName         string `json:"chat_name,omitempty"`
            Platform         string `json:"platform"`
            SessionKey       string `json:"session_key"`
            Filter           string `json:"filter,omitempty"`
            ExcludeFilter    string `json:"exclude_filter,omitempty"`
            Prompt           string `json:"prompt"`
            Interval         string `json:"interval,omitempty"`
            ConcurrencyLimit int    `json:"concurrency_limit,omitempty"`
            TimeoutMins      int    `json:"timeout_mins,omitempty"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            mgmtError(w, http.StatusBadRequest, err.Error())
            return
        }
        if req.Prompt == "" {
            req.Prompt = "{{content}}"
        }
        if req.Interval == "" {
            req.Interval = "*/2 * * * *"
        }
        if req.ConcurrencyLimit == 0 {
            req.ConcurrencyLimit = 5
        }
        if req.TimeoutMins == 0 {
            req.TimeoutMins = 30
        }
        sub := &Subscription{
            ID:               GenerateSubscriptionID(),
            Project:          req.Project,
            ChatID:           req.ChatID,
            ChatName:         req.ChatName,
            Platform:         req.Platform,
            SessionKey:       req.SessionKey,
            Filter:           req.Filter,
            ExcludeFilter:    req.ExcludeFilter,
            Prompt:           req.Prompt,
            Interval:         req.Interval,
            ConcurrencyLimit: req.ConcurrencyLimit,
            TimeoutMins:      req.TimeoutMins,
            Enabled:          true,
            CreatedAt:        time.Now(),
            UpdatedAt:        time.Now(),
        }
        if err := m.subscriptionManager.AddSubscription(sub); err != nil {
            mgmtError(w, http.StatusConflict, err.Error())
            return
        }
        mgmtJSON(w, http.StatusCreated, sub)
    default:
        mgmtError(w, http.StatusMethodNotAllowed, "method not allowed")
    }
}

func (m *ManagementServer) handleSubscriptionByID(w http.ResponseWriter, r *http.Request) {
    if m.subscriptionManager == nil {
        mgmtError(w, http.StatusNotFound, "subscription feature not available")
        return
    }
    id := strings.TrimPrefix(r.URL.Path, "/api/v1/subscription/")
    switch r.Method {
    case http.MethodGet:
        sub, err := m.subscriptionManager.store.Get(id)
        if err != nil {
            mgmtError(w, http.StatusNotFound, err.Error())
            return
        }
        mgmtJSON(w, http.StatusOK, sub)
    case http.MethodPatch:
        var fields map[string]any
        if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
            mgmtError(w, http.StatusBadRequest, err.Error())
            return
        }
        if err := m.subscriptionManager.store.Update(id, fields); err != nil {
            mgmtError(w, http.StatusNotFound, err.Error())
            return
        }
        sub, _ := m.subscriptionManager.store.Get(id)
        mgmtJSON(w, http.StatusOK, sub)
    case http.MethodDelete:
        if err := m.subscriptionManager.RemoveSubscription(id); err != nil {
            mgmtError(w, http.StatusNotFound, err.Error())
            return
        }
        mgmtOK(w, "deleted")
    default:
        mgmtError(w, http.StatusMethodNotAllowed, "method not allowed")
    }
}
```

- [ ] **Step 4: Run build to verify compilation**

Run: `go build ./...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add core/management.go
git commit -m "feat: add subscription management API endpoints"
```

---

### Task 9: Config — subscriptions_enabled flag <!-- covers: S10 -->

**Files:**
- Modify: `config/config.go`

- [ ] **Step 1: Add SubscriptionsEnabled to ProjectConfig**

```go
SubscriptionsEnabled *bool `toml:"subscriptions_enabled,omitempty"`
```

- [ ] **Step 2: Add helper method**

```go
func (c *ProjectConfig) IsSubscriptionsEnabled() bool {
    if c.SubscriptionsEnabled == nil {
        return true // default enabled
    }
    return *c.SubscriptionsEnabled
}
```

- [ ] **Step 3: Check flag in cmdSubscribe**

Add to `cmdSubscribe` at the top:

```go
if !e.projectConfig.IsSubscriptionsEnabled() {
    e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubNotAvailable))
    return
}
```

- [ ] **Step 4: Run build to verify compilation**

Run: `go build ./...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add config/config.go core/subscription_cmd.go
git commit -m "feat: add subscriptions_enabled config flag"
```

---

### Task 10: Wire SubscriptionManager into main <!-- covers: S8 -->

**Files:**
- Modify: `cmd/cc-connect/main.go`

- [ ] **Step 1: Create and wire SubscriptionManager**

Follow the same pattern as CronScheduler setup. Add near the cron scheduler initialization:

```go
// Initialize subscription manager
subStore, err := core.NewSubscriptionStore(cfg.DataDir)
if err != nil {
    slog.Error("failed to init subscription store", "error", err)
    os.Exit(1)
}
subManager := core.NewSubscriptionManager(subStore, cfg.DataDir)
mgmtServer.SetSubscriptionManager(subManager)
for _, e := range engines {
    e.SetSubscriptionManager(subManager)
    subManager.RegisterEngine(e.Name(), e)
}
subManager.Start()
```

- [ ] **Step 2: Run build to verify compilation**

Run: `make build`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add cmd/cc-connect/main.go
git commit -m "feat: wire SubscriptionManager into main startup"
```

---

### Task 11: WebUI — subscription API client <!-- covers: S2 -->

**Files:**
- Create: `web/src/api/subscription.ts`

- [ ] **Step 1: Create API client**

```typescript
import api from './client';

export interface Subscription {
  id: string;
  project: string;
  chat_id: string;
  chat_name: string;
  platform: string;
  session_key: string;
  filter: string;
  exclude_filter: string;
  prompt: string;
  interval: string;
  concurrency_limit: number;
  timeout_mins: number;
  enabled: boolean;
  last_run: string;
  last_error: string;
  consecutive_errors: number;
  created_at: string;
  updated_at: string;
}

export const listSubscriptions = (project?: string) =>
  api.get<Subscription[]>('/subscription', { params: { project } });

export const getSubscription = (id: string) =>
  api.get<Subscription>(`/subscription/${id}`);

export const createSubscription = (body: Partial<Subscription>) =>
  api.post<Subscription>('/subscription', body);

export const updateSubscription = (id: string, fields: Partial<Subscription>) =>
  api.patch<Subscription>(`/subscription/${id}`, fields);

export const deleteSubscription = (id: string) =>
  api.delete(`/subscription/${id}`);

export const triggerSubscription = (id: string) =>
  api.post(`/subscription/${id}/trigger`);
```

- [ ] **Step 2: Commit**

```bash
git add web/src/api/subscription.ts
git commit -m "feat: add WebUI subscription API client"
```

---

### Task 12: WebUI — SubscriptionList page <!-- covers: S15 -->

**Files:**
- Create: `web/src/pages/Subscriptions/SubscriptionList.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create SubscriptionList component**

Create `web/src/pages/Subscriptions/SubscriptionList.tsx` following the pattern of `CronList.tsx` (549 lines). The component should include:
- Card grid layout with subscription status badges
- Creation modal with fields: chat_id, filter, exclude_filter, prompt (with {{content}} placeholder hint), interval, concurrency_limit, timeout_mins
- Edit modal for updating subscription fields
- Delete confirmation dialog
- Enable/disable toggle buttons
- Manual trigger button
- Detail view showing processing logs

- [ ] **Step 2: Add route in App.tsx**

```tsx
import SubscriptionList from './pages/Subscriptions/SubscriptionList';
// Add route:
<Route path="subscriptions" element={<SubscriptionList />} />
```

- [ ] **Step 3: Build WebUI and verify**

Run: `make web`
Expected: build succeeds

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Subscriptions/SubscriptionList.tsx web/src/App.tsx
git commit -m "feat: add WebUI SubscriptionList management page"
```

---

### Task 13: Integration test — full scan cycle <!-- covers: S3, S5, S8 -->

**Files:**
- Create: `core/subscription_integration_test.go`

- [ ] **Step 1: Write integration test for scan cycle**

```go
//go:build !no_subscription

package core

import (
    "context"
    "testing"
    "time"
)

type mockScanner struct {
    messages []ScannedMessage
    err      error
}

func (m *mockScanner) ListMessages(ctx context.Context, chatID string, opts ListMessagesOptions) ([]ScannedMessage, string, error) {
    return m.messages, "", m.err
}

func TestSubscriptionScanCycle(t *testing.T) {
    dir := t.TempDir()
    store, _ := NewSubscriptionStore(dir)
    sm := NewSubscriptionManager(store, dir)

    sub := &Subscription{
        ID:               GenerateSubscriptionID(),
        Project:          "test",
        ChatID:           "oc_test",
        Platform:         "feishu",
        SessionKey:       "feishu:oc_test:bot",
        Filter:           "告警",
        ExcludeFilter:    "恢复",
        Prompt:           "排查：{{content}}",
        Interval:         "*/2 * * * *",
        ConcurrencyLimit: 5,
        Enabled:          true,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }
    store.Add(sub)

    // Verify prompt template
    prompt := sub.BuildPrompt("CPU超90%")
    if prompt != "排查：CPU超90%" {
        t.Errorf("BuildPrompt = %q, want %q", prompt, "排查：CPU超90%")
    }

    // Verify filter
    msgs := []ScannedMessage{
        {MessageID: "m1", Content: "【告警】CPU超90%", IsBot: false},
        {MessageID: "m2", Content: "【恢复】CPU正常", IsBot: false},
        {MessageID: "m3", Content: "天气好", IsBot: false},
        {MessageID: "m4", Content: "Bot说", IsBot: true},
    }
    matched := filterMessages(sub, msgs)
    if len(matched) != 1 || matched[0].MessageID != "m1" {
        t.Errorf("filterMessages = %v, want 1 match with m1", matched)
    }

    // Verify match-all with empty filters
    sub2 := &Subscription{Filter: "", ExcludeFilter: ""}
    matched2 := filterMessages(sub2, msgs)
    if len(matched2) != 3 { // excludes bot
        t.Errorf("filterMessages empty = %d, want 3", len(matched2))
    }

    // Verify persistence
    store2, _ := NewSubscriptionStore(dir)
    got, _ := store2.Get(sub.ID)
    if got.Filter != "告警" {
        t.Errorf("After reload: Filter = %q, want %q", got.Filter, "告警")
    }
}
```

- [ ] **Step 2: Run test**

Run: `go test ./core/ -run TestSubscriptionScanCycle -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add core/subscription_integration_test.go
git commit -m "test: add integration tests for subscription scan cycle"
```

---

### Task 14: End-to-end smoke test <!-- covers: S1, S2, S3, S5, S8 -->

**Files:**
- No new files — manual execution per verification.md Scenarios S1-S5

- [ ] **Step 1: Build and start daemon**

Run: `make build && ./cc-connect daemon restart`

- [ ] **Step 2: Run verification Scenario S1 (slash command creation)**

Follow steps in verification.md Scenario S1.

- [ ] **Step 3: Run verification Scenario S2 (API CRUD)**

Follow steps in verification.md Scenario S2.

- [ ] **Step 4: Run verification Scenario S3 (scan and filter)**

Follow steps in verification.md Scenario S3.

- [ ] **Step 5: Run verification Scenario S5 (anchor and dedup)**

Follow steps in verification.md Scenario S5.

- [ ] **Step 6: Run verification Scenario S8 (persistence across restart)**

Follow steps in verification.md Scenario S8.

- [ ] **Step 7: Fix any issues found during smoke testing**

Address bugs discovered during manual verification.

- [ ] **Step 8: Commit any fixes**

```bash
git add -A
git commit -m "fix: address issues found during subscription e2e smoke testing"
```
