# cc-watch v2 Product Reality

**Status:** active scope alignment
**Date:** 2026-06-18

This document records the current in-scope product boundary before Phase 11.9 cleanup. It overrides older broad release or cross-platform language where those documents conflict with the codebase and current product intent.

## Product Shape

cc-watch v2 is a lightweight macOS terminal tool for inspecting Claude Code session cache state and managing per-session Reminder and KeepAlive workflows.

Lightweight means focused behavior and simple local operation. It does not mean static data: while the TUI is running, session state must update automatically from Claude session file changes.

## In Scope

- List the most recent `n` Claude sessions and their cache status.
- Open one session and display useful session details: cache state, timing, token/cache stats, message excerpts, gaps, IDs, and JSONL path.
- Live-refresh the running TUI automatically when Claude session files change.
- Keep manual refresh and periodic safety refresh as fallbacks for missed file events.
- Expose `--json` as a stable schema-versioned API for scripts and future clients.
- Let users enable Reminder per session. Reminder sends native macOS notifications only and never sends Claude messages.
- Let users enable KeepAlive per session. KeepAlive is visible, bounded, cancellable, session-scoped, and evidence-confirmed.
- Support KeepAlive manual send and Auto-send through the configured per-session Auto-send state.
- Provide a config editor for global Reminder thresholds and KeepAlive defaults.
- Provide a simple local macOS install path after the binary is verified.

## Public CLI Scope

```text
cc-watch                     # List View
cc-watch --n N               # List View with N recent sessions
cc-watch --id <partial-id>   # Session Workspace
cc-watch --json              # Stable JSON API
cc-watch --json --id <id>    # Stable JSON API for one session
cc-watch --remind            # Start TUI with Reminder enabled for loaded sessions
cc-watch config              # Config Editor
cc-watch --help              # Help
cc-watch --version           # Version
```

There is no public watch command or watch flag. Live refresh is internal TUI behavior.

## Platform Scope

The supported product target is macOS. Native notifications use `osascript`. Linux, Windows, and cross-platform notification behavior are out of scope unless explicitly re-approved.

## Distribution Scope

Simple local macOS install is in scope. GitHub Releases, goreleaser, Homebrew tap publishing, Linux packages, and public release automation are out of scope unless explicitly re-approved after the local binary path is stable.

## Architecture Consequences

- Keep live refresh architecture. The watcher/coordinator/fsnotify path is product-relevant and should be wired clearly rather than deleted as dead code.
- Keep snapshot ownership for startup, JSON output, and refresh parsing boundaries.
- Keep KeepAlive and JSON API code separate because they are safety-sensitive and public-contract-sensitive.
- Simplify shallow package/file scattering where it does not protect a real boundary.
- Phase 11.9 cleanup may remove or consolidate Linux notification code, stale docs, and tiny abstraction files, but must not remove live refresh behavior.

## Out Of Scope Unless Re-Approved

- Public watch mode.
- Background daemon.
- Native macOS app.
- Linux or Windows support.
- Homebrew/GitHub release publishing.
- Unbounded or hidden KeepAlive loops.
- Anthropic network/API calls.
- Reminder-triggered Claude sends.
