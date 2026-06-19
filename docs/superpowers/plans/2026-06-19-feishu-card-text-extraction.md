# Feishu Card Text Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** ./../specs/2026-06-19-feishu-card-text-extraction-design.md
**Verification:** ./../specs/2026-06-19-feishu-card-text-extraction-verification.md

**Goal:** Enhance `extractInteractiveCardText` and `extractCardElements` to extract text from `column_set`, `div.fields`, `note`, `action`, and `collapsible_panel` elements that are currently silently dropped.

**Architecture:** Add a `extractLegacyElementText` helper to centralize legacy element parsing, add explicit case branches for missing element types in `extractCardElements`, and add a depth limit to prevent unbounded recursion.

**Tech Stack:** Go 1.22+, standard library `encoding/json`, table-driven tests

---

## File Structure

| File | Responsibility | Change |
|------|---------------|--------|
| `platform/feishu/feishu.go` | Card text extraction logic | Modify: add `extractLegacyElementText`, add cases in `extractCardElements`, add `maxExtractDepth` |
| `platform/feishu/feishu_test.go` | Tests for card text extraction | Modify: add test cases for new element types |

---

### Task 1: Add `maxExtractDepth` constant and `extractLegacyElementText` helper <!-- covers: S1, S2, S3, S4, S12 -->

**Files:**
- Modify: `platform/feishu/feishu.go:2001` (add constant and helper before `extractInteractiveCardText`)

- [ ] **Step 1: Write failing tests for legacy `column_set`, `div.fields`, `note`, `action`**

Add these test cases to the existing `TestExtractInteractiveCardText` table in `platform/feishu/feishu_test.go`:

```go
{
    name:    "legacy_column_set",
    content: `{"elements":[[{"tag":"column_set","columns":[{"tag":"column","elements":[{"tag":"markdown","content":"**PSM:**"}]},{"tag":"column","elements":[{"tag":"div","text":{"tag":"lark_md","content":"my.service.psm"}}]}]}]]}`,
    want:    "**PSM:**\nmy.service.psm",
},
{
    name:    "legacy_div_with_fields",
    content: `{"elements":[[{"tag":"div","text":{"tag":"lark_md","content":"Header text"},"fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**PSM:** svc.psm"}},{"is_short":true,"text":"Plain string field"}]}]]}`,
    want:    "Header text\n**PSM:** svc.psm\nPlain string field",
},
{
    name:    "legacy_note",
    content: `{"elements":[[{"tag":"note","elements":[{"tag":"plain_text","content":"Updated 3 minutes ago"}]}]]}`,
    want:    "Updated 3 minutes ago",
},
{
    name:    "legacy_action_with_button_and_select",
    content: `{"elements":[[{"tag":"action","actions":[{"tag":"button","text":{"tag":"plain_text","content":"Acknowledge"}},{"tag":"select_static","placeholder":{"tag":"plain_text","content":"Select cluster"}}]}]]}`,
    want:    "Acknowledge\nSelect cluster",
},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v`

Expected: The 4 new test cases FAIL (existing 5 still pass). Legacy `column_set`, `note`, and `action` produce `[interactive card]` or empty output. `legacy_div_with_fields` produces only `"Header text"` (missing fields).

- [ ] **Step 3: Add `maxExtractDepth` constant and `extractLegacyElementText` helper**

Insert the following before `extractInteractiveCardText` at line 2001 in `platform/feishu/feishu.go`:

