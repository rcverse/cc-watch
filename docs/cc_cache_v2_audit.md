# cc-cache v2 — UX/UI Audit, Second Revision

**Date:** 2026-06-03
**Change from first revision:** Accepts companion dashboard mental model (Session Workspace stays).
Revises the controls model: two toggle lines + an in-workspace KeepAlive card. No overlay.

---

## What Changed From First Revision

My recommendation to use an overlay was wrong. Here's why the workspace mental model is correct:

> You open a session because you want to understand it *and* decide what to do about it. Having evidence and controls on the same screen means you can see "5 minutes 44 seconds remaining" and immediately toggle KeepAlive — without losing context and without navigating to a second surface. The companion dashboard colocation is the right UX.

The spec's actual problem was never the workspace concept. It was the **controls section being too complex**: per-chip threshold focus, inline editors per chip, a 15-state stateful footer, and three navigation axes. Fix those, keep the workspace.

What follows is a clean redesign of the controls section, with the workspace mental model intact.

---

## The Proposed Simplified Controls Model

### Reminder: One Line

```
  [ ] Reminder   alert at 20%, 10%    Sends no Claude message.
```

That's the entire Reminder control. One line. `r` toggles it. When on:

```
  [x] Reminder   alert at 20%, 10%    ✓ On  next: 20%
```

No per-session threshold editing in Session Workspace. Thresholds live in global config. The only action is on/off. This eliminates:
- Per-chip focus and navigation
- Inline threshold editors
- Last-threshold guardrail error
- `left/right` navigation within a row
- ~6 footer states

### KeepAlive: One Toggle Line + One Expandable Card

When **off**:
```
  [ ] KeepAlive  auto-send on · trigger 5m · scope 1 send    Off. No Claude message.
```

When **on** (toggled with `k`), the toggle line collapses to a badge and a **KeepAlive card** appears below it — within the Controls section, not above the evidence:

```
  [x] KeepAlive ─────────────────────────────────────────── watching ──
    State    Watching cache expiry
    Next     Countdown at 14:33:02
    Message  "Keep-alive check. Reply \"yes\" only."
    Scope    0 / 1 sends
  ─────────────────────────────────────────────────────────────────────
```

The card grows/changes content as the state changes (watching → countdown → sending → success/failure). Only the card changes; the rest of the workspace is stable.

---

## Session Workspace — Complete Layout Walkthrough

### State 1: KeepAlive Off (normal view)

```
╭─────────────────────────────────────────────────────────────────────╮
│  cc-cache / workspace-api / d4b247b7              14:32:18  live    │
╰─────────────────────────────────────────────────────────────────────╯

  Status
    1h cache  ·  active  ·  05m 44s left
    TTL  ████░░░░░░░░░░░░░░░░░░░░  18%    Hit  ████████████████████░  96%
    Last msg  14:26:34  (7m 09s ago)  ·  Session  11:13 → 14:22  (3h 08m)

  Messages
    First  "can you check whether this session is cached for 5m or 1h?"
    Last   "write the implementation plan and keep the cache alive"

  Token Stats
    Cache writes  1,218,696    Cache reads  29,673,296    Output  402,353
    Hit rate  96%   ·   TTL evidence: ephemeral_1h_input_tokens

  Gaps  >1 min  ·  0 resets
    49m 26s  12:38 → 13:27  ← longest, below 1h TTL ✓
    47m 55s  13:27 → 14:15

  ── Controls ──────────────────────────────────────────────────────────
  [ ] Reminder   alert at 20%, 10%   Sends no Claude message.
  [ ] KeepAlive  auto-send on · trigger 5m · scope 1 send   Off. No message.

─────────────────────────────────────────────────────────────────────
r remind   k keepalive   c copy id   b/esc back   ? help   q quit
```

