# Feishu Interactive Card Text Extraction Deployment

> Source spec: ./2026-06-19-feishu-card-text-extraction-design.md
> Cross-reference (for v2): ./2026-06-19-feishu-card-text-extraction-verification.md
>
> **Two-Phase Document**
> - Phase 1 (v1): authored during brainstorming via writing-deployment-plan. Describes intent.
> - Phase 2 (v2): completed during finishing-a-development-branch. Adds executable runbook.

---

## Phase 1 · Release Intent (v1)

### 1.1 Release Scope
- Affected services/components: `cc-connect` (single Go binary)
- Includes frontend release: no
- Includes data migration: no
- Includes config change: no

### 1.2 Infrastructure Touchpoints
- DB migration: none
- Env vars: none
- TCC config: none
- New dependencies: none
- Capacity: no scaling needed

### 1.3 Deployment Order
| Step | Action | Must precede |
|---|---|---|
| 1 | Build and deploy cc-connect binary | — |

Single-step deployment. The change is a pure logic update in the Feishu platform adapter's card text extraction function. No coordinated rollout needed.

### 1.4 Risk Assessment
- R1: Extracted text differs from previous output for existing card types — mitigation: all 5 existing test cases must continue to pass; the new `extractLegacyElementText` helper replaces the loop body but produces the same output for previously handled element types (`div` with `text`, plain `text` elements)
- R2: New element type handling produces unexpected text (e.g., extracting button labels that clutter the output) — mitigation: review test output for the mixed alarm card scenario (S11) to confirm the extracted text is reasonable and not overly verbose
- R3: Deeply nested cards cause performance issues — mitigation: `maxExtractDepth = 10` hard limit prevents unbounded recursion

### 1.5 Rollback Strategy (high-level)
- Rollback trigger: existing card messages produce missing or garbled text after deployment
- Rollback scope: code only — revert the commit
- Data reversibility: N/A (no data changes)

### 1.6 Observability Plan
- Key metrics: no new metrics needed; the DIAG diagnostic code (lines 1096-1143) already logs a comparison of event text vs API text for interactive cards — this can be used to spot-check extraction quality post-deploy
- Key logs: `slog.Debug` messages for dropped/empty interactive cards already exist; no new log lines required
- Observation windows: 15min post-deploy — spot-check a few alarm card messages in the target Feishu group to confirm detail fields (PSM, cluster, region) are visible in agent responses

### 1.7 Communication
- Notify: none required — this is an internal bug fix with no API or behavioral contract change
- Timing: N/A
- Channels: N/A

---

## Phase 2 · Release Runbook (v2)

### 2.1 Pre-Deploy Checklist
- [x] All 19 test cases in `TestExtractInteractiveCardText` pass
- [x] `go build ./...` succeeds
- [x] `go vet ./...` clean
- [x] Race detector clean on `platform/feishu/` tests
- [x] Full test suite `go test ./...` passes
- [ ] PR merged to main

### 2.2 Executable Steps

#### Step 1: Build cc-connect binary
**Execution:** AI-autonomous
```bash
cd /data00/home/chenhao.magic/Project/Source/Github/cc-connect
make build
```
Expected: binary `cc-connect` produced in project root.

#### Step 2: Deploy binary
**Execution:** Human-assisted — deployment target and method depend on the user's infrastructure (direct binary replacement, systemd restart, container rebuild, etc.).
```bash
# Replace with actual deployment command, e.g.:
# scp cc-connect <host>:/usr/local/bin/cc-connect
# ssh <host> 'systemctl restart cc-connect'
```
Expected: cc-connect process running with updated binary.

### 2.3 Post-Deploy Verification

**Automated (run locally):**
```bash
go test ./platform/feishu/ -run TestExtractInteractiveCardText -v
```
Expected: 19/19 PASS. This covers S1–S12 from verification.md.

**Manual smoke test (15min observation window):**
1. Send a Feishu alarm card to the target group (or wait for a real alarm)
2. Confirm the AI agent response includes detail fields (PSM, cluster, region) that were previously missing
3. Verify existing card types (simple div/text cards) still produce correct output

This covers S10 (user_dsl alarm card), S11 (mixed alarm card), and S13 (existing test regression).

### 2.4 Concrete Rollback Commands
```bash
# Revert the feature commits (5 commits on shadow branch)
git revert HEAD~5..HEAD --no-edit

# Rebuild and redeploy
make build
# scp cc-connect <host>:/usr/local/bin/cc-connect
# ssh <host> 'systemctl restart cc-connect'
```

### 2.5 v1 Revisions Log
| Section | v1 description | v2 revision | Reason |
|---|---|---|---|
| 1.6 Observability | DIAG diagnostic code at lines 1096-1143 | No longer recommended for post-deploy spot-check — DIAG makes extra API calls per message; rely on manual smoke test instead | DIAG is debug-only code, not suitable for production observation |
| 1.4 Risk Assessment R1 | "all 5 existing test cases must continue to pass" | Updated: 19 test cases now (5 original + 14 new) | Implementation added 14 new test cases covering all element types |
