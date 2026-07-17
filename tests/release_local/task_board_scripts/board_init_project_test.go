package task_board_scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoardInitProjectFailsWhenWorkspaceBindingsIncomplete(t *testing.T) {
	h := newBoardInitHarness(t)
	writeInitBoardEnv(t, h.homeDir, h.tempDir)
	writeInitConfig(t, h.homeDir, "feishu")
	writeInitLarkCli(t, h.binDir, "")

	out, err := h.run("developer", "demo-project", "demo requirement", filepath.Join(h.tempDir, "work", "demo"))

	if err == nil {
		t.Fatalf("expected init to fail when fewer than 4 workspace bindings exist; output:\n%s", out)
	}
	if strings.Contains(out, "PROJECT_READY") {
		t.Fatalf("must not print PROJECT_READY when workspace bindings are incomplete; output:\n%s", out)
	}

	sends := readOptionalFile(t, h.sendLog)
	if strings.Contains(sends, "workspace 已绑定") {
		t.Fatalf("must not send TL ready message claiming workspace is bound; sends:\n%s", sends)
	}
	upserts := readOptionalFile(t, h.upsertLog)
	if !strings.Contains(upserts, `"项目状态":"初始化失败"`) {
		t.Fatalf("expected project to be marked 初始化失败; upserts:\n%s", upserts)
	}
}

func TestBoardInitProjectMarksFailedWhenWorkspaceInitSendFails(t *testing.T) {
	h := newBoardInitHarness(t)
	writeInitBoardEnv(t, h.homeDir, h.tempDir)
	writeInitConfig(t, h.homeDir, "feishu")
	writeInitLarkCli(t, h.binDir, "")
	writeFile(t, filepath.Join(h.scriptDir, "board-send.sh"), "#!/usr/bin/env bash\nset -euo pipefail\necho send failed >&2\nexit 1\n", 0o755)

	out, err := h.run("developer", "demo-project", "demo requirement", filepath.Join(h.tempDir, "work", "demo"))

	if err == nil {
		t.Fatalf("expected init to fail when workspace init message cannot be sent; output:\n%s", out)
	}
	upserts := readOptionalFile(t, h.upsertLog)
	if !strings.Contains(upserts, `"项目状态":"初始化失败"`) {
		t.Fatalf("expected project to be marked 初始化失败 after send failure; upserts:\n%s", upserts)
	}
}

func TestBoardInitProjectRequiresRoleBindingsForRequestedWorkspace(t *testing.T) {
	h := newBoardInitHarness(t)
	workDir := filepath.Join(h.tempDir, "work", "demo")
	wrongDir := filepath.Join(h.tempDir, "work", "wrong")
	writeInitBoardEnv(t, h.homeDir, h.tempDir)
	writeInitConfig(t, h.homeDir, "feishu")
	writeWorkspaceBindings(t, filepath.Join(h.homeDir, ".cc-connect", "workspace_bindings.json"), map[string]string{
		"team-leader": wrongDir,
		"developer":   workDir,
		"tester":      workDir,
		"reviewer":    workDir,
		"boss":        workDir,
	}, "feishu", "oc_test_chat")
	writeInitLarkCli(t, h.binDir, "")

	out, err := h.run("developer", "demo-project", "demo requirement", workDir)

	if err == nil {
		t.Fatalf("expected init to fail when a required role is bound to the wrong workspace; output:\n%s", out)
	}
	if strings.Contains(out, "PROJECT_READY") {
		t.Fatalf("must not print PROJECT_READY when a required role binding points elsewhere; output:\n%s", out)
	}
}

func TestBoardInitProjectUsesConfigDataDirAndPlatformKeys(t *testing.T) {
	h := newBoardInitHarness(t)
	workDir := filepath.Join(h.tempDir, "work", "demo")
	dataDir := filepath.Join(h.tempDir, "state")
	writeInitBoardEnv(t, h.homeDir, h.tempDir)
	writeInitConfigWithDataDir(t, h.homeDir, "lark", dataDir)
	writeWorkspaceBindings(t, filepath.Join(dataDir, "workspace_bindings.json"), map[string]string{
		"team-leader": workDir,
		"developer":   workDir,
		"tester":      workDir,
		"reviewer":    workDir,
	}, "lark", "oc_test_chat")
	writeInitLarkCli(t, h.binDir, "")

	out, err := h.run("developer", "demo-project", "demo requirement", workDir)

	if err != nil {
		t.Fatalf("expected init to find lark bindings in configured data_dir; output:\n%s", out)
	}
	if !strings.Contains(out, "PROJECT_READY oc_test_chat") {
		t.Fatalf("expected PROJECT_READY after all configured bindings exist; output:\n%s", out)
	}
}

