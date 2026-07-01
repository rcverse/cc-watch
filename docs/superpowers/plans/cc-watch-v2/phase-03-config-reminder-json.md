# cc-watch v2 Phase 3: Config Reminder Json

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 3.

## Phase 3: Config, Reminder Core, JSON Contract, And CLI Non-TUI Behavior

**Purpose:** Build stable non-interactive foundations before Bubbletea surfaces.

- [x] **Step 3.1: Write failing config tests**
  - Create: `internal/config/config_test.go`.
  - Assertions: default config exactly matches PRD/design values, config loads from `~/.config/cc-watch/config.json`, invalid JSON falls back with visible warning state, save writes only valid config, reset restores defaults, cancel path performs no write, active per-session state is not mutated by config changes.
  - Run: `go test ./internal/config`.
  - Expected: fails before config implementation.

- [x] **Step 3.2: Implement config model, store, and validation**
  - Create: `internal/config/model.go`, `internal/config/store.go`, `internal/config/validation.go`.
  - Validation: reminder thresholds whole numbers 1-99 in descending order; trigger positive; countdown positive; max sends positive; countdown plus safety margin summary warns when affected sessions will disable auto-send.
  - Run: `go test ./internal/config`.
  - Expected: pass.

- [x] **Step 3.3: Write failing reminder tests**
  - Create: `internal/reminder/reminder_test.go`.
  - Assertions: per-session runtime toggle, thresholds from global config, fires once per threshold crossing per active session instance, `--remind` enables loaded sessions only, Reminder never starts KeepAlive and never invokes Claude subprocess behavior.
  - Run: `go test ./internal/reminder`.
  - Expected: fails before reminder implementation.

- [x] **Step 3.4: Implement reminder core**
  - Create: `internal/reminder/reminder.go`.
  - Behavior: runtime state is in-memory and discarded at process exit.
  - Run: `go test ./internal/reminder`.
  - Expected: pass.

- [x] **Step 3.5: Write failing JSON output tests**
  - Create: `internal/jsonout/json_test.go`.
  - Assertions: `schema_version` equals `1`, generated timestamp, top-level success shape, session object shape, selected-session output, parser warnings, refresh degraded state, notification degraded state, Reminder state when available, KeepAlive state when available, no ANSI output, allowed error codes, no-match error shape, ambiguous-ID error shape with candidate sessions, and stable field names from the `Public JSON Contract`.
  - Run: `go test ./internal/jsonout`.
  - Expected: fails before JSON output implementation.

- [x] **Step 3.6: Implement JSON output**
  - Create: `internal/jsonout/json.go`.
  - Modify: `internal/app/cli.go`, `internal/app/app.go`, `internal/app/cli_test.go`.
  - Behavior: `cc-watch --json` and `cc-watch --json --id <id>` print JSON and exit without starting Bubbletea, watchers, notifications, or KeepAlive subprocesses.
  - Test assertions: app-level fake dependencies prove `--json` does not call TUI startup, watcher startup, notifier startup, or KeepAlive runner creation.
  - Run: `go test ./internal/jsonout && go test ./internal/app`.
  - Expected: pass.
