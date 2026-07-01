# cc-watch v2 Phase 1: Go Skeleton

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 1.

## Phase 1: Preserve v1 And Establish Go Project Skeleton

**Purpose:** Make v2 buildable without breaking the current command path.

- [x] **Step 1.0: Verify or bootstrap Go toolchain**
  - Files: none unless toolchain installation notes are added to `README.md` later in Phase 13.
  - Run: `go version`.
  - Expected if Go is present: Go 1.23 or newer.
  - If missing or older: ask the user before installing Go tooling. Recommended macOS path is Homebrew Go installation if Homebrew is available; otherwise use the official Go installer. Do not install tooling silently.
  - Verification after install: `go version` reports Go 1.23 or newer.

- [x] **Step 1.1: Archive v1 by copy**
  - Create: `archive/v1/README.md`, `archive/v1/cc_watch.py`, `archive/v1/install-v1.sh`.
  - Modify: none.
  - Verification: `cmp cc_watch.py archive/v1/cc_watch.py` passes immediately after copy; `cmp install.sh archive/v1/install-v1.sh` passes immediately after copy.
  - Gate: root `cc_watch.py` remains present and executable.

- [x] **Step 1.2: Verify v1 command path still works**
  - Run: `command -v cc-watch`.
  - Expected: `/Users/richardchen/.local/bin/cc-watch`.
  - Run: `ls -l "$HOME/.local/bin/cc-watch"`.
  - Expected before switchover: symlink points to `/Users/richardchen/Dev/cc-watch/cc_watch.py`.
  - Run: `cc-watch --help`.
  - Expected: v1 help exits successfully.

- [x] **Step 1.3: Initialize Go module**
  - Create: `go.mod`, `go.sum`.
  - Depends on: `docs/adr/2026-06-03-go-module-and-release-identity.md`.
  - Run: `go mod init github.com/richardchen/cc-watch`.
  - Add dependencies: Bubbletea, bubbles, lipgloss, fsnotify.
  - Verification: `go mod tidy` succeeds and `go test ./...` succeeds with no runtime packages yet or only skeleton packages.

- [x] **Step 1.4: Create CLI skeleton**
  - Create: `cmd/cc-watch/main.go`, `internal/app/cli.go`, `internal/app/app.go`, `internal/app/version.go`, `internal/app/cli_test.go`.
  - Behavior: support `--help`, `--version`, `--n`, `--id`, `--json`, `--remind`, and `config`; reject retired watch flags as unsupported.
  - Test assertions: help/version exit without session discovery; retired watch flags exit non-zero; `--json` dispatch is parsed as non-interactive but returns a clear "not wired yet" app error until Phase 3; `config` dispatch is parsed without starting Bubbletea until Phase 10.
  - Verification: `go test ./internal/app`, help/version commands, and a retired watch flag attempt produce expected output; retired watch flags exit non-zero with a clear message.
