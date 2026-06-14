# 技术设计：Bot1 通过 Card mention Bot2 不生效的彻底修复

## 目标

在群聊中，当 Bot1 通过 Interactive Card 消息 mention Bot2 时，Bot2 应能正确识别自己被 mention 并处理消息。

## 根因回顾

1. 飞书平台限制：Interactive Card 消息的 `<at>` 标签仅做视觉渲染，不填充 `im.message.receive_v1` 事件的 `msg.Mentions` 数组
2. cc-connect 的 `isBotMentioned` 仅遍历 `msg.Mentions` 数组，不扫描消息正文
3. `dispatchMessage` 主 switch 没有 `case "interactive"` 分支，incoming card 消息被 `default` 丢弃

## 方案

**双管齐下：发送端防 + 接收端兜底**

### 变更 1：`isBotMentioned` 增加文本内容匹配（接收端兜底）

当 `msg.Mentions` 数组无法匹配时，回退到从消息正文中检测 `@BotName` 模式。

```go
// isBotMentioned reports whether the bot is mentioned in the message.
// It first checks the mentions array (populated by Feishu for text/post
// messages). If that fails, it falls back to scanning the raw message
// content for @BotName patterns — needed because Interactive Card messages
// do not populate the mentions array even when <at> tags are present.
func isBotMentioned(mentions []*larkim.MentionEvent, botOpenID, botName, msgType, content string) bool {
    // Fast path: check mentions array
    for _, m := range mentions {
        if botOpenID != "" && m.Id != nil && m.Id.OpenId != nil && *m.Id.OpenId == botOpenID {
            return true
        }
        if botName != "" && m.Name != nil && *m.Name == botName {
            return true
        }
    }
    // Slow path: for interactive cards (where mentions array is empty),
    // extract text content and check for @BotName pattern
    if botName != "" && msgType == "interactive" && content != "" {
        text := extractInteractiveCardText(content)
        if strings.Contains(text, "@"+botName) {
            return true
        }
        // Also check for <at> tags with bot's open_id in raw card JSON
        if botOpenID != "" && strings.Contains(content, botOpenID) {
            return true
        }
    }
    return false
}
```

**设计考量：**
- 只对 `msgType == "interactive"` 做文本扫描，text/post 消息已有 `mentions` 数组，无需额外开销
- `extractInteractiveCardText` 已有现成实现，能解析 Schema 2.0 和 legacy 格式的 Card JSON
- 匹配 `@BotName` 覆盖 Card 渲染后文本中的 mention 显示
- 同时检查 Card JSON 中是否包含 botOpenID，覆盖 `<at id=ou_xxx></at>` 格式

### 变更 2：`dispatchMessage` 增加 `case "interactive"` 分支

```go
case "interactive":
    text := extractInteractiveCardText(content)
    text = stripMentions(text, mentions, p.getBotOpenID(), p.getBotName())
    // For interactive cards, also strip @BotName patterns since
    // they're not in the mentions array and won't be handled by
    // the standard stripMentions logic.
    if botName := p.getBotName(); botName != "" {
        text = strings.ReplaceAll(text, "@"+botName, "")
        text = strings.TrimSpace(text)
    }
    if text == "" && quoted.text == "" && len(quoted.images) == 0 {
        return
    }
    p.dispatchCoreMessage(&core.Message{
        SessionKey: sessionKey, Platform: p.platformName,
        MessageID: messageID,
        UserID:    userID, UserName: userName, ChatName: chatName,
        Content: text, ExtraContent: quoted.text, ReplyCtx: rctx,
        UserMessageTimeMs: createTimeMs,
    })
```

**设计考量：**
- 复用 `extractInteractiveCardText` 提取文本
- 复用 `stripMentions` 处理标准 mention placeholder（防御性编程，Card 的 mentions 数组通常为空）
- 额外 strip `@BotName` 文本模式：Card 中 mention 渲染为 `@BotName` 而非 `@_user_N` 占位符
- 空文本检查与 text/post 分支一致

### 变更 3：`onMessage` 中传递必要参数给 `isBotMentioned`

当前调用：
```go
if !isBotMentioned(msg.Mentions, p.getBotOpenID(), p.getBotName()) {
```

修改为：
```go
if !isBotMentioned(msg.Mentions, p.getBotOpenID(), p.getBotName(), msgType, content) {
```

其中 `msgType` 和 `content` 已在 `onMessage` 上下文中可用。

### 变更 4：发送端流式 Card 路径的 mention fallback（防患于未然）

恢复 commit `002fca8` 中的发送端 fallback 逻辑，覆盖流式路径：

**4a. `SendPreviewStart`：检测到 `<at>` 标签时返回 `ErrNotSupported`**

