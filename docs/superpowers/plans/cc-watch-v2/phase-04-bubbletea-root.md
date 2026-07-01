# cc-watch v2 Phase 4: Bubbletea Root

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 4.

## Phase 4: Bubbletea Root Model, Messages, And Visual System

**Purpose:** Establish explicit event flow and shared visual vocabulary before screens become complex.

- [x] **Step 4.1: Write failing TUI update tests**
  - Create: `internal/tui/update_test.go`.
  - Assertions: display tick recomputes time-derived values and countdown timers only; fake discovery/parser/refresh dependencies are not called by one-second display ticks; watcher events arrive as messages; refresh results replace session state without hidden goroutine mutation; stale refresh generations are ignored; `?` toggles help; `q` quits; cursor actions are reachable through focus.
  - Run: `go test ./internal/tui`.
  - Expected: fails before TUI skeleton exists.

- [x] **Step 4.2: Implement root Bubbletea model**
  - Create: `internal/tui/model.go`, `internal/tui/messages.go`, `internal/tui/update.go`, `internal/tui/view.go`.
  - Behavior: List View is default; direct `--id` starts Session Workspace; ambiguous `--id` starts ambiguity selection state; `config` starts Config Editor.
  - Run: `go test ./internal/tui`.
  - Expected: root model tests pass.

- [x] **Step 4.3: Implement semantic styles**
  - Create: `internal/tui/styles.go`.
  - Roles: neutral, muted, selected focus, info, warning, danger, success, disabled, degraded.
  - Verification: render tests assert color is not the only signal by checking state text appears with badges or labels.

- [x] **Step 4.4: Add help overlay**
  - Create: `internal/tui/help.go`.
  - Behavior: help explains cursor navigation and shortcuts; dangerous KeepAlive active state remains visible or summarized when help is open.
  - Verification: render tests assert help includes `arrows`, `enter`, `space`, `?`, `q`; List View, normal Workspace, countdown, confirming, failure, and Config Editor show only currently valid shortcuts or disabled safety-relevant actions with reasons; normal Workspace does not advertise `s` or `x` when no send/cancel action exists.
