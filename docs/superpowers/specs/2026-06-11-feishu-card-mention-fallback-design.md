# Feishu: Card Mention Fallback Design

## Problem

When a Feishu interactive card message contains an `<at>` tag mentioning a bot, the bot cannot read its own mention from the incoming event. Card `<at>` tags are visual-only and do not populate the `mentions` field that triggers `im.message.receive_v1` events for bots.

The existing `buildReplyContent` (feishu.go:2637-2643) already detects `<at>` tags in plain text/markdown replies and falls back from `MsgTypeInteractive` to `MsgTypePost`. However, the `ReplyCard`/`SendCard` path in `interactivePlatform` (card.go) bypasses this logic entirely — structured cards are sent without any mention check.

## Solution

Add mention detection at the `interactivePlatform.ReplyCard()` and `SendCard()` entry points. If the card content contains `<at>` tags, downgrade to Post (rich text) format via the underlying `Platform.Reply()`/`Send()`, which already handles mention-aware format selection.

## Changes

### 1. New function: `cardContainsMention` (card.go)

```go
func cardContainsMention(card *core.Card) bool
```

Traverse all card elements and check for `<at ` substring in text content:
- `CardMarkdown`: check `Content`
- `CardNote`: check `Content`
- `CardActions`: check each button's `Text`
- `CardListItem`: check inner text fields

Returns `true` if any element contains `<at `.

### 2. Modify `interactivePlatform.ReplyCard()` (card.go)

Before sending the card:

```
if cardContainsMention(card) {
    slog.Info("feishu: card contains mention, falling back to post format")
    content := card.RenderText()
    return p.Platform.Reply(chatID, msgID, content)
}
// existing card send logic
```

### 3. Modify `interactivePlatform.SendCard()` (card.go)

Same pattern:

```
if cardContainsMention(card) {
    slog.Info("feishu: card contains mention, falling back to post format")
    content := card.RenderText()
    return p.Platform.Send(chatID, content)
}
// existing card send logic
```

### 4. No changes to streaming card methods

`StartStreamingCard`, `AppendStreamingCard`, `FinishStreamingCard` are not modified. The streaming card path does not go through `resolveMentionsInContent`, so mentions are not injected into streaming content. Adding detection here would add complexity without practical benefit.

### 5. No changes to `RefreshCard`

If a message was downgraded from card to post at send time, it never gets a message ID stored for card refresh, so `RefreshCard` is never called for downgraded messages.

## Data Flow

```
Engine → CardSender.ReplyCard(card)
  → cardContainsMention(card)?
    → Yes: card.RenderText() → Platform.Reply() → buildReplyContent() → MsgTypePost
    → No: renderCard() → replyMessage/createMessage → MsgTypeInteractive
```

## Error Handling

- If `card.RenderText()` or `Platform.Reply()`/`Send()` fails, the error propagates normally — no special handling needed.
- Log at `slog.Info` level on downgrade (not `slog.Warn`, since this is expected behavior).

## Scope

- **Platform**: Feishu only. No changes to other platforms or to `core/`.
- **Card types**: Structured cards via `ReplyCard`/`SendCard` only. Streaming cards excluded.

## Acceptance Criteria

1. Card messages containing `<at>` tags are downgraded to Post (rich text) format
2. Card messages without `<at>` tags are sent as Interactive Card (no behavior change)
3. Downgraded messages have working mentions (bots receive `im.message.receive_v1` events)
4. Streaming card messages are unaffected
5. Downgrade is logged at Info level

## Risks

| Risk | Mitigation |
|------|-----------|
| `card.RenderText()` loses card formatting (buttons, layout) | Accepted trade-off; mention functionality is more important than visual formatting |
| False positives if `<at ` appears in code blocks or literal text | Low likelihood; `<at ` with a space after is distinctive to Feishu mention syntax |
