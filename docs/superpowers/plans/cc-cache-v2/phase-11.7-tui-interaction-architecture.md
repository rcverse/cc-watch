# cc-cache v2 Phase 11.7: TUI Interaction Architecture Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not use implementation subagents by default. After local verification passes, dispatch one read-only reviewer subagent for Phase 11.7.

**Goal:** Replace the current ad hoc TUI focus/render coupling with a coherent interaction architecture where cursor focus is always visible, refresh is autonomous, session information is stable and legible, verbose details expand in place, and KeepAlive appears as an additive operational card.

**Architecture:** Introduce a route-local focus registry and disclosure state so renderers and keyboard handling share one source of truth. Split visual composition into canonical information cards, operational controls, temporary operational cards, and expanded details. Preserve existing parser/JSON/CLI/KeepAlive safety semantics while refactoring TUI layout and refresh wiring.

**Tech Stack:** Go 1.23+, Bubble Tea, Lip Gloss, existing `internal/tui`, `internal/app`, `internal/session`, and `internal/refresh` packages.

---

## Phase 11.7: TUI Interaction Architecture Refactor

**Purpose:** Fix the architectural causes behind invisible cursors, stale refresh, multiline session-card overflow, muddy workspace hierarchy, and inconsistent KeepAlive presentation before Phase 12 packaging/install. This phase supersedes the Phase 11.6 patch pile where it conflicts with the design below.

**Non-goals:**

- Do not change public CLI flags, JSON schema, parser metrics, config file schema, installer behavior, or release behavior.
- Do not run a real Claude KeepAlive send.
- Do not replace `$HOME/.local/bin/cc-cache`.
- Do not implement Phase 12.
- Do not add new dependencies.

## Self-Grill Findings Applied

Reviewer frame: adversarial principal engineer plus opinionated terminal-product designer.

- `⚠️ overclaim` - Phase 11.6 said the UI had "adaptive terminal elegance", but cursor focus could still land on non-rendered actions. Revision: Phase 11.7 must make visible focus a structural invariant, not a styling convention.
- `🔴 conflict` - The UI treated footer prose as actions while Bubble Tea focus treated invisible action IDs as navigable rows. Revision: every focus target must be rendered with a marker, and footer text must be key hints only.
- `[surfaced] gap` - Refresh architecture exists in packages, but app startup does not make live refresh feel autonomous in the TUI. Revision: wire periodic safety parsing and watcher messages into Bubble Tea commands; manual update becomes fallback only.
- `[surfaced] gap` - The current color system uses one percent grammar for both TTL elapsed and hit rate. Revision: use two semantic percent systems: TTL elapsed is worse as it increases; hit rate is better as it increases.
- `[surfaced] gap` - The Python v1 gaps view had useful ranked diagnostic hierarchy; Go currently collapses gaps into a count. Revision: expanded Session Info must show ranked gaps with sort toggle.

## Visual Contract

### Palette And Percent Semantics

- Identity/title: cyan, bold.
- Muted metadata: grey, normal.
- Separators/frame: subdued grey; use frames for primary cards and temporary operational cards, not every section.
- Active/on/healthy: green.
- Warning/pending/manual action: amber.
- Expired/error/reset: coral/red.
- Selected cursor: cyan `>`.
- Disabled/off: grey, normal.
- TTL elapsed percent: low is healthy green, middle amber, high/expired coral.
- Token hit-rate percent: high is healthy green, middle amber, low coral.

### Screen Roles

- **Canonical information:** Cache Status and Session Info cards. They are not cursor-focusable.
- **Disclosure:** `v` toggles expanded Session Info details. It is not a control row.
- **Operational controls:** only these receive cursor focus on Session View.
- **Temporary operational cards:** KeepAlive countdown/manual/failure cards are additive below controls. They do not replace Cache Status, Session Info, or Controls.
- **Footer:** key hints only. It must not list action words that are not bound to direct keys or visible focused controls.

### Target Session Layout

