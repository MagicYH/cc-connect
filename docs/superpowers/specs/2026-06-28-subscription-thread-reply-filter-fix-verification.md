# 订阅回复 Thread 化与过滤规则修复 Verification

> Source spec: ./2026-06-28-subscription-thread-reply-filter-fix-design.md
> Used by: superpowers:writing-plans (TDD coverage) and post-implementation smoke testing.

## Environment & Access

| Item | Value | How to Obtain |
|---|---|---|
| Target environment | local dev (go test + manual Feishu group testing) | `make build && cc-connect daemon restart` |
| Go version | go1.22+ | `go version` |
| Feishu test group | `${FEISHU_TEST_CHAT_ID}` | `grep 'chat_id' config.toml \| head -1 \| awk '{print $3}' \| tr -d '"'` — use the first group chat_id from your config |
| Bot app_id | `${FEISHU_APP_ID}` | `grep 'app_id' config.toml \| awk '{print $3}' \| tr -d '"'` |
| Bot open_id | `${FEISHU_BOT_OPEN_ID}` | `cc-connect daemon start 2>&1 \| grep 'bot identified' \| grep -oP 'open_id=\K[^ ]+'` |
| Feishu tenant token | `${FEISHU_TENANT_TOKEN}` | `curl -s -X POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal -H "Content-Type: application/json" -d "{\"app_id\":\"${FEISHU_APP_ID}\",\"app_secret\":\"${FEISHU_APP_SECRET}\"}" \| jq -r '.tenant_access_token'` |
| Feishu API base | https://open.feishu.cn/open-apis | Default |
| WebUI | http://localhost:8787 (default) | `make build && cc-connect daemon start` |
| Test subscription ID | `${SUB_ID}` (created per scenario) | System-generated via API, extracted with `jq -r '.id'` |
| Required env vars | `FEISHU_APP_ID`, `FEISHU_APP_SECRET` | Feishu developer console → App Credentials |

## Public Operations

### Create Test Subscription

Purpose: Create a subscription in the test group for verification scenarios.

```bash
# Via API (requires running daemon with WebUI)
# FEISHU_TEST_CHAT_ID must be set per Environment & Access table
curl -s -X POST http://localhost:8787/api/subscriptions \
  -H "Content-Type: application/json" \
  -d '{
    "project": "default",
    "chat_id": "'"${FEISHU_TEST_CHAT_ID}"'",
    "filter": "alert|warning|error",
    "exclude_filter": "info|debug",
    "prompt": "investigate: {{content}}",
    "interval": "1m",
    "enabled": true
  }' | jq .
```

### Delete All Test Subscriptions

Purpose: Clean up after scenario execution.

```bash
# List and delete all
SUB_IDS=$(curl -s http://localhost:8787/api/subscriptions | jq -r '.subscriptions[].id')
for id in $SUB_IDS; do
  curl -s -X DELETE "http://localhost:8787/api/subscriptions/$id"
done
```

### Run Unit Tests

Purpose: Execute the Go test suite for subscription-related packages.

```bash
go test -race -v ./core/ -run TestSubscription
go test -race -v ./platform/feishu/ -run TestSubscription
```

### Check Feishu Thread Reply

Purpose: Verify that a message reply created a thread in Feishu.

```bash
# Requires manual verification in Feishu group — check that the reply
# appears nested under the parent message (threaded) rather than in the
# main chat flow.
```

## Acceptance Criteria

