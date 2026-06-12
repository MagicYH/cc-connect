# Workspace Binding

Maps chat channels to agent workspace directories. Lives in `core/workspace_binding.go`.

Risks: `refreshLocked()` re-reads from disk on mtime/size change — TOCTOU gap between lookup and use when multiple instances share a bindings file. Auto-binding by convention creates implicit routing if a directory name matches a channel name.

Cross-references: [toctou](../concepts/toctou.md)
