# Streaming Card Initial Footer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** ./../specs/2026-07-09-streaming-card-initial-footer-design.md
**Verification:** ./../specs/2026-07-09-streaming-card-initial-footer-verification.md

**Goal:** Show model and cwd/session metadata in Feishu streaming rich-card footers before the final response update.

**Architecture:** Keep footer ownership in `core.Engine`: `composeRichStatusFooter(streaming=true, ...)` delegates to a new small helper that composes stable initial metadata. Reuse `replyFooterModel` and `replyFooterWorkDir` so model and cwd/session sources match the final footer, while leaving the non-streaming branch unchanged.

**Tech Stack:** Go, existing cc-connect `core` package, existing Feishu rich-card JSON renderer, Go unit tests via `go test`.

## Global Constraints

- Work in `/data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer`.
- Only modify Feishu rich/streaming card behavior through `core.Engine` footer composition and renderer regression tests.
- Do not add configuration, dependencies, database changes, external API calls, or new Feishu patch calls.
- Respect existing footer toggles: `replyFooterEnabled` disables all rich-card footer output, `showContextIndicator` controls model visibility, and `showWorkdirIndicator` controls cwd/session visibility.
- Keep `composeRichStatusFooter(streaming=false, ...)` final footer behavior unchanged.
- Use TDD: write failing tests first, run them, then implement the minimal code.

---

## File Structure

- Modify `core/engine.go`: replace the streaming early return in `composeRichStatusFooter` with a helper call and add `composeInitialRichStatusFooter` near existing footer helpers.
- Modify `core/engine_test.go`: add unit tests near `TestReplyFooterWorkDir_AppendsSessionID` for streaming initial footer composition, recomputation, toggles, missing metadata, and final footer regression.
- Modify `platform/feishu/platform_test.go`: add one renderer regression test near existing `TestBuildRichCard_*` tests to assert multi-line streaming footer rendering as grey notation blocks.

---

### Task 1: Core Streaming Footer Composition
<!-- covers: S1, S2, S3, S4 -->

**Files:**
- Modify: `core/engine.go:6844-6884`
- Test: `core/engine_test.go` near `TestReplyFooterWorkDir_AppendsSessionID`

**Interfaces:**
- Consumes: existing `replyFooterModel(session AgentSession, agent Agent) string`, `replyFooterWorkDir(session AgentSession, agent Agent, workspaceDir string) string`, `Engine.SetReplyFooterEnabled(bool)`, `Engine.SetShowContextIndicator(bool)`, `Engine.SetShowWorkdirIndicator(bool)`.
- Produces: new method `func (e *Engine) composeInitialRichStatusFooter(agent Agent, session AgentSession, workspaceDir string) string` returning a newline-separated footer string for streaming rich cards.

- [ ] **Step 1: Write failing tests for streaming initial footer metadata and recomputation**

Add these tests immediately after `TestReplyFooterWorkDir_AppendsSessionID` in `core/engine_test.go`:

