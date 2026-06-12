# Instance Lock

File-based exclusive lock in `cmd/cc-connect/instance_lock.go`. `InstanceLock` uses `syscall.Flock` with `LOCK_EX|LOCK_NB` on per-config-path lock file. Writes PID for diagnostics. `Release()` truncates, unlocks, closes. `KillExistingInstance()` reads PID and sends `SIGKILL`. Separate Windows implementation at `instance_lock_windows.go`.

Cross-references: [atomic-write](atomic-write.md), [graceful-shutdown](../concepts/graceful-shutdown.md)
