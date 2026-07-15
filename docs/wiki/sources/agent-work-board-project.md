# Agent Work Board Project（源）

外部项目 `agent-work-board`（`/Users/bytedance/Project/Source/MyProjects/agent-work-board`），2026-07-13 ~ 07-15。在 cc-connect 之上构建多 bot 任务看板并完成 9 项集成验证，是本 wiki「multi-agent-collaboration」概念组与 [agent-work-board-lessons](../analyses/agent-work-board-lessons.md) 的事实来源。

关键文档（该仓库内）：
- `docs/superpowers/specs/2026-07-13-agent-work-board-design.md` — 设计（含对抗性评审修订记录）
- `docs/superpowers/specs/2026-07-13-agent-work-board-verification.md` — 9 Scenario 验证计划 + 执行结果
- `docs/superpowers/specs/2026-07-13-agent-work-board-deployment.md` — 部署计划
- `board/protocol/{common,boss,team-leader}.md` — 注入 bot 的看板协议
- `board/scripts/board-send.sh` — 自有 app 身份发消息脚本（已收录进本仓库 `scripts/board-send.sh`，以本仓库版本为准）

对本代码库的核实结论（file:line 均已实测/复核）：relay 同步阻塞语义、per-session 并发模型、mention 过滤、cron session-key 强制、workspace 本地路径开关、Management API system_prompt PATCH。
