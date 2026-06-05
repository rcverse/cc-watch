# BOOTSTRAP.md — Resume prompt for cc-cache v2

Copy the block below into a new Claude Code / AI session to resume work on
cc-cache v2 immediately with full context.

---

```
You are picking up development of **cc-cache**, a terminal TUI tool for
inspecting Claude Code session cache health. The project lives at
~/Dev/cc-cache/. Read PRD.md in full before doing anything.

## Context

cc-cache reads Claude Code session JSONL files from ~/.claude/projects/ to
surface cache TTL tier (5-min vs 1-hour), cache expiry status, hit rate, and
mid-session gap analysis — all without any API calls.

## Current state

v1 is complete and working. The script is cc_cache.py (~350 lines), installed
as a symlink at ~/.local/bin/cc-cache. It supports:
  - cc-cache           → list 5 most recent sessions (ANSI output)
  - cc-cache --n N     → list N sessions
  - cc-cache --id <partial-uuid>  → detail view
  - cc-cache --json    → NOT YET implemented (v2 target)
  - cc-cache --watch   → NOT YET implemented (v2 target)
  - cc-cache --remind  → NOT YET implemented (v2 target)
  - cc-cache config    → NOT YET implemented (v2 target)

Partial UUID matching works. All data fields (TTL tier, elapsed time,
remaining, hit rate, session duration, first/last msg, gap analysis) are
implemented and tested against real sessions.

## v2 target

A full rewrite of cc_cache.py into a rich-based interactive TUI. Everything
is specced in PRD.md. Do NOT start implementing until you have read PRD.md
completely.

## Tech decisions (already made, do not re-debate)

- rich (installed via uv) for rendering — user already has it
- termios + tty (stdlib) for raw keyboard input
- osascript for macOS notifications
- claude -r <session-id> -p "<msg>" for keep-alive auto-send (confirmed working)
- Config at ~/.config/cc-cache/config.json (stdlib json)
- Single file: cc_cache.py (~600-700 lines target)

## Implementation order

Phase 1 (core TUI — do this first, get it working end-to-end):
1. rich rendering: Panel header, session cards, detail panel
2. termios raw keyboard: ↑↓ jk 1-9 enter b w r q/esc
3. List mode with ▶ cursor selection
4. Detail mode with b=back navigation
5. Hit rate bar: inverted colour (green=high, red=low)
6. Summary footer: "X active · Y expired"

Phase 2 (power features — only after Phase 1 is confirmed working):
7. --watch loop (default 10s, configurable via --interval)
8. --remind: osascript notifications at config thresholds
9. Keep-alive: countdown banner + claude subprocess + JSONL poll to confirm
10. --json output (non-interactive, no rich)
11. cc-cache config interactive editor

## Key gotcha to know upfront

ephemeral_1h_input_tokens is nested at:
  .message.usage.cache_creation.ephemeral_1h_input_tokens
NOT at the flat .usage level. The parser must walk nested dicts. See the
existing parse_session() in cc_cache.py — it already handles this correctly.
Do not break it.

## What to do first

1. Read PRD.md fully (~500 lines)
2. Read the existing cc_cache.py to understand the current data model and
   output structure
3. Confirm your implementation plan for Phase 1 before writing any code
4. Ask if anything in the PRD is ambiguous before proceeding
```
