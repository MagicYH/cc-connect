# SQLite

SQLite (via `modernc.org/sqlite` v1.49.1) provides the embedded data store for cc-connect.

It is a pure-Go implementation (no CGO required), used for session persistence and message history in `cmd/cc-connect/provider.go`. The pure-Go driver simplifies cross-compilation and static linking.

Cross-references: [tech-stack](../sources/tech-stack.md), [External Dependencies Source](../sources/external-dependencies.md)

The `sqlite3` OS binary is also used to read Cursor and OpenCode session metadata from their local SQLite databases (`agent/cursor/cursor.go`, `agent/opencode/opencode.go`). Auto-detected on PATH.
