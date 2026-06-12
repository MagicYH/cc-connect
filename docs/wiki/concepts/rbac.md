# Role-Based Access Control

`UserRoleManager` in `core/user_roles.go` maps user IDs to roles. Each role has `DisabledCmds` (supports `"*"` wildcard) and optional `RateLimitCfg`. Resolution: explicit user ID match -> default role -> wildcard role. Admin allowlist (`admin_from`) is fail-closed; user allowlist (`allow_from`) is fail-open. Privileged commands require admin and are audit-logged.

Cross-references: [sliding-window-rate-limiter](../entities/sliding-window-rate-limiter.md), [token-bucket-rate-limiter](../entities/token-bucket-rate-limiter.md)
