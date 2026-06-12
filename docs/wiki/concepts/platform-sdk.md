# Platform SDK

The pattern of using official or community Go SDK libraries to interface with messaging platform APIs.

## Project Relation

Most platform adapters use a dedicated Go SDK (Feishu, Telegram, Discord, Slack, DingTalk, LINE). Platforms without a suitable SDK (WeCom, Weixin, QQ, QQBot, Weibo, WPS Xiezuo, MAX) use `gorilla/websocket` and/or raw HTTP clients. All implement the `core.Platform` interface regardless of SDK choice.

## Cross-References

- [Feishu/Lark](../entities/feishu-lark.md)
- [gorilla/websocket](../entities/gorilla-websocket.md)
- [External Dependencies Source](../sources/external-dependencies.md)
