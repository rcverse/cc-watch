# cc-watch v2 Meta Implementation Plan

> Active phase checklists live beside this file as `phase-XX-*.md`. Use `docs/superpowers/runbooks/2026-06-03-cc-watch-v2-implementation-runbook.md` as the execution protocol and `docs/superpowers/progress/cc-watch-v2-progress.md` as the progress ledger.

> **For agentic workers:** This file is context and global gates, not the active step checklist. Execute the current phase file linearly. `superpowers:subagent-driven-development` may be used only as a methodology reference, not as the default per-step worker/reviewer loop.

**Goal:** Build cc-watch v2 as a Go/Bubbletea single-binary terminal app that preserves v1 parser behavior, adds safe live TUI workflows for Reminder and bounded KeepAlive, provides stable JSON output, and first ships as a locally installable binary.

**Architecture:** Keep parsing, config, refresh, notification, KeepAlive automation, JSON output, and TUI rendering in separate Go packages under `internal/`, with `cmd/cc-watch` limited to CLI bootstrap. Parser parity is completed and verified before TUI build-out. Existing Python v1 remains callable until the Go binary is built, verified, and explicitly installed.

**Tech Stack:** Go 1.23+, Bubbletea, bubbles, lipgloss, fsnotify, Go standard-library JSONL/config/subprocess handling, `osascript` on macOS, local binary install first. Linux, Homebrew, goreleaser, and GitHub Release publishing are out of scope unless separately approved.

---

## Phase Index

- [Phase 0: Implementation Authorization, ADRs, And Migration Contract](phase-00-adrs-migration.md)
- [Phase 1: Preserve v1 And Establish Go Project Skeleton](phase-01-go-skeleton.md)
- [Phase 2: Parser Parity Before TUI](phase-02-parser-parity.md)
- [Phase 3: Config, Reminder Core, JSON Contract, And CLI Non-TUI Behavior](phase-03-config-reminder-json.md)
- [Phase 4: Bubbletea Root Model, Messages, And Visual System](phase-04-bubbletea-root.md)
- [Phase 5: Refresh Architecture And Degraded State](phase-05-refresh-architecture.md)
- [Phase 6: List View](phase-06-list-view.md)
- [Phase 7: Notification System](phase-07-notifications.md)
- [Phase 8: KeepAlive State Machine And Bounded Automation](phase-08-keepalive-core.md)
- [Phase 9: Session Workspace](phase-09-session-workspace.md)
- [Phase 10: Config Editor](phase-10-config-editor.md)
- [Phase 11: CLI Integration And Full Non-Release Verification](phase-11-cli-integration.md)
- [Phase 11.6: Adaptive Terminal Elegance](phase-11.6-adaptive-terminal-elegance.md)
- [Phase 11.7: TUI Interaction Architecture Refactor](phase-11.7-tui-interaction-architecture.md)
- [Phase 11.8: Architecture Refactor](phase-11.8-architecture-refactor.md)
- [Phase 11.9: Architecture Simplification](phase-11.9-architecture-simplification.md)
- [Phase 12: Packaging, Install, And Release Path](phase-12-packaging-install.md)
- [Phase 13: Documentation Updates](phase-13-documentation.md)
- [Phase 14: Final Acceptance Verification](phase-14-final-acceptance.md)

## Source Of Truth

- **Product source of truth:** `PRD.md`.
- **Current product boundary:** `docs/superpowers/specs/2026-06-18-cc-watch-v2-product-reality.md`.
- **Design source of truth:** `docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md`.
- **Meta plan source of truth:** this file, `docs/superpowers/plans/cc-watch-v2/PLAN.md`.
- **Phase checklist source of truth:** `docs/superpowers/plans/cc-watch-v2/phase-XX-*.md`.
- **Progress source of truth:** `docs/superpowers/progress/cc-watch-v2-progress.md`.
- **Execution protocol source of truth:** `docs/superpowers/runbooks/2026-06-03-cc-watch-v2-implementation-runbook.md`.
- **ADR source of truth:** date-prefixed decision files under `docs/adr/`, created only for decisions named in this plan or newly discovered ambiguity that affects migration safety, architecture, user data, subprocess automation, packaging, or public CLI behavior.
- **Code/test source of truth once implemented:** Go source and tests under `cmd/`, `internal/`, `testdata/`, local binary install behavior in `install.sh`, and user docs in `README.md`.
- **Explicitly excluded as source of truth:** retired implementation plans and historical v1 planning text.

## Current Repo Observations

