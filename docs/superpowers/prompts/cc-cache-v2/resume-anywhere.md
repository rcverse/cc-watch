# Resume-Anywhere Prompt

Use this when a context was interrupted or the user says "continue implementing".

```text
Continue cc-cache v2 implementation in /Users/richardchen/Dev/cc-cache.

Read only:
1. AGENTS.md
2. docs/superpowers/runbooks/2026-06-03-cc-cache-v2-implementation-runbook.md
3. docs/superpowers/progress/cc-cache-v2-progress.md
4. Narrow-scan phase progress:
   - rg -n "Current phase|Current step|Status|^- \\[[ x]\\]|^## Last Context Handoff" docs/superpowers/progress/cc-cache-v2-progress.md
   - if progress is unclear, run:
     rg -n "^(# cc-cache v2 Phase|- \\[ \\])" docs/superpowers/plans/cc-cache-v2/phase-*.md
5. docs/superpowers/plans/cc-cache-v2/PLAN.md:
   - Source Of Truth
   - Implementation Readiness Gates
   - Public JSON Contract, only for Phase 0 Step 0.7 or Phase 3 JSON work
   - Failure-Mode Map entries relevant to the current phase
6. The current phase file under docs/superpowers/plans/cc-cache-v2/
7. PRD.md and docs/superpowers/specs/2026-06-02-cc-cache-v2-design.md only if needed to verify current-phase requirements or if they changed.

Before editing:
- Run git status --short --branch.
- Inspect recent diffs only for files touched in the current phase.
- Identify the current phase and first unchecked step.
- Verify that completed steps claimed in the progress ledger still hold when cheap to verify.

Required skills:
- superpowers:using-superpowers
- superpowers:executing-plans
- superpowers:test-driven-development when writing behavior
- superpowers:verification-before-completion before marking the phase complete
- superpowers:systematic-debugging if current state contradicts the plan or tests fail unexpectedly

Execution:
- Continue from the next safe unchecked step.
- Do not re-plan unless source-of-truth docs changed or implementation contradicts the plan.
- Do not use implementation subagents by default.
- Do not run a real Claude KeepAlive send.
- Do not replace the cc-cache symlink or publish releases.
```
