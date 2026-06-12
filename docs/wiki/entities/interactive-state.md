# Interactive State

Tracks the state of an interactive (turn-based) session in `core/engine.go`. Contains session reference, pending permissions, and turn metadata.

Risks: cleanup in `stopInteractiveSessionWithOptions` does not pass `expected` parameter, so a concurrent new turn can have its state destroyed by a stale cleanup. `OnPlatformUnavailable` does not clean up interactive state entries, causing ghost messages or hangs on reconnection.

Cross-references: [pending-permission](./pending-permission.md), [session-manager](./session-manager.md), [permission-hang](../concepts/permission-hang.md)
