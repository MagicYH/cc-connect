# Messaging Platform

A messaging platform is a communication service (e.g., Feishu, Telegram, Discord, Slack) that cc-connect bridges to AI coding agents.

Each platform is implemented as a self-contained adapter in `platform/<name>/` that satisfies the `core.Platform` interface. Adapters handle message receiving, sending, and platform-specific features (cards, inline buttons). They register via `core.RegisterPlatform()` in `init()`.

Some platforms use dedicated SDKs (Feishu, Telegram, Discord, Slack, DingTalk, LINE); others (WeCom, QQ, Weibo) implement protocols directly over WebSocket.

Cross-references: [tech-stack](../sources/tech-stack.md), [plugin-architecture](./plugin-architecture.md), [feishu](../entities/feishu.md), [telegram](../entities/telegram.md)