- Current tracked runtime is a single executable Python script at `cc_watch.py`.
- Current tracked installer is `install.sh`, which force-symlinks `$HOME/.local/bin/cc-watch` to `/Users/richardchen/Dev/cc-watch/cc_watch.py`.
- The live command path currently resolves to `/Users/richardchen/.local/bin/cc-watch`, and that path is a symlink to `/Users/richardchen/Dev/cc-watch/cc_watch.py`.
- No symlinks exist inside the repository.
- There is no Git remote configured, so public release metadata cannot be inferred from `git remote`.
- Current legacy files include `cc_watch.py` and `install.sh`; current v2 source lives under `cmd/`, `internal/`, and `docs/`.
- Current working tree before this plan had unrelated changes: modified `PRD.md`, modified `docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md`, historical UX audit material now archived under `docs/superpowers/specs/archive/`, untracked `docs/superpowers/plans/`, and untracked `docs/superpowers/specs/archive/`.

## Planning Assumptions And Uncertainties

- Treat `PRD.md` as resolving the design spec's older "draft; implementation planning is blocked" status because `PRD.md` says v2 design is synced and the user requested a plan for the accepted design.
- Preserve the existing usable command path during migration by leaving root `cc_watch.py` in place until the Go binary has passed acceptance verification and the user explicitly approves the local install/symlink switch.
- Archive v1 by copying it into `archive/v1/` during implementation, not by moving the root script early. Moving root `cc_watch.py` before switchover would break the currently installed symlink.
- Parser parity means v2 Go parsing behavior matches the documented v1-compatible rules, not every ANSI rendering detail of `cc_watch.py`.
- Use one repository for governance: v2 Go code, v1 archive, docs, installer, tests, and release metadata all live in `/Users/richardchen/Dev/cc-watch`.
- Use Go module path `github.com/richardchen/cc-watch` unless the user explicitly changes it before Phase 1.
- Defer Homebrew, goreleaser, GitHub Release, Linux, and Windows work until the user explicitly re-approves that scope.
- The exact safety refresh interval is intentionally not a public CLI option. The implementation must choose an internal value by ADR because stale refresh behavior affects user trust and watcher load.
- The user's environment may not have Go installed. Implementation must verify or install Go tooling before `go mod init`; do not assume a working Go toolchain.
- No real Claude KeepAlive send is part of implementation verification. This is a test-driven safety decision: subprocess behavior is tested through fakes and deterministic fixture session files.
- Any uncertainty that affects data safety or automation must be documented in an ADR before code is written for that area.

## Implementation Readiness Gates

- Implementation may start only after the user approves this plan.
- No TUI build-out starts until parser parity tests pass.
- No KeepAlive subprocess execution is wired into the TUI until the state machine has unit tests for every transition and failure path.
- No installer update or symlink replacement happens until a built Go binary passes CLI, JSON, TUI smoke, and v1-preservation checks.
- No public release artifacts are cut until local binary install is stable, goreleaser dry-run succeeds, and the user explicitly approves release publishing.

## Target File Structure

### Create

