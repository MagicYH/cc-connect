# 订阅回复 Thread 化与过滤规则修复设计

## 背景

订阅功能的核心场景：另一个 bot 在群里发送告警消息（不会 @ 我们的 bot），订阅扫描定时拉取群消息，发现匹配的消息后处理并回复。

当前存在两个问题：
1. 订阅回复未使用 `reply_in_thread`，导致回复直接出现在群聊主界面，与其他消息混杂
2. 过滤规则未正确生效：filter/exclude_filter 规则表现异常，且过滤方向错误——应保留 bot 消息、排除人类消息

## 设计目标

1. 订阅回复始终使用 `reply_in_thread=true`，在扫描到的消息下创建 thread
2. thread 内的后续对话路由到独立的 thread session（继承 workspace/project 配置）
3. `filterMessages` 排除人类消息（`IsBot=false`）和自身 bot 消息，保留其他 bot 消息
4. filter/exclude_filter 支持正则表达式匹配
5. WebUI 中以 placeholder 方式说明过滤表达式写法
6. 将 `/sub` 别名移除，仅保留 `/subscribe`
7. 将 `/subscribe` 加入 `/help` 输出

## 详细设计

### 1. replyContext 新增 forceThreadReply 字段

**文件：** `platform/feishu/feishu.go`

```go
type replyContext struct {
    messageID        string
    chatID           string
    sessionKey       string
    forceThreadReply bool // 订阅场景下始终使用 reply_in_thread
}
```

### 2. BuildThreadReplyCtx 修改

**文件：** `platform/feishu/subscription.go`

使用 `p.tag()` 直接构造 thread session key（不从 sessionKey 解析，避免 multi-workspace 格式问题），并设置 `forceThreadReply=true`：

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

Thread session key 格式：`feishu:oc_chatID:root:om_messageID`，与 `threadIsolation` 模式下的 `makeSessionKey` 格式一致。使用 `p.tag()` 保证平台名称正确，不受原始 sessionKey 格式影响（如 multi-workspace 前缀）。

### 3. buildReplyMessageReqBody 与 shouldReplyInThread 修改

**文件：** `platform/feishu/feishu.go`

`shouldReplyInThread` 修改：当 session key 是 thread key 时（`isThreadSessionKey` 为 true），无论 `threadIsolation` 配置如何，都应使用 `reply_in_thread`。这确保订阅创建的 thread 中的后续回复（包括 card 更新等）都保持在 thread 内。

```go
func (p *Platform) shouldReplyInThread(rc replyContext) bool {
    if rc.messageID == "" {
        return false
    }
    // thread session key 始终使用 reply_in_thread，无论 threadIsolation 配置
    // 这确保订阅创建的 thread 中所有后续回复（包括 card 更新）都在 thread 内
    if isThreadSessionKey(rc.sessionKey) {
        return true
    }
    return p.threadIsolation
}
```

`buildReplyMessageReqBody` 中的 `rc.forceThreadReply` 检查仍保留，作为额外的第一道保障（当 `isThreadSessionKey` 还未被调用方设置时）：

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

`replyMessage` 中增加 `reply_in_thread` 失败时的 fallback：如果 Feishu API 返回错误码 230071（群聊不支持 thread）或 230072（聚合消息不支持 thread），则回退到不带 `ReplyInThread` 的普通回复：

```go
// Feishu API 错误码常量（见 https://open.feishu.cn/document/server-docs/im-v1/message/create）
const (
    errCodeThreadNotSupported  = 230071 // 群聊不支持 thread
    errCodeAggregatedMsgThread = 230072 // 聚合消息不支持 thread
)

func (p *Platform) replyMessage(ctx context.Context, rc replyContext, msgType, content string) error {
    // ...existing retry logic...
    resp, err := client.Im.Message.Reply(ctx, req, options...)
    if err != nil {
        return fmt.Errorf("%s: reply api call: %w", p.tag(), err)
    }
    if !resp.Success() {
        // Fallback: 如果 reply_in_thread 失败，尝试不带 thread 的普通回复
        // 仅做单次尝试，不使用 withTransientRetry，避免重试导致重复发送
        if rc.forceThreadReply && (resp.Code == errCodeThreadNotSupported || resp.Code == errCodeAggregatedMsgThread) {
            slog.Warn("subscription: reply_in_thread not supported, falling back to normal reply",
                "chat_id", rc.chatID, "message_id", rc.messageID, "code", resp.Code)
            fallbackBody := larkim.NewReplyMessageReqBodyBuilder().
                MsgType(msgType).Content(content).Build()
            fallbackReq := larkim.NewReplyMessageReqBuilder().
                MessageId(rc.messageID).Body(fallbackBody).Build()
            // 单次尝试，仅刷新 token 重试
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
}
```

