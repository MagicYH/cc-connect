# Subscription Auto-Investigation Verification

> Source spec: ./2026-06-26-subscription-auto-investigation-design.md
> Used by: superpowers:writing-plans (TDD coverage) and post-implementation smoke testing.

## Environment & Access

| Item | Value | How to Obtain |
|---|---|---|
| Target environment | Local development | Run `make build && ./cc-connect daemon start` |
| Management API | `http://localhost:9820` | Default port from config.toml `management.port` |
| Data directory | `./data/subscriptions/` | Relative to cc-connect working directory |
| Feishu bot app_id | From `config.toml` `[platforms.feishu]` section | Read config file |
| Test Feishu group | The group the bot is a member of | `lark-cli group list` or manual |
| lark-cli | `/usr/bin/lark-cli` | Already installed |
| cc-connect binary | `./cc-connect` | `make build` |
| Config file | `./config.toml` | Project root |

## Public Operations

### Op-StartCCConnect

Purpose: Build and start cc-connect daemon for testing

```bash
make build && ./cc-connect daemon restart
# Verify it's running
curl -s http://localhost:9820/api/v1/cron | head -c 100
```

### Op-StopCCConnect

Purpose: Stop cc-connect daemon

```bash
./cc-connect daemon stop
```

### Op-CreateSubscription

Purpose: Create a subscription via management API

```bash
curl -s -X POST http://localhost:9820/api/v1/subscription \
  -H "Content-Type: application/json" \
  -d '{
    "project": "'"${CC_PROJECT}"'",
    "chat_id": "'"${CHAT_ID}"'",
    "platform": "feishu",
    "filter": "'"${FILTER}"'",
    "exclude_filter": "'"${EXCLUDE}"'",
    "prompt": "'"${PROMPT}"'",
    "interval": "'"${INTERVAL}"'"
  }' | jq .
```

### Op-DeleteAllSubscriptions

Purpose: Clean up all subscriptions after a test

```bash
SUB_IDS=$(curl -s http://localhost:9820/api/v1/subscription | jq -r '.[].id // .[].ID')
for id in $SUB_IDS; do
  curl -s -X DELETE "http://localhost:9820/api/v1/subscription/${id}"
done
```

### Op-SendFeishuMessage

Purpose: Send a test message to the Feishu group via lark-cli

```bash
lark-cli message send --chat "${CHAT_ID}" --text "${MESSAGE_TEXT}"
```

## Acceptance Criteria

- [ ] AC-1: Subscription can be created via `/subscribe` slash command with filter, exclude, and prompt parameters (covers spec §Creation & Management)
- [ ] AC-2: Subscription can be created via management API `POST /api/v1/subscription` (covers spec §Management API)
- [ ] AC-3: Uniqueness constraint enforced — duplicate `(Project, ChatID)` subscription is rejected (covers spec §Uniqueness constraint)
- [ ] AC-4: Subscription scans group messages at the configured interval and matches messages by filter keyword (covers spec §Execution Flow steps 1-3)
- [ ] AC-5: ExcludeFilter correctly excludes matching messages (e.g., recovery cards) (covers spec §Filter logic)
- [ ] AC-6: When filter and exclude are both empty, all non-bot messages are matched (covers spec §Filter logic)
- [ ] AC-7: Agent replies are threaded under the original alarm card (covers spec §Reply mode)
- [ ] AC-8: Anchor advances correctly after successful scan; no messages are lost or duplicated (covers spec §Execution Flow step 5, §Deduplication)
- [ ] AC-9: On partial injection failure, anchor stops before failed messages and ProcessedIDs prevents re-injection of already-processed messages (covers spec §Execution Flow step 5, §Deduplication)
- [ ] AC-10: ConcurrencyLimit applies to total active sessions, not per-scan starts (covers spec §Session strategy)
- [ ] AC-11: Subscription auto-disables after 10 consecutive permanent errors; transient errors do not count (covers spec §Auto-disable)
- [ ] AC-12: Auto-disable notification is sent to the group chat (covers spec §Auto-disable)
- [ ] AC-13: Subscription persists across daemon restart (covers spec §Startup Recovery)
- [ ] AC-14: Management API supports full CRUD: list, detail, update, delete, trigger (covers spec §Management API)
- [ ] AC-15: WebUI shows subscription list with creation, edit, and delete actions (covers spec §WebUI)
- [ ] AC-16: Non-admin users cannot create/modify/delete subscriptions (covers spec §AuthZ)
- [ ] AC-17: Feature flag `subscriptions_enabled` disables all subscription functionality (covers spec §Feature Flag)
- [ ] AC-18: Poison message (consistently failing injection) is skipped after 3 attempts and anchor advances (covers spec §Poison Message Handling)
- [ ] AC-19: All user-facing strings use i18n with translations for EN, ZH, ZH-TW, JA, ES (covers spec §i18n keys)
- [ ] AC-20: Subscription logs are written as append-only per-subscription files; no compaction on write (covers spec §Storage, §Logging)

