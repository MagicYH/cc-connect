# Task Board 管理员部署参考

面向部署/运维者（人或管理 agent），非角色 bot 运行时所需。

## 一次性部署

1. **建看板**：运行本技能 `scripts/board-setup.sh [看板名]`——自动建 Base 两表并写 `~/.cc-connect/board.env`（含 v1.0.33 命令面适配与半成品表告警）。
3. **注入协议**：经 Management API `PATCH /api/v1/projects/{name}` 的 `system_prompt` 增量追加（勿覆盖现有 @ 纪律），重启 daemon 生效。
4. **配置硬前提**（每个角色 bot 的 `[[projects]]`）：`workspace_init_allow_local_paths = true`；`allow_chat`/`allow_from` 留空；`resolve_mentions = false`。
5. **防停滞巡检**：运行本技能 `scripts/board-setup-cron.sh`（安装 crontab：`board-watchdog.sh` 以 Boss 身份每 30 分钟巡检，催办直接推到各任务的工作群 @责任 bot）。前提：Boss bot app 在每个工作群里（`board-init-project.sh` 建群已自动拉入；老群手动拉）。**旧方案（每角色一条 LLM 自查 cron 锚定固定群）已废弃**——会话锚在固定群导致回复落错群、workspace 错绑；残留的「看板自查-*」cron 请用 `cc-connect cron` 删除。

## 每个新项目初始化（Boss 流程）

1. `lark-cli im +chat-create --bots <角色app_id逗号分隔> --users <发起人open_id>` 建群。
2. 立即写 Projects 行（项目状态=初始化中）。
3. **逐个 @ 每个角色 bot** 发 `/workspace init <工作目录绝对路径>`（缺这步 bot 被 @ 只回 "No workspace found" 不干活）。
4. 全部成功 → 项目状态=进行中，@team-leader 起步；失败 → 项目状态=初始化失败+备注，不留孤儿。

## 已知坑速查

详见 cc-connect 仓库 `docs/wiki/concepts/multi-agent-collaboration/`（群@唤醒、令牌锁、cron 劫持、共享认证互踩、workspace 绑定）。
