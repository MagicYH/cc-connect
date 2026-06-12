# Platform Interface

The core `Platform` interface in `core/interfaces.go` defines the contract all messaging platform adapters must implement.

## Definition

```go
type Platform interface {
    Name() string
    Start(handler MessageHandler) error
    Reply(ctx context.Context, replyCtx any, content string) error
    Send(ctx context.Context, replyCtx any, content string) error
    Stop() error
}
```

## Project Relation

Each platform package (`platform/feishu/`, `platform/telegram/`, etc.) implements this interface. Platforms opt into additional capabilities via optional interfaces (`CardSender`, `ImageSender`, etc.).

## Cross-References

- [capability-interface](../concepts/capability-interface.md) — optional interfaces platforms can implement
- [plugin-registry](../concepts/plugin-registry.md) — how platforms register themselves
- [agent-interface](./agent-interface.md) — the corresponding agent contract
