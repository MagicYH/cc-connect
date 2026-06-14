# Fix: SendWithStatusFooter 强制纯文本消息走卡片路径

## 问题

飞书平台 `SendWithStatusFooter()` 只要 footer 非空就无条件使用 `MsgTypeInteractive`（交互式卡片）发送消息。而引擎每次 AI 回复都会生成 status footer（模型名 + 上下文百分比 + 工作目录），导致**所有消息都被强制发成卡片**，包括纯中文短文本如 "设计文档已完成但尚未送审。按照工作流，"。

纯文本消息以卡片形式发送会：
- 外观与普通聊天消息不同（带卡片边框、阴影）
- 增加视觉噪音
- 与用户日常聊天体验不一致

## 根因分析

调用链：

```
引擎完成回合
  → sendChunksWithStatusFooter()
    → p.(StatusFooterSender) → true（飞书实现了此接口）
      → p.SendWithStatusFooter(ctx, replyCtx, "设计文档已完成…", "Sonnet 4 · ctx 3% · ~/ws")
        → footer 非空 → 无条件 buildCardJSONWithStatusFooter() → MsgTypeInteractive
```

核心问题在 `platform/feishu/feishu.go:2370-2392`：
- `SendWithStatusFooter()` 注释明确写了 "Always uses the interactive card path so the footer can render with text_size: 'notation'"
- 当前唯一例外：footer 为空时回退到 `Send()`；内容含 `<at>` 标签时回退到 `Send()`
- 缺少：内容为纯文本（无 Markdown）时不应强制走卡片路径

## 方案

### 核心思路

当消息内容不含 Markdown 时，将 footer 用斜体附加到内容后面，使用 `buildPostMdJSON` 构建 `MsgTypePost`（富文本帖子）发送。Post 格式支持 Markdown 渲染（包括 `*斜体*`），且不带卡片边框。

当消息内容含 Markdown 时，保持现有卡片路径不变（卡片 schema 2.0 的 Markdown 渲染效果最佳）。

### 逻辑分支

```
SendWithStatusFooter(content, footer):
  if footer 为空 → Send(content)                           // 现有逻辑不变
  if content 含 <at> 标签 → Send(content)                  // 现有逻辑不变
  if content 不含 Markdown:
    combined = content + "\n\n*" + footer + "*"             // footer 用斜体渲染
    → MsgTypePost, buildPostMdJSON(combined)                // 走 Post 格式，无卡片边框
  else (content 含 Markdown):
    → MsgTypeInteractive, buildCardJSONWithStatusFooter()   // 现有卡片路径不变
```

### 代码变更

**文件：`platform/feishu/feishu.go`**

修改 `SendWithStatusFooter()` 方法（第 2370-2392 行）：

```go
func (p *Platform) SendWithStatusFooter(ctx context.Context, rctx any, content, footer string) error {
    if strings.TrimSpace(footer) == "" {
        return p.Send(ctx, rctx, content)
    }
    rc, ok := rctx.(replyContext)
    if !ok {
        return fmt.Errorf("%s: invalid reply context type %T", p.tag(), rctx)
    }
    content = p.resolveMentionsInContent(ctx, rc.chatID, content)
    // Card <at> tags are visual-only and don't trigger mention events.
    // Fall back to Post format so mentions work correctly.
    if containsAtTag(content) {
        slog.Info(p.tag() + ": status footer card contains mention, falling back to post format")
        return p.Send(ctx, rctx, content)
    }
    // NEW: For plain-text content, append footer as italic text via MsgTypePost
    // instead of forcing an interactive card. Post format renders Markdown (including
    // italic) without the card chrome, giving a more natural chat appearance.
    if !containsMarkdown(content) {
        combined := content + "\n\n*" + footer + "*"
        body := buildPostMdJSON(combined)
        if p.shouldUseThreadOrReplyAPI(rc) {
            return p.replyMessage(ctx, rc, larkim.MsgTypePost, body)
        }
        return p.sendNewMessageToChat(ctx, rc, larkim.MsgTypePost, body)
    }
    // Markdown content: use interactive card for best rendering (schema 2.0).
    processedBody := sanitizeMarkdownURLs(preprocessFeishuMarkdown(content))
    processedFooter := sanitizeMarkdownURLs(preprocessFeishuMarkdown(footer))
    cardJSON := buildCardJSONWithStatusFooter(processedBody, processedFooter)
    if p.shouldUseThreadOrReplyAPI(rc) {
        return p.replyMessage(ctx, rc, larkim.MsgTypeInteractive, cardJSON)
    }
    return p.sendNewMessageToChat(ctx, rc, larkim.MsgTypeInteractive, cardJSON)
}
```

### 不改动的部分

- `buildCardJSONWithStatusFooter()` — 不改，卡片路径的逻辑不变
- `buildReplyContent()` / `predictMsgType()` — 不改，`Send()` 路径不变
- `core/engine.go` 的 `sendChunksWithStatusFooter()` — 不改，接口签名不变
- `core/interfaces.go` 的 `StatusFooterSender` 接口 — 不改

## 边界条件与异常处理

1. **Footer 含 Markdown**：footer 中的 `*` 或其他 Markdown 符号在 Post 格式的 `md` tag 内会被正确解析渲染，无需特殊处理
2. **Content 含 `<at>` 标签 + 无 Markdown**：走已有的 `<at>` 标签回退路径 → `Send()` → `MsgTypePost`，footer 会通过 `appendReplyFooter` 内联追加（引擎 fallback 逻辑）
3. **Content 含复杂 Markdown（代码块/表格）**：保持卡片路径不变
4. **Footer 为纯文本**：`*纯文本*` 在 Post md tag 内渲染为斜体，效果类似卡片中 `text_size: "notation"` 的小字效果

## 风险与代价

| 风险 | 评估 | 缓解 |
|------|------|------|
| Post 格式斜体渲染效果不如卡片 notation 小字 | 低 — 视觉差异可接受，纯文本消息本身不需要精确排版 | 无需缓解 |
| Footer 含特殊字符破坏 Post 格式 | 极低 — `buildPostMdJSON` 把 content 放在 `md` tag 内，飞书会容错解析 | 无需缓解 |
| 纯文本+footer 消息超过 Post 格式长度限制 | 极低 — footer 通常 <50 字符，远小于限制 | 无需缓解 |

## 验收标准

1. **纯文本消息 + footer**：以 `MsgTypePost` 发送，不带卡片边框，footer 以斜体显示
2. **Markdown 消息 + footer**：仍以 `MsgTypeInteractive` 发送，footer 以 notation 小字显示
3. **纯文本消息 + 空 footer**：仍以 `MsgTypeText` 发送（走 `Send()` 路径）
4. **含 `<at>` 标签消息 + footer**：走回退路径（走 `Send()`）
5. **新增测试覆盖**：`TestSendWithStatusFooter_PlainTextUsesPost` 验证纯文本+footer 走 Post 路径
6. **现有测试不受影响**：`go test -race ./platform/feishu/...` 全部通过