**Line count:** 2 (header) + 4 (status) + 3 (messages) + 3 (tokens) + 3 (gaps) + 3 (controls header + 2 toggles) + 2 (footer divider + footer) = **20 lines**. Fits in 24.

---

### State 2: Reminder On, KeepAlive Off

```
  ── Controls ──────────────────────────────────────────────────────────
  [x] Reminder   alert at 20%, 10%   ✓ On  ·  next: 20%
  [ ] KeepAlive  auto-send on · trigger 5m · scope 1 send   Off. No message.
```

No change to anything else. Single-line toggle. Footer unchanged.

---

### State 3: KeepAlive On — Watching

```
  ── Controls ──────────────────────────────────────────────────────────
  [ ] Reminder   alert at 20%, 10%   Sends no Claude message.
  [x] KeepAlive ─────────────────────────────────────── watching ──────
    State    Watching cache expiry
    Next     Countdown at 14:33:02 (if session still active)
    Message  "Keep-alive check. Reply \"yes\" only."
    Scope    0 / 1 sends  ·  Auto-send on
  ─────────────────────────────────────────────────────────────────────

─────────────────────────────────────────────────────────────────────
r remind   k keepalive   c copy id   b/esc back   ? help   q quit
```

**Line count:** +5 lines for the card = 25 lines total. One line over on a strict 24-line terminal.

**Fix:** Compress Token Stats to one line when KeepAlive is on:
```
  Token Stats   writes 1.2M  ·  reads 29.7M  ·  output 402K  ·  hit 96%
```
This saves 2 lines. Total: 23 lines. Comfortable.

The compression rule: when the KeepAlive card is visible, Token Stats collapses to a summary line. Gaps collapses to a summary line too if needed:
```
  Gaps   5 gaps > 1 min  ·  longest 49m 26s  ·  0 cache resets
```

---

### State 4: KeepAlive — Countdown

The card changes. Everything else is stable. The footer shifts to surface `s` and `x`.

```
  [x] KeepAlive ──────────────────────────────────────── countdown ────
    Sending in  24s  ████████████████████░░░░░  72%
    Message  "Keep-alive check. Reply \"yes\" only."
    Target   d4b247b7  ·  workspace-api
    Scope    0 / 1 sends  ·  will count as 1 if sent

─────────────────────────────────────────────────────────────────────
s send now   x cancel instance   b back   ? help   q quit
```

**Key decisions:**
- The countdown bar is inside the card. It is not pinned above the evidence.
- The card's left border uses `danger` color (amber or red) to make it visually salient.
- Footer promotes `s` and `x` during countdown — the only state-sensitive footer.
- The rest of the workspace (status, messages, token stats) is still visible above, giving context for *why* the countdown is happening.

---

### State 5: KeepAlive — Sending and Confirming

```
  [x] KeepAlive ────────────────────────────────────────── sending ────
    ⟳ Running: claude -r d4b247b7 -p "Keep-alive check. Reply yes only."
    Waiting for Claude CLI to exit…
```

Then:

```
  [x] KeepAlive ──────────────────────────────────────── confirming ───
    ⟳ Confirming…  watching d4b247b7.jsonl for new entry after 14:32:58
    x stop waiting
```

Footer during confirming:
```
x stop waiting   b back   ? help   q quit
```

---

### State 6: KeepAlive — Success

```
  [x] KeepAlive ──────────────────────────────────────────── done ─────
    ✓ Cache refreshed at 14:33:19
    New window: active · 58m 41s left
    Scope: 1 / 1 sends used.  Monitoring complete.
    k turn off KeepAlive
```

The Status section above will have already refreshed (via data refresh from the new JSONL entry) to show the new remaining time. The user sees both the new status and the confirmation in one view.

Footer returns to normal:
```
r remind   k keepalive   c copy id   b/esc back   ? help   q quit
```

---

### State 7: KeepAlive — Failure

