# Project Structure

## Project Overview

CC-Connect is a Go bridge connecting AI coding agents (Claude Code, Codex, Gemini CLI, Cursor, etc.) with messaging platforms (Feishu/Lark, Telegram, Discord, Slack, DingTalk, WeChat Work, QQ, LINE). Users interact with their coding agent through their preferred messaging app.

- **Language**: Go 1.25
- **Module**: `github.com/chenhg5/cc-connect`
- **Version**: v1.3.3-beta.4
- **Entry point**: `cmd/cc-connect/main.go` (1908 lines)
- **Total non-test Go**: 204 files, ~99,619 lines
- **Total test Go**: 172 files, ~75,573 lines
- **Test-to-code ratio**: 0.76 (high test coverage)
- **Web frontend**: React 19 + TypeScript + Vite + TailwindCSS, located in `web/`

## Project Architecture

Plugin architecture with a central core orchestrator (`core/Engine`). Agents and platforms register themselves via `core.RegisterAgent()` / `core.RegisterPlatform()` in `init()` functions. The engine creates instances via string names from config using factory functions.

**Dependency direction** (strict, enforced by convention):
```
cmd/cc-connect  →  config/, core/, agent/*, platform/*
agent/*         →  core/       (never other agents or platforms)
platform/*      →  core/       (never other platforms or agents)
core/           →  stdlib + 2 external deps only (gorilla/websocket, robfig/cron/v3)
                 (never imports agent/ or platform/)
```

**Core (`core/`)** defines all interfaces and contains the `Engine` that orchestrates message flow. It has 51 interfaces (10 core + 41 optional capability interfaces), enabling plugins to opt-in to features without coupling.

**Selective compilation** via build tags: each agent/platform has a `plugin_*.go` file with `//go:build !no_X` tags. Build with `make build AGENTS=claudecode PLATFORMS_INCLUDE=feishu,telegram` to include only specific plugins.

## Module Overview

### cmd/cc-connect (CLI Entry Point)
- `main.go` — CLI flags, cobra commands, engine initialization, daemon management
- `cron.go` — Cron job CLI subcommands (add/edit/list/exec)
- `daemon.go` — Daemon mode (systemd/launchd service management)
- `doctor_runas.go` — Diagnostic/doctor subcommand
- `feishu.go`, `weixin.go` — Platform-specific CLI setup helpers
- `provider.go` — Provider/model configuration CLI
- `send.go` — Agent-driven send command (TTS, file)
- `sessions.go`, `sessions_tui.go` — Session management with TUI (bubbletea)
- `timer.go` — One-shot delayed task system
- `update.go` — Self-update mechanism
- `web.go` — Web dashboard server
- `plugin_agent_*.go` / `plugin_platform_*.go` — Build-tagged plugin imports (14 agents, 13 platforms)
- **Size**: ~10,601 non-test lines across 60 files

### core/ (Orchestration Nucleus)
- `engine.go` — Central Engine struct (15,578 lines, largest file): message routing, session lifecycle, permission handling, streaming, workspace management
- `interfaces.go` — 51 interfaces: Platform, Agent, AgentSession + optional capability interfaces (CardSender, InlineButtonSender, ProviderSwitcher, UsageReporter, ContextCompressor, etc.)
- `registry.go` — Agent/Platform factory registry (RegisterAgent, CreateAgent, RegisterPlatform, CreatePlatform)
- `bridge.go` — Platform-Agent bridge: capability matching, event forwarding
- `session.go` — Session management (create, resume, prune, stop)
- `i18n.go` — Internationalization (EN, ZH, ZH-TW, JA, ES) with MsgKey constants
- `message.go` — Message types, dedup, watermark tracking
- `cron.go` — Cron job scheduling and execution
- `timer.go` — One-shot delayed task system
- `card.go` — Rich card message model
- `command.go` — Slash command routing (/help, /cancel, /alias, /model, etc.)
- `hooks.go` — Hook system for extensible event processing
- `management.go` — Management API (list sessions, providers, etc.)
- `api.go` — HTTP API for web dashboard
- `provider.go`, `providerproxy.go` — Provider/model management and proxy
- `tts.go`, `speech.go` — Text-to-speech support
- `streaming.go` — Streaming output handling
- `ratelimit.go`, `outgoing_ratelimit.go` — Rate limiting
- `markdown.go`, `markdown_html.go`, `markdown_slack.go` — Markdown rendering for different platforms
- `webhook.go` — Webhook support
- `workspace_binding.go`, `workspace_state.go` — Multi-workspace support
- `user_roles.go` — Admin/user role management
- `runas.go`, `runas_check.go`, `runas_audit.go` — Run-as permission system
- `observer.go` — Event observer pattern
- `skill.go`, `skill_presets.go` — Skill discovery and presets
- `updater.go` — Self-update logic
- **External dependencies**: Only `github.com/gorilla/websocket` and `github.com/robfig/cron/v3`
- **Size**: ~68,168 total lines (including tests), 93 files

