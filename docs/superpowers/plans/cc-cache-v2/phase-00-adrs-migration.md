# cc-cache v2 Phase 0: Adrs Migration

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 0.

## Phase 0: Implementation Authorization, ADRs, And Migration Contract

**Purpose:** Resolve implementation-blocking decisions before code exists.

- [x] **Step 0.1: Confirm implementation approval**
  - Files: this plan only.
  - Action: user explicitly approves implementation and accepts any listed unresolved questions.
  - Verification: implementation agent records approval in its session summary before editing runtime files.

- [x] **Step 0.2: Record clean starting state**
  - Files: none.
  - Run: `git status --short --branch`.
  - Expected: note existing unrelated changes without reverting or editing them.
  - Gate: if source-of-truth files changed since this plan was written, reread `PRD.md` and the design spec before continuing.

- [x] **Step 0.3: Create ADR for v1 archive and command path**
  - Create: `docs/adr/2026-06-03-v1-archive-and-command-path.md`.
  - Decision to record: copy v1 into `archive/v1/`, leave root `cc_cache.py` in place during implementation, keep the live symlink usable, and switch `$HOME/.local/bin/cc-cache` only after Go binary verification and explicit user approval.
  - Verification: ADR names current symlink target `/Users/richardchen/Dev/cc-cache/cc_cache.py` and explains why early moves are unsafe.

- [x] **Step 0.4: Create ADR for Go module and release identity**
  - Create: `docs/adr/2026-06-03-go-module-and-release-identity.md`.
  - Decision to record: one-repo governance, module path `github.com/richardchen/cc-cache`, no separate Homebrew tap repo during implementation, local binary install as the first distribution target, and GitHub Releases/Homebrew as deferred release work.
  - Verification: `go mod init` is not run until this ADR records `go mod init github.com/richardchen/cc-cache` as the implementation command.

- [x] **Step 0.5: Create ADR for refresh safety interval**
  - Create: `docs/adr/2026-06-03-refresh-safety-interval.md`.
  - Decision to record: internal safety refresh interval, debounce window for fsnotify bursts, manual refresh behavior, degraded watcher messaging, and refresh-result freshness ordering.
  - Freshness rule to record: every data refresh request receives a monotonic generation; manual refresh increments generation immediately; debounced fsnotify and safety refresh results apply only when their generation is current for the affected scope; delete/rename observations cannot be overwritten by an older parse result.
  - Verification: ADR states that there is no public `--watch` mode and no configurable watch interval in v2.

- [x] **Step 0.6: Create ADR for KeepAlive subprocess safety**
  - Create: `docs/adr/2026-06-03-keepalive-subprocess-safety.md`.
  - Decision to record: exact Claude CLI invocation using the selected session ID and configured message, PATH lookup behavior, availability preflight while KeepAlive is armed, timeout, confirmation window, stale async message handling, cancellation semantics, Claude limit detection, scope counting, and manual fallback wording.
  - Verification: ADR states that attempted sends count once at send initiation and that confirmation must be tied to the target session JSONL after the send timestamp.

- [x] **Step 0.7: Create ADR for JSON output schema**
  - Create: `docs/adr/2026-06-03-json-output-schema.md`.
  - Decision to record: schema version value, top-level success object, selected-session object, list object, error object for no-match/ambiguous partial IDs, degraded refresh/notification encoding, and runtime Reminder/KeepAlive encoding when available.
  - Verification: ADR includes the exact JSON contract in the `Public JSON Contract` section of this plan and says JSON output never emits ANSI escape sequences.
