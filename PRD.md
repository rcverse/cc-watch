# cc-cache — Product Requirements Document

**Version:** 2.0 target
**Status:** v2 design synced
**Owner:** Richard Chen
**Last updated:** 2026-06-03

Detailed implementation and UI contract:
`docs/superpowers/specs/2026-06-02-cc-cache-v2-design.md`

---

## 1. Product Summary

cc-cache is a terminal TUI for inspecting Claude Code session cache health and managing bounded reminder and KeepAlive workflows.

It reads Claude Code JSONL session files from `~/.claude/projects/`, shows whether session cache windows are active, expired, or unknown, and helps users avoid accidental cache expiry during long work sessions.

v2 is a Go rewrite using Bubbletea, lipgloss, fsnotify, OS notifications, and the Claude CLI subprocess only for explicit bounded KeepAlive behavior.

---

## 2. Problem

Claude Code session cache behavior is valuable but mostly invisible to users. A user cannot easily see:

- whether a recent session still has a warm cache window;
- whether the session appears to be using a 5-minute or 1-hour cache window;
- when a cache window is close to expiry;
- whether mid-session pauses likely caused cache resets;
- whether cache reads/writes suggest efficient reuse;
- whether any automation has sent, may send, or will not send a Claude message.

The result is uncertainty, surprise cold starts, and unsafe ad hoc keep-alive behavior.

---

## 3. Goals

1. **Fast cache triage:** `cc-cache` opens to a scan-friendly list of recent sessions.
2. **Session clarity:** each Session Workspace separates read-only evidence from interactive controls.
3. **Beginner comprehension:** the UI explains cache windows/TTL without requiring prior cache terminology.
4. **Safe reminders:** Reminder is an alarm only; it never sends Claude messages.
5. **Bounded KeepAlive:** KeepAlive is visible, cancellable, scoped, and evidence-confirmed.
6. **Reliable live refresh:** filesystem events accelerate updates, while safety refresh and manual refresh cover missed events.
7. **Stable machine output:** `--json` gives scripts and future clients a stable interface.
8. **Installable single binary:** v2 ships through GitHub Releases and a Homebrew tap.

---

## 4. Non-Goals

- No native macOS app in v2.
- No background daemon in v2.
- No public watch mode or configurable watch interval.
- No unbounded or hidden KeepAlive loop.
- No network/API calls to Anthropic.
- No Windows support target for v2.
- No mouse-first interaction requirement.

---

## 5. Primary Users

| User | Need |
|---|---|
| Claude Code user | Know whether a session cache is active before resuming work |
| Multi-session power user | Compare recent sessions and choose which needs attention |
| Long-session user | Receive alarms before cache expiry |
| AFK user | Allow one bounded, visible KeepAlive attempt if configured and not cancelled |
| Script/tooling user | Consume session cache state through JSON |

---

## 6. CLI Contract

```text
cc-cache                     # List View, default recent sessions
cc-cache --n N               # List View, N recent sessions
cc-cache --id <partial-id>   # Session Workspace for one session
cc-cache --json              # Machine-readable JSON, then exit
cc-cache --json --id <id>    # JSON for one session, then exit
cc-cache --remind            # Start TUI with reminders enabled for loaded sessions
cc-cache config              # Config Editor
cc-cache --help              # Help
cc-cache --version           # Version
```

`--watch` is not part of v2. Live refresh is internal TUI behavior.

Partial IDs resolve against JSONL filename stems. Ambiguous partial IDs must show a clear selection/error state.

---

## 7. Core UX Surfaces

### 7.1 List View

Primary question: **Which sessions need attention?**

The List View shows recent sessions sorted by JSONL file modification time. It prioritizes:

1. selected row marker;
2. short session ID;
3. project name;
4. cache window label: `1h`, `5m`, or `TTL ?`;
5. status: `active`, `expired`, or `unknown`;
6. remaining time or expired age;
7. TTL elapsed evidence when available;
8. hit-rate summary;
9. message excerpt;
10. warning/degraded markers;
11. short KeepAlive summary when relevant.

Required actions must be reachable by cursor focus and `enter`; shortcuts are accelerators:

| Action | Shortcut |
|---|---|
| Open selected session | none |
| Toggle Reminder for selected session | `r` |
| Toggle KeepAlive for selected session | `k` |
| Refresh list | none |
| Help | `?` |
| Quit | `q` |

Arrow keys are the movement model. `k` is reserved for KeepAlive, not vim-style down navigation.

### 7.2 Session Workspace

Primary question: **What is happening in this session, and what can I safely do?**

The Session Workspace has two visual zones:

1. **Session Evidence:** read-only facts.
2. **Controls:** per-session Reminder and KeepAlive controls.

