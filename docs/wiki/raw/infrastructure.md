# Infrastructure Components

## Local Storage

### File-Based Session Store
- **Type**: JSON file persistence
- **Purpose**: Stores active session mappings (session_key -> session_id) per project and work_dir
- **Location**: `~/.cc-connect/<project>/<work_dir_hash>/sessions.json`
- **Config**: `data_dir` in `config.toml` (default: `~/.cc-connect`)
- **Initialized in**: `cmd/cc-connect/main.go:328` (`sessionStorePath`)
- **Implementation**: `core/session.go` (`SessionManager`)

### Cron Job Store
- **Type**: JSON file persistence
- **Purpose**: Persists scheduled cron jobs across restarts
- **Location**: `<data_dir>/crons/crons.json`
- **Initialized in**: `core/cron.go:123` (`NewCronStore`)
- **Implementation**: `core/cron.go` (`CronStore`, `CronScheduler`)

### Timer Job Store
- **Type**: JSON file persistence
- **Purpose**: Persists one-shot delayed timer jobs
- **Location**: `<data_dir>/timers/timers.json`
- **Initialized in**: `core/timer.go:97` (`NewTimerStore`)
- **Implementation**: `core/timer.go` (`TimerStore`, `TimerScheduler`)

### Relay Bindings Store
- **Type**: JSON file persistence
- **Purpose**: Persists bot-to-bot relay bindings for group chats
- **Location**: `<data_dir>/relay_bindings.json`
- **Initialized in**: `core/relay.go:40` (`NewRelayManager`)
- **Implementation**: `core/relay.go` (`RelayManager`)

### Directory History Store
- **Type**: JSON file persistence
- **Purpose**: Tracks directory switch history per project for multi-workspace mode
- **Location**: `<data_dir>/dir_history.json`
- **Initialized in**: `core/dir_history.go:25` (`NewDirHistory`)
- **Implementation**: `core/dir_history.go` (`DirHistory`)

### Heartbeat State Store
- **Type**: JSON file persistence
- **Purpose**: Persists per-project heartbeat pause state and interval overrides
- **Location**: `<data_dir>/heartbeat/<project>.json`
- **Initialized in**: `core/heartbeat.go` (`HeartbeatScheduler`)
- **Implementation**: `core/heartbeat.go`

### Project State Store
- **Type**: JSON file persistence
- **Purpose**: Per-project state (active workspace, etc.)
- **Location**: `<data_dir>/projects/<project>/state.json`
- **Initialized in**: `cmd/cc-connect/main.go:325` (`NewProjectStateStore`)

### MiniMax TTS Config
- **Type**: JSON config file
- **Purpose**: Stores MiniMax TTS voice configuration locally
- **Location**: `<data_dir>/config/minimax.json`
- **Config**: `config.toml` -> `[tts.minimax] config_file`
- **Initialized in**: `config/config.go:394` (`LoadMiniMaxLocalConfig`)

### cc-switch Database (SQLite)
- **Type**: SQLite database
- **Purpose**: Stores provider configurations from the cc-switch tool
- **Location**: `~/.cc-switch/cc-switch.db` (XDG-aware paths on Linux/macOS)
- **Initialized in**: `cmd/cc-connect/provider.go:317` (`sql.Open("sqlite", dbPath+"?mode=ro")`)
- **Dependency**: `modernc.org/sqlite` (pure-Go SQLite driver)
- **Read-only access**: cc-connect reads providers from cc-switch; does not write to it

### Agent-Side SQLite (External)
- **Type**: SQLite (read via `sqlite3` CLI, not Go driver)
- **Purpose**: Reads Cursor `store.db` and OpenCode `opencode.db` for session metadata and message counts
- **Implementation**: `agent/cursor/cursor.go:476` and `agent/opencode/opencode.go:662`
- **Dependency**: Requires `sqlite3` CLI binary on PATH; gracefully degrades if absent

## HTTP Servers

