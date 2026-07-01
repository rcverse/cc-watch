# cc-watch

Know when your Claude Code cache is alive, fading, or gone.

`cc-watch` is a local macOS TUI for Claude Code sessions. It watches cache
status, can ping you before expiry, and can run a bounded KeepAlive workflow
when you ask it to.

It does three things:

- **Watch cache status** across recent Claude Code sessions.
- **Set reminders** before a useful cache window expires.
- **Keep a session alive** with a visible, limited, cancellable workflow.

It reads Claude Code JSONL files from `~/.claude/projects` and never writes to
those session files.

## Install

Preview the install:

```bash
./install.sh --dry-run
```

Install locally:

```bash
./install.sh --yes
```

The installer builds the Go binary and copies it to:

```text
$HOME/.local/bin/cc-watch
```

Make sure `$HOME/.local/bin` is on your `PATH`.

## Quick Start

```bash
cc-watch
```

That opens the session list.

Common commands:

```bash
cc-watch --id d4b247b7       # open one session; partial UUIDs work
cc-watch --n 10              # load 10 recent sessions
cc-watch --remind            # start with Reminder enabled
cc-watch config              # edit defaults
cc-watch --json              # print one JSON snapshot
cc-watch --json --id d4b247b7
```

There is no public `--watch` flag. The TUI refreshes while it is open.

## What It Shows

The List View shows recent sessions with:

- active, expired, or unknown cache status
- time left or time since expiry
- detected cache tier, for example `1-hour cache`
- cache progress
- token cache writes, reads, and hit rate
- first and last user-message excerpts
- Reminder and KeepAlive state

Open a session to see the Workspace: full session ID, cache timing, message
excerpts, token stats, gap summary, and controls.

Expired sessions show Reminder and KeepAlive as `N/A after expiry`. Dead cache
is dead cache; the tool does not pretend otherwise.

## Reminder

Reminder is an alarm, not automation.

It sends local macOS notifications when a session reaches configured
percent-remaining thresholds. It never sends a Claude message.

Default thresholds:

```json
"reminder_thresholds": [20, 10]
```

That means Reminder can notify near 20% and 10% remaining.

## KeepAlive

KeepAlive is the careful part.

It can send a short configured message to a resumable Claude Code session before
cache expiry. It is visible, limited, cancellable, and confirmed through the
session's JSONL file.

Default message:

```text
Keep-alive check. Reply "yes" only.
```

Default behavior:

- arm near the final 5 minutes of a cache window
- show a 30-second countdown before Auto-send
- allow one send per session
- turn Auto-send off after failures

## How KeepAlive Works

KeepAlive works because Claude Code can resume a session by ID and because each
session writes JSONL evidence when it changes.

The flow:

1. `cc-watch` watches the selected session's cache timing.
2. When the session enters the configured trigger window, KeepAlive arms one
   instance for that session.
3. If Auto-send is enabled and timing is still safe, the TUI starts a visible
   countdown.
4. During the countdown you can send now, cancel, or let it finish.
5. If sending proceeds, `cc-watch` runs:

   ```bash
   claude -r <session-id> -p <configured-message>
   ```

6. After the send starts, `cc-watch` watches that same session JSONL for new
   evidence.
7. New target-session evidence means the KeepAlive attempt is confirmed.
8. Missing `claude`, subprocess failure, timeout, expiry, or exhausted scope is
   shown in the TUI.

Safety rules:

- KeepAlive is per session.
- Each attempt has an instance token, so stale async results are ignored.
- Auto-send is disabled when timing is too tight.
- Failures disable Auto-send for that session.
- Expired sessions cannot start Reminder or KeepAlive.
- `cc-watch` never writes to Claude Code JSONL files.

## CLI Reference

```text
Usage: cc-watch [--n N] [--id <partial-id>] [--json] [--remind]
       cc-watch config
       cc-watch --help
       cc-watch --version
```

| Command | Meaning |
|---|---|
| `cc-watch` | Open the List View. |
| `cc-watch --n 10` | Load 10 recent sessions. |
| `cc-watch --id <partial-id>` | Open one matching session. |
| `cc-watch --remind` | Start with Reminder enabled for loaded sessions. |
| `cc-watch config` | Open the Config Editor. |
| `cc-watch --json` | Print one JSON snapshot and exit. |
| `cc-watch --json --id <id>` | Print JSON for one session and exit. |

## TUI Controls

The footer shows valid keys for the current screen. The common ones:

| Key | Action |
|---|---|
| `↑` / `↓` | Move selection or focus. |
| `←` / `→` | Previous or next page in List View. |
| `enter` | Open or activate the focused item. |
| `space` | Activate the focused control. |
| `r` | Toggle Reminder when available. |
| `k` | Toggle KeepAlive when available. |
| `u` | Refresh session data. |
| `v` | Show or hide detailed Workspace gaps. |
| `c` | Open Config from List View. |
| `s` | Save config. |
| `d` | Reset config defaults after confirmation. |
| `b` / `esc` | Back to the session list. |
| `q` | Quit. |

## Configuration

Config lives at:

```text
~/.config/cc-watch/config.json
```

Open the editor:

```bash
cc-watch config
```

Default config:

```json
{
  "reminder_thresholds": [20, 10],
  "keep_alive": {
    "trigger_before_expiry_m": 5,
    "countdown_s": 30,
    "message": "Keep-alive check. Reply \"yes\" only.",
    "auto_send": true,
    "scope": {
      "mode": "max_sends",
      "max_sends": 1
    }
  }
}
```

Config fields:

| Field | Meaning |
|---|---|
| `reminder_thresholds` | Percent-remaining points for Reminder notifications. |
| `keep_alive.trigger_before_expiry_m` | Minutes before expiry when KeepAlive may arm. |
| `keep_alive.countdown_s` | Visible countdown before Auto-send. |
| `keep_alive.message` | Message sent through `claude -r ... -p ...`. |
| `keep_alive.auto_send` | If `false`, KeepAlive prompts manually and never sends on its own. |
| `keep_alive.scope.max_sends` | Maximum KeepAlive sends per session. |

Keep the KeepAlive message short. You are trying to preserve a cache window, not
start a philosophical detour.

## JSON Output

`--json` prints one snapshot and exits. It does not start the TUI,
notifications, or KeepAlive automation.

The output includes:

- `schema_version`
- `generated_at`
- `query`
- `config`
- `refresh`
- `notifications`
- `sessions`
- `selected_session`
- `error`

Use JSON mode for scripts, dashboards, or checks where a TUI would be awkward.

## Local Behavior And Privacy

`cc-watch` is local-first:

- reads `~/.claude/projects/**/*.jsonl`
- writes `~/.config/cc-watch/config.json`
- sends macOS notifications through `osascript`
- may run `claude -r ... -p ...` only through explicit KeepAlive behavior
- does not call Anthropic APIs directly
- does not run as a daemon

## Development

Requirements:

- macOS
- Go 1.23 or newer

Useful checks:

```bash
go test ./...
scripts/test-install.sh
go run ./cmd/cc-watch --help
go run ./cmd/cc-watch --json
go run ./cmd/cc-watch
```

Use fixture homes for tests and smoke checks. Do not run a real KeepAlive send
as part of verification.

## Project Layout

```text
cmd/cc-watch/       Go CLI entry point
internal/           parser, snapshot, TUI, JSON, config, refresh, notify, KeepAlive
archive/v1/         preserved Python v1 rollback material
docs/               ADRs, specs, plans, progress, prompt bank
install.sh          local macOS installer
PRD.md              product requirements
```
