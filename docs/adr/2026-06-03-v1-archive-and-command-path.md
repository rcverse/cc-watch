# ADR: v1 Archive And Command Path

Date: 2026-06-03

## Status

Accepted

## Context

cc-cache v1 is currently the usable implementation. The installed command path is expected to resolve through `$HOME/.local/bin/cc-cache`, which points at `/Users/richardchen/Dev/cc-cache/cc_cache.py`.

The v2 implementation will introduce a Go binary, but moving, deleting, or replacing the root Python script before v2 is verified would break the current command path and remove the easiest rollback path.

## Decision

Copy the v1 implementation into `archive/v1/` during migration:

- `archive/v1/cc_cache.py`
- `archive/v1/install-v1.sh`
- `archive/v1/README.md`

Leave the root `cc_cache.py` in place during v2 implementation. The live `$HOME/.local/bin/cc-cache` command must remain usable until the Go binary has passed verification and the user explicitly approves switching the command path.

Do not replace `$HOME/.local/bin/cc-cache` as part of normal implementation work. Switchover is a separate approval gate after CLI, JSON, TUI smoke, and v1 preservation checks pass.

## Consequences

The repository temporarily carries both the legacy script and new Go implementation. That duplication is intentional because it preserves user workflow and rollback safety while v2 is being built.

Early moves are unsafe because the installed symlink targets `/Users/richardchen/Dev/cc-cache/cc_cache.py`; moving that file would make the existing command fail before the replacement binary is ready.
