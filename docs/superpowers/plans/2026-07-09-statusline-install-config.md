# Statusline Install Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make statusline a clear Claude Code integration: the config TUI can install/uninstall it, CLI help is transparent, and public `--remind` is removed.

**Architecture:** Add one small `internal/statusline` settings helper that both `internal/app` and `internal/tui` can use. Keep `cc-watch statusline` as the runtime hook and `--check` as read-only instructions; make TUI config own install/uninstall writes to `~/.claude/settings.json`.

**Tech Stack:** Go stdlib JSON/filesystem, existing Bubble Tea TUI model, existing Go tests.

## Global Constraints

- Do not add dependencies.
- Do not expose KeepAlive sends through CLI.
- User-facing copy uses `Install in Claude Code`, `Uninstall from Claude Code`, and `Needs manual review`; avoid `wrap`, `hook`, and `plugin`.
- Writes to `~/.claude/settings.json` must be reversible for safe states and refused for unclear states.
- `cc-watch statusline` runtime still always exits 0.

---

### Task 1: Shared Statusline Settings Helper

**Files:**
- Create: `internal/statusline/settings.go`
- Test: `internal/statusline/settings_test.go`
- Modify: `internal/app/statusline_hook.go`

**Interfaces:**
- Produces: `statusline.Inspect(home string) (Status, error)`, `statusline.Install(home string) (Status, error)`, `statusline.Uninstall(home string) (Status, error)`, `statusline.SettingsSnippet(command string) string`, `statusline.NeedsShell(command string) bool`, `statusline.ShellQuote(s string) string`.
- Consumes: existing `~/.claude/settings.json` shape and existing statusline command detection.

- [ ] **Step 1: Write failing tests for inspect/install/uninstall**

Test safe states: no settings, existing command, installed bare, installed with previous command, malformed JSON, ambiguous cc-watch command.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./internal/statusline`
Expected: fail because package/functions do not exist.

- [ ] **Step 3: Implement minimal helper**

Implement JSON read/write preserving unknown top-level settings, detection states, install/uninstall, and timestamped backup before writes.

- [ ] **Step 4: Update app `--check` to use helper**

Keep output equivalent, but remove duplicated JSON/settings helpers from `internal/app/statusline_hook.go`.

- [ ] **Step 5: Verify**

Run: `go test ./internal/statusline ./internal/app`.

### Task 2: Config TUI Statusline Panel

**Files:**
- Modify: `internal/tui/config_editor.go`
- Modify: `internal/tui/route_actions.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/render_test.go`, `internal/tui/update_config_test.go`

**Interfaces:**
- Consumes: `statusline.Inspect`, `statusline.Install`, `statusline.Uninstall`.
- Produces: one `Statusline` panel with state + details and one action row whose label changes by state.

- [ ] **Step 1: Write failing render/update tests**

Render should show:
`Statusline`, `State`, `Not installed`, `Install in Claude Code`.
Action tests should verify install/uninstall calls change settings and ambiguous state refuses writes.

- [ ] **Step 2: Run focused TUI tests and verify failure**

Run: `go test ./internal/tui`.

- [ ] **Step 3: Implement minimal panel**

Add one focus action `config_statusline_action`. Labels:
`Install in Claude Code`, `Uninstall from Claude Code`, or `Show instructions`.

- [ ] **Step 4: Verify**

Run: `go test ./internal/tui`.

### Task 3: CLI and Help Cleanup

**Files:**
- Modify: `internal/app/cli.go`
- Modify: `internal/app/cli_test.go`
- Modify: `README.md`
- Modify: `docs/decisions.md`

**Interfaces:**
- Removes: public `--remind`.
- Adds: `cc-watch statusline --help`.

- [ ] **Step 1: Write failing parser/help tests**

Assert `--remind` is rejected, `statusline --help` renders statusline help, and top-level help uses the new TUI/statusline wording.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./internal/app`.

- [ ] **Step 3: Implement parser/help changes**

Remove `Remind` from public args and help. Keep internal snapshot/TUI reminder controls unchanged.

- [ ] **Step 4: Verify**

Run: `go test ./internal/app`.

### Task 4: Final Verification

**Files:**
- All touched files.

- [ ] **Step 1: Run full tests**

Run: `go test ./...`

- [ ] **Step 2: Smoke commands**

Run:
`go run ./cmd/cc-watch --help`
`go run ./cmd/cc-watch statusline --help`
`HOME=$(mktemp -d) go run ./cmd/cc-watch statusline --check`

- [ ] **Step 3: Review diff**

Run: `git diff --stat` and `git diff --check`.

## Self-Audit

- Spec coverage: covers CLI cleanup, TUI install/uninstall, reversible writes, manual-review refusal, and docs.
- Placeholder scan: no TBD/TODO placeholders.
- Type consistency: `internal/statusline` owns settings inspection/writes; app and TUI consume it without import cycles.
