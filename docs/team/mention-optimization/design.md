# Feishu Mention 机制分析与优化设计

## 一、问题背景

在飞书群聊中，当 Bot1 在消息中 @Bot2 时，如果消息内容是斜杠命令（如 `@Bot2 /help`），Bot2 无法正常处理该命令。此外，当前的 mention 机制存在以下局限：

1. 依赖 `botOpenID` 进行匹配，当 `botOpenID` 为空时整个机制失效
2. 卡片消息中的 `<at>` 标签无法触发飞书事件
3. 没有基于机器人名称的匹配机制，无法满足"只要出现 @Bot1 就应处理"的需求

## 二、消息处理全链路分析

### 2.1 消息接收入口

飞书消息通过两种模式接收，最终汇聚到同一个 `dispatcher.EventDispatcher`：

**WebSocket 模式**（默认）：
- `platform/feishu/feishu.go:469-492`，`startWebSocketMode()` 创建 `larkws.Client`
- SDK 推送 `P2MessageReceiveV1` 事件到 `OnP2MessageReceiveV1` 处理器

**Webhook 模式**（配置了 `encrypt_key` 时）：
- `platform/feishu/feishu.go:495-544`，HTTP 服务器接收 POST 请求
- `webhookHandler` 构造 `larkevent.EventReq` 并传递给 `p.eventHandler.Handle()`

**共享 WebSocket 扇出**：
- `platform/feishu/ws_shared.go:1-85`，多个 Platform 实例共享同一 `app_id` 时共享一个连接
- `registerSharedWS()` (line 30) 按 `app_id|domain` 分组
- 主 Platform 的 `OnP2MessageReceiveV1` 处理器 (feishu.go:395-403) 扇出给所有同组 Platform

### 2.2 @bot Mention 检测

**`isBotMentioned()` 函数** — `platform/feishu/feishu.go:3037-3044`：

```go
func isBotMentioned(mentions []*larkim.MentionEvent, botOpenID string) bool {
    for _, m := range mentions {
        if m.Id != nil && m.Id.OpenId != nil && *m.Id.OpenId == botOpenID {
            return true
        }
    }
    return false
}
```

关键点：
- 遍历 `msg.Mentions`（`[]*larkim.MentionEvent`，由飞书 SDK 提供）
- 每个 `MentionEvent` 包含 `Key`（占位符如 `@_user_1`）、`Id.OpenId`、`Name`
- 仅当 mention 的 `Id.OpenId` 与本 bot 的 `botOpenID` 匹配时返回 `true`
- **完全依赖 `botOpenID`**，没有名称匹配的 fallback

**Bot OpenID 获取** — `platform/feishu/feishu.go:3016-3035`：

```go
func (p *Platform) fetchBotOpenID() (string, error) {
    resp, err := p.client.Get(context.Background(),
        "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
    // ...
    var result struct {
        Code int `json:"code"`
        Bot  struct {
            OpenID string `json:"open_id"`
        } `json:"bot"`
    }
    // ...
    return result.Bot.OpenID, nil
}
```

关键问题：**只解析了 `open_id`，丢弃了 API 返回的其他字段（如机器人名称）**。

### 2.3 群聊 Mention 过滤门控

`platform/feishu/feishu.go:1070-1089`：

```go
if chatType == "group" && !p.groupReplyAll && p.getBotOpenID() != "" {
    if !isBotMentioned(msg.Mentions, p.getBotOpenID()) {
        switch {
        case p.respondToAtEveryoneAndHere && msg.Content != nil && strings.Contains(*msg.Content, "@_all"):
            // 处理 @all
        case p.threadIsolation && isAttachmentMsgType(msgType) && p.isActiveThreadSession(sessionKey):
            // 允许活跃线程中的附件消息
        default:
            slog.Debug(p.tag()+": ignoring group message without bot mention", "chat_id", chatID)
            return nil  // ← 消息被丢弃
        }
    }
}
```

关键问题：
1. 当 `botOpenID == ""` 时，**整个过滤逻辑被跳过**，bot 会处理所有群消息
2. 过滤仅依赖 `isBotMentioned()` 的 OpenID 匹配，没有名称匹配的 fallback

### 2.4 Mention 文本清理

**文本消息** (`msgType == "text"`) — `platform/feishu/feishu.go:1175-1198`：
- 原始内容 JSON：`{"text":"@_user_1 hello world"}`
- 调用 `stripMentions(textBody.Text, mentions, p.getBotOpenID())`