```text
Claude Code Cache / 785b4c0d                         live · updated 13:08:24

╭─ Cache Status ───────────────────────────────────────────────────────╮
│ ACTIVE     42m19s left      [#####---------------] 29%   1-hour cache │
│ Session    785b4c0d                                                   │
╰──────────────────────────────────────────────────────────────────────╯

╭─ Session Info ───────────────────────────────────────────────────────╮
│ Session ID   785b4c0d-72f3-41a4-8b9f-aa20b7fa3ce7                     │
│                                                                      │
│ Messages                                                             │
│ First  [2h23m ago]  Orient yourself with this repo. WE have been...  │
│ Last   [42m ago]    I think it's now time to dispatch a subagent...  │
│                                                                      │
│ Tokens                                                               │
│ Writes      765,137     Reads   6,311,165     Hit rate [####] 89%    │
│                                                                      │
│ Gaps                                                                 │
│ 22 gaps · 2 resets · longest 184s · latest 64s              v details│
╰──────────────────────────────────────────────────────────────────────╯

Controls
> Reminder     off      alert at 20%, 10%
  KeepAlive    armed    countdown will start near expiry
  Auto-send    ON       sends Claude message after countdown
  Copy ID
  Back

keys: ↑↓ focus  enter act  r Reminder  k KeepAlive  a Auto-send  v details  u update  q quit
```

### Expanded Session Info

```text
╭─ Session Info · details ──────────────────────────────────────────────╮
│ Session ID   785b4c0d-72f3-41a4-8b9f-aa20b7fa3ce7                     │
│ JSONL        /Users/richardchen/.claude/projects/.../785b4c0d.jsonl   │
│ Updated      parsed 13:08:24 · file modified 13:08:21                 │
│                                                                      │
│ Messages                                                             │
│ First  [2h23m ago]  Orient yourself with this repo. WE have been...  │
│ Last   [42m ago]    I think it's now time to dispatch a subagent...  │
│                                                                      │
│ Token Stats                                                          │
│ Cache writes      765,137 tokens                                     │
│ Cache reads     6,311,165 tokens                                     │
│ Hit rate        [##################--] 89%                           │
│ Output             12,044 tokens                                     │
│                                                                      │
│ Mid-session Gaps >1min                                  ↕ longest     │
│ ! RESET     184s    12:41:09 -> 12:44:13   longest                   │
│ ! RESET     117s    12:33:52 -> 12:35:49                             │
│ - pause      64s    12:56:44 -> 12:57:48   latest                    │
│ - pause      43s    12:19:10 -> 12:19:53                             │
│                                                                      │
│ ! 2 cache reset(s) - rebuilt from scratch 2 time(s).                 │
╰──────────────────────────────────────────────────────────────────────╯

keys: v collapse  s sort gaps  u update  q quit
```

The sort label must be subtle and elegant: grey text with a small arrow, e.g. `↕ longest` / `↕ newest`, not a bright chip.

### KeepAlive Additive Card

```text
Controls
  Reminder     off      alert at 20%, 10%
> KeepAlive    armed    countdown will start near expiry
  Auto-send    ON       sends Claude message after countdown
  Copy ID
  Back

╭─ KeepAlive · watching ────────────────────────────────────────────────╮
│ Next         Countdown at 05:39:12 if session is still active          │
│ Msg Preview  "Keep-alive check. Reply yes only."                      │
│ Scope        0 / 1 sends · auto-send ON                               │
├─ Actions ─────────────────────────────────────────────────────────────┤
│ > Cancel watching                                                     │
╰──────────────────────────────────────────────────────────────────────╯
```

No `State` row. The card title carries state. Informational rows and action rows must be visually separated.

### Main List

```text
Claude Code Cache · 5 sessions · watcher ok · live 13:08

> #3  agent-a8     subagents              5-min cache     EXPIRED 11h02m ago
     [##############] 100%    hit 88%    duration 7m56s
     first  Research PKM / Obsidian vault folder-organisation best practices...
     last   Research PKM / Obsidian vault folder-organisation best practices...

  #4  9095bfdd     pkm-system-workspace   1-hour cache    EXPIRED 12h08m ago
     [##############] 100%    hit 95%    duration 3h03m
     first  <local-command-caveat>Caveat: messages were generated while running...
     last   <task-notification> <task-id>a6f6... </task-id> <tool-use-id>...

keys: ↑↓ select  enter open  r Reminder  k KeepAlive  u update  ? help  q quit
```

List cursor navigates sessions only. Root actions must not be hidden cursor targets.

## Step Checklist

