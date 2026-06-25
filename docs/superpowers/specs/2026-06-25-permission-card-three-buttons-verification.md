# Permission Request Card: Three Buttons in One Row — Verification

> Source spec: ./2026-06-25-permission-card-three-buttons-design.md
> Used by: superpowers:writing-plans (TDD coverage) and post-implementation smoke testing.

## Environment & Access

| Item | Value | How to Obtain |
|---|---|---|
| Target environment | Local dev (cc-connect daemon on localhost) | `make build && cc-connect daemon start` |
| Go toolchain | go 1.22+ | `go version` |
| Test runner | `go test -race ./...` | Built into Go |
| Feishu bot | Configured in local config.toml | See `config.example.toml` |
| Test Feishu group | Group where cc-connect bot is member | Manual: add bot to test group |
| Project root | `/data00/home/chenhao.magic/Project/Source/Github/cc-connect` | `pwd` |

## Public Operations

### Build and restart daemon

Purpose: Rebuild cc-connect binary and restart the daemon to pick up code changes.

```bash
make build && cc-connect daemon restart
```

### Run full test suite

Purpose: Run all Go tests with race detector.

```bash
go test -race ./...
```

## Acceptance Criteria

- [ ] AC-1: Permission card renders 3 buttons in a single row with `EqualColumns` layout (covers spec §Design Change 1)
- [ ] AC-2: Feishu card JSON uses `flex_mode: "trisect"` for 3-button `ButtonsEqual` rows (covers spec §Design Change 2)
- [ ] AC-3: `allowAllBtn.Type` is `"primary"` instead of `"default"` (covers spec §Design Change 3)
- [ ] AC-4: Test `TestSendPermissionPrompt_CardPlatform` passes with updated assertions for 3-button single-row layout (covers spec §Design Change 4)
- [ ] AC-5: Existing 3-button `ButtonsEqual` callers (Cron card, help-tab rows) still render correctly with `trisect` (covers spec §Design Change 2 scope note)
- [ ] AC-6: Non-Feishu platforms (Telegram inline buttons) remain unchanged — 2-row layout (covers spec §Scope)

## Test Scenarios

### Scenario S1: Unit test — permission card has 3 buttons in one row

**Verifies:** AC-1, AC-3, AC-4

**Execution:** AI-autonomous

**Preconditions:**
- Go toolchain installed
- Project root is current directory
- No uncommitted changes to `core/engine.go` or `core/engine_test.go` that would conflict

**Steps:**
1. Run the specific permission prompt test:
   ```bash
   go test ./core/ -run TestSendPermissionPrompt -v -race
   ```
   → If test fails, **stop** — assertions need updating per Change 4.

2. Inspect the card structure in the test output. Verify:
   - `Card.Actions` has exactly 1 entry (not 2)
   - The single `CardActions` entry has `Layout == CardActionLayoutEqualColumns`
   - `CardActions.Buttons` has exactly 3 entries
   - Button types are: `"primary"`, `"danger"`, `"primary"` (Allow, Deny, Allow All)
   - Button values are: `"perm:allow"`, `"perm:deny"`, `"perm:allow_all"`

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| test_filter | string | TestSendPermissionPrompt | concrete | Go test -run pattern |

**Expected Results:**
- (must) Test passes with 0 failures
- (must) Card has 1 `CardActions` with 3 buttons in `EqualColumns` layout
- (must) Button types: primary, danger, primary
- (must) Button values: perm:allow, perm:deny, perm:allow_all

**Failure Handling:**
- Test fails with "expected 2 actions, got 1": assertions not yet updated — apply Change 4
- Test fails with button type mismatch: Change 3 not applied correctly

### Scenario S2: Unit test — Feishu renderer outputs trisect for 3 buttons

**Verifies:** AC-2

**Execution:** AI-autonomous

**Preconditions:**
- Change 2 applied to `platform/feishu/card.go`
- Feishu card rendering test exists or new test created

**Steps:**
1. Run Feishu card rendering tests:
   ```bash
   go test ./platform/feishu/ -run TestCard -v -race
   ```

2. If no existing test covers 3-button `ButtonsEqual`, write a targeted test:
   ```go
   func TestRenderCard_Trisection(t *testing.T) {
       card := core.NewCard().
           Title("Test", "blue").
           ButtonsEqual(btn1, btn2, btn3).
           Build()
       result := renderCardMap(card)
       elements := result["elements"].([]map[string]any)
       // First element after header is the column_set
       colSet := elements[0]
       assert.Equal(t, "column_set", colSet["tag"])
       assert.Equal(t, "trisect", colSet["flex_mode"])
       columns := colSet["columns"].([]map[string]any)
       assert.Len(t, columns, 3)
   }
   ```
   Then re-run:
   ```bash
   go test ./platform/feishu/ -run TestRenderCard_Trisection -v -race
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| test_filter | string | TestCard|TestRenderCard_Trisection | concrete | Go test -run pattern |

**Expected Results:**
- (must) `column_set` element has `"flex_mode": "trisect"` when 3 buttons are present
- (must) `columns` array has exactly 3 entries, each with `"weight": 1`
- (must) Each column contains one button element with `"width": "fill"`

**Failure Handling:**
- Test not found: create the test per the example above
- `flex_mode` missing or wrong: Change 2 not applied — check `renderCardMap` around line 234

### Scenario S3: Unit test — existing 2-button ButtonsEqual still uses bisect

**Verifies:** AC-2 (regression)

**Execution:** AI-autonomous

**Preconditions:**
- Change 2 applied

**Steps:**
1. Run or write a test for 2-button `ButtonsEqual`:
   ```go
   func TestRenderCard_Bisection(t *testing.T) {
       card := core.NewCard().
           Title("Test", "blue").
           ButtonsEqual(btn1, btn2).
           Build()
       result := renderCardMap(card)
       elements := result["elements"].([]map[string]any)
       colSet := elements[0]
       assert.Equal(t, "bisect", colSet["flex_mode"])
   }
   ```
   ```bash
   go test ./platform/feishu/ -run TestRenderCard_Bisection -v -race
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| test_filter | string | TestRenderCard_Bisection | concrete | Go test -run pattern |