**富文本消息** (`msgType == "post"`) — `platform/feishu/feishu.go:1257-1270`：
- `parsePostContent()` → `extractPostParts()` 解析富文本结构
- `extractPostParts()` (line 4514-4525) 处理 `at` 标签：

```go
case "at":
    if p.getBotOpenID() != "" && elem.UserId == p.getBotOpenID() {
        continue  // bot mention: 静默移除
    }
    switch {
    case elem.UserId == "all":
        textParts = append(textParts, "@all")
    case elem.UserName != "":
        textParts = append(textParts, "@"+elem.UserName)
    case elem.UserId != "":
        textParts = append(textParts, "@"+p.resolveUserName(elem.UserId))
    }
```

**`stripMentions()` 函数** — `platform/feishu/feishu.go:3078-3098`：

```go
func stripMentions(text string, mentions []*larkim.MentionEvent, botOpenID string) string {
    if len(mentions) == 0 {
        return text
    }
    for _, m := range mentions {
        if m.Key == nil {
            continue
        }
        if botOpenID != "" && m.Id != nil && m.Id.OpenId != nil && *m.Id.OpenId == botOpenID {
            text = strings.ReplaceAll(text, *m.Key, "")      // bot 自身 mention: 移除
        } else if m.Name != nil && *m.Name != "" {
            text = strings.ReplaceAll(text, *m.Key, "@"+*m.Name) // 其他用户: 替换为 @Name
        } else {
            text = strings.ReplaceAll(text, *m.Key, "")      // 无名称: 移除
        }
    }
    return strings.TrimSpace(text)
}
```

### 2.5 引擎层斜杠命令检测

`core/engine.go:2618-2623`：

```go
if len(msg.Images) == 0 && strings.HasPrefix(content, "/") {
    if e.handleCommand(p, msg, content) {
        return
    }
    // 未识别的斜杠命令 — 作为普通消息交给 agent 处理
}
```

关键点：引擎依赖 `strings.HasPrefix(content, "/")` 检测命令。如果 mention 未被正确移除，文本以 `@BotName` 开头，斜杠命令检测失败。

## 三、根本原因分析

### 根因 1（主要）：`botOpenID` 为空时 `stripMentions` 无法正确清理 bot mention

**影响：斜杠命令检测失败**

当 `botOpenID == ""` 时，`stripMentions()` (feishu.go:3081-3098) 无法识别哪个 mention 是 bot 自身的。bot 的 `@_user_N` 占位符被替换为 `@BotName` 而非移除，导致文本以 `@Bot2` 开头而非 `/help`。

消息文本在各阶段的状态：

| 阶段 | botOpenID 正常 | botOpenID 为空 |
|------|---------------|----------------|
| 原始飞书 JSON | `@_user_1 /help` | `@_user_1 /help` |
| stripMentions 后 (line 1183) | `/help` | `@Bot2 /help` |
| 引擎 HasPrefix("/") (line 2618) | true — 命令被处理 | **false — 命令未被识别** |

`botOpenID` 为空的两种场景：

1. **Webhook 模式**：`Start()` (feishu.go:370) 中 `fetchBotOpenID()` 被完全跳过：

```go
// platform/feishu/feishu.go:370
if !p.shouldUseWebhookMode() {
    if openID, err := p.fetchBotOpenID(); err != nil {
```

2. **启动时 API 调用失败**：`fetchBotOpenID()` 失败只记录警告 (line 372)，bot 以 `botOpenID=""` 继续运行：

```go
// platform/feishu/feishu.go:371-372
if openID, err := p.fetchBotOpenID(); err != nil {
    slog.Warn(p.platformName+": failed to get bot open_id, group chat filtering disabled", "error", err)
```

### 根因 2（已部分修复）：Interactive Card `<at>` 标签不触发 Mention 事件

当 Bot1 通过**流式卡片路径**发送包含 `@Bot2` 的消息时，卡片 (MsgTypeInteractive) 中的 `<at>` 标签仅是视觉展示——不会为 Bot2 触发 `im.message.receive_v1` 事件。Bot2 完全收不到这条消息。

**当前修复状态**：已在 `mention-fallback/design.md` 中设计和实现：
- `SendPreviewStart` (line 3978, 3999)：检测到 `<at>` 时返回 `core.ErrNotSupported`
- `UpdateMessage` (line 4202)：调用 `mentionFallback()` 删除卡片并以 Post 格式重发
- `UpdateMessageWithStatusFooter` (line 4238)：同上
- 非流式路径 (`Reply`/`Send`/`buildReplyContent`) 已有此 fallback