```go
func TestComposeRichStatusFooter_StreamingInitialMetadata(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
	e.SetReplyFooterEnabled(true)
	e.SetShowContextIndicator(true)
	e.SetShowWorkdirIndicator(true)

	session := &controllableAgentSession{
		sessionID: "sess-stream-123",
		alive:     true,
		events:    make(chan Event, 1),
		closed:    make(chan struct{}),
		model:     "claude-test-model",
	}

	got := e.composeRichStatusFooter(true, time.Now(), &stubAgent{}, session, "/tmp/cc-connect-fixture")
	want := "model: claude-test-model\ncwd: /tmp/cc-connect-fixture · sess-stream-123"
	if got != want {
		t.Fatalf("composeRichStatusFooter(streaming=true) = %q, want %q", got, want)
	}
}

func TestComposeRichStatusFooter_StreamingRecomputesLateSessionID(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
	e.SetReplyFooterEnabled(true)
	e.SetShowContextIndicator(true)
	e.SetShowWorkdirIndicator(true)

	session := &controllableAgentSession{
		alive:  true,
		events: make(chan Event, 1),
		closed: make(chan struct{}),
		model:  "claude-test-model",
	}

	first := e.composeRichStatusFooter(true, time.Now(), &stubAgent{}, session, "/tmp/cc-connect-fixture")
	if first != "model: claude-test-model\ncwd: /tmp/cc-connect-fixture" {
		t.Fatalf("first streaming footer = %q", first)
	}

	session.sessionID = "sess-late-456"
	second := e.composeRichStatusFooter(true, time.Now(), &stubAgent{}, session, "/tmp/cc-connect-fixture")
	if second != "model: claude-test-model\ncwd: /tmp/cc-connect-fixture · sess-late-456" {
		t.Fatalf("second streaming footer = %q", second)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core -run 'TestComposeRichStatusFooter_StreamingInitialMetadata|TestComposeRichStatusFooter_StreamingRecomputesLateSessionID' -count=1
```

Expected: FAIL because `composeRichStatusFooter(streaming=true, ...)` still returns an empty string.

- [ ] **Step 3: Write failing tests for footer toggles and final footer regression**

Add these tests after the two tests from Step 1:

```go
func TestComposeRichStatusFooter_StreamingRespectsFooterToggles(t *testing.T) {
	session := &controllableAgentSession{
		sessionID: "sess-toggle-123",
		alive:     true,
		events:    make(chan Event, 1),
		closed:    make(chan struct{}),
		model:     "claude-test-model",
	}

	tests := []struct {
		name       string
		master     bool
		showCtx    bool
		showWork   bool
		want       string
		forbidden  []string
	}{
		{
			name:      "master off suppresses all footer text",
			master:    false,
			showCtx:   true,
			showWork:  true,
			want:      "",
		},
		{
			name:      "context off hides model only",
			master:    true,
			showCtx:   false,
			showWork:  true,
			want:      "cwd: /tmp/cc-connect-fixture · sess-toggle-123",
			forbidden: []string{"claude-test-model", "model:"},
		},
		{
			name:      "workdir off hides cwd and session only",
			master:    true,
			showCtx:   true,
			showWork:  false,
			want:      "model: claude-test-model",
			forbidden: []string{"cwd:", "sess-toggle-123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
			e.SetReplyFooterEnabled(tt.master)
			e.SetShowContextIndicator(tt.showCtx)
			e.SetShowWorkdirIndicator(tt.showWork)

			got := e.composeRichStatusFooter(true, time.Now(), &stubAgent{}, session, "/tmp/cc-connect-fixture")
			if got != tt.want {
				t.Fatalf("streaming footer = %q, want %q", got, tt.want)
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(got, forbidden) {
					t.Fatalf("streaming footer %q contains forbidden %q", got, forbidden)
				}
			}
		})
	}
}

func TestComposeRichStatusFooter_FinalBranchUnchanged(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
	e.SetReplyFooterEnabled(true)
	e.SetShowContextIndicator(true)
	e.SetShowWorkdirIndicator(true)

	session := &controllableAgentSession{
		sessionID: "sess-final-789",
		alive:     true,
		events:    make(chan Event, 1),
		closed:    make(chan struct{}),
		model:     "claude-test-model",
		contextUsage: &ContextUsage{
			UsedTokens:               200,
			InputTokens:              40,
			CachedInputTokens:        60,
			CacheCreationInputTokens: 20,
			OutputTokens:             10,
			ContextWindow:            1000,
		},
	}

	got := e.composeRichStatusFooter(false, time.Now().Add(-2*time.Second), &stubAgent{}, session, "/tmp/cc-connect-fixture")
	for _, want := range []string{"claude-test-model", "out 10", "in 40", "cw 20", "cr 60", "ctx 20%", "/tmp/cc-connect-fixture · sess-final-789"} {
		if !strings.Contains(got, want) {
			t.Fatalf("final footer %q missing %q", got, want)
		}
	}
	for _, forbidden := range []string{"model:", "cwd:"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("final footer %q contains initial label %q", got, forbidden)
		}
	}
}
```

