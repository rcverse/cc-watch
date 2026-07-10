# Codebase Reduction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove high-confidence stale code, test-only production surfaces, and dead metadata while preserving every currently surfaced CLI/TUI capability and all KeepAlive/statusline safety boundaries.

**Architecture:** Keep the existing packages and user-visible workflows, but make their interfaces carry only data consumed by the installed binary. Retain fsnotify, the snapshot boundary, the demo harness, responsive Workspace views, statusline backups/manual review, and KeepAlive confirmation/timeouts. Each task is independently testable and must leave the tree smaller.

**Tech Stack:** Go 1.23+, Bubble Tea, Lip Gloss, fsnotify, macOS `osascript`, shell installer tests.

## Global Constraints

- Work with the current dirty statusline changes; never reset or overwrite them.
- Every task must have a negative net LOC result and add no dependency or package.
- Never run a real KeepAlive send. Use fake runners and fixture homes only.
- Keep argv-only subprocess execution, the 30-second KeepAlive timeout, JSONL confirmation, the 5-second statusline timeout, manual-review refusal, and backup-before-settings-write.
- Keep `cc-watch statusline --help`; earlier CLI work explicitly chose layered help. Shorten duplicated top-level copy only.
- Keep `internal/refresh`, `internal/snapshot`, `tools/ui-demo`, `archive/v1`, and the surfaced Workspace detail modes in this pass.
- Do not preserve unreleased internal/test APIs merely for compatibility.
- Commit after each task only after its focused tests pass.

---

### Task 1: Slim the Current Statusline Change

**Files:**
- Modify: `internal/statusline/settings.go`
- Modify: `internal/statusline/settings_test.go`
- Modify: `internal/ratelimit/model.go`
- Modify: `internal/ratelimit/model_test.go`
- Modify: `internal/ratelimit/project.go`
- Modify: `internal/ratelimit/project_test.go`
- Modify: `internal/app/statusline_hook.go`
- Modify: `internal/app/statusline_hook_test.go`
- Modify: `internal/app/cli.go`
- Modify: `internal/app/cli_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/config_editor.go`
- Modify: `internal/tui/update_config_test.go`

**Interfaces:**
- Keep: `statusline.Inspect(home string) (Status, error)`.
- Change: `statusline.Install(home string) error` and `statusline.Uninstall(home string) error`.
- Change TUI dependencies to `InstallStatusline func() error` and `UninstallStatusline func() error`.
- Change: `ratelimit.State.TierCache` to `map[string]int`.
- Change: `ratelimit.State.AddReading(reading Reading)` and `AddSevenDayReading(reading Reading)`.
- Keep the exact visible statusline formats and settings backup behavior.

- [ ] **Step 1: Reduce statusline settings result types**

Make the public status type contain only consumed fields:

```go
type Status struct {
	State           State
	Command         string
	PreviousCommand string
}
```

Change install/uninstall to return only errors. After a successful write, callers re-read through `Inspect` when rendering:

```go
func Install(home string) error
func Uninstall(home string) error
```

Wire the TUI dependency shape directly:

```go
type Dependencies struct {
	// existing dependencies
	InspectStatusline   func() (statusline.Status, error)
	InstallStatusline   func() error
	UninstallStatusline func() error
}
```

Remove the impossible nil install/uninstall branches from `activateStatuslineConfigAction`; production always wires both functions. Keep the manual-review branch before either write.

- [ ] **Step 2: Remove unused rate-limit fields**

Use the smallest persisted model consumed by momentum and display:

```go
type HistoryPoint struct {
	UsedPct  float64
	ResetsAt time.Time
}

type State struct {
	History         []HistoryPoint
	SevenDayHistory []HistoryPoint
	TierCache       map[string]int
}

type Projection struct {
	MessagesLeft int
	AtRisk       bool
}
```

Remove `CapturedAt`, `PingsNeeded`, `TierInfo`, and `TierInfo.Known`. A tier cache entry exists only after a known TTL is parsed.

- [ ] **Step 3: Remove duplicate shell/help code**

Replace `NeedsShell` with one standard-library check:

```go
func NeedsShell(command string) bool {
	return strings.ContainsAny(command, "|&;<>()$`\\\n=")
}
```

Keep `WriteStatuslineHelp`. In `WriteHelp`, retain only the statusline command list and `See: cc-watch statusline --help`; remove the repeated mode explanations and examples already present in the subcommand help.

- [ ] **Step 4: Consolidate tests around behavior**

Keep settings tests for these trust-boundary cases: missing settings install, existing command preservation, shell command preservation, bare uninstall, wrapped uninstall, ambiguous refusal, and backup creation. In the TUI, keep one install/uninstall round-trip test and one manual-review no-write test; remove assertions against synthetic action labels.

- [ ] **Step 5: Run focused verification**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./internal/statusline ./internal/ratelimit ./internal/app ./internal/tui
```