```go
const maxExtractDepth = 10

// extractLegacyElementText extracts text from a single legacy-format card element.
// It handles div (with text and fields), text/markdown/lark_md, column_set, note,
// action, img, hr, and falls back to extracting text/content fields for unknown tags.
func extractLegacyElementText(raw json.RawMessage, depth int) string {
	if depth > maxExtractDepth {
		return "[...]"
	}
	var elem map[string]json.RawMessage
	if json.Unmarshal(raw, &elem) != nil {
		return ""
	}

	var tag string
	if raw, ok := elem["tag"]; ok {
		_ = json.Unmarshal(raw, &tag)
	}

	var parts []string

	switch tag {
	case "div":
		// Extract text (plain string or {tag, content} object)
		if raw, ok := elem["text"]; ok {
			parts = append(parts, extractTextValue(raw)...)
		}
		// Extract fields[].text
		if raw, ok := elem["fields"]; ok {
			var fields []struct {
				Text json.RawMessage `json:"text"`
			}
			if json.Unmarshal(raw, &fields) == nil {
				for _, f := range fields {
					parts = append(parts, extractTextValue(f.Text)...)
				}
			}
		}
	case "markdown", "lark_md":
		if raw, ok := elem["content"]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
				parts = append(parts, s)
			}
		}
	case "text":
		// text can be a plain string in the "text" field
		if raw, ok := elem["text"]; ok {
			parts = append(parts, extractTextValue(raw)...)
		}
	case "column_set":
		if raw, ok := elem["columns"]; ok {
			var columns []struct {
				Elements json.RawMessage `json:"elements"`
			}
			if json.Unmarshal(raw, &columns) == nil {
				for _, col := range columns {
					for _, el := range flattenElements(col.Elements) {
						if s := extractLegacyElementText(el, depth+1); s != "" {
							parts = append(parts, s)
						}
					}
				}
			}
		}
	case "note":
		if raw, ok := elem["elements"]; ok {
			for _, el := range flattenElements(raw) {
				if s := extractLegacyElementText(el, depth+1); s != "" {
					parts = append(parts, s)
				}
			}
		}
	case "action":
		if raw, ok := elem["actions"]; ok {
			var actions []struct {
				Text        json.RawMessage `json:"text"`
				Placeholder json.RawMessage `json:"placeholder"`
			}
			if json.Unmarshal(raw, &actions) == nil {
				for _, a := range actions {
					parts = append(parts, extractTextValue(a.Text)...)
					if len(a.Placeholder) > 0 {
						parts = append(parts, extractTextValue(a.Placeholder)...)
					}
				}
			}
		}
	case "img":
		if raw, ok := elem["alt"]; ok {
			parts = append(parts, extractTextValue(raw)...)
		}
		if raw, ok := elem["title"]; ok && len(parts) == 0 {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				parts = append(parts, s)
			}
		}
	case "hr":
		parts = append(parts, "---")
	default:
		// Fallback: try text, then content
		if raw, ok := elem["text"]; ok {
			parts = append(parts, extractTextValue(raw)...)
		}
		if raw, ok := elem["content"]; ok && len(parts) == 0 {
			var s string
			if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
				parts = append(parts, s)
			}
		}
	}

	return strings.Join(parts, "\n")
}

// extractTextValue extracts text from a value that may be a plain string
// or a {tag, content} object (like {"tag":"lark_md","content":"..."}).
func extractTextValue(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
		return []string{s}
	}
	var obj struct {
		Content string `json:"content"`
	}
	if json.Unmarshal(raw, &obj) == nil && strings.TrimSpace(obj.Content) != "" {
		return []string{obj.Content}
	}
	return nil
}

// flattenElements parses an elements array that may be a flat array
// or an array-of-arrays (legacy nested format) and returns a flat slice.
func flattenElements(raw json.RawMessage) []json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var nested [][]json.RawMessage
	if json.Unmarshal(raw, &nested) == nil && len(nested) > 0 {
		var flat []json.RawMessage
		for _, row := range nested {
			flat = append(flat, row...)
		}
		return flat
	}
	var flat []json.RawMessage
	if json.Unmarshal(raw, &flat) == nil {
		return flat
	}
	return nil
}
```

- [ ] **Step 4: Replace the legacy element loop in `extractInteractiveCardText` with calls to `extractLegacyElementText`**

In `extractInteractiveCardText`, replace the loop at lines 2078-2103:

```go
// OLD (lines 2078-2103):
for _, raw := range elements {
    var elem struct {
        Tag  string          `json:"tag"`
        Text json.RawMessage `json:"text"`
    }
    if json.Unmarshal(raw, &elem) != nil {
        continue
    }
    if len(elem.Text) == 0 {
        continue
    }
    var textStr string
    if json.Unmarshal(elem.Text, &textStr) == nil && strings.TrimSpace(textStr) != "" {
        parts = append(parts, textStr)
    } else {
        var textObj struct {
            Tag     string `json:"tag"`
            Content string `json:"content"`
        }
        if json.Unmarshal(elem.Text, &textObj) == nil && strings.TrimSpace(textObj.Content) != "" {
            parts = append(parts, textObj.Content)
        }
    }
}
```

