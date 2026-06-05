# cc-cache v2 Phase 9: Session Workspace

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 9.

## Phase 9: Session Workspace

**Purpose:** Build the per-session evidence and control surface on top of tested parser, Reminder, and KeepAlive logic.

- [x] **Step 9.1: Write failing workspace render/update tests**
  - Extend: `internal/tui/render_test.go`, `internal/tui/update_test.go`.
  - Assertions: evidence and controls are visually separate; Status, Messages, Token Stats, and Gaps render in order; Reminder row exact safety copy appears; KeepAlive off row exact safety copy appears; focus order matches design; manual refresh action is visible, focusable, activated by `enter`, and updates only the selected session path; `enter` and `space` toggle focused controls; `r`, `k`, `s`, `x`, `c`, `b`, `esc`, `?`, and `q` behave only where available; disabled safety-relevant controls remain focusable with reasons; Auto-send is visibly disabled while sending, confirming, or after hard failure until re-enabled safely; countdown/manual/confirming states move default focus to the active action group.
  - Async safety assertions: stale timer, runner, and confirmation Bubbletea messages are ignored after `x`, dismiss, stop-waiting, session switch, or refresh edge reset.
  - Run: `go test ./internal/tui`.
  - Expected: fails before workspace implementation.

- [x] **Step 9.2: Implement Session Workspace evidence and controls**
  - Create: `internal/tui/workspace.go`.
  - Behavior: evidence scroll region is focusable only when evidence overflows; footer copy reflects focus and active KeepAlive state; manual refresh is in the action row and has no hidden shortcut requirement.
  - Run: `go test ./internal/tui`.
  - Expected: base workspace tests pass.

- [x] **Step 9.3: Implement KeepAlive card rendering**
  - Modify: `internal/tui/workspace.go`.
  - Required card states: watching auto-send on, watching auto-send off, countdown, manual prompt, sending, confirming, success, failure, scope complete.
  - Required rows: state badge, State, Next, Claude message, Message, Scope, Progress/Evidence when relevant, Controls.
  - Verification: render tests assert each state includes whether a Claude message has been sent, may be sent, or will not be sent; `claude` unavailable is visible before any countdown can send; success shows confirmation timestamp and refreshed cache-window evidence; failure shows fallback command and stopped auto-send state where relevant.

- [x] **Step 9.4: Manual workspace smoke without sending**
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" PATH="/usr/bin:/bin" go run ./cmd/cc-cache --id 11111111`.
  - Expected: workspace opens from deterministic fixture data, Reminder toggles, KeepAlive can enter watching/manual states with fixture config `auto_send: false`, `claude` unavailable is visible if Auto-send is toggled on, and no real Claude subprocess starts.