func TestBoardInitProjectAddsCallerBotFromConfig(t *testing.T) {
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
	writeInitLarkCli(t, h.binDir, "cli_boss")

	out, err := h.run("boss", "demo-project", "demo requirement", workDir)

	if err != nil {
		t.Fatalf("expected init to include boss app from config in created chat; output:\n%s", out)
	}
}

func TestBoardInitProjectQuotesWorkspacePathInRouteMessage(t *testing.T) {
	h := newBoardInitHarness(t)
	workDir := filepath.Join(h.tempDir, "work", "demo project")
	writeInitBoardEnv(t, h.homeDir, h.tempDir)
	writeInitConfig(t, h.homeDir, "feishu")
	writeInitLarkCli(t, h.binDir, "")

	_, _ = h.run("developer", "demo-project", "demo requirement", workDir)

	sends := readOptionalFile(t, h.sendLog)
	if !strings.Contains(sends, "/workspace route '"+workDir+"'") {
		t.Fatalf("expected workspace route message to quote path with spaces; sends:\n%s", sends)
	}
}

type boardInitHarness struct {
	tempDir   string
	homeDir   string
	binDir    string
	scriptDir string
	sendLog   string
	upsertLog string
}

func newBoardInitHarness(t *testing.T) boardInitHarness {
	t.Helper()
	repoRoot := findRepoRoot(t)
	tempDir := t.TempDir()
	// 脚本用 pwd -P 归一为物理路径（真机 cc-connect 的 /workspace route 亦把路径解析成物理路径存 binding）；
	// macOS 的 /var 是 /private/var 的 symlink，t.TempDir() 返回逻辑路径 → 与脚本物理路径不一致会让
	// 绑定轮询/route 消息断言 mismatch。解析成物理路径使 fixture 与脚本行为对齐。
	if resolved, err := filepath.EvalSymlinks(tempDir); err == nil {
		tempDir = resolved
	}
	h := boardInitHarness{
		tempDir:   tempDir,
		homeDir:   filepath.Join(tempDir, "home"),
		binDir:    filepath.Join(tempDir, "bin"),
		scriptDir: filepath.Join(tempDir, "scripts"),
		sendLog:   filepath.Join(tempDir, "send.log"),
		upsertLog: filepath.Join(tempDir, "upsert.log"),
	}

	mustMkdirAll(t, filepath.Join(h.homeDir, ".cc-connect"))
	mustMkdirAll(t, h.binDir)
	mustMkdirAll(t, h.scriptDir)

	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-init-project.sh"),
		filepath.Join(h.scriptDir, "board-init-project.sh"), 0o755)
	copyFile(t,
		filepath.Join(repoRoot, "plugins", "cc-connect-skills", "skills", "task-board", "scripts", "board-lib.sh"),
		filepath.Join(h.scriptDir, "board-lib.sh"), 0o644)
	writeFile(t, filepath.Join(h.scriptDir, "board-send.sh"), "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\t%s\\t%s\\n' \"$1\" \"$2\" \"${*:3}\" >> \"$BOARD_SEND_LOG\"\necho OK fake-message\n", 0o755)
	writeFile(t, filepath.Join(h.binDir, "sleep"), "#!/usr/bin/env bash\nexit 0\n", 0o755)

	return h
}

