# Bot1 @mention Bot2 消息丢失 — 根因分析与解决方案

## 问题描述

在飞书群聊中，Bot1（如 Team Leader）通过 @mention 向 Bot2（如 Reviewer）发送消息，Bot2 收不到任何事件，无法被触发。这是 cc-connect 多 Bot 协作场景的核心阻塞问题。

---

## 根因分析

问题由**两层缺陷**叠加导致，缺一不可：

### 第一层：飞书平台限制 — Interactive Card 不填充 `mentions` 数组

飞书 `im.message.receive_v1` 事件中，`EventMessage.Mentions` 字段的填充行为因消息类型而异：

| 消息类型 | `<at>` 渲染 | `mentions` 字段填充 | 触发被 @ Bot 收到事件 |
|----------|-------------|---------------------|----------------------|
| `text`   | ✅          | ✅                  | ✅                   |
| `post`   | ✅          | ✅                  | ✅                   |
| `interactive` (Card) | ✅ 仅视觉渲染 | ❌ **不填充** | ❌ **不触发** |

即使 Card 中写了 `<at user_id="ou_xxx">Bot2</at>`，飞书返回的事件中 `msg.Mentions` 为空。

**证据 — SDK 数据结构定义：**

```go
// /home/chenhao.magic/go/pkg/mod/github.com/larksuite/oapi-sdk-go/v3@v3.5.3/service/im/v1/model.go:2418
type EventSender struct {
    SenderId   *UserId `json:"sender_id,omitempty"`
    SenderType *string `json:"sender_type,omitempty"` // 文档：目前只支持用户(user)发送的消息
    TenantKey  *string `json:"tenant_key,omitempty"`
}

// model.go:2143
type EventMessage struct {
    // ...
    Mentions []*MentionEvent `json:"mentions,omitempty"` // 被提及用户的信息
    // ...
}
```

**证据 — 生产日志：**

群聊 `oc_b3699052712082aa60403044dbbeb4f9` 中 Bot1 发送 Card mention Bot2 时，Bot2 收到事件但 `mentions=1`，随后被 `default` 丢弃：

```
time=2026-06-13T15:55:28.015Z level=DEBUG msg="feishu: inbound message"
    message_id=om_x100b6de3982edca4b275c6f24c1ff06
    chat_id=oc_b3699052712082aa60403044dbbeb4f9
    chat_type=group mentions=2

time=2026-06-13T15:55:32.515Z level=DEBUG msg="feishu: ignoring unsupported message type"
    type=interactive
time=2026-06-13T15:55:32.515Z level=DEBUG msg="feishu: ignoring unsupported message type"
    type=interactive
```

> **注意**：当 Bot1 以 `post` 类型消息发送含 `<at>` 标签的消息时，飞书**会**填充 `mentions` 数组，Bot2 **能**正确收到事件。问题仅发生在 `interactive`（Card）类型消息。

### 第二层：cc-connect 代码缺陷 — 两处缺失

#### 缺陷 1：`isBotMentioned` 仅检查 `mentions` 数组

```go
// platform/feishu/feishu.go:3089
func isBotMentioned(mentions []*larkim.MentionEvent, botOpenID, botName string) bool {
    for _, m := range mentions {
        if botOpenID != "" && m.Id != nil && m.Id.OpenId != nil && *m.Id.OpenId == botOpenID {
            return true
        }
        if botName != "" && m.Name != nil && *m.Name == botName {
            return true
        }
    }
    return false
}
```

对于 Card 消息，`mentions` 数组为空，此函数永远返回 `false`。**没有任何基于消息正文文本的 mention 检测**。

调用点在 `onMessage` 的群聊过滤逻辑中（L1126）：

```go
if chatType == "group" && !p.groupReplyAll {
    if !isBotMentioned(msg.Mentions, p.getBotOpenID(), p.getBotName()) {
        // ...
        slog.Debug(p.tag()+": ignoring group message without bot mention", "chat_id", chatID)
        return nil
    }
}
```

#### 缺陷 2：`dispatchMessage` 无 `case "interactive"` 分支

