# Claude Code Router

Local HTTP router at `127.0.0.1:3456` for model provider selection.

## Project Relation

Configured via `router_url` in agent settings. Enables the Claude Code CLI to route requests through a local proxy that selects among multiple AI model providers dynamically.

Config: `router_url` in `[projects.agent]` section.

## Cross-References

- [SiliconFlow](./siliconflow.md)
- [External Dependencies Source](../sources/external-dependencies.md)
