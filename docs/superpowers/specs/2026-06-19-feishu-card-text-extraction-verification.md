# Feishu Interactive Card Text Extraction Verification

> Source spec: ./2026-06-19-feishu-card-text-extraction-design.md
> Used by: superpowers:writing-plans (TDD coverage) and post-implementation smoke testing.

## Environment & Access

| Item | Value | How to Obtain |
|---|---|---|
| Target environment | Local development machine | — |
| Go version | go1.22+ | `go version` |
| Test runner | `go test` | Pre-installed with Go |
| Project root | `/data00/home/chenhao.magic/Project/Source/Github/cc-connect` | — |

## Public Operations

### Run unit tests for Feishu package

Purpose: Execute the test suite covering `extractInteractiveCardText` and `extractLegacyElementText`.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v
```

### Run full Feishu package test suite

Purpose: Verify no regressions in the broader Feishu platform code.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -v
```

### Build the project

Purpose: Confirm the code compiles without errors.

```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go build ./...
```

## Acceptance Criteria

- [ ] AC-1: Legacy path extracts `column_set` text — columns containing `markdown`, `div`, and `lark_md` elements all produce output (covers spec §1 column_set)
- [ ] AC-2: Legacy path extracts `div` with `fields` — both plain-string and `{tag, content}` field text forms produce output (covers spec §1 div.fields)
- [ ] AC-3: Legacy path extracts `note` element text — footnote elements with `plain_text` children produce output (covers spec §1 note)
- [ ] AC-4: Legacy path extracts `action` element text — button labels (`text.content`) and select placeholders (`placeholder.content`) produce output (covers spec §1 action)
- [ ] AC-5: Schema 2.0 path extracts `column_set` text — columns with Schema 2.0 nested elements produce output via recursive `extractCardElements` (covers spec §2 column_set)
- [ ] AC-6: Schema 2.0 path extracts `note` text — note elements with top-level `elements` (not `property.elements`) produce output (covers spec §2 note)
- [ ] AC-7: Schema 2.0 path extracts `action` text — `text.content` for buttons and `placeholder.content` for selects produce output (covers spec §2 action)
- [ ] AC-8: Schema 2.0 path extracts `collapsible_panel` header title before children — title appears in output before inner element text (covers spec §2 collapsible_panel)
- [ ] AC-9: Schema 2.0 path extracts `div` with `property.fields` — fields with both plain-string and `{tag, content}` forms produce output (covers spec §2 div.fields)
- [ ] AC-10: `user_dsl` wrapping alarm card with `column_set` produces full extracted text (covers spec §3 user_dsl path)
- [ ] AC-11: Mixed alarm card (header + column_set + div.fields + note) in legacy format produces complete output with all sections (covers spec §3 mixed card)
- [ ] AC-12: Depth limit prevents stack overflow — a deeply nested `column_set` (depth > 10) stops recursing and appends `[...]` (covers spec §1 & §2 depth limit)
- [ ] AC-13: Existing tests continue to pass — no regression in the 5 existing `extractInteractiveCardText` test cases (covers spec §3 backward compatibility)
- [ ] AC-14: Project builds and full test suite passes — `go build ./...` and `go test ./...` succeed (covers spec files-changed)

## Test Scenarios

### Scenario S1: Legacy `column_set` extraction

**Verifies:** AC-1

**Execution:** AI-autonomous

**Preconditions:**
- `extractInteractiveCardText` function exists and is callable from test code
- Test file `platform/feishu/feishu_test.go` exists

**Steps:**
1. Add a test case with a legacy-format card containing a `column_set` with two columns. The first column has a `markdown` element with content `"**PSM:**"`, the second has a `div` element with `text: {"tag": "lark_md", "content": "my.service.psm"}`.
   ```go
   // Card JSON: {"elements":[[{"tag":"column_set","columns":[{"tag":"column","elements":[{"tag":"markdown","content":"**PSM:**"}]},{"tag":"column","elements":[{"tag":"div","text":{"tag":"lark_md","content":"my.service.psm"}}]}]}]]}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"**PSM:**"` and `"my.service.psm"`.
