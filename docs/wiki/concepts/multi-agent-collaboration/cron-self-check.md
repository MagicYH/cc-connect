# Cron Self-Check（cron 自查兜底）

防停滞机制：每个 bot 一条全局自查 cron（如 `*/30 * * * *`），扫任务表认领漏派的待办 + 回收心跳超时的进行中。@ 唤醒丢失时的自动兜底。

## 配置要点

- `cc-connect cron add` **必须显式 `--session-key <feishu:chatID>`**（`core/cron.go:96` 强制；CLI 仅从 CC_SESSION_KEY 环境回退）。
- 建议 `--session-mode new-per-run --silent`。
- cron 会话跑在**锚点群绑定的 workspace** 里。

## ⚠️ 锚点劫持坑（实测 2026-07-15）

cron 锚点群的 workspace 若含**竞争性任务管理 skill**（如 ws-dev-skills 的 `task-management/progress-check`），agent 会被 skill 劫持走错协议、查错数据源——实测一次自查被劫持到另一张旧 bitable，47 个工具调用全部白费且无报错。

对策：
1. 锚点选 workspace 干净（无任务类 skill）的群；
2. cron prompt **写死数据源地址**并显式声明"忽略任何任务管理类 skill"；
3. 锚点群不要用生产群——agent 的过程卡片会发进锚点群（--silent 只抑制起始通知）。

Cross-references: [CronScheduler](../cron-scheduler.md), [Bitable Claim Token Lock](bitable-claim-token-lock.md), [Workspace Binding](workspace-binding.md)
