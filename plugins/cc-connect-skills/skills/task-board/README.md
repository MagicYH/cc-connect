# task-board 使用说明

让一组 cc-connect 飞书 bot（如 team-leader / developer / tester / reviewer）围绕一张共享 bitable 任务看板自主协作：领任务、干活、跨角色派发、防停滞兜底。协议中所有易错序列（令牌锁、fencing、日期格式）都固化在 `scripts/` 里，bot 只需按 `SKILL.md` 的工作循环调脚本。

## 文件导航

| 文件 | 给谁看 | 内容 |
|---|---|---|
| `SKILL.md` | 角色 bot（运行时自动加载） | 工作循环、脚本速查、硬规则 |
| `README.md`（本文件） | 人/管理员 | 从零到跑通的完整步骤 |
| `admin-setup.md` | 管理员 | 部署硬前提细节与坑速查 |
| `scripts/` | bot 与管理员 | 9 个运行时脚本 + 2 个部署脚本 |

## 从零部署（管理员，执行机上跑）

前提：`lark-cli` 已 `auth login`（user 身份，含 base+im scope）；cc-connect daemon 在跑；`jq` 可用。

```bash
cd plugins/cc-connect-skills/skills/task-board/scripts

# 1) 建看板（Base + Projects/Tasks 两表），自动写 ~/.cc-connect/board.env
./board-setup.sh "我的任务看板"

# 2) 各角色 bot 的 [[projects]] 配置确认两件事，然后重启 daemon：
#    allow_chat / allow_from 留空；resolve_mentions = false
#    （workspace 绑定走 /workspace route，无需 workspace_init_allow_local_paths）
cc-connect daemon restart

# 3) 安装防停滞巡检（注册 cc-connect cron，webui 可见/可管；board-watchdog.sh 以 Boss 身份周期运行，
#    催办直接推到各任务的工作群 @责任 bot——回复与 workspace 都在正确的群）
#    先在 board.env 配 BOSS_SESSION_KEY=feishu:oc_xxx（cc-connect sessions list 取 boss 会话的群）
./board-setup-cron.sh

# 4) 编辑 ~/.cc-connect/board.env 里的 BOT_LABEL_* 为你实际的 Bot 应用名
#    （决定「角色/认领人」显示为 role(Bot名)，且需与看板「角色」select 选项一致）
# 5) 给各角色 bot 的 system_prompt 追加一句（经 Management API PATCH，见 admin-setup.md）：
#    "看板任务一律使用 task-board 技能处理"
```

## 每个新项目的初始化

1. 建项目群、拉齐角色 bot；
2. **逐个 @ 每个 bot** 发 `/workspace route <工作目录绝对路径>`（绑定已存在的绝对目录，可为空目录或已有 git 仓库；漏了 bot 不干活）；
3. 写 Projects 行（主任务名/工作群/状态=进行中）；
4. 在群里 @team-leader 起步。

## 创建新任务（三种方式）

**方式一：人在看板 UI 直接加行**（最直观）
打开看板 Base → Tasks 表加一行，填五个字段：
- 主任务 = 项目名（与 Projects 表「主任务名」一致）
- 工作群 = 点选对应项目群（Group 字段，可点击跳群）
- 角色 = 选执行者，如 `Delta (developer)`
- 子任务 = 具体要做的事；必须写清 `任务目标`、`需求依据`、`上游产出路径`、`验收标准`、`预期产出文档`
- 状态 = `待办`

然后二选一唤醒：在项目群里 @ 对应 bot 说"看板有新任务"；或什么都不做，等 cron 自查（≤30 分钟）自动认领。

**方式二：bot 在会话里建**（bot 间接力派发即此路径）
```bash
scripts/board-new-task.sh <主任务> <工作群> <角色> "任务目标=<目标>；需求依据=<路径>；上游产出路径=<路径>；验收标准=<检查>；预期产出文档=<路径>" [来源rid]
scripts/board-send.sh <工作群> <对方open_id> "看板有新任务：请先读 task-board 上下文再开工"
```
角色传纯名（如 `tester`）即可，脚本自动映射为 `Zero (tester)` 标签。

**方式三：让 team-leader 拆解**（只有需求、没想好任务时）
在项目群 @team-leader 描述需求，它会先做需求分析与技术设计（SKILL.md『项目启动·设计先行』）：简单需求产出 `.board/design.md`（工作区内的中间产物，不进 git、收尾自动清理）公示后直接拆解开工；复杂需求走 superpowers 设计链并 @你确认设计后才开工。

**方式四：对 Boss 说一句话开全新项目**（自动建新工作群）
在管理群 @boss："开新项目：<项目名>，需求=<一句话需求>"。Boss 会运行 `board-init-project.sh` 自动完成：建新群、拉齐角色 bot 与你、写项目行、绑定 workspace、@team-leader 起步——每个项目一个独立工作群。TL 起步后同方式三：先设计再派发，复杂项目会等你确认设计。

## 日常使用

- **人工触发**：群里 @ 某 bot 说"检查看板"；
- **兜底**：watchdog 巡检（cc-connect cron，默认每 10 分钟，在 webui 可见/可管）扫全表——漏派的待办、心跳超时的进行中、阻塞行都会在**各自的工作群**里被催办；进行中的心跳由 Boss 从群消息自动推导（干活 bot 无需手动发心跳）；等发起人确认设计的行会直接 @发起人；
- **看历史**：打开看板 Base，按「主任务」筛选即该项目全部任务与状态流转；「工作群」为 Group 字段，点击可直接跳转项目群。

## 常见问题

| 问题 | 答案 |
|---|---|
| bot 被 @ 后只回 "No workspace found" | 该群没做 `/workspace route`；见上文项目初始化第 2 步 |
| 本地路径 `/workspace init` 被拒 | 改用 `/workspace route <绝对路径>`（route 绑定已存在目录、不需要 `workspace_init_allow_local_paths`）；或给配置加 `= true` 才能用 init 绑本地路径 |
| 催办消息没发出来 | Boss bot app 不在该工作群（老群需手动拉入；新群 init 脚本已自动拉）；排查看 `~/.cc-connect/logs/board-watchdog.log` 或 `cc-connect cron info <id>`（webui 亦可） |
| 消息发不出 230002 | 发送者不在目标群；board-send 以 bot 自己身份发，确保它在群里 |
| 某任务一直没被催 / 日志出现 `SKIP_UNRESOLVED` | 该行「角色」写成了非法值（既非角色键 `reviewer`、也非 label `Gamma (reviewer)`、也非 bot 名 `Gamma`）→ watchdog 无法映射到 bot。修正该行「角色」字段即可（watchdog 已兜住裸 bot 名，但彻底乱写仍会 SKIP 并在日志留痕） |
| 所有 lark-cli 突然报 need_user_authorization | 有 agent 动了共享认证；恢复见 admin-setup.md，协议已禁止此行为 |
