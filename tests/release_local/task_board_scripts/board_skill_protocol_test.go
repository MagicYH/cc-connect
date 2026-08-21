package task_board_scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskBoardSkillRequiresContextBootstrapBeforeWork(t *testing.T) {
	skill := readTaskBoardSkill(t)

	for _, want := range []string{
		"board-project-context.sh <rid>",
		".board/context-<rid>.md",
		"board-chat-history.sh <rid>",
		"读完整需求",
		"来源任务",
		"上游产出",
		"board-chat-history.sh <rid>",
		"派 subagent 总结群历史",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("expected task-board skill to require %q; skill:\n%s", want, skill)
		}
	}
}

func TestTaskBoardSkillRequiresChatHistorySubagentAfterReminder(t *testing.T) {
	skill := readTaskBoardSkill(t)

	ordered := []string{
		"board-claim.sh` / `board-reclaim.sh` 成功后",
		"被看板提醒后",
		"主 Agent 必须先派**异步 subagent**",
		"board-chat-history.sh <rid>",
		"最近消息",
		"分析当前需要做什么、进展如何",
		"总结交回主 Agent",
		"只有明确需要恢复完整长上下文时",
		"主 Agent 再继续处理任务",
		"再运行 `mkdir -p .board && board-project-context.sh <rid> > .board/context-<rid>.md`",
	}
	requireSkillContainsInOrder(t, skill, "reminder chat history subagent protocol", ordered...)
}

func TestTaskBoardSkillForbidsHistorySubagentFromRediscoveringTasks(t *testing.T) {
	skill := readTaskBoardSkill(t)

	ordered := []string{
		"把已认领的 `<rid>` 和精确命令交给 subagent",
		"subagent 不得再运行 `board-my-todos.sh`",
		"不得 claim/reclaim",
		"只负责执行 `board-chat-history.sh <rid>`",
	}
	requireSkillContainsInOrder(t, skill, "history subagent receives claimed rid", ordered...)
}

func TestTaskBoardSkillRequiresClearDispatchText(t *testing.T) {
	skill := readTaskBoardSkill(t)

	for _, want := range []string{
		"任务目标",
		"需求依据",
		"上游产出路径",
		"验收标准",
		"预期产出文档",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("expected task-board skill dispatch rules to mention %q; skill:\n%s", want, skill)
		}
	}
}

func requireSkillContainsInOrder(t *testing.T, skill, context string, wants ...string) {
	t.Helper()

	start := 0
	for _, want := range wants {
		idx := strings.Index(skill[start:], want)
		if idx < 0 {
			t.Fatalf("expected task-board skill %s to mention %q after offset %d", context, want, start)
		}
		start += idx + len(want)
	}
}

func readTaskBoardSkill(t *testing.T) string {
	t.Helper()
	path := filepath.Join(findRepoRoot(t), "plugins", "cc-connect-skills", "skills", "task-board", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	return string(data)
}
