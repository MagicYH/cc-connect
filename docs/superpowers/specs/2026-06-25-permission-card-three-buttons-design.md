# Permission Request Card: Three Buttons in One Row

## Problem

The Feishu Permission Request card currently renders 3 buttons across 2 rows:
- Row 1: [Allow] [Deny] (equal-width columns via `bisect`)
- Row 2: [Allow All (this session)] (full-width action row)

This wastes vertical space and looks top-heavy. The user wants all 3 buttons on one row with equal width (trisect).

## Design

### Change 1: Merge button rows in `core/engine.go`

In `sendPermissionPrompt` (line ~10856), replace:
```go
ButtonsEqual(allowBtn, denyBtn).
Buttons(allowAllBtn).
```
with:
```go
ButtonsEqual(allowBtn, denyBtn, allowAllBtn).
```

### Change 2: Add `trisect` flex_mode in `platform/feishu/card.go`

In `renderCardMap`, extend the `flex_mode` logic (line ~234):
```go
if len(actions) == 2 {
    columnSet["flex_mode"] = "bisect"
} else if len(actions) == 3 {
    columnSet["flex_mode"] = "trisect"
}
```

This applies to **all** 3-button `ButtonsEqual` callers, including the Cron card and help-tab rows. These already render with `weight: 1` columns; adding `trisect` makes the intent explicit but produces visually identical output. Any snapshot tests asserting card JSON will need updating.

### Change 3: Upgrade `allowAllBtn` type

Change `allowAllBtn.Type` from `"default"` to `"primary"` in `sendPermissionPrompt`. All three buttons should have consistent visual weight (Allow = primary, Deny = danger, Allow All = primary).

### Change 4: Update test assertions

`TestSendPermissionPrompt_CardPlatform` currently asserts 2 `CardActions` rows (allow+deny, then allow_all). Update to assert 1 `CardActions` with 3 buttons in `EqualColumns` layout, and `allowAllBtn.Type == "primary"`.

## Scope

- Files modified: `core/engine.go`, `platform/feishu/card.go`, `core/engine_test.go`
- No i18n changes (button labels unchanged)
- No callback changes (action values `perm:allow`/`perm:deny`/`perm:allow_all` unchanged)
- No other platforms affected (Telegram/Discord ignore `CardActionLayout`; Slack/DingTalk/WeCom use plain text fallback)
- Telegram inline-button path still uses 2 rows — this is intentional because Telegram inline keyboards have narrower per-button width

## Acknowledged Tradeoffs

- **Adjacent primary buttons**: "Allow" and "Allow All" are both blue and adjacent. A mis-tap on "Allow All" when aiming for "Allow" escalates permission; the inverse is a minor inconvenience. This is acceptable because the buttons have distinct labels and the Deny button (red) provides a clear visual anchor in the middle.
- **i18n text width**: "Allow All (this session)" is 26 chars in EN, 24 in ES. In `trisect` columns (~1/3 card width), Feishu may truncate with ellipsis. This is acceptable — the button remains tappable, and the label is still partially readable. CJK translations (8-10 chars) fit comfortably.

## Verification

- `go test ./core/ -run TestSendPermissionPrompt -v` passes (with updated assertions)
- `go test ./...` passes
- Visual check on Feishu: permission card shows 3 equally-spaced buttons in one row
- Cron card continues rendering correctly (visually identical; JSON gains `flex_mode: "trisect"`)
