# cc-cache v2 Implementation Runbook

**Purpose:** Strategy and operating model for implementing cc-cache v2 across fresh context windows.

This runbook is not a task checklist and does not contain copy-paste prompts. Use:

- Meta plan: `docs/superpowers/plans/cc-cache-v2/PLAN.md`
- Phase plans: `docs/superpowers/plans/cc-cache-v2/phase-XX-*.md`
- Progress ledger: `docs/superpowers/progress/cc-cache-v2-progress.md`
- Prompt bank: `docs/superpowers/prompts/cc-cache-v2/`

---

## 1. Authority Stack

Read the narrowest set needed for the current job.

1. `AGENTS.md`
2. This runbook
3. `docs/superpowers/progress/cc-cache-v2-progress.md`
4. `docs/superpowers/plans/cc-cache-v2/PLAN.md`
5. Current phase file under `docs/superpowers/plans/cc-cache-v2/`
6. `PRD.md`
7. `docs/superpowers/specs/2026-06-02-cc-cache-v2-design.md`
8. Current code and tests for the active phase only

Do not use retired plans or stale `BOOTSTRAP.md` text as v2 architecture truth.

## 2. Execution Model

Default execution is **linear by phase**.

Use a fresh context window for each phase when practical. A phase should start, execute, verify, review, update progress, and stop.

Do not parallelize implementation by default. Parser parity gates JSON and TUI, refresh event ordering affects TUI correctness, and KeepAlive safety depends on tested state transitions before UI wiring.

Allowed limited delegation:

- A read-only reviewer subagent runs after each phase's local verification passes.
- No implementation worker subagents are used by default.
- If the user explicitly asks for implementation subagents later, use one bounded worker for a full phase or clearly isolated workstream, never per step. This is an exception, not the default path.

## 3. Skill Policy

Required at the beginning of every implementation context:

- `superpowers:using-superpowers`
- `superpowers:executing-plans`

Required when writing behavior before implementation:

- `superpowers:test-driven-development`

Required before marking a phase complete:

- `superpowers:verification-before-completion`

Use only when needed:

- `superpowers:systematic-debugging` for unexpected test failures, smoke failures, or behavior that contradicts assumptions.
- `superpowers:requesting-code-review` only if the phase contains high-risk behavior or the user asks for extra review.
- `superpowers:subagent-driven-development` as a methodology reference only. Do not invoke its default per-step worker/reviewer loop for this project.

Subagent review policy:

- Implementation context: `gpt-5.5`, medium reasoning when available.
- Reviewer subagent: `gpt-5.5`, high reasoning when available.
- Reviewer is read-only and must not edit files.
- Reviewer returns findings ordered by severity with exact file/plan references.

## 4. Hard Safety Rules

- Do not run a real Claude KeepAlive send.
- Do not move, remove, or overwrite root `cc_cache.py` until the phase plan's migration gates allow it.
- Do not replace `$HOME/.local/bin/cc-cache` until v2 binary verification passes and the user explicitly approves switchover.
- Do not publish a GitHub Release or Homebrew tap update during normal implementation.
- Do not add a public `--watch` mode.
- Do not use retired implementation plans as source of truth.
- If Go is missing or older than Go 1.23, ask before installing tooling.

## 5. Phase Order

Implement phases in order:

1. Phase 0: `phase-00-adrs-migration.md`
2. Phase 1: `phase-01-go-skeleton.md`
3. Phase 2: `phase-02-parser-parity.md`
4. Phase 3: `phase-03-config-reminder-json.md`
5. Phase 4: `phase-04-bubbletea-root.md`
6. Phase 5: `phase-05-refresh-architecture.md`
7. Phase 6: `phase-06-list-view.md`
8. Phase 7: `phase-07-notifications.md`
9. Phase 8: `phase-08-keepalive-core.md`
10. Phase 9: `phase-09-session-workspace.md`
11. Phase 10: `phase-10-config-editor.md`
12. Phase 11: `phase-11-cli-integration.md`
13. Phase 12: `phase-12-packaging-install.md`
14. Phase 13: `phase-13-documentation.md`
15. Phase 14: `phase-14-final-acceptance.md`

Parser parity is the first major gate. Do not implement TUI, JSON session output, Reminder session-consumer code, or KeepAlive session-consumer code before Phase 2.6 passes.

## 6. Prompt Bank

Use prompt files instead of embedding prompts in this runbook:

- Initial bootstrap: `docs/superpowers/prompts/cc-cache-v2/initial-bootstrap.md`
- Per-phase initialization: `docs/superpowers/prompts/cc-cache-v2/per-phase-initialization.md`
- Resume anywhere: `docs/superpowers/prompts/cc-cache-v2/resume-anywhere.md`
- Phase reviewer: `docs/superpowers/prompts/cc-cache-v2/phase-reviewer.md`

## 7. Phase Completion Protocol

Before marking a phase complete:

1. Run the phase's exact verification commands.
2. Run any cheap package-level tests touched by the phase.
3. Check `git status --short`.
4. Confirm no unrelated user changes were reverted or overwritten.
5. Dispatch one read-only reviewer subagent.
6. Integrate accepted findings.
7. Rerun relevant verification.
8. Update the active phase file checkboxes.
9. Update `docs/superpowers/progress/cc-cache-v2-progress.md`.
10. Stop with files changed, verification run, reviewer findings and resolution, remaining risks, and next phase name.
