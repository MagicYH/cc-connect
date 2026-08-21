package task_board_scripts

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSimpleBoardStartDefaultsToTeamLeaderOnly(t *testing.T) {
	h := newSimpleBoardHarness(t)
	workDir := filepath.Join(h.tempDir, "work", "demo")
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)

	out, err := h.run("boss", "demo-project", "demo requirement", workDir)

	if err != nil {
		t.Fatalf("expected simple board start to succeed; output:\n%s", out)
	}
	if !strings.Contains(out, "SIMPLE_BOARD_READY oc_simple_chat") {
		t.Fatalf("expected ready output with chat id; output:\n%s", out)
	}
	chatCreate := readOptionalFile(t, h.chatCreateLog)
	if !strings.Contains(chatCreate, "cli_tl") {
		t.Fatalf("expected team-leader bot in chat-create; args:\n%s", chatCreate)
	}
	if !strings.Contains(chatCreate, "cli_boss") {
		t.Fatalf("expected caller bot from config in chat-create; args:\n%s", chatCreate)
	}
	if strings.Contains(chatCreate, "cli_review") || strings.Contains(chatCreate, "cli_dev") || strings.Contains(chatCreate, "cli_test") {
		t.Fatalf("must not include optional roles by default; args:\n%s", chatCreate)
	}
	sends := readSendRecords(t, h.sendLog)
	expectedRoute := "/workspace route '" + workDir + "'"
	assertSendRecordCount(t, sends, "ou_tl", expectedRoute, 1)
	assertNoRecipients(t, sends, "ou_review", "ou_dev", "ou_test")
	assertTaskKickoffOnlyToTeamLeader(t, sends)
}

func TestSimpleBoardStartIncludesRequestedReviewer(t *testing.T) {
	h := newSimpleBoardHarness(t)
	workDir := filepath.Join(h.tempDir, "work", "demo")
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)

	out, err := h.run("boss", "demo-project", "demo requirement", workDir, "reviewer")

	if err != nil {
		t.Fatalf("expected simple board start to succeed with reviewer; output:\n%s", out)
	}
	chatCreate := readOptionalFile(t, h.chatCreateLog)
	if !strings.Contains(chatCreate, "cli_tl") || !strings.Contains(chatCreate, "cli_review") {
		t.Fatalf("expected team-leader and reviewer bots in chat-create; args:\n%s", chatCreate)
	}
	if strings.Contains(chatCreate, "cli_dev") || strings.Contains(chatCreate, "cli_test") {
		t.Fatalf("must not include unrequested roles; args:\n%s", chatCreate)
	}
	sends := readSendRecords(t, h.sendLog)
	for _, openID := range []string{"ou_tl", "ou_review"} {
		assertSendRecordCount(t, sends, openID, "/workspace route '"+workDir+"'", 1)
	}
	assertNoRecipients(t, sends, "ou_dev", "ou_test")
	assertTaskKickoffOnlyToTeamLeader(t, sends)
}

