# Initial Bootstrap Prompt

Use this once when starting cc-watch v2 implementation from a fresh context before Phase 0 or Phase 1.

```text
You are implementing cc-watch v2 in /Users/richardchen/Dev/cc-watch.

Read only:
1. AGENTS.md
2. docs/superpowers/runbooks/2026-06-03-cc-watch-v2-implementation-runbook.md
3. docs/superpowers/progress/cc-watch-v2-progress.md
4. docs/superpowers/plans/cc-watch-v2/PLAN.md
5. The current phase file under docs/superpowers/plans/cc-watch-v2/
6. docs/superpowers/specs/2026-06-18-cc-watch-v2-product-reality.md for the current product boundary.
7. PRD.md and docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md only enough to verify current-phase requirements.

Before editing:
- Run git status --short --branch.
- Note unrelated changes without touching them.
- Identify the current phase from docs/superpowers/progress/cc-watch-v2-progress.md.
- Confirm the current phase file's first unchecked step.

Required skills:
- superpowers:using-superpowers
- superpowers:executing-plans
- superpowers:test-driven-development when writing behavior
- superpowers:verification-before-completion before marking a phase complete

Execution model:
- Implement linearly by phase.
- Do not use implementation subagents by default.
- Use one read-only reviewer subagent after local phase verification passes.
- Do not continue into the next phase without user approval.
- Current product shape is a lightweight macOS Go/Bubbletea CLI/TUI with stable `--json`, internal TUI live refresh, native macOS notifications for Reminder, bounded visible KeepAlive, config editor, and simple local install.

Safety:
- Do not run a real Claude KeepAlive send.
- Do not replace the cc-watch symlink without explicit user approval.
- Do not run `./install.sh --yes` against the real user HOME unless explicitly approved.
- Do not publish releases, create Homebrew tap work, run goreleaser, or start Linux/Windows packaging.
- Do not add a public watch command or flag; live refresh is internal TUI behavior.
- Do not start daemon, native app, or hidden unbounded KeepAlive work.
- If Go is missing or older than Go 1.23, ask before installing tooling.

Start at the current phase's first unchecked step. If source-of-truth docs changed since the phase plan was written, update the plan before implementation continues.
```