- [x] **Step 11.7.1: Add focus-registry tests before refactor**
  - Modify: `internal/tui/update_test.go`, `internal/tui/render_test.go`.
  - Tests first:
    - List View down-arrow cycles only through visible sessions; cursor never disappears into `refresh/help/quit`.
    - Session View down-arrow cycles only through rendered controls/actions.
    - Help/footer key hints do not imply focusable hidden rows.
    - `FocusedAction()` always maps to a visible marker in rendered output for list, workspace, KeepAlive active states, config, empty, and ambiguous routes.
  - Run: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/tui -run 'TestFocus|TestVisibleFocus|TestListCursor|TestWorkspaceFocus'`.
  - Expected: fail before implementation because current list focus includes invisible root actions and KeepAlive focus can land on non-obvious controls.

- [x] **Step 11.7.2: Introduce visible focus items**
  - Modify: `internal/tui/model.go`, `internal/tui/list.go`, `internal/tui/workspace.go`, `internal/tui/update.go`.
  - Behavior:
    - Add a small route-local focus item concept in `internal/tui/model.go`.
    - List focus items are visible session rows only unless list is empty; empty state renders visible `Refresh`, `Help`, `Quit` rows if they are focusable.
    - Workspace focus items are visible controls/actions only.
    - KeepAlive card actions are focusable only when the corresponding action row is rendered.
    - `moveFocus` clamps/wraps against the active visible focus list.
  - Verification: Step 11.7.1 tests pass.

- [x] **Step 11.7.3: Sanitize excerpts and stabilize List View cards**
  - Modify: `internal/tui/list.go`, `internal/tui/workspace.go`, `internal/tui/render_test.go`.
  - Behavior:
    - Collapse newlines, tabs, and repeated whitespace in rendered excerpts before truncation.
    - Keep parser data unchanged; sanitize only for display.
    - List cards remain fixed-height per session in normal width tiers.
    - Footer advertises only direct keys and visible behavior: `u update`, not bare `refresh`.
  - Tests:
    - A message containing `\n<task-id>...\n<tool-use-id>...` renders as one bounded line.
    - No list row exceeds terminal width.
    - There are no stray lines outside session cards.

- [x] **Step 11.7.4: Redesign Session View canonical cards**
  - Modify: `internal/tui/workspace.go`, `internal/tui/styles.go`, `internal/tui/render_test.go`, `internal/app/cli_test.go` if expected render text changes.
  - Behavior:
    - Render `Cache Status` framed card with TTL state, TTL elapsed progress, cache tier, and short session ID only.
    - Render `Session Info` framed card below it with full session ID, messages, token summary, and gap summary.
    - Remove user-facing `Evidence` from default and expanded TUI.
    - Do not show project in Cache Status.
    - Add two percent styles:
      - TTL elapsed: high percent is bad.
      - Hit rate: high percent is good.
  - Tests:
    - Workspace includes `Cache Status`, `Session Info`, `Session ID`, `Messages`, `Tokens`, `Gaps`.
    - Workspace does not include `Evidence` in default or expanded mode.
    - Active 3% TTL renders with healthy TTL styling, expired 100% TTL renders danger styling, 95% hit rate renders success styling.

- [x] **Step 11.7.5: Add Session Info details disclosure and gap sorting**
  - Modify: `internal/tui/model.go`, `internal/tui/workspace.go`, `internal/tui/update.go`, `internal/tui/render_test.go`.
  - Behavior:
    - `v` toggles expanded Session Info details. It is not a focus item.
    - Expanded details include JSONL path, parse/modified timestamps, full token stats, and ranked mid-session gaps.
    - Gaps default to longest-duration sort.
    - `s` toggles gap sort only while details are open: longest duration <-> newest temporal order.
    - Sort label renders as muted arrow text: `↕ longest` / `↕ newest`.
    - If details exceed height, up/down scroll the expanded details only when no visible control focus movement is available or when a details scroll mode is active; do not let the cursor disappear.
  - Tests:
    - `v` expands/collapses Session Info without changing focused control.
    - `s` toggles gap order only when details are open.
    - Longest sort marks the longest gap; newest sort puts the latest gap first.
    - Expanded details remain bounded at 80x24.

- [x] **Step 11.7.6: Make KeepAlive cards additive and action-separated**
  - Modify: `internal/tui/workspace.go`, `internal/tui/update_test.go`, `internal/tui/render_test.go`.
  - Behavior:
    - KeepAlive card renders below Controls and never replaces Cache Status, Session Info, or Controls.
    - Card title carries state: `KeepAlive · watching`, `KeepAlive · countdown`, `KeepAlive · manual prompt`, `KeepAlive · failed`, etc.
    - Card rows are only `Next`, `Msg Preview`, `Scope`, plus optional failure reason/fallback when needed.
    - Actions are separated under an `Actions` divider and only those actions are focusable.
    - Remove `State` row from KeepAlive cards.
  - Tests:
    - Triggering KeepAlive preserves Cache Status and Session Info line presence.
    - Focus marker appears in the card action row when card action is focused.
    - Pressing down cycles through visible controls/actions without invisible states.
    - No card contains `State    `.

- [x] **Step 11.7.7: Wire autonomous refresh and manual update fallback**
  - Modify: `internal/app/app.go`, `internal/tui/messages.go`, `internal/tui/update.go`, `internal/tui/list.go`, `internal/tui/workspace.go`, `internal/app/cli_test.go`, `internal/tui/update_test.go`.
  - Behavior:
    - TUI starts an autonomous refresh command loop: periodic safety parse and watcher-message driven refresh where available.
    - Display tick remains responsible for time display and KeepAlive checks; refresh tick reparses session data.
    - Manual update key is `u`, not `x` and not `R`.
    - Rationale: `x` is reserved for cancel/stop actions; `R` is conventional but heavier than needed because refresh should be autonomous.
    - Header shows live refresh status quietly: `live · updated HH:MM:SS`, degraded if watcher/safety refresh fails.
  - Tests:
    - `u` triggers `ManualRefreshMsg` in list and workspace.
    - Periodic refresh tick reparses selected workspace session and list sessions.
    - Watcher event refresh still works.
    - Display ticks alone are not treated as data refresh unless refresh tick fires.

- [x] **Step 11.7.8: Refine visual system across List, Workspace, Empty, Ambiguous, Config**
  - Modify: `internal/tui/styles.go`, `internal/tui/list.go`, `internal/tui/workspace.go`, `internal/tui/config_editor.go`, `internal/tui/help.go`, `internal/tui/render_test.go`.
  - Behavior:
    - Use consistent frame, separator, title, muted metadata, selected cursor, and percent styles.
    - Config retains compact form but uses the same selected cursor and on/off vocabulary.
    - Empty and ambiguous states render visible focus rows if they have focusable actions.
    - Key hint grammar is consistent across screens.
  - Tests:
    - Render snapshots/assertions for all routes include visible selected marker.
    - No route contains the stale action prose `Actions: copy ID · manual refresh · help · back · quit`.
    - No route uses `KA`.

- [x] **Step 11.7.9: Verification, smoke, reviewer, and ledger update**
  - Run: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/tui ./internal/app`.
  - Run: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test -count=1 ./...`.
  - Run: `git diff --check`.
  - Build: `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go build -o /private/tmp/cc-cache-phase117-bin/cc-cache ./cmd/cc-cache`.
  - PTY smoke:
    - List View with fixture HOME: verify session-only cursor, no multiline leaks, `u` update hint.
    - Workspace default: verify Cache Status and Session Info cards, controls-only cursor, `v` details toggle.
    - Workspace KeepAlive trigger with fixture `auto_send: false`: verify additive KeepAlive card, separated actions, no real send.
    - Config: verify visible cursor and consistent color grammar.
    - Empty HOME: verify visible refresh/help/quit rows.
  - Dispatch one read-only reviewer subagent for Phase 11.7 after local verification passes.
  - Integrate valid review findings, rerun focused and full verification, then update:
    - `docs/superpowers/plans/cc-cache-v2/phase-11.7-tui-interaction-architecture.md`
    - `docs/superpowers/progress/cc-cache-v2-progress.md`
  - Stop before Phase 12.

## Post-Review Stabilization

- [x] **Phase 11.7 screenshot/review stabilization**
  - Used a fresh-context read-only reviewer for the user-reported residual focus/config/notification issues.
  - Integrated valid findings:
    - Expired sessions now share one KeepAlive eligibility guard across render, toggle, tick, and manual-send paths.
    - Refresh reorders now keep visible list cursor and selected session synchronized by session ID.
    - KeepAlive operational cards are informational; Workspace Controls own focusable send/dismiss/stop/cancel rows.
    - Action feedback uses transient in-TUI notices instead of sticky historical banners.
    - Config editing preloads current values, first typing replaces the prefilled value, and empty KeepAlive messages are rejected.
    - Details gap sort label sits beside the heading; list/workspace labels use domain semantic colors.
    - Dead evidence/card-action render helpers from prior layouts were removed.
  - Verification rerun:
    - `GOCACHE=/private/tmp/cc-cache-go-build go test ./internal/tui`
    - `GOCACHE=/private/tmp/cc-cache-go-build go test ./internal/app ./internal/notify`
    - `GOCACHE=/private/tmp/cc-cache-go-build go test -count=1 ./...`
    - `git diff --check`
    - `GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go build -o /private/tmp/cc-cache-phase117-refactor/cc-cache ./cmd/cc-cache`

## Packaging Gate

Phase 12 remains blocked until Phase 11.7 passes verification. The Go TUI must not be packaged or installed as the default command while cursor focus can disappear, refresh feels stale, KeepAlive changes the canonical session layout, or details/gaps remain hidden in plain text.
