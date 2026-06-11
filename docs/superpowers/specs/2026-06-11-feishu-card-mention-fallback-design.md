# Feishu: Card Mention Fallback Design

## Problem

When a Feishu interactive card message contains a mention, the mentioned bot cannot read its own mention from the incoming event. Card `<at>` tags are visual-only and do not populate the `mentions` field that triggers `im.message.receive_v1` events for bots.

The existing `buildReplyContent` (feishu.go:2637-2643) already detects `<at>` tags in plain text/markdown replies and falls back from `MsgTypeInteractive` to `MsgTypePost`. However, the `ReplyCard`/`SendCard` path in `interactivePlatform` (card.go) bypasses this logic entirely — structured cards are sent without any mention check.

Additionally, `resolveMentionsInContent` (which converts `@name` to `<at>` tags) is only called in `Platform.Reply()`/`Send()`, not in the card path. This means `@AgentB` text in card content is never resolved to `<at>` tags, and the card is sent with the raw `@AgentB` text — no mention notification is triggered at all.

## Solution

At the `ReplyCard`/`SendCard` entry points, extract card text content and run `resolveMentionsInContent` to resolve `@name` patterns into `<at>` tags. If any `<at>` tags are found after resolution, downgrade to Post (rich text) format using the resolved content, which ensures mentions work correctly.

## Changes

### 1. New function: `cardTextContent` (card.go)

```go
func cardTextContent(card *core.Card) string
```

Traverse all card elements and concatenate text content:
- `CardMarkdown`: append `Content`
- `CardNote`: append `Content`
- `CardActions`: append each button's `Text`
- `CardListItem`: append inner text fields

Returns the combined text for mention resolution.

### 2. New function: `cardContainsMention` (card.go)

```go
func (p *interactivePlatform) cardContainsMention(card *core.Card) (bool, string)
```

1. Extract text via `cardTextContent(card)`
2. Call `p.Platform.resolveMentionsInContent(chatID, content)` to resolve `@name` patterns
3. Check if resolved content contains `<at `
4. Return `(true, resolvedContent)` if mention found, `(false, "")` otherwise

This handles two sources of mentions:
- **Already-resolved `<at>` tags**: present in content from prior resolution
- **`@name` patterns**: resolved by `resolveMentionsInContent` using chat member lookup

### 3. Modify `interactivePlatform.ReplyCard()` (card.go)

Before sending the card:

```
hasMention, resolvedContent := p.cardContainsMention(card)
if hasMention {
    slog.Info("feishu: card contains mention, falling back to post format")
    return p.Platform.Reply(chatID, msgID, resolvedContent)
}
// existing card send logic
```

Note: `resolvedContent` is used instead of `card.RenderText()` because it contains the properly resolved `<at>` tags. `Platform.Reply()` will call `buildReplyContent()`, which detects `<at>` tags and selects `MsgTypePost`.

### 4. Modify `interactivePlatform.SendCard()` (card.go)

Same pattern:

```
hasMention, resolvedContent := p.cardContainsMention(card)
if hasMention {
    slog.Info("feishu: card contains mention, falling back to post format")
    return p.Platform.Send(chatID, resolvedContent)
}
// existing card send logic
```

### 5. No changes to streaming card methods

`StartStreamingCard`, `AppendStreamingCard`, `FinishStreamingCard` are not modified. The streaming card path does not go through `resolveMentionsInContent`, so mentions are not injected into streaming content. Adding detection here would add complexity without practical benefit.

### 6. No changes to `RefreshCard`

If a message was downgraded from card to post at send time, it never gets a message ID stored for card refresh, so `RefreshCard` is never called for downgraded messages.

## Data Flow

```
Engine → CardSender.ReplyCard(card)
  → cardTextContent(card) → resolveMentionsInContent(chatID, text)
  → contains "<at "?
    → Yes: Platform.Reply(chatID, msgID, resolvedContent)
           → buildReplyContent() → detects <at> → MsgTypePost (rich text)
    → No: renderCard() → replyMessage/createMessage → MsgTypeInteractive
```

## Error Handling

- If `resolveMentionsInContent` fails (e.g., chat member lookup fails), the check still proceeds — unresolved `@name` patterns simply won't be detected. This is acceptable because the mention wouldn't have worked in a card anyway.
- If `Platform.Reply()`/`Send()` fails, the error propagates normally.
- Log at `slog.Info` level on downgrade (not `slog.Warn`, since this is expected behavior).

## Scope

- **Platform**: Feishu only. No changes to other platforms or to `core/`.
- **Card types**: Structured cards via `ReplyCard`/`SendCard` only. Streaming cards excluded.

## Acceptance Criteria

1. Card messages containing `<at>` tags are downgraded to Post (rich text) format
2. Card messages containing `@name` patterns that resolve to valid chat members are downgraded to Post format with working `<at>` tags
3. Card messages without mentions are sent as Interactive Card (no behavior change)
4. Downgraded messages have working mentions (mentioned bots receive `im.message.receive_v1` events)
5. Streaming card messages are unaffected
6. Downgrade is logged at Info level

## Risks

| Risk | Mitigation |
|------|-----------|
| `card.RenderText()` formatting loss (buttons, layout) | Accepted trade-off; mention functionality is more important than visual formatting |
| `@name` in code blocks or literal text may trigger false positive | Low likelihood; `resolveMentionsInContent` only matches names that exist in chat member list |
| `resolveMentionsInContent` chat member lookup adds latency | Cached for 1 hour; overhead is negligible on cache hit, acceptable on miss |
| Downgraded message loses streaming/refresh capability | Accepted; mention correctness takes priority |
