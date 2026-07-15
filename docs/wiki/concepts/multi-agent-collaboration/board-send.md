# Board Send（bot 自有 app 身份发消息）

让每个 bot 用**自己的 app_id + app_secret** 发群消息的最小脚本模式，替代共享 lark-cli 身份发消息。

## 动机（实测事故驱动）

- 所有 bot 共用一个 lark-cli 认证 store 会互踩（见 [[Shared Auth Hazard]]）；
- 共享身份发消息时，发送者若不在目标群直接报 `230002`；
- 发送者身份≠bot 自己，群内可见性与语义都不对。

## 模式

```
board-send <chat_id> <at_open_id|-> <text>
  ① CC_PROJECT 环境变量定位自己是哪个 bot（cc-connect 注入，per-project 稳定）
  ② 脚本内部从 ~/.cc-connect/config.toml 读该 bot 的 app_id/secret（agent 不经手密钥）
  ③ POST /open-apis/auth/v3/tenant_access_token/internal 换 token（内存态，用完即弃，不落盘）
  ④ POST /open-apis/im/v1/messages?receive_id_type=chat_id 发 text（可内嵌 <at>）
```

无持久化状态 → 无互踩；发送者=bot 自己 → 在群即可发。实测支撑了跨 bot 派发、@TL 收尾、boss 补步骤等全部消息路径。

注意：CC_SESSION_KEY 在共享 agent 对象上会被并发会话污染（`core/engine.go:3737-3754` SetSessionEnv），协议不得依赖它；CC_PROJECT 是 per-project 常量，安全。

Cross-references: [Shared Auth Hazard](shared-auth-hazard.md), [Group @-Mention Wake](group-at-wake.md)
