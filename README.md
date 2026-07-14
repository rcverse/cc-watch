# cc-watch

See which Claude Code sessions are warm, which are fading, and which have
gone cold.

`cc-watch` is a foreground macOS TUI for browsing Claude Code's local session
history and estimating how much prompt-cache time may be left. When several
terminal windows are open, it gives each session a name, a status, and a
reasonable answer to "is this one still warm?"

> **Local by default.** `cc-watch` reads Claude Code's local session files and
> runs in the foreground. It does not make network requests or modify your
> transcripts. [Reminder](#reminders) notifications stay on your Mac. When
> enabled, [KeepAlive (KA)](#keepalive) invokes your local `claude` command,
> while statusline integration may run your existing statusline command for up
> to five seconds.

The interface is organized around four places:

- the [Main page](#main-page), which lists recent sessions;
- [Session Info](#session-info), which explains one session's cache timing,
  messages, gaps, and controls;
- [Config](#config), which stores [Reminder](#reminders), [KeepAlive](#keepalive),
  and statusline preferences;
- the [Claude Code statusline](#claude-code-statusline), an optional hook for
  usage, KeepAlive warnings, and cache timing.

> **Beta:** `v1.0.0-beta.3`. Claude Code's transcript format and statusline hook
> may change. [`KeepAlive`](#keepalive) and the [statusline integration](#claude-code-statusline)
> are beta features.

## Install

`cc-watch` runs on macOS only.

### Homebrew

Homebrew uses the fully qualified form `user/tap/formula` here:

```bash
brew install rcverse/cc-watch/cc-watch
cc-watch
```

Here `rcverse/cc-watch` selects the tap, backed by the GitHub repository
`rcverse/homebrew-cc-watch`, and the final `cc-watch` selects the formula. You
do **not** need to run `brew tap` first. Homebrew adds the tap automatically
when installing a fully qualified formula. See [Homebrew's tap installation documentation](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap#installing).

### Prebuilt binary

Download the archive for your Mac from the
[releases page](https://github.com/rcverse/cc-watch/releases), then install
the binary:

```bash
mkdir -p "$HOME/.local/bin"
tar -xzf cc-watch_*.tar.gz
install -m 0755 cc-watch "$HOME/.local/bin/cc-watch"
cc-watch
```

### Build from source

You need macOS and Go 1.23 or newer.

```bash
git clone https://github.com/rcverse/cc-watch.git
cd cc-watch
./install.sh --yes
cc-watch
```

The installer puts the command at `~/.local/bin/cc-watch`. If your shell does
not find it, add that directory to your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Usage

The default command opens the [Main page](#main-page) with the 25 most recent
sessions.

| Command | Purpose |
| --- | --- |
| `cc-watch` | Open the TUI with 25 recent sessions. |
| `cc-watch --n <count>` | Choose how many recent sessions to load. |
| `cc-watch --id <partial-id>` | Open a session by partial ID. If more than one session matches, choose from the matches. |
| `cc-watch config` | Open [Config](#config) directly. |
| `cc-watch statusline` | Run the [Claude Code statusline](#claude-code-statusline) hook. |
| `cc-watch statusline --check` | Inspect the statusline wiring without changing it. |
| `cc-watch statusline --help` | Print statusline command help. |
| `cc-watch --help` | Print command help. |
| `cc-watch --version` | Print the installed version. |

## Main page

The Main page lists recent sessions. Each row includes the project, short ID,
recent message excerpts, cache state, cache tier, hit rate, and the current
state of [Reminder](#reminders) and [KeepAlive](#keepalive).

The list updates when Claude Code changes its session files. Press `u` to
refresh immediately. Use `←` and `→` to change pages when more sessions are
loaded.

Navigation and actions stay with the list they affect:

- Move between sessions with `↑` and `↓`.
- Press `Enter` to open [Session Info](#session-info).
- Press `r` to toggle [Reminder](#reminders) for the selected session.
- Press `k` to arm or disarm [KeepAlive](#keepalive).
- Press `u` to refresh the list.
- Press `c` to open [Config](#config).
- Press `q` to quit.

If no sessions appear, open a Claude Code session first. `cc-watch` can only
list history after Claude Code writes JSONL files under
`~/.claude/projects/`.

## Session Info

Press `Enter` on the Main page to open the selected session. The workspace
shows **Cache Status**, **Session Info**, any active **KeepAlive** card, and
the controls that apply to the selected session.

### Cache Status

**Cache Status** estimates how long the cache window observed in the transcript
may remain active. It reports one of three states:

- **Active:** the estimate still has time left.
- **Expired:** the observed cache window has run out.
- **Unknown:** the transcript does not provide enough evidence for a safe
  timing claim.

The estimate is local evidence, not a direct reading from Claude's server.
Claude prompt caching is server-side, and the usual windows are five minutes
and one hour. `cc-watch` anchors its countdown only to an assistant response
with positive output usage and cache-token evidence. A user message, a
tool-only event, a local command, or an error does not refresh the anchor just
because it has a timestamp.

Some transcript events can change the cached prompt prefix or cache key. In
this beta, `/compact`, `/model`, `/reload-plugins`, and `/effort` clear the
local timing estimate and make it wait for fresh evidence. Other server-side
changes may only become visible when a later response provides that evidence.

> **Unknown is intentional.** When timing is unknown or expired, [Reminder](#reminders)
> and [KeepAlive](#keepalive) stay off rather than guessing. Press `u` to
> re-parse the selected session after a normal Claude Code turn.

For the server-side background, see Claude Code's
[prompt caching documentation](https://code.claude.com/docs/en/prompt-caching).

The workspace has two clocks:

- **Cache Status** estimates how long the currently observed cache window may
  remain active.
- **KeepAlive** counts down to a scheduled send.

The second clock is a plan for an action. It is not a second cache measurement.

### Reminders

**Reminder** sends a local macOS notification when a selected session's
remaining cache percentage crosses a configured threshold. Press `r` on the
Main page or in Session Info to toggle it.

Reminders only run while timing is known and unexpired. They never send
anything to Claude Code. The thresholds are configured in [Config](#config),
with `20%` and `10%` as the defaults.

### KeepAlive

**KeepAlive (KA)** is an automated process that keeps a selected Claude Code
session's cache warm while you are away. Press `k` to arm it when the session
has known, unexpired timing. The workspace then shows its state and scheduled
send time.

The workflow is visible and bounded:

1. `cc-watch` waits for the configured point before expiry, while respecting a
   safety margin for the cache tier.
2. A visible countdown starts. Press `s` to send immediately or `x` to cancel
   the countdown.
3. `cc-watch` runs the local `claude` command with the selected session ID and
   configured message, from that session's project directory.
4. It watches the session JSONL for a new entry to confirm that the send
   happened.
5. A failed send, timeout, missing confirmation, or reached send limit pauses
   the workflow. It does not begin retrying forever.

The default message is a short check-in. The default limit is five automatic
sends per session. After a confirmed send, KA returns to monitoring and can
schedule another send while it remains armed, up to that limit. A reached limit
can be reset from the workspace.

KeepAlive uses normal Claude Code usage and may incur normal costs. It is for
keeping a session warm, not for starting a new task unattended.

> **Safety boundary:** KeepAlive is the only feature that invokes the local
> `claude` command. The send is explicit, visible, cancellable, and capped per
> session.

### Session details

Press `v` to expand Session Info. Press `v` again to collapse it. In the
expanded view, `↑` and `↓` scroll through the details. Press `s` to switch the
gap order between **longest** and **newest**.

The details view includes:

- the full session ID, JSONL path, and file update time;
- the first and last user-message excerpts;
- token statistics: cache writes, cache reads, cache hit rate, and output
  tokens;
- **Recent Message Cache Status**, which shows whether recent user messages
  still fall inside their estimated cache window;
- **Mid-session Gaps**, which lists pauses longer than one minute.

Per-message cache status is a rewind aid. It applies the observed cache window
to recent message timestamps so a session's warm and cold stretches are easy to
see. It is derived from the transcript, not from server-side cache receipts.

A gap becomes a **reset** when it is longer than the session's cache TTL. A
shorter gap is still a pause, but it does not count as a full cache reset.

When a KeepAlive countdown owns the active controls, `s` means **send now**.
When Session Info is expanded, `s` sorts gaps instead.

## Config

Open Config with `c` from the Main page, or run:

```bash
cc-watch config
```

Move with `↑` and `↓`. Press `Enter` to edit or choose. When editing a text or
number, type the new value and press `Enter` to commit the field. Press `s` to
save the configuration, `d` to reset defaults, or `Esc` to cancel.

The settings page shows a behavior preview and validation status beside each
value:

| Setting | Behavior |
| --- | --- |
| **Reminder thresholds** | Percentages of cache time remaining that trigger local notifications. The defaults are `20%` and `10%`, in descending order. |
| **KeepAlive trigger** | How early [KeepAlive](#keepalive) may begin preparing a send. The default is five minutes before expiry, with a tighter limit for a five-minute cache. |
| **Countdown** | The visible wait before an automatic KeepAlive send. The default is 30 seconds. If the countdown cannot fit safely, the send pauses. |
| **Message** | The text KeepAlive sends through the local `claude` command. Keep it short. |
| **Max sends** | The per-session cap for automatic KeepAlive sends. The default is five. |
| **Statusline** | Opens the [Claude Code statusline](#claude-code-statusline) editor. |

A saved setting changes future Reminder and KeepAlive behavior. Saving does
not send anything by itself.

## Claude Code statusline

Claude Code's statusline hook receives a JSON payload during a Claude Code turn
and prints text into the statusline already in use. `cc-watch` adds its own
readout without replacing that command.

Configure it from `cc-watch config`:

1. Open **Statusline**.
2. Toggle the elements to include.
3. Choose each element's format and line placement.
4. Set the element order.
5. Choose **Install** and confirm the change.

The installer preserves an existing statusline command by wrapping it. It
creates a timestamped backup before changing `~/.claude/settings.json`. If
[Cache timing](#cache-status) is enabled and no refresh interval is already
configured, it asks Claude Code to refresh the hook once per second. An
existing refresh interval wins.

### Statusline elements

The statusline editor has three independent elements:

| Element | Output | Options and behavior |
| --- | --- | --- |
| **Usage** | `⏱ 34% (5h) / 41% (7d) used` | Full or compact, such as `34%/41%`. If Claude provides only one account window, only that window appears. A conservative estimated message count may appear in full format. |
| **[KA warning](#keepalive)** | `✓ KA OK` or `⚠ KeepAlive at risk` | Alert-only shows the warning when needed. Verbose also reports the healthy state. |
| **[Cache timing](#cache-status)** | `⌛ 32m41s left · 1h cache` | Full or compact, such as `32m41s`. Expired timing is shown as expired. Unknown timing produces no cache segment. |

The Usage element reads Claude Code's five-hour and seven-day account windows.
`KeepAlive at risk` means one of those account windows may run out before
enough future KeepAlive sends can happen. It is a warning, not a promise about
the exact next limit event.

Each element can appear on the same line or a new line. The configured order
controls how they are assembled. Disabled or empty elements are skipped.

### Statusline inspection and manual use

Inspect the current wiring without changing it:

```bash
cc-watch statusline --check
```

`--check` reads `~/.claude/settings.json` and prints guidance. It does not
install, uninstall, edit settings, or render statusline output. After
installation, send one normal Claude Code message so Claude Code runs the
hook.

The hook can also be composed manually:

```bash
# Claude Code normally runs this and supplies JSON on stdin.
cc-watch statusline

# Keep an existing statusline command and append cc-watch.
cc-watch statusline -- ~/.claude/statusline.sh

# Use shell syntax only when invoking a shell explicitly.
cc-watch statusline -- sh -c 'git branch --show-current'

# One-off overrides for the Usage element.
cc-watch statusline --layout=new-line --format=compact -- <command> [args...]
```

An existing wrapped command has a five-second limit. Its stderr is relayed,
and a transient wrapper failure does not make Claude Code's statusline command
fail. The hook always exits successfully, even when it omits its own segment
for that turn.

## Where data lives

| Path | Use |
| --- | --- |
| `~/.claude/projects/**/*.jsonl` | Claude Code session history, read-only |
| `~/.config/cc-watch/config.json` | Reminder, KeepAlive, and statusline preferences |
| `~/.config/cc-watch/keepalive.log` | Bounded record of KeepAlive sends and confirmations |
| `~/.config/cc-watch/ratelimit.json` | Local state for the statusline's account-limit estimate |
| `~/.claude/settings.json` | Statusline wiring, changed only by the Config page's install or uninstall action |

## Troubleshooting

### No sessions appear

Open a Claude Code session first. `cc-watch` can only list sessions after
Claude Code writes JSONL history under `~/.claude/projects/`.

### Cache timing is unknown

Send a normal Claude Code turn and press `u`. If timing stays unknown,
`cc-watch` either saw a cache-reset event or did not find enough recognized
cache evidence. Unknown timing disables [Reminder](#reminders) and
[KeepAlive](#keepalive) on purpose.

### Reminder or KeepAlive does not run

Check the relevant setting in [Config](#config). Reminder needs known,
unexpired timing. KeepAlive also needs an available `claude` command, a
selected session, and remaining sends for that session. A failed send pauses
the workflow. Fix the issue, then turn KeepAlive off and on again or reset its
limit.

### Statusline output is missing

Run `cc-watch statusline --check`. If the integration was just installed, send
one Claude Code message first. Claude Code runs the hook during a turn, so an
idle terminal may have no fresh output yet.

### `cc-watch` is not found after installation

Add `~/.local/bin` to your shell `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## For contributors

The project is a macOS-only Go application. The quick checks are:

```bash
go build ./...
go vet ./...
go test ./...
```

See [AGENTS.md](AGENTS.md) for the architecture map, safety rules, and the
full verification commands.

## License

MIT. See [LICENSE](LICENSE).