- [ ] **Step 4: Run tests and verify they fail before implementation**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core -run 'TestComposeRichStatusFooter_Streaming|TestComposeRichStatusFooter_FinalBranchUnchanged' -count=1
```

Expected: streaming toggle tests fail because the streaming branch still returns empty; final regression test should already pass or fail only if the current final footer behavior differs from the spec.

- [ ] **Step 5: Implement the minimal core change**

In `core/engine.go`, replace the streaming early return and add the helper near `composeRichStatusFooter`:

```go
func (e *Engine) composeRichStatusFooter(streaming bool, turnStart time.Time, agent Agent, session AgentSession, workspaceDir string) string {
	if !e.replyFooterEnabled {
		return ""
	}
	if streaming {
		return e.composeInitialRichStatusFooter(agent, session, workspaceDir)
	}
	var lines []string

	// Line 1: elapsed timer (now always the "done" form since streaming branch returned above)
	lines = append(lines, formatElapsed(time.Since(turnStart), streaming, e.i18n.currentLang()))

	// Line 2: model + effort + token usage detail + ctx %
	if e.showContextIndicator {
		usage := replyFooterSessionContextUsage(session)
		model := replyFooterModel(session, agent)
		effort := replyFooterReasoningEffort(session, agent)
		if line := buildClaudeStatusLineFooter(model, effort, usage); line != "" {
			lines = append(lines, line)
		} else if fallback := e.replyFooterUsageText(session, agent); fallback != "" {
			parts := []string{}
			if model != "" {
				parts = append(parts, model)
			}
			if effort != "" {
				parts = append(parts, effort)
			}
			parts = append(parts, fallback)
			lines = append(lines, strings.Join(parts, " · "))
		}
	}

	// Line 3: workdir
	if e.showWorkdirIndicator {
		if dir := replyFooterWorkDir(session, agent, workspaceDir); dir != "" {
			lines = append(lines, dir)
		}
	}

	return strings.Join(lines, "\n")
}

