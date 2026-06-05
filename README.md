# cc-cache

> A terminal TUI for inspecting Claude Code prompt cache TTL, hit rate, expiry status, and session health.

## Quick Start

```bash
cc-cache              # list 5 most recent sessions
cc-cache --n 10       # list N sessions
cc-cache --id d4b247b7  # inspect specific session (partial UUID OK)
cc-cache --watch      # auto-refresh every 10s
cc-cache --remind     # notify when cache is expiring
cc-cache config       # interactive config editor
cc-cache --json       # machine-readable output
```

## Install

```bash
git clone <repo> ~/Dev/cc-cache
ln -sf ~/Dev/cc-cache/cc_cache.py ~/.local/bin/cc-cache
chmod +x ~/Dev/cc-cache/cc_cache.py
```

Requires Python 3.10+ and `rich` (`uv pip install rich` or `pip install rich`).

## Current version

**v1** — single-shot script, ANSI output. Working.

**v2** — full TUI rewrite. See [PRD.md](./PRD.md) for complete spec.

## Project layout

```
cc-cache/
├── cc_cache.py      # source (symlinked from ~/.local/bin/cc-cache)
├── PRD.md           # full product spec for v2
├── README.md        # this file
├── BOOTSTRAP.md     # prompt to resume work in a new session
├── install.sh       # installer script
└── .gitignore
```
