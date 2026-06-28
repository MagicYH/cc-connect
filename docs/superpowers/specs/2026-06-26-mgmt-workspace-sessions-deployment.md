# Management API Workspace Session Aggregation Deployment

> Source spec: ./2026-06-26-mgmt-workspace-sessions-design.md
> Cross-reference (for v2): ./2026-06-26-mgmt-workspace-sessions-verification.md
>
> **Two-Phase Document**
> - Phase 1 (v1): authored during brainstorming via writing-deployment-plan. Describes intent.
> - Phase 2 (v2): completed during finishing-a-development-branch. Adds executable runbook.

---

## Phase 1 · Release Intent (v1)

### 1.1 Release Scope
- Affected services/components: `core/management.go`
- Includes frontend release: no
- Includes data migration: no
- Includes config change: no

### 1.2 Infrastructure Touchpoints
- DB migration: none
- Env vars: none
- TCC config: none
- New dependencies: none
- Capacity: no scaling needed — this is a local CLI tool

### 1.3 Deployment Order
| Step | Action | Must precede |
|---|---|---|
| 1 | Rebuild cc-connect binary | restart |
| 2 | Restart cc-connect daemon | verification |

### 1.4 Risk Assessment
- R1: Nil-pointer dereference on `ws.sessions` if workspace state exists but sessions haven't been initialized yet — mitigation: check `ws.sessions != nil` under `ws.mu` lock before accessing; skip workspace if nil
- R2: Lock contention from iterating all workspace states while holding their mutexes — mitigation: sequential iteration with per-workspace lock release (never hold multiple workspace locks simultaneously)
- R3: Incorrect `live`/`platform` resolution for workspace sessions — mitigation: construct `workspace:sessionKey` interactiveKey pattern matching engine.go's key format, with fallback to bare sessionKey

### 1.5 Rollback Strategy (high-level)
- Rollback trigger: any workspace session causes 500 error or crashes the management server; sessions list returns incomplete data (missing global sessions)
- Rollback scope: code only — revert the commit, rebuild, restart daemon
- Data reversibility: fully reversible — no data format changes, no session file changes

### 1.6 Observability Plan
- Key metrics: management API response status codes for `/sessions` endpoint; daemon process uptime
- Key logs: `slog.Error` / `slog.Warn` output from management handler — no new error paths expected, but watch for panic stack traces
- Observation windows: 1 minute post-restart (hit the sessions endpoint and verify response)

### 1.7 Communication
- Notify: none — single-user local development tool
- Timing: N/A
- Channels: N/A

---

## Phase 2 · Release Runbook (v2)
> Filled in during finishing-a-development-branch after implementation.

### 2.1 Pre-Deploy Checklist
> To be filled during finishing-a-development-branch Step 1.5.

### 2.2 Executable Steps
> To be filled during finishing-a-development-branch Step 1.5.

### 2.3 Post-Deploy Verification
> To be filled during finishing-a-development-branch Step 1.5.

### 2.4 Concrete Rollback Commands
> To be filled during finishing-a-development-branch Step 1.5.

### 2.5 v1 Revisions Log
> To be filled during finishing-a-development-branch Step 1.5.