- [ ] AC-1: Subscription replies use `reply_in_thread=true`, creating a thread under the scanned message (covers spec §2, §3)
- [ ] AC-2: Thread follow-up messages route to a thread session with key format `feishu:oc_chatID:root:om_messageID` (covers spec §5)
- [ ] AC-3: `filterMessages` excludes human messages (`IsBot=false`) (covers spec §7)
- [ ] AC-4: `filterMessages` excludes self-bot messages (`msg.UserID == botID`) (covers spec §7)
- [ ] AC-5: `filterMessages` keeps other bot messages (`IsBot=true` and `UserID != botID`) (covers spec §7)
- [ ] AC-6: filter and exclude_filter support regex matching (covers spec §7)
- [ ] AC-7: Invalid regex is rejected at subscription creation time (covers spec §7)
- [ ] AC-8: Regex is compiled once and cached on Subscription struct (covers spec §7)
- [ ] AC-9: `shouldReplyInThread` returns true when session key is a thread key, regardless of `threadIsolation` config (covers spec §3)
- [ ] AC-10: `reply_in_thread` fallback on error codes 230071/230072 sends a normal reply without retry (covers spec §3)
- [ ] AC-11: Concurrency limit counts use `effectiveSessionKey`, aligned with session creation (covers spec §4)
- [ ] AC-12: `iKey` uses `effectiveSessionKey` for injection lookup alignment (covers spec §4)
- [ ] AC-13: `/sub` alias is removed, only `/subscribe` works (covers spec §10)
- [ ] AC-14: `/subscribe` appears in `/help` output (covers spec §11)
- [ ] AC-15: WebUI filter/exclude_filter placeholders show regex examples (covers spec §12)
- [ ] AC-16: `BuildThreadReplyCtx` returns thread session key as second return value (covers spec §8)
- [ ] AC-17: `makeSessionKey` routes messages with `root_id` to thread session regardless of `threadIsolation` (covers spec §5)
- [ ] AC-18: BotIDProvider interface is implemented by Feishu platform (covers spec §7)
- [ ] AC-19: Feishu API error codes 230071/230072 are defined as named constants (covers spec §3)

## Test Scenarios

### Scenario S1: Subscription reply creates a thread under scanned message

**Verifies:** AC-1, AC-2, AC-16

**Execution:** AI-autonomous (unit test) + Human-assisted (Step 5 requires visual verification in Feishu group)

**Preconditions:**
- cc-connect daemon is running with a Feishu test group configured
- Bot is added to the test group
- No existing subscriptions in the test group
- Another bot (or test webhook) can send a message to the test group

**Steps:**
1. Obtain Feishu tenant token and export environment variables:
   ```bash
   export FEISHU_TEST_CHAT_ID=$(grep 'chat_id' config.toml | head -1 | awk '{print $3}' | tr -d '"')
   export FEISHU_APP_ID=$(grep 'app_id' config.toml | head -1 | awk '{print $3}' | tr -d '"')
   export FEISHU_APP_SECRET=$(grep 'app_secret' config.toml | head -1 | awk '{print $3}' | tr -d '"')
   export FEISHU_TENANT_TOKEN=$(curl -s -X POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal \
     -H "Content-Type: application/json" \
     -d "{\"app_id\":\"${FEISHU_APP_ID}\",\"app_secret\":\"${FEISHU_APP_SECRET}\"}" \
     | jq -r '.tenant_access_token')
   [ -n "$FEISHU_TENANT_TOKEN" ] && [ "$FEISHU_TENANT_TOKEN" != "null" ] || { echo "FAIL: could not obtain tenant token"; exit 1; }
   ```
   → If token is empty or "null", **stop** — check FEISHU_APP_ID and FEISHU_APP_SECRET.
2. Create a subscription for the test group:
   ```bash
   SUB=$(curl -s -X POST http://localhost:8787/api/subscriptions \
     -H "Content-Type: application/json" \
     -d '{
       "project": "default",
       "chat_id": "'"${FEISHU_TEST_CHAT_ID}"'",
       "filter": "",
       "prompt": "echo: received subscription message",
       "interval": "1m",
       "enabled": true
     }')
   SUB_ID=$(echo "$SUB" | jq -r '.id')
   [ -n "$SUB_ID" ] && [ "$SUB_ID" != "null" ] || { echo "FAIL: subscription not created: $SUB"; exit 1; }
   echo "Created subscription: $SUB_ID"
   ```
   → If SUB_ID is empty, **stop** — API may be unreachable or request body invalid.