### Unix Socket API Server
- **Type**: HTTP server on Unix domain socket
- **Purpose**: Local API for external tools (e.g., cron jobs, `cc-connect send`) to send messages to active sessions
- **Listen address**: `<data_dir>/run/api.sock`
- **Initialized in**: `core/api.go:45` (`NewAPIServer`)
- **Endpoints**: `POST /send`, `POST /relay/send`
- **Security**: Socket file permissions 0600; only accessible by same OS user

### Webhook Server
- **Type**: HTTP server (TCP)
- **Purpose**: Receives webhook triggers from external systems (git hooks, CI/CD, file watchers)
- **Config**: `[webhook]` in `config.toml` (port default: 9111, path default: `/hook`)
- **Initialized in**: `core/webhook.go` (`NewWebhookServer`)
- **Security**: Bearer token authentication (configurable)
- **Endpoints**: `POST /hook`

### Management API Server
- **Type**: HTTP server (TCP)
- **Purpose**: REST API for external management tools (web dashboards, TUI clients, Mac tray apps)
- **Config**: `[management]` in `config.toml` (port default: 9820)
- **Initialized in**: `core/management.go:202` (`ManagementServer`)
- **Endpoints**: `/status`, `/restart`, `/reload`, `/config`, `/agents`, `/projects`, `/cron`, `/providers`, `/skills`, `/bridge/adapters`, `/setup/feishu/*`, `/setup/weixin/*`
- **Security**: Bearer token authentication; CORS origin allowlist

### Provider Proxy Server
- **Type**: Local reverse proxy (TCP, ephemeral port)
- **Purpose**: Rewrites incompatible Anthropic API fields (e.g., `thinking.type: "adaptive"`) for third-party providers
- **Listen address**: `127.0.0.1:<random-port>` (OS-assigned ephemeral port)
- **Initialized in**: `core/providerproxy.go:36` (`NewProviderProxy`)
- **Implementation**: `net/http/httputil.ReverseProxy` with request body rewriting
- **Lifecycle**: Started/stopped per-agent when provider changes; one proxy per Claude Code agent instance

## WebSocket Servers

### Bridge Server
- **Type**: WebSocket server (TCP)
- **Purpose**: Accepts connections from external platform adapters written in any language
- **Config**: `[bridge]` in `config.toml` (port default: 9810, path default: `/bridge/ws`)
- **Initialized in**: `core/bridge.go` (`BridgeServer`)
- **Security**: Token-based authentication; CORS origin validation
- **Protocol**: JSON message protocol for bidirectional message routing (see `docs/bridge-protocol.md`)

## WebSocket Clients (Platform Connections)

### Feishu/Lark WebSocket
- **Type**: WebSocket client (long-connection)
- **Purpose**: Receives messages from Feishu/Lark via the official SDK
- **SDK**: `github.com/larksuite/oapi-sdk-go/v3`
- **Implementation**: `platform/feishu/feishu.go`
- **Connection**: Client-initiated; auto-negotiated via app_id/app_secret credentials

### WeChat Work (WeCom) WebSocket
- **Type**: WebSocket client (long-connection)
- **Purpose**: Receives messages from WeChat Work AI Bot in WebSocket mode
- **SDK**: `github.com/gorilla/websocket`
- **Implementation**: `platform/wecom/websocket.go`
- **Connection**: Client-initiated; uses bot_id/bot_secret authentication

### Weibo WebSocket
- **Type**: WebSocket client (long-connection)
- **Purpose**: Receives and sends DM messages via Weibo's WebSocket API
- **SDK**: `github.com/gorilla/websocket`
- **Implementation**: `platform/weibo/weibo.go`
- **Connection**: Client-initiated; uses app_id/app_secret for token exchange

### WPS Xiezuo WebSocket
- **Type**: WebSocket client (long-connection)
- **Purpose**: Receives messages from WPS collaboration platform
- **SDK**: `github.com/gorilla/websocket`
- **Implementation**: `platform/wps-xiezuo/wpsxiezuo.go`
- **Connection**: Client-initiated; uses app_id/app_secret authentication