Fallback 设计要点：
- 不使用 `withTransientRetry`，仅使用 `withFreshTenantAccessTokenRetry`（最多重试一次 token 过期场景）
- 避免 fallback 重试导致重复发送消息

### 4. ExecuteSubscriptionScan 修改

**文件：** `core/engine.go`

使用 thread session key 创建 session，并发限制使用 threadSessionKey 计数，workspace 从原始 session key 继承：

```go
// 在 step 5 中：
for i, msg := range matched {
    // 构建 thread reply context
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

    // 使用 thread session key（如有的话）创建 side session
    effectiveSessionKey := sub.SessionKey
    if threadSessionKey != "" {
        effectiveSessionKey = threadSessionKey
    }

    // 并发限制检查使用 effectiveSessionKey，与 session 创建对齐
    if sub.ConcurrencyLimit > 0 {
        activeCount := len(sessions.ListSessions(effectiveSessionKey))
        if activeCount >= sub.ConcurrencyLimit {
            queuedCount = len(matched) - i
            slog.Warn("subscription: concurrency limit reached, queuing remaining messages",
                "subscription_id", sub.ID, "active", activeCount, "limit", sub.ConcurrencyLimit, "queued", queuedCount)
            break
        }
    }

    session := sessions.NewSideSession(effectiveSessionKey, "subscription-"+sub.ID)
    // ...rest of injection logic...

    // iKey 也必须使用 effectiveSessionKey，与 session 创建对齐
    // 原代码: iKey := fmt.Sprintf("%s#subscription:%s", sub.SessionKey, session.ID)
    // 修改为:
    iKey := fmt.Sprintf("%s#subscription:%s", effectiveSessionKey, session.ID)

    // workspace 从原始 sub.SessionKey 解析（不变）
}
```

iKey 对齐说明：`iKey` 用于 injection 查找，必须与 session 的 sessionKey 一致。如果 iKey 用 `sub.SessionKey` 但 session 创建用 `effectiveSessionKey`，会导致消息注入失败。

### 5. makeSessionKey 修改

**文件：** `platform/feishu/feishu.go`

仅当消息有 `root_id` 时（即在 thread 内），将消息路由到 thread session。这不依赖 `threadIsolation` 配置：

```go
func (p *Platform) makeSessionKey(msg *larkim.EventMessage, chatID, userID string) string {
    // 如果消息在 thread 内（有 root_id），始终路由到 thread session
    // 这确保订阅创建的 thread 中用户后续对话路由正确
    // 也确保 threadIsolation=false 时 thread 内的消息不会丢失上下文
    rootID := stringValue(msg.RootId)
    if rootID != "" {
        return fmt.Sprintf("%s:%s:root:%s", p.tag(), chatID, rootID)
    }
    // threadIsolation=true 时，根消息也作为 thread root
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

行为变更说明：当 `threadIsolation=false` 且消息在 thread 内（有 `root_id`）时，消息路由到 thread session 而非 base session。这改变了之前的行为，但只影响 thread 内的消息。非 thread 消息不受影响。

### 6. ReconstructReplyCtx

**文件：** `platform/feishu/feishu.go`

现有实现已正确处理 `root:` 前缀，无需修改。

### 7. filterMessages 修复

**文件：** `core/subscription.go`

核心逻辑变更：排除人类消息和自身 bot 消息，保留其他 bot 消息。filter/exclude_filter 改用正则表达式匹配。

```go
// filterMessages applies Filter and ExcludeFilter to scanned messages.
// It excludes human messages (IsBot=false, since humans have the @mention channel)
// and self-bot messages. Filter and ExcludeFilter use regex matching.
// Returns an error if regex compilation fails.
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
        // 排除人类消息——人类已有 @mention 通道，无需通过订阅处理
        if !msg.IsBot {
            continue
        }
        // 排除自身 bot 消息，避免循环
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

变更要点：
- `IsBot=false` 的消息（人类消息）被排除——订阅的核心用途是处理其他 bot 的告警消息
- `msg.UserID == botID` 的消息被排除——防止 bot 处理自己的回复形成循环（复用现有 `UserID` 字段）
- filter/exclude_filter 改用预编译的 `*regexp.Regexp` 替代 `strings.Contains`
- 正则在 Subscription 结构体上缓存，每次 scan 不重新编译
- 函数签名从接收字符串改为接收预编译正则，由调用方负责编译
- 返回 `([]ScannedMessage, error)` 以便调用方处理错误

#### Subscription 新增缓存正则字段

**文件：** `core/subscription.go`

