# Task Board Complexity Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update task-board startup routing so Beta/team-leader directly starts clear/simple work and only uses brainstorming for complex or ambiguous work.

**Architecture:** This is a protocol and prompt-copy change, not a data model change. The runtime rule lives in `SKILL.md`; Boss kickoff in `board-init-project.sh` points to that rule; Go release-local tests assert the kickoff message keeps the new routing cues.

**Tech Stack:** Markdown skill protocol, Bash task-board script, Go `testing` package for script harness tests.

## Global Constraints

- Do not change task-board script data protocol, fields, state machine, or role mapping.
- Do not add roles or script parameters.
- Do not change `board-done.sh`, `board-new-task.sh`, or handoff semantics.
- Update only runtime protocol, kickoff wording, and tests for this task.
- Target files: `plugins/cc-connect-skills/skills/task-board/SKILL.md`, `plugins/cc-connect-skills/skills/task-board/scripts/board-init-project.sh`, `tests/release_local/task_board_scripts/board_init_project_test.go`.
- Verification must include task-board script tests, full project tests, dev-sg E2E, then merge to the shadow branch and push remote.

---

## File Structure

- `plugins/cc-connect-skills/skills/task-board/SKILL.md`
  - Runtime protocol loaded by role bots. Owns the durable routing rule: direct execution vs lightweight design vs complex brainstorming.
- `plugins/cc-connect-skills/skills/task-board/scripts/board-init-project.sh`
  - Boss/TL new project initialization script. Owns the kickoff message sent to team-leader after workspace binding succeeds.
- `tests/release_local/task_board_scripts/board_init_project_test.go`
  - Go harness for `board-init-project.sh`. Owns regression coverage for generated kickoff messages.

---

### Task 1: Add kickoff routing regression test

**Files:**
- Modify: `tests/release_local/task_board_scripts/board_init_project_test.go`

**Interfaces:**
- Consumes: `newBoardInitHarness(t)`, `writeInitBoardEnv`, `writeInitConfig`, `writeWorkspaceBindings`, `writeInitLarkCli`, `readOptionalFile` from the existing test file.
- Produces: a failing test that asserts TL kickoff includes new routing language.

- [ ] **Step 1: Add the failing test near `TestBoardInitProjectWritesRequirementToGitignoredBoardDir`**

Add this Go test function after `TestBoardInitProjectWritesRequirementToGitignoredBoardDir`:

```go
func TestBoardInitProjectKickoffTellsTeamLeaderToRouteByComplexity(t *testing.T) {
	h := newBoardInitHarness(t)
	workDir := filepath.Join(h.tempDir, "work", "demo")
	writeInitBoardEnv(t, h.homeDir, h.tempDir)
	writeInitConfig(t, h.homeDir, "feishu")
	writeWorkspaceBindings(t, filepath.Join(h.homeDir, ".cc-connect", "workspace_bindings.json"), map[string]string{
		"team-leader": workDir,
		"developer":   workDir,
		"tester":      workDir,
		"reviewer":    workDir,
	}, "feishu", "oc_test_chat")
	writeInitLarkCli(t, h.binDir, "")

	out, err := h.run("boss", "demo-project", "建一个跟进单并通知相关人", workDir)
	if err != nil {
		t.Fatalf("expected init to succeed with full bindings; output:\n%s", out)
	}

	sends := readOptionalFile(t, h.sendLog)
	for _, want := range []string{
		"项目启动·先分流",
		"目标明确",
		"建单/查询/整理/通知/简单操作",
		"brainstorming",
	} {
		if !strings.Contains(sends, want) {
			t.Fatalf("expected kickoff message to contain %q; sends:\n%s", want, sends)
		}
	}
}
```

- [ ] **Step 2: Run test and verify it fails**

Run:

```bash
go test ./tests/release_local/task_board_scripts -run TestBoardInitProjectKickoffTellsTeamLeaderToRouteByComplexity -v
```

Expected: FAIL because the current kickoff message still says `项目启动·设计先行` and does not contain all new routing cues.

---

### Task 2: Update task-board runtime protocol in SKILL.md

**Files:**
- Modify: `plugins/cc-connect-skills/skills/task-board/SKILL.md:38-50`

**Interfaces:**
- Consumes: no code interfaces; updates Markdown protocol text.
- Produces: durable runtime rule named `项目启动·先分流（team-leader 专属）` used by kickoff message and bots.

- [ ] **Step 1: Replace the old project startup section**

Replace the section from `## 项目启动·设计先行（team-leader 专属）` through the paragraph before `## 硬规则` with this content:

