# cc-watch v2 Phase 14: Final Acceptance

> Refreshed on 2026-06-29 before execution. This phase verifies the implemented product; it must not add behavior, switch the real installed command, run a real Claude KeepAlive send, publish releases, or start Homebrew/goreleaser/Linux/Windows work.

## Phase 14: Final Acceptance Verification

**Purpose:** Verify product completeness against PRD/design before declaring v2 done.

- [x] **Step 14.1: Full automated test suite**
  - Run: `env GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test -count=1 ./...`.
  - Expected: pass.

- [x] **Step 14.2: Build verification**
  - Run: `env GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go build -o dist/cc-watch ./cmd/cc-watch`.
  - Expected: binary builds successfully.
  - Run:
    - `env HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-watch --help`
    - `env HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-watch --version`
    - `env HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-watch --json`
    - `env HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-watch --json --id 11111111`
    - a retired watch flag attempt
  - Expected: help/version/JSON commands succeed; JSON is valid and emits `schema_version: 1`; retired/unknown flags exit non-zero as ordinary unknown flags.

- [x] **Step 14.3: Local install and v1 preservation verification**
  - Run: `./install.sh --dry-run`.
  - Expected: prints build and target paths without writing.
  - Run: `scripts/test-install.sh`.
  - Expected: installs only under temporary `HOME` and smoke-checks the installed binary.
  - Run: `command -v cc-watch` and `ls -l "$HOME/.local/bin/cc-watch"`.
  - Expected: real command path still resolves to the v1 symlink unless the user has explicitly approved switchover.

- [x] **Step 14.4: TUI PTY acceptance**
  - Run in a PTY: `env HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-watch`.
  - Expected: List View renders, shows session/cache status, has visible cursor-focused rows/actions, and exits with `q`.
  - Run in a PTY: `env HOME="$PWD/internal/session/testdata/smoke-home" PATH="/usr/bin:/bin" dist/cc-watch --id 11111111`.
  - Expected: Session Workspace renders, separates Cache Status/Session Info/Controls, keeps `claude` unavailable visible when relevant, and exits with `q`.
  - Run in a PTY: `env HOME="$PWD/internal/session/testdata/smoke-home" dist/cc-watch config`.
  - Expected: Config Editor renders Reminder/KeepAlive defaults, validation/save/reset/cancel affordances, and exits with `q` without saving.

- [x] **Step 14.5: KeepAlive safety acceptance**
  - Run focused deterministic tests:
    - `env GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/keepalive ./internal/tui -run 'TestStateMachineTransitionsThroughRequiredStates|TestStateMachineImmediateManualAndFailureStates|TestStateMachineIsEdgeTriggeredScopedAndPerSession|TestStateMachineIgnoresStaleTokenEventsAfterCancellationAndRefresh|TestStateMachineDoesNotRearmWithinSameTriggerWindowWhenScopeRemains|TestCountdownElapsedDoesNotSendWhenAutoSendTurnsOffOrSafetyDeadlineIsMissed|TestFailureAcknowledgeMovesExhaustedScopeToScopeComplete|TestRunnerAvailabilityFailuresAreVisibleAndBounded|TestRunnerSubprocessFailuresAndLimitErrorsStopAutoSend|TestConfirmationRequiresTargetSessionLineAfterSendAttempt|TestConfirmationIgnoresTargetLinesThatExistedBeforeSendAttempt|TestConfirmationWaitTimeoutPath|TestWorkspaceKeepAliveCardStatesRenderSafetyContract|TestDisplayTickEvaluatesKeepAliveMonitoringSessions|TestWorkspaceIgnoresStaleKeepAliveAsyncMessages|TestWorkspaceIgnoresKeepAliveAsyncAfterRefreshGenerationOrSelectionChanges|TestWorkspaceKeepAliveActionsProduceRunnerAndConfirmationCommands'`
  - Expected: fake-runner/fake-confirmation tests prove countdown visibility, manual prompt with Auto-send off, unsafe timing disablement, cancellation/stale async safety, preflight/failure fallback, confirmation-based success, scope counting, and no repeated send loop without invoking real `claude`.

- [x] **Step 14.6: Requirement mapping check**
  - Review every section of `PRD.md` and `docs/superpowers/specs/2026-06-02-cc-watch-v2-design.md`.
  - Expected: each requirement maps to a completed task in this plan or a documented non-goal.
  - Gate: any missing mapping becomes a new plan task or ADR before release.

- [x] **Step 14.7: Final hygiene, review, and ledger**
  - Run: `git diff --check`.
  - Dispatch one read-only final acceptance reviewer after local verification passes.
  - Integrate any accepted findings and rerun relevant verification.
  - Update this checklist and `docs/superpowers/progress/cc-watch-v2-progress.md`.