With:

```go
for _, raw := range elements {
    if s := extractLegacyElementText(raw, 0); s != "" {
        parts = append(parts, s)
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v`

Expected: All 9 test cases pass (5 existing + 4 new).

- [ ] **Step 6: Commit**

```bash
git add platform/feishu/feishu.go platform/feishu/feishu_test.go
git commit -m "feat(feishu): enhance legacy card parser to extract column_set, div.fields, note, action"
```

---

### Task 2: Add Schema 2.0 element type cases in `extractCardElements` <!-- covers: S5, S6, S7, S8, S9 -->

**Files:**
- Modify: `platform/feishu/feishu.go:2115-2228` (add cases to `extractCardElements`)

- [ ] **Step 1: Write failing tests for Schema 2.0 `column_set`, `note`, `action`, `collapsible_panel`, `div.fields`**

Add these test cases to `TestExtractInteractiveCardText`:

```go
{
    name:    "schema2_column_set",
    content: `{"body":{"tag":"body","property":{"elements":[{"tag":"column_set","property":{"columns":[{"tag":"column","elements":[{"tag":"markdown","property":{"content":"**PSM:**"}}]},{"tag":"column","elements":[{"tag":"markdown","property":{"content":"my.service.psm"}}]}]}}]},"schema":"2.0"}`,
    want:    "**PSM:**\nmy.service.psm",
},
{
    name:    "schema2_note_top_level_elements",
    content: `{"body":{"tag":"body","property":{"elements":[{"tag":"note","elements":[{"tag":"plain_text","property":{"content":"Updated 3 minutes ago"}}]}]}},"schema":"2.0"}`,
    want:    "Updated 3 minutes ago",
},
{
    name:    "schema2_action_with_button_and_select",
    content: `{"body":{"tag":"body","property":{"elements":[{"tag":"action","property":{"actions":[{"tag":"button","text":{"tag":"plain_text","content":"Acknowledge"}},{"tag":"select_static","placeholder":{"tag":"plain_text","content":"Select cluster"}}]}}]}},"schema":"2.0"}`,
    want:    "Acknowledge\nSelect cluster",
},
{
    name:    "schema2_collapsible_panel_header_before_children",
    content: `{"body":{"tag":"body","property":{"elements":[{"tag":"collapsible_panel","property":{"header":{"title":{"tag":"plain_text","content":"Panel Title"}},"elements":[{"tag":"markdown","property":{"content":"Inner content"}}]}}]}},"schema":"2.0"}`,
    want:    "Panel Title\nInner content",
},
{
    name:    "schema2_div_with_property_fields",
    content: `{"body":{"tag":"body","property":{"elements":[{"tag":"div","property":{"text":{"tag":"lark_md","content":"Header"},"fields":[{"text":{"tag":"lark_md","content":"**PSM:** svc.psm"}},{"text":"Plain string field"}]}}]}},"schema":"2.0"}`,
    want:    "Header\n**PSM:** svc.psm\nPlain string field",
},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v`

Expected: The 5 new test cases FAIL (existing 9 pass).

- [ ] **Step 3: Add `Elements` field to the `extractCardElements` outer struct and add new cases**

Modify the struct at line 2117-2131 to add `Elements`:

```go
var elem struct {
    Tag      string            `json:"tag"`
    Content  string            `json:"content"`
    Elements []json.RawMessage `json:"elements"`
    Property struct {
        Content   string            `json:"content"`
        Contents  json.RawMessage   `json:"contents"`
        Language  string            `json:"language"`
        Elements  []json.RawMessage `json:"elements"`
        Text      json.RawMessage   `json:"text"`
        Items     json.RawMessage   `json:"items"`
        Columns   json.RawMessage   `json:"columns"`
        Rows      json.RawMessage   `json:"rows"`
        Behaviors json.RawMessage   `json:"behaviors"`
        Actions   json.RawMessage   `json:"actions"`
        Fields    json.RawMessage   `json:"fields"`
        Header    json.RawMessage   `json:"header"`
    } `json:"property"`
}
```

Add these new cases to the switch statement (before `default:`):

```go
case "column_set":
    var columns []struct {
        Elements []json.RawMessage `json:"elements"`
    }
    if json.Unmarshal(elem.Property.Columns, &columns) == nil {
        for _, col := range columns {
            extractCardElements(col.Elements, parts)
        }
    }
case "note":
    noteElements := elem.Elements
    if len(noteElements) == 0 {
        noteElements = elem.Property.Elements
    }
    if len(noteElements) > 0 {
        extractCardElements(noteElements, parts)
    }
case "action":
    var actions []struct {
        Text        json.RawMessage `json:"text"`
        Placeholder json.RawMessage `json:"placeholder"`
    }
    if json.Unmarshal(elem.Property.Actions, &actions) == nil {
        for _, a := range actions {
            if len(a.Text) > 0 {
                *parts = append(*parts, extractTextValue(a.Text)...)
            }
            if len(a.Placeholder) > 0 {
                *parts = append(*parts, extractTextValue(a.Placeholder)...)
            }
        }
    }
case "collapsible_panel":
    // Extract header title first (before children)
    if len(elem.Property.Header) > 0 {
        var header struct {
            Title json.RawMessage `json:"title"`
        }
        if json.Unmarshal(elem.Property.Header, &header) == nil && len(header.Title) > 0 {
            *parts = append(*parts, extractTextValue(header.Title)...)
        }
    }
    if len(elem.Property.Elements) > 0 {
        extractCardElements(elem.Property.Elements, parts)
    }
```

In the `default:` case, add `property.fields` extraction after the existing `property.text` extraction (after line 2222):

```go
// Extract property.fields (key-value pairs in div elements)
if len(elem.Property.Fields) > 0 {
    var fields []struct {
        Text json.RawMessage `json:"text"`
    }
    if json.Unmarshal(elem.Property.Fields, &fields) == nil {
        for _, f := range fields {
            *parts = append(*parts, extractTextValue(f.Text)...)
        }
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v`

Expected: All 14 test cases pass (9 existing + 5 new).

- [ ] **Step 5: Commit**

```bash
git add platform/feishu/feishu.go platform/feishu/feishu_test.go
git commit -m "feat(feishu): add Schema 2.0 column_set, note, action, collapsible_panel, div.fields extraction"
```

---

### Task 3: Add `user_dsl` alarm card test and depth limit test <!-- covers: S10, S12 -->

**Files:**
- Modify: `platform/feishu/feishu_test.go` (add test cases)
- Modify: `platform/feishu/feishu.go` (add depth parameter to `extractCardElements`)

- [ ] **Step 1: Write failing test for `user_dsl` alarm card with `column_set`**

Add this test case to `TestExtractInteractiveCardText`:

```go
{
    name:    "user_dsl_alarm_card_with_column_set",
    content: `{"elements":[[{"tag":"text","text":"请升级至最新版本客户端"}]],"user_dsl":"{\"header\":{\"title\":{\"content\":\"Mesh Alarm\"}},\"elements\":[[{\"tag\":\"column_set\",\"columns\":[{\"tag\":\"column\",\"elements\":[{\"tag\":\"markdown\",\"content\":\"**PSM:**\"}]},{\"tag\":\"column\",\"elements\":[{\"tag\":\"markdown\",\"content\":\"my.service.psm\"}]}]}]]}"}`,
    want:    "Mesh Alarm\n**PSM:**\nmy.service.psm",
},
```

- [ ] **Step 2: Write test for depth limit**

Add this test case. It programmatically constructs a deeply nested card:

```go
{
    name:    "depth_limit_nested_column_set",
    content: buildDeeplyNestedCard(12),
    want:    "Level 0",
},
```

Add the helper function before `TestExtractInteractiveCardText`:

```go
func buildDeeplyNestedCard(depth int) string {
	// Build a card with depth levels of nested column_set elements.
	// Each level has a markdown element "Level N".
	inner := `{"tag":"markdown","content":"Level ` + strconv.Itoa(depth-1) + `"}`
	for i := depth - 2; i >= 0; i-- {
		inner = fmt.Sprintf(
			`{"tag":"column_set","columns":[{"tag":"column","elements":[{"tag":"markdown","content":"Level %d"},{"tag":"column_set","columns":[{"tag":"column","elements":[%s]}]}]}]}`,
			i, inner,
		)
	}
	return `{"elements":[[` + inner + `]]}`
}
```

Also add `"strconv"` to the imports if not already present.

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v`

Expected: `user_dsl_alarm_card_with_column_set` may pass (since the `user_dsl` path already calls `extractInteractiveCardText` recursively and Task 1 fixed the legacy parser). `depth_limit_nested_column_set` may panic (stack overflow) or produce very long output.

- [ ] **Step 4: Add depth parameter to `extractCardElements`**

Change `extractCardElements` signature to include depth:

```go
func extractCardElements(elements []json.RawMessage, parts *[]string, depth ...int) {
```

At the start of the function, check depth:

```go
d := 0
if len(depth) > 0 {
    d = depth[0]
}
if d > maxExtractDepth {
    *parts = append(*parts, "[...]")
    return
}
```

Update all recursive calls to pass `d+1`:
- Line 2040: `extractCardElements(body.Property.Elements, &parts)` → `extractCardElements(body.Property.Elements, &parts, 0)` (these are top-level calls, depth starts at 0)
- Line 2042: same pattern
- Line 2225: `extractCardElements(elem.Property.Elements, parts)` → `extractCardElements(elem.Property.Elements, parts, d+1)`
- All new recursive calls from Task 2 (column_set, note, collapsible_panel) → pass `d+1`
- `extractCardTable` and `extractCardListItems` internal calls also need updating — pass `d+1`

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText -v`

Expected: All 16 test cases pass. `depth_limit_nested_column_set` returns without panic, output contains `"Level 0"` and `"[...]"`.

- [ ] **Step 6: Commit**

```bash
git add platform/feishu/feishu.go platform/feishu/feishu_test.go
git commit -m "feat(feishu): add user_dsl alarm card test and recursion depth limit"
```

---

### Task 4: Add legacy mixed alarm card test <!-- covers: S11 -->

**Files:**
- Modify: `platform/feishu/feishu_test.go` (add test case)

- [ ] **Step 1: Write the mixed alarm card test**

Add this test case to `TestExtractInteractiveCardText`:

```go
{
    name:    "legacy_mixed_alarm_card",
    content: `{"header":{"title":{"content":"Mesh Alarm"}},"elements":[[{"tag":"column_set","columns":[{"tag":"column","elements":[{"tag":"markdown","content":"**PSM:**"}]},{"tag":"column","elements":[{"tag":"markdown","content":"my.service.psm"}]}]},{"tag":"div","text":{"tag":"lark_md","content":"Details"},"fields":[{"is_short":true,"text":{"tag":"lark_md","content":"**Level:** P2"}},{"is_short":true,"text":{"tag":"lark_md","content":"**Cluster:** default"}}]},{"tag":"note","elements":[{"tag":"plain_text","content":"2026-06-19 10:30:00"}]}]]}`,
    want:    "Mesh Alarm\n**PSM:**\nmy.service.psm\nDetails\n**Level:** P2\n**Cluster:** default\n2026-06-19 10:30:00",
},
```

- [ ] **Step 2: Run the test**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run TestExtractInteractiveCardText/legacy_mixed_alarm_card -v`

Expected: PASS (all element types are handled by Task 1).

- [ ] **Step 3: Commit**

```bash
git add platform/feishu/feishu_test.go
git commit -m "test(feishu): add legacy mixed alarm card test case"
```

---

### Task 5: Full regression test and build <!-- covers: S13, S14 -->

**Files:**
- No code changes

- [ ] **Step 1: Run full Feishu package tests**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -v`

Expected: All tests pass, no FAIL in output.

- [ ] **Step 2: Run full project build**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go build ./...`

Expected: Build succeeds with exit code 0.

- [ ] **Step 3: Run full project test suite**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./...`

Expected: All tests pass, no FAIL in output.

- [ ] **Step 4: Verify existing test cases are unchanged**

Run: `cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect && go test ./platform/feishu/ -run "TestExtractInteractiveCardText/(schema_2.0_body_property_elements|legacy_div_with_lark_md|user_dsl_with_schema_2.0_card|user_dsl_with_legacy_elements|empty_card)" -v`

Expected: All 5 original test cases PASS with same expected output as before.