3. Run the test:
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v
   ```
   → If test fails, **stop** — the `column_set` handling is not working correctly.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"elements":[[{"tag":"column_set","columns":[{"tag":"column","elements":[{"tag":"markdown","content":"**PSM:**"}]},{"tag":"column","elements":[{"tag":"div","text":{"tag":"lark_md","content":"my.service.psm"}}]}]}]]}` | concrete | Legacy format with column_set |

**Expected Results:**
- (must) Output contains `"**PSM:**"`
- (must) Output contains `"my.service.psm"`

**Failure Handling:**
- Output missing column text: check that `extractLegacyElementText` handles `column_set` tag and iterates `columns[].elements[]`

### Scenario S2: Legacy `div` with `fields` extraction

**Verifies:** AC-2

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a legacy card containing a `div` element with `fields`. Include both plain-string and `{tag, content}` field text forms.
   ```go
   // Card JSON: {"elements":[[{"tag":"div","text":{"tag":"lark_md","content":"Header text"},"fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**PSM:** svc.psm"}},{"is_short":true,"text":"Plain string field"}]}]]}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Header text"`, `"**PSM:** svc.psm"`, and `"Plain string field"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"elements":[[{"tag":"div","text":{"tag":"lark_md","content":"Header text"},"fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**PSM:** svc.psm"}},{"is_short":true,"text":"Plain string field"}]}]]}` | concrete | Legacy div with mixed field text forms |

**Expected Results:**
- (must) Output contains `"Header text"`
- (must) Output contains `"**PSM:** svc.psm"`
- (must) Output contains `"Plain string field"`

**Failure Handling:**
- Plain-string field missing: check that `fields[].text` is unmarshaled as plain string first before trying `{tag, content}` object form

### Scenario S3: Legacy `note` element extraction

**Verifies:** AC-3

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a legacy card containing a `note` element with `plain_text` children.
   ```go
   // Card JSON: {"elements":[[{"tag":"note","elements":[{"tag":"plain_text","content":"Updated 3 minutes ago"}]}]]}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Updated 3 minutes ago"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"elements":[[{"tag":"note","elements":[{"tag":"plain_text","content":"Updated 3 minutes ago"}]}]]}` | concrete | Legacy note element |

**Expected Results:**
- (must) Output contains `"Updated 3 minutes ago"`

**Failure Handling:**
- Note text missing: check that `extractLegacyElementText` handles `note` tag and iterates its `elements` array

### Scenario S4: Legacy `action` element extraction

**Verifies:** AC-4

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a legacy card containing an `action` element with a button and a select.
   ```go
   // Card JSON: {"elements":[[{"tag":"action","actions":[{"tag":"button","text":{"tag":"plain_text","content":"Acknowledge"}},{"tag":"select_static","placeholder":{"tag":"plain_text","content":"Select cluster"}}]}]]}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Acknowledge"` and `"Select cluster"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"elements":[[{"tag":"action","actions":[{"tag":"button","text":{"tag":"plain_text","content":"Acknowledge"}},{"tag":"select_static","placeholder":{"tag":"plain_text","content":"Select cluster"}}]}]]}` | concrete | Legacy action with button and select |

**Expected Results:**
- (must) Output contains `"Acknowledge"`
- (must) Output contains `"Select cluster"`

**Failure Handling:**
- Select placeholder missing: check that `action` handler extracts `placeholder.content` in addition to `text.content`

### Scenario S5: Schema 2.0 `column_set` extraction

