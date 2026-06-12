# Plugin Registry Pattern

The self-registration pattern where agents and platforms register themselves in `init()` so the engine can instantiate them by config name.

## How It Works

- Agent registration: `core.RegisterAgent("claudecode", New)` in `agent/claudecode/claudecode.go`
- Platform registration: `core.RegisterPlatform("feishu", New)` in `platform/feishu/feishu.go`
- Factory signature: `func New(opts map[string]any) (core.Agent, error)` or `(core.Platform, error)`
- The engine creates instances via `core.CreateAgent(name, opts)` / `core.CreatePlatform(name, opts)`
- Some packages register multiple names (e.g. feishu registers both "feishu" and "lark")

## Project Relation

This is the mechanism that decouples `core/` from concrete agent/platform implementations. Core never imports agent/platform packages directly; they are pulled in via blank imports in `plugin_*.go` files.

## Cross-References

- [selective-compilation](./selective-compilation.md) — how plugin files are conditionally included
- [dependency-direction](./dependency-direction.md) — the import rule this pattern enables
