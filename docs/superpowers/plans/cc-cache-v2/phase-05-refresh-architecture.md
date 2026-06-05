# cc-cache v2 Phase 5: Refresh Architecture

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 5.

## Phase 5: Refresh Architecture And Degraded State

**Purpose:** Wire fsnotify as an accelerator while keeping safety refresh and manual refresh authoritative.

- [x] **Step 5.1: Write failing refresh tests**
  - Create: `internal/refresh/refresh_test.go`.
  - Assertions: recursive watches for existing project directories, root watch for new project directories, add watch for created subdirectories, create/write/rename/delete event normalization, debounce burst behavior, safety refresh tick behavior, watcher setup failure produces degraded state, partial subdirectory watch failure produces degraded state, permission-denied watch path appears in degraded messages, failed watch for newly created directory degrades without stopping safety refresh, watcher close/error after startup degrades state, manual refresh bypasses debounce.
  - Ordering assertions: overlapping debounced fsnotify refresh, safety refresh, and manual refresh results use monotonic generations; stale debounced parse results cannot overwrite newer manual refresh results; older parse results cannot resurrect a deleted or renamed session.
  - Run: `go test ./internal/refresh`.
  - Expected: fails before refresh implementation.

- [x] **Step 5.2: Implement watcher and refresh coordinator**
  - Create: `internal/refresh/watcher.go`, `internal/refresh/refresh.go`.
  - Behavior: watcher emits Bubbletea messages; it does not mutate app state directly.
  - Run: `go test ./internal/refresh && go test ./internal/tui`.
  - Expected: pass.

- [x] **Step 5.3: Integrate refresh into TUI**
  - Modify: `internal/tui/model.go`, `internal/tui/update.go`, `internal/tui/view.go`.
  - Behavior: header shows `Watcher: ok`, full degradation, or partial degradation with short reason; safety refresh remains active when watcher degrades; Session Workspace manual refresh reparses selected session through a fresh generation.
  - Verification: TUI render tests include watcher degraded, partial watcher degraded, post-start watcher failure, deleted session, new session, stale refresh ignored, no projects directory, and no sessions found states. JSON output includes the same degraded refresh state.
