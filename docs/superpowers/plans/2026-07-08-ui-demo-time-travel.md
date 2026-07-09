# UI Demo Time Travel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a non-distributable interactive UI demo that runs the real TUI renderer/state machine against fake sessions and a fake clock.

**Architecture:** Create a separate `tools/ui-demo` command behind `//go:build demo`. The tool constructs `tui.Options` fixtures, wraps the real `tui.Model` only to intercept demo navigation/time keys, and delegates normal rendering and updates to the real model.

**Tech Stack:** Go, Bubble Tea, existing `internal/tui`, `internal/session`, `internal/keepalive`, and `internal/config`.

## Global Constraints

- No production CLI flag or behavior.
- No reads from or writes to `~/.claude`.
- No real `claude` subprocess.
- No real notifications.
- No config writes.
- Normal `go test ./...` must not include the demo tool.
- Demo is run only with `go run -tags demo ./tools/ui-demo`.

---

### Task 1: Build-Tagged Demo Tool

**Files:**
- Create: `tools/ui-demo/main.go`
- Test: `tools/ui-demo/main_test.go`

**Interfaces:**
- Consumes: `tui.NewModel(options tui.Options) tui.Model`
- Produces: `go run -tags demo ./tools/ui-demo` interactive TUI demo

- [ ] **Step 1: Write failing tests**

Create tests that assert:
- the demo tool is excluded without the `demo` tag;
- the demo binary builds with `-tags demo`;
- demo fixtures render real TUI routes and KA state text.

- [ ] **Step 2: Verify tests fail**

Run: `env GOCACHE=/private/tmp/cc-watch-gocache go test ./tools/ui-demo`

Expected: fail before implementation.

- [ ] **Step 3: Implement minimal demo command**

Create `tools/ui-demo/main.go` with:
- `//go:build demo`;
- fake sessions and config;
- fake KeepAlive runner and confirmation;
- Bubble Tea model wrapper that intercepts only demo keys:
  - `1` list
  - `2` workspace
  - `3` ambiguous
  - `4` config
  - `j` jump to KA trigger
  - `J` jump to expiry
  - `.` advance 5s
  - `,` rewind 5s
- footer/banner text explaining demo controls.

- [ ] **Step 4: Verify**

Run:
- `env GOCACHE=/private/tmp/cc-watch-gocache go test ./tools/ui-demo`
- `env GOCACHE=/private/tmp/cc-watch-gocache go test -tags demo ./tools/ui-demo`
- `env GOCACHE=/private/tmp/cc-watch-gocache go test ./...`
- `env GOCACHE=/private/tmp/cc-watch-gocache go test -tags demo ./...`

Expected: all pass.

## Self-Audit

- Spec coverage: covers non-distributable code, real renderer/state machine, fake fixture inputs, fake clock jumps, and no real side effects.
- Placeholder scan: no deferred implementation placeholders.
- Type consistency: uses existing exported `tui.Options`, `tui.Model`, `session.Session`, `keepalive.SessionState`, and Bubble Tea `tea.Model`/`tea.Cmd`.
