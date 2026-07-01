# cc-watch v2 Phase 2: Parser Parity

> Sliced mechanically from the original consolidated plan. This file is the active checklist for Phase 2.

## Phase 2: Parser Parity Before TUI

**Purpose:** Prove Go parser behavior before any visual build-out.

- [x] **Step 2.1: Create parser fixtures**
  - Create: `internal/session/testdata/*.jsonl`.
  - Create: `internal/session/testdata/smoke-home/.claude/projects/-tmp-cc-watch/11111111-1111-1111-1111-111111111111.jsonl`.
  - Create: `internal/session/testdata/smoke-home/.config/cc-watch/config.json`.
  - Required fixture coverage: long JSONL line, malformed line, empty line, missing timestamp, malformed timestamp, timestamp-less file, top-level `usage`, nested `message.usage`, one-level nested cache creation fields, 5-minute TTL, 1-hour TTL, unknown TTL, gaps over 60 seconds, reset over effective TTL, first/last user text, list content blocks, short/malformed filename stems, deterministic smoke session with partial ID `11111111`.
  - Smoke config: KeepAlive `auto_send` is `false` so manual TUI smoke cannot send a Claude message unless a tester deliberately changes the fixture config.
  - Verification: fixtures are small, deterministic, and contain no real user session content.

- [x] **Step 2.2: Write failing parser model tests**
  - Create: `internal/session/parser_test.go`.
  - Assertions: TTL tier precedence, cache write/read/output totals, hit rate, parser warnings, gap count, reset count, first/last excerpts, long-line handling, malformed-line tolerance, timestamp tolerance, unknown TTL display with conservative reset heuristic, read-error reporting, and no silent partial parse when an underlying reader/scanner reports an error after valid lines.
  - Run: `go test ./internal/session`.
  - Expected: fails because parser implementation does not exist yet.

- [x] **Step 2.3: Implement session model and parser**
  - Create: `internal/session/model.go`, `internal/session/parser.go`.
  - Constraints: parser exposes a reader-based testable path; no `bufio.Scanner` without increased buffer and checked `Err`; no rendering assumptions about UUID length; time-derived elapsed/remaining fields computed from a provided clock outside raw parser output.
  - Run: `go test ./internal/session`.
  - Expected: parser tests pass.

- [x] **Step 2.4: Write failing discovery tests**
  - Create: `internal/session/discover_test.go`.
  - Assertions: recursive discovery under `~/.claude/projects`, sort by JSONL file modification time, partial ID substring matching against filename stems, no-match error, ambiguous-match result listing candidates.
  - Run: `go test ./internal/session`.
  - Expected: fails until discovery exists.

- [x] **Step 2.5: Implement discovery and partial ID resolution**
  - Create: `internal/session/discover.go`.
  - Behavior: missing projects directory returns a structured empty/degraded state, not an unstructured process exit.
  - Run: `go test ./internal/session`.
  - Expected: all session package tests pass.

- [x] **Step 2.6: Parser parity gate**
  - Run: `go test ./internal/session -count=1`.
  - Expected: pass.
  - Gate: no TUI package files, JSON session output, Reminder session-consumer code, or KeepAlive session-consumer code are implemented before this step passes. Normal execution remains phase-linear; do not skip ahead to config-only work unless the user explicitly overrides the runbook.