```
  [x] KeepAlive ──────────────────────────────────────────── failed ───
    ✗ Failed: claude not found.  Auto-send stopped for this session.

    Manual fallback:
      claude -r d4b247b7-be64-417c-a81c-ba686b464cf5 \
        -p "Keep-alive check. Reply \"yes\" only."

    Fix PATH or send manually, then press r to refresh.
    k turn off KeepAlive
```

---

## List View — Simplified

No changes to List View architecture. Minor wording refinements:

```
╭─────────────────────────────────────────────────────────────────────╮
│  cc-cache  ·  5 sessions  ·  14:32:18  ·  live                     │
╰─────────────────────────────────────────────────────────────────────╯

▶  #1  d4b247b7  workspace-api   1h  ●  05m 44s left
       TTL ████░░░░░░░░░░░░░░░░░░░░  18%   Hit ████████████████████░  96%
       "write the implementation plan and keep the cache alive…"
       KeepAlive: countdown 24s  ·  0/1 sends

   #2  a3f12c90  sidechorus       5m  ○  expired 1h 51m ago
       "ok looks good, commit it"

   #3  b8c301e1  billing-docs      ?  ○  no timestamp  ⚠ 2 parse warnings
       "review the docs"

─────────────────────────────────────────────────────────────────────
2 active  1 unknown  2 expired      ↑↓ move   enter open   r remind   k keepalive   q quit
```

**Decisions:**
- Non-selected rows: 2 lines (scan + one excerpt). No bars.
- Selected row: 3–4 lines (scan + bars + excerpt + KA status if relevant).
- Bars appear only on the selected row.
- `r` and `k` in List View toggle Reminder/KeepAlive for the selected session directly, without entering the workspace. The status line of the selected row confirms the toggle.
- Footer is one static line.

---

## Navigation Model (Final)

### List View

| Key | Action |
|---|---|
| `↑ / ↓` or `j / k` | Move selection |
| `1`–`9` | Jump to session by number |
| `enter` | Open Session Workspace |
| `r` | Toggle Reminder for selected session |
| `k` | Toggle KeepAlive for selected session |
| `q` / `esc` | Quit |
| `?` | Help |

### Session Workspace

| Key | Action |
|---|---|
| `↑ / ↓` | Scroll evidence (if longer than screen) |
| `r` | Toggle Reminder |
| `k` | Toggle KeepAlive |
| `s` | Send now (only during countdown / manual-ready) |
| `x` | Cancel / dismiss current KA instance |
| `c` | Copy / show full session ID |
| `b` / `esc` | Back to list |
| `q` | Quit |
| `?` | Help |

**No `tab`. No `left/right` within rows. No per-element focus in the controls section.**

The controls section (`r` Reminder, `k` KeepAlive) uses direct key bindings, not cursor navigation. The cursor in Session Workspace is only for scrolling evidence — it never lands on a "control element" that requires `enter` to activate. This eliminates the focus order complexity entirely.

The tradeoff: the `r` and `k` keys must be clearly advertised in the footer and help overlay, since they're not reached via cursor navigation. This is fine — these are the tool's two primary actions.

---

## Footer Design (Final)

### Normal states (List View, Workspace with KA off or watching)
```
r remind   k keepalive   c copy id   b/esc back   ? help   q quit
```
One line, static. ~60 characters, fits at 80 columns.

### Countdown state
```
s send now   x cancel instance   b back   ? help   q quit
```

### Confirming state
```
x stop waiting   b back   ? help   q quit
```

### Config Editor
```
↑↓ move   enter edit   space toggle   s save   d reset (confirm)   esc cancel
```

That's four footer states total. The spec currently implies ~15.

---

## Config Editor (Simplified)

`cc-cache config` — separate command, flat form, no sub-navigation.

