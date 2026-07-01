# cc-watch v2 Phase 13: Documentation

> Refreshed on 2026-06-29 because the original mechanically sliced checklist referenced a nonexistent `BOOTSTRAP.md`. Use the existing prompt-bank files for project bootstrap/resume guidance instead of creating a new root bootstrap file by assumption.

## Phase 13: Documentation Updates

**Purpose:** Remove stale v1/v2 instructions and document safety-critical behavior.

- [x] **Step 13.1: Update README**
  - Modify: `README.md`.
  - Include: v2 CLI contract, one-repo layout, Go toolchain requirement for developers, local binary install steps for users, config path, Reminder vs KeepAlive safety distinction, JSON output contract summary, degraded watcher/notification behavior, v1 archive and rollback path, and deferred public release/Homebrew status.
  - Verification: README no longer advertises watch mode or Python/Rich as the v2 implementation.

- [x] **Step 13.2: Update implementation prompt bank**
  - Modify:
    - `docs/superpowers/prompts/cc-watch-v2/initial-bootstrap.md`
    - `docs/superpowers/prompts/cc-watch-v2/per-phase-initialization.md`
    - `docs/superpowers/prompts/cc-watch-v2/resume-anywhere.md`
  - Include: current Go/Bubbletea architecture, source-of-truth docs, current product boundary spec, fresh-context resume protocol, parser parity gate, KeepAlive safety gates, one-repo governance, no-real-Claude-send rule, implementation status via progress ledger, local command switchover approval requirement, no public watch command or flag, and no Homebrew/GitHub Release/goreleaser/Linux/Windows scope.
  - Verification: prompt bank no longer implies public release, Homebrew, Linux/Windows packaging, real Claude KeepAlive sends, command switchover, or a public watch mode are in-scope.

- [x] **Step 13.3: Documentation consistency check**
  - Run: `rg --fixed-strings -- 'watch command' README.md docs/superpowers/prompts/cc-watch-v2/*.md PRD.md docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md docs/superpowers/specs/2026-06-18-cc-watch-v2-product-reality.md`.
  - Expected: watch wording appears only as a v2 non-goal/internal-refresh clarification, not as supported usage.
  - Run: `rg --fixed-strings -- 'Rich' README.md docs/superpowers/prompts/cc-watch-v2/*.md` and `rg --fixed-strings -- 'termios' README.md docs/superpowers/prompts/cc-watch-v2/*.md`.
  - Expected: no stale v2 implementation instruction says Rich is the v2 stack.
  - Run: `git diff --check`.
  - Run safe doc examples against `dist/cc-watch` and fixture HOME; run `scripts/test-install.sh` because install docs changed.
