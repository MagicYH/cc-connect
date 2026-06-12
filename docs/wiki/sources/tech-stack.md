# Technology Stack

[Raw scan](../raw/tech-stack.md)

## Language & Runtime

- **Go 1.25.0** — primary backend language (`go.mod`, module `github.com/chenhg5/cc-connect`)
- **TypeScript ~5.8.3** — web dashboard frontend (`web/package.json`)

## Backend Libraries

### Configuration

| Library | Version | Purpose |
|---------|---------|---------|
| `BurntSushi/toml` | v1.6.0 | TOML config file parsing |

### Messaging Platform SDKs

| Library | Version | Platform |
|--------|---------|----------|
| `larksuite/oapi-sdk-go/v3` | v3.5.3 | Feishu/Lark |
| `go-telegram/bot` | v1.20.0 | Telegram |
| `bwmarrin/discordgo` | v0.29.0 | Discord |
| `slack-go/slack` | v0.16.0 | Slack |
| `open-dingtalk/dingtalk-stream-sdk-go` | v0.9.1 | DingTalk |
| `line/line-bot-sdk-go/v8` | v8.19.0 | LINE |

WeChat Work, QQ, and Weibo use raw WebSocket connections without dedicated SDKs.

### Networking

| Library | Version | Purpose |
|--------|---------|---------|
| `gorilla/websocket` | v1.5.0 | WebSocket client/server (wecom, weibo, qq, wps-xiezuo, core/bridge) |

### Terminal & Process

| Library | Version | Purpose |
|--------|---------|---------|
| `creack/pty` | v1.1.24 | PTY for agent subprocess control |
| `charmbracelet/bubbletea` | v1.3.10 | TUI framework (sessions overview) |
| `charmbracelet/bubbles` | v1.0.0 | TUI components |
| `charmbracelet/lipgloss` | v1.1.0 | Terminal styling/layout |

### Data & Scheduling

| Library | Version | Purpose |
|--------|---------|---------|
| `modernc.org/sqlite` | v1.49.1 | Pure-Go SQLite (session persistence, history) |
| `robfig/cron/v3` | v3.0.1 | Cron job scheduling |

### Testing

| Library | Version | Purpose |
|--------|---------|---------|
| `stretchr/testify` | v1.9.0 | Assertions and mocks |

## Frontend Stack

| Library | Version | Purpose |
|--------|---------|---------|
| `react` | ^19.1.0 | UI framework |
| `react-router-dom` | ^7.5.0 | Client-side routing |
| `zustand` | ^5.0.5 | State management |
| `react-markdown` + `remark-gfm` + `rehype-highlight` | ^10.1/^4.0/^7.0 | Markdown rendering with GFM and syntax highlighting |
| `i18next` + `react-i18next` | ^25.1/^15.5 | Internationalization |
| `lucide-react` | ^0.487.0 | Icon library |
| `tailwindcss` | ^3.4.17 | Utility-first CSS |

## Frontend Build Tools

| Tool | Version | Purpose |
|------|---------|---------|
| `vite` | ^6.3.2 | Build tool and dev server |
| `typescript` | ~5.8.3 | Type checking |
| `postcss` + `autoprefixer` | ^8.5/^10.4 | CSS processing |

## Build & Distribution

- **Makefile** — multi-platform builds (linux/darwin/arm64/amd64, windows/amd64)
- **Go build tags** — selective compilation via `//go:build !no_<platform>` for agents/platforms
- **npm package** — Node.js wrapper in `npm/` for registry distribution
- **Go embed** — web dashboard embedded into Go binary via `embed.go`
- Version info injected via ldflags (`main.version`, `main.commit`, `main.buildTime`)