**Expected Results:**
- (must) `flex_mode` is `"bisect"` for 2-button rows (no regression)

**Failure Handling:**
- `flex_mode` is `"trisect"` or missing: regression in Change 2 — check the `len(actions)` condition

### Scenario S4: Full test suite passes

**Verifies:** AC-4, AC-5

**Execution:** AI-autonomous

**Preconditions:**
- All code changes applied
- All test updates applied

**Steps:**
1. Run the full test suite:
   ```bash
   go test -race ./...
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| race_flag | boolean | true | concrete | Enables race detector |

**Expected Results:**
- (must) All tests pass with 0 failures
- (must) No race conditions detected

**Failure Handling:**
- Test failure in `core/`: check engine_test.go assertions
- Test failure in `platform/feishu/`: check card rendering test
- Race condition: identify the data race and add proper synchronization

### Scenario S5: Visual verification — Feishu permission card shows 3 buttons in one row

**Verifies:** AC-1, AC-2, AC-3

**Execution:** Human-assisted — Step 3 requires a human to visually confirm the card layout in Feishu

**Preconditions:**
- cc-connect daemon running with Feishu bot configured
- Test Feishu group has the bot as member
- A Claude Code session is active and connected through cc-connect

**Steps:**
1. Build and restart the daemon:
   ```bash
   make build && cc-connect daemon restart
   ```

2. In the Feishu test group, send a message that triggers a tool permission request (e.g., ask the agent to write a file). The agent will prompt for permission.

3. **Human**: Observe the Permission Request card in Feishu. Verify:
   - 3 buttons appear in a single row: [Allow] [Deny] [Allow All (this session)]
   - Buttons are equally spaced (trisect layout)
   - "Allow" and "Allow All" buttons are blue (primary)
   - "Deny" button is red (danger)
   - No second row of buttons

4. Click "Allow" and confirm the card updates with the green confirmation.

5. Trigger another permission request and click "Allow All". Confirm it works.

6. Trigger another permission request and click "Deny". Confirm it works.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| daemon_cmd | string | make build && cc-connect daemon restart | concrete | Build + restart command |

**Expected Results:**
- (must) Card shows exactly 3 buttons in one row
- (must) "Allow" and "Allow All" are primary (blue), "Deny" is danger (red)
- (must) Buttons are equally spaced
- (must) Clicking each button produces the correct permission response

**Failure Handling:**
- Card still shows 2 rows: daemon may not have restarted — verify with `cc-connect daemon status`
- Button layout looks off: check Feishu client version (trisect requires Interactive Card v2)
- Button click has no effect: check callback handler logs in daemon output

### Scenario S6: Cron card with 3 buttons still renders correctly

**Verifies:** AC-5

**Execution:** AI-autonomous

**Preconditions:**
- Change 2 applied

**Steps:**
1. Run cron-related tests:
   ```bash
   go test ./core/ -run TestCron -v -race
   ```

2. Verify the Cron card's 3-button `ButtonsEqual` row produces a `column_set` with `flex_mode: "trisect"` in Feishu rendering (same as S2 logic).

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| test_filter | string | TestCron | concrete | Go test -run pattern |

**Expected Results:**
- (must) Cron card tests pass
- (must) Cron card's 3-button row uses `trisect` flex_mode in Feishu rendering

**Failure Handling:**
- Test failures unrelated to card layout: pre-existing issue, not caused by this change
- Card layout assertion fails: verify that Change 2 correctly handles all 3-button `ButtonsEqual` callers

### Scenario S7: Telegram inline buttons remain 2-row layout

**Verifies:** AC-6

**Execution:** AI-autonomous

**Preconditions:**
- Code changes applied
- Telegram platform code not modified

**Steps:**
1. Run the permission prompt test for inline-button platforms:
   ```bash
   go test ./core/ -run TestSendPermissionPrompt -v -race
   ```

2. In the test output or code, verify that the `InlineButtonSender` path still constructs:
   ```go
   buttons := [][]ButtonOption{
       {allow, deny},
       {allow_all},
   }
   ```
   This is the existing 2-row inline button layout, untouched by this change.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| test_filter | string | TestSendPermissionPrompt | concrete | Go test -run pattern |

**Expected Results:**
- (must) Telegram/Discord inline-button path still uses 2 rows
- (must) `[][]ButtonOption` for permission prompt has 2 entries (not 1)

**Failure Handling:**
- Inline-button path changed: this spec does not modify inline buttons — revert any accidental changes to that code path

## Coverage Matrix

| Acceptance Criterion | Covered by Scenario |
|---|---|
| AC-1 | S1, S5 |
| AC-2 | S2, S3, S5 |
| AC-3 | S1, S5 |
| AC-4 | S1, S4 |
| AC-5 | S4, S6 |
| AC-6 | S7 |
