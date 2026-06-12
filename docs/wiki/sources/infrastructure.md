# Infrastructure

[Raw scan](../raw/infrastructure.md)

## Storage

All persistence is file-based (JSON). No traditional database or message queue.

| Store | File | Purpose |
|-------|------|---------|
| SessionManager | `<data_dir>/<project>/<hash>/sessions.json` | Session key-to-ID mappings per project/work_dir |
| CronStore | `<data_dir>/crons/crons.json` | Persisted cron jobs across restarts |
| TimerStore | `<data_dir>/timers/timers.json` | One-shot delayed timer jobs |
| RelayManager | `<data_dir>/relay_bindings.json` | Bot-to-bot relay bindings |
| DirHistory | `<data_dir>/dir_history.json` | Directory switch history per project |
| HeartbeatScheduler | `<data_dir>/heartbeat/<project>.json` | Per-project heartbeat pause state |
| ProjectStateStore | `<data_dir>/projects/<project>/state.json` | Active workspace per project |
| MiniMax TTS config | `<data_dir>/config/minimax.json` | Voice config for MiniMax TTS |

Two external SQLite databases are read (not written):
- **cc-switch** (`~/.cc-switch/cc-switch.db`) -- provider configs, via pure-Go `modernc.org/sqlite`
- **Agent-side** (Cursor `store.db`, OpenCode `opencode.db`) -- session metadata, via `sqlite3` CLI

## HTTP Servers

| Server | Address | Auth | Purpose |
|--------|---------|------|---------|
| APIServer | Unix socket `api.sock` | File perm 0600 | Local `/send`, `/relay/send` for cron and CLI |
| WebhookServer | TCP, port 9111 | Bearer token | Receives git hooks, CI/CD triggers |
| ManagementServer | TCP, port 9820 | Bearer token + CORS | REST API for dashboards/TUI/tray apps |
| ProviderProxy | `127.0.0.1:<ephemeral>` | None (local only) | Rewrites Anthropic API fields for third-party providers |

## WebSocket Servers & Clients

**Server:** BridgeServer (port 9810) -- accepts external platform adapters in any language via JSON protocol.

**Clients (inbound connections):** Feishu, WeCom, Weibo, WPS Xiezuo, QQ Bot, DingTalk Stream, Slack Socket Mode, Discord Gateway, QQ (OneBot/NapCat). Most use `gorilla/websocket`; Feishu and DingTalk use their official SDKs.

**HTTP-only platforms:** Telegram (long-polling), LINE (webhook callback), WeCom HTTP callback mode.

## Scheduling

- **CronScheduler** -- recurring jobs via `robfig/cron/v3`; per-job session mode, silent/mute, permission overrides, exec timeout
- **TimerScheduler** -- one-shot delayed tasks; auto-cleanup after firing
- **HeartbeatScheduler** -- periodic wake-up for idle agents; supports HEARTBEAT.md prompts, pause/resume

## Media Processing

- **STT** -- voice-to-text via OpenAI/Groq/Qwen; requires `ffmpeg` for AMR/OGG-to-MP3 conversion
- **TTS** -- text-to-voice via Qwen/OpenAI/MiniMax/MiMo/espeak/pico/edge; requires `ffmpeg` for WAV-to-Opus

## Rate Limiting

- **Inbound** -- sliding-window per-key (default 20 msgs/60s)
- **Outbound** -- token-bucket per-platform with per-platform overrides

## Event Hooks

HookRunner executes shell commands or HTTP POSTs on lifecycle events (`message.received`, `session.started`, `cron.triggered`, `permission.requested`, etc.). Configured via `[[hooks]]` in `config.toml`. Async by default, with timeout and `CC_HOOK_*` env vars.

## Daemon / Service

Platform-native background service via systemd (Linux), launchd (macOS), or Windows Service. Managed through `cc-connect daemon` subcommands.
