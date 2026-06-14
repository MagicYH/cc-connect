# 根因分析：Bot1 通过卡片 mention Bot2 不生效

## 问题现象

Bot1（如 Claude Code）在群聊中通过 Interactive Card 消息 mention Bot2（如 Gemini），Bot2 收不到任何事件，无法被触发。

## 当前代码的两层防御机制

### 第一层：`isBotMentioned` —— 匹配 `msg.Mentions` 数组（OpenID + Name 双匹配）

当前代码（含未提交修改）的 `isBotMentioned` 函数：

```go
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

它对 `msg.Mentions` 数组做了**双重匹配**：
1. 按 `OpenID` 匹配 `MentionEvent.Id.OpenId`
2. 按 `botName` 匹配 `MentionEvent.Name`

**但这两重匹配都依赖飞书 SDK 填充 `msg.Mentions` 数组。** 对于 Interactive Card 消息，飞书**不填充**此数组，所以无论 OpenID 匹配还是 Name 匹配都无效。

### 第二层：`dispatchMessage` 的 `case "interactive"` —— **不存在**

`dispatchMessage` 的主 switch 处理了 `text`、`post`、`image`、`file`、`audio`、`media`、`merge_forward`、`sticker` 等类型，但**没有 `case "interactive"`**。Incoming card 消息走 `default` 被丢弃：

```go
default:
    slog.Debug(p.tag()+": ignoring unsupported message type", "type", msgType)
```

注：代码中确有 `case "interactive"` 在 L1725，但那是 `fetchQuotedMessage`（引用消息解析）里的，不是 `dispatchMessage` 主流程。

### 结论：两层防御都不覆盖 Card mention 场景

| 防御层 | 机制 | 对 Card 消息是否生效 | 原因 |
|--------|------|---------------------|------|
| 第一层 | `isBotMentioned` 匹配 `mentions[]` | ❌ | 飞书对 Card 不填充 `mentions`，数组为空，无从匹配 |
| 第二层 | `dispatchMessage` 处理入站消息 | ❌ | 没有 `case "interactive"`，Card 入站消息直接被 `default` 丢弃 |

**代码里没有基于消息文本内容的 mention 匹配。** `isBotMentioned` 只遍历 `mentions []*larkim.MentionEvent` 数组，不会解析消息正文查找 `@BotName` 模式。

## 飞书平台限制——根因

飞书 `im.message.receive_v1` 事件的行为因消息类型而异：

| 消息类型 | `<at>` 渲染 | `mentions` 字段填充 | 触发 Bot 事件 |
|----------|-------------|---------------------|---------------|
| `text`   | ✅          | ✅                  | ✅            |
| `post`   | ✅          | ✅                  | ✅            |
| `interactive` (Card) | ✅ 仅视觉渲染 | ❌ **不填充** | ❌ **不触发** |

即使 Card 中写了 `<at user_id="ou_xxx">Bot2</at>`，飞书返回的事件中 `msg.Mentions` 为空。

## 完整判断链路

```
Bot1 发送 Card（含 <at> 标签或 @BotName 文本）
  → 飞书 API 接收，渲染出蓝色 @Bot2
  → 飞书生成 im.message.receive_v1 事件
  → event.mentions = []（Card 类型不填充）
  → Bot2 的 cc-connect 收到事件
  → onMessage: isBotMentioned(msg.Mentions, ...) 遍历空数组，返回 false
  → 消息在 onMessage 阶段即被丢弃（"ignoring group message without bot mention"）
  → 即使不被 onMessage 丢弃，dispatchMessage 也无 "interactive" case
  → Bot2 无感知
```

## 已有的修复尝试及现状

### 已提交代码（commit `002fca8`）——发送端 fallback

在 `buildReplyContent`、`ReplyCard`/`SendCard`、Engine rich card path 中，检测到 `<at>` 标签时回退到 `MsgTypePost` 格式发送，避开 Card 的 mention 失效问题。

**有效范围：** 非流式路径（Reply/Send/ReplyCard/SendCard）。
**未覆盖：** 流式 Card 路径（`SendPreviewStart` → `UpdateMessage`）。

### 未提交代码（working tree）——策略反转

将发送端 fallback 策略**全部回退**，改为在接收端做**基于 `MentionEvent.Name` 的匹配**：
- `isBotMentioned()` 增加 `botName` 匹配（匹配 `mentions` 数组中的 `Name` 字段）
- `stripMentions()` 增加 `botName` 匹配
- 移除所有 `containsAtTag()` 检查和 `mentionFallback()` 逻辑

**问题：** 名字匹配仍然只作用于 `msg.Mentions` 数组。飞书对 Interactive Card 不填充此数组，所以名字匹配同样无从谈起。同时 `dispatchMessage` 仍然没有 `case "interactive"` 分支。

## 要彻底修复需要做什么

**核心缺口：缺少基于消息正文文本的 mention 检测。** 当前代码没有任何路径会解析 Card 消息正文查找 `@BotName`。

修复方案需包含：

1. **接收端**：`onMessage` 或 `dispatchMessage` 中，当 `msg.Mentions` 为空且消息类型为 `interactive` 时，从 Card 内容提取文本，做 `@BotName` 文本模式匹配
2. **接收端**：`dispatchMessage` 增加 `case "interactive"` 分支，用 `extractInteractiveCardText()` 提取文本后走正常处理流程
3. **发送端**（可选但推荐）：流式 Card 路径中，检测到 `<at>` 标签时回退到 Post 格式

## 关键代码位置

| 文件 | 行号 | 作用 |
|------|------|------|
| `platform/feishu/feishu.go` L1077-1096 | `isBotMentioned` 调用处 | 判断是否被 mention，否则丢弃 |
| `platform/feishu/feishu.go` L3041-3051 | `isBotMentioned` 定义 | 仅遍历 `mentions` 数组，不扫描正文 |
| `platform/feishu/feishu.go` L1182-1400 | `dispatchMessage` switch | 无 `case "interactive"` |
| `platform/feishu/feishu.go` L1899+ | `extractInteractiveCardText` | Card 文本提取（仅用于引用解析） |
| `platform/feishu/feishu.go` L2647+ | `buildReplyContent` | 非 Card 路径的 mention fallback |
| `platform/feishu/card.go` L51+ | `cardContainsMention` | Card 路径的 mention 检测（发送端） |
| `platform/feishu/feishu.go` L3983+ | `SendPreviewStart` | 流式 Card 发送，未提交代码移除了 mention 检测 |
| `platform/feishu/feishu.go` L4194+ | `UpdateMessage` | 流式 Card 更新，未提交代码移除了 mention 检测 |
