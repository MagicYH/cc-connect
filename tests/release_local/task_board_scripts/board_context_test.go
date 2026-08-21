package task_board_scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoardProjectContextPrintsProjectRequirementAndTaskChain(t *testing.T) {
	h := newBoardContextHarness(t)

	out, err := h.runProjectContext("rec_task")

	if err != nil {
		t.Fatalf("expected project context to succeed; output:\n%s", out)
	}
	for _, want := range []string{
		"# Task Board Context",
		"主任务：goal-context",
		"项目状态：进行中",
		"## 需求描述",
		"完整目标：实现 task-board 上下文恢复",
		"当前任务：实现开发任务",
		"来源任务：rec_source",
		"上游产出：.board/design.md",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q; got:\n%s", want, out)
		}
	}
}

func TestBoardProjectContextFailsWhenProjectNameIsAmbiguous(t *testing.T) {
	h := newBoardContextHarness(t)
	h.projectRows = `{"data":{"fields":["主任务名","项目状态","需求描述","发起人","参与角色","工作群"],"record_id_list":["rec_p1","rec_p2"],"data":[["goal-context","进行中","req 1","ou_initiator","team-leader,developer,tester,reviewer",[{"id":"oc_test_chat"}]],["goal-context","进行中","req 2","ou_initiator","team-leader,developer,tester,reviewer",[{"id":"oc_test_chat"}]]]}}`

	out, err := h.runProjectContext("rec_task")

	if err == nil {
		t.Fatalf("expected ambiguous project to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "PROJECT_AMBIGUOUS goal-context rec_p1 rec_p2") {
		t.Fatalf("expected ambiguous project error; output:\n%s", out)
	}
}

func TestBoardProjectContextChecksLaterProjectPagesForAmbiguity(t *testing.T) {
	h := newBoardContextHarness(t)
	h.projectRows = `{"data":{"fields":["主任务名","项目状态","需求描述","发起人","参与角色","工作群"],"record_id_list":["rec_p1"],"data":[["goal-context","进行中","req 1","ou_initiator","team-leader,developer,tester,reviewer",[{"id":"oc_test_chat"}]]]}}`
	h.projectRowsPage2 = `{"data":{"fields":["主任务名","项目状态","需求描述","发起人","参与角色","工作群"],"record_id_list":["rec_p2"],"data":[["goal-context","进行中","req 2","ou_initiator","team-leader,developer,tester,reviewer",[{"id":"oc_test_chat"}]]]}}`

	out, err := h.runProjectContext("rec_task")

	if err == nil {
		t.Fatalf("expected ambiguity across pages to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "PROJECT_AMBIGUOUS goal-context rec_p1 rec_p2") {
		t.Fatalf("expected cross-page ambiguous project error; output:\n%s", out)
	}
}

func TestBoardChatHistoryFetchesChatMessagesAsMarkdown(t *testing.T) {
	h := newBoardContextHarness(t)

	out, err := h.runChatHistory("rec_task", "25")

	if err != nil {
		t.Fatalf("expected chat history to succeed; output:\n%s", out)
	}
	if !strings.Contains(out, "# Chat History") || !strings.Contains(out, "Alice: 需求补充：保留验收标准") {
		t.Fatalf("expected markdown history with sender and content; output:\n%s", out)
	}
	args := readOptionalFile(t, h.chatArgsLog)
	for _, want := range []string{"im +chat-messages-list", "--chat-id oc_test_chat", "--page-size 25", "--as bot"} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected chat history command to contain %q; args:\n%s", want, args)
		}
	}
}

func TestBoardChatHistoryRendersDataMessagesShape(t *testing.T) {
	h := newBoardContextHarness(t)
	h.chatShape = "data_messages"

	out, err := h.runChatHistory("rec_task", "25")

	if err != nil {
		t.Fatalf("expected chat history to succeed; output:\n%s", out)
	}
	if !strings.Contains(out, "Carol: data.messages 结构消息") {
		t.Fatalf("expected data.messages chat output; output:\n%s", out)
	}
}

func TestBoardChatHistoryAllFetchesEveryPage(t *testing.T) {
	h := newBoardContextHarness(t)

	out, err := h.runChatHistory("rec_task", "all")

	if err != nil {
		t.Fatalf("expected all chat history to succeed; output:\n%s", out)
	}
	if !strings.Contains(out, "Alice: 第 1 页消息") || !strings.Contains(out, "Bob: 第 2 页消息") {
		t.Fatalf("expected history from both pages; output:\n%s", out)
	}
	args := readOptionalFile(t, h.chatArgsLog)
	if !strings.Contains(args, "--page-token token-2") {
		t.Fatalf("expected second page fetch with page token; args:\n%s", args)
	}
}

type boardContextHarness struct {
	t                *testing.T
	homeDir          string
	binDir           string
	scriptDir        string
	chatArgsLog      string
	projectRows      string
	projectRowsPage2 string
	chatShape        string
}