## Test Scenarios

### Scenario S1: Create subscription via slash command

**Verifies:** AC-1, AC-3

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running (Op-StartCCConnect)
- Bot is a member of a Feishu group with ChatID `${CHAT_ID}`
- No existing subscription for this `(CC_PROJECT, CHAT_ID)` pair
- User sending the command is in `admin_from` list

**Steps:**
1. Send the `/subscribe` command in the Feishu group
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "/subscribe 告警 恢复 排查以下报警：{{content}}"
   ```
2. Wait for bot reply (poll up to 10s)
   ```bash
   sleep 5
   ```
3. Verify subscription was created via management API
   ```bash
   SUB_LIST=$(curl -s http://localhost:9820/api/v1/subscription)
   echo "${SUB_LIST}" | jq '.[].filter' | grep -q "告警"
   echo "${SUB_LIST}" | jq '.[].exclude_filter' | grep -q "恢复"
   SUB_ID=$(echo "${SUB_LIST}" | jq -r '.[0].id // .[0].ID')
   ```
4. Attempt duplicate creation — should be rejected
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "/subscribe 告警 恢复 排查以下报警：{{content}}"
   sleep 5
   ```
   → If duplicate is accepted, **stop** — uniqueness constraint is broken.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID from config |
| filter | string | 告警 | concrete | Chinese keyword for "alarm" |
| exclude | string | 恢复 | concrete | Chinese keyword for "recovery" |
| prompt | string | 排查以下报警：{{content}} | concrete | Test prompt template |
| sub_id | string | ${SUB_ID} (extracted in Step 3) | system-generated | Subscription ID |

**Expected Results:**
- (must) Bot replies with `MsgSubCreated` confirmation containing the subscription ID, filter, and exclude values
- (must) Management API returns exactly 1 subscription with `filter = "告警"`, `exclude_filter = "恢复"`, `enabled = true`
- (must) Duplicate creation attempt is rejected with `MsgSubAlreadyExists`

**Failure Handling:**
- Bot does not reply: verify daemon is running, bot is in the group, and user is in admin_from
- Duplicate is accepted: uniqueness constraint bug — investigate data_dir/subscriptions/jobs.json

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S2: Create subscription via management API

**Verifies:** AC-2, AC-14

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- No existing subscription for this `(CC_PROJECT, CHAT_ID)` pair

**Steps:**
1. Create subscription via API
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "error",
       "exclude_filter": "",
       "prompt": "Check this error: {{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   echo "Created subscription: ${SUB_ID}"
   ```
2. List all subscriptions
   ```bash
   curl -s http://localhost:9820/api/v1/subscription | jq .
   ```
3. Get subscription detail
   ```bash
   curl -s "http://localhost:9820/api/v1/subscription/${SUB_ID}" | jq .
   ```
4. Update subscription (change filter)
   ```bash
   curl -s -X PATCH "http://localhost:9820/api/v1/subscription/${SUB_ID}" \
     -H "Content-Type: application/json" \
     -d '{"filter": "warning"}' | jq .
   ```
5. Verify filter was updated
   ```bash
   curl -s "http://localhost:9820/api/v1/subscription/${SUB_ID}" | jq -r '.filter // .Filter'
   ```
   → If filter is not "warning", **stop** — PATCH update failed.
6. Disable subscription via PATCH
   ```bash
   curl -s -X PATCH "http://localhost:9820/api/v1/subscription/${SUB_ID}" \
     -H "Content-Type: application/json" \
     -d '{"enabled": false}' | jq .
   ```
7. Delete subscription
   ```bash
   curl -s -X DELETE "http://localhost:9820/api/v1/subscription/${SUB_ID}" | jq .
   ```
8. Verify deletion
   ```bash
   curl -s http://localhost:9820/api/v1/subscription | jq 'length'
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) Step 1 returns HTTP 200 with subscription ID
- (must) Step 2 lists the newly created subscription
- (must) Step 3 returns full subscription detail with all fields populated
- (must) Step 5 returns `warning` (updated filter)
- (must) Step 6 returns subscription with `enabled = false`
- (must) Step 7 returns success
- (must) Step 8 returns 0 subscriptions after deletion

