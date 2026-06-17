# cc-cache Domain Context

## Terms

### Session Snapshot

The current parsed view of Claude Code cache sessions plus config-derived runtime defaults. It is the source consumed by JSON output, TUI startup, and TUI refresh.

### Cache Status

The user-facing state of one session cache window: active, expired, or unknown, with TTL timing and cache tier.

### Session Info

The user-facing session detail summary: IDs, message excerpts, token stats, and mid-session gaps.

### Refresh Runtime

The internal TUI mechanism that updates session snapshots from manual update, filesystem events, and safety ticks. It is not a public `--watch` mode.

### KeepAlive Runtime

The bounded, visible, cancellable automation state for optionally sending a configured Claude message to one selected session.

### Route Module

A TUI route's local focus, render, and action behavior. List, Workspace, Ambiguous, and Config are route modules.
