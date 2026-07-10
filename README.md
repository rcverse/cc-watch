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
  bounded workflow that can nudge a session before it goes cold, then stop
  at a send limit.

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
cc-watch config            # edit defaults and Statusline install
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
  let it run automatically.
- Sends by running `claude -r <session-id> -p "<your message>"` from that
  session's own project directory (`claude --resume` is scoped to it), then
  confirms success by watching that session's own JSONL for new evidence. No
  evidence, no success claimed.
- Stops at a configured send limit per session. When the limit is reached,
  the TUI shows that state and waits for you to reset it.
- Pauses after a failure instead of retrying forever.

Keep the KeepAlive message short — you're trying to hold a cache window
open, not start a philosophical detour.

Every send and confirmation is recorded in
`~/.config/cc-watch/keepalive.log` (JSON lines: the directory it ran in,
exit code, and why it failed) — the first place to look when a send didn't
work. It's bounded to ~2 MiB and rotates once; set `CC_WATCH_KEEPALIVE_LOG=off`
to disable it.

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
    "scope": { "max_sends": 5 }
  }
}
```

| Field | Meaning |
|---|---|
| `reminder_thresholds` | Percent-remaining points that trigger a Reminder notification. |
| `keep_alive.trigger_before_expiry_m` | Minutes before expiry when KeepAlive may arm. |
| `keep_alive.countdown_s` | Visible countdown before the KeepAlive message sends. |
| `keep_alive.message` | The message sent through `claude -r ... -p ...`. |
| `keep_alive.scope.max_sends` | KeepAlive send limit for one session. |

## statusline

Claude Code's account-wide 5-hour and 7-day rate limits are separate from
the per-session cache TTL cc-watch otherwise tracks — and they matter,
because a capped account can't send anything, including a KeepAlive ping.
`cc-watch statusline` plugs into Claude Code's `statusLine.command` hook and
adds usage plus KeepAlive-risk context inside Claude Code.

```bash
cc-watch statusline                   # emit only cc-watch's own readout
cc-watch statusline -- <command>      # keep an existing statusline and add cc-watch
cc-watch statusline --check           # read-only: print install/uninstall guidance
cc-watch statusline --help            # explain the hook CLI
```

The config TUI can install or uninstall the Statusline integration for you.
It preserves an existing Claude Code statusline when the shape is safe, and
refuses to write when the current setting needs manual review. `--check`
is the read-only path: it reads `~/.claude/settings.json` and prints the
exact snippet to enable or undo cc-watch, without writing the file itself.
For example, if your existing statusline command is some tool, wiring it in
looks like:

```json
{ "statusLine": { "type": "command", "command": "cc-watch statusline -- <your statusline command>" } }
```

If your existing statusline command is a shell pipeline rather than a
single executable, wrap it in `sh -c '...'` first — cc-watch spawns the
wrapped command directly (argv, no shell), so it can't run a pipe on its
own.

When wrapping an existing statusline, cc-watch appends its segment after
` | `. The readout looks like `⏱ 34% (5h) / 41% (7d) used`, or `⏱ 34%
(5h) / 41% (7d) used · ✉ ~12 msgs` once it has enough 5-hour history to
estimate messages left. If the 5-hour or 7-day account limit looks likely
to block future KeepAlive sends, it shows `⏱ 87% (5h) / 94% (7d) used · ⚠
KeepAlive at risk`. "Messages" is always an estimate — Claude Code exposes
percentages and reset times, not an absolute token budget — and it's
deliberately conservative, since it's calibrated from your actual mix of
messages rather than isolating KeepAlive's own cheaper cost. Set `NO_COLOR`
to suppress the at-risk color.

## Local-first, no exceptions

- Reads `~/.claude/projects/**/*.jsonl`. Never writes to them.
- Writes only its own config at `~/.config/cc-watch/config.json` and its
  own logs/state under `~/.config/cc-watch/`: `keepalive.log` for
  KeepAlive sends and `ratelimit.json` for `cc-watch statusline`
  (self-healing, no delete command needed).
- Can install/uninstall Claude Code's statusLine setting from the config
  TUI by editing `~/.claude/settings.json`; it writes a timestamped backup
  first and refuses unclear shapes.
- Notifies through macOS `osascript`. Nothing leaves your machine.
- Only runs `claude` locally, and only as part of a KeepAlive send you
  configured. `cc-watch statusline` may also run one subprocess: your own
  existing statusline command, if you chose to wrap it.
- `cc-watch statusline --check` only reads `~/.claude/settings.json`; it
  never writes.
- Never calls an Anthropic API directly, never runs as a background daemon.

## What this isn't

Not a SaaS. Not a Chrome extension. Not a Homebrew formula (yet). Not
something that phones home, phones anyone, or has opinions about your other
tabs. It runs when you run it, and does nothing when you don't.

## Development

```bash
go test ./...
go test -tags demo ./...
scripts/test-install.sh
go run ./cmd/cc-watch
go run -tags demo ./tools/ui-demo
```

Use fixture homes for tests. Don't run a real KeepAlive send as part of
verification — that's the one rule that actually matters here.

`tools/ui-demo` is a build-tagged fixture harness for rare UI states such as
near-expiry and send-limit flows. It uses the real TUI renderer and state
machine with fake sessions, fake time, fake notifications, and a fake
KeepAlive runner. It is not part of the installed command.

## License

MIT — see [LICENSE](LICENSE).
