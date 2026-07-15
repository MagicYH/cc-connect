# Agent Work Board：多 bot 任务看板实战复盘

基于 cc-connect + Lark bitable 构建"多 bot 自主协作任务看板"的完整实战（设计→对抗性评审→部署→9 项集成验证全过，2026-07-13 ~ 07-15，dev-sg 环境）。**零 cc-connect 核心代码改动**——纯 bitable schema + system_prompt 协议 + cron/config 编排。

## 系统形态

- **看板**（bitable 两表 Projects/Tasks）= 任务状态唯一事实源；
- **[[Group @-Mention Wake]]** = bot→bot 派发唤醒（弃 relay：同步 120s 阻塞不适合长任务）；
- **[[Bitable Claim Token Lock]]** = 并发防重复认领 + fencing + 心跳回收；
- **[[Cron Self-Check]]** = 每 bot 半小时兜底防停滞；
- **[[Board Send]]** = bot 自有 app 身份发消息；
- 角色：boss（建项目）→ team-leader（拆任务/收尾）→ developer/tester/reviewer（执行接力）。

## 验证结果（9/9 通过）

S1 建表 / S2 令牌锁真并发 / S3 Post@ 唤醒 / S3b Card@ 负例 / S4 cron 兜底 / S5 心跳回收 / S6 Boss 初始化 / S7 端到端（TL→developer 写 hello.txt→tester 字节级验证→项目收尾）/ S8 纠错闭环（错派→阻塞+说明→改派→规范接手）。

## 部署硬前提（每条都是实测踩坑）

1. 每群须 `/workspace init`，且 bot 配置 `workspace_init_allow_local_paths=true`（[[Workspace Binding]]）；
2. cron 锚点群 workspace 不得含竞争任务 skill，prompt 写死数据源（[[Cron Self-Check]]）；
3. 协议注入 lark-cli auth 硬禁令（[[Shared Auth Hazard]]）；
4. 发消息一律 [[Board Send]]；角色 bot `allow_chat`/`allow_from` 留空、`resolve_mentions=false`。

## 有效的工程实践

- **对抗性评审拿代码说话**：三个致命设计假设（relay 可当异步唤醒 / 软锁足够 / P2P 并发可测锁）全部在实现前被"对着源码核实"推翻；
- **Management API PATCH `/api/v1/projects/{name}` 的 `system_prompt`** 比手改 27KB TOML 安全得多（持久化+可幂等重放，需重启生效）；
- **协议注入采用增量追加**，不覆盖 bot 现有 @ 纪律/防回弹规则。

## 遗留改进项

- 跨角色自动派发执行率不稳（agent 完成后"建后继行+@下一角色"偶尔漏做，需人工推动）——prompt 遵从度问题，候选方案：TL 巡检 cron 或后继派发写进 cron 自查口径；
- TL 自建自做路径绕过认领协议（认领人为空）；
- boss 遇错不自主重试。

## 产物位置

设计/验证/部署/计划四份文档与协议产物在 `agent-work-board` 仓库（`docs/superpowers/specs/2026-07-13-agent-work-board-*.md`、`board/`）；看板 Base `JYZqbSKrTasFK2sN6OamX5rByZb`；bot 配置在 dev-sg `~/.cc-connect/config.toml`。

Cross-references: [Feishu Mention + Slash Command Failure](feishu-mention-slash-cmd.md), [Engine](../concepts/engine.md), [CronScheduler](../concepts/cron-scheduler.md)
