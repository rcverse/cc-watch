# cc-watch v2 — Design Specification

**Status:** Historical design detail; current scope is constrained by `2026-06-18-cc-watch-v2-product-reality.md`.
**Date:** 2026-06-02
**Stack:** Go 1.23+ + Bubbletea + lipgloss + fsnotify
**Distribution target:** simple local macOS install

Post-acceptance visual grammar note:
`docs/superpowers/specs/2026-06-29-cc-watch-v2-visual-grammar.md`

---

## 1. Product Direction

cc-watch v2 is a terminal TUI for inspecting Claude Code session cache health and managing bounded keep-alive/reminder workflows.

v2 is not a small port of v1. It is a Go rewrite with:

- live countdown rendering in a terminal UI;
- v1-compatible JSONL parsing and metrics;
- internal live refresh with filesystem events plus safety refresh;
- a per-session workspace for details, reminders, and keep-alive controls;
- bounded, visible keep-alive automation;
- stable JSON output for scripting and future external clients.

This design spec is the source of truth for v2 implementation details.

---

## 2. Non-Goals

- No native macOS app in v2.
- No background daemon in v2.
- No public watch command, watch flag, or configurable watch interval.
- No infinite keep-alive loop as a default or ordinary workflow.

---

## 3. Tech Stack Decision

| Layer | Technology | Design Rationale |
|---|---|---|
| Language | Go 1.23+ | Single binary, simple filesystem/subprocess handling, strong fit for terminal tools |
| TUI | Bubbletea | Clear model-update-view loop; good keyboard and resize behavior |
| Styling | lipgloss | Terminal layout and state styling without a custom renderer |
| Components | bubbles | Inputs, lists, spinners where useful |
| JSONL parsing | Go standard library | Avoid unnecessary dependencies |
| File events | fsnotify | Event acceleration for macOS session files |
| Notifications | `osascript` on macOS | Native notification path for the supported product target |
| Keep-alive execution | Claude CLI subprocess | Runs only under bounded user-configured scope |
| Packaging | local macOS binary install | First supported distribution path |

Go remains the right fit for the v2 TUI. A future native macOS app should be treated as a separate Swift client that consumes cc-watch data, not as a reason to move the TUI core out of Go.

---

## 4. CLI Contract

```text
cc-watch                     # List View, default recent sessions
cc-watch --n N               # List View, N recent sessions
cc-watch --id <partial-id>   # Session Workspace for one session
cc-watch --json              # Machine-readable JSON, then exit
cc-watch --json --id <id>    # JSON for one session, then exit
cc-watch --remind            # Start TUI with reminders enabled for loaded sessions
cc-watch config              # Config Editor
cc-watch --help              # Help
cc-watch --version           # Version
```

There is no public watch command or watch flag in v2. Live refresh is internal behavior, not a mode, and must not reintroduce watch intervals or polling semantics.

Partial IDs are resolved by substring match against the JSONL filename stem, matching v1 behavior. Ambiguous partial IDs must produce a clear selection/error state.

---

## 5. Parser Equivalence Contract

The Go parser must preserve v1 behavior before adding new presentation behavior.

Required v1-compatible rules:

- Discover session files from `~/.claude/projects/**/*.jsonl`.
- Sort recent sessions by JSONL file modification time, not last parsed message timestamp.
- Resolve partial IDs against the filename stem.
- Ignore malformed JSONL lines rather than failing the whole session.
- Parse timestamps when present; tolerate missing or malformed timestamps.
- Extract usage from both top-level `usage` and nested `message.usage`.
- Flatten one nested usage map when Claude records token details in nested structures.
- Count cache writes from `cache_creation_input_tokens` and compatible cache-creation fields.
- Count cache reads from `cache_read_input_tokens`.
- Count output tokens from `output_tokens`.
- Determine TTL tier from token evidence:
  - `ephemeral_1h_input_tokens > 0` means 1-hour cache;
  - otherwise `ephemeral_5m_input_tokens > 0` means 5-minute cache;
  - otherwise TTL is unknown for display.
- For gap/reset analysis, unknown TTL is treated as 5 minutes, matching v1’s conservative reset heuristic.
- Record gaps over 60 seconds.
- Mark a cache reset when a gap exceeds the effective TTL.
- Compute hit rate as `cache_read / (cache_read + cache_create)`.
- Track first and last user-visible message excerpts.

Additional Go-specific requirements:

- JSONL reading must handle long lines safely. A default `bufio.Scanner` is not sufficient unless its buffer is increased and `scanner.Err()` is checked.
- Parser tests must include long JSONL lines, malformed lines, no timestamps, timestamp-less files, top-level usage, nested `message.usage`, nested cache creation structures, 5-minute TTL, 1-hour TTL, unknown TTL, gaps, and reset detection.
- Rendering must not assume UUID length. Short or malformed stems must not crash list rendering.

---

## 6. Refresh Architecture

v2 has two independent update streams:

```text
Display tick
  every 1 second
    -> recompute elapsed/remaining time from current state
    -> update countdown timers
    -> render

Data refresh
  fsnotify events + debounced safety refresh
    -> discover changed/new/deleted JSONL files
    -> parse affected files or reload list when needed
    -> update app state
    -> render
```

fsnotify is an event accelerator, not the only source of truth.

Design requirements:

- Register watches recursively for existing project subdirectories.
- Watch the root projects directory for new project directories.
- Add watches for newly created subdirectories.
- Handle create, write, rename, and delete events.
- Debounce bursts so one Claude write sequence does not trigger repeated reparses.
- Run a periodic safety refresh to recover from missed filesystem events.
- Provide manual refresh in the Session Workspace.
- Send watcher events into Bubbletea as explicit messages, not hidden goroutine mutation.

This design avoids the earlier false premise that `fsnotify` directly watches `~/.claude/projects/**/*.jsonl` recursively with perfect reliability.

---

## 7. Data Model

Each parsed session must include:

- session ID;
- JSONL path;
- file modification time;
- project name;
- TTL tier and TTL seconds when known;
- status: active, expired, or unknown;
- last message timestamp when known;
- first/last user message excerpt;
- session start/end/duration when known;
- cache write/read/output token totals;
- hit rate;
- gap list and reset count;
- parser warnings for degraded states.

Time-derived fields such as elapsed seconds, remaining seconds, and percentage elapsed should be computed from the current clock, not stored as stale parser output.

---

## 8. Reminder System

Reminder is a user-defined alarm system. It is independent of keep-alive.

Behavior:

- Per-session toggle during the current TUI run.
- Global remaining-percentage thresholds from config.
- Reminder thresholds are edited in Config Editor, not in Session Workspace.
- The Session Workspace only toggles Reminder on/off for the selected session.
- Default thresholds are 20% and 10%.
- Fires once per threshold crossing per active session instance.
- Does not send Claude messages.
- Does not alter keep-alive state.

Reminder notifications fire only while cc-watch is running.

---

## 9. KeepAlive System

KeepAlive is a bounded, user-visible workflow for preserving an active cache window. It must not be designed as hidden or unbounded automation.

### 9.1 Concepts

- **Monitoring:** per-session toggle; default off.
- **Auto-send:** per-session toggle, initialized from global KeepAlive defaults; default on, but only within a bounded scope.
- **Scope:** required boundary for auto-send behavior.
- **Countdown:** visible pre-send state that can be cancelled or sent immediately.
- **Confirmation:** subprocess result plus session-specific JSONL evidence.

### 9.2 Default Config

Global KeepAlive config is a default template for new per-session KeepAlive state. It is not a hidden global execution switch for active sessions.

```json
{
  "reminder_thresholds": [20, 10],
  "keep_alive": {
    "trigger_before_expiry_m": 5,
    "countdown_s": 30,
    "message": "Keep-alive check. Reply \"yes\" only.",
    "auto_send": true,
    "scope": {
      "mode": "max_sends",
      "max_sends": 1
    }
  }
}
```

Supported scope mode:

| Mode | Meaning |
|---|---|
| `max_sends` | Stop after N successful or attempted sends |

An endless mode is not part of normal v2. If added later, it must be advanced/explicit, visibly dangerous, and notify the user periodically that automation is still running.

### 9.3 TTL-Aware Trigger And Send Timing

`trigger_before_expiry_m = 5` must not mean “send immediately” for 5-minute cache sessions.

Effective trigger is TTL-aware:

```text
configured_trigger_s = keep_alive.trigger_before_expiry_m * 60
ttl_fraction_trigger_s = ttl_seconds * 0.20
effective_trigger_s = min(configured_trigger_s, ttl_fraction_trigger_s)
```

Default outcomes:

- 1-hour TTL: `min(300s, 720s)` -> trigger at 5 minutes remaining.
- 5-minute TTL: `min(300s, 60s)` -> trigger at 1 minute remaining.
- unknown TTL: use 5-minute TTL for trigger calculation, matching the conservative gap/reset heuristic.

The send timing is configurable through `trigger_before_expiry_m` and `countdown_s`, but automatic sending must preserve a fixed safety margin before expiry.

Safety rule:

- Auto-send must be scheduled at least 30 seconds before the cache window expires.
- If the configured countdown would send later than that, clamp the countdown.
- If the effective trigger window cannot preserve the 30-second margin, disable auto-send for that instance and show a manual-send warning.

```text
safety_margin_s = 30
effective_countdown_s = min(configured_countdown_s, effective_trigger_s - safety_margin_s)
```

If `effective_trigger_s <= safety_margin_s`, auto-send is disabled for that instance and the UI shows a manual-send warning instead. If per-session Auto-send is off, threshold crossing enters `manual_ready` directly; it must not show a sending countdown.

### 9.4 State Machine

```text
off
  -> monitoring_idle             user enables KeepAlive

monitoring_idle
  -> countdown                   threshold crossed, scope permits, auto-send is on, and countdown window is usable
  -> manual_ready                threshold crossed or already crossed, and auto-send is off or unsafe for this instance
  -> scope_complete              scope already exhausted

countdown
  -> sending                     per-session Auto-send is on and countdown reaches zero, or user activates send now
  -> manual_ready                auto-send is off and countdown reaches zero
  -> cancelled_instance          user cancels this instance
  -> monitoring_idle             session refreshes before send is needed

manual_ready
  -> sending                     user activates send
  -> cancelled_instance          user dismisses or session expires
  -> monitoring_idle             session refreshes before send is needed

sending
  scope_count += 1               exactly once when a send attempt is initiated
  -> confirming                  subprocess started
  -> error_no_claude             claude command unavailable
  -> error_subprocess            subprocess exits unsuccessfully

confirming
  -> success                     session-specific JSONL confirmation found
  -> error_timeout               no confirmation within timeout
  -> cancelled_instance          user cancels waiting state

success
  -> monitoring_idle             after short success display, if scope remains
  -> scope_complete              if scope exhausted

error_no_claude / error_subprocess / error_timeout
  -> scope_complete              if scope exhausted
  -> off                         if auto-send was stopped by hard failure and user turns KeepAlive off
  -> monitoring_idle             only after explicit user re-enable and a new eligible cache window

cancelled_instance
  -> monitoring_idle             next eligible threshold may re-arm only after refresh/edge reset
  -> off                         user disables KeepAlive

scope_complete
  -> off                         user acknowledges or disables
```

Required safeguards:

- Per-session state map, not one global KeepAlive state.
- Entering monitoring must evaluate the current remaining time. If the session is already inside the effective trigger window, the state must move immediately to `countdown` or `manual_ready`; it must not wait for a crossing that already happened.
- Edge-triggered threshold crossing; no repeated send loop on every tick. Re-arming requires a session refresh/new cache window that moves the session back above the trigger threshold.
- Single-flight subprocess execution per session.
- Cancellation must stop countdown, manual-ready, or confirmation wait, not leave background sends orphaned.
- Confirmation must be tied to the target session file and a timestamp/new line after the send attempt.
- Scope count increments once when a send attempt is initiated, whether the attempt later succeeds, fails to find `claude`, exits unsuccessfully, or times out during confirmation.
- Any Claude limit or subprocess error must stop auto-send for that session.
- The UI must always show next send time, remaining scope, last send result, and manual fallback command where relevant.

### 9.5 User-Facing KeepAlive Paths

The state machine is for implementation. The TUI must explain KeepAlive through user-facing paths:

```text
Path A: Auto-send on

Off
  -> user turns KeepAlive On
  -> Watching
       Shows: trigger time, message, Auto-send on, scope
  -> Countdown
       Shows: exact message, seconds left, Send now, Cancel
  -> Sending
       Runs: claude -r <session> -p <message>
  -> Confirming
       Watches this session JSONL for a new line after send time
  -> Success
       Shows refreshed cache window and updated scope
  -> Done or Watching again
       Done when scope is exhausted; Watching again only if scope remains
```

```text
Path B: Auto-send off

Off
  -> user turns KeepAlive On and Auto-send Off
  -> Watching
       Shows: trigger time, message, Auto-send off, scope
  -> Manual prompt
       Shows: no Claude message was sent, Send now, Dismiss
  -> user sends manually
       Then follows Sending -> Confirming -> Success/Failure
  -> user dismisses
       This instance ends; no Claude message is sent
```