**但此修复仅解决出站方向**（Bot1 发给 Bot2 的消息）。入站方向（根因 1）仍未修复。

### 根因 3（副作用）：`botOpenID` 为空时群聊过滤失效

`platform/feishu/feishu.go:1070`：

```go
if chatType == "group" && !p.groupReplyAll && p.getBotOpenID() != "" {
```

当 `botOpenID` 为空时，整个 mention 过滤被跳过，导致 bot 处理所有群消息。这不会直接破坏斜杠命令，但造成不正确的消息路由。

## 四、优化设计：基于名称的 Mention 匹配

### 4.1 设计目标

**只要消息中出现 `@Bot1` 的形式，且 Bot1 的名字与飞书机器人的名称完全一致，Bot1 就应该处理这条消息。** 无论消息类型（卡片、文本、流式消息）。

### 4.2 核心设计原则

1. **双重匹配**：OpenID 匹配（现有）+ 名称匹配（新增），任一命中即认为 bot 被 mention
2. **名称来源**：从飞书 API `/open-apis/bot/v3/info` 获取机器人名称，而非手动配置，避免歧义
3. **向后兼容**：不影响已有 `botOpenID` 匹配逻辑，名称匹配作为 fallback 和增强

### 4.3 实现方案

#### 变更 1：扩展 `fetchBotOpenID` 同时获取 bot 名称

**文件**：`platform/feishu/feishu.go:3016-3035`

当前函数只返回 `openID`，需扩展为同时返回 bot 名称：

```go
// 修改前
func (p *Platform) fetchBotOpenID() (string, error) {
    // ...
    var result struct {
        Code int `json:"code"`
        Bot  struct {
            OpenID string `json:"open_id"`
        } `json:"bot"`
    }
    return result.Bot.OpenID, nil
}

// 修改后
func (p *Platform) fetchBotInfo() (openID, botName string, err error) {
    resp, err := p.client.Get(context.Background(),
        "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
    if err != nil {
        return "", "", fmt.Errorf("api call: %w", err)
    }
    var result struct {
        Code int `json:"code"`
        Bot  struct {
            OpenID   string `json:"open_id"`
           AppName string `json:"app_name"`
        } `json:"bot"`
    }
    if err := json.Unmarshal(resp.RawBody, &result); err != nil {
        return "", "", fmt.Errorf("parse response: %w", err)
    }
    if result.Code != 0 {
        return "", "", fmt.Errorf("api code=%d", result.Code)
    }
    return result.Bot.OpenID, result.Bot.AppName, nil
}
```

**注意**：飞书 `/open-apis/bot/v3/info` API 返回的 `app_name` 字段需要通过实际 API 调用验证。如果 API 不返回名称字段，则需要备选方案（见 4.4 节）。

#### 变更 2：Platform 结构体增加 `botName` 字段

**文件**：`platform/feishu/feishu.go:115`

```go
type Platform struct {
    // ... 现有字段 ...
    botOpenID        string
    botName          string  // 新增：飞书机器人的显示名称，用于名称匹配
    // ...
}
```

新增 getter：

```go
func (p *Platform) getBotName() string {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return p.botName
}
```

#### 变更 3：Webhook 模式也获取 bot 信息

**文件**：`platform/feishu/feishu.go:365-379`

```go
// 修改前
if !p.shouldUseWebhookMode() {
    if openID, err := p.fetchBotOpenID(); err != nil {
        slog.Warn(...)
    } else {
        p.mu.Lock()
        p.botOpenID = openID
        p.mu.Unlock()
        slog.Info(p.platformName+": bot identified", "open_id", openID)
    }
}

// 修改后：始终尝试获取 bot 信息
if openID, botName, err := p.fetchBotInfo(); err != nil {
    slog.Warn(p.platformName+": failed to get bot info, mention matching may be incomplete", "error", err)
} else {
    p.mu.Lock()
    p.botOpenID = openID
    p.botName = botName
    p.mu.Unlock()
    slog.Info(p.platformName+": bot identified", "open_id", openID, "name", botName)
}
```

#### 变更 4：`isBotMentioned` 增加名称匹配

**文件**：`platform/feishu/feishu.go:3037-3044`

