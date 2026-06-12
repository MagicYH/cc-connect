# Gorilla WebSocket

`gorilla/websocket` (v1.5.0) is the WebSocket library used across cc-connect for real-time bidirectional communication.

It is used by multiple platform adapters (wecom, weibo, qq, wps-xiezuo) and in `core/bridge.go` for the core bridging mechanism.

Also used by the QQBot platform adapter. Not used by platforms with SDK-provided WebSocket (Feishu, Slack, DingTalk, Discord).

Cross-references: [tech-stack](../sources/tech-stack.md), [messaging-platform](../concepts/messaging-platform.md), [project-structure](../sources/project-structure.md), [External Dependencies Source](../sources/external-dependencies.md), [Platform SDK](../concepts/platform-sdk.md)

From [project-structure](../sources/project-structure.md): One of only two external dependencies allowed in `core/`.