```text
Path C: Already near expiry

Off
  -> user turns KeepAlive On
  -> app checks current remaining time immediately
  -> if already inside trigger window:
       Auto-send on  -> Countdown now
       Auto-send off -> Manual prompt now
```

```text
Path D: Unsafe or failed automation

Countdown or Manual prompt
  -> claude unavailable, trigger window too short, subprocess error, or Claude limit
  -> Failure/manual fallback
       Shows reason, command to run manually, and that Auto-send stopped where relevant
```

```text
Path E: User cancellation

Countdown / Manual prompt / Confirming
  -> user cancels or dismisses this instance
  -> no background send or confirmation wait remains
  -> KeepAlive can re-arm only after the session refreshes into a new eligible cache window
```

Every path must show whether a Claude message has been sent, may be sent, or will not be sent.

---

## 10. TUI UX Contract

The TUI is an operator surface, not a decorative dashboard. It must prioritize fast scanning, clear control boundaries, and explicit degraded states over visual novelty.

Primary UX principles:

- Show one primary question per surface:
  - List View: "Which sessions need attention?"
  - Session Workspace: "What is happening in this session, and what can I safely do?"
  - Config Editor: "What KeepAlive defaults should future sessions use?"
- Use plain beginner-facing labels next to technical terms on first exposure. Prefer "cache window" over bare "TTL" in user-facing copy, with TTL kept as a compact technical label where density matters.
- Keep reminders and KeepAlive visually and textually separate. A reminder is an alarm. KeepAlive can send a Claude message.
- Every automated KeepAlive state must show scope, next action, cancellation, and last result without requiring the user to remember config.
- Cursor navigation is the primary approachable interaction model. Single-key commands are accelerators, not the only way to reach actions.
- Unknown, degraded, or unavailable states must be visible in the active surface and included in `--json`.

### 10.0 Visual Presentation Contract

The ASCII mockups are structural. The implemented TUI must use a polished terminal visual system, not a plain text dump.

Required visual structure:

- Use a persistent top header with app name, current surface, selected project/session, refresh state, and degraded-state badges.
- Use panel boundaries for major regions: List, Session Evidence, Controls, Config fields, and Validation.
- Do not use nested boxed panels. Inside a panel, use spacing, subtle dividers, labels, and indentation rather than more borders.
- At 80 columns, stack panels vertically in this order: header, evidence, controls, footer.
- At wide widths, Session Workspace may use a two-column layout: Evidence on the left, Controls on the right, with Controls kept visually distinct because it can change state.
- Keep footers stable across screens: cursor instructions first, shortcuts second.
- Keep dangerous KeepAlive states pinned inside the visible KeepAlive card in Controls. Footer/navigation copy must not displace an active countdown, sending, confirming, or failure state.

Required visual vocabulary:

- Use restrained lipgloss color roles, not ad hoc colors per string.
- Semantic roles: neutral text, muted labels, selected focus, info, warning, danger, success, disabled, degraded.
- Color must never be the only signal. Pair color with labels, symbols, or state text.
- Use status badges sparingly: `active`, `expired`, `unknown`, `watcher degraded`, `notify degraded`, `KeepAlive on`, `countdown`, `failed`.
- Focus indication must be consistent across all controls. Prefer a leading cursor plus selected styling, for example `> [ ] On`, rather than changing control shape between screens.
- Read-only evidence should use muted labels and stable alignment. Editable controls should use checkbox, input, or action-button vocabulary.
- Warning/danger color is reserved for automation that may send a Claude message, failed automation, expiring sessions, and degraded system behavior.
- Reminder uses info/neutral styling because it is an alarm only. KeepAlive uses warning styling when it is armed or counting down because it can send a Claude message.

Panel and divider rules:

- Normal panel borders use a low-contrast neutral.
- The focused panel may use a stronger neutral or accent border.
- Countdown, sending, confirming, and failure states may tint the KeepAlive panel border with warning/danger roles.
- Internal dividers should be one-line separators or blank-space rhythm, not full nested boxes.
- Section headings inside panels should be short nouns: `Status`, `Messages`, `Token Stats`, `Gaps`, `Reminder`, `KeepAlive`.
- Use compact progress bars only where they express time or completion: TTL elapsed, KeepAlive countdown, loading skeletons.
- Avoid decorative icons. Symbols are allowed only when they carry state meaning and have text nearby.

Session Workspace control structure:

- The Controls region contains exactly two primary control rows:
  1. Reminder row;
  2. KeepAlive row.
- Reminder is one line: toggle, global thresholds summary, and a short safety statement.
- KeepAlive is one compact line while off: KeepAlive toggle, Auto-send toggle, trigger/scope summary, and a short safety statement.
- Auto-send is a secondary focus target within the KeepAlive row/card, not a third primary control row.
- When KeepAlive is on, the KeepAlive row expands into an in-workspace KeepAlive card directly below the row, still inside the Controls region.
- The KeepAlive card owns active-state display and active-state actions: Send now, Cancel instance, Dismiss, Stop waiting, Copy fallback command, and Turn off.
- The KeepAlive card uses the same state row grammar across all states: state badge, State, Next, Message, Scope, Progress/Evidence when relevant, Controls.
- Countdown, Sending, Confirming, and Failure states may tint the KeepAlive card border or left accent. The rest of the workspace stays stable.
- KeepAlive must render inside Controls, not as an overlay, modal, or detached card.

### 10.1 List View

The List View shows recent sessions sorted by JSONL modification time. It is optimized for scanning at 80 columns and richer detail at wider widths.

Required row information, in priority order:

1. cursor/selection marker;
2. short ID;
3. project name;
4. cache window label: `1h`, `5m`, or `TTL ?`;
5. status: `active`, `expired`, or `unknown`;
6. remaining time or expired age;
7. TTL elapsed bar, when TTL is known;
8. hit-rate summary;
9. first/last message excerpt;
10. session duration;
11. optional KeepAlive summary.

Responsive density requirements:

- At narrow widths, keep ID, project, cache window, status, and remaining/expired time. Truncate excerpts first.
- At medium widths, include hit rate and one message excerpt.
- At wide widths, include both first/last excerpts, duration, parse warning count, and KeepAlive summary.
- Truncation must preserve meaningful suffix/prefix context for session IDs and message excerpts.
- Rows with dangerous or user-actionable states take precedence visually: KeepAlive countdown/sending/failure, expired active workspace, parser warning, watcher degraded, notification degraded.

List-level actions:

| Focusable Action | Primary Activation | Shortcut |
|---|---|---|
| Session row | `enter` opens Session Workspace | none |
| Toggle Reminder for selected session | `enter` on focused action | `r` |
| Toggle KeepAlive for selected session | `enter` on focused action | `k` |
| Refresh list | `enter` on focused action | none |
| Quit | `enter` on focused action | `q` |

Cursor movement uses arrow keys. `j`/`k` movement is not part of v2 because `k` is reserved for KeepAlive.

Required states:

- loading with skeleton rows or a compact loading line, not a blank screen;
- no `~/.claude/projects` directory, with the exact path and the fact that no sessions can be discovered yet;
- no sessions found, with the exact discovery path and a short hint that sessions appear after Claude Code writes JSONL files;
- parse warnings, shown as a row marker plus a details line in Session Workspace;
- ambiguous partial ID, shown as a selection/error state listing matching short IDs and projects;
- watcher unavailable/degraded, shown as a persistent banner while safety refresh remains active;
- notification command unavailable, shown as a degraded banner and per-event in the notification log/status area;
- `claude` unavailable when KeepAlive is armed, shown before countdown can send.

### 10.2 Session Workspace

The Session Workspace is the main per-session control surface. It must make status, evidence, and controls understandable without assuming the user already knows cache/TTL terminology.

The workspace has two visual zones:

1. **Session Evidence:** read-only facts about the selected session.
2. **Controls:** per-session toggles and active KeepAlive workflow card.

Evidence sections, in order:

1. Status: project, full session ID, JSONL path, cache window/TTL, active/expired/unknown state, remaining or expired time, last refresh time.
2. Messages: first and last user-visible excerpts.
3. Token Stats: cache writes, cache reads, output tokens, hit rate, and TTL evidence.
4. Gaps: gap count, reset count, and latest significant gaps.

Control sections, in order:

1. Reminder row: on/off toggle, global threshold summary, fired/next threshold status for this session instance.
2. KeepAlive row/card: on/off toggle, Auto-send toggle, trigger/scope summary, and active KeepAlive card when enabled.
3. Action row: copy/show full ID, manual refresh, help, back, quit.

Session Workspace toggles are commit-on-action for the current TUI run. Reminder thresholds and KeepAlive defaults are edited in Config Editor.

Session Workspace focus order:

```text
Evidence scroll region, only when evidence overflows
Reminder row
KeepAlive row
KeepAlive Auto-send setting
KeepAlive card actions, when present
Footer actions
```

Keyboard movement:

- `up/down` moves focus between rows/actions by default.
- If the Evidence scroll region is focused, `up/down` scrolls evidence instead; moving past the top/bottom returns focus to the adjacent focusable row.
- The cursor may focus the Evidence scroll region when scrollable, Reminder row, KeepAlive row, KeepAlive Auto-send setting, KeepAlive card actions, and footer actions.
- `enter` toggles the focused Reminder row, KeepAlive row, or KeepAlive Auto-send setting, or activates the focused KeepAlive card/footer action.
- `space` toggles the focused checkbox-style control.
- `r` toggles Reminder for the selected session.
- `k` toggles KeepAlive for the selected session.
- `s` sends now only during KeepAlive countdown or manual-ready states.
- `x` cancels/dismisses the current KeepAlive instance only when such an instance exists.
- No `tab` navigation and no `left/right` navigation are required in Session Workspace.
- `esc` returns to List View, or quits when the user entered directly via `--id`.

Footer requirements:

- Normal Workspace footer: `r remind   k keepalive   v details   u update   b/esc back   q quit`.
- Countdown footer: `s send now   x cancel instance   b/esc back   q quit`.
- Confirming footer: `x stop waiting   b/esc back   q quit`.
- Config Editor footer: `up/down move   enter edit   space toggle   s save   d reset (confirm)   esc cancel`.
- Do not show `s` or `x` in the normal footer when no send/cancel action is available.
- Footer copy must reflect the current focus. When evidence is focused, mention scrolling; when actions are focused, mention activation.

KeepAlive section requirements:

- When off: the KeepAlive control must show that no Claude messages will be sent automatically and include a concise preview of what enabling KeepAlive will do.
- The KeepAlive On checkbox toggles monitoring for this session.
- The Auto-send setting toggles whether this session may send the configured Claude message. It must be reachable as a secondary focus target in the KeepAlive row/card. It must be disabled with a visible reason while sending, confirming, or after a hard failure until the user re-enables KeepAlive.
- When monitoring: the KeepAlive card shows effective trigger time, remaining scope, next eligible send time, whether auto-send is enabled, and the exact condition under which a Claude message will be sent.
- During countdown: the KeepAlive card shows a high-visibility countdown, exact message to be sent, target session, remaining scope, `s` to send now, and `x` to cancel.
- During manual-ready state: the KeepAlive card shows that auto-send is off or disabled for this instance, cache expiry risk, `s` to send now, and `x` to dismiss.
- During sending/confirming: the KeepAlive card shows subprocess status and confirmation evidence requirements.
- On success: the KeepAlive card shows confirmation timestamp and updated cache window until the user turns KeepAlive off or the card returns to a compact done state.
- On failure: the KeepAlive card shows failure reason, last attempted command, manual fallback command, and that auto-send is stopped if Claude limit or subprocess errors were detected.
- For unknown TTL: the KeepAlive row/card labels the cache window as unknown and states that KeepAlive uses the conservative 5-minute trigger heuristic.

KeepAlive off row:

```text
[ ] KeepAlive  [x] auto-send · trigger 5m · scope 1 send   Off. No message.
```

KeepAlive card layout:

```text
[x] KeepAlive -------------------------------- <state badge> ----
  State                  <state-specific content>
  Next                   <next state/action/time>
  Claude message         <sent / may send / will not send>
  Message                Keep-alive check. Reply "yes" only.
  Scope                  0/1 sends
  Controls               <state-specific visible actions>
```

State-specific content rows:

| State | State Row | Next Row | Claude Message Row | Controls Row |
|---|---|---|---|---|
| Watching, Auto-send on | `Watching cache expiry` | `Countdown at <time>` | `May send after countdown unless canceled` | none; settings stay in KeepAlive control |
| Watching, Auto-send off | `Watching cache expiry` | `Manual prompt at <time>` | `Will not send automatically` | none; settings stay in KeepAlive control |
| Countdown | `Countdown <24s>` | `Send now or cancel before countdown ends` | `Will send at zero if not canceled` | `Send now`, `Cancel this instance` |
| Manual prompt | `Manual send available` | `Send now or dismiss` | `Not sent` | `Send now`, `Dismiss` |
| Sending | `Sending message` | `Waiting for Claude CLI` | `Send started` | disabled or `Stop waiting` only if cancellable |
| Confirming | `Confirming result` | `Watching this session JSONL` | `Sent, awaiting evidence` | `Stop waiting` |
| Success | `Cache refreshed` | `Monitoring complete or re-armed by scope` | `Sent and confirmed` | `Acknowledge`, `Refresh` |
| Failure | `Failed: <reason>` | `Use manual fallback or re-enable after fixing` | `Stopped or not confirmed` | `Copy command`, `Refresh`, `Turn Off` |
| Scope complete | `Scope complete` | `Turn KeepAlive off or wait for a new eligible cache window` | `No more automatic sends` | `Acknowledge`, `Turn Off` |