Expected: all four packages pass; statusline settings tests confirm backup and ambiguous-write refusal.

- [ ] **Step 6: Commit the slim statusline feature**

```bash
git add internal/statusline internal/ratelimit internal/app internal/tui AGENTS.md README.md docs/decisions.md
git commit -m "Add reversible Claude Code statusline integration"
```

Expected net: remove 50-90 lines from the current uncommitted implementation before committing it.

### Task 2: Remove Retired Compatibility and an Unused Dependency

**Files:**
- Delete: `cc_watch.py`
- Modify: `scripts/test-install.sh`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/config/store.go`
- Modify: `internal/config/config_test.go`
- Modify: `AGENTS.md`
- Modify: `archive/v1/README.md`

**Interfaces:**
- Keep the Go binary installer and archived `archive/v1/cc_watch.py`.
- Remove `config.Reset`; the config route already calls `Save(Default())`.

- [ ] **Step 1: Establish the installer baseline**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false scripts/test-install.sh
```

Expected: PASS before deletion.

- [ ] **Step 2: Delete the duplicate Python entry point**

Delete root `cc_watch.py`. In `scripts/test-install.sh`, remove the root-v1 help/executable checks at lines 47-60. Keep the switchover test, but seed its legacy symlink from the archive:

```bash
ln -s "$ROOT/archive/v1/cc_watch.py" "$TEST_HOME/.local/bin/cc-watch"
```

Update `AGENTS.md` and `archive/v1/README.md` so only `archive/v1/cc_watch.py` is described as historical.

- [ ] **Step 3: Remove the unused direct module**

Run:

```bash
go mod edit -droprequire=github.com/charmbracelet/bubbles
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go mod tidy
```

Expected: no `github.com/charmbracelet/bubbles` requirement remains; Bubble Tea and Lip Gloss remain.

- [ ] **Step 4: Delete `config.Reset` and its test**

Remove:

```go
func Reset(home string) error {
	return Save(home, Default())
}
```

Delete `TestResetRestoresDefaults`; config reset remains covered through the TUI save/reset path.

- [ ] **Step 5: Verify and commit**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./internal/config ./internal/app
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false scripts/test-install.sh
```

Expected: all commands pass.

```bash
git add -A cc_watch.py scripts/test-install.sh go.mod go.sum internal/config AGENTS.md archive/v1/README.md
git commit -m "Remove retired v1 compatibility code"
```

Expected net: 400-440 lines and one direct dependency removed.

### Task 3: Finish the `--remind` Retirement in Snapshot State

**Files:**
- Modify: `internal/config/store.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/snapshot/snapshot.go`
- Modify: `internal/snapshot/snapshot_test.go`
- Modify: `internal/tui/snapshot_options.go`
- Modify: `internal/tui/snapshot_options_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/cli_test.go`

**Interfaces:**
- Change: `config.Load(home string) (Config, error)`.
- Keep: `snapshot.Build` and `snapshot.ConfigOnly`.
- Remove snapshot Reminder/KeepAlive runtime maps and ignored query/warning metadata.

- [ ] **Step 1: Simplify config loading**

Replace the invisible warning envelope with the config value already used after fallback:

```go
func Load(home string) (Config, error) {
	data, err := os.ReadFile(ConfigPath(home))
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if json.Unmarshal(data, &cfg) != nil || Validate(cfg) != nil {
		return Default(), nil
	}
	return cfg, nil
}
```

Remove `WarningCode`, `Warning`, `LoadResult`, and tests that assert warnings no interface displays. Keep malformed/invalid config fallback tests.

- [ ] **Step 2: Reduce snapshot request/result types**

Use these final shapes:

```go
type Request struct {
	Home  string
	Now   time.Time
	Limit int
	ID    string
}

type Result struct {
	GeneratedAt time.Time
	Config      config.Config
	ProjectsDir string
	EmptyState  EmptyState
	Sessions    []session.Session
	Selected    *session.Session
	Candidates  []session.Session
	Error       *Error
}

