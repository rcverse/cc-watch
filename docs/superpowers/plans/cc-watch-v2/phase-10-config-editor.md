# cc-watch v2 Phase 10: Config Editor

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 10.

## Phase 10: Config Editor

**Purpose:** Build global defaults editor without mutating active runtime session state.

- [x] **Step 10.1: Write failing Config Editor tests**
  - Extend: `internal/tui/render_test.go`, `internal/tui/update_test.go`.
  - Assertions: fields for reminder thresholds, trigger, countdown, message, auto-send, max sends; live 1-hour and 5-minute behavior summary; auto-send warning; validation errors next to fields and in summary; invalid config cannot save; `d` requires repeat confirmation; `esc` cancels with no write.
  - Run: `go test ./internal/tui ./internal/config`.
  - Expected: fails before editor implementation.

- [x] **Step 10.2: Implement Config Editor**
  - Create: `internal/tui/config_editor.go`.
  - Behavior: edits global defaults only; active per-session KeepAlive auto-send state is copied at session state creation and not silently changed by later config saves.
  - Run: `go test ./internal/tui ./internal/config`.
  - Expected: pass.

- [x] **Step 10.3: Manual config smoke**
  - Run: `go run ./cmd/cc-watch config`.
  - Expected: invalid values show validation; `esc` cancels; `d` requires repeat confirmation; save writes valid config only.