This table is the implementation contract for KeepAlive card rendering. Avoid replacing it with prose-only state descriptions.

Session actions:

| Focusable Action | Primary Activation | Shortcut |
|---|---|---|
| Toggle KeepAlive monitoring | `enter` or `space` on focused action | `k` |
| Toggle KeepAlive Auto-send for this session | `enter` or `space` on focused setting | none |
| Toggle Reminder | `enter` or `space` on focused action | `r` |
| Send KeepAlive now, only during countdown/manual-ready | `enter` on focused action | `s` |
| Cancel/dismiss current KeepAlive instance | `enter` on focused action | `x` |
| Manual refresh for this session | `enter` on focused action | none |
| Back to list | `enter` on focused action | `b` / `esc` |
| Quit | `enter` on focused action | `q` |

Session ID remains visible in Session Info. KeepAlive cancellation uses `x` and must not conflict with route navigation.

### 10.3 Reminder vs KeepAlive UX Separation

Reminder and KeepAlive must never share labels, state lines, or notification wording that suggests they do the same thing.

Required wording patterns:

- Reminder: `[ ] Reminder`, `alert at 20%, 10%`, `Sends no Claude message.`
- KeepAlive control off: `[ ] KeepAlive`, `auto-send on`, `Off. No message.`
- KeepAlive watching: `State: Watching cache expiry`, `Next: Countdown at <time>` or `Next: Manual prompt at <time>`, `Claude message: May send after countdown unless canceled` or `Claude message: Will not send automatically`.
- KeepAlive countdown: `State: Countdown <time>`, `Next: Send now or cancel before countdown ends`, `Claude message: Will send at zero if not canceled`.

`cc-watch --remind` enables reminder alarms for loaded sessions only. It must not enable KeepAlive, auto-send, or any Claude subprocess behavior.

### 10.4 Config Editor

The config editor configures Reminder thresholds and KeepAlive defaults.

The config editor must support:

- editing Reminder thresholds as a comma-separated list of percentages;
- editing KeepAlive trigger/countdown/message;
- toggling auto-send;
- configuring max sends;
- saving, cancelling, and resetting defaults.

Usability requirements:

- Show a live summary of the effective KeepAlive behavior, including the TTL-aware trigger examples for 1-hour and 5-minute cache windows.
- Show a visible warning when the Auto-send default is enabled because future sessions may send a Claude message if KeepAlive is turned on.
- Reminder thresholds must validate as whole numbers from 1 to 99 in descending order.
- Max sends must be valid before save.
- Reset defaults must require confirmation.
- Cancel must leave the previous config unchanged.
- Validation errors must appear next to the field and in a compact summary line; invalid config must not save.

It must validate:

- countdown is positive;
- trigger is positive;
- `max_sends` is positive;
- Reminder thresholds are valid whole percentages from 1 to 99;
- countdown plus safety margin can fit inside at least one effective trigger scenario, or the UI explains that auto-send will be disabled for affected sessions.

Config actions:

| Focusable Action | Primary Activation | Shortcut |
|---|---|---|
| Field or option | `enter` edits/selects, `space` toggles boolean options | none |
| Save when valid | `enter` on focused action | `s` |
| Reset defaults | `enter` on focused action, then confirm | `d` |
| Cancel/back | `enter` on focused action | `esc` |

Cursor movement uses arrows within fields/actions.

### 10.5 Cursor-First Navigation Rules

All functions must be reachable through visible cursor navigation. Single-key commands are shortcuts for experienced users, not the primary accessibility path.

Required behavior:

- Every screen has a focusable action row or action list containing all available actions for that state.
- Arrow keys move through content rows and selectable actions.
- `enter` activates the focused action. `space` toggles focused binary controls.
- Single-key commands such as `r`, `k`, `s`, `x`, and `c` mirror visible focused actions; they must never expose hidden functionality that is absent from the cursor path.
- Disabled actions remain focusable when safety-relevant, with a short reason shown inline or in the status line.
- When a dangerous state is active, such as KeepAlive countdown, the focus should move to the relevant action group by default: `Send now`, `Cancel this instance`, `Back`.
- Footers should describe both models: cursor controls first, shortcuts second.

Example action row:

```text
Actions
  > Send now        Cancel this instance        Back to session
    shortcut: s     shortcut: x                 shortcut: b

enter activate focused action   arrows move
```

### 10.6 Shortcut Rules

- `?` is intentionally inert in the TUI. Inline navigation bars are the TUI help surface; CLI `--help` remains the command help surface.
- `r` means toggle Reminder on List View and Session Workspace.
- `d` means reset defaults only inside Config Editor and only with confirmation.
- `s` means save in Config Editor and send-now only inside the KeepAlive area/state. Footer/navigation copy must reflect the current meaning.
- `x` means cancel/dismiss current KeepAlive instance only where such an instance exists.
- Disabled or unavailable safety-relevant actions remain visible in the focused control list with a short reason.


### 10.7 Screen Design Walkthrough

These mockups are structural, not pixel-perfect. The implementation may adjust spacing for terminal width, but it must preserve hierarchy, wording intent, and state visibility.

#### 10.7.1 List View, Normal State

Primary user question: "Which sessions need attention?"

The List View is a triage surface. It should let the user scan active sessions, see which cache windows are close to expiry, and notice automation or degraded states without opening every session.

```text
+ cc-watch ---------------------------------------------- 5 sessions  14:32:18 +
| Refresh: live events + safety refresh      Watcher: ok      Notify: ok       |
+------------------------------------------------------------------------------+
| > d4b247b7  workspace-api     cache 1h   active    05m 44s left             |
|   TTL [######################--] 90%   hit [##################----] 82%      |
|   "write the implementation plan and keep the cache alive..."                |
|   KeepAlive: countdown 24s, auto-send on, scope 0/1 sends used               |
|                                                                              |
|   a3f12c90  sidechorus        cache 5m   expired   1h 51m ago               |
|   "ok looks good, commit"                                                    |
|                                                                              |
|   b8c301e1  billing-docs      cache ?    unknown   no timestamp             |
|   "continue from here"                                  warnings: 2          |
+------------------------------------------------------------------------------+
| 2 active  1 unknown  2 expired   enter open  r remind  k keepalive  ? q       |
+------------------------------------------------------------------------------+
```

