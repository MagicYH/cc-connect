# Root Cause Analysis: Feishu Mention + Slash Command Failure

## Summary

When Bot1 mentions Bot2 in a Feishu group message with a slash command (e.g. `@Bot2 /help`), Bot2 cannot process the command. Three root causes were identified.

## Root Cause 1: `botOpenID` Empty → Mention Not Removed

**File:** `platform/feishu/feishu.go:3035`

`stripMentions()` replaces the bot's own `@_user_N` placeholder with empty string only when `botOpenID` matches. When `botOpenID == ""`, the condition fails and the placeholder is replaced with `@BotName` instead, producing `@Bot2 /help` instead of `/help`. The engine's command detection (`strings.HasPrefix(content, "/")` at `core/engine.go:2618`) then fails.

`botOpenID` is empty in two cases:
1. **Webhook mode** — `Start()` at line 370 skips `fetchBotOpenID()` entirely
2. **Startup failure** — `fetchBotOpenID()` fails, only logs a warning, bot continues running

| Stage | botOpenID set | botOpenID empty |
|---|---|---|
| Raw Feishu JSON | `@_user_1 /help` | `@_user_1 /help` |
| After stripMentions | `/help` | `@Bot2 /help` |
| Engine HasPrefix("/") | true | false |

## Root Cause 2: Interactive Card `<at>` Tags Don't Trigger Mention Events

**File:** `docs/team/mention-fallback/design.md`

When Bot1's agent sends a response containing `@Bot2` via the streaming card path (`SendPreviewStart` / `UpdateMessage`), the `<at>` tag is rendered visually but does NOT fire a `im.message.receive_v1` event. Bot2 therefore never receives the event at all. The non-streaming paths (`Reply`/`Send` → `buildReplyContent`) correctly detect `<at>` tags and fall back to Post format, but the streaming path does not check.

## Root Cause 3: Group Chat Filtering Coupled to `botOpenID`

**File:** `platform/feishu/feishu.go:1070`

When `botOpenID == ""`, the group mention filter (`isBotMentioned`) is entirely skipped, causing the bot to process messages it shouldn't. This doesn't directly break slash commands but causes incorrect behavior.

## Fix Recommendations

1. **`stripMentions` fallback**: When `botOpenID == ""`, strip remaining `@_user_N` placeholders via regex as they are Feishu API artifacts
2. **Always fetch `botOpenID`**: Remove the webhook mode skip in `Start()` so `fetchBotOpenID()` is always attempted
3. **Implement streaming card mention fallback**: Per `docs/team/mention-fallback/design.md`, detect `<at>` tags in streaming card path and fall back to Post format

## Related Pages

- [Feishu](../entities/feishu.md)
- [Slash Commands](../concepts/slash-commands.md)
- [Session Manager](../entities/session-manager.md)