type Error struct {
	Code  string
	Query string
}
```

Remove `ReminderState`, `KeepAliveState`, `populateRuntime`, `QueryID`, `QueryLimit`, `ConfigWarnings`, and `Error.Message`.

- [ ] **Step 3: Remove the snapshot-to-reminder projection**

Delete `reminderEnabledFromSnapshot`. `OptionsFromSnapshot` should initialize with no active reminders:

```go
return Options{
	Now:                result.GeneratedAt,
	Dependencies:       input.Dependencies,
	Sessions:           sessions,
	SelectedID:         selectedID,
	AmbiguousID:        ambiguousID,
	ReminderThresholds: result.Config.ReminderThresholds,
	KeepAliveConfig:    result.Config.KeepAlive,
	Refresh:            refreshState,
	StartMode:          input.StartMode,
	StartDisplayTicker: true,
	StartRefreshTicker: input.StartMode != StartConfig,
	Config:             result.Config,
}
```

- [ ] **Step 4: Run focused verification and commit**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./internal/config ./internal/snapshot ./internal/tui ./internal/app
```

Expected: PASS; `--remind` remains rejected and config mode still skips session discovery.

```bash
git add internal/config internal/snapshot internal/tui/snapshot_options.go internal/tui/snapshot_options_test.go internal/app
git commit -m "Remove stale snapshot runtime metadata"
```

Expected net: 130-180 lines removed.

### Task 4: Delete Test-Only TUI Telemetry

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/list.go`
- Modify: `internal/tui/config_editor.go`
- Modify: `internal/tui/route_actions.go`
- Modify: `internal/tui/update_test.go`
- Modify: `internal/tui/update_config_test.go`
- Modify: `internal/tui/update_keepalive_test.go`
- Modify: `internal/tui/update_refresh_test.go`
- Modify: `internal/app/cli_test.go`

**Interfaces:**
- Remove `lastAction` and test-only getters omitted from the installed binary.
- Remove the legacy `WatcherEventMsg` path.
- Store only the latest notification status because the UI reads only index zero.

- [ ] **Step 1: Delete action telemetry**

Remove `Model.lastAction`, `LastAction()`, and every assignment to `m.lastAction`. Replace tests with assertions against one of: route, notice text, KeepAlive state, config persistence, returned command, or rendered view.

- [ ] **Step 2: Delete test-only model observers**

Remove getters used only by tests: `Route`, `Sessions`, `SessionStatuses`, `Countdown`, `WatcherEvents`, `NotificationStatuses`, `RefreshGeneration`, `LastRefreshSource`, `LastRefreshBypassedDebounce`, `RefreshDebounceToken`, and `ReminderEnabled`. Tests in package `tui` should read the corresponding unexported fields directly; app tests should assert `View()` or captured `tui.Options`.

- [ ] **Step 3: Delete the unwired watcher message**

Remove:

```go
type WatcherEventMsg struct {
	Path string
	Op   string
}
```

Also remove `Model.watcherEvents`, its `Update` branch, and the unreachable fallback rendering that reports watcher-event counts.

- [ ] **Step 4: Retain only the latest notification**

Replace the five-entry slice with:

```go
type Model struct {
	// existing fields
	lastNotification *NotificationStatus
}
```

`applyNotificationResult` assigns one copied status, and `listDegradedBanner` reads that pointer.

- [ ] **Step 5: Verify and commit**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./internal/tui ./internal/app
```

Expected: PASS with assertions against behavior rather than action strings.

```bash
git add internal/tui internal/app/cli_test.go
git commit -m "Remove test-only TUI telemetry"
```

Expected net: 140-200 lines removed.

### Task 5: Shrink Refresh Plumbing Without Removing fsnotify

**Files:**
- Modify: `internal/refresh/watcher.go`
- Modify: `internal/refresh/refresh.go`
- Modify: `internal/refresh/refresh_test.go`
- Modify: `internal/tui/live_refresh.go`
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/update_refresh_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/cli_test.go`

**Interfaces:**
- Keep fsnotify and dynamic watches for newly created directories.
- Replace normalized event payloads with one changed signal.
- Remove refresh source/timing diagnostics that production ignores.

- [ ] **Step 1: Reduce watcher output**

Use:

```go
type WatcherResult struct {
	Changed bool
	State   State
	Err     error
	Closed  bool
}
```

Delete `EventKind`, `NormalizedEvent`, and `normalizeKind`. When `Watcher.Next` receives an event, add a watch if it is a newly created directory, then return `Changed: true`.

- [ ] **Step 2: Reduce the coordinator**

Use:

```go
type Decision struct {
	ShouldRefresh bool
	Generation    int
	DebounceToken int
}

type Coordinator struct {
	pendingDebounce   bool
	debounceToken     int
	currentGeneration int
}