Design notes:

- Row line 1 is the scan line: ID, project, cache window, status, time.
- Bars are supporting evidence, not the primary label. Users should not have to decode a bar to know the state.
- KeepAlive appears only as a short row summary in List View. `k` toggles KeepAlive for the selected session and must visibly update the selected row.
- Degraded system states live in the header so the user sees them before trusting the rows.
- The selected row gets richer action hints. Non-selected rows stay quieter.

#### 10.7.2 List View, Narrow Terminal

At narrow widths, density wins over completeness. The list must keep the action-critical columns and drop excerpts first.

```text
+ cc-watch ----------------------------- 5 sessions 14:32:18 +
| Watcher ok  Notify degraded: osascript failed               |
+-------------------------------------------------------------+
| > d4b247b7 workspace-api  1h active   05m44s  KA 24s 0/1   |
|   TTL 90%  hit 82%  last: "write the implementation..."    |
|   a3f12c90 sidechorus     5m expired  1h51m   hit 41%       |
|   b8c301e1 billing-docs   ?  unknown  no ts   warn 2        |
+-------------------------------------------------------------+
| cursor move  enter open focused  shortcuts: r remind  k keepalive  q quit |
+-------------------------------------------------------------+
```

Narrow-mode removal order:

1. Remove first message excerpt.
2. Compress bars to percentages.
3. Shorten footer labels.
4. Keep degraded banners, status, remaining time, and KeepAlive summary.

#### 10.7.3 List View, Empty And First-Run States

First-time users may not know where Claude Code stores session files. Empty states must teach the discovery model without sounding like an error.

```text
+ cc-watch ---------------------------------------------------- 14:32:18 +
| No Claude Code session files found.                                   |
|                                                                       |
| cc-watch looks for JSONL files under:                                  |
|   ~/.claude/projects/                                                 |
|                                                                       |
| Sessions appear here after Claude Code writes conversation history.    |
| Start or resume a Claude Code session, then wait for live refresh.     |
+-----------------------------------------------------------------------+
| q quit                                                                |
+-----------------------------------------------------------------------+
```

If the directory itself is missing, say that explicitly:

```text
No ~/.claude/projects directory exists yet.
cc-watch cannot discover sessions until Claude Code creates that directory.
```

#### 10.7.4 List View, Ambiguous `--id`

Partial ID ambiguity should behave like a selection state, not a vague CLI error.

```text
+ cc-watch -------------------------------- ambiguous session id: d4b +
| The partial id matched more than one session. Choose one to open.      |
+-----------------------------------------------------------------------+
| > d4b247b7  workspace-api   modified 14:29   active   05m44s left     |
|   d4b901aa  workspace-api   modified 12:11   expired  2h18m ago       |
|   d4bc0ee2  docs-review     modified 09:42   unknown  no timestamp    |
+-----------------------------------------------------------------------+
| up/down move  enter open selected  esc list  q quit                   |
+-----------------------------------------------------------------------+
```

#### 10.7.5 Session Workspace, KeepAlive Off

Primary user question: "What is happening in this session, and what can I safely do?"

```text
+ cc-watch / workspace-api / d4b247b7 ----------------------------------------+
| Session details                                                              |
|                                                                              |
| Status                                                                       |
|   Cache window: 1h TTL, active, 05m 44s left                                 |
|   Last message: 14:26:34   Run time: 3h 08m   File refresh: 14:32:18         |
|   JSONL: ~/.claude/projects/workspace-api/d4b247b7.jsonl                    |
|                                                                              |
| Messages                                                                     |
|   First user: can you check whether this session is cached for 5m or 1h?     |
|   Last user:  write the implementation plan and keep the cache alive          |
|                                                                              |
| Token Stats                                                                  |
|   Cache writes: 1,218,696   Cache reads: 29,673,296   Output: 402,353        |
|   Hit rate: 96%   TTL evidence: ephemeral_1h_input_tokens                    |
|                                                                              |
| Gaps                                                                         |
|   0 cache resets detected   Longest gap: 49m 26s, below 1h TTL               |
|                                                                              |
| Controls                                                                     |
|                                                                              |
|   [ ] Reminder   alert at 20%, 10%   Sends no Claude message.                |
|   [ ] KeepAlive  [x] auto-send · trigger 5m · scope 1 send   Off. No message.|
+------------------------------------------------------------------------------+
| arrows move focus   enter/space act   r remind   k keepalive   q quit       |
+------------------------------------------------------------------------------+
```

Design notes:

- Status, Messages, Token Stats, and Gaps are session details. They must not look like controls.
- Reminder and KeepAlive are controls. They use simple checkbox rows and concise state text.
- KeepAlive is visible even when off because it is a dangerous capability. Hidden controls make users guess what enabling it will do.
- The Reminder control explicitly says it will not send Claude messages.
- Reminder thresholds are configured in Config Editor, not in Session Workspace.

#### 10.7.5a Session Workspace, KeepAlive On

When KeepAlive is enabled, the KeepAlive row expands into a card inside Controls. Evidence remains above Controls.

```text
+ cc-watch / workspace-api / d4b247b7 ----------------------------------------+
| Session details                                                              |
|   Status / Messages / Token Stats / Gaps                                     |
|                                                                              |
| Controls                                                                     |
|   [ ] Reminder   alert at 20%, 10%   Sends no Claude message.                |
|   [x] KeepAlive --------------------------------------------- watching ------|
|     State    Watching cache expiry                                           |
|     Next     Countdown at 14:33:02 if session still active                   |
|     Claude message  May send after countdown unless canceled                 |
|     Message  "Keep-alive check. Reply \"yes\" only."                       |
|     Scope    0 / 1 sends · auto-send [x] on                                  |
+------------------------------------------------------------------------------+
| arrows move focus   enter/space act   r remind   k keepalive   q quit       |
+------------------------------------------------------------------------------+
```

#### 10.7.6 KeepAlive Card, Watching

```text
|   [x] KeepAlive --------------------------------------------- watching ------|
|     State    Watching cache expiry                                           |
|     Next     Countdown at 14:33:02 if session still active                   |
|     Claude message  May send after countdown unless canceled                 |
|     Message  "Keep-alive check. Reply \"yes\" only."                       |
|     Scope    0 / 1 sends · auto-send [x] on                                  |
```

This card stays in Controls. It is calm but visually distinct. It should not look like an emergency until the threshold is crossed.

When Auto-send is off:

