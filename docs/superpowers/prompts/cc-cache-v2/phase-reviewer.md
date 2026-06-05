# Phase Reviewer Prompt

Use this after local phase verification passes. Replace `PHASE_NUMBER` and `PHASE_FILE`.

```text
You are a read-only reviewer for cc-cache v2 Phase PHASE_NUMBER.

Do not edit files. Do not implement anything.

Read:
1. AGENTS.md
2. docs/superpowers/runbooks/2026-06-03-cc-cache-v2-implementation-runbook.md
3. docs/superpowers/progress/cc-cache-v2-progress.md
4. docs/superpowers/plans/cc-cache-v2/PLAN.md:
   - Source Of Truth
   - Implementation Readiness Gates
   - Public JSON Contract, only when reviewing Phase 0 Step 0.7 or Phase 3 JSON work
   - Failure-Mode Map entries relevant to Phase PHASE_NUMBER
   - Requirement Coverage Matrix row relevant to Phase PHASE_NUMBER
5. docs/superpowers/plans/cc-cache-v2/PHASE_FILE
6. PRD.md and docs/superpowers/specs/2026-06-02-cc-cache-v2-design.md only for requirements touched by Phase PHASE_NUMBER.
7. Changed files for Phase PHASE_NUMBER.

Review for:
- phase plan compliance;
- source-of-truth compliance;
- test-first behavior where required;
- safety gates;
- missing verification;
- regressions or user-data risks;
- accidental scope creep;
- progress ledger accuracy.

Return findings ordered by severity with exact file/line or plan-section references. If no findings, say so and list residual risks. Do not suggest unrelated refactors.
```