func NewCoordinator(initialGeneration int) *Coordinator
func (c *Coordinator) OnWatcherEvent() int
func (c *Coordinator) OnDebounceElapsed(token int) Decision
func (c *Coordinator) Refresh() Decision
```

Delete coordinator debounce duration, safety interval, clock, `Source`, `BypassedDebounce`, and `PendingDebounceCount`. Bubble Tea still owns the debounce and safety timers.

- [ ] **Step 3: Reduce TUI refresh dependencies/messages**

Change the callback to:

```go
RefreshSnapshot func(selected *session.Session) RefreshSnapshot
```

The TUI owns generation ordering around the callback. Replace `RefreshWatcherEventsMsg` with:

```go
type RefreshWatcherChangedMsg struct {
	State refresh.State
}
```

Manual and safety refreshes both call `Coordinator.Refresh`; watcher changes call `OnWatcherEvent` and schedule the existing debounce command.

- [ ] **Step 4: Preserve focused refresh behavior**

Keep the existing rule in `app.tuiDependencies`: Workspace refresh parses only the selected JSONL path; List refresh discovers and parses the configured recent-session limit.

- [ ] **Step 5: Verify and commit**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./internal/refresh ./internal/tui ./internal/app
```

Expected: PASS for debounce coalescing, stale-generation rejection, watcher degradation, manual refresh, safety refresh, and selected-only refresh.

```bash
git add internal/refresh internal/tui internal/app
git commit -m "Shrink live refresh plumbing"
```

Expected net: 120-180 lines removed with no refresh-latency change.

### Task 6: Keep One TUI Launch Seam

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/cli_test.go`

**Interfaces:**
- Remove `Dependencies.StartTUI`.
- Keep `Dependencies.RunTUIProgram func(tui.Options) error`.

- [ ] **Step 1: Remove the bypass seam**

Delete `StartTUI` from `Dependencies` and combine dispatch:

```go
case ModeConfig, ModeTUI:
	deps = fillDependencies(deps)
	if err := runTUI(cmd, deps); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
```

`runTUI` continues to call `RunTUIProgram(options)` when injected, otherwise it starts Bubble Tea.

- [ ] **Step 2: Consolidate dispatch tests**

Use one table test that captures `tui.Options` through `RunTUIProgram`. Assert default, `--n`, `--id`, and config start mode through the assembled options rather than a captured `Command` bypass.

- [ ] **Step 3: Verify and commit**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./internal/app
```

Expected: PASS for all CLI modes, TUI option assembly, watcher cleanup, and config startup.

```bash
git add internal/app
git commit -m "Collapse duplicate TUI launch seams"
```

Expected net: 70-110 lines removed.

### Task 7: Remove Unconsumed Domain Diagnostics

**Files:**
- Modify: `internal/session/model.go`
- Modify: `internal/session/parser.go`
- Modify: `internal/session/parser_test.go`
- Modify: `internal/snapshot/snapshot.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/list.go`
- Modify: `internal/tui/render_test.go`
- Modify: `internal/keepalive/model.go`
- Modify: `internal/keepalive/state.go`
- Modify: `internal/keepalive/timing.go`
- Modify: `internal/keepalive/confirm.go`
- Modify: `internal/keepalive/keepalive_test.go`
- Modify: `internal/notify/notifier.go`
- Modify: `internal/notify/notifier_test.go`
- Modify: `tools/ui-demo/main.go`

**Interfaces:**
- Preserve malformed JSON/timestamp handling and the visible warning count.
- Preserve every KeepAlive send guard, instance token, limit, timeout, confirmation, and fallback display.
- Remove test-only diagnostic fields and aliases.

- [ ] **Step 1: Reduce session parser output**

Use:

```go
type CacheWindow struct {
	Tier       CacheTier
	Label      string
	TTLSeconds int
	Known      bool
}

type MessageWindow struct {
	At      time.Time
	Excerpt string
}

type Session struct {
	SessionID       string
	ShortID         string
	Project         string
	Cwd             string
	JSONLPath       string
	FileModifiedAt  time.Time
	CacheWindow     CacheWindow
	DurationSeconds *int
	LastMessageAt   *time.Time
	Messages        Messages
	RecentMessages  []MessageWindow
	TokenStats      TokenStats
	Gaps            []Gap
	ResetCount      int
	WarningCount    int
}
```

Remove `Evidence`, `StartedAt`, `EndedAt`, `MessageWindow.Role`, `WarningCode`, `ParseWarning`, and warning-detail cloning. Keep `warn` as a counter increment and continue returning real read errors.

- [ ] **Step 2: Reduce KeepAlive diagnostics**

Use:

```go
type TimingDecision struct {
	EffectiveCountdownSeconds int
	InsideTrigger             bool
	SendAllowed               bool
}
```