### agent/ (AI Agent Adapters) — 14 packages
Each package is self-contained: handles process lifecycle, output parsing, session management.

| Agent | Non-test Lines | Key Files |
|-------|---------------|-----------|
| codex | 4,301 | `codex.go`, `session.go` |
| claudecode | 3,631 | `claudecode.go` (1,464), `session.go` (1,024), `cc_hooks.go` (347), `claude_usage.go` (663) |
| acp | 2,046 | `agent.go` — OpenClaw/ACP protocol adapter |
| copilot | 1,762 | `copilot.go`, `session.go`, `jsonrpc.go` |
| iflow | 1,385 | `iflow.go`, `session.go` |
| gemini | 1,302 | `gemini.go`, `session.go` |
| cursor | 1,301 | `cursor.go`, `session.go` |
| opencode | 1,271 | `opencode.go`, `session.go` |
| pi | 1,176 | `pi.go`, `session.go` |
| antigravity | 1,041 | `antigravity.go`, `session.go` |
| kimi | 937 | `kimi.go`, `session.go` |
| tmux | 764 | `tmux.go`, `session.go` |
| qoder | 577 | `qoder.go` |
| devin | 93 | `devin.go` |

All agents follow the same pattern:
1. `func New(opts map[string]any) (core.Agent, error)` — constructor
2. `func init() { core.RegisterAgent("name", New) }` — self-registration
3. Implement `core.Agent` interface (Name, StartSession, ListSessions, Stop)
4. Session type implements `core.AgentSession` (Send, RespondPermission, Events)
5. Optional interfaces: `UsageReporter`, `ContextUsageReporter`, `ProviderSwitcher`, `AgentDoctorInfo`, etc.

### platform/ (Messaging Platform Adapters) — 13 packages
Each package is self-contained: handles API connection, message receiving/sending, card rendering.

| Platform | Non-test Lines | Key Files |
|----------|---------------|-----------|
| feishu | 6,714 | `feishu.go` (6,099 — largest single file), `card.go` (464), `ws_shared.go` (84) |
| wecom | 2,352 | `wecom.go`, `websocket_outbound_media.go`, `message_split.go` |
| dingtalk | 2,110 | `dingtalk.go` |
| weixin | 2,028 | `weixin.go` |
| telegram | 1,857 | `telegram.go` (1,774), `telegram_reply.go` (66) |
| discord | 1,854 | `discord.go` |
| qqbot | 1,792 | `qqbot.go` |
| max | 1,538 | `max.go` |
| wps-xiezuo | 1,063 | `wpsxiezuo.go` |
| qq | 800 | `qq.go` |
| slack | 764 | `slack.go` |
| weibo | 679 | `weibo.go` |
| line | 322 | `line.go` |

All platforms follow the same pattern:
1. `func New(opts map[string]any) (core.Platform, error)` — constructor
2. `func init() { core.RegisterPlatform("name", New) }` — self-registration
3. Implement `core.Platform` interface (Name, Start, Reply, Send, Stop)
4. Optional interfaces: `CardSender`, `InlineButtonSender`, `ImageSender`, `FileSender`, `MessageUpdater`, `StatusFooterSender`, etc.
5. Note: Feishu registers two names — `"feishu"` and `"lark"`

### config/ (Configuration)
- `config.go` (3,646 lines) — TOML config parsing, all config structs
- `config_test.go` (3,315 lines)
- **Size**: 6,961 lines total

### daemon/ (Service Management)
- `manager.go` — Daemon lifecycle management
- `systemd.go` — Linux systemd service
- `launchd.go` — macOS launchd service
- `windows.go` — Windows service
- `logrotate.go`, `logsize.go` — Log rotation and size management
- `env_extension.go` — Environment variable discovery for service files
- **Size**: 2,794 lines total

### tests/ (Test Suites)
| Directory | Lines | Purpose |
|-----------|-------|---------|
| integration | 3,231 | Integration tests |
| blackbox | 2,410 | Black-box tests (p0, p1, p2 tiers + platform-specific) |
| release_local | 2,053 | Deterministic release checks without real IM credentials |
| e2e | 1,742 | End-to-end tests (smoke + regression build tags) |
| mocks | 1,051 | Test mocks (fake Platform, Agent) |
| performance | 323 | Performance benchmarks |

