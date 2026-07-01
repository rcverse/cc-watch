# cc-watch v2 Visual Grammar

**Status:** Active post-acceptance refinement note
**Date:** 2026-06-29

This note documents the terminal visual grammar used by the Go/Bubbletea TUI after the post-acceptance coherency pass.

## Principles

- The TUI is an operator surface. Color supports scanning; it does not decorate.
- Use light, low-saturation terminal colors by default. Reserve stronger visual emphasis for active automation, expiry, failures, and degraded system behavior.
- Color is never the only signal. Active, expired, and unknown states include text plus a Unicode marker.
- Inline navigation bars replace the in-app help overlay. CLI `--help` remains part of the public command contract.
- List View loads 25 recent sessions by default and paginates visually at four complete sessions per page. `--n N` still controls how many sessions are loaded; `←`/`→` page cues appear only when another loaded page exists.
- The normal List header is only `Claude Code Watch` plus a rule. Degraded watcher state appears in warning banners, not as routine header metadata.

## Palette Roles

| Role | xterm color | Usage |
|---|---:|---|
| Identity | 111 | App title, session IDs, projects, current surface, selected page affordance; saturated readable blue |
| Muted | 245 | Secondary metadata, inactive affordances, footers |
| Separator | 242 | Rules and panel structure |
| Cache tier | 250 | Cache-window labels such as `1-hour cache` |
| First message label | 187 | `first` excerpt label; light khaki |
| Last message label | 109 | `last` excerpt label; cool muted blue-gray, distinct from `first` |
| Reminder | 110 | Reminder labels; alarm-only and non-automation |
| KeepAlive | 147 | KeepAlive labels; potentially sends a Claude message; muted purple |
| Info | 109 | Informational notices |
| Warning | 179 | Countdown, armed automation, and recoverable attention states |
| Danger | 167 | Expired cache, failed automation, invalid config |
| Success | 108 | Active cache, successful outcomes, healthy hit-rate |
| Disabled | 244 | `N/A`, unavailable controls, inactive progress remainder |
| Degraded | 173 | System degraded states without reusing saturated orange |

## State Vocabulary

- Active cache: `● Active` in List View and `● ACTIVE` in Workspace.
- Expired cache: `× Expired` in List View and `× EXPIRED` in Workspace.
- Unknown cache: `○ Unknown` in List View and `○ UNKNOWN` in Workspace.
- List status labels use a fixed label column before remaining/expired time, e.g. `● Active    45m22s left` and `× Expired   8h54m02s ago`.
- Expired KeepAlive controls use `N/A after expiry`, not `unavailable`; the styling is disabled/muted rather than warning orange.
- Expired List rows render automation chips as disabled `KeepAlive N/A` / `remind N/A`.

## Workspace Controls

- Controls use three visual columns: neutral option label, semantic state, muted explanation.
- Option labels (`Reminder`, `KeepAlive`, `Auto-send`, `Back`, `Notice`) do not reuse Reminder/KeepAlive accent colors. Accent colors belong to the state or active content, not the menu label.
- Detail copy is short, lower-case helper text: `notify at 20%, 10%`, `5m before expiry · 1 send`, `send after countdown`, `manual prompt only`, `session list`.
- Expired controls share one unavailable grammar: state `N/A`, detail `after expiry`. Do not use `disabled while KeepAlive is N/A`, `unavailable`, or warning/orange styling for expiry.
- Transient Workspace notices appear as a `Notice` row inside Controls so the page frame and vertical rhythm stay stable.
- Config rows follow the same row grammar: neutral field label, semantic value, muted helper detail. Safety-critical explanation, such as Auto-send sending a Claude message, belongs in the warning/preview copy rather than the compact row detail.

## Navigation

- List footer: selection, `←`/`→` page navigation when paged, open, Reminder, KeepAlive, update, config, quit.
- Workspace footer: focus/action controls plus only currently valid KeepAlive actions.
- Config footer: move, edit, toggle, save, reset, cancel.
- The TUI no longer renders a separate help overlay from `?`; the footer/navigation bars are the help surface.