func TestSimpleBoardStartRejectsUnknownRoleBeforeCreatingGroup(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)

	out, err := h.run("boss", "demo-project", "demo requirement", filepath.Join(h.tempDir, "work", "demo"), "reviewer,architect")

	if err == nil {
		t.Fatalf("expected unknown role to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "unknown role") {
		t.Fatalf("expected unknown role error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartRejectsGlobRoleBeforeCreatingGroup(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)
	writeFile(t, filepath.Join(h.tempDir, "reviewer"), "", 0o644)

	out, err := h.runWithDir("boss", "demo-project", "demo requirement", filepath.Join(h.tempDir, "work", "demo"), h.tempDir, "*")

	if err == nil {
		t.Fatalf("expected glob role to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "invalid role") {
		t.Fatalf("expected invalid role error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartRejectsMissingCallerAppBeforeCreatingGroup(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)
	writeFile(t, filepath.Join(h.homeDir, ".cc-connect", "config.toml"), "", 0o600)

	out, err := h.run("boss", "demo-project", "demo requirement", filepath.Join(h.tempDir, "work", "demo"))

	if err == nil {
		t.Fatalf("expected missing caller app to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "caller bot app_id not found") {
		t.Fatalf("expected caller app error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartRejectsMissingWorkspaceBeforeCreatingGroup(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)

	out, err := h.runWithoutCreatingWorkspace("boss", "demo-project", "demo requirement", filepath.Join(h.tempDir, "missing"))

	if err == nil {
		t.Fatalf("expected missing workspace to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "工作目录不存在") {
		t.Fatalf("expected missing workspace error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartRejectsWorkspaceWithApostropheBeforeCreatingGroup(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)
	workDir := filepath.Join(h.tempDir, "work", "team's-demo")

	out, err := h.run("boss", "demo-project", "demo requirement", workDir)

	if err == nil {
		t.Fatalf("expected apostrophe workspace to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "工作目录不能包含单引号") {
		t.Fatalf("expected apostrophe workspace error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartRejectsWorkspaceWithEscBeforeCreatingGroup(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)
	workDir := filepath.Join(h.tempDir, "work", "bad\x1bworkspace")

	out, err := h.run("boss", "demo-project", "demo requirement", workDir)

	if err == nil {
		t.Fatalf("expected ESC workspace to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "工作目录包含控制字符") {
		t.Fatalf("expected control character error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartRejectsCanonicalWorkspaceWithControlCharsBeforeCreatingGroup(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)
	target := filepath.Join(h.tempDir, "bad\nworkspace")
	link := filepath.Join(h.tempDir, "safe-link")
	mustMkdirAll(t, target)
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}

	out, err := h.runWithoutCreatingWorkspace("boss", "demo-project", "demo requirement", link)

	if err == nil {
		t.Fatalf("expected canonical workspace with control chars to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "工作目录包含控制字符") {
		t.Fatalf("expected control character error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartRejectsFileTaskInput(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)
	secretPath := filepath.Join(h.homeDir, ".cc-connect", "config.toml")
	writeFile(t, secretPath, "app_secret = \"real-secret\"", 0o600)

	out, err := h.run("boss", "demo-project", "@"+secretPath, filepath.Join(h.tempDir, "work", "demo"))

	if err == nil {
		t.Fatalf("expected @file input to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "@file task input is not supported") {
		t.Fatalf("expected @file rejection; output:\n%s", out)
	}
	for _, record := range readSendRecords(t, h.sendLog) {
		if strings.Contains(record.Message, "real-secret") {
			t.Fatalf("must not send file contents; records=%+v", readSendRecords(t, h.sendLog))
		}
	}
}

func TestSimpleBoardStartLogsOneSimpleTaskRowWithoutCron(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)
	workDir := filepath.Join(h.tempDir, "work", "demo")

	out, err := h.run("boss", "demo-project", "demo requirement", workDir, "reviewer")

	if err != nil {
		t.Fatalf("expected simple board start to succeed; output:\n%s", out)
	}
	if !strings.Contains(out, "SIMPLE_BOARD_READY oc_simple_chat") {
		t.Fatalf("expected ready output after Bitable log succeeds; output:\n%s", out)
	}
	calls := readOptionalFile(t, h.larkLog)
	if strings.Contains(calls, "cron") || strings.Contains(calls, "timer") {
		t.Fatalf("simple board start must not use scheduled polling; calls:\n%s", calls)
	}

	upserts := readRecordUpsertPayloads(t, h.recordUpsertLog)
	if len(upserts) != 1 {
		t.Fatalf("expected exactly one Bitable record upsert, got %d: %+v", len(upserts), upserts)
	}
	upsert := upserts[0]
	assertArgPrefix(t, upsert.Args, "base", "+record-upsert")
	assertFlagValue(t, upsert.Args, "--base-token", "simple_base_token")
	assertFlagValue(t, upsert.Args, "--table-id", "simple_table_id")
	assertNoArg(t, upsert.Args, "--view-id")
	assertFlagValue(t, upsert.Args, "--as", "user")

	assertStringField(t, upsert.Fields, "主任务名", "demo-project")
	assertStringField(t, upsert.Fields, "项目状态", "进行中")
	assertStringField(t, upsert.Fields, "需求描述", "demo requirement")
	assertStringField(t, upsert.Fields, "参与角色", "team-leader,reviewer")
	assertStringField(t, upsert.Fields, "发起人", "ou_writer")
	assertGroupChatField(t, upsert.Fields, "工作群", "oc_simple_chat")
}

func TestSimpleBoardStartRequiresSimpleTasksConfig(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnvWithoutSimpleTasks(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)

	out, err := h.run("boss", "demo-project", "demo requirement", filepath.Join(h.tempDir, "work", "demo"))

	if err == nil {
		t.Fatalf("expected missing Simple Tasks config to fail; output:\n%s", out)
	}
	if !strings.Contains(out, "SIMPLE_TASKS_BASE_TOKEN not set") {
		t.Fatalf("expected missing Simple Tasks base token error; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); chatCreate != "" {
		t.Fatalf("must fail before creating group; args:\n%s", chatCreate)
	}
}

func TestSimpleBoardStartFailsWhenBitableLoggingFails(t *testing.T) {
	h := newSimpleBoardHarness(t)
	writeSimpleBoardEnv(t, h.homeDir)
	writeSimpleLarkCli(t, h.binDir)

	out, err := h.runWithExtraEnv("boss", "demo-project", "demo requirement", filepath.Join(h.tempDir, "work", "demo"), []string{"SIMPLE_BOARD_FAIL_RECORD_UPSERT=1"})

	if err == nil {
		t.Fatalf("expected Bitable failure to fail script; output:\n%s", out)
	}
	if !strings.Contains(out, "FATAL: Simple Tasks record write failed") {
		t.Fatalf("expected explicit Bitable failure; output:\n%s", out)
	}
	if strings.Contains(out, "SIMPLE_BOARD_READY") {
		t.Fatalf("must not report ready when Bitable logging fails; output:\n%s", out)
	}
	if strings.Contains(out, "demo requirement") || strings.Contains(out, "--json") {
		t.Fatalf("must not leak Bitable payload on write failure; output:\n%s", out)
	}
	if chatCreate := readOptionalFile(t, h.chatCreateLog); !strings.Contains(chatCreate, "im +chat-create") {
		t.Fatalf("expected failure after chat creation; chat-create log=%s", chatCreate)
	}
}

type simpleSendRecord struct {
	Chat    string `json:"chat"`
	At      string `json:"at"`
	Message string `json:"message"`
}

type simpleRecordUpsert struct {
	Args   []string       `json:"args"`
	Fields map[string]any `json:"fields"`
}

func readSendRecords(t *testing.T, path string) []simpleSendRecord {
	t.Helper()
	raw := strings.TrimSpace(readOptionalFile(t, path))
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	records := make([]simpleSendRecord, 0, len(lines))
	for _, line := range lines {
		var record simpleSendRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("parse send log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func readRecordUpsertPayloads(t *testing.T, path string) []simpleRecordUpsert {
	t.Helper()
	raw := strings.TrimSpace(readOptionalFile(t, path))
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	records := make([]simpleRecordUpsert, 0, len(lines))
	for _, line := range lines {
		var record simpleRecordUpsert
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("parse record-upsert log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func assertArgPrefix(t *testing.T, args []string, want ...string) {
	t.Helper()
	if len(args) < len(want) {
		t.Fatalf("expected args to start with %v, got %v", want, args)
	}
	for i, value := range want {
		if args[i] != value {
			t.Fatalf("expected args to start with %v, got %v", want, args)
		}
	}
}

func assertFlagValue(t *testing.T, args []string, flag string, want string) {
	t.Helper()
	for i, arg := range args {
		if arg == flag {
			if i+1 >= len(args) {
				t.Fatalf("expected value after %s in args %v", flag, args)
			}
			if args[i+1] != want {
				t.Fatalf("expected %s %q, got %q in args %v", flag, want, args[i+1], args)
			}
			return
		}
	}
	t.Fatalf("expected flag %s in args %v", flag, args)
}

func assertNoArg(t *testing.T, args []string, forbidden string) {
	t.Helper()
	for _, arg := range args {
		if arg == forbidden {
			t.Fatalf("did not expect arg %s in args %v", forbidden, args)
		}
	}
}

func assertStringField(t *testing.T, fields map[string]any, name string, want string) {
	t.Helper()
	got, ok := fields[name].(string)
	if !ok || got != want {
		t.Fatalf("expected field %s=%q, got %#v in fields=%+v", name, want, fields[name], fields)
	}
}

func assertGroupChatField(t *testing.T, fields map[string]any, name string, wantID string) {
	t.Helper()
	raw, ok := fields[name].([]any)
	if !ok || len(raw) != 1 {
		t.Fatalf("expected one group chat entry for %s, got %#v", name, fields[name])
	}
	entry, ok := raw[0].(map[string]any)
	if !ok || entry["id"] != wantID {
		t.Fatalf("expected group chat id %q for %s, got %#v", wantID, name, fields[name])
	}
}

func assertSendRecordCount(t *testing.T, records []simpleSendRecord, at string, message string, want int) {
	t.Helper()
	got := 0
	for _, record := range records {
		if record.Chat == "oc_simple_chat" && record.At == at && record.Message == message {
			got++
		}
	}
	if got != want {
		t.Fatalf("expected %d send records to %s with exact message %q, got %d; records=%+v", want, at, message, got, records)
	}
}

func assertNoRecipients(t *testing.T, records []simpleSendRecord, recipients ...string) {
	t.Helper()
	for _, record := range records {
		for _, recipient := range recipients {
			if record.At == recipient {
				t.Fatalf("unexpected send to %s; records=%+v", recipient, records)
			}
		}
	}
}

func assertTaskKickoffOnlyToTeamLeader(t *testing.T, records []simpleSendRecord) {
	t.Helper()
	teamLeaderKickoffs := 0
	for _, record := range records {
		if strings.HasPrefix(record.Message, "新任务") {
			if record.Chat != "oc_simple_chat" || record.At != "ou_tl" {
				t.Fatalf("task kickoff must only target team-leader in the created chat; records=%+v", records)
			}
			teamLeaderKickoffs++
		}
	}
	if teamLeaderKickoffs != 1 {
		t.Fatalf("expected one task kickoff to team-leader, got %d; records=%+v", teamLeaderKickoffs, records)
	}
}

type simpleBoardHarness struct {
	tempDir         string
	homeDir         string
	binDir          string
	scriptDir       string
	sendLog         string
	larkLog         string
	chatCreateLog   string
	recordUpsertLog string
}

func newSimpleBoardHarness(t *testing.T) simpleBoardHarness {
	t.Helper()
	repoRoot := findRepoRoot(t)
	tempDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(tempDir); err == nil {
		tempDir = resolved
	}
	h := simpleBoardHarness{
		tempDir:         tempDir,
		homeDir:         filepath.Join(tempDir, "home"),
		binDir:          filepath.Join(tempDir, "bin"),
		scriptDir:       filepath.Join(tempDir, "scripts"),
		sendLog:         filepath.Join(tempDir, "send.log"),
		larkLog:         filepath.Join(tempDir, "lark.log"),
		chatCreateLog:   filepath.Join(tempDir, "chat-create.log"),
		recordUpsertLog: filepath.Join(tempDir, "record-upsert.log"),
	}
	mustMkdirAll(t, filepath.Join(h.homeDir, ".cc-connect"))
	writeSimpleConfig(t, h.homeDir)
	mustMkdirAll(t, h.binDir)
	mustMkdirAll(t, h.scriptDir)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "simple-task-board", "scripts", "simple-board-start.sh"),
		filepath.Join(h.scriptDir, "simple-board-start.sh"), 0o755)
	writeFile(t, filepath.Join(h.scriptDir, "board-send.sh"), `#!/usr/bin/env bash
set -euo pipefail
python3 - "$SIMPLE_BOARD_SEND_LOG" "$1" "$2" "${*:3}" <<'PY'
import json, sys
path, chat, at, message = sys.argv[1:]
with open(path, "a", encoding="utf-8") as f:
    f.write(json.dumps({"chat": chat, "at": at, "message": message}, ensure_ascii=False) + "\n")
PY
echo OK fake-message
`, 0o755)
	return h
}

func (h simpleBoardHarness) run(project string, name string, requirement string, workDir string, roles ...string) (string, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", err
	}
	return h.runWithoutCreatingWorkspace(project, name, requirement, workDir, roles...)
}

func (h simpleBoardHarness) runWithExtraEnv(project string, name string, requirement string, workDir string, extraEnv []string, roles ...string) (string, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", err
	}
	return h.runWithoutCreatingWorkspaceWithOptions(project, name, requirement, workDir, extraEnv, "", roles...)
}

func (h simpleBoardHarness) runWithDir(project string, name string, requirement string, workDir string, dir string, roles ...string) (string, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", err
	}
	return h.runWithoutCreatingWorkspaceWithOptions(project, name, requirement, workDir, nil, dir, roles...)
}

func (h simpleBoardHarness) runWithoutCreatingWorkspace(project string, name string, requirement string, workDir string, roles ...string) (string, error) {
	return h.runWithoutCreatingWorkspaceWithOptions(project, name, requirement, workDir, nil, "", roles...)
}

func (h simpleBoardHarness) runWithoutCreatingWorkspaceWithOptions(project string, name string, requirement string, workDir string, extraEnv []string, dir string, roles ...string) (string, error) {
	args := []string{name, requirement, workDir}
	args = append(args, roles...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(h.scriptDir, "simple-board-start.sh"), args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = []string{
		"HOME=" + h.homeDir,
		"PATH=" + h.binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"CC_PROJECT=" + project,
		"BOARD_ENV=" + filepath.Join(h.homeDir, ".cc-connect", "board.env"),
		"SIMPLE_BOARD_SEND_LOG=" + h.sendLog,
		"SIMPLE_BOARD_LARK_LOG=" + h.larkLog,
		"SIMPLE_BOARD_CHAT_CREATE_LOG=" + h.chatCreateLog,
		"SIMPLE_BOARD_RECORD_UPSERT_LOG=" + h.recordUpsertLog,
	}
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeSimpleBoardEnv(t *testing.T, homeDir string) {
	t.Helper()
	boardEnv := strings.Join([]string{
		"BOT_APPID_team_leader=cli_tl",
		"BOT_OPENID_team_leader=ou_tl",
		"BOT_APPID_reviewer=cli_review",
		"BOT_OPENID_reviewer=ou_review",
		"BOT_APPID_developer=cli_dev",
		"BOT_OPENID_developer=ou_dev",
		"BOT_APPID_tester=cli_test",
		"BOT_OPENID_tester=ou_test",
		"BOARD_WRITER_OPENID=ou_writer",
		"SIMPLE_TASKS_BASE_TOKEN=simple_base_token",
		"SIMPLE_TASKS_TABLE_ID=simple_table_id",
		"",
	}, "\n")
	writeFile(t, filepath.Join(homeDir, ".cc-connect", "board.env"), boardEnv, 0o644)
}

func writeSimpleBoardEnvWithoutSimpleTasks(t *testing.T, homeDir string) {
	t.Helper()
	boardEnv := strings.Join([]string{
		"BOT_APPID_team_leader=cli_tl",
		"BOT_OPENID_team_leader=ou_tl",
		"BOT_APPID_reviewer=cli_review",
		"BOT_OPENID_reviewer=ou_review",
		"BOT_APPID_developer=cli_dev",
		"BOT_OPENID_developer=ou_dev",
		"BOT_APPID_tester=cli_test",
		"BOT_OPENID_tester=ou_test",
		"BOARD_WRITER_OPENID=ou_writer",
		"",
	}, "\n")
	writeFile(t, filepath.Join(homeDir, ".cc-connect", "board.env"), boardEnv, 0o644)
}

func writeSimpleConfig(t *testing.T, homeDir string) {
	t.Helper()
	config := strings.Join([]string{
		"[[projects]]",
		"name = \"boss\"",
		"[[projects.platforms]]",
		"type = \"feishu\"",
		"[projects.platforms.options]",
		"app_id = \"cli_boss\"",
		"app_secret = \"fake\"",
		"",
	}, "\n")
	writeFile(t, filepath.Join(homeDir, ".cc-connect", "config.toml"), config, 0o600)
}

func writeSimpleLarkCli(t *testing.T, binDir string) {
	t.Helper()
	writeFile(t, filepath.Join(binDir, "lark-cli"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s
' "$*" >> "$SIMPLE_BOARD_LARK_LOG"
if [[ "$*" == *"im +chat-create"* ]]; then
  printf '%s
' "$*" >> "$SIMPLE_BOARD_CHAT_CREATE_LOG"
  printf '[lark-cli] [WARN] proxy detected\n'
  printf '{"data":{"chat_id":"oc_simple_chat"}}\n'
elif [[ "$*" == *"base +record-upsert"* ]]; then
  if [[ "${SIMPLE_BOARD_FAIL_RECORD_UPSERT:-}" == "1" ]]; then
    printf 'record-upsert failed intentionally: %s\n' "$*" >&2
    exit 1
  fi
  python3 - "$SIMPLE_BOARD_RECORD_UPSERT_LOG" "$@" <<'PYEOF'
import json
import sys
path = sys.argv[1]
args = sys.argv[2:]
json_payload = None
for idx, arg in enumerate(args):
    if arg == "--json" and idx + 1 < len(args):
        json_payload = args[idx + 1]
        break
if json_payload is None:
    raise SystemExit("missing --json payload")
with open(path, "a", encoding="utf-8") as f:
    f.write(json.dumps({"args": args, "fields": json.loads(json_payload)}, ensure_ascii=False) + "\n")
PYEOF
  printf '{"data":{"record":{"record_id":"rec_simple"}}}\n'
else
  printf 'unexpected lark-cli call: %s\n' "$*" >&2
  exit 1
fi
`, 0o755)
}
