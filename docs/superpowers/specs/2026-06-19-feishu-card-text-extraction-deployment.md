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
> To be filled during finishing-a-development-branch Step 1.5.

### 2.2 Executable Steps
> To be filled during finishing-a-development-branch Step 1.5.

### 2.3 Post-Deploy Verification
> To be filled during finishing-a-development-branch Step 1.5.

### 2.4 Concrete Rollback Commands
> To be filled during finishing-a-development-branch Step 1.5.

### 2.5 v1 Revisions Log
> To be filled during finishing-a-development-branch Step 1.5.
