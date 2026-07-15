# Task Board 管理员部署参考

面向部署/运维者（人或管理 agent），非角色 bot 运行时所需。

## 一次性部署

1. **建看板**：Base 两表 Projects/Tasks（字段 schema 见 agent-work-board 仓库 `board/schema/create-board.sh`；type 用字符串 text/select/datetime/created_at，select 选项用平级 `options` 键；建表失败会留半成品表，重跑前先删同名残留）。
2. **落配置**：执行机写 `~/.cc-connect/board.env`（BOARD_BASE/TBL_PROJECTS/TBL_TASKS 三行）。
3. **注入协议**：经 Management API `PATCH /api/v1/projects/{name}` 的 `system_prompt` 增量追加（勿覆盖现有 @ 纪律），重启 daemon 生效。
4. **配置硬前提**（每个角色 bot 的 `[[projects]]`）：`workspace_init_allow_local_paths = true`；`allow_chat`/`allow_from` 留空；`resolve_mentions = false`。
5. **自查 cron**：每 bot 一条 `cc-connect cron add -p <bot> -s <锚点sessionKey> --cron "*/30 * * * *" --session-mode new-per-run --silent --prompt "看板自查：用 task-board 技能扫描并处理你的任务"`。**锚点群的 workspace 不得含其它任务管理 skill**（会劫持）；锚点勿用生产群（过程卡片会发进去）。

## 每个新项目初始化（Boss 流程）

1. `lark-cli im +chat-create --bots <角色app_id逗号分隔> --users <发起人open_id>` 建群。
2. 立即写 Projects 行（项目状态=初始化中）。
3. **逐个 @ 每个角色 bot** 发 `/workspace init <工作目录绝对路径>`（缺这步 bot 被 @ 只回 "No workspace found" 不干活）。
4. 全部成功 → 项目状态=进行中，@team-leader 起步；失败 → 项目状态=初始化失败+备注，不留孤儿。

## 已知坑速查

详见 cc-connect 仓库 `docs/wiki/concepts/multi-agent-collaboration/`（群@唤醒、令牌锁、cron 劫持、共享认证互踩、workspace 绑定）。