Remove `SessionState.SafetyDisabled`, `Manager.Dismiss`, `Manager.Refresh`, and `Manager.SetState`. Remove `Options.KeepAliveStates`; tests should create state through `Enable`, `CountdownElapsed`, runner results, and reset actions. Keep `ClaudeRunner` because it prevents real sends in tests.

Change fallback formatting to return the only consumed value:

```go
func ManualFallbackCommand(sessionID, message, dir string) string
```

Delete the test-only `ConfirmJSONL` wrapper; keep `ConfirmationTarget.Check` and offset-based confirmation.

- [ ] **Step 3: Remove notification attempt history**

Delete `Attempt`, `Manager.attempts`, `Attempts`, `record`, and `Event.CountdownSeconds`. Keep failure suppression and the returned `notify.Result`; this task does not alter notification behavior.

- [ ] **Step 4: Verify and commit**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./internal/session ./internal/snapshot ./internal/keepalive ./internal/notify ./internal/tui
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test -tags demo ./tools/ui-demo
```

Expected: PASS; malformed input still increments the visible warning count, and all KeepAlive safety tests still pass.

```bash
git add internal/session internal/snapshot internal/keepalive internal/notify internal/tui tools/ui-demo
git commit -m "Remove unconsumed runtime diagnostics"
```

Expected net: 160-240 lines removed.

### Task 8: Full Verification and Reduction Accounting

**Files:**
- Modify only when current behavior documentation became inaccurate: `README.md`, `AGENTS.md`, `docs/decisions.md`.

**Interfaces:**
- No behavior changes beyond removal of the retired root Python entry point.

- [ ] **Step 1: Run the complete verification set**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go build ./...
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go vet ./...
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test ./...
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go test -tags demo ./...
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false scripts/test-install.sh
```

Expected: every command exits zero.

- [ ] **Step 2: Run read-only CLI smoke checks**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go run ./cmd/cc-watch --help
GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go run ./cmd/cc-watch statusline --help
HOME="$(mktemp -d /private/tmp/cc-watch-home.XXXXXX)" GOCACHE=/private/tmp/cc-watch-gocache GOFLAGS=-buildvcs=false go run ./cmd/cc-watch statusline --check
```

Expected: help is readable; `--check` reports an unconfigured statusline and writes nothing.

- [ ] **Step 3: Recalculate LOC**

Run:

```bash
runtime=$(rg --files cmd internal -g '*.go' -g '!*_test.go' | xargs wc -l | tail -1 | awk '{print $1}')
tests=$(rg --files cmd internal -g '*_test.go' | xargs wc -l | tail -1 | awk '{print $1}')
demo=$(rg --files tools -g '*.go' | xargs wc -l | tail -1 | awk '{print $1}')
printf 'runtime=%s tests=%s demo=%s\n' "$runtime" "$tests" "$demo"
```

Expected: total source falls by 900-1,200 lines from the audited 16,537-line baseline; direct dependencies fall by one.

- [ ] **Step 4: Review the final diff**

Run:

```bash
git diff --check
git status --short
git log --oneline -8
```

Expected: no whitespace errors; only intended files changed; each task has its own passing commit.

## Deferred Findings

- Do not delete `tools/ui-demo` in this plan. It is a real development workflow and was explicitly retained by the previous slim-down.
- Do not collapse responsive Workspace, rewind-window, gap-sort, or scrolling behavior without a separate product decision; those capabilities are surfaced.
- Do not delete fsnotify. The existing 30-second safety ticker could replace it, but that changes freshness and deserves a separate decision.
- Do not flatten `internal/snapshot` into `internal/app` yet. First remove its stale fields; reassess its remaining size after this plan.
- Do not remove notification failure suppression, KeepAlive logs, subprocess guards, or statusline settings backups.

## Self-Audit

- Spec coverage: includes the independent audit's high-confidence cuts and the first audit's verified test-only surfaces.
- Scope control: excludes all debatable user-visible and developer-workflow cuts.
- Placeholder scan: contains no deferred implementation placeholders; every task names exact files, final interfaces, commands, and expected results.
- Type consistency: statusline dependency signatures, config load signature, snapshot result shape, refresh messages, and session fields are updated in the tasks that own all callers.
- Safety: all real-send prevention, timeouts, confirmation, backup, and manual-review boundaries remain explicit.

## Expected Net

- Conservative code/test reduction: 900-1,200 lines.
- Direct dependency reduction: 1 (`github.com/charmbracelet/bubbles`).
- Product capability reduction: none, except deleting the retired root Python entry point already replaced by the Go binary.