```go
// platform/feishu/feishu.go:1230-1448
switch msgType {
case "text":    // ✅
case "image":   // ✅
case "audio":   // ✅
case "file":    // ✅
case "post":    // ✅
case "merge_forward": // ✅
case "sticker": // ✅
case "media":   // ✅
default:
    slog.Debug(p.tag()+": ignoring unsupported message type", "type", msgType)  // ❌ 丢弃 interactive
}
```

即使 `isBotMentioned` 能通过，Card 消息也会在 `dispatchMessage` 被 `default` 分支丢弃。

### 完整判断链路

```
Bot1 发送 Card（含 <at> 标签或 @BotName 文本）
  → 飞书 API 接收，渲染出蓝色 @Bot2
  → 飞书生成 im.message.receive_v1 事件
  → event.mentions = []（Card 类型不填充）  ← 第一层
  → Bot2 的 cc-connect 收到事件
  → onMessage: isBotMentioned([], ...) 遍历空数组，返回 false  ← 第二层缺陷1
  → 消息在 onMessage 阶段被丢弃
  → 即使通过，dispatchMessage 也无 "interactive" case，被 default 丢弃  ← 第二层缺陷2
  → Bot2 无感知
```

### 已有修复（commit `002fca8`）— 仅覆盖发送端非流式路径

`buildReplyContent` 在检测到 `<at>` 标签时回退到 `MsgTypePost` 格式发送：

```go
// platform/feishu/feishu.go:2695-2714
func buildReplyContent(content string, useInteractiveCard bool) (msgType string, body string) {
    if !containsMarkdown(content) {
        if containsAtTag(content) {
            // use MsgTypePost so @mentions trigger bot events.
            body = buildPostMdJSON(content)
            return larkim.MsgTypePost, body
        }
    }
    if !useInteractiveCard || containsAtTag(content) {
        body = buildPostMdJSON(content)
        return larkim.MsgTypePost, body
    }
    // ...card path...
}
```

**有效范围**：非流式路径（`Reply`/`Send`/`ReplyCard`/`SendCard`）。
**未覆盖**：流式 Card 路径（`SendPreviewStart` → `UpdateMessage`），以及接收端兜底。

---

## 解决方案

**双管齐下：接收端兜底 + 发送端防护**

### 变更 1：`isBotMentioned` 增加文本内容匹配（接收端兜底）

当 `msg.Mentions` 数组无法匹配时，回退到从消息正文中检测 `@BotName` 模式。

```go
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
        if botOpenID != "" && strings.Contains(content, botOpenID) {
            return true
        }
    }
    return false
}
```

**设计考量：**
- 只对 `msgType == "interactive"` 做文本扫描，text/post 消息已有 `mentions` 数组，无需额外开销
- `extractInteractiveCardText` 已有现成实现（L1947），能解析 Schema 2.0 和 legacy 格式的 Card JSON
- 匹配 `@BotName` 覆盖 Card 渲染后文本中的 mention 显示
- 同时检查 Card JSON 中是否包含 botOpenID，覆盖 `<at id=ou_xxx></at>` 格式

### 变更 2：`dispatchMessage` 增加 `case "interactive"` 分支

