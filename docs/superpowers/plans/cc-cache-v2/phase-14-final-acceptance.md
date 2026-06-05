# cc-cache v2 Phase 14: Final Acceptance

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 14.

## Phase 14: Final Acceptance Verification

**Purpose:** Verify product completeness against PRD/design before declaring v2 done.

- [ ] **Step 14.1: Full automated test suite**
  - Run: `go test ./... -count=1`.
  - Expected: pass.

- [ ] **Step 14.2: Build verification**
  - Run: `go build -o dist/cc-cache ./cmd/cc-cache`.
  - Expected: binary builds successfully.
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-cache --help && dist/cc-cache --version && dist/cc-cache --json`.
  - Expected: commands succeed; JSON is valid.

- [ ] **Step 14.3: TUI manual acceptance**
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-cache`.
  - Expected: List View answers "Which sessions need attention?", shows active/expired/unknown/degraded/KeepAlive states, and supports cursor-first actions.
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" PATH="/usr/bin:/bin" dist/cc-cache --id 11111111`.
  - Expected: Session Workspace separates evidence from controls and all actions are cursor reachable.
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-cache config`.
  - Expected: Config Editor edits defaults with validation and confirmation behavior.

- [ ] **Step 14.4: KeepAlive safety acceptance**
  - Verify with fake runner: countdown is visible, cancellable, scoped, and confirmation-based.
  - Verify Auto-send off: manual prompt appears and no Claude message is sent.
  - Verify unsafe timing: auto-send disables for that instance and shows manual prompt.
  - Verify cancellation: stale timer, subprocess, and confirmation messages cannot change UI state or send after cancel/dismiss/stop-waiting/session switch/refresh reset.
  - Verify preflight: `claude` unavailable is visible while armed and before countdown can send.
  - Verify failure: `claude` unavailable/subprocess/timeout states stop auto-send where relevant and show fallback.
  - Verify success: success card shows confirmation timestamp and refreshed cache-window evidence.
  - Verify scope: attempted sends count once and cannot loop every tick.

- [ ] **Step 14.5: Requirement mapping check**
  - Review every section of `PRD.md` and `docs/superpowers/specs/2026-06-02-cc-cache-v2-design.md`.
  - Expected: each requirement maps to a completed task in this plan or a documented non-goal.
  - Gate: any missing mapping becomes a new plan task or ADR before release.