3. Send a bot message to the test group (via Feishu API):
   ```bash
   curl -s -X POST "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id" \
     -H "Authorization: Bearer ${FEISHU_TENANT_TOKEN}" \
     -H "Content-Type: application/json" \
     -d '{
       "receive_id": "'"${FEISHU_TEST_CHAT_ID}"'",
       "msg_type": "text",
       "content": "{\"text\":\"alert: CPU usage over 90%\"}"
     }' | jq .
   ```
   → If API returns error, **stop** — check tenant token and chat ID.
4. Wait for scan cycle (up to 1 minute based on interval):
   ```bash
   sleep 65
   ```
5. Check daemon logs for thread reply context creation:
   ```bash
   journalctl -u cc-connect --since "2 min ago" 2>/dev/null | grep -i "subscription.*thread" || \
     tail -100 /tmp/cc-connect.log 2>/dev/null | grep -i "subscription.*thread" || \
     echo "WARN: no log access method available"
   ```
   → If no log lines found, **stop** — subscription scan may not have triggered.
6. **Human-assisted step:** Verify in Feishu group that the bot's reply appears as a thread under the alert message (nested, not in main chat flow).

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${FEISHU_TEST_CHAT_ID} | runtime-fetch | extracted from config.toml in Step 1 |
| sub_id | string, UUID | $SUB_ID | system-generated | extracted in Step 2 via jq |
| feishu_tenant_token | string | ${FEISHU_TENANT_TOKEN} | runtime-fetch | obtained in Step 1 via Feishu auth API |

**Expected Results:**
- (must) Daemon log contains "subscription: built thread reply context" with `thread_session_key` matching pattern `feishu:oc_testGroupChatID:root:om_*`
- (must) Daemon log contains "subscription: filter results" showing `filter_matched >= 1`
- (must) Bot reply is sent with `ReplyInThread=true` (visible in debug logs)
- (should) Thread session key follows format `feishu:oc_chatID:root:om_messageID`

**Failure Handling:**
- Step 2 API error: verify FEISHU_TENANT_TOKEN is valid and not expired
- Step 4 no log lines: verify subscription is enabled, scan interval has elapsed, bot message was sent by another bot (not human)

**Cleanup:**
```bash
curl -s -X DELETE "http://localhost:8787/api/subscriptions/$SUB_ID"
```

### Scenario S2: Thread follow-up messages route to thread session

**Verifies:** AC-2, AC-17

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up
- Feishu platform package compiled

**Steps:**
1. Write a unit test that verifies `makeSessionKey` routes messages with `root_id` to thread session:
   ```go
   func TestMakeSessionKey_RootIDRouting(t *testing.T) {
       p := &Platform{threadIsolation: false}
       // Simulate a message inside a thread (has root_id)
       msg := &larkim.EventMessage{
           RootId:    larkdp.PtrStr("om_rootMsgID123"),
           ChatId:    larkdp.PtrStr("oc_chatID"),
           ChatType:  larkdp.PtrStr("group"),
           MessageId: larkdp.PtrStr("om_childMsgID"),
       }
       key := p.makeSessionKey(msg, "oc_chatID", "ou_userID")
       assert.Equal(t, "feishu:oc_chatID:root:om_rootMsgID123", key)
   }
   ```
2. Run the test:
   ```bash
   go test -race -v ./platform/feishu/ -run TestMakeSessionKey_RootIDRouting
   ```
3. Verify the test passes.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| root_id | string | om_rootMsgID123 | concrete | simulated thread root message ID |
| chat_id | string | oc_chatID | concrete | simulated group chat ID |

**Expected Results:**
- (must) `makeSessionKey` returns `feishu:oc_chatID:root:om_rootMsgID123` when `root_id` is set, even with `threadIsolation=false`
- (must) `makeSessionKey` returns `feishu:oc_chatID:ou_userID` when `root_id` is empty and `threadIsolation=false`

**Failure Handling:**
- Test fails: verify that the `root_id` check in `makeSessionKey` is placed before the `threadIsolation` check

### Scenario S3: filterMessages excludes human messages

