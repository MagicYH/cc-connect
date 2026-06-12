# Process Group Kill

Killing the agent process group on session close. Attempted in `agent/claudecode/session.go` and `agent/codex/session.go` via process group signal.

Risk: grandchild processes that detach from the process group (e.g., daemon started by an MCP server) survive the group kill. The 130-second close timeout may abandon the process entirely, leaving orphaned subprocesses.

Project relation: agent close paths, `core/engine.go` `closeAgentSessionWithTimeout`.

Cross-references: [session-manager](../entities/session-manager.md)
