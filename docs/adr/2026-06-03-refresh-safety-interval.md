# ADR: Refresh Safety Interval

Date: 2026-06-03

## Status

Accepted

## Context

cc-watch v2 uses filesystem events to accelerate updates, but filesystem watchers are not perfectly reliable across project directory creation, rename/delete behavior, terminal contexts, and platform differences.

The product requires no public watch mode, watch flag, or user-configurable watch interval. Live refresh is internal TUI behavior, supported by manual refresh and degraded-state visibility.

## Decision

Use three refresh paths:

- fsnotify events for changed, created, renamed, and deleted JSONL files or project directories;
- a debounced refresh after bursts of fsnotify events;
- an internal periodic safety refresh to recover from missed events.

Use a 300 ms debounce window for fsnotify bursts.

Use a 30 second internal safety refresh interval while the TUI is running. This interval is not configurable and is not exposed as a public CLI option.

Manual refresh is always available in the TUI and starts a refresh immediately for the relevant scope.

Watcher failures are degraded states, not total app failures. When watcher setup partially or fully fails, the TUI and JSON output must surface degraded watcher messaging while safety refresh remains active where possible.

Every data refresh request receives a monotonic generation. Manual refresh increments generation immediately. Debounced fsnotify and safety refresh results apply only when their generation is current for the affected scope. Delete and rename observations cannot be overwritten by an older parse result.

## Consequences

The TUI can feel live when file events work, while still recovering from missed events without exposing polling controls to users.

Generation ordering prevents stale parse results from restoring sessions that were deleted, renamed, or superseded by newer manual refreshes.
