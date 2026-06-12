# External Dependencies

## Messaging Platform APIs

| Service | Protocol | Purpose | Call Location | Config Location |
|---------|----------|---------|---------------|-----------------|
| Feishu / Lark (open.feishu.cn / open.larksuite.com) | WebSocket + HTTPS (SDK) | Receive/send messages, interactive cards, reactions, file download | `platform/feishu/` | `config.example.toml` `[projects.platforms] type="feishu"` |
| Telegram Bot API (api.telegram.org) | HTTPS (long-poll) + WebSocket | Receive/send messages, reactions, file download, proxy support | `platform/telegram/telegram.go` | `config.example.toml` `[projects.platforms] type="telegram"` |
| Discord Gateway (discord.com) | WebSocket (Gateway) + HTTPS REST | Receive/send messages, slash commands, threads, reactions | `platform/discord/discord.go` | `config.example.toml` `[projects.platforms] type="discord"` |
| Slack (slack.com) | WebSocket (Socket Mode) + HTTPS | Receive/send messages, thread sessions | `platform/slack/slack.go` | `config.example.toml` `[projects.platforms] type="slack"` |
| DingTalk (api.dingtalk.com) | WebSocket (Stream SDK) + HTTPS | Receive/send messages, AI cards, file download/upload, reactions | `platform/dingtalk/dingtalk.go` | `config.example.toml` `[projects.platforms] type="dingtalk"` |
| WeChat Work / WeCom (qyapi.weixin.qq.com) | HTTPS (callback) + WebSocket | Receive/send messages, file/image download, Markdown messages | `platform/wecom/wecom.go`, `platform/wecom/websocket.go` | `config.example.toml` `[projects.platforms] type="wecom"` |
| WeChat Personal / Weixin (ilinkai.weixin.qq.com) | HTTPS (long-poll) + CDN | Receive/send messages, CDN download/upload with AES-128-ECB decryption | `platform/weixin/client.go`, `platform/weixin/cdn.go` | `config.example.toml` `[projects.platforms] type="weixin"` |
| QQ via NapCat (ws://127.0.0.1:3001) | WebSocket (OneBot v11) | Receive/send messages, file operations | `platform/qq/` | `config.example.toml` `[projects.platforms] type="qq"` |
| QQ Bot Official (api.sgroup.qq.com) | WebSocket + HTTPS | Receive/send messages, inline keyboard, markdown messages | `platform/qqbot/qqbot.go` | `config.example.toml` `[projects.platforms] type="qqbot"` |
| LINE Messaging API (api.line.me) | HTTPS (webhook) | Receive/send messages, media download | `platform/line/line.go` | `config.example.toml` `[projects.platforms] type="line"` |
| Weibo DM (open-im.api.weibo.com) | WebSocket | Receive/send private messages | `platform/weibo/weibo.go` | `config.example.toml` `[projects.platforms] type="weibo"` |
| WPS Xiezuo (openapi.wps.cn) | WebSocket + HTTPS | Receive/send messages, file upload | `platform/wps-xiezuo/wpsxiezuo.go` | `config.example.toml` `[projects.platforms] type="wps-xiezuo"` |
| MAX (platform-api.max.ru) | HTTPS (long-poll / webhook) | Receive/send messages | `platform/max/max.go` | `config.example.toml` `[projects.platforms] type="max"` |

## AI Agent Provider APIs

| Service | Protocol | Purpose | Call Location | Config Location |
|---------|----------|---------|---------------|-----------------|
| Anthropic API (api.anthropic.com) | HTTPS | Health check for `cc-connect doctor`; used indirectly by Claude Code CLI | `core/doctor.go` | `config.example.toml` `[projects.agent.providers] name="anthropic"` |
| OpenAI API (api.openai.com) | HTTPS | Used indirectly by Codex CLI and Copilot CLI; STT Whisper provider; TTS provider | `core/speech.go`, `core/tts.go` | `config.example.toml` `[projects.agent.providers] name="openai"` |
| Google Gemini (generativelanguage.googleapis.com) | HTTPS | STT provider; used indirectly by Gemini CLI agent | `core/speech.go`, `agent/gemini/gemini.go` | `config.example.toml` `[projects.agent.providers] name="google"` |
| DashScope / Qwen (dashscope.aliyuncs.com) | HTTPS | STT (Qwen ASR) provider; TTS (Qwen TTS) provider | `core/speech.go`, `core/tts.go` | `config.example.toml` `[speech] provider="qwen"`, `[tts] provider="qwen"` |
| MiniMax (api.minimaxi.com / api.minimax.io) | HTTPS | TTS provider; OpenAI-compatible agent provider | `core/tts.go` | `config.example.toml` `[tts] provider="minimax"`, `[projects.agent.providers] name="minimax"` |
| Xiaomi MiMo (api.xiaomimimo.com) | HTTPS | TTS provider | `core/tts.go` | `config.example.toml` `[tts] provider="mimo"` |
| Groq (api.groq.com) | HTTPS | STT (Whisper) provider | `core/speech.go` | `config.example.toml` `[speech] provider="groq"` |
| AWS Bedrock (bedrock-runtime.us-east-1.amazonaws.com) | HTTPS | Claude Code via Bedrock (indirect, env-based) | `agent/claudecode/` | `config.example.toml` `[projects.agent.providers] env={CLAUDE_CODE_USE_BEDROCK="1"}` |
| Google Vertex AI (us-east1-aiplatform.googleapis.com) | HTTPS | Gemini CLI via Vertex AI (indirect, env-based) | `agent/gemini/` | `config.example.toml` `[projects.agent.providers] env={GOOGLE_GENAI_USE_VERTEXAI="true"}` |
| SiliconFlow / third-party OpenAI-compatible | HTTPS | Claude Code via third-party proxy (indirect) | `agent/claudecode/` | `config.example.toml` `[projects.agent.providers] thinking="disabled"` |
| Claude Code Router (127.0.0.1:3456) | HTTPS | Local router for model provider selection | `agent/claudecode/` | `config.example.toml` `router_url` |

## Self-Hosted / Built-in HTTP Services

| Service | Protocol | Purpose | Listen Location | Config Location |
|---------|----------|---------|-----------------|-----------------|
| Webhook Server | HTTPS (inbound) | External triggers (git hooks, CI/CD) | `core/webhook.go` | `config.example.toml` `[webhook]` port=9111 |
| Bridge Platform (WebSocket) | WebSocket (inbound) | External adapter connections in any language | `core/bridge.go` | `config.example.toml` `[bridge]` port=9810 |
| Management API | HTTPS (inbound) | REST API for web dashboards, TUI, tray apps | `core/management.go` | `config.example.toml` `[management]` port=9820 |
| Codex App Server | HTTP (inbound) | Local app server for Codex sandbox communication | `agent/codex/codex.go` | Internal, `ws://127.0.0.1:3845` |
| Timer HTTP Callback | HTTP (outbound) | One-shot delayed task HTTP callbacks | `cmd/cc-connect/timer.go` | Internal |

## Update & Preset Services

| Service | Protocol | Purpose | Call Location | Config Location |
|---------|----------|---------|---------------|-----------------|
| GitHub Releases API (api.github.com/repos/chenhg5/cc-connect/releases) | HTTPS | Auto-update check, version info | `core/updater.go`, `cmd/cc-connect/update.go` | Automatic |
| Gitee Releases API (gitee.com/api/v5/repos/cg33/cc-connect/releases) | HTTPS | Fallback update source for China users | `core/updater.go`, `cmd/cc-connect/update.go` | Automatic |
| GitHub Raw (raw.githubusercontent.com/chenhg5/cc-connect/main/provider-presets.json) | HTTPS | Remote provider presets list | `core/provider_presets.go` | `config.example.toml` `provider_presets_url` |
| Gitee Raw (gitee.com/chenhg5/cc-connect/raw/main/provider-presets.json) | HTTPS | Fallback provider presets for China users | `core/provider_presets.go` | Automatic fallback |
| GitHub Raw (skill-presets.json) | HTTPS | Remote skill presets list | `core/skill_presets.go` | Automatic |
| Gitee Raw (skill-presets.json) | HTTPS | Fallback skill presets for China users | `core/skill_presets.go` | Automatic fallback |

## Local Storage & CLI Dependencies

| Dependency | Type | Purpose | Call Location | Config Location |
|------------|------|---------|---------------|-----------------|
| SQLite (modernc.org/sqlite) | Embedded DB | Provider switching database (`cc-switch`) | `cmd/cc-connect/provider.go` | Internal |
| sqlite3 CLI binary | OS binary | Read Cursor/OpenCode session metadata from their local SQLite databases | `agent/cursor/cursor.go`, `agent/opencode/opencode.go` | Auto-detected on PATH |
| ffmpeg | OS binary | Audio format conversion for STT (AMR/OGG -> MP3) and TTS (WAV -> Opus) | `core/speech.go`, `core/tts.go` | Auto-detected on PATH |
| pty (creack/pty) | Go library | Pseudo-terminal for agent subprocess management | `agent/claudecode/`, `agent/codex/`, others | `go.mod` |

## Platform SDK Libraries

| Library | Version | Purpose | Used By |
|---------|---------|---------|---------|
| github.com/larksuite/oapi-sdk-go/v3 | v3.5.3 | Feishu/Lark official Go SDK (WebSocket + REST) | `platform/feishu/` |
| github.com/go-telegram/bot | v1.20.0 | Telegram Bot API Go SDK | `platform/telegram/` |
| github.com/bwmarrin/discordgo | v0.29.0 | Discord Go SDK | `platform/discord/` |
| github.com/slack-go/slack | v0.16.0 | Slack Go SDK (Socket Mode + REST) | `platform/slack/` |
| github.com/open-dingtalk/dingtalk-stream-sdk-go | v0.9.1 | DingTalk Stream mode SDK | `platform/dingtalk/` |
| github.com/line/line-bot-sdk-go/v8 | v8.19.0 | LINE Messaging API SDK | `platform/line/` |
| github.com/gorilla/websocket | v1.5.0 | WebSocket client/server for QQ, QQBot, WeCom, WPS Xiezuo, Bridge | `platform/qq/`, `platform/qqbot/`, `platform/wecom/`, `platform/wps-xiezuo/`, `core/bridge.go` |

## Utility Libraries (Indirect External Dependencies)

| Library | Version | Purpose |
|---------|---------|---------|
| github.com/BurntSushi/toml | v1.6.0 | TOML config file parsing |
| github.com/robfig/cron/v3 | v3.0.1 | Cron job scheduling |
| github.com/charmbracelet/bubbletea | v1.3.10 | TUI framework (terminal UI) |
| github.com/charmbracelet/bubbles | v1.0.0 | TUI components (table, viewport) |
| github.com/charmbracelet/lipgloss | v1.1.0 | Terminal styling |
| github.com/mdp/qrterminal/v3 | v3.2.1 | QR code rendering in terminal (WeChat setup) |
| rsc.io/qr | v0.2.0 | QR code generation |
| modernc.org/sqlite | v1.49.1 | Pure-Go SQLite driver |
| github.com/creack/pty | v1.1.24 | Unix pseudo-terminal |
| github.com/stretchr/testify | v1.9.0 | Test assertions and mocking |

## Agent CLI Dependencies (External Binaries)

These are not Go libraries but external CLI tools that cc-connect spawns as subprocesses:

| CLI Binary | Agent Type | Installation | Config Location |
|------------|-----------|--------------|-----------------|
| `claude` | claudecode | Claude Code CLI | `config.example.toml` `[projects.agent] type="claudecode"` |
| `codex` | codex | `npm install -g @openai/codex` | `config.example.toml` `[projects.agent] type="codex"` |
| `agent` | cursor | `npm i -g @anthropic-ai/cursor-agent` | `config.example.toml` `[projects.agent] type="cursor"` |
| `gemini` | gemini | `npm install -g @google/gemini-cli` | `config.example.toml` `[projects.agent] type="gemini"` |
| `iflow` | iflow | `npm install -g @iflow-ai/iflow-cli` | `config.example.toml` `[projects.agent] type="iflow"` |
| `kimi` | kimi | `pip install kimi-cli` | `config.example.toml` `[projects.agent] type="kimi"` |
| `opencode` | opencode | OpenCode CLI | `config.example.toml` `[projects.agent] type="opencode"` |
| `qodercli` | qoder | `curl -fsSL https://qoder.com/install \| bash` | `config.example.toml` `[projects.agent] type="qoder"` |
| `devin` | devin | Devin CLI from https://cli.devin.ai/ | `config.example.toml` `[projects.agent] type="devin"` |
| `agy` | antigravity | Antigravity CLI | `config.example.toml` `[projects.agent] type="antigravity"` |
| `copilot` | copilot | GitHub Copilot CLI | `config.example.toml` `[projects.agent] type="copilot"` |
| `openclaw` | acp | OpenClaw CLI | `config.example.toml` `[projects.agent] type="acp"` |
| `tmux` | tmux | System package | `config.example.toml` `[projects.agent] type="tmux"` |

## Scan Warnings

- QQ platform connects to a user-provided NapCat WebSocket server (default `ws://127.0.0.1:3001`); this is a local intermediary rather than a direct external API.
- WeChat Personal (weixin) CDN endpoint `novac2c.cdn.weixin.qq.com` is used for media download/upload with AES-128-ECB decryption; the CDN base URL is configurable.
- Several platform API base URLs are configurable overrides (e.g., `api_base_url` for WeCom, `domain` for Feishu/Lark, `api_base` for MAX), so the actual endpoints may differ from defaults in production.
- Codex App Server (`ws://127.0.0.1:3845`) is a local loopback HTTP server started by the Codex agent for sandbox communication; it is not a true external dependency.
- Agent CLIs (claude, codex, gemini, etc.) are spawned as child processes and make their own outbound API calls independently; cc-connect does not directly call AI provider APIs on their behalf (except for STT/TTS and doctor health checks).
- The `github.com/org/repo` import found in source is a placeholder in a code template, not a real dependency.
