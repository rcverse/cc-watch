# cc-cache v2 Phase 11.6: Adaptive Terminal Elegance

> Inserted after Phase 11.5 and before Phase 12 because the Phase 11.5 UI audit found the Go TUI still visually and ergonomically below the Python v1 baseline.

## Phase 11.6: Adaptive Terminal Elegance

**Purpose:** Replace the boxed Phase 11.5 visual grammar with a Python-inspired but Bubbletea-native TUI: session-card list layout, thoughtful semantic color, clear on/off state separation, explicit focus affordance, bounded content, and viewport-aware rendering. Do not change CLI flags, JSON output, parser behavior, KeepAlive safety semantics, installer behavior, or release behavior.

**Design principles:**

- Base the list view on Python v1's calm session-card rhythm: identity row, status row, evidence rows, optional warning/action row.
- Do not copy Python v1 mechanically. Use Go/Bubbletea flexibility for terminal-width tiers, pinned header/footer, focused controls, and state-specific operational chips.
- Use `KeepAlive`, never `KA`, in visible copy.
- Treat Reminder and KeepAlive as session-bound operational state, not global buttons.
- Make on/off states visually distinct by text, weight, and color: `ON` is bold green; `off` is muted normal; risky enabled Auto-send is bold amber/coral with explicit copy.
- Use terminal chips sparingly. Prefer words plus spacing over bracket-heavy pseudo-web controls.
- Favor whitespace, indentation, subtle rules, and dim metadata over large bordered panels.
- Use semantic color only: identity cyan, active/on green, warning/pending amber, expired/error coral, muted/off gray, selected marker cyan.
- Do not display impossible elapsed percentages. Expired progress is capped and paired with truthful elapsed-overrun copy.

**Target surfaces:**

- List View: terminal session cards, no healthy System panel, no enclosing Sessions panel.
- Empty and ambiguous states: compact bounded onboarding/choice surfaces with width-safe path and ID text.
- Workspace: pinned header/footer with a concise inspector body and focused operational controls.
- Config Editor: compact focused form with clear on/off separation and sticky validation/action copy.

- [x] **Step 11.6.1: Add viewport and visual contract tests**
  - Modify: `internal/tui/model.go`, `internal/tui/update.go`, `internal/tui/render_test.go`.
  - Behavior: model stores both terminal width and height from `tea.WindowSizeMsg`.
  - Tests first:
    - List View at `80x24` has no line wider than 80 columns.
    - List View at `80x24` uses session cards, not `System` or `Sessions` bordered panels.
    - Expired sessions do not render raw percentages above 100.
    - Visible copy uses `KeepAlive`, not `KA`.
  - Run: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/tui -run 'TestAdaptive|TestListView|TestExpired|TestKeepAliveCopy'`.
  - Expected red before implementation, pass after implementation.

- [x] **Step 11.6.2: Replace List View with adaptive session cards**
  - Modify: `internal/tui/styles.go`, `internal/tui/list.go`, `internal/tui/render_test.go`.
  - Behavior: render Python-inspired terminal session cards with adaptive density:
    - narrow: identity, status, one evidence line, operational chips.
    - normal: identity, status, first and last evidence lines.
    - wide: add duration/warnings/KeepAlive state when space allows.
  - Behavior: healthy watcher/safety state is quiet header metadata; degraded notification/watcher/Claude state renders as a compact warning banner.
  - Behavior: Reminder/KeepAlive chips are text-first and bounded: `remind ON`, `remind off`, `KeepAlive armed`, `KeepAlive off`, `KeepAlive failed`.
  - Verification: targeted TUI tests pass and no rendered line exceeds model width in tested tiers.

- [x] **Step 11.6.3: Refine Workspace controls and operational states**
  - Modify: `internal/tui/workspace.go`, `internal/tui/render_test.go`.
  - Behavior: remove duplicated section headings and large healthy System panel.
  - Behavior: render focused controls with direct markers and distinct on/off treatment:
    - `Reminder     ON     alert at ...`
    - `KeepAlive    armed  trigger ...`
    - `Auto-send    off    manual prompt only`
  - Behavior: countdown/manual/failure states remain safety-explicit and use `KeepAlive` in full.
  - Verification: workspace tests assert visible focus marker on controls, on/off styling text, and no `KA` copy.

- [x] **Step 11.6.4: Refine Config, empty, and ambiguous states**
  - Modify: `internal/tui/config_editor.go`, `internal/tui/list.go`, `internal/tui/render_test.go`.
  - Behavior: config form is compact, focusable, and shows on/off distinctions without stacked oversized panels.
  - Behavior: empty state gives calm onboarding copy and width-safe projects path.
  - Behavior: ambiguous state uses a compact choice list with focused session-card rows.
  - Verification: tests assert bounded lines, visible focused field/action, and direct navigation copy.

- [x] **Step 11.6.5: Full verification and PTY smoke**
  - Run: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/tui ./internal/app`.
  - Run: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test -count=1 ./...`.
  - Run: `git diff --check`.
  - Build: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go build -o /private/tmp/cc-cache-phase116-bin/cc-cache ./cmd/cc-cache`.
  - PTY smoke: fixture List View, Workspace, Config Editor, and empty HOME. Exit with `q`.
  - Safety: no real Claude KeepAlive send, no `$HOME/.local/bin/cc-cache` replacement.

## Packaging Gate

Phase 12 remains blocked until Phase 11.6 passes local verification and PTY smoke. The installed v2 default must not ship while list/session/workspace/config views still overflow normal terminals or visually trail the Python v1 baseline.
