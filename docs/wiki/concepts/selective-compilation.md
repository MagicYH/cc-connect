# Selective Compilation

The build-tag system that allows including or excluding specific agents and platforms at compile time.

## How It Works

- Each agent/platform has a `plugin_*.go` file in `cmd/cc-connect/`
- File naming: `plugin_agent_<name>.go` and `plugin_platform_<name>.go`
- Build tag: `//go:build !no_<name>` (negative opt-out)
- Content: blank import only — `import _ "github.com/chenhg5/cc-connect/agent/claudecode"`
- Build with `make build AGENTS=... PLATFORMS_INCLUDE=...` or `EXCLUDE=...`

## Project Relation

Controls which agents and platforms are compiled into the binary. Without the blank import, the `init()` registration never runs and the engine cannot create that agent/platform.

## Cross-References

- [plugin-registry](./plugin-registry.md) — the registration mechanism enabled by compilation
- [dependency-direction](./dependency-direction.md) — how blank imports maintain the dependency rule
