# PTY (Pseudo-Terminal)

A pseudo-terminal is a pair of virtual devices providing bidirectional IPC that emulates a real terminal. cc-connect uses `creack/pty` (v1.1.24) to control agent subprocesses.

PTY is used by the iflow agent (`agent/iflow/session.go`) and for Claude Code usage tracking (`agent/claudecode/claude_usage.go`). It allows the Go process to interact with CLI-based agent processes as if they were connected to a real terminal.

Cross-references: [tech-stack](../sources/tech-stack.md)
