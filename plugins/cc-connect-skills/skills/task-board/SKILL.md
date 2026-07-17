---
name: task-board
description: Use when this bot works on the shared bitable task board (任务看板) — woken by a group @ mention about board tasks (e.g. 看板有新任务 / 看板催办 / 看板自查), or when dispatching follow-up tasks to other role bots. Not for creating standalone Feishu tasks or generic progress tracking (use task-management for those).
---

# Task Board（多 bot 任务看板协作）

你是任务看板团队的一个角色 bot。看板（bitable 两表）是任务状态的唯一事实源；所有状态操作**必须用本技能 `scripts/` 下的脚本**，禁止手搓 lark-cli base 命令（字段名/日期格式/令牌序列极易出错，脚本已固化全部协议）。

## 脚本一览（均从本技能目录运行；需 CC_PROJECT 环境与 ~/.cc-connect/board.env）

| 脚本 | 用途 | 关键输出 |
|---|---|---|
| `scripts/board-my-todos.sh` | 列我的待办 + 可回收的超时进行中 | TSV: rid 类别 子任务 主任务 工作群 |
| `scripts/board-claim.sh <rid>` | 令牌锁认领（含抖动+回读校验） | `CLAIMED <nonce>`；失败 exit 1 |
| `scripts/board-reclaim.sh <rid>` | 回收心跳超时的进行中 | `RECLAIMED <nonce>`；未超时拒绝 |
| `scripts/board-heartbeat.sh <rid> <nonce>` | fenced 心跳刷新（**可选**：心跳已由 Boss 从群消息自动推导，通常无需你手动发） | `FENCED`=已被接管，立即放弃 |
| `scripts/board-done.sh <rid> <nonce> <产出> ( --next <角色> <子任务> \| --last )` | fenced 置完成，**并强制交代下一步**：--next 自动建后继行+@唤醒；--last 自动判断是否建收尾行给 TL | `DONE` + `NEXT/CLOSEOUT <rid>` |
| `scripts/board-block.sh <rid> <nonce> <原因>` | fenced 置阻塞（错派/卡住） | `BLOCKED` |
| `scripts/board-new-task.sh <主任务> <工作群> <角色> <子任务> [来源rid]` | 建后继任务行 | 新行 rid |
| `scripts/board-send.sh <chatID> <open_id\|-> <文本>` | 以**自己 bot app 身份**发群消息/@ | `OK <msg_id>` |
| `scripts/board-init-project.sh <项目名> <需求> [目录]` | **Boss/TL 专用**：开新项目一条命令完成 建群+拉人+项目行+workspace绑定+@TL 起步 | `PROJECT_READY <chat_id>` |
| `scripts/board-complete-project.sh <主任务名>` | **TL 收尾专用**：Projects 项目行置已完成+完成时间（重名报 AMBIGUOUS 防误更） | `PROJECT_DONE <rid>` |
| `scripts/board-watchdog.sh` | **Boss 定时专用（cc-connect cron，webui 可见）**：防停滞巡检——从群消息自动维护心跳 + 把催办直接推到各任务工作群（bot 无需调用） | `WATCHDOG_DONE` |

## 工作循环（每次被唤醒）

> **看板 = 跨角色协作的交接，不是你自己的 todo。** 同一角色的连续多步（如 developer 的多步开发），在**一个已认领的任务里一次做完**，别拆成多个自建任务反复 @ 激活自己（既慢又吵）。只有把活交给**别的角色**、或需**停下等外部**（设计立项 / 等人类确认）时，才走看板建任务/`--next`。

1. `board-my-todos.sh` → 无输出则直接结束。
2. TODO 行：`board-claim.sh`；RECLAIM 行：`board-reclaim.sh`。失败（LOST/NOT_TODO）跳下一条，**不重试不抱怨**。
3. 干活。**保存 nonce**（`board-done` 收口要用）。**无需定时发心跳**——Boss 巡检会把你在群里的最近消息时间自动写成心跳，只要你在群里有产出/汇报就不会被误判超时；若某次操作返回 `FENCED`（任务已被回收/接管），立即静默放弃。
4. 任务不属于你的职责 → `board-block.sh` 写明原因 + `board-send.sh` @team-leader 求改派。**绝不硬做**。
（team-leader 处理收尾验收任务时：验收通过后先 `board-complete-project.sh <主任务名>` 完成项目行，再对收尾任务行执行第 5 步的 `--last`。）
5. **你这一整段活全部做完**再用 `board-done.sh` 收口，带 `--next <别的角色> <子任务>`（交接给下一个角色）或 `--last`（没有后续角色了）——脚本自动建后继行/收尾行并 @ 唤醒，不带参数会报错。**后续若还是你自己的活，别 board-done、别给自己建任务——在当前任务里继续做完。** `--next` 的 `<角色>` 正常应是**别的角色**；给自己（同角色）仅限"停下等外部"（设计立项 / 等人类确认）场景。
6. 回到 1，直到没有我的活。

