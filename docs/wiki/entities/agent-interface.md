# Agent Interface

The core `Agent` interface in `core/interfaces.go` defines the contract all AI agent adapters must implement.

## Definition

```go
type Agent interface {
    Name() string
    StartSession(ctx context.Context, sessionID string) (AgentSession, error)
    ListSessions(ctx context.Context) ([]AgentSessionInfo, error)
    Stop() error
}
```

## Project Relation

Each agent package (`agent/claudecode/`, `agent/codex/`, etc.) implements this interface. Agents may also implement optional interfaces like `AgentDoctorInfo`.

## Cross-References

- [agent-session-interface](./agent-session-interface.md) — the session contract returned by `StartSession`
- [platform-interface](./platform-interface.md) — the corresponding platform contract
- [plugin-registry](../concepts/plugin-registry.md) — how agents register themselves
