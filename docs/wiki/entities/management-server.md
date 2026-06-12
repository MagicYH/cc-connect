# ManagementServer

REST API server for external management tools (web dashboards, TUI clients, Mac tray apps). Default port 9820. Bearer token auth with CORS origin allowlist.

**Implementation:** `core/management.go:202` (`ManagementServer`)
**Config:** `[management]` in `config.toml`
**Endpoints:** `/status`, `/restart`, `/reload`, `/config`, `/agents`, `/projects`, `/cron`, `/providers`, `/skills`, `/bridge/adapters`, `/setup/feishu/*`, `/setup/weixin/*`

Cross-references: [APIServer](api-server.md), [BridgeServer](bridge-server.md)
