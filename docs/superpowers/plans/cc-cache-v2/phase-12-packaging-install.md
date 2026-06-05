# cc-cache v2 Phase 12: Packaging Install

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 12.

## Phase 12: Packaging, Install, And Release Path

**Purpose:** Produce a locally installable binary without breaking existing local usage. Public GitHub/Homebrew distribution is deferred until after local v2 verification.

- [ ] **Step 12.1: Build local binary artifact**
  - Create: `dist/cc-cache`.
  - Run: `go build -o dist/cc-cache ./cmd/cc-cache`.
  - Verification: `dist/cc-cache --version`, `dist/cc-cache --help`, and `HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-cache --json` succeed.

- [ ] **Step 12.2: Add build artifact ignores**
  - Modify: `.gitignore`.
  - Add: `dist/`, coverage outputs, local binary outputs, while keeping `internal/session/testdata/*.jsonl` tracked.
  - Verification: `git status --short` does not hide source files or fixtures.

- [ ] **Step 12.3: Update installer after binary verification**
  - Modify: `install.sh`.
  - Behavior: build or locate the Go binary, install it as `$HOME/.local/bin/cc-cache`, and avoid deleting the v1 archive.
  - Safety: before install, print current command target and ask for confirmation if replacing a symlink to `cc_cache.py`.
  - Verification before switch: `go test ./...`, `go build -o dist/cc-cache ./cmd/cc-cache`, `dist/cc-cache --version`, `dist/cc-cache --json`.
  - Verification after switch with user approval: `command -v cc-cache`, `cc-cache --version`, `HOME="$PWD/internal/session/testdata/smoke-home" cc-cache --json`, and `archive/v1/cc_cache.py --help`.
  - Rollback verification with separate user approval: run `archive/v1/install-v1.sh`, verify `ls -l "$HOME/.local/bin/cc-cache"` points to `archive/v1/cc_cache.py`, run `cc-cache --help` and confirm v1 help, then rerun the v2 install path and confirm `cc-cache --version` reports v2.

- [ ] **Step 12.4: Defer public release configuration**
  - Files: none by default.
  - Decision: do not create or maintain a separate Homebrew tap repository during v2 implementation.
  - Verification: README explains local binary install first and says GitHub Releases/Homebrew are deferred.

- [ ] **Step 12.5: Optional release dry-run after local stability**
  - Create: `.goreleaser.yaml` only after the user asks for public release preparation.
  - Targets when enabled: macOS arm64, macOS amd64, Linux amd64, Linux arm64.
  - Run when enabled: `goreleaser release --snapshot --clean`.
  - Expected when enabled: snapshot artifacts for all target OS/arch combinations.
  - Gate: do not publish a GitHub Release or Homebrew tap update until user explicitly approves release publishing.