**Failure Handling:**
- Step 1 returns 4xx: check request body format and that no duplicate (Project, ChatID) exists
- Step 5 returns old value: PATCH may not be applying partial updates — check API handler

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S3: Subscription scans and filters alarm messages

**Verifies:** AC-4, AC-5, AC-6

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists with `filter="告警"`, `exclude_filter="恢复"`, `interval="*/2 * * * *"`
- Subscription anchor is set (not first run)

**Steps:**
1. Create subscription (or reuse existing)
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "告警",
       "exclude_filter": "恢复",
       "prompt": "排查以下报警：{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Send an alarm message in the group
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "【告警】CPU使用率超过90%"
   ```
3. Send a recovery message (should be excluded)
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "【恢复】CPU使用率已恢复正常"
   ```
4. Send an unrelated message (should not match)
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "今日天气不错"
   ```
5. Wait for the next scan cycle (up to 2 minutes)
   ```bash
   sleep 130
   ```
6. Check subscription logs for processing results
   ```bash
   cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | jq . || echo "No log file yet"
   ```
7. Verify the alarm message was processed but recovery and unrelated messages were not
   ```bash
   LOG_ENTRIES=$(cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null)
   ALARM_COUNT=$(echo "${LOG_ENTRIES}" | grep -c "CPU使用率超过90%" || echo 0)
   RECOVERY_COUNT=$(echo "${LOG_ENTRIES}" | grep -c "CPU使用率已恢复正常" || echo 0)
   echo "Alarm matched: ${ALARM_COUNT}, Recovery matched: ${RECOVERY_COUNT}"
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) Log file contains exactly 1 entry matching "CPU使用率超过90%" (the alarm)
- (must) Log file contains 0 entries matching "CPU使用率已恢复正常" (recovery excluded)
- (must) Log file contains 0 entries matching "今日天气不错" (unrelated, no filter match)
- (should) Agent replies are threaded under the alarm message in the group

**Failure Handling:**
- No log file after 2+ minutes: check daemon logs for "subscription: scan started" and "subscription: scan completed" messages
- Recovery message is processed: ExcludeFilter matching is broken — check filter logic
- Unrelated message is processed: Filter matching is broken — it should only match "告警" substring

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S4: Match-all subscription with empty filters

**Verifies:** AC-6

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- No existing subscription for this group

**Steps:**
1. Create subscription with empty filter and exclude
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "",
       "exclude_filter": "",
       "prompt": "收到消息：{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Send various message types
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "普通消息测试"
   lark-cli message send --chat "${CHAT_ID}" --text "另一条消息"
   ```
3. Wait for scan cycle
   ```bash
   sleep 130
   ```
4. Check log entries — both messages should be matched
   ```bash
   cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | jq .
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) Log file contains entries for both test messages
- (must) Bot messages (from lark-cli using the same bot) are skipped (IsBot=true)

**Failure Handling:**
- Bot's own messages are processed: IsBot filtering is broken — check ScannedMessage.IsBot field
- No messages matched: empty-filter "match all" logic is broken

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S5: Anchor advancement and deduplication

**Verifies:** AC-8, AC-9

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists for the group with `filter=""` (match all)

**Steps:**
1. Create match-all subscription
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "",
       "exclude_filter": "",
       "prompt": "{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Send one message, wait for one scan cycle
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "dedup-test-msg-1"
   sleep 130
   ```
3. Check anchor advanced and message was processed
   ```bash
   SUB_DETAIL=$(curl -s "http://localhost:9820/api/v1/subscription/${SUB_ID}")
   ANCHOR_1=$(echo "${SUB_DETAIL}" | jq -r '.anchor // .Anchor')
   echo "Anchor after first scan: ${ANCHOR_1}"
   LOG_1=$(cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | grep -c "dedup-test-msg-1")
   echo "Message 1 processed count: ${LOG_1}"
   ```
   → If anchor is empty or message count is 0, **stop** — first scan failed.
4. Wait for another scan cycle (no new messages)
   ```bash
   sleep 130
   ```
5. Verify message was NOT processed again (dedup)
   ```bash
   LOG_2=$(cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | grep -c "dedup-test-msg-1")
   echo "Message 1 processed count after second scan: ${LOG_2}"
   ```
6. Send another message, wait for scan
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "dedup-test-msg-2"
   sleep 130
   ```
