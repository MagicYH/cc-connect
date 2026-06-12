# File-Based Persistence

cc-connect uses JSON files for all internal state -- no traditional database (MySQL, PostgreSQL, Redis) or message queue. Stores include sessions, crons, timers, relay bindings, dir history, heartbeat state, and project state. Data directory defaults to `~/.cc-connect`, configurable via `data_dir`.

**See also:** [CronStore](../entities/cron-store.md), [TimerStore](../entities/timer-store.md), [SessionManager](../entities/session-manager.md)
