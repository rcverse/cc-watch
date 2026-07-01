# Bootstrap Prompt: Execute cc-watch v2 Phase 11.7

Use this prompt to start a fresh implementation context for Phase 11.7 only.

```text
PHASE_NUMBER = 11.7
PHASE_FILE = phase-11.7-tui-interaction-architecture.md

You are implementing Phase PHASE_NUMBER only for cc-watch v2 in /Users/richardchen/Dev/cc-watch.

Read only:
1. AGENTS.md
2. docs/superpowers/runbooks/2026-06-03-cc-watch-v2-implementation-runbook.md
3. docs/superpowers/progress/cc-watch-v2-progress.md
4. docs/superpowers/plans/cc-watch-v2/PLAN.md:
   - Source Of Truth
   - Implementation Readiness Gates
   - Phase Index
   - Target File Structure entries relevant to internal/tui, internal/app, internal/session, and internal/refresh
   - Failure-Mode Map entries relevant to refresh, Bubbletea, notifications, Reminder, and KeepAlive
5. docs/superpowers/plans/cc-watch-v2/PHASE_FILE
6. PRD.md and docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md only for requirements directly touched by Phase PHASE_NUMBER.

Before editing:
- Run git status --short --branch.
- Confirm Phase 11.6 gates are satisfied.
- Identify the first unchecked Phase PHASE_NUMBER step.
- Note any unrelated dirty files and do not stage/revert them.

Required skills:
- superpowers:using-superpowers
- superpowers:executing-plans
- superpowers:test-driven-development when writing behavior
- superpowers:verification-before-completion before marking Phase PHASE_NUMBER complete

Execution:
- Execute Phase PHASE_NUMBER linearly.
- Do not implement Phase 12.
- Do not use implementation subagents by default.
- Do not run a real Claude KeepAlive send.
- Do not replace $HOME/.local/bin/cc-watch.
- Use TDD for focus, refresh, details, KeepAlive-card, and rendering behavior.
- Preserve public CLI flags, JSON schema, parser metrics, config schema, installer behavior, and release behavior.
- After local verification passes, dispatch one read-only reviewer subagent for Phase PHASE_NUMBER.
- Integrate valid review findings, rerun verification, update the phase checklist and progress ledger, then stop with a concise summary.

Primary design invariants:
- Every cursor-focusable item must render a visible marker.
- List cursor navigates sessions only.
- Session cursor navigates controls/actions only.
- Verbose/details is a disclosure state, not a control row.
- Expanded Session Info contains JSONL, timestamps, full token stats, and gaps.
- Evidence is not user-facing TUI copy.
- Gaps default to longest-duration sort and can toggle to newest temporal sort; sort label is muted arrow text such as ↕ longest / ↕ newest.
- KeepAlive cards are additive below Controls and separate info rows from action rows.
- Refresh is autonomous; manual update key is u.
- TTL elapsed percent and token hit-rate percent use different semantic color scales.
```
