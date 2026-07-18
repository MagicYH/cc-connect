package task_board_scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestBoardCompleteProjectMarksMatchingProjectCompleted(t *testing.T) {
	harness := newBoardCompleteProjectHarness(t, `{"data":{"fields":["主任务名","项目状态","完成时间"],"record_id_list":["rec_other","rec_project"],"data":[["other-project","进行中",null],["goal-e2e-1784176632","进行中",null]]}}`)

	output, err := harness.run("goal-e2e-1784176632")

	if err != nil {
		t.Fatalf("expected project completion to succeed; output:\n%s", output)
	}
	if strings.TrimSpace(output) != "PROJECT_DONE rec_project" {
		t.Fatalf("expected exact completion output; output:\n%s", output)
	}

	upsert := harness.readUpsertArgs()
	if !strings.Contains(upsert, "--table-id tbl_projects") {
		t.Fatalf("expected Projects table upsert; got:\n%s", upsert)
	}
	if !strings.Contains(upsert, "--record-id rec_project") {
		t.Fatalf("expected matching project record upsert; got:\n%s", upsert)
	}

	payload := harness.readUpsertJSON()
	if payload["项目状态"] != "已完成" {
		t.Fatalf("expected completed project status; got payload: %#v", payload)
	}
	completionTime, ok := payload["完成时间"].(string)
	if !ok || completionTime == "" {
		t.Fatalf("expected non-empty completion time; got payload: %#v", payload)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`).MatchString(completionTime) {
		t.Fatalf("expected completion time format YYYY-MM-DD HH:MM:SS; got %q", completionTime)
	}
}

func TestBoardCompleteProjectFailsWhenProjectIsMissing(t *testing.T) {
	harness := newBoardCompleteProjectHarness(t, `{"data":{"fields":["主任务名","项目状态","完成时间"],"record_id_list":["rec_other"],"data":[["other-project","进行中",null]]}}`)

	output, err := harness.run("goal-e2e-1784176632")

	if err == nil {
		t.Fatalf("expected missing project to fail; output:\n%s", output)
	}
	if !strings.Contains(output, "PROJECT_NOT_FOUND goal-e2e-1784176632") {
		t.Fatalf("expected missing project error; output:\n%s", output)
	}
	harness.assertNoUpsert()
}

func TestBoardCompleteProjectFailsWhenProjectNameIsAmbiguous(t *testing.T) {
	harness := newBoardCompleteProjectHarness(t, `{"data":{"fields":["主任务名","项目状态","完成时间"],"record_id_list":["rec_project_1","rec_project_2"],"data":[["goal-e2e-1784176632","进行中",null],["goal-e2e-1784176632","进行中",null]]}}`)

	output, err := harness.run("goal-e2e-1784176632")

	if err == nil {
		t.Fatalf("expected ambiguous project to fail; output:\n%s", output)
	}
	if !strings.Contains(output, "PROJECT_AMBIGUOUS goal-e2e-1784176632 rec_project_1 rec_project_2") {
		t.Fatalf("expected ambiguous project error with matching record ids; output:\n%s", output)
	}
	harness.assertNoUpsert()
}

func TestBoardCompleteProjectCleansGitignoredBoardArtifacts(t *testing.T) {
	records := `{"data":{"fields":["主任务名","项目状态","工作群","完成时间"],"record_id_list":["rec_project"],"data":[["proj-clean","进行中",[{"id":"oc_test_chat","name":"proj"}],null]]}}`
	harness := newBoardCompleteProjectHarness(t, records)
	tempDir := filepath.Dir(harness.upsertArgs)
	workDir := filepath.Join(tempDir, "work")
	board := filepath.Join(workDir, ".board")
	mustMkdirAll(t, board)
	writeFile(t, filepath.Join(board, "requirement.md"), "orig requirement", 0o644)
	writeFile(t, filepath.Join(board, ".gitignore"), "*\n", 0o644)
	// 完成角色(team-leader)在此群绑定的 workspace
	bindings := `{"project:team-leader":{"feishu:oc_test_chat":{"workspace":"` + workDir + `"}}}`
	writeFile(t, filepath.Join(harness.homeDir, ".cc-connect", "workspace_bindings.json"), bindings, 0o644)

	output, err := harness.run("proj-clean")
	if err != nil {
		t.Fatalf("expected completion to succeed; output:\n%s", output)
	}
	if !strings.Contains(output, "PROJECT_DONE rec_project") {
		t.Fatalf("expected PROJECT_DONE; output:\n%s", output)
	}
	// 任务最终结束时，中间产物 .board/ 被清理
	if _, statErr := os.Stat(board); !os.IsNotExist(statErr) {
		t.Fatalf("expected .board artifacts removed on completion; stat err=%v", statErr)
	}
}

type boardCompleteProjectHarness struct {
	t          *testing.T
	homeDir    string
	binDir     string
	scriptDir  string
	upsertArgs string
	upsertJSON string
}

func newBoardCompleteProjectHarness(t *testing.T, recordsJSON string) boardCompleteProjectHarness {
	t.Helper()
	repoRoot := findRepoRoot(t)
	tempDir := t.TempDir()
	harness := boardCompleteProjectHarness{
		t:          t,
		homeDir:    filepath.Join(tempDir, "home"),
		binDir:     filepath.Join(tempDir, "bin"),
		scriptDir:  filepath.Join(tempDir, "scripts"),
		upsertArgs: filepath.Join(tempDir, "upsert.args"),
		upsertJSON: filepath.Join(tempDir, "upsert.json"),
	}

	mustMkdirAll(t, filepath.Join(harness.homeDir, ".cc-connect"))
	mustMkdirAll(t, harness.binDir)
	mustMkdirAll(t, harness.scriptDir)

	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-complete-project.sh"),
		filepath.Join(harness.scriptDir, "board-complete-project.sh"), 0o755)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-lib.sh"),
		filepath.Join(harness.scriptDir, "board-lib.sh"), 0o644)

	writeFile(t, filepath.Join(harness.binDir, "lark-cli"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"base +record-list"* ]]; then
  [[ "$*" == *"--base-token base_token"* ]] || { printf 'record-list must use board base: %s\n' "$*" >&2; exit 1; }
  [[ "$*" == *"--table-id tbl_projects"* ]] || { printf 'record-list must use Projects table: %s\n' "$*" >&2; exit 1; }
  [[ "$*" == *"--format json"* ]] || { printf 'record-list must request json: %s\n' "$*" >&2; exit 1; }
  [[ "$*" == *"--as user"* ]] || { printf 'record-list must run as user: %s\n' "$*" >&2; exit 1; }
  printf '%s\n' "$RECORD_LIST_JSON"
elif [[ "$*" == *"base +record-upsert"* ]]; then
  [[ "$*" == *"--base-token base_token"* ]] || { printf 'record-upsert must use board base: %s\n' "$*" >&2; exit 1; }
  [[ "$*" == *"--table-id tbl_projects"* ]] || { printf 'record-upsert must use Projects table: %s\n' "$*" >&2; exit 1; }
  [[ "$*" == *"--as user"* ]] || { printf 'record-upsert must run as user: %s\n' "$*" >&2; exit 1; }
  json=''
  prev=''
  for arg in "$@"; do
    if [[ "$prev" == "--json" ]]; then json="$arg"; fi
    prev="$arg"
  done
  printf '%s\n' "$*" > "$UPSERT_ARGS_LOG"
  printf '%s\n' "$json" > "$UPSERT_JSON_LOG"
  printf '{"data":{"record":{"record_id_list":["rec_project"]}}}\n'
else
  printf 'unexpected lark-cli call: %s\n' "$*" >&2
  exit 1
fi
`, 0o755)

	boardEnv := strings.Join([]string{
		"BOARD_BASE=base_token",
		"TBL_TASKS=tbl_tasks",
		"TBL_PROJECTS=tbl_projects",
		"",
	}, "\n")
	writeFile(t, filepath.Join(harness.homeDir, ".cc-connect", "board.env"), boardEnv, 0o644)
	writeFile(t, filepath.Join(tempDir, "records.json"), recordsJSON, 0o644)

	return harness
}

