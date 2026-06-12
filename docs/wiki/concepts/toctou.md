# TOCTOU (Time-of-Check-to-Time-of-Use)

Race condition where state is checked then acted upon with an unlock gap in between. In `handlePendingPermission`, releasing `interactiveMu` before acquiring `state.mu` allows concurrent cleanup. In `workspace_binding.go`, re-reading from disk between lookup and use can race with concurrent unbinds.

Project relation: affects permission handling (`core/engine.go`) and workspace binding (`core/workspace_binding.go`).

Cross-references: [pending-permission](../entities/pending-permission.md), [workspace-binding](../entities/workspace-binding.md)
