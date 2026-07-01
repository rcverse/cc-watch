# Per-Phase Initialization Prompt

Use this at the start of each fresh phase context. Replace `PHASE_NUMBER` and `PHASE_FILE`.

```text
You are implementing Phase PHASE_NUMBER only for cc-watch v2 in /Users/richardchen/Dev/cc-watch.

Read only:
1. AGENTS.md
2. docs/superpowers/runbooks/2026-06-03-cc-watch-v2-implementation-runbook.md
3. docs/superpowers/progress/cc-watch-v2-progress.md
4. docs/superpowers/plans/cc-watch-v2/PLAN.md:
   - Source Of Truth
   - Implementation Readiness Gates
   - Target File Structure entries relevant to Phase PHASE_NUMBER
   - Public JSON Contract, only for Phase 0 Step 0.7 or Phase 3 JSON work
   - Failure-Mode Map entries relevant to Phase PHASE_NUMBER
   - Requirement Coverage Matrix row relevant to Phase PHASE_NUMBER
5. docs/superpowers/specs/2026-06-18-cc-watch-v2-product-reality.md for the current product boundary.
6. docs/superpowers/plans/cc-watch-v2/PHASE_FILE
7. PRD.md and docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md only for requirements directly touched by Phase PHASE_NUMBER.

Before editing:
- Run git status --short --branch.
- Confirm prior phase gates required by Phase PHASE_NUMBER are satisfied.
- Identify the first unchecked Phase PHASE_NUMBER step.

Required skills:
- superpowers:using-superpowers
- superpowers:executing-plans
- superpowers:test-driven-development when writing behavior
- superpowers:verification-before-completion before marking Phase PHASE_NUMBER complete

Execution:
- Execute Phase PHASE_NUMBER linearly.
- Do not implement the next phase.
- Do not use implementation subagents by default.
- Preserve the current product scope: lightweight macOS Go/Bubbletea CLI/TUI, stable `--json`, internal TUI live refresh, native macOS Reminder notifications only, visible bounded KeepAlive, config editor, and simple local install.
- Do not run a real Claude KeepAlive send.
- Do not replace $HOME/.local/bin/cc-watch or run `./install.sh --yes` against the real user HOME without explicit approval.
- Do not add a public watch command or flag; live refresh is internal TUI behavior.
- Do not publish releases, create Homebrew tap work, run goreleaser, start Linux/Windows packaging, start daemon work, or start native app work unless explicitly re-approved.
- After local verification passes, dispatch one read-only reviewer subagent for Phase PHASE_NUMBER.
- Integrate valid review findings, rerun verification, update the phase checklist and progress ledger, then stop with a concise summary.
```
