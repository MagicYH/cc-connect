# Factory Pattern

The constructor pattern used by all agent and platform adapters in CC-Connect.

## What It Does

Each agent/platform exposes a `New(opts map[string]any) (core.Agent, error)` or `(core.Platform, error)` factory function. The factory is registered with `core.RegisterAgent()`/`core.RegisterPlatform()` in `init()`. The Engine creates instances via `core.CreateAgent(name, opts)`/`core.CreatePlatform(name, opts)` using the string name from config.

## Project Relation

The `map[string]any` options parameter keeps factories flexible — new config fields can be added without changing the factory signature. `core/registry.go` holds the factory maps: `map[string]PlatformFactory` and `map[string]AgentFactory`.

## Cross-References

- [plugin-registry](./plugin-registry.md) — the registration mechanism
- [plugin-architecture](./plugin-architecture.md) — overall plugin system
- [project-structure](../sources/project-structure.md)
