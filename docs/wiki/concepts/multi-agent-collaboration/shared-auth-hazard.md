# Shared Auth Hazard（共享 lark-cli 认证互踩）

多个自主 agent 共用同一份 lark-cli 认证 store（`~/.lark-cli/config.json`）时，任何一个 agent 动认证都会破坏所有人。**实测事故（2026-07-15）**：boss agent 执行任务遇到身份问题，自作主张把自己的 app 注册进 lark-cli 并切换 `currentApp`，导致 user 登录整体失效，全部 bot 的看板读写与 cron 扫描静默失败约 40 分钟。

## 教训与对策

1. **协议硬禁令（最高优先级注入所有 bot）**：严禁 `lark-cli auth login/logout`、注册 app、切 currentApp、改 `~/.lark-cli/config.json`；遇认证错误如实报告并停止，绝不自行"修复"。
2. **发消息路径与共享认证解耦**：用 [[Board Send]]（bot 自有 app + tenant_token 即取即用）。
3. **恢复方法**：lark-cli 的 config.json 按 app 分条保存 user token，`currentApp` 被切走时原 app 的登录通常还在——切回 `currentApp` 即恢复，无需重新授权。
4. 诊断特征：user 身份调用报 `need_user_authorization (user: )`（user 字段为空），`auth status` 显示 "No user logged in" 或 appId 变化。

Cross-references: [Board Send](board-send.md), [Token Redaction](../token-redaction.md)