```go
type Subscription struct {
    // ...existing fields...
    Filter            string    `json:"filter,omitempty"`
    ExcludeFilter     string    `json:"exclude_filter,omitempty"`
    // ...

    // 以下字段不序列化，从 Filter/ExcludeFilter 编译而来
    filterRe      *regexp.Regexp `json:"-"`
    excludeFilterRe *regexp.Regexp `json:"-"`
}

// compileFilters 编译 Filter 和 ExcludeFilter 为正则表达式。
// 在创建/更新订阅时调用，编译失败返回错误。
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

`executeScan()` 中调用：
```go
matched, err := filterMessages(allMsgs, snapshot.filterRe, snapshot.excludeFilterRe, snapshot.ProcessedIDs, botID)
```

从 store 加载订阅时也需要调用 `compileFilters()`。如果加载时编译失败（如正则语法被外部修改），记录警告并将该订阅标记为错误状态。

#### cmdSubscribeAdd 中验证正则

**文件：** `core/subscription_cmd.go`

在 `cmdSubscribeAdd` 中，创建订阅前验证 filter/exclude_filter 是否为合法正则：

```go
sub := &Subscription{Filter: filter, ExcludeFilter: exclude, ...}
if err := sub.compileFilters(); err != nil {
    e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubInvalidFilter), err))
    return
}
```

新增 i18n key `MsgSubInvalidFilter`，中文："过滤表达式无效: %s"，英文："Invalid filter expression: %s"。

#### botID 传递

`filterMessages` 的 `botID` 参数从 `SubscriptionManager.executeScan()` 传入。

调用链：
1. `SubscriptionManager` 持有 `botIDProvider BotIDProvider`（在 `NewSubscriptionManager()` 时注入）
2. `executeScan()` 获取 `botID := ""`，如果 `m.botIDProvider != nil` 则 `botID = m.botIDProvider.BotID()`
3. `filterMessages(allMsgs, snapshot.filterRe, snapshot.excludeFilterRe, snapshot.ProcessedIDs, botID)`

方案：通过新接口 `BotIDProvider` 实现，避免在 core 中硬编码平台名称。

**文件：** `core/interfaces.go`

```go
// BotIDProvider 返回当前 bot 的 ID，用于订阅过滤排除自身消息
type BotIDProvider interface {
    BotID() string
}
```

Feishu Platform 实现 `BotID()` 返回 `p.getBotOpenID()`。`SubscriptionManager` 持有 `botIDProvider BotIDProvider`（可为 nil，表示不支持该接口）。

### 8. ThreadReplyContextBuilder 接口签名变更

**文件：** `core/interfaces.go`

签名变更，额外返回 `threadSessionKey`：

```go
type ThreadReplyContextBuilder interface {
    BuildThreadReplyCtx(sessionKey string, chatID string, messageID string) (replyCtx any, threadSessionKey string, err error)
}
```

当前只有 Feishu 实现此接口，需同步修改。测试中的 stub 实现也需适配。

### 9. 诊断日志增强

在 `ExecuteSubscriptionScan` 中添加结构化日志：

```go
slog.Info("subscription: filter results",
    "subscription_id", sub.ID,
    "total_scanned", len(allMsgs),
    "human_excluded", humanCount,
    "self_excluded", selfCount,
    "filter_matched", len(matched),
    "filter", sub.Filter,
    "exclude_filter", sub.ExcludeFilter)
```

在 `BuildThreadReplyCtx` 中：

```go
slog.Debug("subscription: built thread reply context",
    "chat_id", chatID, "message_id", messageID,
    "thread_session_key", threadSessionKey)
