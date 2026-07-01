# cc-watch Domain Context

## Terms

### Session Snapshot

The current parsed view of Claude Code cache sessions plus config-derived runtime defaults. It is the source consumed by JSON output, TUI startup, and TUI refresh.

### Cache Status

The user-facing state of one session cache window: active, expired, or unknown, with TTL timing and cache tier.

### Session Info

The user-facing session detail summary: IDs, message excerpts, token stats, and mid-session gaps.

### Refresh Runtime

The internal TUI mechanism that updates session snapshots from manual update, filesystem events, and safety ticks. It is not a public `--watch` mode.

### JSON API

The stable `--json` interface for scripts and future clients. It is an in-scope public API, not a debug dump.

### KeepAlive Runtime

The bounded, visible, cancellable automation state for optionally sending a configured Claude message to one selected session.

### Product Platform

The current product target is macOS with native `osascript` notifications and a simple local binary install. Linux, Windows, Homebrew, and public release automation are out of scope unless re-approved.

### Implementation Stack

The current implementation stack is Go because cc-watch is a local macOS terminal binary with filesystem watching, native notification command execution, bounded subprocess control, stable JSON output, and simple local install. TypeScript is out of scope unless the product becomes Node-native, web-native, or editor-extension-native.

### Route Module

A TUI route's local focus, render, and action behavior. List, Workspace, Ambiguous, and Config are route modules.

### Notification Runtime

The macOS notification path for Reminder and KeepAlive events. It owns event wording, osascript command construction, failure suppression, and notification degraded state.

### Live Refresh Adapter

The TUI adapter that turns Refresh Runtime watcher results, debounce decisions, and safety ticks into Bubble Tea messages. Refresh Runtime owns refresh policy; the adapter owns Bubble Tea message conversion.
