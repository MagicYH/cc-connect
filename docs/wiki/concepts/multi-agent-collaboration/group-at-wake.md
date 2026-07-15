# Group @-Mention Wake（群@唤醒）

多 bot 协作中"叫醒下一个 bot"的推荐通道：向共同所在的群发一条**文本(Post)消息**、内嵌 `<at user_id="对方open_id"></at>`。异步、非阻塞、群内天然可见、按 chatID 显式定址。

## 代码事实（实测验证 2026-07-15）

- 群消息与 P2P 共用 handler：`platform/feishu/feishu.go:428 OnP2MessageReceiveV1`，`:1186` 分支 `chatType=="group"`。
- 默认**被 @ 才唤起**：`:1191 isBotMentioned`（`:3468` 靠 `msg.Mentions` 匹配各 bot 自己的 open_id）；未被 @ 的 bot 在 `:1205` 丢弃——同群多 bot 不会广播唤醒。
- **Post/text 的 `<at>` 触发接收方 bot 事件；Interactive Card 里的 `<at>` 只是视觉渲染，飞书不推送事件，对方收不到**（负例实测：卡片 @ 后 4 分钟目标 bot 无反应）。
- bot→bot 的 text @ 可正常触发（实测：mention 注册为对方 app_id，接收方被唤起并完成任务）。

## 为什么不用 relay

`relay send` 是**同步 RPC、默认 120s 超时**（`config/config.go` RelayConfig.timeout_secs；阻塞语义 `core/engine.go` HandleRelay "blocks until the complete response is collected"）。真实任务耗时以分钟计 → 每次派发必然超时、调用方被卡、链式嵌套阻塞。relay 适合秒级问答，不适合长任务派发。

## 陷阱

- 发送者必须在目标群里，否则 API 报 `230002 Bot/User can NOT be out of the chat`——用 [[Board Send]] 让每个 bot 以自己身份发送可根治。
- `resolve_mentions=true` 会把回复里随口的 "@名字" 转成真 at 造成误唤醒，多 bot 群建议置 false。
- 相关失败模式：[Feishu Mention + Slash Command Failure](../../analyses/feishu-mention-slash-cmd.md)。

Cross-references: [Board Send](board-send.md), [Bitable Claim Token Lock](bitable-claim-token-lock.md), [Engine](../engine.md)