func newBoardContextHarness(t *testing.T) *boardContextHarness {
	t.Helper()
	repoRoot := findRepoRoot(t)
	tempDir := t.TempDir()
	h := &boardContextHarness{
		t:           t,
		homeDir:     filepath.Join(tempDir, "home"),
		binDir:      filepath.Join(tempDir, "bin"),
		scriptDir:   filepath.Join(tempDir, "scripts"),
		chatArgsLog: filepath.Join(tempDir, "chat.args"),
		projectRows: `{"data":{"fields":["主任务名","项目状态","需求描述","发起人","参与角色","工作群"],"record_id_list":["rec_project"],"data":[["goal-context","进行中","完整目标：实现 task-board 上下文恢复","ou_initiator","team-leader,developer,tester,reviewer",[{"id":"oc_test_chat"}]]]}}`,
	}
	mustMkdirAll(t, filepath.Join(h.homeDir, ".cc-connect"))
	mustMkdirAll(t, h.binDir)
	mustMkdirAll(t, h.scriptDir)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-lib.sh"),
		filepath.Join(h.scriptDir, "board-lib.sh"), 0o644)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-project-context.sh"),
		filepath.Join(h.scriptDir, "board-project-context.sh"), 0o755)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-chat-history.sh"),
		filepath.Join(h.scriptDir, "board-chat-history.sh"), 0o755)
	writeFile(t, filepath.Join(h.binDir, "lark-cli"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"base +record-get"* && "$*" == *"--record-id rec_task"* ]]; then
  printf '{"data":{"fields":["子任务","主任务","工作群","角色","状态","来源任务","产出备注"],"data":[["实现开发任务","goal-context",[{"id":"oc_test_chat"}],"Delta (developer)","进行中","rec_source",""]]}}\n'
elif [[ "$*" == *"base +record-get"* && "$*" == *"--record-id rec_source"* ]]; then
  printf '{"data":{"fields":["子任务","主任务","工作群","角色","状态","来源任务","产出备注"],"data":[["需求分析与技术设计","goal-context",[{"id":"oc_test_chat"}],"Beta (team-leader)","完成","",".board/design.md"]]}}\n'
elif [[ "$*" == *"base +record-list"* && "$*" == *"--table-id tbl_projects"* ]]; then
  if [[ "$*" == *"--offset 200"* && -n "${PROJECT_ROWS_PAGE2_JSON:-}" ]]; then
    printf '%s\n' "$PROJECT_ROWS_PAGE2_JSON"
  elif [[ "$*" == *"--offset 200"* || "$*" == *"--offset 400"* ]]; then
    printf '{"data":{"fields":["主任务名","项目状态","需求描述","发起人","参与角色","工作群"],"record_id_list":[],"data":[]}}\n'
  else
    printf '%s\n' "$PROJECT_ROWS_JSON"
  fi
elif [[ "$*" == *"im +chat-messages-list"* ]]; then
  printf '%s\n' "$*" >> "$CHAT_ARGS_LOG"
  if [[ "${CHAT_SHAPE:-}" == "data_messages" ]]; then
    printf '{"data":{"messages":[{"sender":{"name":"Carol"},"body":{"text":"data.messages 结构消息"},"create_time":"1785260002000"}]}}\n'
  elif [[ "$*" == *"--page-token token-2"* ]]; then
    printf '{"items":[{"sender":{"name":"Bob"},"body":{"text":"第 2 页消息"},"create_time":"1785260001000"}],"has_more":false}\n'
  elif [[ "$*" == *"--page-size 50"* ]]; then
    printf '{"items":[{"sender":{"name":"Alice"},"body":{"text":"第 1 页消息"},"create_time":"1785260000000"}],"has_more":true,"page_token":"token-2"}\n'
  else
    printf '{"items":[{"sender":{"name":"Alice"},"body":{"text":"需求补充：保留验收标准"},"create_time":"1785260000000"}]}\n'
  fi
else
  printf 'unexpected lark-cli call: %s\n' "$*" >&2
  exit 1
fi
`, 0o755)
	writeFile(t, filepath.Join(h.homeDir, ".cc-connect", "board.env"), strings.Join([]string{
		"BOARD_BASE=base_token",
		"TBL_TASKS=tbl_tasks",
		"TBL_PROJECTS=tbl_projects",
		"BOT_LABEL_developer='Delta (developer)'",
		"",
	}, "\n"), 0o644)
	return h
}

func (h *boardContextHarness) runProjectContext(rid string) (string, error) {
	h.t.Helper()
	cmd := exec.Command(filepath.Join(h.scriptDir, "board-project-context.sh"), rid)
	cmd.Env = h.env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (h *boardContextHarness) runChatHistory(rid string, limit string) (string, error) {
	h.t.Helper()
	cmd := exec.Command(filepath.Join(h.scriptDir, "board-chat-history.sh"), rid, limit)
	cmd.Env = h.env()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (h *boardContextHarness) env() []string {
	h.t.Helper()
	return append(os.Environ(),
		"HOME="+h.homeDir,
		"PATH="+h.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CC_PROJECT=developer",
		"PROJECT_ROWS_JSON="+h.projectRows,
		"PROJECT_ROWS_PAGE2_JSON="+h.projectRowsPage2,
		"CHAT_SHAPE="+h.chatShape,
		"CHAT_ARGS_LOG="+h.chatArgsLog,
	)
}