func (h boardCompleteProjectHarness) run(projectName string) (string, error) {
	h.t.Helper()
	cmd := exec.Command(filepath.Join(h.scriptDir, "board-complete-project.sh"), projectName)
	cmd.Env = append(os.Environ(),
		"HOME="+h.homeDir,
		"PATH="+h.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CC_PROJECT=team-leader",
		"RECORD_LIST_JSON="+h.recordsJSON(),
		"UPSERT_ARGS_LOG="+h.upsertArgs,
		"UPSERT_JSON_LOG="+h.upsertJSON,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (h boardCompleteProjectHarness) recordsJSON() string {
	h.t.Helper()
	recordsPath := filepath.Join(filepath.Dir(h.upsertArgs), "records.json")
	data, err := os.ReadFile(recordsPath)
	if err != nil {
		h.t.Fatalf("read records fixture: %v", err)
	}
	return string(data)
}

func (h boardCompleteProjectHarness) readUpsertArgs() string {
	h.t.Helper()
	data, err := os.ReadFile(h.upsertArgs)
	if err != nil {
		h.t.Fatalf("read upsert args: %v", err)
	}
	return string(data)
}

func (h boardCompleteProjectHarness) readUpsertJSON() map[string]any {
	h.t.Helper()
	data, err := os.ReadFile(h.upsertJSON)
	if err != nil {
		h.t.Fatalf("read upsert json: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		h.t.Fatalf("parse upsert json %q: %v", string(data), err)
	}
	return payload
}

func (h boardCompleteProjectHarness) assertNoUpsert() {
	h.t.Helper()
	if _, err := os.Stat(h.upsertArgs); !os.IsNotExist(err) {
		h.t.Fatalf("expected no upsert; stat error=%v", err)
	}
}