### web/ (Dashboard Frontend)
- React 19 + TypeScript + Vite + TailwindCSS + Zustand
- Pages: Dashboard, Login, Projects, Providers, Sessions, Skills, System, Cron, Bridge, Chat
- i18n support via react-i18next
- **Size**: ~15 files in `web/src/`

### embed.go
- Embeds `config.example.toml` into the binary for reference.

## Directory Layout

```
cc-connect/
├── cmd/cc-connect/          # CLI entry point + plugin wiring (60 files)
│   ├── main.go              # Root cobra command, engine init (1,908 lines)
│   ├── plugin_agent_*.go    # 14 build-tagged agent imports
│   ├── plugin_platform_*.go # 13 build-tagged platform imports
│   └── *.go                 # Subcommands (cron, daemon, send, sessions, etc.)
├── core/                    # Orchestration nucleus (93 files)
│   ├── engine.go            # Central Engine (15,578 lines)
│   ├── interfaces.go        # 51 interfaces
│   ├── registry.go          # Factory registry
│   └── *.go                 # Cross-cutting concerns (i18n, cards, sessions, etc.)
├── agent/                   # AI agent adapters (14 packages)
│   ├── claudecode/          # Claude Code (3,631 lines)
│   ├── codex/               # OpenAI Codex (4,301 lines)
│   ├── cursor/              # Cursor (1,301 lines)
│   ├── gemini/              # Gemini CLI (1,302 lines)
│   ├── acp/                 # OpenClaw/ACP (2,046 lines)
│   ├── copilot/             # GitHub Copilot (1,762 lines)
│   ├── iflow/               # iFlow (1,385 lines)
│   ├── opencode/            # OpenCode (1,271 lines)
│   ├── pi/                  # Pi (1,176 lines)
│   ├── antigravity/         # Antigravity (1,041 lines)
│   ├── kimi/                # Kimi (937 lines)
│   ├── tmux/                # tmux (764 lines)
│   ├── qoder/               # Qoder (577 lines)
│   └── devin/               # Devin (93 lines)
├── platform/                # Messaging platform adapters (13 packages)
│   ├── feishu/              # Feishu/Lark (6,714 lines)
│   ├── wecom/               # WeChat Work (2,352 lines)
│   ├── dingtalk/            # DingTalk (2,110 lines)
│   ├── weixin/              # WeChat (2,028 lines)
│   ├── telegram/            # Telegram (1,857 lines)
│   ├── discord/             # Discord (1,854 lines)
│   ├── qqbot/               # QQ Bot (1,792 lines)
│   ├── max/                 # Max (1,538 lines)
│   ├── wps-xiezuo/          # WPS Collaboration (1,063 lines)
│   ├── qq/                  # QQ (800 lines)
│   ├── slack/               # Slack (764 lines)
│   ├── weibo/               # Weibo (679 lines)
│   └── line/                # LINE (322 lines)
├── config/                  # TOML config parsing (2 files, ~7K lines)
├── daemon/                  # Service management (14 files, ~2.8K lines)
├── tests/                   # Test suites (~10.8K lines)
│   ├── integration/
│   ├── blackbox/
│   ├── release_local/
│   ├── e2e/
│   ├── mocks/
│   └── performance/
├── web/                     # React dashboard frontend
│   ├── src/                 # Pages: Dashboard, Login, Projects, etc.
│   └── package.json
├── embed.go                 # Embeds config.example.toml
├── Makefile                 # Build system with selective compilation
├── go.mod / go.sum          # Go module definition
├── AGENTS.md                # Agent documentation
├── CLAUDE.md                # Development guide
└── config.example.toml      # Full config reference (100K lines)
```

## Dependency Rules

