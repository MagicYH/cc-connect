# Management API Workspace Session Aggregation Verification

> Source spec: ./2026-06-26-mgmt-workspace-sessions-design.md
> Used by: superpowers:writing-plans (TDD coverage) and post-implementation smoke testing.

## Environment & Access

| Item | Value | How to Obtain |
|---|---|---|
| Target environment | Local cc-connect instance, http://localhost:9820 | — |
| Management API base URL | http://localhost:9820/api/v1 | — |
| Auth token | `${MGMT_TOKEN}` | `grep 'token' ~/.cc-connect/config.toml \| cut -d'"' -f2` |
| Session files directory | `~/.cc-connect/sessions/` | `ls ~/.cc-connect/sessions/` |
| Workspace bindings file | `~/.cc-connect/workspace_bindings.json` | `cat ~/.cc-connect/workspace_bindings.json` |
| Test project | team-leader | Has both global and workspace sessions |
| Workspace session file | `team-leader_ws_554233e8.json` | Workspace ea-ia-gym-java under team-leader |
| Required env vars | `${MGMT_TOKEN}`: management API bearer token | Extract from config.toml per row 2 |

## Public Operations

### Authenticate API Request

Purpose: Set up authorization header for all API calls in scenarios.

```bash
export MGMT_TOKEN=$(grep 'token' ~/.cc-connect/config.toml | head -1 | cut -d'"' -f2)
```

### Rebuild and Restart cc-connect

Purpose: Rebuild the binary after code changes and restart the daemon so changes take effect.

```bash
make build && cc-connect daemon restart
```

## Acceptance Criteria

- [ ] AC-1: `GET /api/v1/projects/{name}/sessions` returns sessions from both global and workspace SessionManagers (covers spec §handleProjectSessions)
- [ ] AC-2: Each session object includes a `workspace` field — empty string for global sessions, workspace path for workspace sessions (covers spec §Output schema addition)
- [ ] AC-3: `live` and `platform` fields are correctly resolved for workspace sessions using the `workspace:sessionKey` interactiveKey pattern (covers spec §live and platform fields)
- [ ] AC-4: `user_name` and `chat_name` are populated from workspace SessionManager's UserMeta when available (covers spec §handleProjectSessions)
- [ ] AC-5: `GET /api/v1/projects/{name}/sessions/{id}?workspace=<path>` returns workspace session detail without 404 (covers spec §handleProjectSessionDetail)
- [ ] AC-6: `POST /api/v1/projects/{name}/sessions/switch` with `workspace` body field switches workspace session successfully (covers spec §handleProjectSessionSwitch)
- [ ] AC-7: `GET /api/v1/projects` sessions_count includes workspace session counts (covers spec §handleProjects sessions_count)

## Test Scenarios

### Scenario S1: Workspace sessions appear in list endpoint

**Verifies:** AC-1, AC-2

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect is running with team-leader project
- Workspace `ea-ia-gym-java` is bound to team-leader (confirmed in workspace_bindings.json)
- Workspace session file `team-leader_ws_554233e8.json` exists in `~/.cc-connect/sessions/`

**Steps:**
1. Extract management token and call the sessions list endpoint.
   ```bash
   export MGMT_TOKEN=$(grep 'token' ~/.cc-connect/config.toml | head -1 | cut -d'"' -f2)
   curl -s -H "Authorization: Bearer ${MGMT_TOKEN}" http://localhost:9820/api/v1/projects/team-leader/sessions | python3 -m json.tool > /tmp/sessions_response.json
   ```
2. Count total sessions and identify workspace vs global sessions.
   ```bash
   TOTAL=$(python3 -c "import json; d=json.load(open('/tmp/sessions_response.json')); print(len(d['data']['sessions']))")
   WORKSPACE_SESSIONS=$(python3 -c "import json; d=json.load(open('/tmp/sessions_response.json')); print(len([s for s in d['data']['sessions'] if s.get('workspace','') != '']))")
   GLOBAL_SESSIONS=$(python3 -c "import json; d=json.load(open('/tmp/sessions_response.json')); print(len([s for s in d['data']['sessions'] if s.get('workspace','') == '']))")
   echo "Total: ${TOTAL}, Global: ${GLOBAL_SESSIONS}, Workspace: ${WORKSPACE_SESSIONS}"
   ```
   → If `${WORKSPACE_SESSIONS}` is 0, **stop** — workspace sessions are not being aggregated.
