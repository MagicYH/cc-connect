# Subscription Thread Reply & Filter Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** ./../specs/2026-06-28-subscription-thread-reply-filter-fix-design.md
**Verification:** ./../specs/2026-06-28-subscription-thread-reply-filter-fix-verification.md

**Goal:** Fix subscription replies to use reply_in_thread and fix filter rules to exclude human/self-bot messages with regex support.

**Architecture:** Add `forceThreadReply` flag to replyContext, change BuildThreadReplyCtx to return thread session key, align ExecuteSubscriptionScan to use effectiveSessionKey for sessions/iKey/concurrency, fix filterMessages to exclude humans and self-bot with regex caching, update /sub alias and /help text.

**Tech Stack:** Go 1.22+, regexp (stdlib), Feishu OAPI SDK

---

### Task 1: Add BotIDProvider interface and forceThreadReply to replyContext <!-- covers: S4, S18 -->

**Files:**
- Modify: `core/interfaces.go:60-64`
- Modify: `platform/feishu/feishu.go:109-113`
- Test: `core/subscription_test.go`

- [ ] **Step 1: Write failing test for BotIDProvider interface existence**

```go
func TestBotIDProviderInterface(t *testing.T) {
    var _ BotIDProvider = (*stubBotIDProvider)(nil)
}
```

Add stub in test file:
```go
type stubBotIDProvider struct{ id string }
func (s *stubBotIDProvider) BotID() string { return s.id }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./core/ -run TestBotIDProviderInterface`
Expected: FAIL — `BotIDProvider` undefined

- [ ] **Step 3: Add BotIDProvider interface to core/interfaces.go**

After `ThreadReplyContextBuilder` (line 64), add:

```go
// BotIDProvider returns the current bot's ID, used by subscription
// filtering to exclude self-bot messages.
type BotIDProvider interface {
	BotID() string
}
```

- [ ] **Step 4: Add forceThreadReply to replyContext in platform/feishu/feishu.go**

Change lines 109-113 from:
```go
type replyContext struct {
	messageID  string
	chatID     string
	sessionKey string
}
```
To:
```go
type replyContext struct {
	messageID        string
	chatID           string
	sessionKey       string
	forceThreadReply bool
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -race -v ./core/ -run TestBotIDProviderInterface`
Expected: PASS

