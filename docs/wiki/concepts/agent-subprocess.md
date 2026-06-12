# Agent Subprocess Management

AI agent CLIs (claude, codex, gemini, cursor, etc.) are spawned as OS processes via `os/exec`. Each agent package (`agent/*/`) manages its CLI lifecycle: context-based cancellation, PTY allocation (`creack/pty`), stdout/stderr streaming, and permission prompt interception.

Cross-references: [SessionManager](../entities/session-manager.md)
