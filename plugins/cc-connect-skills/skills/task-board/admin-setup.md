# Task Board 管理员部署参考

面向部署/运维者（人或管理 agent），非角色 bot 运行时所需。

## 一次性部署

1. **建看板**：运行本技能 `scripts/board-setup.sh [看板名]`——自动建 Base 两表并写 `~/.cc-connect/board.env`（含 v1.0.33 命令面适配与半成品表告警）。
3. **注入协议**：经 Management API `PATCH /api/v1/projects/{name}` 的 `system_prompt` 增量追加（勿覆盖现有 @ 纪律），重启 daemon 生效。
4. **配置硬前提**（每个角色 bot 的 `[[projects]]`）：`workspace_init_allow_local_paths = true`；`allow_chat`/`allow_from` 留空；`resolve_mentions = false`。
5. **自查 cron**：运行本技能 `scripts/board-setup-cron.sh feishu:<锚点群chatID>`（默认四角色、*/30）。**锚点群的 workspace 不得含其它任务管理 skill**（会劫持）；锚点勿用生产群（过程卡片会发进去）。

## 每个新项目初始化（Boss 流程）

1. `lark-cli im +chat-create --bots <角色app_id逗号分隔> --users <发起人open_id>` 建群。
2. 立即写 Projects 行（项目状态=初始化中）。
3. **逐个 @ 每个角色 bot** 发 `/workspace init <工作目录绝对路径>`（缺这步 bot 被 @ 只回 "No workspace found" 不干活）。
4. 全部成功 → 项目状态=进行中，@team-leader 起步；失败 → 项目状态=初始化失败+备注，不留孤儿。

## 已知坑速查

详见 cc-connect 仓库 `docs/wiki/concepts/multi-agent-collaboration/`（群@唤醒、令牌锁、cron 劫持、共享认证互踩、workspace 绑定）。
