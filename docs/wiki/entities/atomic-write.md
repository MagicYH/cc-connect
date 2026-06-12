# Atomic Write

File write pattern in `core/atomicwrite.go`. `AtomicWriteFile()` writes to temp file in same directory, syncs, then renames over target. Cleanups on failure. Used by session persistence, cron store, timer store, relay bindings, and directory history to prevent corrupt state on crash.

Cross-references: [instance-lock](instance-lock.md)