```go
// 修改后：双重匹配逻辑
func isBotMentioned(mentions []*larkim.MentionEvent, botOpenID, botName string) bool {
    for _, m := range mentions {
        // 匹配方式 1：OpenID 匹配（现有逻辑）
        if botOpenID != "" && m.Id != nil && m.Id.OpenId != nil && *m.Id.OpenId == botOpenID {
            return true
        }
        // 匹配方式 2：名称匹配（新增）
        // 飞书 MentionEvent 的 Name 字段包含被 mention 用户的显示名称
        if botName != "" && m.Name != nil && *m.Name == botName {
            return true
        }
    }
    return false
}
```

关键依据：飞书 `MentionEvent` 结构体（来自 SDK `larkim.MentionEvent`）包含 `Name` 字段，该字段为被 mention 用户的显示名称。当用户在飞书群聊中 @Bot2 时，`MentionEvent.Name` 的值就是 Bot2 的飞书机器人名称。

#### 变更 5：群聊过滤门控使用双重匹配

**文件**：`platform/feishu/feishu.go:1070-1071`

```go
// 修改前
if chatType == "group" && !p.groupReplyAll && p.getBotOpenID() != "" {
    if !isBotMentioned(msg.Mentions, p.getBotOpenID()) {

// 修改后：不再要求 botOpenID 非空
if chatType == "group" && !p.groupReplyAll {
    if !isBotMentioned(msg.Mentions, p.getBotOpenID(), p.getBotName()) {
```

这样即使 `botOpenID` 为空，只要 `botName` 可用，名称匹配仍能正常工作。

#### 变更 6：`stripMentions` 增加名称匹配识别 bot mention

**文件**：`platform/feishu/feishu.go:3078-3098`

```go
func stripMentions(text string, mentions []*larkim.MentionEvent, botOpenID, botName string) string {
    if len(mentions) == 0 {
        return text
    }
    for _, m := range mentions {
        if m.Key == nil {
            continue
        }
        // 判断是否为 bot 自身的 mention：OpenID 匹配或名称匹配
        isBotMention := (botOpenID != "" && m.Id != nil && m.Id.OpenId != nil && *m.Id.OpenId == botOpenID) ||
            (botName != "" && m.Name != nil && *m.Name == botName)
        if isBotMention {
            text = strings.ReplaceAll(text, *m.Key, "")
        } else if m.Name != nil && *m.Name != "" {
            text = strings.ReplaceAll(text, *m.Key, "@"+*m.Name)
        } else {
            text = strings.ReplaceAll(text, *m.Key, "")
        }
    }
    return strings.TrimSpace(text)
}
```

#### 变更 7：`extractPostParts` 中 `at` 标签处理增加名称匹配

**文件**：`platform/feishu/feishu.go:4514-4517`

```go
// 修改前
case "at":
    if p.getBotOpenID() != "" && elem.UserId == p.getBotOpenID() {
        continue
    }

// 修改后
case "at":
    botOpenID := p.getBotOpenID()
    botName := p.getBotName()
    isBotAt := (botOpenID != "" && elem.UserId == botOpenID) ||
        (botName != "" && elem.UserName == botName)
    if isBotAt {
        continue
    }
```

### 4.4 备选方案：API 不返回名称时的兜底

如果 `/open-apis/bot/v3/info` 不返回 `app_name` 字段，可使用以下方式获取 bot 名称：

1. **从第一次收到的 MentionEvent 中提取**：当 bot 第一次被 mention 时，MentionEvent 的 `Name` 字段包含 bot 的名称。可在首次收到时缓存。

```go
// 在 onMessage 中，收到 mention 后提取 bot 名称
if p.getBotName() == "" {
    for _, m := range msg.Mentions {
        if m.Name != nil && *m.Name != "" {
            if p.getBotOpenID() != "" && m.Id != nil && m.Id.OpenId != nil && *m.Id.OpenId == p.getBotOpenID() {
                p.mu.Lock()
                p.botName = *m.Name
                p.mu.Unlock()
                slog.Info(p.platformName+": bot name discovered from mention event", "name", *m.Name)
                break
            }
        }
    }
}
```

2. **从群成员列表中提取**：`chatMemberCache` 中已缓存群成员的 `displayName -> openID` 映射，可从中查找 bot 的名称：

