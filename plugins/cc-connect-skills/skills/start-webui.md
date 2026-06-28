---
name: start-webui
description: Use when starting or deploying the cc-connect Web UI in production mode, accessing the management dashboard, or troubleshooting Web UI availability
---

# Start WebUI in Production Mode

## Overview

cc-connect's Web UI is a Vite-built SPA embedded into the Go binary at compile time via `go:embed`. There is no separate frontend server process — the Management Server serves both API and static files on a single port.

## When to Use

- Starting cc-connect with Web UI accessible
- Checking if Web UI is running
- Rebuilding after frontend changes
- Troubleshooting Web UI not loading

## Quick Reference

| Action | Command |
|--------|---------|
| Build (with Web UI) | `make build` |
| Build without Web UI | `make build-noweb` or `make build NO_WEB=1` |
| Rebuild frontend only | `cd web && npm run build` |
| Start foreground | `./cc-connect` |
| Daemon install | `./cc-connect daemon install` |
| Daemon start | `./cc-connect daemon start` |
| Daemon restart | `./cc-connect daemon restart` |
| Daemon status | `./cc-connect daemon status` |
| Daemon logs | `./cc-connect daemon logs -f` |

## Production Startup

**Prerequisite:** `make build` compiles the Go binary with `web/dist/` embedded. Run `make build` after any frontend change.

**Daemon mode (recommended for long-running):**

```bash
./cc-connect daemon install    # first time: register as systemd service
./cc-connect daemon start      # start in background
```

**Foreground mode:**

```bash
./cc-connect
```

## Access

The Management Server port and auth token come from `config.toml`:

```toml
[management]
enabled = true
port = 9820
token = "your-mgmt-secret"
cors_origins = ["*"]
```

- Web UI: `http://<host>:9820`
- API: `http://<host>:9820/api/v1/`
- SPA routing: non-`/api` paths fall back to `index.html`

## Key Points

- **No separate process.** Web assets are embedded; the Go binary is the only runtime artifact.
- **Rebuild required.** Frontend changes are invisible until `make build` re-embeds `web/dist/`.
- **NO_WEB tag.** Build tag `no_web` swaps `web/embed.go` for `web/embed_stub.go` (empty FS). Use `make build-noweb` to exclude Web UI entirely.
- **Dev server** (`cd web && npm run dev`) runs Vite on port 9821 with API proxy to 9820 — for frontend development only, not production.

## Common Mistakes

| Problem | Cause | Fix |
|---------|-------|-----|
| 404 on Web UI | Binary built without web | `make build` (not `build-noweb`) |
| Stale UI after edit | Forgot to rebuild | `make build && ./cc-connect daemon restart` |
| API 401 | Missing management token | Set `token` in `[management]` section |
| CORS blocked | Missing `cors_origins` | Add origins or `["*"]` in config |
