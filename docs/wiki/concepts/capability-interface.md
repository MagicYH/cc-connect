# Capability Interface

The structural typing pattern using ~25 optional interfaces in `core/interfaces.go` that platforms and agents opt into by implementing.

## How It Works

Core defines small optional interfaces (e.g. `CardSender`, `ImageSender`, `TypingIndicator`, `ProviderSwitcher`, `ModelSwitcher`, `StreamingCardPlatform`). Code checks capability at runtime via type assertion:

```go
if sender, ok := p.(core.ImageSender); ok {
    sender.SendImage(ctx, replyCtx, img)
}
```

This avoids hardcoded name checks and type switches in core.

## Project Relation

Replaces what would otherwise be `if p.Name() == "telegram"` checks. Each platform/agent implements only the capabilities it supports. A few special-case hardcodes remain in engine.go where behavior could not be expressed via interfaces.

## Cross-References

- [platform-interface](../entities/platform-interface.md) — base interface capabilities extend
- [agent-interface](../entities/agent-interface.md) — base agent interface
- [card-builder](../entities/card-builder.md) — uses `CardSender` / `StreamingCardPlatform` capabilities

From [project-structure](../sources/project-structure.md): `core/interfaces.go` defines 51 interfaces total (10 core + 41 optional capability). Platform capabilities include `CardSender`, `InlineButtonSender`, `ImageSender`, `FileSender`, `MessageUpdater`, `StatusFooterSender`, `StreamingCard`, `AtMentionSender`, `TypingIndicator`, etc. Agent capabilities include `ProviderSwitcher`, `UsageReporter`, `ContextCompressor`, `ToolAuthorizer`, `SkillProvider`, `AgentDoctorInfo`, etc.