- `archive/v1/README.md` - explains the v1 archive, command-path migration, and how to run v1 directly if needed.
- `archive/v1/cc_watch.py` - copied v1 script for provenance.
- `archive/v1/install-v1.sh` - copied v1 installer for rollback/reference.
- `docs/adr/2026-06-03-v1-archive-and-command-path.md` - final migration/symlink decision.
- `docs/adr/2026-06-03-go-module-and-release-identity.md` - Go module path, one-repo governance, deferred public release, and deferred Homebrew tap decision.
- `docs/adr/2026-06-03-refresh-safety-interval.md` - internal safety refresh interval and debounce behavior.
- `docs/adr/2026-06-03-keepalive-subprocess-safety.md` - Claude subprocess, confirmation, cancellation, and scope safety decision.
- `docs/adr/2026-06-03-json-output-schema.md` - public JSON schema version, success shape, error shape, and degraded-state encoding.
- `go.mod`, `go.sum` - Go module definition.
- `cmd/cc-watch/main.go` - CLI entry point only.
- `internal/app/cli.go` - argument parsing and command dispatch.
- `internal/app/app.go` - top-level app assembly.
- `internal/app/version.go` - version metadata used by `--version` and goreleaser.
- `internal/app/cli_test.go` - CLI dispatch, rejected mode, and no-side-effect tests.
- `internal/session/model.go` - parsed session domain model and time-derived status helpers.
- `internal/session/discover.go` - `~/.claude/projects/**/*.jsonl` discovery and partial ID resolution.
- `internal/session/parser.go` - JSONL parsing and v1-compatible metrics.
- `internal/session/parser_test.go` - parser parity tests.
- `internal/session/discover_test.go` - discovery and partial-ID tests.
- `internal/session/testdata/*.jsonl` - parser fixtures for all documented parity cases.
- `internal/session/testdata/smoke-home/.claude/projects/-tmp-cc-watch/11111111-1111-1111-1111-111111111111.jsonl` - deterministic safe smoke session.
- `internal/session/testdata/smoke-home/.config/cc-watch/config.json` - deterministic smoke config with KeepAlive auto-send disabled.
- `internal/config/model.go` - config model and defaults.
- `internal/config/store.go` - config file load/save/reset behavior for `~/.config/cc-watch/config.json`.
- `internal/config/validation.go` - config validation and effective KeepAlive summary.
- `internal/config/config_test.go` - defaults, validation, save/cancel/reset tests.
- `internal/jsonout/json.go` - stable JSON output structs and encoding.
- `internal/jsonout/json_test.go` - schema and degraded-state output tests.
- `internal/notify/notifier.go` - notifier interface, macOS `osascript` command construction, delivery, and failure suppression.
- `internal/notify/notifier_test.go` - escaping, failure suppression, and event wording tests.
- `internal/refresh/watcher.go` - fsnotify watcher setup and event normalization.
- `internal/refresh/refresh.go` - debounced data refresh and safety refresh coordination.
- `internal/refresh/refresh_test.go` - watcher message, debounce, and degraded-state tests.
- `internal/keepalive/model.go` - KeepAlive per-session state model.
- `internal/keepalive/timing.go` - TTL-aware trigger and countdown calculations.
- `internal/keepalive/state.go` - KeepAlive state machine.
- `internal/keepalive/runner.go` - Claude CLI runner interface and real subprocess runner.
- `internal/keepalive/confirm.go` - session-specific JSONL confirmation watcher.
- `internal/keepalive/keepalive_test.go` - state, timing, scope, cancellation, and failure tests.
- `internal/tui/model.go` - Bubbletea root model, shared state, and routing.
- `internal/tui/messages.go` - explicit Bubbletea messages for ticks, watcher events, refresh results, notifications, and KeepAlive events.
- `internal/tui/update.go` - root update loop and command production.
- `internal/tui/live_refresh.go` - Bubble Tea adapter for refresh watcher results.
- `internal/tui/reminder_runtime.go` - TUI-local reminder threshold runtime.
- `internal/tui/route_actions.go` - route-local focused action dispatch.
- `internal/tui/snapshot_options.go` - snapshot-to-TUI option projection.
- `internal/tui/workspace_keepalive.go` - Workspace KeepAlive view-state helpers.
- `internal/tui/view.go` - root view composition.
- `internal/tui/styles.go` - lipgloss semantic color/style roles.
- `internal/tui/list.go` - List View model/render/update.
- `internal/tui/workspace.go` - Session Workspace model/render/update.
- `internal/tui/config_editor.go` - Config Editor model/render/update.
- `internal/tui/render_test.go` - render assertions for required wording and degraded states.
- `internal/tui/update_test.go` - shared update, focus, shortcut, and state transition tests.
- `internal/tui/update_config_test.go`, `internal/tui/update_keepalive_test.go`, `internal/tui/update_refresh_test.go` - focused Config, KeepAlive, and Refresh update tests.
- `.goreleaser.yaml` - optional release build configuration after local binary install is verified.

### Modify

- `install.sh` - after v2 binary verification, install the compiled Go binary to `$HOME/.local/bin/cc-watch` without breaking the existing v1 symlink before switchover.
- `README.md` - replace stale v1/v2 usage with actual v2 CLI, install, JSON, config, KeepAlive safety, and rollback notes.
- `.gitignore` - add Go build, coverage, and local release artifacts without hiding source fixtures.
- `PRD.md` and `docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md` - do not modify unless implementation reveals a product/design conflict that requires user-approved source-of-truth change.
- `cc_watch.py` - leave in place during v2 implementation. Remove or replace only after ADR approval and verified command-path switchover; the default plan is to keep it as a legacy v1 entry point through the first v2 release.

## Public JSON Contract

`--json` is a public interface and must be implemented from this contract, then recorded in `docs/adr/2026-06-03-json-output-schema.md`.

