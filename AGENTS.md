# AGENTS.md

cc-watch is a local, macOS-only Go TUI + JSON CLI that watches Claude Code's
session cache (`~/.claude/projects/**/*.jsonl`) so a user knows when a cache
window is active, fading, or expired. It also runs an optional Reminder
alarm and a bounded KeepAlive workflow. See `README.md` for the user-facing
pitch.

## Layout

- `cmd/cc-watch/` — entry point.
- `internal/session/` — JSONL discovery and parsing (cache tier, gaps,
  token stats).
- `internal/snapshot/` — assembles one point-in-time view for the TUI/JSON.
- `internal/refresh/` — fsnotify watcher, debounce, safety refresh.
- `internal/keepalive/` — the KeepAlive state machine and subprocess
  runner.
- `internal/ratelimit/` — account-wide 5-hour rate-limit tracking
  (momentum estimate, tier-TTL cache) for the `statusline` subcommand.
- `internal/notify/` — macOS `osascript` notifications.
- `internal/config/` — config file load/save/validate.
- `internal/jsonout/` — the `--json` schema.
- `internal/tui/` — the Bubbletea model (List / Workspace / Config routes).
- `internal/app/` — CLI arg parsing and mode dispatch.
- `archive/v1/` — retired Python v1, kept for reference/rollback only. Not
  load-bearing; don't treat it as source of truth for v2 behavior.

## Build, test, run

```bash
go build ./...
go vet ./...
go test ./...
go test ./... -race     # for anything touching internal/refresh or internal/keepalive concurrency
go run ./cmd/cc-watch --help
scripts/test-install.sh # exercises install.sh against a temp HOME, safe to run
```

## Glossary

- **Session Snapshot** — the parsed view of sessions plus config-derived
  defaults, consumed by JSON output, TUI startup, and TUI refresh.
- **Cache Status** — one session's active/expired/unknown state, TTL
  timing, and cache tier.
- **Refresh Runtime** — the internal TUI mechanism that updates snapshots
  from manual refresh, filesystem events, and the safety tick. Not a
  public watch mode.
- **JSON API** — the stable `--json` interface for scripts and future
  clients. Treat it as a public contract; see `docs/decisions.md`.
- **KeepAlive Runtime** — the bounded, visible, cancellable automation for
  optionally sending one configured Claude message to one selected
  session.
- **Route** — a TUI screen's local focus/render/action behavior (List,
  Workspace, Ambiguous, Config).

## Hard rules

- Never let a test or manual verification run a real KeepAlive send (never
  actually invoke `claude -r ... -p ...` against a real session). Use fake
  runners and fixture homes.
- Never write to `~/.claude/projects/**/*.jsonl` — read-only, always.
- `statusline` is the only feature besides KeepAlive allowed to spawn a
  subprocess, and only the user's own configured statusline command,
  argv-only (never a shell), bounded 5s timeout, always relays output and
  exits 0. It never writes `~/.claude/settings.json`.
- Don't add Linux/Windows support, a daemon, a public watch/interval flag,
  Homebrew/GitHub Release packaging, or direct Anthropic API calls without
  the user asking first — these are deliberate non-goals, not oversights.
- `cc-watch` (the Go binary) is the live installed command
  (`$HOME/.local/bin/cc-watch`, switched over from v1 on 2026-07-02).
  `cc_watch.py` / `archive/v1/` are historical only.

## Where to look for "why"

`docs/decisions.md` has the distilled rationale for KeepAlive subprocess
safety, JSON schema stability, refresh timing, and non-goals. Read it
before changing behavior in those areas.
