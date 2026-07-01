# ADR: KeepAlive Subprocess Safety

Date: 2026-06-03

## Status

Accepted

## Context

KeepAlive is the only v2 feature that can invoke the Claude CLI. It must be bounded, visible, cancellable, session-specific, and evidence-confirmed. Reminder behavior must never send Claude messages.

The implementation and verification process must not run a real Claude KeepAlive send.

## Decision

KeepAlive execution is scoped to one selected session and one configured message. The real subprocess runner, when enabled by runtime state, invokes the Claude CLI by resolving `claude` from `PATH` at send time and passing the selected session ID explicitly.

The command shape is:

```text
claude -r <session-id> -p <configured-message>
```

The exact argv form must be constructed without shell interpolation.

While KeepAlive is armed, preflight checks verify that `claude` is available on `PATH`. If unavailable, the session enters a visible unavailable or failed state before any countdown can send.

Each send attempt has:

- a per-instance token so stale async messages can be ignored after cancellation or a newer instance;
- a timeout of 30 seconds for the subprocess;
- a 20 second confirmation window after subprocess completion;
- confirmation tied to the target session JSONL and a new entry timestamped after the send initiation time.

Attempted sends count once at send initiation, not at confirmation. Scope counting therefore prevents repeated subprocess attempts even if confirmation fails.

Cancellation prevents future countdown/send actions for that instance and causes stale timer, subprocess, and confirmation results to be ignored. Cancellation does not rewrite JSONL and does not claim a send was undone if the subprocess already started.

Claude limit detection uses the subprocess exit status plus stderr/stdout text. Any non-zero result whose combined output contains `limit`, `rate limit`, `usage limit`, `quota`, or `too many requests`, case-insensitively, is classified as `claude_limit`. Other non-zero results are classified as `subprocess_failed`. A missing executable is classified as `claude_unavailable`.

Claude limit, unavailable executable, timeout, confirmation timeout, or subprocess failures stop auto-send for that session until the user explicitly re-enables or starts a new eligible workflow. The UI must show the failure reason and this manual fallback wording:

```text
Send manually if you still want to keep this session warm:
claude -r <session-id> -p <configured-message>
Then refresh cc-watch to confirm the session JSONL changed.
```

No production code or test may perform a real Claude KeepAlive send. Tests use fake runners and fixture HOME/session files.

## Consequences

KeepAlive can preserve cache windows without becoming hidden or unbounded automation. Users can see whether a Claude message may be sent, was sent, failed, or is waiting for evidence.

The state machine must treat subprocess result and JSONL confirmation as separate events.
