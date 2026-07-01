# cc-watch v2 Phase 7: Notifications

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 7.

## Phase 7: Notification System

**Purpose:** Make Reminder and KeepAlive events visible without noisy retry loops.

- [x] **Step 7.1: Write failing notifier tests**
  - Create: `internal/notify/notifier_test.go`.
  - Assertions: AppleScript title/body escaping, Linux command argument handling, unsupported command degraded state, repeated identical failure suppression, distinct event resets suppression, wording separates Reminder alarm from KeepAlive automation.
  - Event wording assertions: Reminder threshold crossed says alarm only and "Sends no Claude message"; KeepAlive countdown started says a message may be sent after countdown unless canceled; manual prompt says no Claude message was sent; sent says send started; success says sent and confirmed; failure says stopped or not confirmed; scope complete says no more automatic sends.
  - Run: `go test ./internal/notify`.
  - Expected: fails before notifier implementation.

- [x] **Step 7.2: Implement notifiers**
  - Create: `internal/notify/notifier.go`, `internal/notify/macos.go`, `internal/notify/linux.go`, `internal/notify/noop.go`.
  - Behavior: notification failure updates TUI state; it does not retry noisily or hide that the underlying event happened.
  - Run: `go test ./internal/notify && go test ./internal/tui`.
  - Expected: pass.
