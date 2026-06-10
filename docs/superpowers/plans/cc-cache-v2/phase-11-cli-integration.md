# cc-cache v2 Phase 11: Cli Integration

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 11.

## Phase 11: CLI Integration And Full Non-Release Verification

**Purpose:** Exercise the implemented product through its public CLI contract before packaging.

- [x] **Step 11.1: CLI command tests**
  - Create or extend: `internal/app` tests.
  - Assertions: `cc-cache`, `--n N`, `--id <partial-id>`, `--json`, `--json --id <id>`, `--remind`, `config`, `--help`, `--version`, and rejected `--watch`.
  - Run: `go test ./internal/app ./...`.
  - Expected: pass.

- [x] **Step 11.2: JSON command smoke**
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" go run ./cmd/cc-cache --json`.
  - Expected: valid JSON to stdout and process exits without launching TUI.
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" go run ./cmd/cc-cache --json --id 11111111`.
  - Expected: selected-session JSON matching `schema_version: 1` and no `error`.
  - Run: `HOME="$PWD/internal/session/testdata/smoke-home" go run ./cmd/cc-cache --json --id no-such-session`.
  - Expected: JSON error object with `code: "session_not_found"` and non-zero exit.

- [x] **Step 11.3: TUI degraded-state smoke**
  - Run with a temporary `HOME` containing no `.claude/projects`.
  - Expected: first-run empty state names exact discovery path and no crash.
  - Run with notifier command unavailable in a controlled environment.
  - Expected: Notify degraded visible; event state still updates.

- [x] **Step 11.4: KeepAlive no-real-send integration**
  - Run tests with fake Claude runner and fake confirmation watcher.
  - Expected: countdown, manual prompt, failure, success, and scope-complete paths pass without invoking real `claude`.
  - Gate: real Claude send testing is excluded from v2 implementation verification. If it is ever desired later, it must be a separate user-approved task outside this plan.
