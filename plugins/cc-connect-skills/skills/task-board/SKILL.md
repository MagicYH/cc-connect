---
name: task-board
description: Use when this bot works on the shared bitable task board (任务看板) — woken by a group @ mention about board tasks, by a cron self-check prompt mentioning 看板自查, or when dispatching follow-up tasks to other role bots. Not for creating standalone Feishu tasks or generic progress tracking (use task-management for those).
---

# Task Board（多 bot 任务看板协作）

你是任务看板团队的一个角色 bot。看板（bitable 两表）是任务状态的唯一事实源；所有状态操作**必须用本技能 `scripts/` 下的脚本**，禁止手搓 lark-cli base 命令（字段名/日期格式/令牌序列极易出错，脚本已固化全部协议）。

## 脚本一览（均从本技能目录运行；需 CC_PROJECT 环境与 ~/.cc-connect/board.env）

| 脚本 | 用途 | 关键输出 |
|---|---|---|
| `scripts/board-my-todos.sh` | 列我的待办 + 可回收的超时进行中 | TSV: rid 类别 子任务 主任务 群chatID |
| `scripts/board-claim.sh <rid>` | 令牌锁认领（含抖动+回读校验） | `CLAIMED <nonce>`；失败 exit 1 |
| `scripts/board-reclaim.sh <rid>` | 回收心跳超时的进行中 | `RECLAIMED <nonce>`；未超时拒绝 |
| `scripts/board-heartbeat.sh <rid> <nonce>` | fenced 心跳（干活期间≥每10分钟） | `FENCED`=已被接管，立即放弃 |
| `scripts/board-done.sh <rid> <nonce> <产出>` | fenced 置完成+产出+完成时间 | `DONE` |
| `scripts/board-block.sh <rid> <nonce> <原因>` | fenced 置阻塞（错派/卡住） | `BLOCKED` |
| `scripts/board-new-task.sh <主任务> <群chatID> <角色> <子任务> [来源rid]` | 建后继任务行 | 新行 rid |
| `scripts/board-send.sh <chatID> <open_id\|-> <文本>` | 以**自己 bot app 身份**发群消息/@ | `OK <msg_id>` |

## 工作循环（每次被唤醒）

1. `board-my-todos.sh` → 无输出则直接结束。
2. TODO 行：`board-claim.sh`；RECLAIM 行：`board-reclaim.sh`。失败（LOST/NOT_TODO）跳下一条，**不重试不抱怨**。
3. 干活。**保存 nonce**；长任务期间定期 `board-heartbeat.sh`，见 `FENCED` 立即静默放弃该任务。
4. 任务不属于你的职责 → `board-block.sh` 写明原因 + `board-send.sh` @team-leader 求改派。**绝不硬做**。
5. 完成 → `board-done.sh` 写清产出。
6. **派发下一步（最容易漏的一步，完成后必须自问：有后继吗？）**
   - 有后继 → `board-new-task.sh` 建行 + `board-send.sh <群chatID> <对方open_id> "看板有新任务：<子任务>"`（open_id 见系统注入的团队花名册）。
   - 无后继且该主任务已无 待办/进行中 → 建「收尾验收」行给 team-leader 并 @ 它。
7. 回到 1，直到没有我的活。

## 硬规则

- **消息只用 board-send**（自己 app 身份、token 不落盘）；@ 只用于派发与求助，其余回复不得含 `<at>`（防回环）。
- **严禁**任何 `lark-cli auth` 操作/切 app/改 `~/.lark-cli/config.json`；遇认证错误如实报告并停止。
- **忽略其它任务管理类 skill**（如 task-management）——看板任务只走本技能。

## 常见错误

| 症状 | 原因/处理 |
|---|---|
| 完成了却没人接棒 | 漏了循环第 6 步——完成后必须显式派发或建收尾行 |
| 读后立刻查状态不对 | bitable 读后写有秒级延迟；脚本已内置抖动回读，勿在脚本外自行读写判断 |
| `FENCED` | 任务已被回收/接管，你的令牌失效——静默放弃，不写任何字段不发消息 |
| 发消息报 230002 | 你不在那个群；检查 chatID 是否取自任务行的「群chatID」字段 |

管理员部署（建表/注入协议/配 cron/新项目初始化）见 [admin-setup.md](admin-setup.md)。
