# SiliconFlow / OpenAI-Compatible Proxy

Third-party OpenAI-compatible API providers (e.g., SiliconFlow) used as proxies for agent CLIs.

## Project Relation

Enables Claude Code to route through a third-party proxy instead of direct Anthropic API access. Configured via provider settings with `thinking="disabled"`.

Config: `[projects.agent.providers] thinking="disabled"` with custom base URL.

## Cross-References

- [Claude Code Router](./claude-code-router.md)
- [External Dependencies Source](../sources/external-dependencies.md)