3. Verify the specific workspace session key that was missing before the fix appears.
   ```bash
   python3 -c "
   import json
   d = json.load(open('/tmp/sessions_response.json'))
   keys = [s['session_key'] for s in d['data']['sessions']]
   target = 'feishu:oc_5a27bcb9ee8d86533ab62e276ba62999:ou_8d3dbd02714bb466bd4a5d270981741c'
   if target in keys:
       s = [s for s in d['data']['sessions'] if s['session_key'] == target][0]
       print(f'FOUND: workspace={s.get(\"workspace\", \"MISSING\")}')
   else:
       print('NOT FOUND: session key still missing from API')
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| MGMT_TOKEN | string | ${MGMT_TOKEN} | runtime-fetch | extracted from config.toml in Step 1 |
| project_name | string | team-leader | concrete | project with known workspace bindings |
| target_session_key | string, pattern: feishu:oc_\w+:ou_\w+ | feishu:oc_5a27bcb9ee8d86533ab62e276ba62999:ou_8d3dbd02714bb466bd4a5d270981741c | concrete | the session key that was missing before the fix |

**Expected Results:**
- (must) Response contains more sessions than the pre-fix count (3 global-only sessions)
- (must) At least one session has `workspace` field set to a non-empty directory path
- (must) Global sessions have `workspace` field equal to `""`
- (must) The target session key `feishu:oc_5a27bcb9ee8d86533ab62e276ba62999:ou_8d3dbd02714bb466bd4a5d270981741c` appears in the response

**Failure Handling:**
- Step 2 returns WORKSPACE_SESSIONS=0: workspacePool aggregation not working — check that `e.workspacePool` is non-nil and `e.workspacePool.All()` returns workspace states
- Step 3 returns NOT FOUND: the specific workspace's SessionManager may not have loaded the session — verify the session file exists and is not corrupted

### Scenario S2: Workspace session live and platform fields resolved correctly

**Verifies:** AC-3, AC-4

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect is running with team-leader project
- At least one workspace session is currently active (interactive state exists with `workspace:sessionKey` pattern in `interactiveStates`)
- Workspace session has UserMeta populated (user_name, chat_name fields)

**Steps:**
1. Get the sessions list and find an active workspace session.
   ```bash
   export MGMT_TOKEN=$(grep 'token' ~/.cc-connect/config.toml | head -1 | cut -d'"' -f2)
   curl -s -H "Authorization: Bearer ${MGMT_TOKEN}" http://localhost:9820/api/v1/projects/team-leader/sessions | python3 -c "
   import json, sys
   d = json.load(sys.stdin)
   active_ws = [s for s in d['data']['sessions'] if s.get('workspace','') != '' and s.get('live') == True]
   if active_ws:
       s = active_ws[0]
       print(f'session_key: {s[\"session_key\"]}')
       print(f'workspace: {s[\"workspace\"]}')
       print(f'live: {s[\"live\"]}')
       print(f'platform: {s[\"platform\"]}')
       print(f'user_name: {s.get(\"user_name\", \"MISSING\")}')
       print(f'chat_name: {s.get(\"chat_name\", \"MISSING\")}')
   else:
       # Fall back to any workspace session for platform check
       ws = [s for s in d['data']['sessions'] if s.get('workspace','') != '']
       if ws:
           s = ws[0]
           print(f'session_key: {s[\"session_key\"]}')
           print(f'workspace: {s[\"workspace\"]}')
           print(f'live: {s[\"live\"]}')
           print(f'platform: {s[\"platform\"]}')
           print(f'user_name: {s.get(\"user_name\", \"MISSING\")}')
           print(f'chat_name: {s.get(\"chat_name\", \"MISSING\")}')
       else:
           print('NO_WORKSPACE_SESSIONS')
   " > /tmp/ws_session_check.txt 2>&1
   cat /tmp/ws_session_check.txt
   ```
2. Verify the `active_keys` map contains the workspace-prefixed key pattern.
   ```bash
   python3 -c "
   import json
   d = json.load(open('/tmp/sessions_response.json'))
   active_keys = d['data'].get('active_keys', {})
   ws_keys = [k for k in active_keys if ':' in k and k.startswith('/')]
   print(f'Workspace-prefixed active keys: {len(ws_keys)}')
   for k in ws_keys:
       print(f'  {k} -> {active_keys[k]}')
   "
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| MGMT_TOKEN | string | ${MGMT_TOKEN} | runtime-fetch | extracted from config.toml |