Success output uses `schema_version: 1` and one top-level object:

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-03T12:00:00Z",
  "query": {"id": null, "limit": 5},
  "refresh": {
    "mode": "snapshot",
    "watcher": {"status": "not_started", "degraded": false, "messages": []},
    "safety_refresh_active": false,
    "last_refresh_at": "2026-06-03T12:00:00Z"
  },
  "notifications": {"status": "not_started", "degraded": false, "recent": []},
  "sessions": [],
  "selected_session": null,
  "error": null
}
```

Each session object includes stable fields:

```json
{
  "session_id": "11111111-1111-1111-1111-111111111111",
  "short_id": "11111111",
  "project": "tmp-cc-watch",
  "jsonl_path": "/tmp/home/.claude/projects/-tmp-cc-watch/11111111-1111-1111-1111-111111111111.jsonl",
  "file_modified_at": "2026-06-03T12:00:00Z",
  "cache_window": {"tier": "1h", "label": "1h", "ttl_seconds": 3600, "known": true, "evidence": ["ephemeral_1h_input_tokens"]},
  "status": {"state": "active", "last_message_at": "2026-06-03T11:55:00Z", "remaining_seconds": 3300, "expired_seconds": null, "percent_elapsed": 8.33},
  "messages": {"first_user_excerpt": "start", "last_user_excerpt": "continue"},
  "token_stats": {"cache_writes": 100, "cache_reads": 900, "output_tokens": 50, "hit_rate": 90.0},
  "gaps": {"count": 0, "reset_count": 0, "latest": []},
  "warnings": [],
  "reminder": {"available": false, "enabled": null, "thresholds": [20, 10], "fired": []},
  "keep_alive": {"available": false, "enabled": null, "auto_send": null, "state": "unavailable", "scope": null, "last_result": null}
}
```

Error output uses the same top-level shape with `sessions` containing candidates when safe and `error` set:

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-03T12:00:00Z",
  "query": {"id": "111", "limit": 5},
  "refresh": {"mode": "snapshot", "watcher": {"status": "not_started", "degraded": false, "messages": []}, "safety_refresh_active": false, "last_refresh_at": "2026-06-03T12:00:00Z"},
  "notifications": {"status": "not_started", "degraded": false, "recent": []},
  "sessions": [{"session_id": "11111111-1111-1111-1111-111111111111", "short_id": "11111111", "project": "tmp-cc-watch"}],
  "selected_session": null,
  "error": {"code": "ambiguous_session_id", "message": "partial id matched multiple sessions", "query": "111"}
}
```

Allowed error codes are `projects_dir_missing`, `no_sessions_found`, `session_not_found`, `ambiguous_session_id`, `parse_error`, and `config_error`. Error commands exit non-zero except first-run empty list states, which may exit zero with `sessions: []` and `error: null`.

## Failure-Mode Map

| Path / component | What can go wrong | Visibility | Mitigation | Test coverage |
|---|---|---|---|---|
| v1 migration | Existing symlink breaks if root `cc_watch.py` moves early | User command fails | Copy archive first; leave root script until verified switchover | Phase 1 command-path checks |
| Parser | Long JSONL line exceeds default scanner buffer or read error occurs after valid lines | Session data silently incomplete | Increase buffer or use reader approach; check read errors and report warning/error | Phase 2 long-line and read-error tests |
| Parser | Malformed JSONL line aborts file | Valid later lines lost | Ignore malformed line and record warning | Phase 2 malformed fixture |
| Discovery | Partial ID matches multiple files | Wrong session opens | Ambiguous selection/error state lists matches | Phase 2 and Phase 6 tests |
| Refresh | fsnotify misses event, partial watch fails, or stale parse completes late | Stale UI | Safety refresh, manual refresh, degraded banner, generation-based result ordering | Phase 5 tests |
| Bubbletea | Goroutine mutates model directly | Race/stale state | watcher sends explicit messages only | Phase 4/5 tests |
| Reminder | Reminder triggers Claude subprocess | User sends unintended message | Reminder package has no runner dependency | Phase 3 tests |
| Notification | OS notification command fails | User misses alert | TUI degraded state and event log/status | Phase 7 tests |
| Notification | AppleScript injection through title/body | Unsafe command text | Escape title/body safely | Phase 7 tests |
| KeepAlive timing | 5-minute TTL sends immediately | Unsafe automation | TTL-aware trigger and safety margin clamp | Phase 8 timing tests |
| KeepAlive loop | Tick repeatedly starts sends | Multiple unintended messages | Edge-triggered state machine and scope count | Phase 8 state tests |
| KeepAlive async | Stale timer/runner/confirmation message fires after cancellation | Unintended send or false success | Instance tokens and TUI stale-message ignore tests | Phase 8/9 tests |
| KeepAlive subprocess | `claude` unavailable or limit error | Hidden failed automation | failure card, stop auto-send, fallback command | Phase 8/9 tests |
| KeepAlive confirmation | Success inferred from subprocess only | False confidence | Require target JSONL evidence after send time | Phase 8 tests |
| Config | Invalid defaults saved | Runtime unsafe or broken | validation blocks save | Phase 3/10 tests |
| JSON | Scripts depend on unstable fields | Downstream breakage | explicit schema version, success/error contract, and JSON tests | Phase 0 ADR, Phase 3/11 tests |
| Packaging | Installer overwrites working v1 path too early or rollback cannot restore command | Local command breakage | build/smoke checks before user-approved switch and rollback verification | Phase 12 checks |

