# QQ

QQ is a messaging platform by Tencent. In cc-connect, it is one of the supported messaging platforms (`platform/qq/`).

The QQ adapter has no dedicated SDK; it communicates over raw WebSocket using `gorilla/websocket`.

Cross-references: [tech-stack](../sources/tech-stack.md), [messaging-platform](../concepts/messaging-platform.md), [project-structure](../sources/project-structure.md)

From [project-structure](../sources/project-structure.md): 800 lines. A separate `platform/qqbot/` package (1,792 lines) implements the newer QQ Bot protocol.