func (h boardInitHarness) run(project string, name string, requirement string, workDir string) (string, error) {
	cmd := exec.Command(filepath.Join(h.scriptDir, "board-init-project.sh"), name, requirement, workDir)
	cmd.Env = append(os.Environ(),
		"HOME="+h.homeDir,
		"PATH="+h.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CC_PROJECT="+project,
		"BOARD_SEND_LOG="+h.sendLog,
		"UPSERT_LOG="+h.upsertLog,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeInitBoardEnv(t *testing.T, homeDir string, tempDir string) {
	t.Helper()
	boardEnv := strings.Join([]string{
		"BOARD_BASE=base_token",
		"TBL_TASKS=tbl_tasks",
		"TBL_PROJECTS=tbl_projects",
		"BOT_APPID_team_leader=cli_tl",
		"BOT_OPENID_team_leader=ou_tl",
		"BOT_APPID_developer=cli_dev",
		"BOT_OPENID_developer=ou_dev",
		"BOT_APPID_tester=cli_test",
		"BOT_OPENID_tester=ou_test",
		"BOT_APPID_reviewer=cli_review",
		"BOT_OPENID_reviewer=ou_review",
		"BOARD_WRITER_OPENID=ou_writer",
		"PROJECTS_BASE_DIR=" + filepath.Join(tempDir, "projects"),
		"",
	}, "\n")
	writeFile(t, filepath.Join(homeDir, ".cc-connect", "board.env"), boardEnv, 0o644)
}

func writeInitConfig(t *testing.T, homeDir string, platformType string) {
	t.Helper()
	writeInitConfigWithDataDir(t, homeDir, platformType, "")
}

func writeInitConfigWithDataDir(t *testing.T, homeDir string, platformType string, dataDir string) {
	t.Helper()
	projects := []struct{ name, appID string }{
		{"team-leader", "cli_tl"},
		{"developer", "cli_dev"},
		{"tester", "cli_test"},
		{"reviewer", "cli_review"},
		{"boss", "cli_boss"},
	}
	var b strings.Builder
	b.WriteString("data_dir = \"")
	b.WriteString(dataDir)
	b.WriteString("\"\n")
	for _, p := range projects {
		b.WriteString("\n[[projects]]\n")
		b.WriteString("name = \"")
		b.WriteString(p.name)
		b.WriteString("\"\n")
		b.WriteString("[[projects.platforms]]\n")
		b.WriteString("type = \"")
		b.WriteString(platformType)
		b.WriteString("\"\n")
		b.WriteString("[projects.platforms.options]\n")
		b.WriteString("app_id = \"")
		b.WriteString(p.appID)
		b.WriteString("\"\n")
		b.WriteString("app_secret = \"secret\"\n")
	}
	writeFile(t, filepath.Join(homeDir, ".cc-connect", "config.toml"), b.String(), 0o644)
}

func writeWorkspaceBindings(t *testing.T, path string, workspaces map[string]string, platform string, chat string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	var b strings.Builder
	b.WriteString("{\n")
	i := 0
	for project, workspace := range workspaces {
		if i > 0 {
			b.WriteString(",\n")
		}
		i++
		b.WriteString(`  "project:`)
		b.WriteString(project)
		b.WriteString(`": { "`)
		b.WriteString(platform)
		b.WriteString(":" + chat)
		b.WriteString(`": { "workspace": "`)
		b.WriteString(strings.ReplaceAll(workspace, `\`, `\\`))
		b.WriteString(`" } }`)
	}
	b.WriteString("\n}\n")
	writeFile(t, path, b.String(), 0o644)
}

func writeInitLarkCli(t *testing.T, binDir string, requiredBot string) {
	t.Helper()
	writeFile(t, filepath.Join(binDir, "lark-cli"), `#!/usr/bin/env bash
set -euo pipefail
required_bot="`+requiredBot+`"
if [[ "$*" == *"im +chat-create"* ]]; then
  if [[ -n "$required_bot" && "$*" != *"$required_bot"* ]]; then
    printf 'missing required bot %s in chat-create: %s\n' "$required_bot" "$*" >&2
    exit 1
  fi
  printf '{"data":{"chat_id":"oc_test_chat"}}\n'
elif [[ "$*" == *"base +record-upsert"* ]]; then
  printf '%s\n' "$*" >> "$UPSERT_LOG"
  printf '{"data":{"record":{"record_id_list":["rec_project"]}}}\n'
else
  printf 'unexpected lark-cli call: %s\n' "$*" >&2
  exit 1
fi
`, 0o755)
}

func readOptionalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", wd)
		}
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func copyFile(t *testing.T, src string, dst string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	writeFile(t, dst, string(data), mode)
}