```
╭─────────────────────────────────────────────────────────────────────╮
│  cc-cache config                                                     │
╰─────────────────────────────────────────────────────────────────────╯

  Reminder
    Alert at     [ 20, 10 ]%     (comma-separated, 1–99, descending)

  Keep-alive
    Trigger      [  5 ] minutes before expiry
    Countdown    [ 30 ] seconds
    Auto-send    [✓] enabled
    Max sends    [  1 ]
    Message      [ Keep-alive check. Reply "yes" only. ]

  What will happen
    1h cache:  countdown starts at 5m left, auto-send after 30s unless canceled
    5m cache:  countdown starts at 1m left, auto-send after 15s unless canceled
    Scope: stop after 1 attempted send

  Validation: OK

  [ Save ]   [ Reset to defaults ]   [ Cancel ]

─────────────────────────────────────────────────────────────────────
↑↓ move   enter edit   space toggle   s save   d reset (confirm)   esc cancel
```

**Threshold field:** plain text, comma-separated. Enter to open inline editor, type new value, enter to save. Validation: "Use whole numbers 1–99 in descending order (e.g. 20, 10)."

**Scope modes:** `max_sends` only in v2. Cut `until_timestamp` and `for_duration` — they're speculative complexity. The config field is simply "Max sends: [1]".

---

## Visual Design Language (Concrete Spec Addition)

Add this to §10.0 of the spec as a normative table, not prose.

### Lipgloss Semantic Color Palette

| Role | Usage | lipgloss `Color` |
|---|---|---|
| `text` | Default readable content | terminal default |
| `muted` | Section labels, secondary info, timestamps | `"240"` |
| `selected` | Focused list row | bold + `"81"` (bright cyan) |
| `active` | Cache active status, success state | `"114"` (soft green) |
| `warning` | Expiring soon (< 20% left), KA armed, amber states | `"221"` (amber) |
| `danger` | Expired, countdown, failure, critical | `"203"` (soft red) |
| `info` | Reminder state, hit rate bar (high), info badges | `"117"` (sky blue) |
| `disabled` | Unavailable actions, unknown state, dim | `"238"` |
| `border.normal` | Panel borders, internal dividers | `"237"` |
| `border.focused` | Selected list row border, focused panel | `"243"` |
| `border.danger` | KA countdown / failure card left border | `"203"` |

### Border and Typography Rules

- **Outer panels**: `lipgloss.RoundedBorder()` → produces `╭─╮ │ ╰─╯`, matching v1 Python aesthetic.
- **Section headers inside panels**: muted label + `─` separator. E.g. `── Token Stats ──────`. Not nested boxes.
- **Status badges**: colored text only, no background. E.g. `● active` in `active` color, `○ expired` in `danger` color.
- **KA card border accent**: when KeepAlive is in countdown, sending, or failure, the card's left border (or the entire border) uses `border.danger` (amber for countdown, red for failure).

### Progress Bar

Filled block `█`, empty `░`, 24 chars wide. Color by fill percentage:
- `< 50%`: `active` (soft green)
- `50–79%`: `warning` (amber)
- `≥ 80%`: `danger` (red)

Hit rate bar uses **inverted** semantics (high fill = good):
- `≥ 80%`: `active` (green)
- `50–79%`: `warning` (amber)
- `< 50%`: `danger` (red)

---

## Full Issue Register (Final)

### Technical (unchanged)

| # | Issue | Severity |
|---|---|---|
| T1 | Project name decode inherits v1 last-segment truncation bug | Medium |
| T2 | Scanner buffer size unspecified (recommend 1 MB) | Low |
| T3 | fsnotify race on new subdirectory not documented as known gap | Low |
| T4 | `cancelled_instance` re-arm condition ambiguous | Medium |
| T5 | Confirmation timeout value unspecified (recommend 120s) | Medium |
| T6 | No `x` escape from `sending` state; subprocess needs hard timeout (60s) | Medium |

### UX/UI (revised)