7. Verify second message processed and anchor advanced
   ```bash
   SUB_DETAIL=$(curl -s "http://localhost:9820/api/v1/subscription/${SUB_ID}")
   ANCHOR_2=$(echo "${SUB_DETAIL}" | jq -r '.anchor // .Anchor')
   echo "Anchor after third scan: ${ANCHOR_2}"
   LOG_3=$(cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | grep -c "dedup-test-msg-2")
   echo "Message 2 processed count: ${LOG_3}"
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) After first scan: anchor is non-empty, message "dedup-test-msg-1" appears exactly 1 time in logs
- (must) After second scan (no new messages): message "dedup-test-msg-1" still appears exactly 1 time (no duplicate)
- (must) After third scan: anchor has advanced, message "dedup-test-msg-2" appears exactly 1 time

**Failure Handling:**
- Anchor stays empty after first scan: MessageScanner.ListMessages may be returning errors — check daemon logs
- Duplicate entries in logs: ProcessedIDs dedup is broken or anchor is not advancing properly
- Second message not processed: anchor may have advanced past it — check anchor semantics

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S6: Reply-in-thread under original message

**Verifies:** AC-7

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists with `filter="告警"`, `exclude_filter="恢复"`
- Feishu platform implements `ThreadReplyContextBuilder` interface

**Steps:**
1. Create subscription
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "告警",
       "exclude_filter": "恢复",
       "prompt": "排查以下报警：{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Send alarm message
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "【告警】磁盘空间不足，剩余10%"
   ```
3. Wait for scan + agent processing
   ```bash
   sleep 180
   ```
4. Check that agent reply exists as a thread reply under the alarm message (inspect via Feishu group UI or API)
   ```bash
   # Verify via log that the session was created with thread reply context
   cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | jq .
   ```