### QQ Bot WebSocket
- **Type**: WebSocket client
- **Purpose**: Receives messages from QQ Bot official platform
- **Implementation**: `platform/qqbot/qqbot.go`

### DingTalk Stream WebSocket
- **Type**: WebSocket client (stream mode)
- **Purpose**: Receives messages from DingTalk via stream SDK
- **SDK**: `github.com/open-dingtalk/dingtalk-stream-sdk-go`
- **Implementation**: `platform/dingtalk/dingtalk.go`

## HTTP Client Connections (Platform APIs)

### Telegram Bot API
- **Type**: Long-polling HTTP client
- **Purpose**: Receives and sends messages via Telegram Bot API
- **SDK**: `github.com/go-telegram/bot`
- **Implementation**: `platform/telegram/telegram.go`

### Slack Socket Mode
- **Type**: WebSocket client (Socket Mode)
- **Purpose**: Receives and sends messages via Slack's Socket Mode
- **SDK**: `github.com/slack-go/slack`
- **Implementation**: `platform/slack/slack.go`

### Discord Gateway
- **Type**: Gateway WebSocket + REST API
- **Purpose**: Receives and sends messages via Discord Gateway
- **SDK**: `github.com/bwmarrin/discordgo`
- **Implementation**: `platform/discord/discord.go`

### LINE Webhook
- **Type**: HTTP webhook receiver + REST API sender
- **Purpose**: Receives messages via HTTP callback; sends via LINE Messaging API
- **SDK**: `github.com/line/line-bot-sdk-go/v8`
- **Implementation**: `platform/line/line.go`

### WeChat Work (WeCom) HTTP Callback
- **Type**: HTTP webhook receiver + REST API sender
- **Purpose**: Receives messages via HTTP callback; sends via WeCom API
- **Implementation**: `platform/wecom/wecom.go`
- **Config**: `port`, `callback_path`, corp credentials

### QQ (via OneBot/NapCat)
- **Type**: WebSocket client (forward WebSocket to NapCat)
- **Purpose**: Receives/sends messages via OneBot v11 protocol
- **Implementation**: `platform/qq/qq.go`

## Scheduling Infrastructure

### Cron Scheduler
- **Type**: Time-based job scheduler
- **Purpose**: Executes recurring tasks (prompts or shell commands) on cron schedules
- **SDK**: `github.com/robfig/cron/v3`
- **Initialized in**: `core/cron.go:423` (`NewCronScheduler`)
- **Features**: Per-job session mode (reuse/new_per_run), silent/mute modes, permission mode overrides, exec timeout

### Timer Scheduler
- **Type**: One-shot delayed task scheduler
- **Purpose**: Executes delayed tasks (prompts or shell commands) at a specific future time
- **Initialized in**: `core/timer.go` (`TimerScheduler`)
- **Features**: Same job features as cron; auto-cleanup of fired jobs

### Heartbeat Scheduler
- **Type**: Periodic interval scheduler
- **Purpose**: Periodically wakes agent to check environment and continue unfinished work
- **Initialized in**: `core/heartbeat.go` (`HeartbeatScheduler`)
- **Features**: Idle-only mode, configurable interval, prompt or HEARTBEAT.md, pause/resume

## Media Processing

### Speech-to-Text (STT)
- **Type**: External API integration
- **Purpose**: Transcribes voice messages to text before sending to agents
- **Providers**: OpenAI (whisper-1), Groq (whisper-large-v3-turbo), Qwen
- **Dependency**: Requires `ffmpeg` for audio format conversion (AMR/OGG -> MP3)
- **Implementation**: `core/speech.go`
- **Config**: `[speech]` in `config.toml`

