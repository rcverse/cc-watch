# cc-cache v2 Phase 8: Keepalive Core

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 8.

## Phase 8: KeepAlive State Machine And Bounded Automation

**Purpose:** Build dangerous automation as tested state logic before UI control wiring.

- [ ] **Step 8.1: Write failing KeepAlive timing tests**
  - Create: `internal/keepalive/keepalive_test.go`.
  - Assertions: 1-hour TTL default trigger at 5 minutes, 5-minute TTL default trigger at 1 minute, unknown TTL uses conservative 5-minute heuristic, countdown clamps to preserve 30-second safety margin, auto-send disables for an instance when margin cannot be preserved.
  - Run: `go test ./internal/keepalive`.
  - Expected: fails before KeepAlive implementation.

- [ ] **Step 8.2: Implement timing helpers**
  - Create: `internal/keepalive/timing.go`.
  - Run: `go test ./internal/keepalive`.
  - Expected: timing tests pass.

- [ ] **Step 8.3: Write failing state-machine tests**
  - Extend: `internal/keepalive/keepalive_test.go`.
  - Assertions: every design state transition from `off`, `monitoring_idle`, `countdown`, `manual_ready`, `sending`, `confirming`, `success`, hard failure states, `cancelled_instance`, and `scope_complete`; immediate evaluation when enabling inside trigger window; edge-triggered threshold crossing; re-arm only after refresh/new cache window; per-session state isolation; single-flight subprocess per session; cancellation stops countdown/manual/confirmation work; stale timer/runner/confirmation events include an instance token and are ignored after cancel, dismiss, stop-waiting, session switch, or refresh edge reset.
  - Run: `go test ./internal/keepalive`.
  - Expected: fails until state machine exists.

- [ ] **Step 8.4: Implement KeepAlive model and state machine**
  - Create: `internal/keepalive/model.go`, `internal/keepalive/state.go`.
  - Behavior: scope count increments exactly once when send attempt is initiated, regardless of subprocess or confirmation outcome.
  - Run: `go test ./internal/keepalive`.
  - Expected: state-machine tests pass.

- [ ] **Step 8.5: Write failing subprocess and confirmation tests**
  - Extend: `internal/keepalive/keepalive_test.go`.
  - Assertions: `claude` unavailable preflight while KeepAlive is armed, `claude` unavailable before countdown can send, subprocess non-zero path, Claude limit path, confirmation tied to target session file and timestamp/new line after send attempt, timeout path, manual fallback command generated without unsafe quoting display.
  - Run: `go test ./internal/keepalive`.
  - Expected: fails before runner and confirmation implementation.

- [ ] **Step 8.6: Implement runner and confirmation**
  - Create: `internal/keepalive/runner.go`, `internal/keepalive/confirm.go`.
  - Behavior: real subprocess is behind an interface; tests use fakes; no test sends a real Claude message.
  - Run: `go test ./internal/keepalive`.
  - Expected: all KeepAlive tests pass.