**Verifies:** AC-5

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a Schema 2.0 card containing a `column_set` element in `body.property.elements`. Each column contains a `markdown` element.
   ```go
   // Card JSON: {"body":{"tag":"body","property":{"elements":[{"tag":"column_set","property":{"columns":[{"tag":"column","elements":[{"tag":"markdown","property":{"content":"**PSM:**"}}]},{"tag":"column","elements":[{"tag":"markdown","property":{"content":"my.service.psm"}}]}]}}]},"schema":"2.0"}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"**PSM:**"` and `"my.service.psm"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"body":{"tag":"body","property":{"elements":[{"tag":"column_set","property":{"columns":[{"tag":"column","elements":[{"tag":"markdown","property":{"content":"**PSM:**"}}]},{"tag":"column","elements":[{"tag":"markdown","property":{"content":"my.service.psm"}}]}]}}]},"schema":"2.0"}` | concrete | Schema 2.0 column_set |

**Expected Results:**
- (must) Output contains `"**PSM:**"`
- (must) Output contains `"my.service.psm"`

**Failure Handling:**
- Column text missing: check that `extractCardElements` has an explicit `column_set` case that iterates `property.columns` and recursively calls `extractCardElements` on each column's `elements`

### Scenario S6: Schema 2.0 `note` extraction (top-level `elements`)

**Verifies:** AC-6

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a Schema 2.0 card containing a `note` element with top-level `elements` (not `property.elements`).
   ```go
   // Card JSON: {"body":{"tag":"body","property":{"elements":[{"tag":"note","elements":[{"tag":"plain_text","property":{"content":"Updated 3 minutes ago"}}]}]}},"schema":"2.0"}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Updated 3 minutes ago"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"body":{"tag":"body","property":{"elements":[{"tag":"note","elements":[{"tag":"plain_text","property":{"content":"Updated 3 minutes ago"}}]}]}},"schema":"2.0"}` | concrete | Schema 2.0 note with top-level elements |

**Expected Results:**
- (must) Output contains `"Updated 3 minutes ago"`

**Failure Handling:**
- Note text missing: check that `extractCardElements` struct has `Elements []json.RawMessage` field and the `note` case prefers it over `Property.Elements`

### Scenario S7: Schema 2.0 `action` extraction

**Verifies:** AC-7

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a Schema 2.0 card containing an `action` element with a button and a select.
   ```go
   // Card JSON: {"body":{"tag":"body","property":{"elements":[{"tag":"action","property":{"actions":[{"tag":"button","text":{"tag":"plain_text","content":"Acknowledge"}},{"tag":"select_static","placeholder":{"tag":"plain_text","content":"Select cluster"}}]}}]}},"schema":"2.0"}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Acknowledge"` and `"Select cluster"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"body":{"tag":"body","property":{"elements":[{"tag":"action","property":{"actions":[{"tag":"button","text":{"tag":"plain_text","content":"Acknowledge"}},{"tag":"select_static","placeholder":{"tag":"plain_text","content":"Select cluster"}}]}}]}},"schema":"2.0"}` | concrete | Schema 2.0 action with button and select |

**Expected Results:**
- (must) Output contains `"Acknowledge"`
- (must) Output contains `"Select cluster"`

**Failure Handling:**
- Select placeholder missing: check that `action` case extracts `placeholder.content` in addition to `text.content`

### Scenario S8: Schema 2.0 `collapsible_panel` header title before children

**Verifies:** AC-8

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a Schema 2.0 card containing a `collapsible_panel` with a header title and inner elements.
   ```go
   // Card JSON: {"body":{"tag":"body","property":{"elements":[{"tag":"collapsible_panel","property":{"header":{"title":{"tag":"plain_text","content":"Panel Title"}},"elements":[{"tag":"markdown","property":{"content":"Inner content"}}]}}]}},"schema":"2.0"}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Panel Title"` and `"Inner content"`.
