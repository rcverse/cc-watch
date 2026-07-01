# cc-watch

**Know when your Claude Code cache is alive, fading, or already a ghost.**

Claude Code's prompt cache is either a 1-hour perk or a 5-minute default, and
either way it expires in total silence, taking your token savings with it.
`cc-watch` is a small terminal app that watches the clock so you don't have to
keep refreshing a session and squinting at timestamps.

It's one Go binary. No daemon, no server, no login, no subscription, no
dashboard you'll forget exists in a week. It reads a folder of JSONL files and
tells you the truth about them.

## What it does

- **Watches cache status** across your recent Claude Code sessions — active,
  expired, or unknown, with time remaining and a progress bar.
- **Reminds you** before a cache window expires, via a local notification.
  It's an alarm. It never sends anything on your behalf.
- **Keeps a session alive**, if you opt in — a visible, cancellable,
  bounded workflow that can nudge a session before it goes cold.

That's it. Three jobs, done well, in a TUI that opens in milliseconds.

## Install

Requires macOS and Go 1.23+.

```bash
git clone <this-repo>
cd cc-watch
./install.sh --dry-run   # see what would happen, changes nothing
./install.sh --yes       # build and install to ~/.local/bin/cc-watch
```

Make sure `~/.local/bin` is on your `PATH`. That's the whole install story —
no Homebrew tap, no curl-pipe-bash, no releases page to check.

## Quick start

```bash
cc-watch
```

Opens the session list. From there:

```bash
cc-watch --id d4b247b7     # jump straight into one session (partial IDs OK)
cc-watch --n 10            # load 10 recent sessions instead of the default 25
cc-watch --remind          # start with Reminder switched on
cc-watch config            # edit your defaults
cc-watch --json            # one machine-readable snapshot, then exit
```

There's no `--watch` flag, on purpose — the TUI already refreshes itself
while it's open. A flag that just means "keep doing what you're already
doing" isn't a feature, it's a decoy.

## The List View and the Workspace

The **List View** shows recent sessions at a glance: cache status, time
left, detected tier (`1-hour` vs `5-minute`), a progress bar, token hit rate,
and the first/last thing you said to it.

Open one to get the **Workspace**: full session ID, cache timing, token
stats, a gap analysis (did the cache actually reset mid-session, or did you
just take a long lunch?), and the Reminder/KeepAlive controls.

Expired sessions show Reminder and KeepAlive as `N/A after expiry`. Dead
cache is dead cache — the tool won't pretend otherwise just to look busy.

## Reminder

An alarm clock, not an assistant. When a session's cache crosses a
configured percent-remaining threshold, you get a local notification.
Nothing gets sent anywhere. Default thresholds: `20%` and `10%` remaining.

## KeepAlive

The part that actually touches your session, so it's built to be boring and
predictable rather than clever:

- Arms only in the final few minutes before a cache expires.
- Shows a countdown before it does anything — you can send now, cancel, or
  let it run.
- Sends by running `claude -r <session-id> -p "<your message>"` locally,
  then confirms success by watching that session's own JSONL for new
  evidence. No evidence, no success claimed.
- Capped at a configured number of sends per session, and turns itself off
  after a failure instead of retrying forever.

Keep the KeepAlive message short — you're trying to hold a cache window
open, not start a philosophical detour.

## Configuration

Lives at `~/.config/cc-watch/config.json`. Edit it by hand or run
`cc-watch config` for the in-TUI editor.

```json
{
  "reminder_thresholds": [20, 10],
  "keep_alive": {
    "trigger_before_expiry_m": 5,
    "countdown_s": 30,
    "message": "Keep-alive check. Reply \"yes\" only.",
    "auto_send": true,
    "scope": { "mode": "max_sends", "max_sends": 1 }
  }
}
```

| Field | Meaning |
|---|---|
| `reminder_thresholds` | Percent-remaining points that trigger a Reminder notification. |
| `keep_alive.trigger_before_expiry_m` | Minutes before expiry when KeepAlive may arm. |
| `keep_alive.countdown_s` | Visible countdown before Auto-send fires. |
| `keep_alive.message` | The message sent through `claude -r ... -p ...`. |
| `keep_alive.auto_send` | If `false`, KeepAlive only ever prompts you manually. |
| `keep_alive.scope.max_sends` | Max KeepAlive sends allowed per session. |

## JSON output

`cc-watch --json` prints one snapshot (schema-versioned) and exits — no TUI,
no notifications, no KeepAlive automation. Point it at a script, a status
bar, or a dashboard, if you must have one of those.

## Local-first, no exceptions

- Reads `~/.claude/projects/**/*.jsonl`. Never writes to them.
- Writes only its own config at `~/.config/cc-watch/config.json`.
- Notifies through macOS `osascript`. Nothing leaves your machine.
- Only runs `claude` locally, and only as part of a KeepAlive send you
  configured.
- Never calls an Anthropic API directly, never runs as a background daemon.

## What this isn't

Not a SaaS. Not a Chrome extension. Not a Homebrew formula (yet). Not
something that phones home, phones anyone, or has opinions about your other
tabs. It runs when you run it, and does nothing when you don't.

## Development

```bash
go test ./...
scripts/test-install.sh
go run ./cmd/cc-watch
```

Use fixture homes for tests. Don't run a real KeepAlive send as part of
verification — that's the one rule that actually matters here.

## License

MIT — see [LICENSE](LICENSE).