```

### 10. 命令别名修改：移除 `/sub`，仅保留 `/subscribe`

**文件：** `core/engine.go`

将命令别名从 `{[]string{"subscribe", "sub"}, "subscribe"}` 改为 `{[]string{"subscribe"}, "subscribe"}`：

```go
{[]string{"subscribe"}, "subscribe"},
```

`/sub` 不再作为有效命令。用户必须使用 `/subscribe`。

### 11. `/help` 输出添加 `/subscribe` 命令

**文件：** `core/i18n.go`

在所有语言的 `MsgHelpContent` 中添加 `/subscribe` 命令说明。

**英文：** 在 `/bind` 之前添加：
```
"/subscribe [add|list|del|enable|disable]\n  Manage subscriptions (auto-scan group messages)\n\n" +
```

**中文：**
```
"/subscribe [add|list|del|enable|disable]\n  管理订阅（自动扫描群消息）\n\n" +
```

**繁体中文：**
```
"/subscribe [add|list|del|enable|disable]\n  管理訂閱（自動掃描群訊息）\n\n" +
```

**日文：**
```
"/subscribe [add|list|del|enable|disable]\n  サブスクリプション管理（グループメッセージの自動スキャン）\n\n" +
```

**西班牙文：**
```
"/subscribe [add|list|del|enable|disable]\n  Gestionar suscripciones (escaneo automático de mensajes de grupo)\n\n" +
```

### 12. WebUI 过滤表达式 placeholder 更新

**文件：** `web/src/i18n/locales/*.json`

将 filter 和 exclude_filter 的 placeholder 从当前的简单关键词提示改为正则表达式说明：

**英文 (en.json)：**
- `filterPlaceholder`: `"Regex: alert|warning|error"`
- `excludeFilterPlaceholder`: `"Regex: info|debug"`

**中文 (zh.json)：**
- `filterPlaceholder`: `"正则表达式: alert|warning|error"`
- `excludeFilterPlaceholder`: `"正则表达式: info|debug"`

**繁体中文 (zh-TW.json)：**
- `filterPlaceholder`: `"正規表示式: alert|warning|error"`
- `excludeFilterPlaceholder`: `"正規表示式: info|debug"`

**日文 (ja.json)：**
- `filterPlaceholder`: `"正規表現: alert|warning|error"`
- `excludeFilterPlaceholder`: `"正規表現: info|debug"`

**西班牙文 (es.json)：**
- `filterPlaceholder`: `"Regex: alert|warning|error"`
- `excludeFilterPlaceholder`: `"Regex: info|debug"`

## 影响范围

| 文件 | 修改类型 | 影响 |
|------|---------|------|
| `core/interfaces.go` | ThreadReplyContextBuilder 签名变更 + BotIDProvider 接口 | 接口变更，Feishu 实现方需适配 |
| `core/subscription.go` | filterMessages 排除人类消息+自身bot消息 + Subscription 缓存正则 + compileFilters() | 行为变更 + 正则缓存 |
| `core/subscription_cmd.go` | cmdSubscribeAdd 验证正则 | 创建时校验 |
| `core/engine.go` | ExecuteSubscriptionScan 使用 thread session key + iKey 对齐 + 移除 /sub 别名 | 订阅回复路由变更，命令别名变更 |
| `core/i18n.go` | /help 添加 /subscribe + MsgSubInvalidFilter | 帮助文本 + 错误消息变更 |
| `platform/feishu/feishu.go` | replyContext + shouldReplyInThread + buildReplyMessageReqBody + makeSessionKey + replyMessage fallback + BotID() + 错误码常量 | thread 回复逻辑变更 + BotID 实现 |
| `platform/feishu/subscription.go` | BuildThreadReplyCtx 返回 thread session key | 回复上下文构造变更 |
| `web/src/i18n/locales/*.json` | filterPlaceholder / excludeFilterPlaceholder 更新 | WebUI placeholder 文本变更 |
| 测试文件 | 适配接口变更和过滤逻辑 | 测试更新 |

## 兼容性考虑

- `makeSessionKey` 对 `threadIsolation=false` 的行为变更：thread 内消息（有 `root_id`）路由到 thread session。非 thread 消息不受影响。此变更确保订阅创建的 thread 中后续对话路由正确。
- `shouldReplyInThread` 行为变更：当 session key 是 thread key（`isThreadSessionKey` 为 true）时，无论 `threadIsolation` 配置如何都返回 true。这确保订阅创建的 thread 中所有回复（包括 card 更新）都保持在 thread 内。
- `ThreadReplyContextBuilder` 接口签名变更：返回值从 `(any, error)` 变为 `(any, string, error)`。当前只有 Feishu 实现此接口。
- `filterMessages` 行为变更：之前不过滤任何消息类型，现在排除人类消息和自身 bot 消息。现有测试 `TestSubscriptionFilterEmpty` 需更新。
- `filterMessages` 正则变更：filter/exclude_filter 从 `strings.Contains` 改为 `regexp.MatchString`。已有的纯文本 filter 仍能正常工作（纯文本是合法的正则表达式），但特殊字符如 `.`、`*`、`+` 会获得正则语义。如果用户之前在 filter 中使用了这些字符作为字面量，行为会变化。
- 正则在 `Subscription` 结构体上缓存（`filterRe`/`excludeFilterRe` 字段），在创建/更新/加载时编译。无效正则在创建时拒绝（返回错误），在加载时标记为错误状态。
- `iKey` 使用 `effectiveSessionKey` 而非 `sub.SessionKey`，与 session 创建对齐。
- `/sub` 别名移除：使用 `/sub` 的用户需改用 `/subscribe`。
- `reply_in_thread` fallback：如果 API 返回 230071/230072，单次尝试不带 thread 的普通回复（不使用 transient retry，避免重复发送）。
- `ScannedMessage` 不新增字段——复用已有 `UserID` 字段进行 botID 比较。
- Feishu API 错误码定义为命名常量（`errCodeThreadNotSupported`、`errCodeAggregatedMsgThread`）。