1. **core/ must never import agent/ or platform/** — Verified: core only imports stdlib + `gorilla/websocket` + `robfig/cron/v3`
2. **agent/* must only import core/** — Agents import `github.com/chenhg5/cc-connect/core` and stdlib; they never import other agents or platforms
3. **platform/* must only import core/** — Platforms import `github.com/chenhg5/cc-connect/core` and their respective SDK; they never import other platforms or agents
4. **cmd/cc-connect imports everything** — The entry point wires all plugins together via build-tagged import files
5. **Plugin self-registration via init()** — Each agent/platform registers itself in `init()` using `core.RegisterAgent()`/`core.RegisterPlatform()`, and the engine creates instances via string names from config

## Internal Key Components

### Engine (core/engine.go — 15,578 lines)
The central orchestrator. Handles:
- Message routing between platforms and agents
- Session lifecycle (create, resume, prune, cancel)
- Permission request/response flow
- Streaming output with throttling
- Multi-workspace binding and state
- Slash command routing
- Hook system for extensible event processing
- Provider/model management and switching
- TTS and file send
- Rate limiting (incoming and outgoing)
- Cron job execution delegation
- Timer (one-shot delayed tasks)

### Interface Hierarchy (core/interfaces.go — 663 lines, 51 interfaces)

**Core interfaces** (must implement):
- `Platform` — messaging adapter (Name, Start, Reply, Send, Stop)
- `Agent` — AI agent adapter (Name, StartSession, ListSessions, Stop)
- `AgentSession` — bidirectional session (Send, RespondPermission, Events)

**Platform capability interfaces** (optional):
- `CardSender`, `CardNavigable`, `CardRefresher` — rich card messages
- `InlineButtonSender` — inline keyboard buttons
- `ImageSender`, `FileSender` — media support
- `MessageUpdater` — edit existing messages
- `StatusFooterSender`, `StatusFooterUpdater` — status footers
- `ProgressStyleProvider`, `ProgressCardPayloadSupport`, `ProgressUpdateThrottler` — progress indicators
- `StreamingCard`, `StreamingCardPlatform` — streaming card support
- `AtMentionSender` — @-mention support
- `TypingIndicator`, `TypingIndicatorDone` — typing indicators
- `ChannelNameResolver` — channel name resolution
- `ReplyContextReconstructor` — cron job reply reconstruction
- `MessageRecallDetector` — deleted message detection
- `CronReplyTargetResolver` — cron reply target resolution
- `PlatformLifecycleHandler` — lifecycle events
- `AsyncRecoverablePlatform` — async error recovery
- `PlatformPromptInjector` — platform-specific prompt injection
- `PreviewStatusUpdater` — preview status updates

**Agent capability interfaces** (optional):
- `ProviderSwitcher`, `ModelSwitcher`, `ReasoningEffortSwitcher` — model switching
- `UsageReporter`, `ContextUsageReporter`, `ContextCompressor` — usage/compression
- `ToolAuthorizer` — tool permission handling
- `HistoryProvider` — session history
- `MemoryFileProvider` — memory file management
- `CommandProvider`, `CommandRegistrar` — command registration
- `SkillProvider` — skill discovery
- `SessionDeleter`, `SessionTitleProvider` — session management
- `WorkDirSwitcher` — working directory switching
- `AgentOptsProvider` — agent options
- `ModeSwitcher`, `LiveModeSwitcher` — mode switching
- `WorkspaceAgentOptionSnapshotter` — workspace option snapshots
- `StartupWarner` — startup warnings
- `SessionIDValidator` — session ID validation
- `SessionEnvInjector` — per-session environment variables
- `SystemPromptSupporter` — custom system prompt
- `FormattingInstructionProvider` — formatting instructions
- `AgentDoctorInfo` — diagnostic info (CLI binary name, display name)

### Registry (core/registry.go)
Factory pattern with `map[string]PlatformFactory` and `map[string]AgentFactory`. Functions: `RegisterPlatform`, `RegisterAgent`, `CreatePlatform`, `CreateAgent`, `ListRegisteredPlatforms`, `ListRegisteredAgents`.

### Bridge (core/bridge.go — 1,420 lines)
Maps platform capabilities to agent expectations. Handles capability matching, event forwarding between platform and agent sessions, and streaming output adaptation.

### i18n System (core/i18n.go — 4,221 lines)
All user-facing strings use `MsgKey` constants with translations in 5 languages (EN, ZH, ZH-TW, JA, ES). Usage: `e.i18n.T(MsgKey)` or `e.i18n.Tf(MsgKey, args...)`.

## Scan Warnings

- `core/engine.go` at 15,578 lines is extremely large for a single Go file, far exceeding the 800-line guideline in CLAUDE.md. This is the most actively changed file (13 changes in last 50 commits).
- `config.example.toml` at ~100K lines is unusually large for a config example file; may contain inline documentation or generated content.
- `platform/feishu/feishu.go` at 6,099 lines exceeds the 800-line file guideline.
- `cmd/cc-connect/main.go` at 1,908 lines exceeds the 800-line file guideline.
- `config/config.go` at 3,646 lines exceeds the 800-line file guideline.
- The dependency rule "core/ imports stdlib only" has two exceptions: `gorilla/websocket` and `robfig/cron/v3`, which are external packages but serve core infrastructure needs.
- `agent/devin/` at only 93 lines is a minimal stub — may be incomplete or early-stage.
