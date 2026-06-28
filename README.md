# cc-cache

> A macOS terminal TUI and JSON API for inspecting Claude Code session cache status, setting Reminder alarms, and running bounded KeepAlive workflows.

## Quick Start

```bash
cc-cache                # list recent sessions
cc-cache --n 10         # list N sessions
cc-cache --id d4b247b7  # inspect specific session (partial UUID OK)
cc-cache --remind       # start TUI with Reminder enabled for loaded sessions
cc-cache config         # edit Reminder and KeepAlive defaults
cc-cache --json         # stable machine-readable API
cc-cache --json --id d4b247b7
```

There is no public `--watch` mode. The running TUI live-refreshes internally from Claude session file changes, with manual and safety refresh fallbacks.

## Current Scope

- List recent Claude Code sessions and cache status.
- Show per-session details: timing, cache window, token/cache stats, message excerpts, gaps, IDs, and JSONL path.
- Live-refresh automatically while the TUI is running.
- Send Reminder alarms through native macOS notifications only.
- Run KeepAlive per session, manually or with Auto-send, through a visible bounded workflow.
- Expose stable `schema_version: 1` JSON output.
- Support simple local macOS install after the Go binary is verified.

Out of scope unless re-approved: Linux/Windows support, public `--watch`, background daemon, Homebrew/GitHub release publishing, and unbounded hidden automation.

## Development Run

```bash
go run ./cmd/cc-cache --help
go run ./cmd/cc-cache --json
go run ./cmd/cc-cache
```

Do not replace `$HOME/.local/bin/cc-cache` until the local install phase is explicitly approved. The legacy Python v1 entry point remains in the repo for rollback during migration.

## Source Of Truth

- Current product boundary: [docs/superpowers/specs/2026-06-18-cc-cache-v2-product-reality.md](docs/superpowers/specs/2026-06-18-cc-cache-v2-product-reality.md)
- Full product requirements: [PRD.md](PRD.md)
- Implementation progress: [docs/superpowers/progress/cc-cache-v2-progress.md](docs/superpowers/progress/cc-cache-v2-progress.md)

## Project layout

```
cc-cache/
├── cmd/cc-cache/       # Go CLI entry point
├── internal/           # parser, snapshot, TUI, JSON, config, refresh, notify, KeepAlive
├── archive/v1/         # preserved Python v1
├── cc_cache.py         # legacy v1 entry point during migration
├── docs/               # ADRs, specs, plans, progress
├── install.sh          # legacy installer until v2 local install is approved
└── PRD.md
```
