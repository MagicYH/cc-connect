# Bot1 @mention Bot2 消息丢失 — 关键点总结

## 问题

飞书群聊中 Bot1 通过 @mention 向 Bot2 发消息，Bot2 收不到事件。

## 根因

两层缺陷叠加：

1. **飞书平台限制**：`interactive`（Card）类型消息的 `im.message.receive_v1` 事件中 `mentions` 数组为空，即使 Card 中包含 `<at>` 标签。`text`/`post` 消息不受影响。
2. **cc-connect 代码缺失**：
   - `isBotMentioned()` 仅检查 `mentions` 数组，无文本内容兜底
   - `dispatchMessage()` 无 `case "interactive"` 分支，Card 消息被 `default` 丢弃

## 代码变更（3 处核心 + 1 处增强）

### 变更 1：`isBotMentioned` 增加 Card 文本匹配（`feishu.go`）

签名从 `(mentions, botOpenID)` 扩展为 `(mentions, botOpenID, botName, msgType, content)`。当 `msgType == "interactive"` 且 `mentions` 数组未命中时，回退扫描：
- `extractInteractiveCardText(content)` 提取的文本中是否含 `@BotName`
- Card JSON 中是否含 botOpenID

### 变更 2：`dispatchMessage` 增加 `case "interactive"`（`feishu.go`）

用 `extractInteractiveCardText` 提取文本，strip mention 后 dispatch 到 Engine。空消息丢弃（与 text/post 分支行为一致）。

### 变更 3：`onMessage` 传递 msgType 和 content（`feishu.go`）

调用 `isBotMentioned` 时新增 `msgType` 和 `content` 参数，使 Card 文本匹配可用。

### 增强：`fetchBotOpenID` → `fetchBotInfo`（`feishu.go`）

同时获取 `open_id` 和 `app_name`（botName），存入 `p.botName` 字段。所有模式（WebSocket + webhook）均在启动时获取 bot 信息，用于基于名称的 mention 匹配和 strip。

### 增强：Legacy Card 格式文本提取（`feishu.go`）

`extractInteractiveCardText` 的 legacy 元素解析从仅支持 `tag:"text"` + 纯字符串 `text` 字段，扩展为也支持 `tag:"div"`/`tag:"note"` + `{"tag":"lark_md","content":"..."}` 格式。通过 `json.RawMessage` 统一处理。

### 增强：`stripMentions` 支持名称匹配（`feishu.go`）

签名从 `(text, mentions, botOpenID)` 扩展为 `(text, mentions, botOpenID, botName)`。除 openID 匹配外，也通过 `m.Name == botName` 识别并 strip bot mention。

### 移除：Card 发送端 mention fallback（`feishu.go`）

移除 `SendPreviewStart`、`UpdateMessage`、`UpdateMessageWithStatusFooter`、`SendWithStatusFooter` 中的 `containsAtTag` → Post fallback 逻辑，以及 `mentionFallback` 方法。**原因**：接收端兜底已完善，发送端强制 Post 会导致 Card 被降级为纯文本、丢失富格式和 status footer。现在 Card 中可以正常包含 `<at>` 标签，接收端通过文本匹配检测。

**保留**：`buildReplyContent`（非流式路径）的 Post fallback 不受影响，因为非流式场景无需 Card 格式。

## 诊断日志

`onMessage` 中新增 `DIAG interactive card event content` / `DIAG interactive card API content` 日志，对比事件内容和 API 获取内容（`raw_card_content`），仅对 `interactive` 类型消息触发。生产验证后可移除。

## 测试覆盖

`TestIsBotMentioned`：15 个子测试，覆盖 openID 匹配、名称匹配、Card 文本匹配（Schema 2.0 + legacy 格式）、openID 在 JSON 中匹配、无 mention 过滤、text 消息不触发内容扫描等场景。

`TestStripMentions`：新增名称匹配 strip 子测试。

全量测试 `go test ./...` 通过，0 失败。

## 风险

| 场景 | 处理 |
|------|------|
| botName 为空 | 文本匹配跳过，依赖 openID 匹配或发送端 fallback |
| Card 中 mention 用了 bot 别名 | openID 匹配兜底 |
| `@BotName` 出现在代码块 | 误匹配风险极低 |
| 移除发送端 Post fallback 后 Card 含 `<at>` | 接收端通过文本匹配检测，不再需要发送端降级 |

## 文件变更清单

| 文件 | 变更概要 |
|------|----------|
| `platform/feishu/feishu.go` | `isBotMentioned` 签名和逻辑、`dispatchMessage` 新增 interactive case、`fetchBotInfo` 替换 `fetchBotOpenID`、`stripMentions` 增加 botName 参数、legacy Card 文本提取增强、移除 Card 发送端 mention fallback、DIAG 日志 |
| `platform/feishu/feishu_test.go` | `TestIsBotMentioned` 新增 15 子测试、`TestStripMentions` 新增名称匹配子测试、签名适配 |