| # | Issue | Severity | Resolution |
|---|---|---|---|
| U1 | Session Workspace controls section: per-chip threshold focus | **High** | Remove. Reminder = one toggle line. Thresholds in global config only. |
| U2 | Per-session threshold state + `state.json` | **High** | Remove `state.json`. All session Reminder/KA state is in-memory per run. |
| U3 | 3 navigation axes (`↑↓`, `←→`, `tab`) for controls | **High** | Controls use direct key bindings (`r`, `k`). Cursor only scrolls evidence. No `tab`, no `←→`. |
| U4 | 15+ footer states based on focused element | **High** | 4 footer states: normal, countdown, confirming, config editing. |
| U5 | 5-line list rows overflow 24-line terminals | Medium | Non-selected = 2 lines. Selected = 3–4 lines with bars. |
| U6 | Token Stats and Gaps may overflow when KA card is visible | Medium | Compress to summary lines when KA card is shown. Saves 2–3 lines. |
| U7 | ASCII mockups will anchor Go implementation to flat look | Medium | Add lipgloss palette table + `RoundedBorder()` to §10.0 as normative spec. |
| U8 | `esc` ambiguity on `--id` entry point with no prior list | Low | If entered via `--id`, `esc` quits. From TUI navigation, `esc` returns to list. |
| U9 | `r` key in Config Editor has no defined behavior | Low | No-op; state explicitly in spec. |
| U10 | `until_timestamp` and `for_duration` scope modes are speculative | Low | Cut from v2. `max_sends` only. |
| U11 | `cache ?` vs `TTL ?` label inconsistency for unknown TTL | Low | Standardize to `cache ?` everywhere. |

### Refinements

| # | Refinement | Priority |
|---|---|---|
| R1 | Specify project name decode: strip leading `-`, reconstruct last 2 path segments | Medium |
| R2 | Add `reminder_thresholds: [20, 10]` to global config (only config knob for thresholds) | Medium |
| R3 | Specify 1 MB minimum scanner buffer and error surfacing behavior | Low |
| R4 | Specify 120s confirmation timeout; add as `confirm_timeout_s` config key | Medium |
| R5 | Add `reminder_default_thresholds` to JSON output alongside session data | Low |
| R6 | Expired session: collapse controls to one line "Cache expired. Reminder and KeepAlive inactive." | Low |
| R7 | JSON output schema: define v1 contract (schema_version field, stability promise) | Low |

---

## Summary: What to Change in the Spec

**Remove:**
1. Per-chip threshold focus, inline threshold editors, and last-threshold guardrail from Session Workspace.
2. `state.json` and per-session state persistence. State is in-memory per run.
3. Scope modes `until_timestamp` and `for_duration`. Keep only `max_sends`.
4. "Session Workspace focus order" list (15 elements). Not needed with direct key bindings.
5. "Contextual footer requirements" with per-focus-element footer states. Replace with 4-state footer.
6. `left/right` navigation within rows and `tab` between groups.

**Add:**
1. Lipgloss semantic color palette table (normative, not prose).
2. `RoundedBorder()` as required border style.
3. `reminder_thresholds` to global config schema. Remove from per-session state.
4. Reminder control spec: one toggle line, no threshold editing in workspace.
5. KeepAlive card spec: compact card within Controls section, not pinned above evidence.
6. Layout compression rule: Token Stats and Gaps collapse to summary lines when KA card is shown.
7. 4-state footer spec.
8. Scanner buffer minimum (1 MB) and error surfacing.
9. Confirmation timeout value (120s).
10. Project name decode algorithm.

**Keep (unchanged from current spec):**
- KeepAlive state machine (§9.4) — correct logic, only display changes.
- TTL-aware trigger calculation (§9.3) — correct.
- fsnotify + safety refresh + display tick architecture (§6) — correct.
- Parser equivalence contract (§5) minus the project name issue.
- Reminder vs KeepAlive semantic separation — correct and important.
- `--json` output as stable scripting interface.
- CLI contract (§4) — correct.