```go
func (p *Platform) discoverBotNameFromMembers(ctx context.Context, chatID string) string {
    members := p.getChatMembers(ctx, chatID)
    botOpenID := p.getBotOpenID()
    for name, openID := range members {
        if openID == botOpenID {
            return name
        }
    }
    return ""
}
```

3. **配置项兜底**：在 config.toml 中添加可选的 `bot_name` 配置项，仅在 API 无法获取时使用：

```toml
[platforms.feishu]
bot_name = "MyBot"  # 可选，仅在自动获取失败时使用
```

### 4.5 调用点变更汇总

需要更新所有调用 `isBotMentioned`、`stripMentions` 的位置：

| 函数 | 文件位置 | 变更说明 |
|------|---------|---------|
| `onMessage` 中群聊过滤 | feishu.go:1070-1071 | 移除 `botOpenID != ""` 条件，传入 `botName` |
| `dispatchMessage` 中文本消息 | feishu.go:1183 | `stripMentions` 增加 `botName` 参数 |
| `dispatchMessage` 中 post 消息 | feishu.go:1259 | `stripMentions` 增加 `botName` 参数 |
| `isBotMentioned` 函数签名 | feishu.go:3037 | 增加 `botName` 参数 |
| `stripMentions` 函数签名 | feishu.go:3081 | 增加 `botName` 参数 |
| `extractPostParts` at 标签处理 | feishu.go:4514-4517 | 增加名称匹配判断 |
| `Start` 中 bot 信息获取 | feishu.go:365-379 | 始终调用 `fetchBotInfo`，存储 `botName` |
| `fetchBotOpenID` → `fetchBotInfo` | feishu.go:3016-3035 | 返回 `(openID, botName, error)` |
| Setup CLI `fetchBotOpenIDForSetup` | cmd/cc-connect/feishu.go:255-306 | 同步更新（可选，不影响运行时） |
| 单元测试 | feishu_test.go | 更新 `TestStripMentions`、新增名称匹配测试 |

### 4.6 各消息类型下的行为对比

| 场景 | 当前行为 | 优化后行为 |
|------|---------|-----------|
| `@Bot2 /help`（文本，botOpenID 正常） | `/help` → 命令执行 | 无变化 |
| `@Bot2 /help`（文本，botOpenID 为空） | `@Bot2 /help` → 命令丢失 | `@Bot2` 被识别并移除 → `/help` → 命令执行 |
| `@Bot2 /help`（post 富文本，botOpenID 为空） | `@Bot2 /help` → 命令丢失 | `@Bot2` 被识别并移除 → `/help` → 命令执行 |
| `@Bot2 你好`（群聊，botOpenID 为空） | 消息被处理（过滤失效） | 通过名称匹配正确识别 mention |
| Bot1 卡片消息含 `@Bot2` | Bot2 收不到事件（已修复） | mentionFallback 已处理 |
| Bot1 流式卡片含 `@Bot2` | Bot2 收不到事件（已修复） | mentionFallback 已处理 |

## 五、验证计划

### 5.1 单元测试

1. **`TestIsBotMentioned_NameMatch`**：当 `botOpenID` 为空但 `botName` 匹配时，返回 `true`
2. **`TestIsBotMentioned_BothMatch`**：OpenID 和名称同时匹配时，返回 `true`
3. **`TestIsBotMentioned_NameMismatch`**：名称不匹配时，返回 `false`
4. **`TestStripMentions_BotNameMatch`**：当 `botOpenID` 为空但 `botName` 匹配时，bot mention 被移除
5. **`TestStripMentions_SlashCommandAfterBotName`**：`@Bot2 /help` → `/help`
6. **`TestFetchBotInfo`**：验证 API 返回的 `app_name` 被正确解析

### 5.2 手动测试

1. 在飞书群聊中，用户发送 `@Bot2 /help`，验证 Bot2 正确执行命令
2. 在 Webhook 模式下，验证 `@Bot2 /help` 仍然正常工作
3. Bot1 在回复中 @Bot2，验证 Bot2 收到消息并处理
4. 群聊中不带 @mention 的消息，验证 bot 不处理（过滤正常）

## 六、风险评估

- **低风险**：名称匹配是增量添加，不影响现有 OpenID 匹配逻辑
- **潜在风险**：如果多个 bot 拥有相同名称，名称匹配可能误判。但飞书在同一个群中不允许同名 bot，所以实际风险极低
- **Tradeoff**：Webhook 模式下新增 API 调用可能影响启动速度，但该调用有失败容错，不会阻塞启动
