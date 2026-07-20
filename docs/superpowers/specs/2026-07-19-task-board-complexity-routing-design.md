# Task Board 项目启动先分流设计

## 背景

`task-board` 技能当前把 team-leader 收到新项目 kickoff 或整块需求后的流程定义为“设计先行”：先自建设计任务，评估复杂度，简单需求写 `.board/design.md` 后派发，复杂需求走 `brainstorming → writing-plans`。

这个规则对开发类项目合理，但对目标明确的非开发工作过重。例如建单、查询、整理、通知、简单脚本或配置操作，不需要先进入设计流程。Boss 发起 kickoff 时也应该明确告诉 Beta/team-leader：先判断任务是否可直接执行，复杂或含糊任务才走 brainstorming。

## 目标

- Boss kickoff 给 Beta/team-leader 的消息明确要求先按复杂度和明确度分流。
- `SKILL.md` 固化同一规则，覆盖 Boss kickoff、人工 @team-leader、watchdog 催办后的二次唤醒。
- 目标明确的非开发任务可以直接开始，不被强制拉入设计流程。
- 复杂或含糊任务仍使用 `superpowers:brainstorming → superpowers:writing-plans`。
- 添加测试覆盖 Boss kickoff 文案，防止回归。

## 非目标

- 不改变看板脚本的数据协议、字段、状态机或角色映射。
- 不新增角色或新的脚本参数。
- 不改变 `board-done.sh`、`board-new-task.sh` 等交接语义。
- 不扩大到 README/admin 文档同步；本次只更新运行协议和 kickoff 文案。

## 行为设计

Beta/team-leader 收到新项目或整块需求后，先通读 `.board/requirement.md` 或用户原始需求，然后分为三类处理。

### 1. 直接执行

适用条件：

- 目标明确；
- 无需架构、技术选型或方案取舍；
- 无需发起人确认；
- 不需要拆成多角色协作。

典型任务：建单、查询、整理、通知、简单脚本/配置操作、明确的小改动等。

行为：team-leader 自建一个执行把手任务并认领，在当前任务里直接完成。完成后按真实后续处理：无需后续角色则 `--last` 收口；确实需要交接时才 `--next <别的角色> <子任务>`。

### 2. 轻量设计后派发

适用条件：

- 需要跨角色协作，或需要交给 developer/tester/reviewer；
- 需求清楚；
- 无关键取舍或发起人确认点。

行为：写 `.board/design.md`，包含需求理解、方案、任务拆解、各任务验收标准；用 `board-send.sh` 向工作群公示设计要点和文档路径；然后 `board-done.sh ... --next <首个角色> <首个子任务>` 派发。

### 3. 复杂设计流程

适用条件满足任一即可：

- 需求含糊；
- 有关键产品或技术取舍；
- 需要架构或技术选型；
- 跨多模块或服务；
- 预计子任务超过 3 个；
- 风险高或影响面不清。

行为：调用 superpowers 技能链 `brainstorming → writing-plans`，产出正式设计与计划文档。若需要发起人确认，使用原有待确认任务规则：`board-done.sh ... --next team-leader "待发起人<open_id>确认设计后拆解派发（设计=<路径>）"`，并 `board-send.sh` @发起人确认。

## 文件改动

### `plugins/cc-connect-skills/skills/task-board/SKILL.md`

- 将章节“项目启动·设计先行（team-leader 专属）”改为“项目启动·先分流（team-leader 专属）”。
- 删除“禁止直接给 developer 建任务，先走设计阶段”的绝对表达。
- 添加“直接执行 / 轻量设计后派发 / 复杂设计流程”三类规则。
- 保留原有确认跟进规则、拆解粒度规则、文档交接规则。

### `plugins/cc-connect-skills/skills/task-board/scripts/board-init-project.sh`

- 修改 Boss @team-leader 的 kickoff 文案。
- 文案引用 `SKILL.md` 的“项目启动·先分流”规则。
- 明确目标明确的建单/查询/整理/通知/简单操作可直接开始。
- 明确复杂、含糊、有关键取舍时才走 `brainstorming → writing-plans`。

### `tests/release_local/task_board_scripts/board_init_project_test.go`

- 修改或新增 kickoff 消息断言。
- 断言 team-leader 唤醒消息包含：
  - `项目启动·先分流`
  - `目标明确`
  - `建单/查询/整理/通知/简单操作`
  - `brainstorming`

## 测试计划

运行 task-board 初始化脚本相关测试：

```bash
go test ./tests/release_local/task_board_scripts -run Test
```

如测试文件中已有更精确的测试名，则优先运行被修改的具体测试，再运行该包测试。

## 成功标准

- `SKILL.md` 和 Boss kickoff 文案不再表达“一律设计先行”。
- 新规则明确允许目标清楚的非开发任务直接开始。
- 复杂或含糊任务仍明确走 `brainstorming → writing-plans`。
- Boss kickoff 文案有自动化测试覆盖。
- 相关 Go 测试通过。
