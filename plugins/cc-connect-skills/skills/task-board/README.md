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

# 2) 各角色 bot 的 [[projects]] 配置确认三件事，然后重启 daemon：
#    workspace_init_allow_local_paths = true
#    allow_chat / allow_from 留空；resolve_mentions = false
cc-connect daemon restart

# 3) 注册自查 cron（锚点群须 workspace 干净、非生产群）
./board-setup-cron.sh feishu:oc_你的锚点群chatID

# 4) 给各角色 bot 的 system_prompt 追加一句（经 Management API PATCH，见 admin-setup.md）：
#    "看板任务一律使用 task-board 技能处理"
```

## 每个新项目的初始化

1. 建项目群、拉齐角色 bot；
2. **逐个 @ 每个 bot** 发 `/workspace init <工作目录绝对路径>`（漏了 bot 不干活）；
3. 写 Projects 行（主任务名/群chatID/状态=进行中）；
4. 在群里 @team-leader 起步。

## 日常使用

- **派任务**：任何人/bot 用 `board-new-task.sh` 建行，再 `board-send.sh` @ 对应角色；
- **人工触发**：群里 @ 某 bot 说"检查看板"；
- **兜底**：cron 每 30 分钟自动扫（漏派的待办、心跳超时的进行中都会被捞起）；
- **看历史**：打开看板 Base，按「主任务」筛选即该项目全部任务与状态流转。

## 常见问题

| 问题 | 答案 |
|---|---|
| bot 被 @ 后只回 "No workspace found" | 该群没做 `/workspace init`；见上文项目初始化第 2 步 |
| 本地路径 init 被拒 | 配置缺 `workspace_init_allow_local_paths = true` |
| cron 自查干了别的事/查错表 | 锚点群 workspace 有竞争 skill；换干净锚点（admin-setup.md） |
| 消息发不出 230002 | 发送者不在目标群；board-send 以 bot 自己身份发，确保它在群里 |
| 所有 lark-cli 突然报 need_user_authorization | 有 agent 动了共享认证；恢复见 admin-setup.md，协议已禁止此行为 |
