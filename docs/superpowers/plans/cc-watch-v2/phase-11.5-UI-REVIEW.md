# Phase 11.5 UI Review: Visual Elegance And TUI UX E2E

**Scope:** Current Phase 11.5 Go/Bubbletea TUI after visual polish, audited against the v2 TUI UX contract, v1 Python visual baseline, and practical CLI/TUI best practices: fit common 80x24 terminals, keep critical state visible, preserve stable navigation, make focus obvious, avoid chrome that crowds content, and ensure color is never the only signal.

**Audit method:** Static review of `internal/tui`, render tests, and PTY e2e checks for List View, first-run empty state, Session Workspace, Config Editor, and KeepAlive manual prompt. No real Claude send was run.

## Scorecard

| Pillar | Score | Assessment |
|---|---:|---|
| Copywriting | 3/4 | Safety copy is mostly clear, but long lines and duplicated labels reduce scan speed. |
| Visuals | 2/4 | Panels and bars improve v2, but viewport clipping and unbounded content still make the product feel rough. |
| Color | 2/4 | Semantic roles exist, but borders/focus panels do not use them meaningfully; some states rely on plain text only. |
| Typography | 2/4 | Monospace alignment is serviceable, but hierarchy is inconsistent and labels compete with content. |
| Spacing | 1/4 | At 80x24, key surfaces exceed the viewport and lose header/context. |
| Experience Design | 2/4 | Core flows work, but focus, empty states, dangerous-state pinning, and e2e visual QA are not strong enough for packaging. |

**Overall:** 12/24. Do not package as the default installed v2 UI yet.

## Findings

### P1: Workspace and Config are not height-aware, so critical context disappears in normal terminals

Evidence:
- `Model` stores width but not height: `internal/tui/model.go:85-109`.
- `tea.WindowSizeMsg` records only width: `internal/tui/update.go:134-136`.
- Workspace renders header, System, Evidence, Controls, and footer as one long string without viewport budgeting: `internal/tui/workspace.go:26-33`.
- Config renders all groups and footer as one long string: `internal/tui/config_editor.go:29-84`.
- PTY e2e at 80x24 showed Workspace and Config starting mid-panel, with the header and top panel border scrolled out of the captured viewport.

Why this violates CLI/TUI best practice:
Critical location, mode, focus, and dangerous-state context must remain visible. Terminal apps should explicitly budget header, body, and footer regions for common 80x24 terminals.

Recommended fix:
Track `height`, update it from `tea.WindowSizeMsg`, and render with a viewport budget. Keep header and footer pinned; scroll only the main body. For Workspace, make Evidence scrollable first and keep Controls visible. For Config, either compress to a denser form or make field groups scrollable with a sticky validation/footer area.

### P1: Generic panel rendering is unbounded, so any long dynamic line can break 80-column layout

Evidence:
- `RenderPanel` sizes itself to the longest body line with no maximum or wrapping: `internal/tui/styles.go:56-76`.
- Only Workspace JSONL path is manually truncated: `internal/tui/workspace.go:78`.
- Dynamic lines from notifications, degraded watcher messages, fallback commands, config validation, or long projects/messages can still expand panels beyond the terminal.

Why this violates CLI/TUI best practice:
Terminal UI components need deterministic width behavior. Long external strings should be wrapped or truncated at the component boundary, not fixed case-by-case.

Recommended fix:
Make `RenderPanel` accept width or use `m.width`. Add `WrapLine` / `TruncateLine` helpers with suffix-preserving path truncation. Apply consistently to system messages, notifications, fallback commands, projects, excerpts, and config fields.

### P2: Expired cache progress shows impossible percentages, weakening trust

Evidence:
- List View prints raw `PercentElapsed`: `internal/tui/list.go:126-128`.
- Workspace prints raw `PercentElapsed`: `internal/tui/workspace.go:71-72`.
- PTY smoke showed `TTL 17437%` and `17437%` in Workspace for an expired 1h fixture, while the progress bar clamps to full.

Why this violates CLI/TUI best practice:
Progress indicators should be interpretable. A full danger bar plus `expired 173h ago` is clear; `17437%` reads like a bug.

Recommended fix:
For expired sessions, display `expired` plus elapsed-overrun text, or cap the label at `100%+`. Keep the actual expired duration as the primary evidence. Example: `cache window 1h [expired] 173h ago` and `TTL ████████████ expired`.