**Expected Results:**
- (must) Workspace sessions have `platform` field set (not empty/missing) — either resolved from `activeKeys[workspace:sessionKey]` or from `splitSessionKey` fallback
- (must) Workspace sessions that are currently interactive have `live: true`
- (should) Workspace sessions with UserMeta have `user_name` and `chat_name` populated (not `"MISSING"`)

**Failure Handling:**
- NO_WORKSPACE_SESSIONS: no workspace sessions at all — check workspace bindings are loaded and at least one workspace has received a message
- `platform` field empty for workspace session: `interactiveKey` lookup with `workspace:sessionKey` prefix is failing — verify the key construction matches what `engine.go` stores in `interactiveStates`
- `user_name` is `"MISSING"`: `GetUserMeta` call on workspace SessionManager is not wired up

### Scenario S3: Session detail endpoint resolves workspace sessions

**Verifies:** AC-5

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect is running with team-leader project
- A workspace session exists (confirmed from S1 response)

**Steps:**
1. Get a workspace session's ID and workspace path from the list endpoint.
   ```bash
   export MGMT_TOKEN=$(grep 'token' ~/.cc-connect/config.toml | head -1 | cut -d'"' -f2)
   WS_SESSION=$(python3 -c "
   import json
   d = json.load(open('/tmp/sessions_response.json'))
   ws = [s for s in d['data']['sessions'] if s.get('workspace','') != '']
   if ws:
       s = ws[0]
       print(f'{s[\"id\"]}|{s[\"workspace\"]}')
   else:
       print('NONE')
   ")
   WS_ID=$(echo "${WS_SESSION}" | cut -d'|' -f1)
   WS_PATH=$(echo "${WS_SESSION}" | cut -d'|' -f2)
   echo "Workspace session ID: ${WS_ID}, workspace: ${WS_PATH}"
   ```
   → If `${WS_SESSION}` is "NONE", **stop** — no workspace sessions to test with.
2. Call the detail endpoint with the `workspace` query parameter.
   ```bash
   curl -s -H "Authorization: Bearer ${MGMT_TOKEN}" "http://localhost:9820/api/v1/projects/team-leader/sessions/${WS_ID}?workspace=${WS_PATH}" | python3 -m json.tool
   ```