func (e *Engine) composeInitialRichStatusFooter(agent Agent, session AgentSession, workspaceDir string) string {
	var lines []string
	if e.showContextIndicator {
		if model := strings.TrimSpace(replyFooterModel(session, agent)); model != "" {
			lines = append(lines, "model: "+model)
		}
	}
	if e.showWorkdirIndicator {
		if dir := strings.TrimSpace(replyFooterWorkDir(session, agent, workspaceDir)); dir != "" {
			lines = append(lines, "cwd: "+dir)
		}
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 6: Run core tests and verify they pass**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core -run 'TestComposeRichStatusFooter_Streaming|TestComposeRichStatusFooter_FinalBranchUnchanged|TestReplyFooterWorkDir' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit core footer change**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
git add core/engine.go core/engine_test.go
git commit -m "feat: show initial footer on streaming cards"
```

Expected: commit succeeds.

---

### Task 2: Feishu Renderer Footer Regression
<!-- covers: S5 -->

**Files:**
- Test: `platform/feishu/platform_test.go` near existing `TestBuildRichCard_*` tests

**Interfaces:**
- Consumes: existing unexported `buildRichCard(status core.CardStatus, title string, steps []core.ToolStep, markdown string, streaming bool, statusFooter string) string`.
- Produces: regression coverage proving a non-empty multi-line `statusFooter` renders on streaming cards as grey notation footer blocks after an `hr` separator.

- [ ] **Step 1: Write the Feishu renderer regression test**

Add this test near `TestBuildRichCard_UsesCodexRuntimeToolDescriptors` in `platform/feishu/platform_test.go`:

```go
func TestBuildRichCard_RendersStreamingStatusFooterAsNotationBlocks(t *testing.T) {
	cardJSON := buildRichCard(core.CardStatusWorking, "", nil, "answer", true, "model: claude-test-model\ncwd: /tmp/cc-connect-fixture · sess-stream-123")

	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("card JSON is invalid: %v", err)
	}
	body, ok := card["body"].(map[string]any)
	if !ok {
		t.Fatalf("body = %#v, want object", card["body"])
	}
	elements, ok := body["elements"].([]any)
	if !ok {
		t.Fatalf("body.elements = %#v, want array", body["elements"])
	}

	var hrIndex = -1
	var footerLines []map[string]any
	for i, element := range elements {
		elem, ok := element.(map[string]any)
		if !ok {
			continue
		}
		if elem["tag"] == "hr" {
			hrIndex = i
			continue
		}
		if hrIndex >= 0 && elem["tag"] == "markdown" {
			footerLines = append(footerLines, elem)
		}
	}
	if hrIndex < 0 {
		t.Fatalf("card JSON should contain hr before footer: %s", cardJSON)
	}
	if len(footerLines) != 2 {
		t.Fatalf("footer line count = %d, want 2: %#v", len(footerLines), footerLines)
	}

	wants := []string{"model: claude-test-model", "cwd: /tmp/cc-connect-fixture · sess-stream-123"}
	for i, want := range wants {
		line := footerLines[i]
		if line["content"] != want {
			t.Fatalf("footer line %d content = %#v, want %q", i, line["content"], want)
		}
		if line["text_size"] != "notation" {
			t.Fatalf("footer line %d text_size = %#v, want notation", i, line["text_size"])
		}
		if line["text_color"] != "grey" {
			t.Fatalf("footer line %d text_color = %#v, want grey", i, line["text_color"])
		}
	}
}
```

- [ ] **Step 2: Run the renderer test**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./platform/feishu -run 'TestBuildRichCard_RendersStreamingStatusFooterAsNotationBlocks' -count=1
```

Expected: PASS because the renderer already supports non-empty multi-line `statusFooter`; if it fails, fix only the footer rendering path in `platform/feishu/feishu.go` so it splits `statusFooter` on `\n` and creates grey notation markdown blocks after an `hr`.

- [ ] **Step 3: Commit Feishu renderer test**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
git add platform/feishu/platform_test.go
git commit -m "test: cover streaming card footer rendering"
```

Expected: commit succeeds.

---

### Task 3: Verification Run
<!-- covers: S1, S2, S3, S4, S5 -->

**Files:**
- No planned source modifications.
- May modify only if test failures reveal a defect in Task 1 or Task 2.

**Interfaces:**
- Consumes: tests added in Tasks 1 and 2.
- Produces: passing focused and package-level verification commands required by `2026-07-09-streaming-card-initial-footer-verification.md`.

- [ ] **Step 1: Run focused verification tests**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core ./platform/feishu -run 'TestComposeRichStatusFooter|TestBuildRichCard.*Footer|TestReplyFooterWorkDir' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full relevant package tests**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
go test ./core ./platform/feishu -count=1
```

Expected: PASS.

- [ ] **Step 3: Inspect git status**

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect-streaming-card-initial-footer
git status --short
```

Expected: no uncommitted source/test changes. If uncommitted changes exist from fixes, commit them with a precise `fix:` or `test:` message after rerunning the relevant tests.

---

## Acceptance Mapping

| Verification Scenario | Covered by Plan Task |
|---|---|
| S1: Streaming footer composes model and cwd/session immediately | Task 1 |
| S2: Streaming footer recomputes when session id appears later | Task 1 |
| S3: Footer toggles preserve existing visibility semantics | Task 1 |
| S4: Final footer behavior remains unchanged | Task 1 and Task 3 |
| S5: Feishu renderer emits multi-line footer as notation blocks | Task 2 and Task 3 |
