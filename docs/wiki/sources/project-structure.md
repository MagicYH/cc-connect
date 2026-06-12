# Project Structure

[Raw scan](../raw/project-structure.md)

## Architecture

CC-Connect is a Go 1.25 bridge connecting AI coding agents to messaging platforms via a plugin architecture. A central `core/Engine` orchestrates all message flow; agents and platforms self-register at init time through factory functions. Strict dependency rules enforce that `core/` never imports `agent/` or `platform/`, keeping the nucleus decoupled.

- **Module**: `github.com/chenhg5/cc-connect` (v1.3.3-beta.4)
- **Non-test Go**: 204 files, ~99,619 lines
- **Test Go**: 172 files, ~75,573 lines (ratio 0.76)
- **Web frontend**: React 19 + TypeScript + Vite + TailwindCSS in `web/`

## Dependency Direction

```
cmd/cc-connect  ->  config/, core/, agent/*, platform/*
agent/*         ->  core/    (never other agents or platforms)
platform/*      ->  core/    (never other platforms or agents)
core/           ->  stdlib + gorilla/websocket + robfig/cron/v3 only
```

## Module Breakdown

| Module | Files | Lines | Role |
|--------|-------|-------|------|
| `cmd/cc-connect/` | 60 | ~10,601 | CLI entry point, cobra commands, daemon, plugin wiring |
| `core/` | 93 | ~68,168 | Engine, interfaces (51), registry, i18n, sessions, cards |
| `agent/` | 14 pkgs | ~21,679 | 14 AI agent adapters (codex, claudecode, acp, copilot, etc.) |
| `platform/` | 13 pkgs | ~23,913 | 13 messaging platform adapters (feishu, wecom, dingtalk, etc.) |
| `config/` | 2 | ~6,961 | TOML config parsing |
| `daemon/` | 14 | ~2,794 | systemd/launchd/Windows service management |
| `tests/` | 6 dirs | ~10,807 | integration, blackbox, e2e, mocks, performance |
| `web/` | ~15 | — | React dashboard frontend |

## Key Files

- `core/engine.go` (15,578 lines) — central orchestrator: routing, sessions, permissions, streaming, workspaces, commands, hooks
- `core/interfaces.go` (663 lines) — 51 interfaces: Platform, Agent, AgentSession + optional capability interfaces
- `core/registry.go` — factory registry (RegisterAgent/RegisterPlatform, CreateAgent/CreatePlatform)
- `core/bridge.go` (1,420 lines) — platform-agent capability matching, event forwarding
- `core/i18n.go` (4,221 lines) — 5-language i18n (EN, ZH, ZH-TW, JA, ES)
- `cmd/cc-connect/main.go` (1,908 lines) — CLI root, engine init
- `config/config.go` (3,646 lines) — all config structs and TOML parsing
- `platform/feishu/feishu.go` (6,099 lines) — largest platform adapter

## Agent Adapters (14)

All follow: `New()` constructor, `init()` self-registration, implement `core.Agent` + `core.AgentSession`. Largest: codex (4,301), claudecode (3,631), acp (2,046), copilot (1,762). Smallest: devin (93, stub).

## Platform Adapters (13)

All follow: `New()` constructor, `init()` self-registration, implement `core.Platform` + optional capability interfaces. Largest: feishu (6,714), wecom (2,352), dingtalk (2,110). Feishu registers as both `"feishu"` and `"lark"`.

## Selective Compilation

Build tags (`//go:build !no_X`) on `plugin_*.go` files enable excluding agents/platforms at compile time: `make build AGENTS=claudecode PLATFORMS_INCLUDE=feishu,telegram`.

## Warnings

- `core/engine.go` at 15,578 lines far exceeds the 800-line guideline; most actively changed file.
- Several other files exceed 800 lines: `feishu.go` (6,099), `main.go` (1,908), `config.go` (3,646).
- `config.example.toml` at ~100K lines may contain generated content.
- `agent/devin/` at 93 lines appears to be a minimal stub.
