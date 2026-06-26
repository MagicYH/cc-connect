# Management API Workspace Session Aggregation

## Problem

In multi-workspace mode, each workspace has its own `SessionManager` storing sessions independently. The Management API's `GET /api/v1/projects/{name}/sessions` only queries the engine's global `e.sessions`, so workspace sessions are invisible. This causes the WebUI cron form's session_key dropdown to miss sessions from bound workspaces.

Example: `feishu:oc_5a27bcb9ee8d86533ab62e276ba62999:ou_8d3dbd02714bb466bd4a5d270981741c` exists in `team-leader_ws_554233e8.json` (workspace `ea-ia-gym-java`) but not in the global session file, so the API never returns it.

## Solution

Aggregate workspace sessions into the existing `handleProjectSessions` response. No new endpoints. No frontend changes.

## Changes

### `core/management.go` — `handleProjectSessions`

After collecting sessions from `e.sessions`, if `e.workspacePool != nil`:

1. Call `e.workspacePool.All()` to get a snapshot of all workspace states
2. For each workspace state:
   - Hold `ws.mu` while reading `ws.sessions`; skip if nil
   - Snapshot each session's data under its own `s.mu` lock, then release
   - Build session info maps same as existing logic
   - Populate `user_name`/`chat_name` from `ws.sessions.GetUserMeta(sessionKey)`
   - Add `"workspace": ws.workspace` to each entry
   - Append to the result list
3. Global sessions get `"workspace": ""` (always present in JSON, even if empty, for schema consistency)

#### Concurrency safety

- `workspacePool.All()` returns a snapshot (releases pool RWMutex before we iterate)
- Hold `ws.mu` when checking `ws.sessions != nil`; skip the workspace if nil (benign TOCTOU: a workspace getting its first message while we iterate will simply not appear this time)
- Per-session data access follows existing pattern: `s.mu.Lock()` → read fields → `s.mu.Unlock()`
- No lock ordering concern: we never hold multiple workspace locks simultaneously (sequential iteration, lock per workspace, release before next)

#### `live` and `platform` fields for workspace sessions

The `activeKeys` map (from `e.interactiveStates`) is keyed by `interactiveKey`. For workspace sessions, the `interactiveKey` is `workspace + ":" + sessionKey`. To correctly resolve `live` and `platform`:

- After building the session info for a workspace session, construct the expected `interactiveKey` as `ws.workspace + ":" + sessionKey`
- Look up this key in `activeKeys` to set `live` and `platform`
- If not found, fall back to looking up `sessionKey` directly (covers edge cases where workspace prefix was not applied)

#### No deduplication

Session IDs (`s1`, `s2`, ...) are auto-incremented per-SessionManager and are NOT globally unique. Two different SessionManagers will produce colliding IDs. Therefore, do NOT deduplicate by ID — include all entries and rely on the `workspace` field to distinguish them. The frontend uses `session_key` (not session ID) for the cron dropdown, and session_keys are unique within a project.

### `core/management.go` — `handleProjectSessionDetail` and `handleProjectSessionSwitch`

After the list endpoint exposes workspace sessions, clicking one in the WebUI would hit the detail endpoint and get a 404. To avoid this broken journey:

- Both `handleProjectSessionDetail` and `handleProjectSessionSwitch` accept a query parameter `workspace` (matching the `workspace` field from the list response)
- If `workspace` is non-empty, look up the workspace via `e.workspacePool.Get(workspace)` and use its `SessionManager` instead of `e.sessions`
- If the workspace is not found or has no sessions, return 404

### `core/management.go` — `handleProjects` sessions_count

Update the sessions_count calculation to include workspace sessions:

```go
count := len(e.sessions.AllSessions())
if e.workspacePool != nil {
    for _, ws := range e.workspacePool.All() {
        ws.mu.Lock()
        if ws.sessions != nil {
            count += len(ws.sessions.AllSessions())
        }
        ws.mu.Unlock()
    }
}
```

### Output schema addition

Each session object gains a field (always present):

```json
{
  "workspace": "/path/to/workspace"  // "" for global sessions
}
```

### No changes to

- Frontend code — dropdown auto-populates from the enriched list; existing `session_key` values work unchanged for cron
- `workspacePool.All()` — already exists, returns `map[string]*workspaceState`

## Testing

- Unit test: mock `workspacePool` with 2 workspaces, verify `handleProjectSessions` returns sessions from all 3 sources (global + 2 workspaces)
- Unit test: verify `workspace` field is empty for global sessions and populated for workspace sessions
- Unit test: verify `live` and `platform` fields are correct for workspace sessions (using workspace-prefixed interactiveKey)
- Unit test: verify `user_name`/`chat_name` populated from workspace SessionManager's UserMeta
- Unit test: verify no deduplication — colliding session IDs from different managers both appear
- Unit test: verify `handleProjectSessionDetail` with `workspace` query param resolves workspace sessions
