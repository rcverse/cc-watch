# cc-cache v2 Phase 13: Documentation

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 13.

## Phase 13: Documentation Updates

**Purpose:** Remove stale v1/v2 instructions and document safety-critical behavior.

- [ ] **Step 13.1: Update README**
  - Modify: `README.md`.
  - Include: v2 CLI contract, one-repo layout, Go toolchain requirement for developers, local binary install steps for users, config path, Reminder vs KeepAlive safety distinction, JSON output contract summary, degraded watcher/notification behavior, v1 archive and rollback path, and deferred public release/Homebrew status.
  - Verification: README no longer advertises `--watch` or Python/Rich as the v2 implementation.

- [ ] **Step 13.2: Update BOOTSTRAP**
  - Modify: `BOOTSTRAP.md`.
  - Include: current Go/Bubbletea architecture, source-of-truth docs, this plan path, fresh-context resume protocol, parser parity gate, KeepAlive safety gates, one-repo governance, no-real-Claude-send rule, and implementation status.
  - Verification: BOOTSTRAP no longer tells agents to build a Rich/termios single-file Python v2.

- [ ] **Step 13.3: Documentation consistency check**
  - Run: `rg --fixed-strings -- '--watch' README.md BOOTSTRAP.md PRD.md docs/superpowers/specs/2026-06-02-cc-cache-v2-design.md`.
  - Expected: `--watch` appears only as a v2 non-goal/rejected mode, not as supported usage.
  - Run: `rg --fixed-strings 'rich' README.md BOOTSTRAP.md`.
  - Expected: no stale v2 implementation instruction says Rich is the v2 stack.
