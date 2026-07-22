# Decisions

Durable technical decisions and invariants for cc-watch. Distilled from the
original v1-to-v2 migration ADRs and PRD; the phase-by-phase build history
was dropped once v2 became the installed local command. If you're changing
behavior in one of these areas, read the relevant section first.

## Module & distribution

- Module path: `github.com/rcverse/cc-watch`.
- The supported user-facing distribution path is the
  `rcverse/homebrew-cc-watch` tap. GitHub Release archives are the artifacts
  consumed by the tap and remain available for manual recovery.
  `install.sh` is a contributor-only source-build installer. Release binaries
  target macOS `arm64` and `amd64`. The project does not use goreleaser.
- macOS only. No Linux/Windows target unless separately approved.
- cc-watch is beta and local-first. Removed internal surfaces do not
  need compatibility shims by default. Delete stale flags, config knobs,
  tests, and docs unless the user explicitly asks for a migration path.

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
  Any of these pauses that KeepAlive instance until the user resets or
  re-enables it.
- The send limit counts a send at initiation, not at confirmation, so a
  failed confirmation can't trigger a retry loop.
- **The send runs from the session's own project directory** (`cmd.Dir` =
  the `cwd` parsed from the transcript). `claude --resume` lookup is scoped to
  the current directory, so running from anywhere else fails with "No
  conversation found with session ID". An unparsed cwd falls back to
  inheriting cc-watch's own cwd.
- **Every send/confirm is appended to `~/.config/cc-watch/keepalive.log`**
  (slog JSONL: cwd, exit, classification, truncated stdout/stderr, confirm
  outcome). It's the only durable record — in-memory state is overwritten on
  the next refresh. Bounded to ~2 MiB via single-backup rotation at startup;
  disable with `CC_WATCH_KEEPALIVE_LOG=off`.
- **No production code or test may perform a real Claude send.** Tests use
  fake runners and fixture HOME/session files. This rule is non-negotiable.

## KeepAlive trigger timing

```
effective_trigger_s   = min(trigger_before_expiry_m * 60, ttl_seconds * 0.20)
effective_countdown_s = min(countdown_s, effective_trigger_s - 30)   # 30s countdown-sizing margin
latest_safe_send_at   = cache_anchor_at + ttl_seconds - 10           # 10s hard-stop margin
```

If the countdown-sizing margin can't be preserved, the KeepAlive instance is
paused rather than sending too close to expiry. With defaults, a 1-hour cache
triggers at 5 minutes remaining; a 5-minute cache triggers at 1 minute
remaining.

The countdown is sized to finish ~30s before expiry, but the hard send
deadline (`latest_safe_send_at`) uses a **smaller** 10s margin on purpose. The
countdown is tick-counted, so it always elapses a beat after its nominal
duration; if the deadline used the same 30s margin it would coincide with the
countdown's own end and any drift would silently pause sending — which made
KeepAlive effectively never fire for 5-minute caches (their countdown ends
right at the 30s mark). The 10s hard-stop absorbs normal drift
while still refusing to send within seconds of expiry.

## Refresh architecture

- Three refresh paths: fsnotify events, a 300ms debounce after event bursts,
  and a 30s internal safety refresh. The safety interval is not
  configurable and not a public flag — there is deliberately no
  `--watch`/`--interval` flag.
- Every refresh gets a monotonic generation; a stale (older-generation)
  result can't overwrite a newer parse result, delete, or rename.
- Watcher failures are a degraded state, not a crash — safety refresh keeps
  working even if fsnotify setup partially fails.

## Watchlist and list ordering

- Pinned session IDs and enabled Reminder session IDs persist in
  `~/.config/cc-watch/config.json`. Reminder execution remains foreground-only;
  persistence restores intent but does not create a daemon.
- List snapshots parse the configured number of recent sessions plus any older
  pinned or reminded sessions. Discovery already walks session metadata before
  applying its limit, so this does not add a second filesystem traversal.
- Recent order remains file-modification order. Attention order is an in-memory
  view: active sessions by least time remaining, unknown sessions, then expired
  sessions. Sorting runs when the mode changes or sessions refresh, not every
  display tick, and selection is restored by session ID.
- Missing watched-session files are ignored. cc-watch does not rewrite or
  automatically prune watchlist entries when a transcript disappears.

## UI demo harness

- Rare TUI states are exercised through `tools/ui-demo`, gated behind the
  `demo` build tag. The harness may expose fake time travel and fixtures, but
  it must render through the real TUI model.
- Demo fixtures must never touch `~/.claude`, send real notifications, write
  config, or spawn `claude`. The fixture layer is allowed to fake inputs only;
  production rendering and state transitions stay load-bearing.
- `go test ./...` should remain free of demo-only code. Use
  `go test -tags demo ./...` when validating demo routes.

## statusline subprocess safety and rate-limit estimate

- `cc-watch statusline` is the only other feature (besides KeepAlive)
  allowed to spawn a subprocess — the user's own existing statusline
  command, given via `cc-watch statusline -- <command> [args...]`. Argv
  only, via `exec.CommandContext`, never a shell.
- Subprocess timeout: 5s (short — this hook runs on every turn and must
  not visibly stall Claude Code's UI, unlike KeepAlive's rare, 30s,
  user-visible send).