```markdown
## 项目启动·先分流（team-leader 专属）

被新项目 kickoff @（消息含"新项目"）或收到整块新需求时，先通读 `.board/requirement.md` 或用户原始需求全文，**按任务明确度与复杂度分流**，不要把所有工作机械拉进设计流程。

1. 自建设计/执行把手任务并认领：`board-new-task.sh <主任务> <工作群> team-leader "需求分流与执行规划"` → `board-claim.sh`。
2. **直接执行**：目标明确、无需架构/技术选型/方案取舍、无需发起人确认、无需拆成多角色协作的任务，直接在当前任务里完成。典型场景：建单/查询/整理/通知/简单操作（脚本、配置、明确小改动等）。完成后按真实后续收口：无后续则 `--last`；确需交接才 `--next <别的角色> <子任务>`。
3. **轻量设计后派发**：需要跨角色协作或交给 developer/tester/reviewer，但需求清楚、无关键取舍 → 写 `.board/design.md`（需求理解 / 方案 / 任务拆解 / 各任务验收标准）→ `board-send.sh` 向工作群公示设计要点+文档路径（不 @）→ `board-done.sh <rid> <nonce> .board/design.md --next <首个角色> "<首个子任务>"`。
4. **复杂设计流程**：满足任一即复杂：需求含糊；有关键产品/技术取舍；需要架构/技术选型；预计子任务 >3 个；跨多模块或服务；风险高或影响面不清。复杂任务调用 superpowers 技能链（brainstorming → writing-plans，自主推进；歧义与关键取舍**列成问题清单写进设计**，不臆测）产出设计与计划文档 → `board-done.sh <rid> <nonce> <设计文档路径> --next team-leader "待发起人<open_id>确认设计后拆解派发（设计=<路径>）"` → `board-send.sh <工作群> <发起人open_id> "<设计要点+路径+问题清单，请确认后开工>"`。发起人 open_id 取自 kickoff 消息。
5. **确认跟进任务规则**（子任务含「待发起人…确认」的行）：
   - 本次唤醒消息就是发起人的回复 → 认领：确认则 `--next developer <首个开发子任务>` 开工；有修改意见则按意见修订设计后重复第 4 步收尾。
   - 被看板催办消息唤醒时遇到它 → **不认领不心跳**（watchdog 巡检会直接催发起人，无需你转达）。

> **拆解粒度（TL 拆计划时）**：给同一角色的连续步骤**合并成一个看板任务**（如把 plan 的 Task1–5 作为**一行** developer 任务，让 developer 在这一个任务内连续做完），**不要一个 plan-step 建一行**再让它反复自我激活。看板行只在**换角色**、**设计/执行把手**、或**停下等外部确认**时才新增。
```

- [ ] **Step 2: Check wording for forbidden old absolute rule**

Run:

```bash
rg -n "设计先行|禁止直接给 developer|先走设计阶段|机械拉进设计流程" plugins/cc-connect-skills/skills/task-board/SKILL.md
```

Expected: only `机械拉进设计流程` may appear as part of the new negative instruction. `设计先行`, `禁止直接给 developer`, and `先走设计阶段` should not appear in `SKILL.md`.

---

### Task 3: Update Boss kickoff message

**Files:**
- Modify: `plugins/cc-connect-skills/skills/task-board/scripts/board-init-project.sh:136-140`

**Interfaces:**
- Consumes: existing variables `NAME`, `INITIATOR_OPENID`, `BOARD_WRITER_OPENID`, `CHAT`, `REQ`, and `SEND`.
- Produces: TL kickoff message containing `项目启动·先分流`, `目标明确`, `建单/查询/整理/通知/简单操作`, and `brainstorming`.

- [ ] **Step 1: Replace the TL kickoff send block**

Replace lines 136-140 with:

```bash
# 7) @team-leader 起步（完整需求以 .board/requirement.md 为准；消息附原文做冗余，二者皆为完整原文）
"$SEND" "$CHAT" "$BOT_OPENID_team_leader" "新项目「${NAME}」。完整需求（发起人原文）已写入工作目录 .board/requirement.md，**以该文档全文为准，勿凭节选臆测或自行删减**。工作群与看板已就绪、workspace 已绑定，发起人=${INITIATOR_OPENID:-$BOARD_WRITER_OPENID}。请按 task-board 技能『项目启动·先分流』处理（主任务=${NAME}, 工作群=${CHAT}）：先通读 .board/requirement.md 全文；目标明确、无需设计/取舍/多角色拆解的建单/查询/整理/通知/简单操作可直接开始；需要协作但需求清楚则轻量设计后派发；复杂、含糊或有关键取舍时才走 brainstorming → writing-plans，并在需要确认时 @发起人。

需求原文：
${REQ}" >/dev/null
```

