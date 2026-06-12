# External Dependencies

[Raw scan](../raw/external-dependencies.md)

## Messaging Platform APIs

13 messaging platforms supported via their official APIs:

- **Feishu/Lark** -- WebSocket + REST via official Go SDK (`larksuite/oapi-sdk-go/v3`)
- **Telegram** -- Long-poll + WebSocket via `go-telegram/bot`
- **Discord** -- Gateway WebSocket + REST via `bwmarrin/discordgo`
- **Slack** -- Socket Mode + REST via `slack-go/slack`
- **DingTalk** -- Stream SDK + REST via `open-dingtalk/dingtalk-stream-sdk-go`
- **WeChat Work (WeCom)** -- HTTPS callback + WebSocket; custom implementation
- **WeChat Personal (Weixin)** -- Long-poll + CDN with AES-128-ECB decryption
- **QQ (NapCat)** -- WebSocket via OneBot v11 protocol; connects to user-hosted NapCat server
- **QQ Bot Official** -- WebSocket + REST
- **LINE** -- Webhook + REST via `line/line-bot-sdk-go/v8`
- **Weibo DM** -- WebSocket
- **WPS Xiezuo** -- WebSocket + REST
- **MAX** -- Long-poll/webhook + REST

All platforms register via `core.RegisterPlatform()` and implement the `core.Platform` interface. `gorilla/websocket` is shared across QQ, QQBot, WeCom, WPS Xiezuo, and Bridge.

## AI Agent Provider APIs

Provider APIs are used for STT, TTS, and health checks -- not for direct agent communication:

- **Anthropic** -- `doctor` health check; indirect via Claude Code CLI
- **OpenAI** -- STT (Whisper), TTS; indirect via Codex/Copilot CLIs
- **Google Gemini** -- STT; indirect via Gemini CLI
- **DashScope/Qwen** -- STT (Qwen ASR), TTS (Qwen TTS)
- **MiniMax** -- TTS; OpenAI-compatible agent provider
- **Xiaomi MiMo** -- TTS provider
- **Groq** -- STT (Whisper)
- **AWS Bedrock / Google Vertex AI** -- Indirect via env vars on Claude Code / Gemini CLIs
- **SiliconFlow / OpenAI-compatible** -- Third-party proxy for Claude Code
- **Claude Code Router** -- Local router (`127.0.0.1:3456`) for model selection

## Self-Hosted HTTP Services

| Service | Port | Purpose |
|---------|------|---------|
| Webhook Server | 9111 | Git hooks, CI/CD triggers |
| Bridge Platform | 9810 | External adapter WebSocket connections |
| Management API | 9820 | REST API for web dashboards, TUI, tray apps |
| Codex App Server | 3845 | Codex sandbox loopback (internal) |

## Update & Preset Services

- **GitHub Releases API** -- Auto-update checks and version info
- **Gitee Releases API** -- Fallback update source for China users
- **GitHub/Gitee Raw** -- Remote provider presets and skill presets JSON

Both Gitee endpoints serve as automatic fallbacks when GitHub is unreachable.

## Local Storage & CLI Dependencies

- **SQLite** (`modernc.org/sqlite`) -- Embedded DB for provider switching
- **sqlite3 binary** -- Reading Cursor/OpenCode session metadata from their local DBs
- **ffmpeg** -- Audio format conversion for STT/TTS pipelines
- **pty** (`creack/pty`) -- Pseudo-terminal for agent subprocess management

## Agent CLI Binaries

12 external CLI tools spawned as child processes: `claude`, `codex`, `agent` (Cursor), `gemini`, `iflow`, `kimi`, `opencode`, `qodercli`, `devin`, `agy`, `copilot`, `openclaw`, plus `tmux` for the tmux agent type.

cc-connect does not call AI provider APIs on behalf of agent CLIs; each CLI manages its own outbound calls independently.
