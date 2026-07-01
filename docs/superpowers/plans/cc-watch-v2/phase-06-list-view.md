# cc-watch v2 Phase 6: List View

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 6.

## Phase 6: List View

**Purpose:** Build the scan-first session triage surface.

- [x] **Step 6.1: Write failing List View render/update tests**
  - Create or extend: `internal/tui/render_test.go`, `internal/tui/update_test.go`.
  - Assertions: rows sort by JSONL modification time, required priority fields render at narrow/medium/wide widths, excerpts truncate before action-critical columns, `enter` opens selected session when a row is focused, Reminder action is focusable and `enter` toggles it, KeepAlive action is focusable and `enter` toggles it, `r` toggles Reminder as accelerator, `k` toggles KeepAlive as accelerator, refresh/help/quit are cursor reachable, `k` is not down navigation.
  - Run: `go test ./internal/tui`.
  - Expected: fails before List View implementation.

- [x] **Step 6.2: Implement List View**
  - Create: `internal/tui/list.go`.
  - Required states: loading, missing projects directory, no sessions found, ambiguous partial ID selection, parse warnings, watcher degraded, partial watcher degraded, notification degraded, `claude` unavailable when KeepAlive is armed or about to send.
  - Run: `go test ./internal/tui`.
  - Expected: pass.

- [x] **Step 6.3: Manual List View smoke**
  - Run: `go run ./cmd/cc-watch`.
  - Expected: terminal opens List View or first-run empty state; arrow keys move selection; `?` help works; `q` exits; no KeepAlive subprocess starts.