## Acceptance Mapping

### PRD Mapping

| PRD area | Acceptance evidence |
|---|---|
| 1. Product Summary, 2. Problem, 3. Goals | Phases 2-13 implemented parser, TUI, Reminder, KeepAlive, refresh, JSON, install, and docs; Phase 14 full suite/build/PTY smokes verified the final product shape. |
| 4. Non-Goals | Public watch mode, daemon, native app, Linux/Windows, Homebrew/GitHub Release/goreleaser, Anthropic API calls, and hidden/unbounded KeepAlive remain out of scope; retired watch flags are not registered in the CLI. |
| 5. Primary Users | List View, Workspace, Reminder/KeepAlive, and JSON paths map to the named user needs; verified by focused tests and Phase 14 PTY/JSON smokes. |
| 6. CLI Contract | `dist/cc-watch --help`, `--version`, `--json`, `--json --id`, TUI, Workspace, Config, and ordinary unknown-flag checks for retired flags. |
| 7. Core UX Surfaces | List View, Workspace, and Config Editor covered by `internal/tui` tests plus Phase 14 PTY smokes. |
| 8. Reminder Requirements | Reminder runtime/config/notification separation covered by `internal/tui`, `internal/notify`, and JSON tests; docs and PTY output preserve "sends no Claude message" copy. |
| 9. KeepAlive Requirements | `internal/keepalive` and `internal/tui` focused safety tests cover trigger timing, manual/Auto-send paths, cancellation, stale async, confirmation, failure, and scope. No real Claude send was run. |
| 10. Data Requirements | `internal/session`, `internal/snapshot`, and `internal/jsonout` tests cover discovery, parser parity, status, gaps/resets, warnings, and JSON projection. |
| 11. Live Refresh Requirements | `internal/refresh`, `internal/tui`, and `internal/app` tests cover watcher messages, debounce, safety refresh, manual refresh, degraded state, and snapshot refresh boundaries. |
| 12. Notifications | `internal/notify` and TUI tests cover macOS `osascript`, escaping, failure suppression, degraded visibility, and Reminder/KeepAlive wording separation. |
| 13. Config And Runtime State | `internal/config` and `internal/tui` config tests cover defaults, validation, save/cancel/reset, global defaults, and in-memory per-session runtime state. |
| 14. JSON Output | `internal/jsonout`, `internal/app`, and Phase 14 fixture `--json` smokes verify `schema_version: 1`, selected/list shapes, degraded state, Reminder, and KeepAlive fields. |
| 15. Tech Stack | Go/Bubbletea/lipgloss/fsnotify/osascript/local install are implemented in code and verified by build/tests. |
| 16. Packaging | `./install.sh --dry-run`, `scripts/test-install.sh`, and real command-path checks verify local install while preserving the v1 symlink. |
| 17. Acceptance Criteria | Covered by the Phase 14 automated suite, PTY smokes, KeepAlive safety tests, install checks, and this mapping. |

### Design Spec Mapping

| Design area | Acceptance evidence |
|---|---|
| 1-4 Product direction, non-goals, stack, CLI | Product reality/README/plan aligned in Phase 13; Phase 14 build and CLI checks verify current behavior. |
| 5 Parser equivalence | Phase 2 parser fixtures and current `go test -count=1 ./...` cover v1-compatible parsing and long-line/error cases. |
| 6 Refresh architecture | Phase 5/11.7/11.9 tests cover fsnotify acceleration, safety refresh, manual refresh, degraded state, and Bubbletea messages. |
| 7 Data model | Session/snapshot/jsonout tests and fixture JSON smoke cover model fields and time-derived output. |
| 8 Reminder system | TUI reminder runtime, notification, JSON, and config tests cover alarm-only behavior. |
| 9 KeepAlive system | KeepAlive core and TUI safety acceptance tests cover state machine, timing, user paths, confirmation, failure, and scope. |
| 10 TUI UX contract and walkthroughs | TUI render/update/help/config tests plus Phase 14 PTY smokes cover List, Workspace, Config, help, focus, shortcuts, and degraded workflows. |
| 11 Notifications | Notify and TUI tests cover macOS notification command construction, degraded states, suppression, and event wording. |
| 12 Config and state files | Config package and Config Editor tests cover file path, defaults, validation, runtime/session separation, and reset behavior. |
| 13 JSON output | JSON contract ADR plus jsonout/app tests and Phase 14 JSON smokes cover stable machine output. |
| 14 Packaging | Phase 12 installer tests and Phase 14 dry-run/temp-HOME install checks cover local macOS install. |
| 15 Implementation readiness gate | Earlier phase ordering enforced parser parity before TUI and KeepAlive safety before UI wiring; current full suite verifies those gates remain intact. |