5. Check daemon logs for ThreadReplyContextBuilder usage
   ```bash
   grep -r "BuildThreadReplyCtx" data/logs/ 2>/dev/null || journalctl --user -u cc-connect --since "5 min ago" 2>/dev/null | grep -i "thread" | tail -5
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) Log entry exists for the alarm message with status "submitted"
- (must) Agent response appears as a thread reply under the alarm message in the Feishu group (visually confirmed or verified via API)
- (should) Daemon logs show ThreadReplyContextBuilder was used to construct the reply context

**Failure Handling:**
- Agent replies in the chat root instead of thread: ThreadReplyContextBuilder not being used — check ExecuteSubscriptionScan implementation
- No agent response: session may have timed out or agent error — check daemon logs

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S7: Auto-disable on persistent errors

**Verifies:** AC-11, AC-12

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists for a group the bot does NOT have access to (simulated by using a non-existent ChatID)

**Steps:**
1. Create subscription pointing to a non-existent group
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "oc_invalid_chat_id_for_auto_disable_test",
       "platform": "feishu",
       "filter": "",
       "exclude_filter": "",
       "prompt": "{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Wait for 10+ scan cycles (approximately 20+ minutes at 2-min interval)
   ```bash
   sleep 1300
   ```
3. Check subscription status — should be auto-disabled
   ```bash
   SUB_DETAIL=$(curl -s "http://localhost:9820/api/v1/subscription/${SUB_ID}")
   ENABLED=$(echo "${SUB_DETAIL}" | jq -r '.enabled // .Enabled')
   CONSEC_ERR=$(echo "${SUB_DETAIL}" | jq -r '.consecutive_errors // .ConsecutiveErrors')
   echo "Enabled: ${ENABLED}, ConsecutiveErrors: ${CONSEC_ERR}"
   ```
4. Check daemon logs for auto-disable event
   ```bash
   grep "subscription: auto-disabled" data/logs/*.log 2>/dev/null | tail -3 || echo "Check journalctl if using systemd"
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |
| invalid_chat_id | string | oc_invalid_chat_id_for_auto_disable_test | concrete | Non-existent ChatID to trigger permanent errors |

**Expected Results:**
- (must) After 10+ scan cycles, subscription `enabled = false`
- (must) `consecutive_errors >= 10`
- (must) Daemon logs contain `subscription: auto-disabled` entry for this subscription
- (should) A `MsgSubAutoDisabled` notification was attempted (may fail since the chat is invalid)

**Failure Handling:**
- Subscription stays enabled after 20+ minutes: auto-disable threshold not working — check error classification (permanent vs transient)
- ConsecutiveErrors not incrementing: errors may be classified as transient — check error type classification logic

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S8: Subscription persists across daemon restart

**Verifies:** AC-13

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists and is enabled

**Steps:**
1. Create subscription
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "告警",
       "exclude_filter": "恢复",
       "prompt": "排查以下报警：{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Verify subscription exists and record anchor
   ```bash
   SUB_DETAIL=$(curl -s "http://localhost:9820/api/v1/subscription/${SUB_ID}")
   ANCHOR_BEFORE=$(echo "${SUB_DETAIL}" | jq -r '.anchor // .Anchor')
   echo "Anchor before restart: ${ANCHOR_BEFORE}"
   ```
3. Restart daemon
   ```bash
   ./cc-connect daemon restart
   sleep 5
   ```
4. Verify subscription still exists with same ID and anchor
   ```bash
   SUB_DETAIL_AFTER=$(curl -s "http://localhost:9820/api/v1/subscription/${SUB_ID}")
   ANCHOR_AFTER=$(echo "${SUB_DETAIL_AFTER}" | jq -r '.anchor // .Anchor')
   ENABLED_AFTER=$(echo "${SUB_DETAIL_AFTER}" | jq -r '.enabled // .Enabled')
   echo "Anchor after restart: ${ANCHOR_AFTER}, Enabled: ${ENABLED_AFTER}"
   ```
5. Send a test message and wait for scan to verify subscription is still active
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "【告警】重启后测试"
   sleep 130
   cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | grep -c "重启后测试"
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) After restart, subscription with same ID exists
- (must) Anchor value is preserved (same as before restart)
- (must) Subscription is still enabled
- (must) After restart, the subscription continues to scan and process new messages

**Failure Handling:**
- Subscription missing after restart: jobs.json not being loaded on startup — check startup recovery logic
- Anchor reset to empty: anchor state not persisted or not loaded — check AtomicWriteFile usage
- Subscription disabled after restart: Enabled flag not persisted — check persistence logic

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S9: Non-admin user cannot create subscription

**Verifies:** AC-16

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- A non-admin user is available in the Feishu group

**Steps:**
1. As a non-admin user, send `/subscribe` command
   ```bash
   # Note: lark-cli sends as the bot, so this test requires a human to send
   # the command from a non-admin account. Alternatively, test via management API
   # without proper auth to verify the AuthZ layer.
   curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "test",
       "exclude_filter": "",
       "prompt": "{{content}}"
     }' | jq .
   ```
   Note: Full non-admin slash command test requires a human to send the message from a non-admin Feishu account. The API-level AuthZ test above verifies the server-side check.

2. Verify that the slash command AuthZ is implemented by checking the command handler code
   ```bash
   grep -n "MsgAdminRequired\|admin_from\|isAdmin" core/subscription_cmd.go 2>/dev/null || grep -rn "MsgAdminRequired\|admin_from\|isAdmin" core/ | grep -i subscribe | head -5
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |

**Expected Results:**
- (must) `/subscribe` command from non-admin user is rejected with `MsgAdminRequired`
- (must) Non-admin users can still use `/subscribe list` and `/subscribe show`
- (should) Management API AuthZ is consistent with slash command AuthZ

**Failure Handling:**
- Non-admin can create subscriptions: AuthZ check is missing — add admin_from check in command handler

**Cleanup:** none needed

### Scenario S10: Feature flag disables subscription functionality

**Verifies:** AC-17

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running with `subscriptions_enabled = true`

**Steps:**
1. Create a subscription while feature is enabled (should succeed)
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "test",
       "exclude_filter": "",
       "prompt": "{{content}}"
     }')
   echo "${CREATE_RESP}" | jq .
   ```
2. Stop daemon, set `subscriptions_enabled = false` in config.toml, restart
   ```bash
   ./cc-connect daemon stop
   # Use sed to set the feature flag
   sed -i 's/^subscriptions_enabled.*=.*true/subscriptions_enabled = false/' config.toml || \
     echo 'subscriptions_enabled = false' >> config.toml
   ./cc-connect daemon start
   sleep 5
   ```
3. Attempt to create a subscription (should fail)
   ```bash
   curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "test2",
       "exclude_filter": "",
       "prompt": "{{content}}"
     }' | jq .
   ```
4. Verify subscription API returns 404
   ```bash
   curl -s -o /dev/null -w "%{http_code}" http://localhost:9820/api/v1/subscription
   ```
5. Restore feature flag
   ```bash
   ./cc-connect daemon stop
   sed -i 's/^subscriptions_enabled.*=.*false/subscriptions_enabled = true/' config.toml
   ./cc-connect daemon start
   sleep 5
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |

**Expected Results:**
- (must) Step 1 succeeds (subscription created while feature enabled)
- (must) Step 3 returns error (404 or feature-disabled message)
- (must) Step 4 returns HTTP 404
- (must) After re-enabling, subscription API works again

**Failure Handling:**
- Step 3 succeeds: feature flag is not being checked — verify config is loaded and flag is checked in the API handler

**Cleanup:** Op-DeleteAllSubscriptions; ensure `subscriptions_enabled = true` in config.toml

### Scenario S11: Poison message handling

**Verifies:** AC-18

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists with match-all filter
- There is a mechanism to simulate a consistently-failing message injection (e.g., a message with content that causes engine injection to fail)

**Steps:**
1. Create subscription
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "",
       "exclude_filter": "",
       "prompt": "{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Verify poison message skip logic exists in the code
   ```bash
   grep -n "poison\|PoisonMessage\|skipAfterRetry\|maxRetryPerMessage" core/subscription.go 2>/dev/null | head -5
   ```
3. Check that after 3 scan attempts of the same failing message, a warning is logged and anchor advances
   ```bash
   # This is a code-level verification; the actual behavior depends on
   # what causes injection failure. Monitor logs for:
   grep "poison message skipped" data/logs/*.log 2>/dev/null | tail -3 || echo "No poison messages detected in logs yet"
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) Code contains poison message handling logic (grep hits in Step 2)
- (should) After 3 consecutive failures on the same message, the message is skipped and anchor advances with a `slog.Warn` entry

**Failure Handling:**
- No poison message logic found: implementation is incomplete — add retry counter per message

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S12: i18n coverage for all subscription strings

**Verifies:** AC-19

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect codebase is available

**Steps:**
1. Check that all required MsgKey constants exist in i18n.go
   ```bash
   for key in MsgSubCreated MsgSubAlreadyExists MsgSubNotFound MsgSubListTitle MsgSubListAllTitle MsgSubEnabled MsgSubDisabled MsgSubDeleted MsgSubAutoDisabled MsgSubDelConfirm MsgSubShowFormat MsgSubEditUsage MsgSubUsage MsgSubHelp; do
     if grep -q "$key" core/i18n.go; then
       echo "FOUND: $key"
     else
       echo "MISSING: $key"
     fi
   done
   ```
2. Check that each key has translations for all 5 languages (EN, ZH, ZH-TW, JA, ES)
   ```bash
   grep -A 20 "MsgSubCreated" core/i18n.go | grep -c "translator\|Translate\|\"en\"\|\"zh\"\|\"zh-TW\"\|\"ja\"\|\"es\""
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| i18n_keys | array<string> | All MsgSub* keys listed in spec §i18n keys | concrete | Full list from design spec |

**Expected Results:**
- (must) All 14 MsgKey constants are defined in `core/i18n.go`
- (must) Each key has translations for EN, ZH, ZH-TW, JA, ES

**Failure Handling:**
- Missing keys: add them to i18n.go with translations
- Missing translations: add the missing language entries

**Cleanup:** none needed

### Scenario S13: Logging — append-only per-subscription log files

**Verifies:** AC-20

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists and has processed at least one message

**Steps:**
1. Create subscription and trigger a message
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "",
       "exclude_filter": "",
       "prompt": "{{content}}",
       "interval": "*/2 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   lark-cli message send --chat "${CHAT_ID}" --text "log-test-message"
   sleep 130
   ```
2. Verify log file exists for this subscription
   ```bash
   ls -la data/subscriptions/logs/${SUB_ID}.log 2>/dev/null || echo "Log file not found"
   ```
3. Verify log format is JSON lines (append-only)
   ```bash
   head -1 data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | jq . > /dev/null 2>&1 && echo "Valid JSON line" || echo "Not valid JSON"
   ```
4. Verify no compaction happened (file was only appended to, not rewritten)
   ```bash
   wc -l data/subscriptions/logs/${SUB_ID}.log 2>/dev/null
   # Send another message and verify line count increases
   lark-cli message send --chat "${CHAT_ID}" --text "log-test-message-2"
   sleep 130
   wc -l data/subscriptions/logs/${SUB_ID}.log 2>/dev/null
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) Log file at `data/subscriptions/logs/${SUB_ID}.log` exists
- (must) Each line is valid JSON (LogEntry format)
- (must) Line count increases monotonically (append-only, no compaction on write)