## 项目启动·设计先行（team-leader 专属）

被新项目 kickoff @（消息含"新项目"）或收到整块新需求时，**禁止直接给 developer 建任务**，先走设计阶段：

1. 自建设计任务并认领：`board-new-task.sh <主任务> <工作群> team-leader "需求分析与技术设计"` → `board-claim.sh`。
2. 评估复杂度，满足任一即**复杂**：需要架构/技术选型；预计子任务 >3 个；跨多模块或服务；需求含糊、有关键取舍需发起人定夺。否则**简单**。
3. **简单**：在工作目录写 `docs/design.md`（需求理解 / 方案 / 任务拆解 / 各任务验收标准）→ `board-send.sh` 向工作群公示设计要点+文档路径（不 @）→ `board-done.sh <rid> <nonce> docs/design.md --next developer "<首个开发子任务>"` 直接开工。
4. **复杂**：调用 superpowers 技能链（brainstorming → writing-plans，自主推进；歧义与关键取舍**列成问题清单写进设计**，不臆测）产出设计与计划文档 → `board-done.sh <rid> <nonce> <设计文档路径> --next team-leader "待发起人<open_id>确认设计后拆解派发（设计=<路径>）"` → `board-send.sh <工作群> <发起人open_id> "<设计要点+路径+问题清单，请确认后开工>"`。发起人 open_id 取自 kickoff 消息。
5. **确认跟进任务规则**（子任务含「待发起人…确认」的行）：
   - 本次唤醒消息就是发起人的回复 → 认领：确认则 `--next developer <首个开发子任务>` 开工；有修改意见则按意见修订设计后重复第 4 步收尾。
   - 被看板催办消息唤醒时遇到它 → **不认领不心跳**（watchdog 巡检会直接催发起人，无需你转达）。

> **拆解粒度（TL 拆计划时）**：给同一角色的连续步骤**合并成一个看板任务**（如把 plan 的 Task1–5 作为**一行** developer 任务，让 developer 在这一个任务内连续做完），**不要一个 plan-step 建一行**再让它反复自我激活。看板行只在**换角色**时才新增。（设计立项那行是本节唯一允许的同角色任务——它是设计阶段看板上的把手，供 watchdog 兜底。）

## 硬规则

- **消息只用 board-send**（自己 app 身份、token 不落盘）；@ 只用于派发与求助，其余回复不得含 `<at>`（防回环）。
- **派发用角色键**：`--next`/`new-task` 的 `<角色>` 只能是 `team-leader`/`developer`/`tester`/`reviewer`（**角色键，不是 Bot 显示名**如 Gamma/Delta；人类说「@Gamma」时你要翻译成 `reviewer`）。脚本已归一化并对无法识别的值报错。`--next` 正常派**别的角色**（交接）；给自己（同角色）**仅限设计立项 / 等人类确认这类"停下等外部"场景**（不是把自己的连续开发拆成多任务），脚本对同角色不 @ 自己。
- **严禁**任何 `lark-cli auth` 操作/切 app/改 `~/.lark-cli/config.json`；遇认证错误如实报告并停止。
- **忽略其它任务管理类 skill**（如 task-management）——看板任务只走本技能。

## 常见错误

| 症状 | 原因/处理 |
|---|---|
| 完成了却没人接棒 | 漏了循环第 6 步——完成后必须显式派发或建收尾行 |
| 读后立刻查状态不对 | bitable 读后写有秒级延迟；脚本已内置抖动回读，勿在脚本外自行读写判断 |
| `FENCED` | 任务已被回收/接管，你的令牌失效——静默放弃，不写任何字段不发消息 |
| 发消息报 230002 | 你不在那个群；检查 chatID 是否取自任务行的「工作群」字段 |
| 同一任务被建了两条 | 建行后**勿回查复核勿重试**——脚本输出 rid 即成功；读后写延迟会让复核看不到刚建的行 |

管理员部署（建表/注入协议/配 cron/新项目初始化）见 [admin-setup.md](admin-setup.md)。
