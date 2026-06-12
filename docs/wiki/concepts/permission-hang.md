# Permission Hang

When a permission prompt (`control_request`) is sent to the user but the platform disconnects before the user responds, the event loop goroutine blocks on `<-pending.Resolved` forever. No automatic timeout exists for permission requests.

Partial mitigations: `/cancel` command, `reset_on_idle_mins` config, and `cleanupInteractiveState` (called only on explicit stop or idle reset).

Severity: High — requires platform disconnect during active permission prompt.

Project relation: `core/engine.go:4781-4812`.

Cross-references: [pending-permission](../entities/pending-permission.md), [interactive-state](../entities/interactive-state.md)
