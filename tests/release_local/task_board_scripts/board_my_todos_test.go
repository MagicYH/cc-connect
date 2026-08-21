package task_board_scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoardMyTodosReadsAllRecordListPages(t *testing.T) {
	h := newBoardMyTodosHarness(t)

	output, err := h.run()

	if err != nil {
		t.Fatalf("expected my-todos to succeed; output:\n%s", output)
	}
	line := strings.TrimSpace(output)
	if !strings.Contains(line, "rec_todo_second_page\tTODO\tsecond page task\tmain task\toc_chat") {
		t.Fatalf("expected TODO from second page; output:\n%s", output)
	}
	calls := readOptionalFile(t, h.callsLog)
	if !strings.Contains(calls, "--offset 0") || !strings.Contains(calls, "--offset 1") {
		t.Fatalf("expected record-list to request both pages with offsets; calls:\n%s", calls)
	}
}

type boardMyTodosHarness struct {
	t         *testing.T
	homeDir   string
	binDir    string
	scriptDir string
	callsLog  string
}

func newBoardMyTodosHarness(t *testing.T) boardMyTodosHarness {
	t.Helper()
	repoRoot := findRepoRoot(t)
	tempDir := t.TempDir()
	h := boardMyTodosHarness{
		t:         t,
		homeDir:   filepath.Join(tempDir, "home"),
		binDir:    filepath.Join(tempDir, "bin"),
		scriptDir: filepath.Join(tempDir, "scripts"),
		callsLog:  filepath.Join(tempDir, "calls.log"),
	}
	mustMkdirAll(t, filepath.Join(h.homeDir, ".cc-connect"))
	mustMkdirAll(t, h.binDir)
	mustMkdirAll(t, h.scriptDir)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-my-todos.sh"),
		filepath.Join(h.scriptDir, "board-my-todos.sh"), 0o755)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-lib.sh"),
		filepath.Join(h.scriptDir, "board-lib.sh"), 0o644)
	writeFile(t, filepath.Join(h.homeDir, ".cc-connect", "board.env"), strings.Join([]string{
		"BOARD_BASE=base_token",
		"TBL_TASKS=tbl_tasks",
		"TBL_PROJECTS=tbl_projects",
		"BOT_OPENID_developer=ou_dev",
		"BOT_LABEL_developer='Delta (developer)'",
		"",
	}, "\n"), 0o644)
	writeFile(t, filepath.Join(h.binDir, "lark-cli"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$CALLS_LOG"
if [[ "$*" != *"base +record-list"* ]]; then
  printf 'unexpected lark-cli call: %s\n' "$*" >&2
  exit 1
fi
[[ "$*" == *"--base-token base_token"* ]] || { printf 'record-list must use board base: %s\n' "$*" >&2; exit 1; }
[[ "$*" == *"--table-id tbl_tasks"* ]] || { printf 'record-list must use Tasks table: %s\n' "$*" >&2; exit 1; }
[[ "$*" == *"--format json"* ]] || { printf 'record-list must request json: %s\n' "$*" >&2; exit 1; }
[[ "$*" == *"--as user"* ]] || { printf 'record-list must run as user: %s\n' "$*" >&2; exit 1; }
offset=0
prev=''
for arg in "$@"; do
  if [[ "$prev" == "--offset" ]]; then offset="$arg"; fi
  prev="$arg"
done
if [[ "$offset" == "0" ]]; then
  printf '{"data":{"fields":["角色","状态","子任务","主任务","工作群","心跳时间"],"record_id_list":["rec_done_first_page"],"data":[["Delta (developer)","完成","done task","main task",[{"id":"oc_done","name":"done"}],""]],"has_more":true}}\n'
elif [[ "$offset" == "1" ]]; then
  printf '{"data":{"fields":["角色","状态","子任务","主任务","工作群","心跳时间"],"record_id_list":["rec_todo_second_page"],"data":[["Delta (developer)","待办","second page task","main task",[{"id":"oc_chat","name":"chat"}],""]],"has_more":false}}\n'
else
  printf 'unexpected offset %s\n' "$offset" >&2
  exit 1
fi
`, 0o755)
	return h
}

func (h boardMyTodosHarness) run() (string, error) {
	h.t.Helper()
	cmd := exec.Command(filepath.Join(h.scriptDir, "board-my-todos.sh"))
	cmd.Env = append(os.Environ(),
		"HOME="+h.homeDir,
		"PATH="+h.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CC_PROJECT=developer",
		"CALLS_LOG="+h.callsLog,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