3. Verify that `"Panel Title"` appears before `"Inner content"` in the output string.
4. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"body":{"tag":"body","property":{"elements":[{"tag":"collapsible_panel","property":{"header":{"title":{"tag":"plain_text","content":"Panel Title"}},"elements":[{"tag":"markdown","property":{"content":"Inner content"}}]}}]}},"schema":"2.0"}` | concrete | Schema 2.0 collapsible_panel |

**Expected Results:**
- (must) Output contains `"Panel Title"`
- (must) Output contains `"Inner content"`
- (must) Index of `"Panel Title"` in output is less than index of `"Inner content"`

**Failure Handling:**
- Panel title missing: check that `collapsible_panel` explicit case extracts `property.header.title` (try `{tag, content}` object, then plain string)
- Wrong ordering: check that panel title is appended to `parts` before the recursive `extractCardElements` call on `property.elements`

### Scenario S9: Schema 2.0 `div` with `property.fields` extraction

**Verifies:** AC-9

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a Schema 2.0 card containing a `div` element with `property.text` and `property.fields` (both `{tag, content}` and plain-string forms).
   ```go
   // Card JSON: {"body":{"tag":"body","property":{"elements":[{"tag":"div","property":{"text":{"tag":"lark_md","content":"Header"},"fields":[{"text":{"tag":"lark_md","content":"**PSM:** svc.psm"}},{"text":"Plain string field"}]}}]}},"schema":"2.0"}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Header"`, `"**PSM:** svc.psm"`, and `"Plain string field"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | `{"body":{"tag":"body","property":{"elements":[{"tag":"div","property":{"text":{"tag":"lark_md","content":"Header"},"fields":[{"text":{"tag":"lark_md","content":"**PSM:** svc.psm"}},{"text":"Plain string field"}]}}]}},"schema":"2.0"}` | concrete | Schema 2.0 div with property.fields |

**Expected Results:**
- (must) Output contains `"Header"`
- (must) Output contains `"**PSM:** svc.psm"`
- (must) Output contains `"Plain string field"`

**Failure Handling:**
- Plain-string field missing: check that `property.fields[].text` is unmarshaled as plain string first before trying `{tag, content}` object form
- Fields entirely missing: check that default case in `extractCardElements` checks for `property.fields` after `property.text`

### Scenario S10: `user_dsl` wrapping alarm card with `column_set`

**Verifies:** AC-10

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a card where the top-level has `user_dsl` wrapping a legacy alarm card with `column_set`.
   ```go
   // The user_dsl value is a JSON-escaped string of: {"header":{"title":{"content":"Mesh Alarm"}},"elements":[[{"tag":"column_set","columns":[{"tag":"column","elements":[{"tag":"markdown","content":"**PSM:**"}]},{"tag":"column","elements":[{"tag":"markdown","content":"my.service.psm"}]}]}]]}
   // Top-level: {"elements":[[{"tag":"text","text":"请升级至最新版本客户端"}]],"user_dsl":"<escaped JSON>"}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains `"Mesh Alarm"`, `"**PSM:**"`, and `"my.service.psm"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | (constructed with `user_dsl` wrapping a legacy alarm card with column_set) | concrete | user_dsl alarm card path |

**Expected Results:**
- (must) Output contains `"Mesh Alarm"`
- (must) Output contains `"**PSM:**"`
- (must) Output contains `"my.service.psm"`

**Failure Handling:**
- Only fallback text extracted: check that `user_dsl` path correctly unescapes the inner JSON and recursively calls `extractInteractiveCardText`

### Scenario S11: Legacy mixed alarm card (full scenario)

**Verifies:** AC-11

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Add a test case with a realistic legacy alarm card containing header, column_set (PSM + region), div with fields (level + cluster), and note (timestamp).
   ```go
   // Card JSON: {"header":{"title":{"content":"Mesh Alarm"}},"elements":[[{"tag":"column_set","columns":[{"tag":"column","elements":[{"tag":"markdown","content":"**PSM:**"}]},{"tag":"column","elements":[{"tag":"markdown","content":"my.service.psm"}]}]},{"tag":"div","text":{"tag":"lark_md","content":"Details"},"fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**Level:** P2"}},{"is_short":true,"text":{"tag":"lark_md","content":"**Cluster:** default"}}]},{"tag":"note","elements":[{"tag":"plain_text","content":"2026-06-19 10:30:00"}]}]]}
   ```
