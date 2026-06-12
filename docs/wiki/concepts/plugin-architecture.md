# Plugin Architecture

cc-connect uses a plugin-style registry pattern where agents and platforms self-register at init time via `core.RegisterAgent()` and `core.RegisterPlatform()`.

Each agent/platform is compiled in via a separate `plugin_*.go` file with a build tag (e.g., `//go:build !no_feishu`). The engine creates instances by string name from config using `core.CreateAgent()` / `core.CreatePlatform()`.

Selective compilation allows building with only needed agents/platforms via `make build AGENTS=... PLATFORMS_INCLUDE=...` or build tags like `no_discord`.

Cross-references: [tech-stack](../sources/tech-stack.md), [messaging-platform](./messaging-platform.md), [project-structure](../sources/project-structure.md)

From [project-structure](../sources/project-structure.md): 14 agent adapters and 13 platform adapters follow this pattern. Each is self-contained in its own package under `agent/` or `platform/`.
