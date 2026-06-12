# Agent CLI

External CLI binaries spawned as child processes to run AI coding agents.

## Project Relation

12 agent CLIs (`claude`, `codex`, `agent`, `gemini`, `iflow`, `kimi`, `opencode`, `qodercli`, `devin`, `agy`, `copilot`, `openclaw`) plus `tmux` are spawned via `creack/pty` with full PTY support. Each CLI manages its own outbound API calls independently; cc-connect does not call AI provider APIs on their behalf.

Config: `[projects.agent] type="<agent_type>"`.

## Cross-References

- [pty](../entities/pty.md)
- [Anthropic API](../entities/anthropic-api.md)
- [External Dependencies Source](../sources/external-dependencies.md)