### Text-to-Speech (TTS)
- **Type**: External API integration
- **Purpose**: Synthesizes AI text replies into voice messages
- **Providers**: Qwen (qwen3-tts-flash), OpenAI (tts-1), MiniMax (speech-2.8-hd), MiMo (mimo-v2.5-tts), espeak (local), pico (local), edge (local)
- **Dependency**: Requires `ffmpeg` for WAV -> Opus conversion (Feishu requirement)
- **Implementation**: `core/tts.go`
- **Config**: `[tts]` in `config.toml`

## Daemon / Service Management

### systemd (Linux)
- **Type**: User-level systemd service
- **Purpose**: Runs cc-connect as a background service on Linux
- **Implementation**: `daemon/systemd.go`
- **Features**: Install, uninstall, start, stop, restart, status; log file rotation; environment variable capture; linger check

### launchd (macOS)
- **Type**: LaunchAgent plist
- **Purpose**: Runs cc-connect as a background service on macOS
- **Implementation**: `daemon/launchd.go`
- **Label**: `com.cc-connect.service`
- **Features**: Install, uninstall, start, stop, restart, status; auto-restart on crash

### Windows Service
- **Type**: Windows service
- **Purpose**: Runs cc-connect as a background service on Windows
- **Implementation**: `daemon/windows.go`

## Rate Limiting

### Inbound Rate Limiter
- **Type**: Sliding-window per-key rate limiter
- **Purpose**: Prevents users from flooding the bot with too many messages
- **Implementation**: `core/ratelimit.go` (`RateLimiter`)
- **Config**: `[rate_limit]` in `config.toml` (default: 20 messages per 60s window)

### Outgoing Rate Limiter
- **Type**: Token-bucket per-platform rate limiter
- **Purpose**: Throttles outgoing messages to platforms to prevent API rate limit violations
- **Implementation**: `core/outgoing_ratelimit.go` (`OutgoingRateLimiter`)
- **Config**: `[outgoing_rate_limit]` in `config.toml`; per-platform overrides supported

## Event Hooks System

### Lifecycle Hook Engine
- **Type**: Event-driven hook executor
- **Purpose**: Executes shell commands or HTTP requests on lifecycle events
- **Events**: `message.received`, `message.sent`, `session.started`, `session.ended`, `cron.triggered`, `timer.triggered`, `permission.requested`, `error`, `*` (wildcard)
- **Handlers**: `command` (shell exec), `http` (POST JSON)
- **Implementation**: `core/hooks.go` (`HookRunner`)
- **Config**: `[[hooks]]` array in `config.toml`
- **Features**: Async by default, configurable timeout, environment variables with CC_HOOK_* prefix

## Subprocess Management

### Agent Process Spawning
- **Type**: OS process management via `os/exec`
- **Purpose**: Spawns and manages AI agent CLI processes (claude, codex, gemini, cursor, etc.)
- **Implementation**: Each agent package (`agent/*/`) spawns its CLI binary
- **Features**: Context-based cancellation, PTY allocation (`github.com/creack/pty`), stdout/stderr streaming, permission prompt interception

### Shell Command Execution
- **Type**: OS process execution
- **Purpose**: Executes `/shell` commands, cron exec, hook commands
- **Implementation**: Engine delegates to agent session or direct exec
- **Security**: `admin_from` controls who can run privileged commands; `run_as_user` for OS-level isolation

## Scan Warnings

- The cc-switch SQLite database is read in read-only mode from an external tool's data directory. Its schema is not owned by cc-connect and may change independently.
- Agent-side SQLite access (Cursor `store.db`, OpenCode `opencode.db`) relies on the `sqlite3` CLI binary rather than a Go driver, and gracefully degrades when absent.
- The project has no traditional database (MySQL, PostgreSQL, Redis, etc.) or message queue (Kafka, RabbitMQ). All persistence is file-based (JSON files) or delegated to external agent tools' databases.
- No infrastructure keywords matching ByteDance internal services (RDS, TCC, TOS, ABase, etc.) were found, confirming this is an open-source project without internal platform dependencies.
