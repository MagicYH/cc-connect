# AgentSession Interface

The core `AgentSession` interface in `core/interfaces.go` defines a running bidirectional session with an AI agent.

## Definition

```go
type AgentSession interface {
    Send(prompt string, images []ImageAttachment, files []FileAttachment) error
    RespondPermission(requestID string, result PermissionResult) error
    Events() <-chan Event
    CurrentSessionID() string
    Alive() bool
    Close() error
}
```

## Project Relation

Created by `Agent.StartSession()`. The engine reads events from `Events()` channel and routes permission requests back via `RespondPermission()`. Sessions are concurrency-safe; state protected with `sync.RWMutex`.

## Cross-References

- [agent-interface](./agent-interface.md) — parent interface that creates sessions
- [concurrency-safety](../concepts/concurrency-safety.md) — mutex patterns used in sessions