```go
case "interactive":
    text := extractInteractiveCardText(content)
    text = stripMentions(text, mentions, p.getBotOpenID(), p.getBotName())
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

### 变更 3：`onMessage` 传递必要参数

```go
// 旧：
if !isBotMentioned(msg.Mentions, p.getBotOpenID(), p.getBotName()) {
// 新：
content := ""
if msg.Content != nil { content = *msg.Content }
if !isBotMentioned(msg.Mentions, p.getBotOpenID(), p.getBotName(), msgType, content) {
```

### 变更 4（可选）：发送端流式 Card 路径 mention fallback

在 `SendPreviewStart` 检测到 `<at>` 标签时返回 `ErrNotSupported`，让 Engine 回退到 Post 格式。

---

## 边界条件与风险

| 场景 | 处理方式 |
|------|----------|
| botName 为空（API 获取失败） | 文本匹配跳过，依赖 mentions 数组或发送端 fallback |
| Card 中 mention 用了 bot 别名而非注册名 | 接收端文本匹配不命中，但 openID 匹配可兜底 |
| `@BotName` 出现在代码块中 | 误匹配风险极低：Bot1 回复中 mention Bot2 是有意为之 |
| Card 中有多段文本都包含 `@BotName` | `extractInteractiveCardText` 提取所有文本段，`strings.Contains` 能检测到 |
| `@BotName` 被 strip 后消息为空 | 与 text/post 分支一致，空消息不触发 `dispatchCoreMessage` |
| `resolveMentions` 配置关闭 | `@Name` 不被转为 `<at>` 标签，但接收端文本匹配仍能检测 `@BotName` |

## 性能影响

- `extractInteractiveCardText` 在 `isBotMentioned` 中对每条 incoming interactive card 消息执行一次 JSON 解析
- 只对 `msgType == "interactive"` 且 `mentions` 为空时触发
- 群聊中 Card 消息比例低，影响可忽略

---

## 端到端验证方案

### 验证 1：Bot1 通过 Post 消息 mention Bot2（已有功能回归测试）

**前置条件**：Bot1 和 Bot2 在同一飞书群聊中，均已绑定 relay

**步骤**：
1. 用户在群聊中 @Bot1 发送任务
2. Bot1 回复中包含 `<at user_id="ou_xxx">Bot2</at>`
3. cc-connect 的 `buildReplyContent` 检测到 `<at>` 标签，回退到 `MsgTypePost` 发送
4. 飞书填充 `mentions` 数组，Bot2 收到 `im.message.receive_v1` 事件
5. Bot2 的 `isBotMentioned` 通过 OpenID 匹配成功

**预期**：Bot2 收到消息并处理（不变）

**验证方法**：
```bash
# 检查日志：Bot2 应出现 "routed inbound message" 且 session_key 包含 Bot2 的 open_id
grep "routed inbound message" /home/chenhao.magic/.cc-connect/logs/cc-connect.log | tail -5
```

### 验证 2：Bot1 通过 Interactive Card mention Bot2（核心修复验证）

**前置条件**：同上。需要通过修改代码临时绕过 `buildReplyContent` 的 Post fallback，强制走 Card 路径。

**Mock 数据（与飞书真实事件结构一致）**：

```json
{
  "event": {
    "sender": {
      "sender_id": { "open_id": "ou_6f9aff0a9ab66afe09b6dcaed8bda52b" },
      "sender_type": "app"
    },
    "message": {
      "message_id": "om_test_card_mention_001",
      "chat_id": "oc_b3699052712082aa60403044dbbeb4f9",
      "chat_type": "group",
      "message_type": "interactive",
      "content": "{\"schema\":\"2.0\",\"config\":{\"wide_screen_mode\":true},\"body\":{\"elements\":[{\"tag\":\"markdown\",\"content\":\"<at id=ou_130f3942917d99c384b393ee3310a513></at> 请评审这份设计\"}]}}",
      "mentions": []
    }
  }
}
```

**步骤**：
1. 使用上述 Mock 数据构造 `P2MessageReceiveV1` 事件
2. 调用 Bot2 的 `onMessage` 方法
3. 验证 `isBotMentioned` 通过文本匹配检测到 `@Reviewer`（botName）或 `ou_130f3942917d99c384b393ee3310a513`（botOpenID）
4. 验证 `dispatchMessage` 的 `case "interactive"` 提取文本 `请评审这份设计`
5. 验证 `dispatchCoreMessage` 被调用，消息内容正确

**预期**：
- `isBotMentioned` 返回 `true`（通过文本匹配或 openID 匹配）
- `dispatchMessage` 不走 `default` 分支
- Bot2 收到消息内容 `请评审这份设计`

### 验证 3：Bot1 发送不含 mention 的 Card（防误触发）

**Mock 数据**：

```json
{
  "event": {
    "sender": {
      "sender_id": { "open_id": "ou_6f9aff0a9ab66afe09b6dcaed8bda52b" },
      "sender_type": "app"
    },
    "message": {
      "message_id": "om_test_card_no_mention_001",
      "chat_id": "oc_b3699052712082aa60403044dbbeb4f9",
      "chat_type": "group",
      "message_type": "interactive",
      "content": "{\"schema\":\"2.0\",\"config\":{\"wide_screen_mode\":true},\"body\":{\"elements\":[{\"tag\":\"markdown\",\"content\":\"这是普通通知消息\"}]}}",
      "mentions": []
    }
  }
}
```

**预期**：
- `isBotMentioned` 返回 `false`
- 消息在 `onMessage` 阶段被过滤（"ignoring group message without bot mention"）
- Bot2 不收到消息

### 验证 4：Unit Test 覆盖

```go
func TestIsBotMentioned_InteractiveCardTextFallback(t *testing.T) {
    // Case 1: Card with @BotName in text
    mentions := []*larkim.MentionEvent{} // Card 消息 mentions 为空
    cardContent := `{"schema":"2.0","body":{"elements":[{"tag":"markdown","content":"@Reviewer 请评审"}]}}`
    assert.True(t, isBotMentioned(mentions, "ou_130f3942917d99c384b393ee3310a513", "Reviewer", "interactive", cardContent))

    // Case 2: Card with botOpenID in JSON
    cardContent2 := `{"schema":"2.0","body":{"elements":[{"tag":"markdown","content":"<at id=ou_130f3942917d99c384b393ee3310a513></at> 请评审"}]}}`
    assert.True(t, isBotMentioned(mentions, "ou_130f3942917d99c384b393ee3310a513", "", "interactive", cardContent2))

    // Case 3: Card without mention
    cardContent3 := `{"schema":"2.0","body":{"elements":[{"tag":"markdown","content":"普通通知"}]}}`
    assert.False(t, isBotMentioned(mentions, "ou_130f3942917d99c384b393ee3310a513", "Reviewer", "interactive", cardContent3))

    // Case 4: text/post still uses mentions array (no content scanning)
    textMentions := []*larkim.MentionEvent{{Id: &larkim.UserId{OpenId: ptrStr("ou_130f3942917d99c384b393ee3310a513")}}}
    assert.True(t, isBotMentioned(textMentions, "ou_130f3942917d99c384b393ee3310a513", "Reviewer", "text", ""))
}

func TestDispatchMessage_InteractiveCard(t *testing.T) {
    // 验证 case "interactive" 分支正确提取文本并 dispatch
}
```

### 验证 5：生产环境端到端测试

1. 部署修改后的 cc-connect 二进制
2. 在飞书群聊中 @Team Leader 发送任务
3. TL 回复中 @Reviewer（应走 Post fallback）
4. 确认 Reviewer 收到消息并响应
5. 如需验证 Card 路径，临时关闭 Post fallback 让 TL 以 Card 格式发送含 mention 的消息
6. 确认 Reviewer 仍然收到消息

```bash
# 部署后检查日志
grep "interactive.*mention\|mention.*interactive\|card mention" /home/chenhao.magic/.cc-connect/logs/cc-connect.log | tail -10
grep "ignoring unsupported message type.*interactive" /home/chenhao.magic/.cc-connect/logs/cc-connect.log | tail -5
# 后者应不再出现
```

---

## 关键代码位置

| 文件 | 行号 | 作用 | 修复状态 |
|------|------|------|----------|
| `platform/feishu/feishu.go` L1126 | `isBotMentioned` 调用处 | 判断是否被 mention，否则丢弃 | 待修改：传入 msgType, content |
| `platform/feishu/feishu.go` L3089 | `isBotMentioned` 定义 | 仅遍历 `mentions` 数组 | 待修改：增加 Card 文本匹配 |
| `platform/feishu/feishu.go` L1230-1448 | `dispatchMessage` switch | 无 `case "interactive"` | 待修改：增加分支 |
| `platform/feishu/feishu.go` L1947 | `extractInteractiveCardText` | Card 文本提取（已有） | 无需修改 |
| `platform/feishu/feishu.go` L2695-2728 | `buildReplyContent` | 发送端 Post fallback（已有） | 无需修改 |
| `platform/feishu/card.go` L51 | `cardContainsMention` | Card mention 检测（发送端，已有） | 无需修改 |

---

## 实现顺序

1. `isBotMentioned` 增加文本匹配（变更 1 + 变更 3）—— 接收端兜底
2. `dispatchMessage` 增加 `case "interactive"`（变更 2）—— 接收端处理
3. 补充单元测试覆盖所有场景（验证 4）
4. 端到端验证（验证 1-3, 5）