- The wrapped command's **stderr is relayed directly**, never captured —
  a wrapped command may depend on stderr passthrough for its own
  progress/warnings. Only stdout is captured.
- Exactly one trailing newline is trimmed from the wrapped command's
  stdout before cc-watch's own segment is appended on the configured same line
  or new line; everything else is handled as raw bytes, never string-normalized
  (could mangle ANSI sequences the wrapped command emitted).
- On a non-clean wrapped-command exit (nonzero, spawn error, or timeout),
  cc-watch relays whatever partial stdout it produced and appends
  **nothing** — never risk turning a truncated line into a garbled
  combined one. `cc-watch statusline` (and its wrapping) **always exits
  0** regardless: this is invisible plumbing riding along the user's real
  statusline, and a non-zero exit or eaten output would visibly break
  their setup on any transient hiccup, which is worse than a stale or
  missing readout for one turn.
- Rate-limit state persists at `~/.config/cc-watch/ratelimit.json`
  (`internal/ratelimit`), same plain `json.MarshalIndent` +
  `os.WriteFile` pattern as `internal/config`. It self-heals every turn
  (rebuilt from the next hook invocation regardless of prior contents),
  so there's no delete/`--clean` verb — `rm` the file if you ever need to.
- The config TUI may install or uninstall the Claude Code statusLine
  setting in `~/.claude/settings.json`. It only writes for unambiguous
  states, preserves existing non-statusLine settings, writes a timestamped
  backup first, and refuses unclear commands with manual-review copy. The
  installed command uses the current cc-watch executable path so Claude Code
  does not need to inherit the user's shell `PATH`; reinstall repairs an older
  path-dependent wrapper.
  `cc-watch statusline --check` remains read-only.
- The cc-watch config stores three independent statusline elements — `usage`,
  `warning`, and `cache` — each with `enabled`, `layout`, and `format`, plus an
  explicit `order`. Usage and cache support `full`/`compact`; the warning uses
  `alert_only`/`verbose`. Install state remains derived from Claude Code's
  settings, and the legacy CLI flags override the Usage element for one
  invocation.
- When the cache element is enabled, installation adds Claude Code's
  `refreshInterval: 1` only when the user has not already chosen an interval.
  The hook reuses parsed cache timing while the transcript mtime is unchanged,
  so a one-second countdown does not trigger a full JSONL scan every second.
- Both Claude Code account windows matter for KeepAlive availability. The
  statusline displays both `five_hour` and `seven_day` when Claude provides
  them, and `KeepAlive at risk` means at least one account window may run
  out before enough future KeepAlive sends can happen.
- **Momentum is a conservative estimate, never a fact.** `pctPerMessage`
  is the average of the last few consecutive reading-to-reading deltas in
  the account's overall `used_percentage` — not isolated to KeepAlive's
  own (cheaper) cost, deliberately: an earlier draft that tried to isolate
  KA-specific cost via transcript-text matching was structurally broken,
  since `rate_limits` is account-wide but the transcript checked is
  per-session. Below a small delta epsilon (~0.5%, e.g. an idle account),
  momentum reports `unknown`, never a confidently-wrong large
  `messagesLeft` — this is the load-bearing safety property of the whole
  feature.
- On a rate-limit window rollover (a new `resets_at` differs from the
  last stored reading), the reading history is cleared — momentum across
  a reset boundary is meaningless.
- Cache timing is anchored only by a timestamped assistant response with
  positive output usage and cache-token evidence. User messages, local
  commands, tool-only events, and explicit error events do not advance the
  anchor. The latest accepted response determines the 1h/5m tier; a cache
  read/write without new tier evidence retains the previous tier until an
  invalidation is observed. `/compact` starts a new cache lineage;
  `/model` and `/reload-plugins` may change the cached prefix, so they clear the
  anchor conservatively. `/effort` changes the cache key, so it also clears the
  anchor conservatively. Unknown timing is never treated as an active cache for
  KeepAlive or reminders.
- Statusline snapshots, including unknown timing, are cached per
  `transcript_path` while the transcript mtime is unchanged. This avoids
  re-scanning a long JSONL file on every one-second statusline refresh.

## Parser rules (don't casually change)

- `ephemeral_1h_input_tokens > 0` on an accepted response → 1h tier; else
  `ephemeral_5m_input_tokens > 0` → 5m tier; else retain the last known tier
  for the same cache sequence or remain unknown.
- Hit rate = `cache_read / (cache_read + cache_create)`.
- A gap is only recorded above 60s between consecutive timestamps; it's a
  "reset" if the gap exceeds the effective TTL.

## Non-goals (still true, don't add unprompted)

No native macOS app, no background daemon, no public watch mode, no
machine-readable JSON mode, no unbounded/hidden KeepAlive loop, no direct
Anthropic API calls, no mouse-first interaction requirement.

## Historical note

v1 (`cc_watch.py`) was kept live at the installed command path throughout
the v2 migration as a rollback safety net, archived in full at
`archive/v1/`. The switchover to the Go binary happened on 2026-07-02; v1
is fully retired from the installed command path, but the archive remains
for reference.
