# cc-watch

`cc-watch` is a small macOS app that watches Claude Code's local sessions. It
shows which sessions are active, close to expiry, or already expired.

It runs in the foreground. It does not use a daemon, cloud service, or network
connection.

> Beta: `v1.0.0-beta.2`. Claude Code's local files and statusline hook may
> change. KeepAlive and statusline integration are beta features.

## Install

### Homebrew (recommended)

```bash
brew install rcverse/cc-watch/cc-watch
```

### Prebuilt download

Download the archive for your Mac from the
[latest release](https://github.com/rcverse/cc-watch/releases/latest), then
install it:

```bash
mkdir -p "$HOME/.local/bin"
tar -xzf cc-watch_*.tar.gz
install -m 0755 cc-watch "$HOME/.local/bin/cc-watch"
```

### Build from source

This option requires macOS and Go 1.23 or newer.

```bash
git clone https://github.com/rcverse/cc-watch.git
cd cc-watch
./install.sh --dry-run
./install.sh --yes
```

The command is installed at `~/.local/bin/cc-watch`. Make sure that directory is
on your `PATH`, then check it:

```bash
cc-watch --version
```

## Start

```bash
cc-watch
```

Use the session list to choose a session. Press `Enter` to see its details.

- `r`: turn reminders on or off for the selected session
- `k`: turn KeepAlive on or off
- `u`: refresh the session list
- `c`: open settings
- `q`: quit

In settings, use the arrow keys to move, `Enter` to edit or choose, `s` to
save, and `Esc` to go back or cancel.

## What it shows

For each session, cc-watch shows the project, recent activity, cache time left,
cache tier, and basic token statistics. A session can be active, fading, or
expired.

## KeepAlive

KeepAlive is optional. It can send one short message to a selected Claude Code
session before its cache expires.

- You see a countdown before anything is sent.
- You can cancel it or send immediately.
- Each session has a send limit.
- A failed send pauses instead of retrying forever.
- It uses your local `claude` command and normal Claude usage. It may incur
  normal Claude costs.

Keep the message short. The feature is meant to keep a session warm, not to
start a new task.

## Claude Code statusline

The optional statusline integration adds Claude's 5-hour and 7-day usage to
your existing Claude Code statusline.

Open `cc-watch config`, choose `Statusline`, then choose `Layout` and `Format`.
The install, reinstall, and uninstall actions require a second confirmation.

Useful commands:

```bash
cc-watch statusline --check
cc-watch statusline --help
cc-watch statusline --layout=new-line --format=compact -- <command>
```

`--check` only reads Claude Code settings. It does not change them.

If you install the integration, send one Claude Code message first. Claude
Code only runs the hook during a turn, so there may be no statusline output
before that.

The full output looks like:

```text
⏱ 34% (5h) / 41% (7d) used
```

Compact output looks like:

```text
34%/41% · ⚠ KA
```

If you already have a statusline command, cc-watch keeps it and adds its own
output. Shell pipelines must be wrapped explicitly, for example with `sh -c`.

## Settings

Run:

```bash
cc-watch config
```

Settings are stored at `~/.config/cc-watch/config.json`. The settings screen
controls reminders, KeepAlive, and statusline display preferences.

## Safety and privacy

- macOS only.
- cc-watch reads `~/.claude/projects/**/*.jsonl` and never changes those files.
- It makes no network requests.
- Reminder notifications stay on your Mac.
- KeepAlive is visible, cancellable, and bounded.
- Statusline may run your existing statusline command for up to five seconds.
- Installing or removing the Claude Code statusline creates a timestamped
  backup first.

## Troubleshooting

### No sessions appear

Open a Claude Code session first. Claude Code must write local session history
before cc-watch has anything to show.

### KeepAlive cannot start

Make sure `claude` is available on your `PATH`, select a session, and check the
KeepAlive settings with `cc-watch config`.

### Statusline output is missing

Run `cc-watch statusline --check`. If you just installed it, send one Claude
Code message so the hook receives input.

### `cc-watch` is not found after installation

Add this directory to your shell `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Development

```bash
go test ./...
go vet ./...
go build ./...
scripts/test-install.sh
```

## License

MIT. See [LICENSE](LICENSE).