- [ ] **Step 6: Verify full build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add core/interfaces.go platform/feishu/feishu.go
git commit -m "feat: add BotIDProvider interface and forceThreadReply field"
```

---

### Task 2: Change ThreadReplyContextBuilder signature and BuildThreadReplyCtx <!-- covers: S13, S16 -->

**Files:**
- Modify: `core/interfaces.go:60-64`
- Modify: `platform/feishu/subscription.go:109-111`
- Modify: `core/subscription_test.go` (stub adapters)
- Test: `platform/feishu/subscription_test.go` (new file or existing)

- [ ] **Step 1: Write failing test for BuildThreadReplyCtx returning thread session key**

In `platform/feishu/subscription_test.go` (create if needed):

```go
package feishu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildThreadReplyCtx_ReturnsThreadSessionKey(t *testing.T) {
	p := &Platform{}
	rc, tsk, err := p.BuildThreadReplyCtx("feishu:oc_chat:ou_user", "oc_chat", "om_msg123")
	require.NoError(t, err)
	assert.Equal(t, "feishu:oc_chat:root:om_msg123", tsk)
	replyCtx := rc.(replyContext)
	assert.True(t, replyCtx.forceThreadReply)
	assert.Equal(t, tsk, replyCtx.sessionKey)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./platform/feishu/ -run TestBuildThreadReplyCtx_ReturnsThreadSessionKey`
Expected: FAIL — `BuildThreadReplyCtx` returns 2 values, test expects 3

- [ ] **Step 3: Change ThreadReplyContextBuilder interface signature in core/interfaces.go**

Change lines 60-64 from:
```go
type ThreadReplyContextBuilder interface {
	BuildThreadReplyCtx(sessionKey string, chatID string, messageID string) (any, error)
}
```
To:
```go
type ThreadReplyContextBuilder interface {
	BuildThreadReplyCtx(sessionKey string, chatID string, messageID string) (replyCtx any, threadSessionKey string, err error)
}
```

- [ ] **Step 4: Update BuildThreadReplyCtx implementation in platform/feishu/subscription.go**

Change lines 109-111 from:
```go
func (p *Platform) BuildThreadReplyCtx(sessionKey string, chatID string, messageID string) (any, error) {
	return replyContext{chatID: chatID, messageID: messageID, sessionKey: sessionKey}, nil
}
```
To:
```go
func (p *Platform) BuildThreadReplyCtx(sessionKey string, chatID string, messageID string) (any, string, error) {
	threadSessionKey := fmt.Sprintf("%s:%s:root:%s", p.tag(), chatID, messageID)
	return replyContext{
		chatID:           chatID,
		messageID:        messageID,
		sessionKey:       threadSessionKey,
		forceThreadReply: true,
	}, threadSessionKey, nil
}
```

- [ ] **Step 5: Fix compilation errors in callers**

Update `core/engine.go:16358` — change from:
```go
if rc, err := threadBuilder.BuildThreadReplyCtx(sub.SessionKey, sub.ChatID, msg.MessageID); err == nil {
    replyCtx = rc
}
```
To:
```go
if rc, tsk, err := threadBuilder.BuildThreadReplyCtx(sub.SessionKey, sub.ChatID, msg.MessageID); err == nil {
    replyCtx = rc
    _ = tsk
}
```

Update all test stub implementations of `ThreadReplyContextBuilder` in `core/subscription_test.go` — change `BuildThreadReplyCtx` methods to return `(any, string, error)` with empty string as second return value.

Search for all `BuildThreadReplyCtx` implementations:
```bash
grep -rn 'BuildThreadReplyCtx' core/ platform/
```
Update each one.

- [ ] **Step 6: Run test to verify it passes**

Run: `go test -race -v ./platform/feishu/ -run TestBuildThreadReplyCtx_ReturnsThreadSessionKey`
Expected: PASS

- [ ] **Step 7: Run full test suite**

Run: `go test -race ./...`
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add core/interfaces.go platform/feishu/subscription.go core/engine.go core/subscription_test.go
git commit -m "feat: BuildThreadReplyCtx returns thread session key"
```

---

### Task 3: Fix shouldReplyInThread and buildReplyMessageReqBody <!-- covers: S7, S9 -->

**Files:**
- Modify: `platform/feishu/feishu.go:3505-3510` (shouldReplyInThread)
- Modify: `platform/feishu/feishu.go:3527-3535` (buildReplyMessageReqBody)
- Test: `platform/feishu/feishu_test.go` (new or existing)

- [ ] **Step 1: Write failing test for shouldReplyInThread with thread session key**

```go
func TestShouldReplyInThread_ThreadSessionKey(t *testing.T) {
	p := &Platform{threadIsolation: false}
	rc := replyContext{
		messageID:  "om_msg123",
		chatID:     "oc_chat123",
		sessionKey: "feishu:oc_chat123:root:om_rootMsg",
	}
	assert.True(t, p.shouldReplyInThread(rc))
}

func TestShouldReplyInThread_NoThreadKey_WithThreadIsolation(t *testing.T) {
	p := &Platform{threadIsolation: true}
	rc := replyContext{
		messageID:  "om_msg123",
		chatID:     "oc_chat123",
		sessionKey: "feishu:oc_chat123:ou_user",
	}
	assert.True(t, p.shouldReplyInThread(rc))
}

func TestShouldReplyInThread_NoThreadKey_NoThreadIsolation(t *testing.T) {
	p := &Platform{threadIsolation: false}
	rc := replyContext{
		messageID:  "om_msg123",
		chatID:     "oc_chat123",
		sessionKey: "feishu:oc_chat123:ou_user",
	}
	assert.False(t, p.shouldReplyInThread(rc))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./platform/feishu/ -run TestShouldReplyInThread`
Expected: `TestShouldReplyInThread_ThreadSessionKey` FAILS — returns false when `threadIsolation=false`

- [ ] **Step 3: Fix shouldReplyInThread**

Change lines 3505-3510 from:
```go
func (p *Platform) shouldReplyInThread(rc replyContext) bool {
	if rc.messageID == "" {
		return false
	}
	return p.threadIsolation && isThreadSessionKey(rc.sessionKey)
}
```
To:
```go
func (p *Platform) shouldReplyInThread(rc replyContext) bool {
	if rc.messageID == "" {
		return false
	}
	if isThreadSessionKey(rc.sessionKey) {
		return true
	}
	return p.threadIsolation
}
```

- [ ] **Step 4: Fix buildReplyMessageReqBody to check forceThreadReply**

Change lines 3527-3535 from:
```go
func (p *Platform) buildReplyMessageReqBody(rc replyContext, msgType, content string) *larkim.ReplyMessageReqBody {
	body := larkim.NewReplyMessageReqBodyBuilder().
		MsgType(msgType).
		Content(content)
	if p.shouldReplyInThread(rc) {
		body.ReplyInThread(true)
	}
	return body.Build()
}
```
To:
```go
func (p *Platform) buildReplyMessageReqBody(rc replyContext, msgType, content string) *larkim.ReplyMessageReqBody {
	body := larkim.NewReplyMessageReqBodyBuilder().
		MsgType(msgType).
		Content(content)
	if rc.forceThreadReply || p.shouldReplyInThread(rc) {
		body.ReplyInThread(true)
	}
	return body.Build()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -race -v ./platform/feishu/ -run TestShouldReplyInThread`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add platform/feishu/feishu.go
git commit -m "fix: shouldReplyInThread returns true for thread session keys"
```

---

### Task 4: Add reply_in_thread fallback for error codes 230071/230072 <!-- covers: S8, S19 -->

**Files:**
- Modify: `platform/feishu/feishu.go:3537-3554` (replyMessage)
- Test: `platform/feishu/feishu_test.go`

- [ ] **Step 1: Write failing test for thread fallback**

```go
func TestReplyMessage_ThreadFallback(t *testing.T) {
	// This test verifies the fallback path exists.
	// A full integration test would mock the Feishu API client.
	// For now, verify the error code constants exist.
	assert.Equal(t, 230071, errCodeThreadNotSupported)
	assert.Equal(t, 230072, errCodeAggregatedMsgThread)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./platform/feishu/ -run TestReplyMessage_ThreadFallback`
Expected: FAIL — `errCodeThreadNotSupported` undefined

- [ ] **Step 3: Add error code constants**

Before `replyMessage` function (around line 3537), add:

```go
const (
	errCodeThreadNotSupported  = 230071
	errCodeAggregatedMsgThread = 230072
)
```

- [ ] **Step 4: Add fallback logic to replyMessage**

Change `replyMessage` (lines 3537-3554) from:
```go
func (p *Platform) replyMessage(ctx context.Context, rc replyContext, msgType, content string) error {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(rc.messageID).
		Body(p.buildReplyMessageReqBody(rc, msgType, content)).
		Build()
	return p.withTransientRetry(ctx, "reply", func() error {
		return p.withFreshTenantAccessTokenRetry(ctx, "reply", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			resp, err := client.Im.Message.Reply(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: reply api call: %w", p.tag(), err)
			}
			if !resp.Success() {
				return fmt.Errorf("%s: reply failed code=%d msg=%s", p.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
}
```
To:
```go
func (p *Platform) replyMessage(ctx context.Context, rc replyContext, msgType, content string) error {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(rc.messageID).
		Body(p.buildReplyMessageReqBody(rc, msgType, content)).
		Build()
	return p.withTransientRetry(ctx, "reply", func() error {
		return p.withFreshTenantAccessTokenRetry(ctx, "reply", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
			resp, err := client.Im.Message.Reply(ctx, req, options...)
			if err != nil {
				return fmt.Errorf("%s: reply api call: %w", p.tag(), err)
			}
			if !resp.Success() {
				if rc.forceThreadReply && (resp.Code == errCodeThreadNotSupported || resp.Code == errCodeAggregatedMsgThread) {
					slog.Warn("subscription: reply_in_thread not supported, falling back to normal reply",
						"chat_id", rc.chatID, "message_id", rc.messageID, "code", resp.Code)
					fallbackBody := larkim.NewReplyMessageReqBodyBuilder().
						MsgType(msgType).Content(content).Build()
					fallbackReq := larkim.NewReplyMessageReqBuilder().
						MessageId(rc.messageID).Body(fallbackBody).Build()
					return p.withFreshTenantAccessTokenRetry(ctx, "reply-fallback", func(client *lark.Client, options ...larkcore.RequestOptionFunc) error {
						fallbackResp, fallbackErr := client.Im.Message.Reply(ctx, fallbackReq, options...)
						if fallbackErr != nil {
							return fmt.Errorf("%s: reply fallback api call: %w", p.tag(), fallbackErr)
						}
						if !fallbackResp.Success() {
							return fmt.Errorf("%s: reply fallback failed code=%d msg=%s", p.tag(), fallbackResp.Code, fallbackResp.Msg)
						}
						return nil
					})
				}
				return fmt.Errorf("%s: reply failed code=%d msg=%s", p.tag(), resp.Code, resp.Msg)
			}
			return nil
		})
	})
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -race -v ./platform/feishu/ -run TestReplyMessage_ThreadFallback`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go test -race ./...`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add platform/feishu/feishu.go
git commit -m "feat: add reply_in_thread fallback for error codes 230071/230072"
```

---

### Task 5: Fix makeSessionKey to route thread messages <!-- covers: S2, S17 -->

**Files:**
- Modify: `platform/feishu/feishu.go:3477-3491` (makeSessionKey)
- Test: `platform/feishu/feishu_test.go`

- [ ] **Step 1: Write failing test for makeSessionKey with root_id**

```go
func TestMakeSessionKey_RootIDRouting(t *testing.T) {
	p := &Platform{threadIsolation: false}
	msg := &larkim.EventMessage{
		RootId:    larkdp.PtrStr("om_rootMsgID123"),
		ChatId:    larkdp.PtrStr("oc_chatID"),
		ChatType:  larkdp.PtrStr("group"),
		MessageId: larkdp.PtrStr("om_childMsgID"),
	}
	key := p.makeSessionKey(msg, "oc_chatID", "ou_userID")
	assert.Equal(t, "feishu:oc_chatID:root:om_rootMsgID123", key)
}

func TestMakeSessionKey_NoRootID_NoThreadIsolation(t *testing.T) {
	p := &Platform{threadIsolation: false}
	msg := &larkim.EventMessage{
		ChatId:    larkdp.PtrStr("oc_chatID"),
		ChatType:  larkdp.PtrStr("group"),
		MessageId: larkdp.PtrStr("om_msgID"),
	}
	key := p.makeSessionKey(msg, "oc_chatID", "ou_userID")
	assert.Equal(t, "feishu:oc_chatID:ou_userID", key)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./platform/feishu/ -run TestMakeSessionKey_RootIDRouting`
Expected: FAIL — returns `feishu:oc_chatID:ou_userID` instead of `feishu:oc_chatID:root:om_rootMsgID123`

- [ ] **Step 3: Fix makeSessionKey**

Change lines 3477-3491 from:
```go
func (p *Platform) makeSessionKey(msg *larkim.EventMessage, chatID, userID string) string {
	if p.threadIsolation && msg != nil && stringValue(msg.ChatType) == "group" {
		rootID := stringValue(msg.RootId)
		if rootID == "" {
			rootID = stringValue(msg.MessageId)
		}
		if rootID != "" {
			return fmt.Sprintf("%s:%s:root:%s", p.tag(), chatID, rootID)
		}
	}
	if p.shareSessionInChannel {
		return fmt.Sprintf("%s:%s", p.tag(), chatID)
	}
	return fmt.Sprintf("%s:%s:%s", p.tag(), chatID, userID)
}
```
To:
```go
func (p *Platform) makeSessionKey(msg *larkim.EventMessage, chatID, userID string) string {
	rootID := stringValue(msg.RootId)
	if rootID != "" {
		return fmt.Sprintf("%s:%s:root:%s", p.tag(), chatID, rootID)
	}
	if p.threadIsolation && msg != nil && stringValue(msg.ChatType) == "group" {
		rootID = stringValue(msg.MessageId)
		if rootID != "" {
			return fmt.Sprintf("%s:%s:root:%s", p.tag(), chatID, rootID)
		}
	}
	if p.shareSessionInChannel {
		return fmt.Sprintf("%s:%s", p.tag(), chatID)
	}
	return fmt.Sprintf("%s:%s:%s", p.tag(), chatID, userID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -v ./platform/feishu/ -run TestMakeSessionKey`
Expected: all PASS

- [ ] **Step 5: Run full test suite**

Run: `go test -race ./...`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add platform/feishu/feishu.go
git commit -m "fix: makeSessionKey routes thread messages regardless of threadIsolation"
```

---

### Task 6: Fix filterMessages — exclude humans/self-bot, regex support, caching <!-- covers: S3, S4, S5, S6, S8, S14 -->

**Files:**
- Modify: `core/subscription.go:19-41` (Subscription struct — add filterRe/excludeFilterRe)
- Modify: `core/subscription.go:365-385` (filterMessages)
- Add new method: `compileFilters()` on Subscription
- Modify: `core/subscription.go:404-412` (SubscriptionManager — add botIDProvider)
- Modify: `core/subscription_cmd.go:45-126` (cmdSubscribeAdd — validate regex)
- Modify: `core/engine.go:16324` (filterMessages call site)
- Test: `core/subscription_test.go`

- [ ] **Step 1: Write failing tests for new filterMessages behavior**

```go
func TestFilterMessages_ExcludesHumanMessages(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "alert: high CPU", IsBot: false, UserID: "ou_human1"},
		{MessageID: "m2", Content: "warning: disk full", IsBot: true, UserID: "ou_bot2"},
		{MessageID: "m3", Content: "error: timeout", IsBot: false, UserID: "ou_human3"},
	}
	matched, err := filterMessages(msgs, nil, nil, nil, "")
	require.NoError(t, err)
	assert.Len(t, matched, 1)
	assert.Equal(t, "m2", matched[0].MessageID)
}

func TestFilterMessages_ExcludesSelfBot(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "alert from other bot", IsBot: true, UserID: "ou_otherBot"},
		{MessageID: "m2", Content: "my own reply", IsBot: true, UserID: "ou_selfBotID"},
	}
	matched, err := filterMessages(msgs, nil, nil, nil, "ou_selfBotID")
	require.NoError(t, err)
	assert.Len(t, matched, 1)
	assert.Equal(t, "m1", matched[0].MessageID)
}

func TestFilterMessages_RegexMatching(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "alert: CPU 95%", IsBot: true, UserID: "ou_bot1"},
		{MessageID: "m2", Content: "info: routine check", IsBot: true, UserID: "ou_bot1"},
		{MessageID: "m3", Content: "warning: disk 88%", IsBot: true, UserID: "ou_bot1"},
	}
	filterRe := regexp.MustCompile("alert|warning|error")
	excludeRe := regexp.MustCompile("info|debug")
	matched, err := filterMessages(msgs, filterRe, excludeRe, nil, "")
	require.NoError(t, err)
	assert.Len(t, matched, 2)
	assert.Equal(t, "m1", matched[0].MessageID)
	assert.Equal(t, "m3", matched[1].MessageID)
}

func TestSubscription_CompileFilters_InvalidRegex(t *testing.T) {
	sub := &Subscription{Filter: "alert["}
	err := sub.compileFilters()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter regex")
}

func TestSubscription_CompileFilters_Cached(t *testing.T) {
	sub := &Subscription{Filter: "alert|warning"}
	err := sub.compileFilters()
	require.NoError(t, err)
	assert.NotNil(t, sub.filterRe)
	assert.Equal(t, "alert|warning", sub.filterRe.String())
}
```

Add `"regexp"` and `"github.com/stretchr/testify/assert"` and `"github.com/stretchr/testify/require"` to test imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -race -v ./core/ -run "TestFilterMessages_Excludes|TestFilterMessages_Regex|TestSubscription_CompileFilters"`
Expected: FAIL — `filterMessages` signature mismatch, `compileFilters` undefined

- [ ] **Step 3: Add filterRe/excludeFilterRe fields to Subscription struct**

In `core/subscription.go`, add after `ExcludeFilter` field (line 28):

```go
	// Cached compiled regex (not serialized)
	filterRe      *regexp.Regexp `json:"-"`
	excludeFilterRe *regexp.Regexp `json:"-"`
```

Add `"regexp"` to imports.

- [ ] **Step 4: Add compileFilters method**

After the `BuildPrompt` method (line 390), add:

```go
func (s *Subscription) compileFilters() error {
	if s.Filter != "" && s.Filter != "-" {
		re, err := regexp.Compile(s.Filter)
		if err != nil {
			return fmt.Errorf("invalid filter regex %q: %w", s.Filter, err)
		}
		s.filterRe = re
	}
	if s.ExcludeFilter != "" && s.ExcludeFilter != "-" {
		re, err := regexp.Compile(s.ExcludeFilter)
		if err != nil {
			return fmt.Errorf("invalid exclude_filter regex %q: %w", s.ExcludeFilter, err)
		}
		s.excludeFilterRe = re
	}
	return nil
}
```

- [ ] **Step 5: Rewrite filterMessages**

Replace lines 365-385 with:

```go
func filterMessages(msgs []ScannedMessage, filterRe, excludeRe *regexp.Regexp, processedIDs []string, botID string) ([]ScannedMessage, error) {
	processedSet := make(map[string]struct{}, len(processedIDs))
	for _, id := range processedIDs {
		processedSet[id] = struct{}{}
	}

	var matched []ScannedMessage
	for _, msg := range msgs {
		if _, seen := processedSet[msg.MessageID]; seen {
			continue
		}
		if !msg.IsBot {
			continue
		}
		if botID != "" && msg.UserID == botID {
			continue
		}
		if filterRe != nil && !filterRe.MatchString(msg.Content) {
			continue
		}
		if excludeRe != nil && excludeRe.MatchString(msg.Content) {
			continue
		}
		matched = append(matched, msg)
	}
	return matched, nil
}
```

Remove `"strings"` from imports if no longer used in this file (check first — other functions may use it).

- [ ] **Step 6: Add botIDProvider to SubscriptionManager**

In `core/subscription.go`, change SubscriptionManager struct from:
```go
type SubscriptionManager struct {
	store   *SubscriptionStore
	cron    *cron.Cron
	engines map[string]*Engine
	mu      sync.RWMutex
	entries map[string]cron.EntryID
	running sync.Map
	logsDir string
}
```
To:
```go
type SubscriptionManager struct {
	store         *SubscriptionStore
	cron          *cron.Cron
	engines       map[string]*Engine
	mu            sync.RWMutex
	entries       map[string]cron.EntryID
	running       sync.Map
	logsDir       string
	botIDProvider BotIDProvider
}
```

Change `NewSubscriptionManager` (line 415) to accept `botIDProvider BotIDProvider`:
```go
func NewSubscriptionManager(store *SubscriptionStore, dataDir string, botIDProvider BotIDProvider) *SubscriptionManager {
	return &SubscriptionManager{
		store:         store,
		cron:          cron.New(),
		engines:       make(map[string]*Engine),
		entries:       make(map[string]cron.EntryID),
		logsDir:       filepath.Join(dataDir, "subscriptions", "logs"),
		botIDProvider: botIDProvider,
	}
}
```

Update all callers of `NewSubscriptionManager` — search with:
```bash
grep -rn 'NewSubscriptionManager' --include='*.go' .
```
Add `nil` as the third argument where `botIDProvider` is not yet available. The Feishu platform can pass itself (it implements `BotID()`).

- [ ] **Step 7: Update filterMessages call site in engine.go**

In `core/engine.go:16324`, change from:
```go
matched := filterMessages(allMsgs, sub.Filter, sub.ExcludeFilter, sub.ProcessedIDs)
```
To:
```go
botID := ""
if e.subscriptionManager != nil && e.subscriptionManager.botIDProvider != nil {
	botID = e.subscriptionManager.botIDProvider.BotID()
}
matched, err := filterMessages(allMsgs, snapshot.filterRe, snapshot.excludeFilterRe, snapshot.ProcessedIDs, botID)
if err != nil {
	return fmt.Errorf("subscription: filter messages: %w", err)
}
```

Also need to call `compileFilters` on snapshot before use. After `snapshot.ProcessedIDs = append([]string(nil), sub.ProcessedIDs...)` (around line 16287), add:
```go
if err := snapshot.compileFilters(); err != nil {
	slog.Error("subscription: filter compilation failed", "id", subID, "error", err)
	sm.store.MarkRun(subID, err.Error(), true)
	sm.unscheduleIfDisabled(subID)
	return
}
```

Wait — this is in `executeScan()` in SubscriptionManager, not in `ExecuteSubscriptionScan`. The snapshot is created in `executeScan()` (line 16285-16286). The `filterMessages` call is in `ExecuteSubscriptionScan`. Need to verify the exact call chain.

Actually, looking at the code: `executeScan` calls `engine.ExecuteSubscriptionScan(&snapshot)`. So `compileFilters()` should be called on the snapshot before passing it. Add it after the snapshot creation in `executeScan()`:

```go
snapshot := *sub
snapshot.ProcessedIDs = append([]string(nil), sub.ProcessedIDs...)
if err := snapshot.compileFilters(); err != nil {
    slog.Error("subscription: filter compilation failed", "id", subID, "error", err)
    sm.store.MarkRun(subID, err.Error(), true)
    sm.unscheduleIfDisabled(subID)
    return
}
```

And in `ExecuteSubscriptionScan`, the `filterMessages` call becomes:
```go
botID := ""
matched, filterErr := filterMessages(allMsgs, sub.filterRe, sub.excludeFilterRe, sub.ProcessedIDs, botID)
if filterErr != nil {
    return fmt.Errorf("subscription: filter messages: %w", filterErr)
}
```

The botID should be obtained from SubscriptionManager and passed into ExecuteSubscriptionScan, or obtained via a field on Engine. Since Engine already has `subscriptionManager`, and SubscriptionManager has `botIDProvider`, the botID can be obtained through the chain.

Actually, the simplest approach: pass `botID` as a parameter to `ExecuteSubscriptionScan`. But that changes the signature. Alternative: have `executeScan` set `botID` on the snapshot struct. But Subscription doesn't have a botID field.

Simplest correct approach: have `executeScan` obtain botID and pass it alongside the snapshot. Add a `botID` field to a new context struct, or just modify `ExecuteSubscriptionScan` to accept an optional botID. Let's keep it simple — add botID to Engine's method chain:

In `executeScan()` in subscription.go, after `compileFilters`:
```go
botID := ""
if sm.botIDProvider != nil {
    botID = sm.botIDProvider.BotID()
}
```

Then change `ExecuteSubscriptionScan` signature to accept botID:
```go
func (e *Engine) ExecuteSubscriptionScan(sub *Subscription, botID string) error
```

And in `executeScan`, change the call:
```go
done <- engine.ExecuteSubscriptionScan(&snapshot, botID)
```

And in `ExecuteSubscriptionScan`, use:
```go
matched, filterErr := filterMessages(allMsgs, sub.filterRe, sub.excludeFilterRe, sub.ProcessedIDs, botID)
```

Update all test callers of `ExecuteSubscriptionScan` to pass `""` as botID.

- [ ] **Step 8: Fix existing tests that use old filterMessages signature**

Update `TestSubscriptionFilter`, `TestSubscriptionFilterEmpty`, `TestSubscriptionFilterDashWildcard`, `TestSubscriptionFilter_ProcessedIDsDedup` in `core/subscription_test.go` to use new signature. Example for `TestSubscriptionFilterEmpty`:

```go
func TestSubscriptionFilterEmpty(t *testing.T) {
	msgs := []ScannedMessage{
		{MessageID: "m1", Content: "消息1", IsBot: false},
		{MessageID: "m2", Content: "消息2", IsBot: true},
	}
	matched, err := filterMessages(msgs, nil, nil, nil, "")
	require.NoError(t, err)
	assert.Len(t, matched, 1)
	assert.Equal(t, "m2", matched[0].MessageID)
}
```

- [ ] **Step 9: Run tests**

Run: `go test -race -v ./core/ -run "TestFilterMessages|TestSubscription_CompileFilters|TestSubscriptionFilter"`
Expected: all PASS

- [ ] **Step 10: Fix compilation of all callers and run full test suite**

Run: `go build ./... && go test -race ./...`
Expected: build succeeds, all tests pass

- [ ] **Step 11: Commit**

```bash
git add core/subscription.go core/subscription_cmd.go core/engine.go core/subscription_test.go
git commit -m "fix: filterMessages excludes humans/self-bot, supports regex with caching"
```

---

### Task 7: Add regex validation in cmdSubscribeAdd and i18n key <!-- covers: S6, S7 -->

**Files:**
- Modify: `core/subscription_cmd.go:99-126`
- Modify: `core/i18n.go` (add MsgSubInvalidFilter + translations)
- Test: `core/subscription_cmd_test.go`

- [ ] **Step 1: Write failing test for invalid regex rejection**

```go
func TestCmdSubscribe_AddInvalidRegex(t *testing.T) {
	e, p, msg := newSubscribeTestEngine(t)
	msg.Args = []string{"alert["}
	e.cmdSubscribeAdd(p, msg, []string{"alert["})
	// Should reply with error about invalid filter
	assert.Contains(t, p.lastReply, "invalid filter")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./core/ -run TestCmdSubscribe_AddInvalidRegex`
Expected: FAIL — no "invalid filter" in reply

- [ ] **Step 3: Add MsgSubInvalidFilter i18n key**

In `core/i18n.go`, add constant after line 666:
```go
MsgSubInvalidFilter MsgKey = "sub_invalid_filter"
```

Add translations for all 5 languages in the MsgSubInvalidFilter section:
- English: `"Invalid filter expression: %s"`
- Chinese: `"过滤表达式无效: %s"`
- Traditional Chinese: `"過濾表達式無效: %s"`
- Japanese: `"フィルター表現が無効です: %s"`
- Spanish: `"Expresión de filtro inválida: %s"`

- [ ] **Step 4: Add regex validation in cmdSubscribeAdd**

In `core/subscription_cmd.go`, after creating the `sub` struct (line 113) and before `AddSubscription` (line 115), add:

```go
if err := sub.compileFilters(); err != nil {
	e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubInvalidFilter), err))
	return
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -race -v ./core/ -run TestCmdSubscribe_AddInvalidRegex`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add core/subscription_cmd.go core/i18n.go
git commit -m "feat: validate filter regex at subscription creation time"
```

---

### Task 8: Wire effectiveSessionKey in ExecuteSubscriptionScan <!-- covers: S9, S11, S12 -->

**Files:**
- Modify: `core/engine.go:16343-16414` (ExecuteSubscriptionScan loop)

- [ ] **Step 1: Write failing test for effectiveSessionKey**

```go
func TestExecuteSubscriptionScan_EffectiveSessionKey(t *testing.T) {
	// Use existing test infrastructure
	// Verify that when BuildThreadReplyCtx returns a threadSessionKey,
	// it is used for session creation and iKey
	// This test uses the existing stubScannerPlatform
}
```

Since the existing test infrastructure in `core/subscription_test.go` already has `stubScannerPlatform`, update it to return thread session key from `BuildThreadReplyCtx`:

In the existing `stubScannerPlatform` (line 1147), update `BuildThreadReplyCtx` to return 3 values:
```go
func (s *stubScannerPlatform) BuildThreadReplyCtx(sessionKey, chatID, messageID string) (any, string, error) {
	return replyContext{chatID: chatID, messageID: messageID, sessionKey: sessionKey}, "", nil
}
```

Then write a test that verifies the effectiveSessionKey is used. This requires the stub to return a non-empty threadSessionKey. Create a new `stubThreadScannerPlatform` for this purpose.

- [ ] **Step 2: Update ExecuteSubscriptionScan to use effectiveSessionKey**

In `core/engine.go`, change the loop in `ExecuteSubscriptionScan` (lines 16343-16414). Replace the BuildThreadReplyCtx call and session creation:

From:
```go
var replyCtx any
if hasThreadBuilder {
    if rc, err := threadBuilder.BuildThreadReplyCtx(sub.SessionKey, sub.ChatID, msg.MessageID); err == nil {
        replyCtx = rc
    }
}
if replyCtx == nil {
    if reconstructor, ok := targetPlatform.(ReplyContextReconstructor); ok {
        if rc, err := reconstructor.ReconstructReplyCtx(sub.SessionKey); err == nil {
            replyCtx = rc
        }
    }
}
```
To:
```go
var replyCtx any
var threadSessionKey string
if hasThreadBuilder {
    if rc, tsk, err := threadBuilder.BuildThreadReplyCtx(sub.SessionKey, sub.ChatID, msg.MessageID); err == nil {
        replyCtx = rc
        threadSessionKey = tsk
    }
}
if replyCtx == nil {
    if reconstructor, ok := targetPlatform.(ReplyContextReconstructor); ok {
        if rc, err := reconstructor.ReconstructReplyCtx(sub.SessionKey); err == nil {
            replyCtx = rc
        }
    }
}

effectiveSessionKey := sub.SessionKey
if threadSessionKey != "" {
    effectiveSessionKey = threadSessionKey
}
```

Change concurrency limit check from `sub.SessionKey` to `effectiveSessionKey`:
```go
activeCount := len(sessions.ListSessions(effectiveSessionKey))
```

Change session creation:
```go
session := sessions.NewSideSession(effectiveSessionKey, "subscription-"+sub.ID)
```

Change iKey:
```go
iKey := fmt.Sprintf("%s#subscription:%s", effectiveSessionKey, session.ID)
```

Change the goroutine's sessionKey parameter from `sub.SessionKey` to `effectiveSessionKey`:
```go
}(targetPlatform, prompt, replyCtx, session, iKey, effectiveSessionKey)
```

- [ ] **Step 3: Add diagnostic log**

After filterMessages, add:
```go
slog.Info("subscription: filter results",
    "subscription_id", sub.ID,
    "total_scanned", len(allMsgs),
    "filter_matched", len(matched),
    "filter", sub.Filter,
    "exclude_filter", sub.ExcludeFilter)
```

- [ ] **Step 4: Run full test suite**

Run: `go test -race ./...`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add core/engine.go core/subscription_test.go
git commit -m "feat: use effectiveSessionKey for session creation and iKey alignment"
```

---

### Task 9: Remove /sub alias, add /subscribe to /help <!-- covers: S10, S11, S13, S14 -->

**Files:**
- Modify: `core/engine.go:5946` (command alias)
- Modify: `core/i18n.go:997-1084` (MsgHelp content — all 5 languages)
- Test: `core/engine_test.go` or `core/subscription_cmd_test.go`

- [ ] **Step 1: Write failing test for /sub removal**

```go
func TestCommandAlias_SubRemoved(t *testing.T) {
	e := newTestEngineWithBuiltinCommands()
	aliases := e.findCommandAliases("subscribe")
	for _, a := range aliases {
		assert.NotEqual(t, "sub", a)
	}
}
```

If `findCommandAliases` doesn't exist as a test helper, test it differently — send `/sub` and verify "unknown command" response.

Actually, check how the existing `TestCmdSubscribe_RemoveAliases` works (line 426) and adapt.

- [ ] **Step 2: Remove /sub alias**

In `core/engine.go:5946`, change from:
```go
{[]string{"subscribe", "sub"}, "subscribe"},
```
To:
```go
{[]string{"subscribe"}, "subscribe"},
```

- [ ] **Step 3: Add /subscribe to /help in all 5 languages**

In `core/i18n.go`, add `/subscribe` section to MsgHelp content. Insert before `/bind` line in each language.

English (before line 1026 `"/bind`"):
```
"/subscribe [add|list|del|enable|disable]\n  Manage subscriptions (auto-scan group messages)\n\n" +
```

Chinese (before the Chinese `/bind` line):
```
"/subscribe [add|list|del|enable|disable]\n  管理订阅（自动扫描群消息）\n\n" +
```

Traditional Chinese:
```
"/subscribe [add|list|del|enable|disable]\n  管理訂閱（自動掃描群訊息）\n\n" +
```

Japanese:
```
"/subscribe [add|list|del|enable|disable]\n  サブスクリプション管理（グループメッセージの自動スキャン）\n\n" +
```

Spanish:
```
"/subscribe [add|list|del|enable|disable]\n  Gestionar suscripciones (escaneo automático de mensajes de grupo)\n\n" +
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./core/ -run "TestCmdSubscribe|TestCommandAlias"`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add core/engine.go core/i18n.go
git commit -m "feat: remove /sub alias, add /subscribe to /help"
```

---

### Task 10: Update WebUI filter placeholders <!-- covers: S12, S15 -->

**Files:**
- Modify: `web/src/i18n/locales/en.json:144,146`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/es.json`

- [ ] **Step 1: Update en.json placeholders**

Change:
```json
"filterPlaceholder": "from:user1 keyword",
"excludeFilterPlaceholder": "from:bot keyword",
```
To:
```json
"filterPlaceholder": "Regex: alert|warning|error",
"excludeFilterPlaceholder": "Regex: info|debug",
```

- [ ] **Step 2: Update zh.json placeholders**

Change to:
```json
"filterPlaceholder": "正则表达式: alert|warning|error",
"excludeFilterPlaceholder": "正则表达式: info|debug",
```

- [ ] **Step 3: Update zh-TW.json placeholders**

Change to:
```json
"filterPlaceholder": "正規表示式: alert|warning|error",
"excludeFilterPlaceholder": "正規表示式: info|debug",
```

- [ ] **Step 4: Update ja.json placeholders**

Change to:
```json
"filterPlaceholder": "正規表現: alert|warning|error",
"excludeFilterPlaceholder": "正規表現: info|debug",
```

- [ ] **Step 5: Update es.json placeholders**

Change to:
```json
"filterPlaceholder": "Regex: alert|warning|error",
"excludeFilterPlaceholder": "Regex: info|debug",
```

- [ ] **Step 6: Verify WebUI builds**

Run: `cd web && npm run build`
Expected: build succeeds

- [ ] **Step 7: Commit**

```bash
git add web/src/i18n/locales/
git commit -m "feat: update WebUI filter placeholders to show regex examples"
```

---

### Task 11: Implement BotID() on Feishu Platform <!-- covers: S4, S18 -->

**Files:**
- Modify: `platform/feishu/feishu.go` (add BotID method)
- Modify: `cmd/cc-connect/daemon.go` or wherever SubscriptionManager is created (wire botIDProvider)

- [ ] **Step 1: Add BotID() method to Feishu Platform**

In `platform/feishu/feishu.go`, add:

```go
func (p *Platform) BotID() string {
	return p.getBotOpenID()
}
```

- [ ] **Step 2: Wire botIDProvider when creating SubscriptionManager**

Search for where `NewSubscriptionManager` is called:
```bash
grep -rn 'NewSubscriptionManager' --include='*.go' .
```

Pass the Feishu Platform as `botIDProvider` when it's available. If multiple platforms exist, pass the first one that implements `BotIDProvider`, or pass nil if none.

- [ ] **Step 3: Run full test suite**

Run: `go test -race ./...`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add platform/feishu/feishu.go cmd/cc-connect/
git commit -m "feat: implement BotID() on Feishu Platform and wire botIDProvider"
```

---

### Task 12: Compile filters on store load and update <!-- covers: S8, S14 -->

**Files:**
- Modify: `core/subscription.go` (load + update paths)

- [ ] **Step 1: Add compileFilters call in store load**

In `SubscriptionStore.load()`, after `json.Unmarshal`, add:
```go
for _, sub := range s.subs {
    if err := sub.compileFilters(); err != nil {
        slog.Warn("subscription: filter compilation failed on load", "id", sub.ID, "error", err)
    }
}
```

- [ ] **Step 2: Add compileFilters call in updateSubscriptionField**

When `filter` or `exclude_filter` field is updated via `Update()`, recompile. Add after the field update loop in `Update()` method:

```go
if _, hasFilter := fields["filter"]; hasFilter {
    if err := sub.compileFilters(); err != nil {
        return fmt.Errorf("recompile filters: %w", err)
    }
}
if _, hasExclude := fields["exclude_filter"]; hasExclude {
    if err := sub.compileFilters(); err != nil {
        return fmt.Errorf("recompile filters: %w", err)
    }
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./core/ -run TestSubscription`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add core/subscription.go
git commit -m "feat: compile filters on store load and field update"
```

---

### Task 13: Final integration test and cleanup <!-- covers: S1 -->

**Files:**
- Test: `core/subscription_test.go`

- [ ] **Step 1: Update existing integration test to verify thread reply context**

In `TestIntegration_SubscriptionFullScanCycle` (line 1627), verify that the subscription scan creates sessions with thread session keys when `BuildThreadReplyCtx` returns one.

- [ ] **Step 2: Run full test suite with race detector**

Run: `go test -race ./...`
Expected: all PASS

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit any test updates**

```bash
git add core/subscription_test.go
git commit -m "test: update integration tests for thread reply context"
```