**Failure Handling:**
- Log file in `logs.json` instead of per-subscription file: storage format doesn't match spec — update implementation
- JSON parse error: log entry format is wrong — check serialization logic

**Cleanup:** Op-DeleteAllSubscriptions

### Scenario S14: Manual scan trigger via API

**Verifies:** AC-14

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect daemon is running
- Subscription exists and is enabled

**Steps:**
1. Create subscription with a long interval (every hour)
   ```bash
   CREATE_RESP=$(curl -s -X POST http://localhost:9820/api/v1/subscription \
     -H "Content-Type: application/json" \
     -d '{
       "project": "'"${CC_PROJECT}"'",
       "chat_id": "'"${CHAT_ID}"'",
       "platform": "feishu",
       "filter": "告警",
       "exclude_filter": "",
       "prompt": "{{content}}",
       "interval": "0 * * * *"
     }')
   SUB_ID=$(echo "${CREATE_RESP}" | jq -r '.id // .ID')
   ```
2. Send an alarm message
   ```bash
   lark-cli message send --chat "${CHAT_ID}" --text "【告警】手动触发测试"
   ```
3. Trigger manual scan via API
   ```bash
   curl -s -X POST "http://localhost:9820/api/v1/subscription/${SUB_ID}/trigger" | jq .
   ```
