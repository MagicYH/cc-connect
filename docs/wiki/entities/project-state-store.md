# ProjectStateStore

Per-project state (active workspace, etc.). Persists to `<data_dir>/projects/<project>/state.json`.

**Implementation:** `cmd/cc-connect/main.go:325` (`NewProjectStateStore`)

Cross-references: [SessionManager](session-manager.md), [DirHistory](dir-history.md)