## Requirement Coverage Matrix

| Requirement area | Plan coverage |
|---|---|
| v1 archive/migration and current command path | Phases 0, 1, 12 |
| Clean Go project structure | Target File Structure, Phase 1 |
| Parser parity before TUI | Phase 2 gate before Phase 4 |
| Parser fixtures and tests | Phase 2 |
| Bubbletea app model | Phase 4 |
| fsnotify plus safety refresh | Phase 5 |
| List View | Phase 6 |
| Session Workspace | Phase 9 |
| Reminder behavior | Phases 3, 7, 9 |
| KeepAlive bounded state machine | Phase 8 |
| KeepAlive visible/cancellable/confirmation-based UI | Phase 9 |
| Notification behavior and degraded states | Phases 7, 9, 11 |
| Config Editor | Phase 10 |
| JSON output | Phases 3, 11 |
| Packaging/release path | Phase 12 local binary first, optional public release later |
| Documentation updates | Phase 13 |
| Implementation readiness gates | Implementation Readiness Gates, Phases 0, 2, 8, 12, 14 |

## Dependency Map

This map explains dependencies and merge risk. It is not permission to parallelize implementation. The runbook's default is linear phase execution.

| Area | Scope | Depends on | Normal execution | Merge/conflict risk |
|---|---|---|---|---|
| Parser/data | `internal/session`, fixtures | Phase 1 | Phase 2, must complete before TUI/session consumers | Low |
| Config | `internal/config` | Phase 1 | Phase 3, after parser parity in normal execution | Low |
| JSON/Reminder | `internal/jsonout`, `internal/reminder` | Phase 2.6 parser parity gate | Phase 3 | Medium |
| Refresh/notifications | `internal/refresh`, `internal/notify` | TUI message contracts | Phases 5 and 7 | Medium |
| KeepAlive core | `internal/keepalive` | config/session models and ADR | Phase 8 | Medium |
| TUI surfaces | `internal/tui` | parser, config, reminder, refresh, KeepAlive contracts | Phases 4, 6, 9, 10 | High |
| Packaging/docs | optional `.goreleaser.yaml`, `install.sh`, docs | verified binary behavior | Phases 12 and 13 | Medium |

## Adversarial Audit Integration

- Parser/migration audit findings accepted: added parser read-error tests, earlier CLI no-side-effect tests, stricter parser-parity lane sequencing, and rollback verification through the command path.
- Watcher/TUI audit findings accepted: added refresh generation ordering, display tick negative tests, cursor-focus List actions for Reminder/KeepAlive, workspace manual refresh cursor path, partial watcher degraded cases, and contextual help/footer assertions.
- KeepAlive/UX audit findings accepted: added stale async event cancellation tests, `claude` availability preflight while armed, event-specific notification wording, disabled-control focus reasons, dangerous-state default focus, and success evidence rendering.
- Final readiness audit findings accepted: added a public JSON contract and ADR, deterministic fixture HOME for smoke commands, explicit no-real-send manual smoke setup, and app file-scope updates for JSON CLI wiring.

## Remaining Approval Questions

- Approve Go toolchain installation if `go version` is missing or older than Go 1.23.
- Approve local command-path switchover separately before replacing `$HOME/.local/bin/cc-watch`.
- Approve release publishing separately after snapshot release verification.

## Out Of Scope For v2

- Native macOS app.
- Background daemon.
- Public watch mode.
- Configurable watch interval.
- Unbounded or hidden KeepAlive loop.
- Anthropic network/API calls.
- Windows support.
- Mouse-first interaction requirement.
- Publishing a release or changing the live symlink without explicit user approval.