4. Wait for processing
   ```bash
   sleep 30
   ```
5. Check that the message was processed
   ```bash
   cat data/subscriptions/logs/${SUB_ID}.log 2>/dev/null | grep -c "手动触发测试"
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| chat_id | string | ${CHAT_ID} | runtime-fetch | Feishu group ChatID |
| sub_id | string | ${SUB_ID} (extracted in Step 1) | system-generated | Subscription ID |

**Expected Results:**
- (must) `/trigger` endpoint returns HTTP 200
- (must) Message "手动触发测试" appears in subscription logs

**Failure Handling:**
- `/trigger` returns 404: endpoint not implemented — check management API handler
- Message not processed: trigger may not have actually run the scan — check daemon logs

**Cleanup:** Op-DeleteAllSubscriptions

## Coverage Matrix

| Acceptance Criterion | Covered by Scenario |
|---|---|
| AC-1: Slash command creation | S1 |
| AC-2: API creation | S2 |
| AC-3: Uniqueness constraint | S1 |
| AC-4: Scan and filter by keyword | S3 |
| AC-5: ExcludeFilter | S3 |
| AC-6: Match-all with empty filters | S4 |
| AC-7: Reply-in-thread | S6 |
| AC-8: Anchor advancement and dedup | S5 |
| AC-9: Partial failure dedup | S5 |
| AC-10: ConcurrencyLimit total active | S5 (concurrent sessions observed), code review |
| AC-11: Auto-disable on permanent errors | S7 |
| AC-12: Auto-disable notification | S7 |
| AC-13: Persistence across restart | S8 |
| AC-14: Management API CRUD | S2, S14 |
| AC-15: WebUI management | Manual visual check in browser (not automatable via CLI) |
| AC-16: Non-admin AuthZ | S9 |
| AC-17: Feature flag | S10 |
| AC-18: Poison message handling | S11 |
| AC-19: i18n coverage | S12 |
| AC-20: Append-only log files | S13 |
