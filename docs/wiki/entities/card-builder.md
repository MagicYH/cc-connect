# CardBuilder

A fluent builder in `core/` for constructing rich card messages across platforms.

## Usage

```go
card := NewCard().
    Title(e.i18n.T(MsgCardTitleStatus), "blue").
    Markdown(content).
    Divider().
    Buttons(PrimaryBtn("OK", "cmd:/ok")).
    Note("footer text").
    Build()
```

## Project Relation

Card elements (`CardMarkdown`, `CardDivider`, `CardActions`, `CardNote`, `CardListItem`, `CardSelect`) implement the unexported `cardElement()` marker interface. `Card.RenderText()` provides plain-text fallback for platforms without card support. Platforms opt in via the `CardSender` / `StreamingCardPlatform` optional interfaces.

## Cross-References

- [capability-interface](../concepts/capability-interface.md) — `CardSender` and `StreamingCardPlatform` optional interfaces
- [i18n](./i18n.md) — translations used in card titles/content
