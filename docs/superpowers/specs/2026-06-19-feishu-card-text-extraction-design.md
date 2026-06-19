# Enhance Feishu Interactive Card Text Extraction

## Problem

When a Feishu alarm/notification card message is received, `extractInteractiveCardText()` produces incomplete text. The agent only receives the card title (e.g., "Mesh 入流量延迟大于2000ms") and loses all detail fields (PSM, cluster, region, etc.) because the parser skips several element types.

### Root Cause

Two code paths have gaps:

1. **Legacy parser** (feishu.go:2078-2103) — only extracts elements with a top-level `text` field. Elements without it are silently skipped:
   - `column_set` (used for key-value layouts in alarm cards)
   - `div` with `fields` (another key-value layout pattern)
   - `note` (footnotes/timestamps)
   - `action` (button rows)

2. **Schema 2.0 parser** (`extractCardElements`, feishu.go:2115-2228) — has explicit cases for `button`, `code_block`, `code_span`, `hr`, `table`, `list` and a default catch-all, but misses:
   - `column_set` (no `property.content` or `property.elements`, so default case produces nothing)
   - `note` (has `elements` not `property.elements`, not reached by default recursion)
   - `action` (has `actions` array, not `property.elements`)
   - `collapsible_panel` header title (only inner `property.elements` recursed, header dropped)
   - `div` with `property.fields` (only `property.text` extracted, fields skipped)

## Approach

Extend both parsing paths with explicit handling for the missing element types. Extract a `extractLegacyElementText` helper to centralize legacy element parsing (avoiding asymmetry between top-level and nested element handling).

## Design

### 1. Legacy Path Enhancement

Replace the current `for _, raw := range elements` loop body (lines 2078-2103) with calls to a new `extractLegacyElementText(raw json.RawMessage, depth int) string` helper. This eliminates the asymmetry where top-level elements without `text` are skipped but the same elements inside `column_set` would be extracted.

**`extractLegacyElementText` handles:**

- `div` — extract `text` (plain string or `{tag, content}` object), then check `fields` array
- `text` / `markdown` / `lark_md` — extract `content` field (or `text` as plain string for `text` tag)
- `column_set` — iterate `columns`, for each column flatten its `elements` (handling array-of-arrays), recursively call `extractLegacyElementText` on each element
- `note` — iterate `elements`, recursively call `extractLegacyElementText`
- `action` — iterate `actions`, extract `text.content` for buttons, `placeholder.content` for selects/overflows
- `img` — extract `alt.content` or `title` if present
- `hr` — append `---`
- fallback — extract `text.content`, `content`, or `text` as plain string

**`div` with `fields`**: After extracting `text`, iterate `fields[].text` which can be either a plain string or a `{tag, content}` object — handle both forms.

**Depth limit**: `maxExtractDepth = 10`. When depth exceeds the limit, stop recursing and append `[...]`.

### 2. Schema 2.0 Path Enhancement

Add explicit `case` branches in `extractCardElements`. Also add `Elements []json.RawMessage` to the outer struct (alongside existing `Property`) so that elements using top-level `elements` (like `note`) are captured.

**`column_set`** (explicit case): Unmarshal `property.columns`. Each column contains `elements` (Schema 2.0 format with `property` nesting). Recursively call `extractCardElements(column.elements, &parts)`.

**`note`** (explicit case): Prefer `elem.Elements` (top-level `elements`), fall back to `elem.Property.Elements`. Recursively call `extractCardElements(noteElements, &parts)`.

**`action`** (explicit case): Unmarshal `property.actions` (or top-level `actions`). For each action, extract `text.content` (buttons) and `placeholder.content` (selects, date pickers, overflow menus). Document which action sub-types are handled.

**`collapsible_panel`** (explicit case, not in default): Extract `property.header.title` — try as `{tag, content}` object first, then as plain string. Append title to `parts` before recursing into `property.elements`. This ensures correct ordering (title before children).

**`div` with `fields`** (in default case): After the existing `property.text` extraction, check for `property.fields`. Each field's `text` can be a plain string or `{tag, content}` — handle both forms. Extract `fields[].text.content` for each field.

**Depth limit**: Add `depth` parameter to `extractCardElements`. Same `maxExtractDepth = 10` constant. Stop recursing beyond limit.

### 3. Test Coverage

Add table-driven test cases to `feishu_test.go` for `extractInteractiveCardText`:

**Legacy path:**
1. `column_set` with multiple columns — all column text extracted
2. `div` with `fields` (mixed plain-text and lark_md) — all field text extracted
3. `note` element — footnote text extracted
4. `action` with buttons and selects — button labels and select placeholders extracted
5. Mixed card (header + column_set + div.fields + note) — full alarm card scenario
6. `column_set` inside `elements` array-of-arrays — flattening works correctly

**Schema 2.0 path:**
1. `column_set` in `body.property.elements` — nested column content recursively extracted
2. `note` in `body.property.elements` (top-level `elements`, not `property.elements`) — footnote extracted
3. `action` with button actions and selects — labels and placeholders extracted
4. `collapsible_panel` with header title — panel title extracted before children
5. `div` with `property.fields` — fields extracted (both plain string and `{tag, content}` forms)

**`user_dsl` path:**
1. `user_dsl` wrapping an alarm card with `column_set` — the most common real-world alarm callback path

All tests use inline JSON strings consistent with existing test style.

## Files Changed

| File | Change |
|------|--------|
| `platform/feishu/feishu.go` | Add `extractLegacyElementText` helper; add element type handling in legacy parser and `extractCardElements`; add depth limit |
| `platform/feishu/feishu_test.go` | Add test cases for new element types |

## Not Changed

- `extractCardTable`, `extractCardListItems` — already working correctly
- `card.go` — outbound card rendering, not affected
- `core/` — no interface or data model changes
- DIAG diagnostic code — left as-is (separate concern)