3. Call the same endpoint without the `workspace` parameter to confirm it returns 404 (pre-fix behavior).
   ```bash
   curl -s -w "\nHTTP_STATUS: %{http_code}" -H "Authorization: Bearer ${MGMT_TOKEN}" "http://localhost:9820/api/v1/projects/team-leader/sessions/${WS_ID}" | tail -1
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| MGMT_TOKEN | string | ${MGMT_TOKEN} | runtime-fetch | extracted from config.toml |
| WS_ID | string | $WS_ID (extracted in Step 1) | system-generated | workspace session ID from S1 list response |
| WS_PATH | string | $WS_PATH (extracted in Step 1) | system-generated | workspace directory path from S1 list response |

**Expected Results:**
- (must) Step 2 returns HTTP 200 with session detail including `session_key`, `history`, and `workspace` fields
- (must) Step 3 returns HTTP 404 (workspace session ID not found in global SessionManager)

**Failure Handling:**
- Step 2 returns 404: the `workspace` query parameter is not being read or the workspace SessionManager lookup is failing — check `handleProjectSessionDetail` reads `r.URL.Query().Get("workspace")`
- Step 3 returns 200: the session ID happens to exist in the global SessionManager too (ID collision) — try a different workspace session or verify with a session ID that only exists in the workspace

### Scenario S4: Session switch endpoint works for workspace sessions

**Verifies:** AC-6

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect is running with team-leader project
- A workspace session exists with at least 2 sessions for the same session key (so there is another session to switch to)

**Steps:**
1. Get a workspace session's key, active session ID, and workspace path.
   ```bash
   export MGMT_TOKEN=$(grep 'token' ~/.cc-connect/config.toml | head -1 | cut -d'"' -f2)
   curl -s -H "Authorization: Bearer ${MGMT_TOKEN}" http://localhost:9820/api/v1/projects/team-leader/sessions | python3 -c "
   import json, sys
   d = json.load(sys.stdin)
   ws = [s for s in d['data']['sessions'] if s.get('workspace','') != '']
   for s in ws:
       print(f'{s[\"session_key\"]}|{s[\"id\"]}|{s[\"workspace\"]}|{s[\"active\"]}')
   " > /tmp/ws_sessions_all.txt
   cat /tmp/ws_sessions_all.txt
   ```
2. Pick a workspace session and attempt to switch to it.
   ```bash
   # Use the first workspace session entry
   FIRST_WS=$(head -1 /tmp/ws_sessions_all.txt)
   WS_KEY=$(echo "${FIRST_WS}" | cut -d'|' -f1)
   WS_ID=$(echo "${FIRST_WS}" | cut -d'|' -f2)
   WS_PATH=$(echo "${FIRST_WS}" | cut -d'|' -f3)
   curl -s -w "\nHTTP_STATUS: %{http_code}" -X POST \
     -H "Authorization: Bearer ${MGMT_TOKEN}" \
     -H "Content-Type: application/json" \
     -d "{\"session_key\": \"${WS_KEY}\", \"session_id\": \"${WS_ID}\", \"workspace\": \"${WS_PATH}\"}" \
     http://localhost:9820/api/v1/projects/team-leader/sessions/switch
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| MGMT_TOKEN | string | ${MGMT_TOKEN} | runtime-fetch | extracted from config.toml |
| WS_KEY | string | $WS_KEY (extracted in Step 2) | system-generated | session key of workspace session |
| WS_ID | string | $WS_ID (extracted in Step 2) | system-generated | session ID within workspace SessionManager |
| WS_PATH | string | $WS_PATH (extracted in Step 2) | system-generated | workspace directory path |

**Expected Results:**
- (must) Step 2 returns HTTP 200 with `{"message": "active session switched", "active_session_id": "<id>"}`
- (should) The response includes the correct session ID

**Failure Handling:**
- Step 2 returns 404: the `workspace` field in the request body is not being read — check `handleProjectSessionSwitch` parses the `workspace` field and looks up the correct SessionManager
- Step 2 returns 400: the `session_key` or `session_id` fields may be empty — check the extraction logic

### Scenario S5: Project list sessions_count includes workspace sessions

**Verifies:** AC-7

**Execution:** AI-autonomous

**Preconditions:**
- cc-connect is running with team-leader project
- Workspace sessions exist (confirmed from S1)

**Steps:**
1. Get the project list and extract team-leader's sessions_count.
   ```bash
   export MGMT_TOKEN=$(grep 'token' ~/.cc-connect/config.toml | head -1 | cut -d'"' -f2)
   curl -s -H "Authorization: Bearer ${MGMT_TOKEN}" http://localhost:9820/api/v1/projects | python3 -c "
   import json, sys
   d = json.load(sys.stdin)
   for p in d['data']['projects']:
       if p['name'] == 'team-leader':
           print(f'sessions_count: {p[\"sessions_count\"]}')
   "
   ```
2. Compare against the total session count from the sessions list endpoint (which now includes workspace sessions).
   ```bash
   LIST_COUNT=$(python3 -c "import json; d=json.load(open('/tmp/sessions_response.json')); print(len(d['data']['sessions']))")
   echo "Project list sessions_count vs sessions list total: compare after implementation"
   ```

**Parameters:**
| Name | Type | Value | Source | Notes |
|---|---|---|---|---|
| MGMT_TOKEN | string | ${MGMT_TOKEN} | runtime-fetch | extracted from config.toml |

**Expected Results:**
- (must) team-leader's `sessions_count` is greater than 3 (the pre-fix global-only count)
- (must) `sessions_count` equals the total number of sessions from the sessions list endpoint (global + workspace)

**Failure Handling:**
- `sessions_count` still equals 3: the workspace session count is not being added in `handleProjects` — check that the workspacePool iteration and count accumulation is implemented

## Coverage Matrix

| Acceptance Criterion | Covered by Scenario |
|---|---|
| AC-1 | S1 |
| AC-2 | S1 |
| AC-3 | S2 |
| AC-4 | S2 |
| AC-5 | S3 |
| AC-6 | S4 |
| AC-7 | S5 |
