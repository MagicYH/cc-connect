# Technology Stack

## Language & Runtime

- **Go 1.25.0** — primary backend language (`go.mod`)
- **TypeScript ~5.8.3** — web dashboard frontend (`web/package.json`)
- Module path: `github.com/chenhg5/cc-connect`

## Backend Frameworks & Libraries

### Configuration

| Library | Version | Purpose |
|---------|---------|---------|
| `github.com/BurntSushi/toml` | v1.6.0 | TOML config file parsing |

### Messaging Platform SDKs

| Library | Version | Purpose | Used In |
|---------|---------|---------|---------|
| `github.com/larksuite/oapi-sdk-go/v3` | v3.5.3 | Feishu/Lark bot API | `platform/feishu/` |
| `github.com/go-telegram/bot` | v1.20.0 | Telegram Bot API | `platform/telegram/` |
| `github.com/bwmarrin/discordgo` | v0.29.0 | Discord API | `platform/discord/` |
| `github.com/slack-go/slack` | v0.16.0 | Slack API | `platform/slack/` |
| `github.com/open-dingtalk/dingtalk-stream-sdk-go` | v0.9.1 | DingTalk Stream API | `platform/dingtalk/` |
| `github.com/line/line-bot-sdk-go/v8` | v8.19.0 | LINE Messaging API | `platform/line/` |

### Networking & Communication

| Library | Version | Purpose | Used In |
|---------|---------|---------|---------|
| `github.com/gorilla/websocket` | v1.5.0 | WebSocket client/server | `platform/wecom/`, `platform/weibo/`, `platform/qq/`, `platform/wps-xiezuo/`, `core/bridge.go` |

### Terminal & Process Management

| Library | Version | Purpose | Used In |
|---------|---------|---------|---------|
| `github.com/creack/pty` | v1.1.24 | Pseudo-terminal (PTY) for agent subprocess control | `agent/iflow/session.go`, `agent/claudecode/claude_usage.go` |
| `github.com/charmbracelet/bubbletea` | v1.3.10 | TUI framework for terminal UI | `cmd/cc-connect/sessions_tui.go` |
| `github.com/charmbracelet/bubbles` | v1.0.0 | TUI components (bubbletea companion) | `cmd/cc-connect/sessions_tui.go` |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | Terminal styling/layout | `cmd/cc-connect/sessions_tui.go` |

### Data Storage

| Library | Version | Purpose | Used In |
|---------|---------|---------|---------|
| `modernc.org/sqlite` | v1.49.1 | Pure-Go SQLite driver (session persistence, history) | `cmd/cc-connect/provider.go` |

### Scheduling

| Library | Version | Purpose | Used In |
|---------|---------|---------|---------|
| `github.com/robfig/cron/v3` | v3.0.1 | Cron job scheduling | `core/cron.go` |

### QR Code Generation

| Library | Version | Purpose | Used In |
|---------|---------|---------|---------|
| `github.com/mdp/qrterminal/v3` | v3.2.1 | QR code rendering in terminal | `cmd/cc-connect/feishu.go` |
| `rsc.io/qr` | v0.2.0 | QR code encoding | `cmd/cc-connect/feishu.go` |

### Testing

| Library | Version | Purpose | Used In |
|---------|---------|---------|---------|
| `github.com/stretchr/testify` | v1.9.0 | Test assertions and mocks | `tests/`, `platform/*/`, `config/`, `agent/kimi/` |

## Frontend Stack

| Library | Version | Purpose |
|---------|---------|---------|
| `react` | ^19.1.0 | UI framework |
| `react-dom` | ^19.1.0 | React DOM renderer |
| `react-router-dom` | ^7.5.0 | Client-side routing |
| `zustand` | ^5.0.5 | State management |
| `react-markdown` | ^10.1.0 | Markdown rendering |
| `remark-gfm` | ^4.0.1 | GitHub Flavored Markdown support |
| `rehype-highlight` | ^7.0.2 | Code syntax highlighting in markdown |
| `highlight.js` | ^11.11.1 | Syntax highlighting engine |
| `i18next` | ^25.1.2 | Internationalization |
| `react-i18next` | ^15.5.1 | React bindings for i18next |
| `lucide-react` | ^0.487.0 | Icon library |
| `qrcode.react` | ^4.2.0 | QR code rendering |
| `clsx` | ^2.1.1 | Conditional CSS class utility |
| `@tailwindcss/typography` | ^0.5.19 | Tailwind typography plugin |

## Frontend Build Tools

| Tool | Version | Purpose |
|------|---------|---------|
| `vite` | ^6.3.2 | Build tool and dev server |
| `typescript` | ~5.8.3 | Type checking |
| `tailwindcss` | ^3.4.17 | Utility-first CSS framework |
| `postcss` | ^8.5.3 | CSS processing |
| `autoprefixer` | ^10.4.21 | CSS vendor prefixing |
| `@vitejs/plugin-react` | ^4.4.1 | Vite React plugin |

## Build & Distribution

- **Makefile** — multi-platform builds (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
- **Go build tags** — selective compilation via `//go:build !no_<platform>` tags for agents and platforms
- **npm package** — `npm/` directory contains a Node.js wrapper for distribution via npm registry
- **Go embed** — `embed.go` embeds the web dashboard into the Go binary
- Version injected via ldflags: `main.version`, `main.commit`, `main.buildTime`

## Scan Warnings

- WeChat Work (`platform/wecom/`) uses `gorilla/websocket` for streaming but has no dedicated SDK dependency in go.mod — it likely implements the WeCom API protocol directly over WebSocket
- QQ platform (`platform/qq/`) similarly uses raw WebSocket without a dedicated QQ SDK
- Weibo platform (`platform/weibo/`) uses raw WebSocket without a dedicated SDK
- The `npm/package.json` is a distribution wrapper only (not a Node.js application); the actual web frontend lives in `web/`
