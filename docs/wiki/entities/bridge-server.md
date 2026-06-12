# BridgeServer

WebSocket server accepting connections from external platform adapters written in any language. Default port 9810, path `/bridge/ws`. Token-based auth with CORS validation.

**Implementation:** `core/bridge.go` (`BridgeServer`)
**Config:** `[bridge]` in `config.toml`
**Protocol:** JSON message protocol (see `docs/bridge-protocol.md`)

Cross-references: [ManagementServer](management-server.md), [Web Admin Token](../concepts/web-admin-token.md), [Allow From](../concepts/allow-from.md), [PendingPermission](pending-permission.md)

## Risky Areas

- Token read from URL query param (`r.URL.Query().Get("token")`) — leaks into logs/proxy histories despite `subtle.ConstantTimeCompare`.
- `NewBridgeServerInsecure` creates a server that allows all requests without a token when `insecure=true` and `bs.token` is empty. No production guard prevents deploying in this mode.
- Untracked fire-and-forget goroutines (`go ref.platform.handler(...)`) with no context propagation; cleanup ordering not guaranteed during shutdown.
- Single shared "web-admin" identity with no per-user authorization — any bridge user who knows the token can send permission responses on behalf of "web-admin".