```go
// In SendPreviewStart, after cardJSON is determined:
if containsAtTag(cardJSON) {
    slog.Info(p.tag()+": preview card contains mention, skipping for post fallback",
        "chat_id", chatID)
    return nil, core.ErrNotSupported
}
```

Engine 收到 `ErrNotSupported` 后，跳过 Card 创建，最终走 `p.Send()` → `buildReplyContent` → `MsgTypePost` 路径。

**4b. `UpdateMessage` / `UpdateMessageWithStatusFooter`：检测到 `<at>` 标签时回退到 Post**

```go
if containsAtTag(cardJSON) {
    return p.mentionFallback(ctx, h, content)
}
```

**4c. `mentionFallback` helper：删除正在流式更新的 Card，改为 Post 格式发送**

```go
func (p *Platform) mentionFallback(ctx context.Context, h *feishuPreviewHandle, content string) error {
    if err := p.DeletePreviewMessage(ctx, h); err != nil {
        slog.Warn(p.tag()+": mention fallback: failed to delete card", "error", err)
    }
    rc := replyContext{chatID: h.chatID, messageID: h.messageID}
    return p.Send(ctx, rc, content)
}
```

**4d. `SendWithStatusFooter`：检测到 `<at>` 标签时回退到 `p.Send`**

```go
content = p.resolveMentionsInContent(ctx, rc.chatID, content)
if containsAtTag(content) {
    return p.Send(ctx, rctx, content)
}
```

## 边界条件与异常处理

| 场景 | 处理方式 |
|------|----------|
| botName 为空（API 获取失败） | 文本匹配跳过，依赖 mentions 数组或发送端 fallback |
| Card 中 mention 用了 bot 别名而非注册名 | 接收端文本匹配不命中，但发送端 `resolveMentions` 会将 `@别名` 解析为 openID，生成 `<at>` 标签，被发送端 fallback 捕获 |
| Card 中有多段文本都包含 `@BotName` | `extractInteractiveCardText` 提取所有文本段，`strings.Contains` 能检测到 |
| 流式 Card 更新中途出现 mention | `UpdateMessage` 中 `mentionFallback` 删除 Card 重发 Post |
| 流式 Card 第一次发送就有 mention | `SendPreviewStart` 返回 `ErrNotSupported`，Engine 走 Post 路径 |
| `@BotName` 出现在代码块中（非真实 mention） | 误匹配风险极低：Bot1 回复中 mention Bot2 是有意为之，代码块中 `@` 通常是邮箱，不太可能恰好是 Bot2 完整名 |
| `@BotName` 被 strip 后消息为空 | 与 text/post 分支一致，空消息不触发 `dispatchCoreMessage` |
| `resolveMentions` 配置关闭 | `@Name` 不被转为 `<at>` 标签，但接收端文本匹配仍能检测 `@BotName` |

## 风险与代价

1. **性能**：`extractInteractiveCardText` 在 `isBotMentioned` 中对每条 incoming interactive card 消息执行一次 JSON 解析。只对 `msgType == "interactive"` 且 `mentions` 为空时触发，群聊中 Card 消息比例低，影响可忽略。
2. **误匹配**：`@BotName` 文本模式匹配可能误匹配代码块中的内容。风险极低且后果仅是 Bot2 多收一条消息。
3. **`mentionFallback` 的 Card 删除**：用户会看到消息闪一下（Card 消失 → Post 出现）。这是 Card mention 不生效的必然代价，比完全不触发 Bot2 更可接受。
4. **向后兼容**：`isBotMentioned` 签名变更（加 `msgType`, `content` 参数），需更新调用点（当前仅 `onMessage` 一处）。

## 验收标准

1. Bot1 通过 Card 消息 mention Bot2 → Bot2 收到消息并处理
2. Bot1 通过 Post 消息 mention Bot2 → 行为不变（仍通过 mentions 数组匹配）
3. Bot1 通过流式 Card 发送含 mention 的回复 → 回退到 Post 格式，Bot2 被正确触发
4. Bot1 发送不含 mention 的 Card → 正常渲染为 Card，无 fallback
5. botName 为空时 → 退化为仅 OpenID 匹配 + 发送端 fallback
6. 群聊中非 mention 消息仍被正确过滤

## 实现顺序

1. `isBotMentioned` 增加文本匹配（变更 1 + 变更 3）—— 接收端兜底
2. `dispatchMessage` 增加 `case "interactive"`（变更 2）—— 接收端处理
3. 发送端 fallback 恢复（变更 4a/4b/4c/4d）—— 发送端防护
4. 补充单元测试覆盖所有场景