Evidence sections:

- Status: project, full session ID, JSONL path, cache window/TTL, state, remaining/expired time, refresh time.
- Messages: first and last user-visible excerpts.
- Token Stats: cache writes, cache reads, output tokens, hit rate, TTL evidence.
- Gaps: gap count, reset count, latest significant gaps.

Control sections:

- Reminder row: per-session on/off toggle, global threshold summary, no-Claude-message safety copy.
- KeepAlive row/card: per-session on/off toggle, Auto-send setting, trigger/scope summary, active workflow card when enabled.
- Action row: copy/show full ID, manual refresh, help, back, quit.

All controls must be reachable through cursor navigation. `enter` activates focused actions and `space` toggles checkbox-style controls.

### 7.3 Config Editor

Primary question: **What Reminder and KeepAlive defaults should future sessions use?**

The Config Editor edits global defaults only. It does not edit active per-session runtime state.

It must support:

- editing Reminder thresholds as a comma-separated list of percentages;
- editing KeepAlive trigger, countdown, and message;
- toggling the KeepAlive Auto-send default;
- configuring max sends;
- saving, cancelling, and resetting defaults with confirmation.

---

## 8. Reminder Requirements

Reminder is an alarm system, independent of KeepAlive.

Behavior:

- Per-session toggle during the current TUI run.
- Thresholds are global config values.
- Default thresholds are `20%` and `10%` remaining.
- Fires once per threshold crossing per active session instance.
- Does not send Claude messages.
- Does not alter KeepAlive state.
- Notifications fire only while cc-cache is running.

Required UI copy must preserve this distinction:

```text
[ ] Reminder   alert at 20%, 10%   Sends no Claude message.
```

`cc-cache --remind` enables reminder alarms for loaded sessions only. It must not enable KeepAlive, Auto-send, or any Claude subprocess behavior.

---

## 9. KeepAlive Requirements

KeepAlive is a bounded, user-visible workflow for preserving an active cache window. It is not hidden automation.

Core concepts:

- **Monitoring:** per-session toggle; default off.
- **Auto-send:** per-session setting initialized from global defaults.
- **Scope:** required boundary for auto-send behavior.
- **Countdown:** visible pre-send state for Auto-send-on sessions.
- **Manual prompt:** visible send option when Auto-send is off or unsafe.
- **Confirmation:** session-specific JSONL evidence after a send attempt.

Default config:

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

An endless mode is not part of normal v2.

### 9.1 Trigger And Send Timing

`trigger_before_expiry_m` is TTL-aware:

```text
configured_trigger_s = keep_alive.trigger_before_expiry_m * 60
ttl_fraction_trigger_s = ttl_seconds * 0.20
effective_trigger_s = min(configured_trigger_s, ttl_fraction_trigger_s)
```

Default outcomes:

- 1-hour cache window: trigger at 5 minutes remaining.
- 5-minute cache window: trigger at 1 minute remaining.
- unknown TTL: use the conservative 5-minute trigger heuristic.

Automatic sending must be scheduled at least 30 seconds before expiry.

```text
safety_margin_s = 30
effective_countdown_s = min(configured_countdown_s, effective_trigger_s - safety_margin_s)
```

If the safety margin cannot be preserved, Auto-send is disabled for that instance and the UI shows a manual prompt.

### 9.2 User-Facing Paths

Auto-send on:

```text
KeepAlive off
  -> user enables KeepAlive
  -> watching cache expiry
  -> countdown starts at trigger
  -> user may send now or cancel
  -> countdown reaches zero
  -> cc-cache runs claude -r <session> -p <message>
  -> cc-cache confirms by watching this session JSONL
  -> success, failure, or scope complete is shown
```

Auto-send off:

```text
KeepAlive off
  -> user disables Auto-send for this session
  -> user enables KeepAlive
  -> watching cache expiry
  -> manual prompt appears at trigger
  -> no Claude message has been sent
  -> user may send now or dismiss
```

Unsafe or failed automation:

```text
trigger window too short, claude unavailable, subprocess error, timeout, or Claude limit
  -> failure/manual fallback
  -> Auto-send stops where relevant
  -> scope counts attempted sends exactly once when a send attempt starts
```

Every KeepAlive state must show whether a Claude message has been sent, may be sent, or will not be sent.

---

## 10. Data Requirements

Source data comes from `~/.claude/projects/**/*.jsonl`. cc-cache must not write to session JSONL files.

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

Parser compatibility requirements:

- Sort sessions by JSONL file modification time.
- Resolve partial IDs against filename stems.
- Ignore malformed JSONL lines without failing the whole file.
- Parse top-level `usage` and nested `message.usage`.
- Support nested cache creation token structures.
- Treat `ephemeral_1h_input_tokens > 0` as 1-hour cache.
- Treat `ephemeral_5m_input_tokens > 0` as 5-minute cache when 1-hour evidence is absent.
- Display unknown TTL when no TTL evidence exists.
- Treat unknown TTL as 5 minutes for conservative gap/reset analysis.
- Record gaps over 60 seconds.
- Mark reset when a gap exceeds the effective TTL.
- Compute hit rate as `cache_read / (cache_read + cache_create)`.

Time-derived values such as elapsed seconds and remaining seconds are computed from the current clock, not stored parser output.

---

## 11. Live Refresh Requirements

v2 has two update streams:

- **Display tick:** every second, recompute time-derived display fields and countdowns.
- **Data refresh:** fsnotify events plus debounced safety refresh parse changed/new/deleted JSONL files.

fsnotify is an event accelerator, not the only source of truth.

Requirements:

- Watch existing project directories recursively.
- Watch the root projects directory for new project directories.
- Add watches for newly created directories.
- Handle create, write, rename, and delete events.
- Debounce bursts.
- Run safety refresh for missed filesystem events.
- Provide manual refresh from the TUI.
- Send watcher events into Bubbletea as messages, not hidden goroutine mutation.
- Surface watcher degraded states in the TUI and JSON.

---

## 12. Notifications

Notifications are always attempted for Reminder and KeepAlive events while cc-cache is running.

Requirements:

- macOS uses `osascript`; Linux uses `notify-send`.
- Notification title/body must be escaped safely.
- Notification failure must show as a degraded state in the TUI.
- The TUI must distinguish "event happened" from "notification delivery failed".
- Repeated identical notification failures should be suppressed until a distinct event or manual refresh.
- Reminder notification wording must describe an alarm only.
- KeepAlive notification wording must describe automation state and whether a Claude message was or was not sent.

Notification events:

- Reminder threshold crossed.
- KeepAlive countdown started.
- KeepAlive manual prompt shown.
- KeepAlive sent.
- KeepAlive success.
- KeepAlive failure.
- KeepAlive scope complete.

---

## 13. Config And Runtime State

Global config file:

```text
~/.config/cc-cache/config.json
```

Global config includes:

- Reminder thresholds;
- KeepAlive trigger before expiry;
- KeepAlive countdown;
- KeepAlive message;
- KeepAlive Auto-send default;
- KeepAlive max sends;
- future app-level display/retention defaults if needed.

Runtime per-session state is in-memory and discarded when cc-cache exits.

Runtime state includes:

- Reminder enabled;
- KeepAlive enabled;
- KeepAlive Auto-send setting for that session;
- KeepAlive sent count;
- fired Reminder thresholds;
- last KeepAlive attempt/result;
- cancellation state.

Changing global config does not silently mutate already-active per-session KeepAlive state.

---

## 14. JSON Output

`--json` is the stable machine-readable interface.

It must include:

- generated timestamp;
- schema version;
- session list or one selected session;
- parser warnings;
- active/degraded refresh state when relevant;
- Reminder/KeepAlive state when available.

JSON output is the integration seam for scripts and future clients.

---

## 15. Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.23+ |
| TUI | Bubbletea |
| Styling | lipgloss |
| Components | bubbles |
| JSONL parsing | Go standard library |
| File events | fsnotify |
| Notifications | `osascript` on macOS, `notify-send` on Linux |
| KeepAlive execution | Claude CLI subprocess |
| Packaging | goreleaser + Homebrew tap |

---

## 16. Packaging

v2 targets:

- macOS arm64;
- macOS amd64;
- Linux amd64;
- Linux arm64.

Distribution must use GitHub Releases and a Homebrew tap via goreleaser.

---

## 17. Acceptance Criteria

v2 is product-complete when:

- List View makes active, expired, unknown, degraded, and KeepAlive states scanable.
- Session Workspace cleanly separates evidence from controls.
- Reminder thresholds are global config; per-session Reminder is a runtime toggle.
- Reminder never sends Claude messages.
- KeepAlive is per-session, visible, bounded, cancellable, and evidence-confirmed.
- Auto-send behavior is explicit and disabled when the safety margin cannot be preserved.
- Scope prevents repeated send loops and counts attempted sends.
- All functions are reachable by cursor navigation; shortcuts are accelerators only.
- Empty, loading, error, parse-warning, watcher-degraded, notification-degraded, and `claude`-unavailable states are visible.
- Go parser parity is verified before TUI build-out.
- Watcher/Bubbletea integration uses explicit messages.
- `--json` remains stable and includes degraded state.
- Packaging produces installable macOS/Linux binaries and a Homebrew path.