```text
|   [x] KeepAlive --------------------------------------------- watching ------|
|     State    Watching cache expiry                                           |
|     Next     Manual prompt at 14:33:02 if session still active               |
|     Claude message  Will not send automatically                              |
|     Message  "Keep-alive check. Reply \"yes\" only."                       |
|     Scope    0 / 1 sends · auto-send [ ] off                                 |
```

#### 10.7.7 KeepAlive Card, Countdown

```text
|   [x] KeepAlive -------------------------------------------- countdown ------|
|     State       Countdown 24s  [##################------]                    |
|     Next        Send now or cancel before countdown ends                     |
|     Claude message  Will send at zero if not canceled                        |
|     Message     "Keep-alive check. Reply \"yes\" only."                    |
|     Target      d4b247b7 · workspace-api                                     |
|     Scope       0 / 1 sends · will count as 1 if sent                        |
+------------------------------------------------------------------------------+
| arrows choose action   enter act   s send now   x cancel instance   q quit    |
```

Countdown UX requirements:

- The exact outgoing message is visible before send.
- `x` cancels only this instance, not the entire configured feature.
- The footer must promote `s` and `x` while countdown is active.
- If `claude` is unavailable, the countdown must not proceed to a hidden failed subprocess. It should switch to an explicit failure/manual state.

#### 10.7.8 KeepAlive Card, Manual Prompt

This state appears when per-session Auto-send is off or when auto-send is disabled for the instance because the trigger window is too short.

```text
|   [x] KeepAlive ----------------------------------------- manual prompt -----|
|     State    Manual send available                                           |
|     Next     Send now or dismiss                                             |
|     Claude message  Not sent                                                 |
|     Message  "Keep-alive check. Reply \"yes\" only."                       |
|     Scope    0 / 1 sends                                                     |
+------------------------------------------------------------------------------+
| arrows choose action   enter act   s send now   x dismiss   q quit            |
```

This is intentionally not called "awaiting confirmation" because no send has happened yet.

#### 10.7.9 KeepAlive Card, Sending And Confirming

```text
|   [x] KeepAlive ---------------------------------------------- sending ------|
|     State    Sending message                                                 |
|     Next     Waiting for Claude CLI                                          |
|     Claude message  Send started                                             |
|     Running  claude -r d4b247b7 -p "Keep-alive check. Reply yes only."       |
```

Then:

```text
|   [x] KeepAlive -------------------------------------------- confirming -----|
|     State    Confirming result                                               |
|     Next     Watching this session JSONL                                     |
|     Claude message  Sent, awaiting evidence                                  |
|     Evidence watching d4b247b7.jsonl for a new entry after 14:32:58          |
+------------------------------------------------------------------------------+
| arrows choose action   enter act   x stop waiting   b back   q quit           |
```

Confirmation is evidence-based. A successful subprocess exit is not enough.

#### 10.7.10 Session Workspace, Success And Failure

Success:

```text
|   [x] KeepAlive ------------------------------------------------ done -------|
|     State    Cache refreshed                                                 |
|     Next     Monitoring complete                                             |
|     Claude message  Sent and confirmed                                       |
|     Evidence Cache refreshed at 14:33:19 · new window active, 58m 41s left   |
|     Scope    1 / 1 sends used                                                |
```

Failure, `claude` unavailable:

```text
|   [x] KeepAlive ---------------------------------------------- failed ------|
|     State    Failed: claude command not found                                |
|     Next     Use manual fallback or turn KeepAlive off                       |
|     Claude message  Stopped or not confirmed                                 |
|     Scope    1 / 1 sends used. Auto-send stopped for this session.           |
|                                                                              |
|     Manual fallback:                                                          |
|       claude -r d4b247b7-be64-417c-a81c-ba686b464cf5 \                       |
|         -p "Keep-alive check. Reply yes only."                               |
|                                                                              |
|     Fix PATH or send manually, then refresh.                                  |
```

Failure, Claude limit:

```text
|   [x] KeepAlive ---------------------------------------------- failed ------|
|     State    Failed: Claude reported a limit error                           |
|     Next     Check Claude Code before re-enabling                            |
|     Claude message  Stopped or not confirmed                                 |
|     Scope    1 / 1 sends used. Auto-send stopped.                            |
```

#### 10.7.11 Config Editor

Primary user question: "What reminder and KeepAlive defaults should future sessions use?"

```text
+ cc-watch config -------------------------------------------------------------+
| Reminder                                                                     |
|   Alert at:              [20, 10] %                                           |
|                                                                              |
| KeepAlive automation                                                         |
|   Trigger before expiry: [5] minutes                                          |
|   Countdown:             [30] seconds                                         |
|   Message:               [Keep-alive check. Reply "yes" only.]               |
|   Auto-send:             [x] enabled, sends Claude message                    |
|   Max sends: [1]                                                              |
|                                                                              |
| What will happen                                                             |
|   1h cache: countdown starts at 05m left, auto-send after 30s unless canceled |
|   5m cache: countdown starts at 01m left, auto-send after 30s unless canceled |
|   Scope: stop after 1 attempted or successful send                            |
|                                                                              |
| Validation                                                                   |
|   OK                                                                         |
+------------------------------------------------------------------------------+
| up/down move  enter edit  space toggle  s save  d reset(confirm)  esc cancel |
+------------------------------------------------------------------------------+
```

Validation error example:

```text
| KeepAlive automation                                                         |
|   Countdown: [120] seconds                                                    |
|   Error: countdown may not fit the 5m cache trigger window.                   |
|                                                                              |
| Validation                                                                   |
|   Cannot save. Reduce countdown or accept that auto-send will be disabled     |
|   for affected sessions.                                                      |
```

Reset defaults must use inline confirmation, not a modal-style interruption:

```text
| Reset defaults? This will replace KeepAlive defaults.                         |
| Press d again to confirm, esc to keep current settings.                       |
```

#### 10.7.12 Inline Navigation Bars

The TUI does not render a separate help overlay. Each surface carries compact navigation bars that expose current cursor movement, activation, and available shortcuts without hiding active dangerous state lines.

```text
up/down focus  enter/space act  r remind  k KeepAlive  v details  u update  q quit
```

### 10.8 UX Process Workflows

#### 10.8.1 First Launch Workflow

```text
User runs cc-watch
  -> app loads config
  -> discovers ~/.claude/projects
  -> if directory missing, show first-run empty state
  -> if files exist, parse recent sessions
  -> show List View with live header state
```

Success criteria:

- The user learns where data comes from.
- The user can quit or refresh without reading documentation.
- No empty/loading state looks like a crash.

#### 10.8.2 Cache Triage Workflow

