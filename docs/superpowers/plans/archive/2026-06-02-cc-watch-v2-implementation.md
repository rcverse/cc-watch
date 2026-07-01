# cc-watch v2 Implementation Plan — Retired

**Status:** Retired after architectural audit.
**Date retired:** 2026-06-02

This plan must not be used for implementation.

Reason:

- It was written against the pre-audit design.
- It assumes fsnotify provides complete recursive real-time behavior without the required safety refresh architecture.
- It does not integrate watcher events into Bubbletea.
- It does not preserve all v1 parser behavior.
- It leaves KeepAlive as a partial/stubbed implementation rather than a bounded per-session state machine.
- It contains CLI, config editor, notification, packaging, and Go-version assumptions that are no longer valid.
- It does not reflect the UsageAware decision: postpone unless a Claude Code statusline bridge is explicitly designed and tested.

The next implementation plan should be written from the revised design spec:

`docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md`

No replacement plan is drafted here by request.
