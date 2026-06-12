# Daemon Service Management

cc-connect runs as a platform-native background service. **Linux:** user-level systemd unit with log rotation, env capture, and linger check. **macOS:** LaunchAgent plist (`com.cc-connect.service`) with auto-restart. **Windows:** Windows Service. All managed via `cc-connect daemon` subcommands.

**Implementation:** `daemon/systemd.go`, `daemon/launchd.go`, `daemon/windows.go`