**Verifies:** AC-3

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test:
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
   ```
2. Run:
   ```bash
   go test -race -v ./core/ -run TestFilterMessages_ExcludesHumanMessages
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| msgs | array | 3 messages (2 human, 1 bot) | concrete | test fixture |

**Expected Results:**
- (must) Only bot messages (`IsBot=true`) pass the filter
- (must) Human messages (`IsBot=false`) are excluded

**Failure Handling:**
- Test fails: verify `!msg.IsBot` check is present in `filterMessages`

### Scenario S4: filterMessages excludes self-bot messages

**Verifies:** AC-4, AC-18

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test:
   ```go
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
   ```
2. Run:
   ```bash
   go test -race -v ./core/ -run TestFilterMessages_ExcludesSelfBot
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| botID | string | ou_selfBotID | concrete | simulated self bot ID |

**Expected Results:**
- (must) Self-bot message (`UserID == botID`) is excluded
- (must) Other bot messages are kept

**Failure Handling:**
- Test fails: verify `msg.UserID == botID` check and that `BotIDProvider` is wired correctly

### Scenario S5: filter and exclude_filter use regex matching

**Verifies:** AC-6, AC-8

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test:
   ```go
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
       assert.Equal(t, []string{"m1", "m3"}, collectIDs(matched))
   }
   ```
2. Run:
   ```bash
   go test -race -v ./core/ -run TestFilterMessages_RegexMatching
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| filter | string, regex | alert\|warning\|error | concrete | standard alternation regex |
| exclude_filter | string, regex | info\|debug | concrete | standard alternation regex |

**Expected Results:**
- (must) Messages matching filter regex are kept
- (must) Messages matching exclude_filter regex are excluded
- (must) Filter and exclude_filter use regex semantics (not substring)

**Failure Handling:**
- Test fails: verify `filterRe.MatchString()` and `excludeRe.MatchString()` are used instead of `strings.Contains`

### Scenario S6: Invalid regex is rejected at subscription creation

**Verifies:** AC-7

**Execution:** AI-autonomous (unit test) + Human-assisted — Step 3 requires sending a slash command in Feishu group

**Preconditions:**
- Go test environment set up
- Daemon running (for API test)

**Steps:**
1. Write a unit test for `compileFilters`:
   ```go
   func TestSubscription_CompileFilters_InvalidRegex(t *testing.T) {
       sub := &Subscription{Filter: "alert["}
       err := sub.compileFilters()
       assert.Error(t, err)
       assert.Contains(t, err.Error(), "invalid filter regex")
   }
   ```
2. Run unit test:
   ```bash
   go test -race -v ./core/ -run TestSubscription_CompileFilters_InvalidRegex
   ```
