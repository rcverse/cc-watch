# cc-watch UsageAware Future Note

**Status:** Archived future investigation, not part of cc-watch v2.
**Date:** 2026-06-02

This note preserves the UsageAware investigation that was removed from the active v2 design spec.
It is not a v2 implementation requirement and must not create UI placeholders in the v2 TUI.

## Decision

Usage-aware plan-limit monitoring is deferred until a Claude Code statusline integration is designed
and tested separately.

## Investigation Notes

- `claude -p "/usage"` is not reliable as a real-time source for plan limits. In non-interactive
  testing it returned command/session usage, not plan bars.
- Post-limit JSONL errors such as rate-limit messages are too late for proactive warnings.
- Claude Code statusline JSON appears to be the promising future source because it may expose
  `rate_limits.five_hour.used_percentage`, `rate_limits.five_hour.resets_at`,
  `rate_limits.seven_day.used_percentage`, and `rate_limits.seven_day.resets_at`.

## Future Requirements

If UsageAware is revisited later, the provider must:

- wrap or cooperate with the user's existing Claude Code `statusLine` command;
- never overwrite an existing statusline;
- write a minimal local cc-watch usage snapshot;
- degrade cleanly when rate-limit fields are absent;
- stop KeepAlive automation when usage is near or past configured thresholds.

## v2 Boundary

The v2 TUI should not show a UsageAware or usage-limit section. KeepAlive safety in v2 comes from:

- visible countdown before send;
- per-session monitoring toggle;
- bounded scope;
- single-flight subprocess execution;
- confirmation from the target session JSONL;
- stopping auto-send on Claude limit or subprocess errors.