### P2: Focus indication is too weak and inconsistent for keyboard-driven operation

Evidence:
- Header shows `focus: ...`, but most controls do not visibly mark the focused row.
- List rows use `>` only: `internal/tui/list.go:100-115`.
- Workspace controls render Reminder/KeepAlive rows without a focus marker tied to `FocusedAction`: `internal/tui/workspace.go:113-131`.
- Config fields render the same whether focused or not: `internal/tui/config_editor.go:31-63`.

Why this violates CLI/TUI best practice:
Keyboard-first TUIs need visible focus at the action itself. A status label in the header is not enough, especially when the header can scroll away.

Recommended fix:
Add a shared `FocusMarker(actionID, line)` helper. Render `>` or selected styling on the actual focused config field, Reminder row, KeepAlive row, Auto-send control, and footer actions. Tests should assert visible focus moves through the main flows.

### P2: Empty and ambiguous states still use pre-polish plain text

Evidence:
- Empty states bypass `RenderPanel` and render plain lines: `internal/tui/list.go:22-35`.
- Ambiguous partial-ID view is still a plain text layout: `internal/tui/list.go:51-65`.
- PTY first-run flow showed the missing path clipped at 80 columns and no visual state panel.

Why this violates CLI/TUI best practice:
Error/empty states are high-value onboarding surfaces. They should be as structured as normal states, especially when they explain where session discovery happens.

Recommended fix:
Render empty and ambiguous states inside bounded panels with concise title, exact path using path-truncation, and available actions. Preserve the exact path either in a wrapped line or a copyable detail line.

### P3: Visual chrome consumes too much vertical space and duplicates headings

Evidence:
- Workspace renders a standalone `Session Evidence` label and then an `Evidence` panel title: `internal/tui/workspace.go:29-30`.
- It also renders `Controls` and then a `Controls` panel: `internal/tui/workspace.go:31-32`.
- Config stacks four full bordered panels and footer, which exceeds 24 rows.

Why this violates CLI/TUI best practice:
Borders are useful, but excessive framing crowds operational content. In 80x24 terminals, vertical economy matters more than decorative structure.

Recommended fix:
Remove duplicated standalone headings. Prefer one outer panel per region, with internal dividers for `Status`, `Messages`, `Token Stats`, and `Gaps`. Consider compact one-line System state in the header instead of a full System panel when no degraded state exists.

### P3: Semantic visual roles exist but are underused

Evidence:
- `DefaultStyles` defines roles: `internal/tui/styles.go:29-40`.
- `RenderPanel` does not accept a role or focus state and renders borders uniformly: `internal/tui/styles.go:56-76`.
- Dangerous KeepAlive card states are textually correct but not visually distinguished by panel role.

Why this violates CLI/TUI best practice:
Color should reinforce state, not decorate arbitrary strings. Danger/warning states should be visually grouped and consistently emphasized.

Recommended fix:
Add `PanelOptions{Role, Focused}` or dedicated helpers such as `RenderPanelWithRole`. Use warning/danger roles for KeepAlive countdown/failure panels and degraded system panels, while keeping color paired with text labels.

## E2E UX Gaps

- No automated terminal-height tests. Existing render tests check strings, but not whether the first and last critical lines fit within 80x24.
- No golden snapshots for List, Workspace, Config, empty, ambiguous, degraded notification, and active KeepAlive states.
- No test asserting header/footer remain visible during Workspace and Config interactions.
- No e2e test for focus visibility across `down`, `enter`, `space`, `r`, `k`, `s`, `x`, `esc`, and `q`.
- No visual QA for notification/degraded messages with long external strings.

## Recommended Remediation Order

1. Add height-aware rendering and 80x24 snapshot tests.
2. Make panel rendering width-aware and consistently wrap/truncate dynamic content.
3. Redesign Workspace and Config for pinned header/footer and scrollable main content.
4. Add visible focus markers on actual controls and fields.
5. Polish empty/ambiguous/degraded states into the same visual system.
6. Re-run PTY e2e before Phase 12 packaging.

## Packaging Gate

Phase 12 should remain blocked until at least P1 and P2 findings are fixed. The current UI is functionally usable but not visually reliable enough to become the installed v2 default.