3. Test via slash command (requires Feishu interaction or mock):
   - Send `/subscribe alert[ test --prompt "test"` in a group
   - Verify bot replies with an error about invalid filter expression

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| invalid_filter | string | alert[ | concrete | unclosed bracket = invalid regex |

**Expected Results:**
- (must) `compileFilters()` returns error for invalid regex
- (must) `cmdSubscribeAdd` rejects the subscription and replies with error message containing "invalid filter" or equivalent i18n string
- (must) Subscription is NOT created in the store

**Failure Handling:**
- Unit test fails: verify `compileFilters()` calls `regexp.Compile()` and returns the error
- Slash command test fails: verify `cmdSubscribeAdd` calls `compileFilters()` before `AddSubscription()`

### Scenario S7: shouldReplyInThread returns true for thread session keys regardless of threadIsolation

**Verifies:** AC-9

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test:
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
   ```
2. Run:
   ```bash
   go test -race -v ./platform/feishu/ -run TestShouldReplyInThread_ThreadSessionKey
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| session_key | string | feishu:oc_chat123:root:om_rootMsg | concrete | thread session key format |
| threadIsolation | boolean | false | concrete | thread isolation disabled |

**Expected Results:**
- (must) `shouldReplyInThread` returns `true` even when `threadIsolation=false`, because the session key has `root:` prefix

**Failure Handling:**
- Test fails: verify `shouldReplyInThread` checks `isThreadSessionKey` before `threadIsolation`

### Scenario S8: reply_in_thread fallback on error code 230071

**Verifies:** AC-10, AC-19

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test that mocks the Feishu API to return error code 230071 on first call, then success on fallback:
   ```go
   func TestReplyMessage_ThreadFallback(t *testing.T) {
       mockAPI := &mockFeishuAPI{
           replyErr: &lark.ErrorResponse{Code: errCodeThreadNotSupported},
       }
       p := &Platform{api: mockAPI}
       rc := replyContext{
           messageID:        "om_msg123",
           chatID:           "oc_chat123",
           sessionKey:       "feishu:oc_chat123:root:om_rootMsg",
           forceThreadReply: true,
       }
       err := p.replyMessage(context.Background(), rc, "fallback text")
       require.NoError(t, err)
       // Verify: first call had ReplyInThread=true
       assert.True(t, mockAPI.firstCallReplyInThread)
       // Verify: second fallback call had ReplyInThread=false
       assert.False(t, mockAPI.secondCallReplyInThread)
       // Verify: only 2 API calls (no retry wrapper)
       assert.Equal(t, 2, mockAPI.callCount)
   }
   ```
2. Run:
   ```bash
   go test -race -v ./platform/feishu/ -run TestReplyMessage_ThreadFallback
   ```
3. Verify error code constants exist:
   ```bash
   grep -n 'errCodeThreadNotSupported\|errCodeAggregatedMsgThread' platform/feishu/feishu.go
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| error_code | integer | 230071 | concrete | Feishu thread-not-supported error |
| forceThreadReply | boolean | true | concrete | subscription reply context flag |

**Expected Results:**
- (must) Fallback reply is sent without `ReplyInThread` when error code is 230071 or 230072
- (must) Fallback is a single attempt (no `withTransientRetry` wrapper)
- (must) Error codes are defined as named constants, not magic numbers
- (should) Warning log is emitted when fallback triggers

**Failure Handling:**
- Test fails: verify fallback path does not wrap in `withTransientRetry`
- Constant check fails: verify `errCodeThreadNotSupported` and `errCodeAggregatedMsgThread` are defined

### Scenario S9: Concurrency limit uses effectiveSessionKey

**Verifies:** AC-11, AC-12

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test that verifies `ExecuteSubscriptionScan` uses `effectiveSessionKey` for both session creation and `iKey`:
   ```go
   func TestExecuteSubscriptionScan_EffectiveSessionKey(t *testing.T) {
       sub := &Subscription{
           ID:           "sub123",
           SessionKey:   "feishu:oc_chat:ou_user",
           ChatID:       "oc_chat",
           Prompt:       "echo: {{content}}",
           ProcessedIDs: nil,
       }
       engine, mockAgent := newTestEngineWithMockAgent()
       mockScanner := &mockMessageScanner{
           messages: []ScannedMessage{
               {MessageID: "om_msg1", Content: "alert: CPU 95%", IsBot: true, UserID: "ou_otherBot"},
           },
       }
       engine.platform = &mockPlatformWithScanner{scanner: mockScanner, builder: &mockThreadBuilder{
           threadKey: "feishu:oc_chat:root:om_msg1",
       }}
       err := engine.ExecuteSubscriptionScan(sub)
       require.NoError(t, err)
       // Verify: session was created with thread session key (not sub.SessionKey)
       sess := mockAgent.lastCreatedSession
       require.NotNil(t, sess)
       assert.Equal(t, "feishu:oc_chat:root:om_msg1", sess.sessionKey)
       // Verify: iKey uses thread session key
       assert.Contains(t, mockAgent.lastIKey, "feishu:oc_chat:root:om_msg1#subscription:")
   }
   ```
2. Run:
   ```bash
   go test -race -v ./core/ -run TestExecuteSubscriptionScan_EffectiveSessionKey
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| subscription_id | string | sub123 | concrete | test fixture ID |
| session_key | string | feishu:oc_chat:ou_user | concrete | original subscription session key |
| effective_session_key | string | feishu:oc_chat:root:om_msg1 | system-generated | derived by BuildThreadReplyCtx in Step 1 |
| chat_id | string | oc_chat | concrete | test chat ID |

**Expected Results:**
- (must) `sessions.NewSideSession(effectiveSessionKey, ...)` is called with thread session key, not original `sub.SessionKey`
- (must) `iKey` format is `effectiveSessionKey#subscription:sessionID`
- (must) `sessions.ListSessions(effectiveSessionKey)` is used for concurrency count

**Failure Handling:**
- Test fails: verify `effectiveSessionKey` variable is used consistently for session creation, iKey, and concurrency limit

### Scenario S10: /sub alias removed, /subscribe works

**Verifies:** AC-13

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Verify command alias mapping:
   ```bash
   grep -A1 'subscribe' core/engine.go | grep -c '"sub"'
   ```
   → Result should be 0 (no "sub" alias).
2. Write a unit test that verifies `/sub` is not recognized:
   ```go
   func TestCommandAlias_SubRemoved(t *testing.T) {
       // Verify that "sub" is not in the command aliases for "subscribe"
       aliases := findCommandAliases("subscribe")
       assert.NotContains(t, aliases, "sub")
       assert.Contains(t, aliases, "subscribe")
   }
   ```
3. Run:
   ```bash
   go test -race -v ./core/ -run TestCommandAlias_SubRemoved
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| removed_alias | string | sub | concrete | alias to verify removal |
| valid_command | string | subscribe | concrete | the only valid command name |

**Expected Results:**
- (must) `"sub"` is not a valid alias for the subscribe command
- (must) `"subscribe"` is the only valid command name
- (must) Sending `/sub` in a group results in "unknown command" response

**Failure Handling:**
- grep shows "sub": verify the alias entry was changed from `{[]string{"subscribe", "sub"}, "subscribe"}` to `{[]string{"subscribe"}, "subscribe"}`

### Scenario S11: /subscribe appears in /help output

**Verifies:** AC-14

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Verify /help content contains /subscribe:
   ```bash
   grep -c '/subscribe' core/i18n.go
   ```
   → Result should be >= 1 in MsgHelpContent section.
2. Write a unit test:
   ```go
   func TestHelpContent_ContainsSubscribe(t *testing.T) {
       helpText := i18nInstance.T(MsgHelpContent)
       assert.Contains(t, helpText, "/subscribe")
   }
   ```
3. Run:
   ```bash
   go test -race -v ./core/ -run TestHelpContent_ContainsSubscribe
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| command_name | string | /subscribe | concrete | command to verify in help output |
| languages | array | EN, ZH, ZH-TW, JA, ES | concrete | all supported i18n languages |

**Expected Results:**
- (must) `/help` output contains `/subscribe` command description in all 5 languages (EN, ZH, ZH-TW, JA, ES)

**Failure Handling:**
- grep shows 0: verify `/subscribe` was added to all language variants of `MsgHelpContent`

### Scenario S12: WebUI filter placeholders show regex examples

**Verifies:** AC-15

**Execution:** AI-autonomous

**Preconditions:**
- WebUI source files available

**Steps:**
1. Verify placeholder text in all locale files:
   ```bash
   for lang in en zh zh-TW ja es; do
     echo "=== $lang ==="
     grep -A1 'filterPlaceholder' "web/src/i18n/locales/$lang.json"
     grep -A1 'excludeFilterPlaceholder' "web/src/i18n/locales/$lang.json"
   done
   ```
2. Verify placeholders contain regex-related keywords:
   - EN: "Regex"
   - ZH: "正则表达式"
   - ZH-TW: "正規表示式"
   - JA: "正規表現"
   - ES: "Regex"

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| locale_files | array | en.json, zh.json, zh-TW.json, ja.json, es.json | concrete | paths under web/src/i18n/locales/ |
| placeholder_keys | array | filterPlaceholder, excludeFilterPlaceholder | concrete | i18n keys to verify |

**Expected Results:**
- (must) All 5 locale files have `filterPlaceholder` and `excludeFilterPlaceholder` keys
- (must) Each placeholder text mentions regex/正则/正規/正規 in the appropriate language
- (must) Placeholders include example regex patterns (e.g., `alert|warning|error`)

**Failure Handling:**
- Missing locale: verify all 5 JSON files were updated
- Wrong keywords: verify locale-specific regex term was used

### Scenario S13: BuildThreadReplyCtx returns thread session key

**Verifies:** AC-16

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test:
   ```go
   func TestBuildThreadReplyCtx_ReturnsThreadSessionKey(t *testing.T) {
       p := &Platform{}
       rc, tsk, err := p.BuildThreadReplyCtx("feishu:oc_chat:ou_user", "oc_chat", "om_msg123")
       require.NoError(t, err)
       assert.Equal(t, "feishu:oc_chat:root:om_msg123", tsk)
       // Verify replyContext has forceThreadReply=true
       replyCtx := rc.(replyContext)
       assert.True(t, replyCtx.forceThreadReply)
       assert.Equal(t, tsk, replyCtx.sessionKey)
   }
   ```
2. Run:
   ```bash
   go test -race -v ./platform/feishu/ -run TestBuildThreadReplyCtx_ReturnsThreadSessionKey
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| session_key | string | feishu:oc_chat:ou_user | concrete | original session key |
| chat_id | string | oc_chat | concrete | test chat ID |
| message_id | string | om_msg123 | concrete | test message ID |

**Expected Results:**
- (must) `BuildThreadReplyCtx` returns `(replyCtx, threadSessionKey, nil)` with 3 values
- (must) `threadSessionKey` matches format `feishu:oc_chat:root:om_msg123`
- (must) `replyContext.forceThreadReply == true`
- (must) `replyContext.sessionKey == threadSessionKey`

**Failure Handling:**
- Compilation error: verify `ThreadReplyContextBuilder` interface signature changed to `(any, string, error)`

### Scenario S14: Subscription regex is cached and not recompiled per scan

**Verifies:** AC-8

**Execution:** AI-autonomous (unit test)

**Preconditions:**
- Go test environment set up

**Steps:**
1. Write a unit test verifying `compileFilters` populates cached fields:
   ```go
   func TestSubscription_CompileFilters_Cached(t *testing.T) {
       sub := &Subscription{Filter: "alert|warning"}
       err := sub.compileFilters()
       require.NoError(t, err)
       assert.NotNil(t, sub.filterRe)
       assert.Equal(t, "alert|warning", sub.filterRe.String())
   }
   ```
2. Verify `filterMessages` accepts `*regexp.Regexp` parameters (not string):
   ```bash
   grep 'func filterMessages' core/subscription.go
   ```
   → Should show `filterRe, excludeRe *regexp.Regexp` parameters.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| filter_expr | string | alert\|warning | concrete | valid regex expression |
| cached_field | string | filterRe | concrete | unexported field on Subscription struct |

**Expected Results:**
- (must) `Subscription.filterRe` is populated after `compileFilters()`
- (must) `filterMessages` accepts pre-compiled `*regexp.Regexp` parameters
- (must) Regex compilation does not occur inside `filterMessages`

**Failure Handling:**
- Test fails: verify `compileFilters()` sets the unexported `filterRe`/`excludeFilterRe` fields

## Coverage Matrix

| Acceptance Criterion | Covered by Scenario |
|---|---|
| AC-1 | S1 |
| AC-2 | S1, S2 |
| AC-3 | S3 |
| AC-4 | S4 |
| AC-5 | S3, S4 |
| AC-6 | S5 |
| AC-7 | S6 |
| AC-8 | S5, S14 |
| AC-9 | S7 |
| AC-10 | S8 |
| AC-11 | S9 |
| AC-12 | S9 |
| AC-13 | S10 |
| AC-14 | S11 |
| AC-15 | S12 |
| AC-16 | S13 |
| AC-17 | S2 |
| AC-18 | S4 |
| AC-19 | S8 |
