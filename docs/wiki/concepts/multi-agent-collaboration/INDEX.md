# Multi-Agent Collaboration

基于 cc-connect 构建多 bot 自主协作系统（任务看板）的机制与实战教训。来源：[agent-work-board 项目](../../sources/agent-work-board-project.md)（2026-07-15 实测）。

## Pages

- [group-at-wake](./group-at-wake.md) — 群@唤醒：Post/Card at 触发差异、为何弃 relay、误唤醒陷阱
- [bitable-claim-token-lock](./bitable-claim-token-lock.md) — bitable 无 CAS 下的认领令牌锁 + fencing + 心跳回收
- [cron-self-check](./cron-self-check.md) — cron 自查兜底：session-key 必填、锚点 skill 劫持坑
- [board-send](./board-send.md) — bot 自有 app 身份发消息模式（tenant_token 即取即用）
- [workspace-binding](./workspace-binding.md) — 群↔工作目录绑定硬前提与动态建群初始化清单
- [shared-auth-hazard](./shared-auth-hazard.md) — 共享 lark-cli 认证互踩事故与对策