```text
List View
  -> user scans status and remaining time
  -> selected row reveals richer KeepAlive hint if relevant
  -> user opens the highest-risk session with enter
  -> Session Workspace explains evidence and controls
  -> user chooses reminder, KeepAlive, manual refresh, copy ID, or back
```

The flow is intentionally list-first. List View answers "where should I look?" Session Workspace answers "what should I do?"

#### 10.8.3 Reminder Workflow

```text
Session Workspace
  -> user focuses Reminder
  -> user toggles On
  -> active thresholds come from global config
  -> when a threshold crosses, notification fires once
  -> no Claude subprocess starts
```

Reminder UX invariant: every Reminder label must reinforce that it is an alarm only.

#### 10.8.4 Bounded KeepAlive Workflow, Auto-Send On

```text
Session Workspace
  -> user focuses KeepAlive On or presses k
  -> KeepAlive monitoring starts for this session
  -> Auto-send uses this session's visible Auto-send setting
  -> UI shows message, trigger, countdown, scope, and visible cancellation controls
  -> threshold crosses
  -> countdown state starts and notification fires
  -> user can focus Send now or Cancel, or use shortcuts s/x
  -> countdown reaches zero
  -> Claude subprocess starts
  -> app confirms by watching this session JSONL
  -> success or failure is shown
  -> scope count updates
  -> monitoring stops if scope is complete
```

Safety criteria:

- No send happens without visible countdown state first.
- No repeated send loop can occur on a one-second tick.
- Scope counts attempts, not only successes, so a failing subprocess cannot loop forever.
- Any Claude limit or subprocess error stops auto-send for that session.

#### 10.8.5 Manual KeepAlive Workflow, Auto-Send Off

```text
Session Workspace
  -> user focuses KeepAlive Auto-send and turns it off
  -> user enables KeepAlive for this session
  -> threshold crosses
  -> manual_ready state appears
  -> no Claude message has been sent
  -> user focuses Send now or Dismiss, or uses shortcuts s/x
```

UX wording must avoid "confirming" until after a send has actually happened.

#### 10.8.6 Degraded Watcher Workflow

```text
Watcher setup fails or misses events
  -> header shows Watcher degraded
  -> safety refresh continues
  -> manual refresh remains available
  -> JSON output includes degraded refresh state
```

The TUI should never imply live filesystem events are perfect. Degraded watcher state is a reduced-freshness state, not a full app failure.

#### 10.8.7 Notification Failure Workflow

```text
Notification command fails
  -> current event still updates TUI state
  -> header/status area shows Notify degraded
  -> recent notification attempt says delivery failed
  -> repeated identical command failures are suppressed until next event/manual refresh
```

User-visible distinction: KeepAlive or Reminder event may have happened even when the OS notification failed.

#### 10.8.8 Config Editing Workflow

```text
cc-watch config
  -> fields load from config or defaults
  -> user edits values inline
  -> effective behavior summary updates
  -> validation errors block save
  -> s saves only when valid
  -> esc cancels with no file write
  -> d requires repeat confirmation before reset defaults
```

The config editor edits defaults, not active session runtime state. Per-session toggles still live in Session Workspace.

### 10.9 First-Time User Comprehension Rules

A user new to cache and TTL concepts should be able to infer the basics from the UI.

Required copy rules:

- On first exposure in each screen, pair `TTL` with `cache window`.
- Prefer `active`, `expired`, and `unknown` over cache jargon alone.
- Explain unknown TTL as missing evidence, not a parser failure by default.
- Keep action labels literal: `send now`, `cancel`, `refresh`, `copy ID`, `save`.
- Present visible actions before shortcut labels in footers and help text.
- Keep dangerous verbs attached to their object: `Sends Claude message`, not just `Auto-send`.


---

## 11. Notifications

Notifications are always attempted for reminder and KeepAlive events while cc-watch is running, but notification failures must not create noisy retry loops.

Requirements:

- Escape notification title/body safely. Do not build unescaped AppleScript strings.
- If notification execution fails, show a visible degraded state in the TUI.
- Record recent notification attempts/results in an in-memory status area so users can distinguish "event did not happen" from "notification could not be delivered".
- Suppress repeated failure notifications/status churn for the same command failure until the next distinct event or manual refresh.
- macOS is the supported product platform. Linux notification behavior is out of scope unless re-approved.

Notification wording must preserve Reminder vs KeepAlive separation:

- Reminder notifications describe an alarm threshold only.
- KeepAlive notifications describe automation state and whether a Claude message was or was not sent.

Notification events:

- reminder threshold crossed;
- KeepAlive countdown started;
- KeepAlive manual prompt shown;
- KeepAlive sent;
- KeepAlive success;
- KeepAlive failure;
- KeepAlive scope complete;
- long-running/endless automation warning if that mode is ever added.

---

## 12. Config And State Files

Global config:

```text
~/.config/cc-watch/config.json
```

Global config includes:

- Reminder thresholds;
- KeepAlive trigger before expiry;
- KeepAlive countdown;
- KeepAlive message;
- KeepAlive auto-send default;
- KeepAlive max sends;
- app-level retention or display defaults if added later.

Global config owns Reminder thresholds. Reminder enabled/disabled state is per session and runtime-only for the current TUI run.

When a session first creates KeepAlive state, its Auto-send value is copied from the global KeepAlive default. Changing global config later does not silently change active per-session KeepAlive state.

Runtime per-session state includes:

- reminders enabled;
- KeepAlive enabled;
- KeepAlive auto-send setting for that session;
- KeepAlive sent count;
- fired reminder thresholds;
- last KeepAlive attempt/result;
- cancellation state.

Runtime state is in-memory and is discarded when cc-watch exits.

---

## 13. JSON Output

`--json` is the stable machine-readable interface.

It must include:

- generated timestamp;
- schema version;
- session list or one selected session;
- parser warnings;
- active/degraded refresh state when relevant;
- reminder/KeepAlive state when available.

This is the extension point for scripts and a future native macOS app. The native app remains out of scope.

---

## 14. Packaging

v2 currently targets:

- macOS arm64;
- macOS amd64.

Go minimum is 1.23+ unless dependencies are deliberately pinned to support an older Go version.

Simple local install is the first supported distribution path. Homebrew, goreleaser, GitHub Release publishing, and Linux packages are out of scope unless re-approved.

---

## 15. Implementation Readiness Gate

Implementation planning may begin only after this design resolves:

- parser parity contract;
- refresh architecture with fsnotify plus safety refresh;
- Bubbletea watcher message integration;
- KeepAlive bounded state machine;
- keybinding conflicts;
- notification degraded states;
- Go version/dependency floor;
- local install contract.