- [ ] **Step 2: Run the previously failing kickoff test**

Run:

```bash
go test ./tests/release_local/task_board_scripts -run TestBoardInitProjectKickoffTellsTeamLeaderToRouteByComplexity -v
```

Expected: PASS.

---

### Task 4: Run focused package tests

**Files:**
- Test: `tests/release_local/task_board_scripts/board_init_project_test.go`

**Interfaces:**
- Consumes: completed Tasks 1-3.
- Produces: passing release-local task-board script package tests.

- [ ] **Step 1: Format the Go test file**

Run:

```bash
gofmt -w tests/release_local/task_board_scripts/board_init_project_test.go
```

Expected: command exits 0.

- [ ] **Step 2: Run task-board script tests**

Run:

```bash
go test ./tests/release_local/task_board_scripts -v
```

Expected: PASS.

---

### Task 5: Full verification and dev-sg E2E

**Files:**
- Read if needed: `memory/dev-sg-deploy.md` from project memory index.
- Test: full repository and dev-sg E2E commands.

**Interfaces:**
- Consumes: completed Tasks 1-4.
- Produces: evidence that all local tests and dev-sg E2E passed.

- [ ] **Step 1: Run full local Go tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run race tests if feasible**

Run:

```bash
go test -race ./...
```

Expected: PASS. If this is too slow or environment-limited, record the exact failure or limitation and continue only after deciding whether it blocks completion.

- [ ] **Step 3: Load dev-sg deployment memory**

Read `/Users/bytedance/.claude/projects/-Users-bytedance-Project-Source-Github-cc-connect/memory/dev-sg-deploy.md` for the exact dev-sg build/deploy/test procedure.

- [ ] **Step 4: Run dev-sg E2E**

Follow the memory instructions exactly for dev-sg. Expected: E2E passes and confirms the task-board-related build still works in dev-sg.

---

### Task 6: Review, commit, merge to shadow branch, and push

**Files:**
- Commit: `plugins/cc-connect-skills/skills/task-board/SKILL.md`, `plugins/cc-connect-skills/skills/task-board/scripts/board-init-project.sh`, `tests/release_local/task_board_scripts/board_init_project_test.go`, `docs/superpowers/specs/2026-07-19-task-board-complexity-routing-design.md`, `docs/superpowers/plans/2026-07-19-task-board-complexity-routing.md`.

**Interfaces:**
- Consumes: passing verification from Task 5.
- Produces: pushed remote shadow branch containing this work.

- [ ] **Step 1: Inspect working tree**

Run:

```bash
git status --short
```

Expected: only intended changes for this session plus pre-existing branch changes. Do not discard unrelated existing modifications.

- [ ] **Step 2: Review changed files**

Run:

```bash
git diff -- plugins/cc-connect-skills/skills/task-board/SKILL.md plugins/cc-connect-skills/skills/task-board/scripts/board-init-project.sh tests/release_local/task_board_scripts/board_init_project_test.go docs/superpowers/specs/2026-07-19-task-board-complexity-routing-design.md docs/superpowers/plans/2026-07-19-task-board-complexity-routing.md
```

Expected: diff matches the approved spec and plan.

- [ ] **Step 3: Commit intended changes**

Run:

```bash
git add plugins/cc-connect-skills/skills/task-board/SKILL.md \
  plugins/cc-connect-skills/skills/task-board/scripts/board-init-project.sh \
  tests/release_local/task_board_scripts/board_init_project_test.go \
  docs/superpowers/specs/2026-07-19-task-board-complexity-routing-design.md \
  docs/superpowers/plans/2026-07-19-task-board-complexity-routing.md

git commit -m "feat: route task-board kickoff by complexity"
```

Expected: commit succeeds.

- [ ] **Step 4: Merge to shadow branch if current branch is not already the shadow branch**

Run:

```bash
git branch --show-current
```

If output is `worktree-team-roster-append-prompt-shadow`, no merge is needed. If output differs, merge this commit into `worktree-team-roster-append-prompt-shadow` without dropping unrelated work.

- [ ] **Step 5: Push remote branch**

Run:

```bash
git push -u origin worktree-team-roster-append-prompt-shadow
```

Expected: push succeeds.
