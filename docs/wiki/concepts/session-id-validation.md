# Session ID Validation

Optional interface (`SessionIDValidator`) that agents implement to validate session IDs before resuming. If not implemented, the engine resumes with whatever `AgentSessionID` is stored, which could belong to a different project sharing the same workspace directory.

Cross-project session leakage means a user could resume a conversation from a different project, exposing sensitive data or executing actions in the wrong context. Severity: High — only agents implementing the interface are protected.

Project relation: `core/interfaces.go`, `core/engine.go`.

Cross-references: [session-manager](../entities/session-manager.md), [interactive-state](../entities/interactive-state.md)
