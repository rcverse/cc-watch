# Decisions

Durable technical decisions and invariants for cc-watch. Distilled from the
original v1-to-v2 migration ADRs and PRD; the phase-by-phase build history
was dropped once v2 shipped and installed. If you're changing behavior in
one of these areas, read the relevant section first.

## Module & distribution

- Module path: `github.com/richardchen/cc-watch`.
- Local binary install (`install.sh`) is the only supported distribution
  path. No Homebrew tap, GitHub Releases, or goreleaser without explicit
  approval first.
- macOS only. No Linux/Windows target unless separately approved.

## JSON output contract (`--json`)

- The top-level object is versioned: `schema_version: 1`. Field additions
  are fine; changing the meaning of an existing field requires a version
  bump.
- Error codes are a closed set: `projects_dir_missing`, `no_sessions_found`,
  `session_not_found`, `ambiguous_session_id`, `parse_error`,
  `config_error`.
- Never emit ANSI escape sequences in JSON output.
- See `internal/jsonout/json.go` for the current field-level shape.

## KeepAlive subprocess safety

- KeepAlive is the only feature allowed to invoke the `claude` CLI. It runs
  `claude -r <session-id> -p <message>` via an explicit argv — never shell
  interpolation.
- Subprocess timeout: 30s. Confirmation window after the subprocess exits:
  20s, tied to a new JSONL entry timestamped after send initiation. No new
  entry means no confirmed success.
- Each send attempt carries an instance token so stale async results (after
  a cancel or a newer instance) are ignored.
- Failure classification: exit status/output containing `limit`,
  `rate limit`, `usage limit`, `quota`, or `too many requests`
  (case-insensitive) → `claude_limit`; missing executable →
  `claude_unavailable`; any other non-zero result → `subprocess_failed`.
  Any of these disables Auto-send for that session until the user
  re-enables it.
- Scope counts a send at initiation, not at confirmation, so a failed
  confirmation can't trigger a retry loop.
- **No production code or test may perform a real Claude send.** Tests use
  fake runners and fixture HOME/session files. This rule is non-negotiable.

## KeepAlive trigger timing

```
effective_trigger_s   = min(trigger_before_expiry_m * 60, ttl_seconds * 0.20)
effective_countdown_s = min(countdown_s, effective_trigger_s - 30)   # 30s safety margin
```

If the safety margin can't be preserved, Auto-send is disabled for that
instance and the UI falls back to a manual prompt. With defaults, a 1-hour
cache triggers at 5 minutes remaining; a 5-minute cache triggers at 1 minute
remaining.

## Refresh architecture

- Three refresh paths: fsnotify events, a 300ms debounce after event bursts,
  and a 30s internal safety refresh. The safety interval is not
  configurable and not a public flag — there is deliberately no
  `--watch`/`--interval` flag.
- Every refresh gets a monotonic generation; a stale (older-generation)
  result can't overwrite a newer parse result, delete, or rename.
- Watcher failures are a degraded state, not a crash — safety refresh keeps
  working even if fsnotify setup partially fails.

## Parser rules (don't casually change)

- `ephemeral_1h_input_tokens > 0` → 1h tier; else
  `ephemeral_5m_input_tokens > 0` → 5m tier; else unknown (treated as 5m
  for gap analysis).
- Hit rate = `cache_read / (cache_read + cache_create)`.
- A gap is only recorded above 60s between consecutive timestamps; it's a
  "reset" if the gap exceeds the effective TTL.

## Non-goals (still true, don't add unprompted)

No native macOS app, no background daemon, no public watch mode, no
unbounded/hidden KeepAlive loop, no direct Anthropic API calls, no
mouse-first interaction requirement.

## Historical note

v1 (`cc_watch.py`) was kept live at the installed command path throughout
the v2 migration as a rollback safety net, archived in full at
`archive/v1/`. The switchover to the Go binary happened on 2026-07-02; v1
is fully retired from the installed command path, but the archive remains
for reference.
