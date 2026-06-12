# Pending Permission

Struct representing a permission request awaiting user response (Allow/Deny). Created in `core/engine.go` during `control_request` handling.

Blocked on `<-pending.Resolved` until the user responds via platform. Resolution uses `sync.Once` for safety. If the platform disconnects before response, the goroutine hangs forever — no automatic timeout exists. Callbacks are matched by session key, not RequestID, so stale card approvals can resolve the wrong request.

Cross-references: [permission-hang](../concepts/permission-hang.md), [toctou](../concepts/toctou.md), [interactive-state](./interactive-state.md)