2. Call `extractInteractiveCardText(cardJSON)` and verify the output contains: `"Mesh Alarm"`, `"**PSM:**"`, `"my.service.psm"`, `"Details"`, `"**Level:** P2"`, `"**Cluster:** default"`, and `"2026-06-19 10:30:00"`.
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| card_json | string | (full alarm card with header, column_set, div.fields, note) | concrete | Realistic alarm card |

**Expected Results:**
- (must) Output contains `"Mesh Alarm"`
- (must) Output contains `"**PSM:**"` and `"my.service.psm"`
- (must) Output contains `"Details"`, `"**Level:** P2"`, `"**Cluster:** default"`
- (must) Output contains `"2026-06-19 10:30:00"`

**Failure Handling:**
- Missing sections: isolate which element type is not being extracted by testing it individually via S1–S4

### Scenario S12: Depth limit prevents infinite recursion

**Verifies:** AC-12

**Execution:** AI-autonomous

**Preconditions:**
- Same as S1

**Steps:**
1. Programmatically construct a card JSON with a `column_set` nested 12 levels deep (each column contains another `column_set`).
2. Call `extractInteractiveCardText(cardJSON)` and verify:
   - The function returns without panicking or stack overflow
   - The output contains `[...]` at the point where depth limit was hit
3. Run the test.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| max_depth | integer | 10 | concrete | `maxExtractDepth` constant |
| test_depth | integer | 12 | concrete | Exceeds max to trigger limit |

**Expected Results:**
- (must) Function returns without panic
- (must) Output contains `[...]`

**Failure Handling:**
- Stack overflow: depth limit check is not being applied; verify `depth` parameter is incremented on each recursive call and checked against `maxExtractDepth`

### Scenario S13: Existing tests pass without regression

**Verifies:** AC-13

**Execution:** AI-autonomous

**Preconditions:**
- The 5 existing `extractInteractiveCardText` test cases are still present in `feishu_test.go`
- No test case names or expected outputs have been changed

**Steps:**
1. Run the existing test suite:
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v
   ```
2. Verify all existing test cases pass:
   - `schema_2.0_body_property_elements`
   - `legacy_div_with_lark_md`
   - `user_dsl_with_schema_2.0_card`
   - `user_dsl_with_legacy_elements`
   - `empty_card`

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| existing_test_count | integer | 5 | concrete | Known count of existing test cases |

**Expected Results:**
- (must) All 5 existing test cases pass
- (must) No test output contains `FAIL`

**Failure Handling:**
- Existing test fails: the new code changed behavior for already-covered element types; compare output of failing test before and after the change to identify the regression

### Scenario S14: Project builds and full test suite passes

**Verifies:** AC-14

**Execution:** AI-autonomous

**Preconditions:**
- All code changes have been made to `feishu.go` and `feishu_test.go`
- No other files have been modified

**Steps:**
1. Build the project:
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go build ./...
   ```
   → If build fails, **stop** — fix compilation errors before proceeding.
2. Run the full test suite:
   ```bash
   cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./...
   ```
3. Verify all tests pass.

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| build_cmd | string | `go build ./...` | concrete | Standard Go build |
| test_cmd | string | `go test ./...` | concrete | Standard Go test |

**Expected Results:**
- (must) `go build ./...` exits with code 0
- (must) `go test ./...` exits with code 0
- (must) No `FAIL` in test output

**Failure Handling:**
- Build fails: check for compilation errors in modified files; likely a type mismatch or undefined function
- Test fails in other packages: verify the change did not affect any exported API; `extractInteractiveCardText` and `extractLegacyElementText` are unexported, so cross-package impact is unlikely

## Coverage Matrix

| Acceptance Criterion | Covered by Scenario |
|---|---|
| AC-1 | S1 |
| AC-2 | S2 |
| AC-3 | S3 |
| AC-4 | S4 |
| AC-5 | S5 |
| AC-6 | S6 |
| AC-7 | S7 |
| AC-8 | S8 |
| AC-9 | S9 |
| AC-10 | S10 |
| AC-11 | S11 |
| AC-12 | S12 |
| AC-13 | S13 |
| AC-14 | S14 |
