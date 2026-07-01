# cc-watch v2 Phase 11.9 Architecture Deepening Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not use parallel implementation subagents for this phase; the refactors touch overlapping runtime seams.

**Goal:** Deepen the architecture around cc-watch's current product reality: macOS local TUI, automatic live refresh, stable JSON API, Reminder alarms, bounded KeepAlive, and simple local install.

**Architecture:** Keep Go as the implementation stack. Make product-core behavior live behind deep modules: Refresh Runtime owns filesystem events, debounce, safety refresh, and generation decisions; KeepAlive Runtime owns send/confirmation policy; Route Modules own route-local focus/render/action behavior. Remove shallow or out-of-scope modules, especially duplicate Reminder state and Linux/unsupported notification adapters.

**Tech Stack:** Go 1.23+, Bubble Tea, lipgloss, fsnotify, standard-library JSON/config/subprocess handling, macOS `osascript` notifications. No new dependencies.

---

## Product Reality

Source of truth: `docs/superpowers/specs/2026-06-18-cc-watch-v2-product-reality.md`.

In scope:

- list recent Claude sessions and their Cache Status;
- open a Session Workspace with Session Info;
- live-refresh the running TUI from filesystem changes;
- keep manual refresh and safety refresh as fallbacks;
- expose stable `--json` as the JSON API;
- provide per-session Reminder through native macOS notifications;
- provide per-session KeepAlive with manual send and Auto-send;
- provide Config Editor defaults;
- support simple local macOS install later.

Out of scope:

- public watch mode;
- Linux/Windows notification support;
- Homebrew, GitHub Release, goreleaser publishing;
- native macOS app;
- background daemon;
- real Claude KeepAlive sends during verification.

## Hard Constraints

- Preserve public CLI flags and exit behavior.
- Preserve JSON `schema_version` and existing field names.
- Preserve config schema.
- Preserve parser semantics and fixture behavior.
- Preserve KeepAlive safety: no hidden loop, bounded scope, cancellation, evidence confirmation.
- Do not run a real Claude KeepAlive send.
- Do not replace `$HOME/.local/bin/cc-watch`.
- Do not start Phase 12 packaging or installer work.
- Do not add dependencies.
- Keep live refresh. Do not delete fsnotify/watcher/coordinator behavior.

## File Structure

### Create

- `internal/tui/live_refresh.go` - Bubble Tea adapter from Refresh Runtime events to TUI messages and commands.
- `internal/tui/reminder_runtime.go` - TUI-local Reminder Runtime if the standalone package is deleted.
- `internal/tui/reminder_runtime_test.go` - focused Reminder Runtime coverage moved from `internal/reminder`.
- `internal/tui/route_actions.go` - route-local action dispatch helpers extracted from `update.go`.
- `internal/tui/workspace_keepalive.go` - Workspace KeepAlive action/render helpers extracted from `workspace.go` and `update.go`.
- `internal/tui/update_refresh_test.go` - refresh/runtime TUI tests split out of `update_test.go`.
- `internal/tui/update_keepalive_test.go` - KeepAlive TUI tests split out of `update_test.go`.
- `internal/tui/update_config_test.go` - Config route TUI tests split out of `update_test.go`.

### Modify

- `internal/refresh/watcher.go` - remove Bubble Tea import; return refresh-domain results instead of `tea.Msg`.
- `internal/refresh/refresh.go` - keep coordinator as the Refresh Runtime decision module; add only the small methods needed by the TUI adapter.
- `internal/refresh/refresh_test.go` - keep watcher/coordinator tests and add production-shape runtime tests.
- `internal/app/app.go` - wire live Refresh Runtime into TUI startup for List and Workspace; keep JSON/Config non-live.
- `internal/app/cli_test.go` - assert live refresh startup wiring and no public watch command or flag.
- `internal/tui/model.go` - hold Refresh Runtime adapter state and slimmer route/runtime fields.
- `internal/tui/messages.go` - add refresh debounce message types and remove direct refresh-domain message leakage.
- `internal/tui/update.go` - route refresh and KeepAlive work through deeper modules; shrink central branching.
- `internal/tui/workspace.go` - keep Workspace rendering, move KeepAlive-specific branching to `workspace_keepalive.go`.
- `internal/tui/config_editor.go` - no product behavior change; tests move out of giant files.
- `internal/jsonout/json.go` - replace `internal/reminder` event dependency with a JSON-local type.
- `internal/jsonout/json_test.go` - update Reminder event fixture type.
- `internal/notify/notifier.go` - collapse to macOS command notifier plus manager.
- `internal/notify/notifier_test.go` - remove Linux/unsupported-platform expectations; keep macOS escaping, failure suppression, wording.
- `CONTEXT.md` - add terms that become canonical during the refactor.
- `docs/superpowers/progress/cc-watch-v2-progress.md` - update phase status and verification ledger.

### Delete

- `internal/reminder/reminder.go`
- `internal/reminder/reminder_test.go`
- `internal/notify/command.go`
- `internal/notify/macos.go`
- `internal/notify/linux.go`
- `internal/notify/noop.go`

## Gate A: Baseline And Product Reality

### Task A1: Confirm branch, docs, and Go stack

**Files:**
- Modify: none

- [x] **Step 1: Confirm worktree and branch**

Run:

```bash
git status --short --branch
```

Expected:

```text
## codex/phase-11.8-architecture-refactor
```

The worktree may contain the pre-11.9 documentation alignment files. Do not revert them.

- [x] **Step 2: Read required docs**

Run:

```bash
cat AGENTS.md
cat docs/superpowers/runbooks/2026-06-03-cc-watch-v2-implementation-runbook.md
cat docs/superpowers/progress/cc-watch-v2-progress.md
cat docs/superpowers/plans/cc-watch-v2/PLAN.md
cat docs/superpowers/specs/2026-06-18-cc-watch-v2-product-reality.md
cat docs/superpowers/plans/cc-watch-v2/phase-11.9-architecture-simplification.md
```

Expected: no instruction says to replace `$HOME/.local/bin/cc-watch`, publish releases, delete live refresh, or run a real Claude send.

- [x] **Step 3: Record Go-vs-TypeScript decision in `CONTEXT.md`**

Append this term if it is not already present:

```markdown
### Implementation Stack

The current implementation stack is Go because cc-watch is a local macOS terminal binary with filesystem watching, native notification command execution, bounded subprocess control, stable JSON output, and simple local install. TypeScript is out of scope unless the product becomes Node-native, web-native, or editor-extension-native.
```

Run:

```bash
wc -l CONTEXT.md
```

Expected: `CONTEXT.md` remains under 100 lines.

## Gate B: Refresh Runtime Production Wiring

### Task B1: Remove Bubble Tea from `internal/refresh`

**Files:**
- Modify: `internal/refresh/watcher.go`
- Modify: `internal/refresh/refresh_test.go`
- Modify: `internal/tui/messages.go`
- Create: `internal/tui/live_refresh.go`
- Test: `internal/refresh/refresh_test.go`
- Test: `internal/tui/update_refresh_test.go`

- [x] **Step 1: Write failing refresh-domain test**

Add this test to `internal/refresh/refresh_test.go`:

```go
func TestWatcherNextReturnsDomainResultWithoutTeaDependency(t *testing.T) {
	events := make(chan RawEvent, 1)
	errors := make(chan error, 1)
	projectsDir := t.TempDir()
	fs := &fakeWatchFS{
		dirs:   []string{projectsDir},
		events: events,
		errors: errors,
	}
	watcher, err := NewWatcher(projectsDir, fs)
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	events <- RawEvent{Path: projectsDir + "/session.jsonl", Op: OpWrite}

	result := watcher.Next()
	if len(result.Events) != 1 {
		t.Fatalf("len(result.Events) = %d, want 1: %#v", len(result.Events), result)
	}
	if result.Events[0].Kind != EventWritten {
		t.Fatalf("event kind = %q, want %q", result.Events[0].Kind, EventWritten)
	}
	if result.State.Status != StatusOK {
		t.Fatalf("state = %#v, want ok", result.State)
	}
}
```

Run:

```bash
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/refresh -run TestWatcherNextReturnsDomainResultWithoutTeaDependency
```

Expected: fail with `watcher.Next undefined`.

- [x] **Step 2: Implement `WatcherResult` and `Next`**

In `internal/refresh/watcher.go`, remove the Bubble Tea import and replace `NextMessage` with:

```go
type WatcherResult struct {
	Events []NormalizedEvent
	State  State
	Err    error
	Closed bool
}

var ErrWatcherClosed = errors.New("refresh watcher closed")

func (w *Watcher) Next() WatcherResult {
	select {
	case event, ok := <-w.fs.Events():
		if !ok {
			w.MarkRuntimeError(ErrWatcherClosed)
			return WatcherResult{State: w.State(), Err: ErrWatcherClosed, Closed: true}
		}
		return WatcherResult{
			Events: w.Normalize([]RawEvent{event}),
			State:  w.State(),
		}
	case err, ok := <-w.fs.Errors():
		if !ok {
			w.MarkRuntimeError(ErrWatcherClosed)
			return WatcherResult{State: w.State(), Err: ErrWatcherClosed, Closed: true}
		}
		w.MarkRuntimeError(err)
		return WatcherResult{State: w.State(), Err: err}
	}
}
```

Keep `WatcherEventsMsg` and `WatcherDegradedMsg` out of `internal/refresh`; those are TUI messages.
Import `errors` in `watcher.go`.

Run:

```bash
gofmt -w internal/refresh/watcher.go internal/refresh/refresh_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/refresh
```

Expected: fail in tests that still refer to `NextMessage`, `WatcherEventsMsg`, or `WatcherDegradedMsg`.

- [x] **Step 3: Prove closed watcher channels do not spin**

Add a focused test to `internal/refresh/refresh_test.go` using a fake `WatchFS` whose `Events()` and `Errors()` channels are already closed. Assert:

```go
result := watcher.Next()
if !result.Closed {
	t.Fatal("Closed = false, want true")
}
if !errors.Is(result.Err, ErrWatcherClosed) {
	t.Fatalf("Err = %v, want ErrWatcherClosed", result.Err)
}
if result.State.Status != StatusDegraded {
	t.Fatalf("status = %q, want degraded", result.State.Status)
}
```

Run:

```bash
gofmt -w internal/refresh/refresh_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/refresh -run TestWatcherNextClosedChannelsReturnClosedResult
```

Expected: pass.

- [x] **Step 4: Move watcher message types to TUI**

In `internal/tui/messages.go`, keep TUI-local message types:

```go
type WatcherEventMsg struct {
	Path string
	Op   string
}

type RefreshWatcherEventsMsg struct {
	Events []refresh.NormalizedEvent
	State  refresh.State
}

type RefreshWatcherDegradedMsg struct {
	State refresh.State
}

type RefreshWatcherClosedMsg struct {
	State refresh.State
}

type RefreshDebounceElapsedMsg struct {
	Now time.Time
}
```

Update `internal/refresh/refresh_test.go` to assert `WatcherResult` instead of Bubble Tea messages.

Run:

```bash
gofmt -w internal/tui/messages.go internal/refresh/refresh_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/refresh
```

Expected: pass.

### Task B2: Make TUI use `refresh.Coordinator` for debounce, safety, and generation

**Files:**
- Create: `internal/tui/live_refresh.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/messages.go`
- Create: `internal/tui/update_refresh_test.go`

- [x] **Step 1: Write failing TUI debounce test**

Create `internal/tui/update_refresh_test.go` with:

```go
package tui

import (
	"testing"
	"time"

	"github.com/richardchen/cc-watch/internal/refresh"
	"github.com/richardchen/cc-watch/internal/session"
)

func TestWatcherEventsDebounceBeforeRefreshSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	calls := 0
	model := NewModel(Options{
		Now: now,
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				calls++
				if source != refresh.SourceFsnotify {
					t.Fatalf("source = %q, want fsnotify", source)
				}
				return RefreshSnapshot{Sessions: []session.Session{{SessionID: "after-event", ShortID: "after"}}}
			},
		},
	})

	updated, cmd := model.Update(RefreshWatcherEventsMsg{
		Events: []refresh.NormalizedEvent{{Kind: refresh.EventWritten, Path: "/tmp/session.jsonl"}},
		State:  refresh.State{Status: refresh.StatusOK, SafetyRefreshActive: true},
	})
	model = updated.(Model)
	if calls != 0 {
		t.Fatalf("watcher event called RefreshSnapshot immediately; calls = %d", calls)
	}
	if cmd == nil {
		t.Fatal("watcher event returned nil debounce command")
	}

	updated, cmd = model.Update(RefreshDebounceElapsedMsg{Now: now.Add(300 * time.Millisecond)})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("debounce elapsed returned nil refresh command")
	}
	msg := cmd()
	result, ok := msg.(RefreshResultMsg)
	if !ok {
		t.Fatalf("debounce command returned %#v, want RefreshResultMsg", msg)
	}
	updated, _ = model.Update(result)
	model = updated.(Model)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if model.LastRefreshSource() != refresh.SourceFsnotify {
		t.Fatalf("LastRefreshSource = %q, want fsnotify", model.LastRefreshSource())
	}
}
```

Run:

```bash
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run TestWatcherEventsDebounceBeforeRefreshSnapshot
```

Expected: fail because `RefreshWatcherEventsMsg` and `RefreshDebounceElapsedMsg` are not handled.

- [x] **Step 2: Add Refresh Runtime state to `Model`**

In `internal/tui/model.go`, add:

```go
type RefreshTiming struct {
	Debounce       time.Duration
	SafetyInterval time.Duration
}
```

Add to `Options`:

```go
RefreshTiming RefreshTiming
```

Add to `Model`:

```go
refreshCoordinator *refresh.Coordinator
refreshTiming      RefreshTiming
```

In `NewModel`, initialize:

```go
refreshTiming := options.RefreshTiming
if refreshTiming.Debounce <= 0 {
	refreshTiming.Debounce = 300 * time.Millisecond
}
if refreshTiming.SafetyInterval <= 0 {
	refreshTiming.SafetyInterval = 30 * time.Second
}
refreshCoordinator := refresh.NewCoordinator(refresh.Options{
	Debounce:        refreshTiming.Debounce,
	SafetyInterval:  refreshTiming.SafetyInterval,
	InitialNow:      now,
	InitialSessions: sessions,
})
```

Store both on `model`.

Run:

```bash
gofmt -w internal/tui/model.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run TestWatcherEventsDebounceBeforeRefreshSnapshot
```

Expected: still fail because update handling is missing.

- [x] **Step 3: Route refresh decisions through coordinator**

In `internal/tui/update.go`, replace direct `scheduleRefresh(source, bypassedDebounce)` calls with decision-based helpers:

```go
func (m Model) scheduleRefreshDecision(decision refresh.Decision) (tea.Model, tea.Cmd) {
	if !decision.ShouldRefresh {
		return m, nil
	}
	m.refreshGeneration = decision.Generation
	m.lastRefreshSource = decision.Source
	m.lastBypassedDebounce = decision.BypassedDebounce
	return m.refreshCommand(decision.Source, decision.Generation)
}

func (m Model) refreshCommand(source refresh.Source, generation int) (tea.Model, tea.Cmd) {
	refreshSnapshot := m.deps.RefreshSnapshot
	if refreshSnapshot == nil {
		return m, nil
	}
	selected := m.selectedSession()
	var selectedForRefresh *session.Session
	if m.route == RouteWorkspace && selected != nil {
		copied := *selected
		selectedForRefresh = &copied
	}
	return m, func() tea.Msg {
		snapshot := refreshSnapshot(source, generation, selectedForRefresh)
		return RefreshResultMsg{
			Generation:   generation,
			Sessions:     snapshot.Sessions,
			Refresh:      snapshot.Refresh,
			HasRefresh:   snapshot.HasRefresh,
			SelectedOnly: snapshot.SelectedOnly,
			SelectedID:   snapshot.SelectedID,
		}
	}
}
```

Handle messages:

```go
case RefreshWatcherEventsMsg:
	m.refresh.Watcher = typed.State
	_ = m.refreshCoordinator.OnWatcherEvents(typed.Events)
	return m, refreshDebounceCommand(m.refreshTiming.Debounce)
case RefreshWatcherDegradedMsg:
	m.refresh.Watcher = typed.State
	return m, nil
case RefreshDebounceElapsedMsg:
	decision := m.refreshCoordinator.OnDebounceElapsed(typed.Now)
	return m.scheduleRefreshDecision(decision)
case RefreshTickMsg:
	m.now = typed.Now
	decision := m.refreshCoordinator.OnSafetyTick(typed.Now)
	updated, cmd := m.scheduleRefreshDecision(decision)
	m = updated.(Model)
	if m.startRefreshTicker {
		return m, tea.Batch(cmd, refreshTickCommand(m.refreshTiming.SafetyInterval))
	}
	return m, cmd
case ManualRefreshMsg:
	// keep existing notification suppression and notice code
	decision := m.refreshCoordinator.OnManualRefresh()
	return m.scheduleRefreshDecision(decision)
```

Add:

```go
func refreshDebounceCommand(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return RefreshDebounceElapsedMsg{Now: t.UTC()}
	})
}

func refreshTickCommand(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return RefreshTickMsg{Now: t.UTC()}
	})
}
```

Remove the old zero-argument `refreshTickCommand`.

Run:

```bash
gofmt -w internal/tui/update.go internal/tui/messages.go internal/tui/live_refresh.go internal/tui/update_refresh_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'TestWatcherEventsDebounceBeforeRefreshSnapshot|TestRefresh'
```

Expected: pass after adjusting old tests to call the interval-aware `refreshTickCommand` through `Model.Init`.

### Task B3: Wire live watcher into app startup

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/cli_test.go`
- Create: `internal/tui/live_refresh.go`

- [x] **Step 1: Write failing app wiring test**

Add to `internal/app/cli_test.go`:

```go
func TestTUIOptionsStartLiveRefreshForListAndWorkspaceOnly(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	deps := fakeDeps(t)
	deps.Now = func() time.Time { return now }
	deps.NewLiveWatcher = func(projectsDir string) (tui.Watcher, func() error, error) {
		if projectsDir != "/tmp/home/.claude/projects" {
			t.Fatalf("projectsDir = %q, want fixture projects dir", projectsDir)
		}
		return fakeLiveWatcher{}, func() error { return nil }, nil
	}
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{
			ProjectsDir: "/tmp/home/.claude/projects",
			Sessions: []session.SessionFile{{
				SessionID: "session-id",
				ShortID:   "session",
				Project:   "tmp",
				Path:      "/tmp/home/.claude/projects/-tmp/session.jsonl",
				ModTime:   now,
			}},
		}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		return session.Session{SessionID: "session-id", ShortID: "session", JSONLPath: path, Project: "tmp"}, nil
	}

	list, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions list returned error: %v", err)
	}
if list.LiveRefresh == nil {
	t.Fatal("list LiveRefresh = nil, want watcher command")
}
if list.CloseLiveRefresh == nil {
	t.Fatal("list CloseLiveRefresh = nil, want watcher cleanup")
}

	workspace, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5, ID: "session"}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions workspace returned error: %v", err)
	}
if workspace.LiveRefresh == nil {
	t.Fatal("workspace LiveRefresh = nil, want watcher command")
}
if workspace.CloseLiveRefresh == nil {
	t.Fatal("workspace CloseLiveRefresh = nil, want watcher cleanup")
}

	configOptions, err := buildTUIOptions(Command{Mode: ModeConfig, Limit: 5}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions config returned error: %v", err)
	}
if configOptions.LiveRefresh != nil {
	t.Fatal("config LiveRefresh != nil, want no watcher command")
}
if configOptions.CloseLiveRefresh != nil {
	t.Fatal("config CloseLiveRefresh != nil, want no watcher cleanup")
}
}

type fakeLiveWatcher struct{}

func (fakeLiveWatcher) Next() refresh.WatcherResult {
	return refresh.WatcherResult{State: refresh.State{Status: refresh.StatusOK, SafetyRefreshActive: true}}
}
```

Run:

```bash
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/app -run TestTUIOptionsStartLiveRefreshForListAndWorkspaceOnly
```

Expected: fail because `Dependencies.NewLiveWatcher`, `tui.Options.LiveRefresh`, and `tui.Options.CloseLiveRefresh` do not exist.

- [x] **Step 2: Add live refresh command option**

In `internal/tui/model.go`, add to `Options`:

```go
LiveRefresh tea.Cmd
CloseLiveRefresh func() error
```

Add to `Model`:

```go
liveRefresh tea.Cmd
```

Store only `LiveRefresh` in `NewModel`, and in `Init`:

```go
if m.liveRefresh != nil {
	commands = append(commands, m.liveRefresh)
}
```

Run:

```bash
gofmt -w internal/tui/model.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/app -run TestTUIOptionsStartLiveRefreshForListAndWorkspaceOnly
```

Expected: still fail because app does not populate `LiveRefresh`.

- [x] **Step 3: Add TUI live refresh adapter**

Create `internal/tui/live_refresh.go`:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/refresh"
)

type Watcher interface {
	Next() refresh.WatcherResult
}

func LiveRefreshCommand(watcher Watcher) tea.Cmd {
	if watcher == nil {
		return nil
	}
	return func() tea.Msg {
		result := watcher.Next()
		if result.Closed {
			return RefreshWatcherClosedMsg{State: result.State}
		}
		if result.Err != nil {
			return RefreshWatcherDegradedMsg{State: result.State}
		}
		return RefreshWatcherEventsMsg{Events: result.Events, State: result.State}
	}
}
```

In `Update`, after handling `RefreshWatcherEventsMsg` and non-closed `RefreshWatcherDegradedMsg`, batch `m.liveRefresh` so the watcher keeps listening:

```go
return m, tea.Batch(refreshDebounceCommand(m.refreshTiming.Debounce), m.liveRefresh)
```

Handle `RefreshWatcherClosedMsg` by updating the watcher state and returning no `m.liveRefresh` command. This prevents a closed channel from producing an immediate infinite message loop.

Run:

```bash
gofmt -w internal/tui/live_refresh.go internal/tui/update.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'TestWatcherEventsDebounceBeforeRefreshSnapshot|Test.*Init'
```

Expected: pass after tests account for non-nil live refresh command where configured.

- [x] **Step 4: Build live watcher in app startup**

In `internal/app/app.go`, add to `Dependencies`:

```go
NewLiveWatcher func(projectsDir string) (tui.Watcher, func() error, error)
RunTUIProgram  func(tui.Options) error
```

Add this default helper:

```go
func defaultLiveWatcher(projectsDir string) (tui.Watcher, func() error, error) {
	fs, err := refresh.NewFSNotifyFS()
	if err != nil {
		return nil, nil, err
	}
	watcher, err := refresh.NewWatcher(projectsDir, fs)
	if err != nil {
		_ = fs.Close()
		return nil, nil, err
	}
	return watcher, fs.Close, nil
}
```

Set this default in `fillDependencies` when `NewLiveWatcher` is nil.

Inside `buildTUIOptionsFromSnapshot`, build the `tui.Options` value as a local named `options`, then create a watcher command for non-config modes before returning:

```go
options := tui.Options{
	Now:                result.GeneratedAt,
	Dependencies:       tuiDependencies(cmd, deps, home),
	Sessions:           sessions,
	SelectedID:         selectedID,
	AmbiguousID:        ambiguousID,
	ReminderEnabled:    tuiReminderEnabled(result.Reminder),
	ReminderThresholds: result.Config.ReminderThresholds,
	KeepAliveConfig:    result.Config.KeepAlive,
	Refresh:            refreshState,
	StartMode:          startMode,
	StartDisplayTicker: true,
	StartRefreshTicker: startMode != tui.StartConfig,
	Config:             result.Config,
}
var liveRefresh tea.Cmd
if startMode != tui.StartConfig && result.ProjectsDir != "" {
	watcher, closer, err := deps.NewLiveWatcher(result.ProjectsDir)
	if err != nil {
		options.Refresh.Watcher = refresh.State{
			Status:              refresh.StatusDegraded,
			Messages:            []string{err.Error()},
			SafetyRefreshActive: true,
		}
	} else {
		liveRefresh = tui.LiveRefreshCommand(watcher)
		options.CloseLiveRefresh = closer
	}
}
options.LiveRefresh = liveRefresh
return options
```

This factory shape lets tests use a fake watcher, preserves the watcher closer, and ensures default setup closes the fsnotify adapter when watcher creation fails.

In `runTUI`, defer the closer after options are built and before running the Bubble Tea program:

```go
options, err := buildTUIOptions(cmd, deps)
if err != nil {
	return err
}
if options.CloseLiveRefresh != nil {
	defer func() { _ = options.CloseLiveRefresh() }()
}
if deps.RunTUIProgram != nil {
	return deps.RunTUIProgram(options)
}
model := tui.NewModel(options)
_, err = tea.NewProgram(model).Run()
return err
```

Add an app test with a fake `NewLiveWatcher` closer and a fake `RunTUIProgram` that returns nil. Assert the closer is called after `runTUI` returns. This test proves lifecycle ownership in app startup rather than only proving the option field exists.

Run:

```bash
gofmt -w internal/app/app.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/app -run 'TestTUIOptionsStartLiveRefreshForListAndWorkspaceOnly|TestRunTUIClosesLiveRefresh|Test.*Watch'
```

Expected: pass. Retired watch flags remain rejected or absent as public CLI modes.

## Gate C: KeepAlive Runtime Safety Locality

### Task C1: Use KeepAlive Runtime for runner result policy

**Files:**
- Modify: `internal/keepalive/runner.go`
- Modify: `internal/tui/update.go`
- Create: `internal/tui/update_keepalive_test.go`
- Test: `internal/keepalive/keepalive_test.go`

- [x] **Step 1: Write failing production-seam test**

Add to `internal/tui/update_keepalive_test.go`:

```go
func TestWorkspaceKeepAliveRunnerUsesRuntimePolicy(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	manager := keepalive.NewManager(cfg)
	s := workspaceSession(now.Add(-55 * time.Minute))
	manager.SetState(keepalive.SessionState{
		SessionID:     s.SessionID,
		State:         keepalive.StateManualReady,
		AutoSend:      false,
		MaxSends:      1,
		InstanceToken: 7,
	})
	model := NewModel(Options{
		Now:              now,
		Sessions:         []session.Session{s},
		SelectedID:       s.SessionID,
		KeepAliveConfig:  cfg,
		KeepAliveManager: manager,
		Dependencies: Dependencies{
			KeepAliveRunner: fakeKeepAliveRunner{startedAt: now.Add(time.Second)},
			ConfirmKeepAlive: func(context.Context, keepalive.ConfirmationTarget) (keepalive.ConfirmationResult, error) {
				return keepalive.ConfirmationResult{ConfirmedAt: now.Add(2 * time.Second)}, nil
			},
		},
	})

	updated, cmd := model.sendKeepAliveNow()
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("sendKeepAliveNow returned nil command")
	}
	runnerMsg := cmd().(KeepAliveRunnerResultMsg)
	updated, cmd = model.Update(runnerMsg)
	model = updated.(Model)
	if got := model.KeepAliveState(s.SessionID).State; got != keepalive.StateConfirming {
		t.Fatalf("state after runner = %q, want confirming", got)
	}
}
```

Run:

```bash
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run TestWorkspaceKeepAliveRunnerUsesRuntimePolicy
```

Expected: pass before refactor. Keep it as a guard while changing production code to call `keepalive.Manager.Run`.

- [x] **Step 2: Split runner execution from Update-owned state mutation**

Do not call `keepalive.Manager.Run` from inside a Bubble Tea command. `Manager` owns map-backed TUI state and is not synchronized; all `Manager` mutations must remain inside the `Update` loop.

In `internal/keepalive/runner.go`, split the current `Manager.Run` behavior into:

```go
type RunnerExecution struct {
	Result            RunResult
	ClaudeUnavailable bool
}

func ExecuteRunner(ctx context.Context, action Action, runner ClaudeRunner, now time.Time) RunnerExecution {
	if runner == nil {
		return RunnerExecution{
			Result:            RunResult{StartedAt: now, Err: ErrClaudeUnavailable},
			ClaudeUnavailable: true,
		}
	}
	if err := runner.Available(); err != nil {
		return RunnerExecution{
			Result:            RunResult{StartedAt: now, Err: err},
			ClaudeUnavailable: true,
		}
	}
	result := runner.Send(ctx, RunRequest{SessionID: action.SessionID, Message: action.Message})
	if result.StartedAt.IsZero() {
		result.StartedAt = now
	}
	if result.Limit && result.Err == nil {
		result.Err = ErrClaudeLimit
	}
	return RunnerExecution{Result: result}
}
```

Add a manager method that applies runtime policy synchronously in `Update`:

```go
func (m *Manager) ApplyRunnerExecution(action Action, execution RunnerExecution) SessionState {
	state := m.State(action.SessionID)
	if action.Kind != ActionStartRunner || state.State != StateSending || state.InstanceToken != action.InstanceToken {
		return state
	}
	if execution.ClaudeUnavailable {
		m.MarkNoClaude(action.SessionID, action.InstanceToken, execution.Result.Err.Error())
		return m.State(action.SessionID)
	}
	result := execution.Result
	if result.Err != nil || result.ExitCode != 0 || result.Limit {
		m.MarkSubprocessFailure(action.SessionID, action.InstanceToken, failureMessage(result))
		return m.State(action.SessionID)
	}
	m.MarkSendStarted(action.SessionID, action.InstanceToken, result.StartedAt)
	return m.State(action.SessionID)
}
```

Keep `Manager.Run` only if existing tests or call sites still need it, but make it a thin compatibility wrapper around `ExecuteRunner` plus `ApplyRunnerExecution`. Do not use it from TUI commands.

In `internal/tui/update.go`, change `keepAliveRunnerCommand` so the command calls `keepalive.ExecuteRunner`, then builds `KeepAliveRunnerResultMsg` from the execution and the same confirmation target. Extend the message to carry the original `keepalive.Action` and `keepalive.RunnerExecution`.

Remove duplicate TUI-side checks for `result.ExitCode`, `result.Limit`, and subprocess failure classification where `ApplyRunnerExecution` now handles the state transition.

Then rewrite the `KeepAliveRunnerResultMsg` handler to treat the manager state as authoritative:

```go
case KeepAliveRunnerResultMsg:
	if !m.keepAliveAsyncCurrent(typed.SessionID, typed.InstanceToken, typed.Generation, typed.SelectedID) {
		m.lastAction = ErrKeepAliveStaleMessage.Error()
		return m, nil
	}
	state := m.keepAliveManager.ApplyRunnerExecution(typed.Action, typed.Execution)
	if state.State == keepalive.StateConfirming {
		return m, m.keepAliveConfirmationCommand(typed)
	}
	return m, nil
```

The handler must not call `MarkNoClaude`, `MarkSubprocessFailure`, or `MarkSendStarted`; `ApplyRunnerExecution` owns runner policy and state transition policy.

Run:

```bash
gofmt -w internal/tui/update.go internal/tui/update_keepalive_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/keepalive ./internal/tui -run 'TestWorkspaceKeepAliveRunnerUsesRuntimePolicy|TestRunner|TestConfirmation'
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test -race ./internal/keepalive ./internal/tui -run 'TestWorkspaceKeepAliveRunnerUsesRuntimePolicy|TestRunner'
```

Expected: pass.

### Task C2: Fix confirmation timeout to match ADR

**Files:**
- Modify: `internal/keepalive/confirm.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/keepalive/keepalive_test.go`
- Modify: `internal/tui/update_keepalive_test.go`

- [x] **Step 1: Add timeout constant**

In `internal/keepalive/confirm.go`, add:

```go
const ConfirmationTimeout = 20 * time.Second
```

Run:

```bash
gofmt -w internal/keepalive/confirm.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/keepalive -run TestConfirmation
```

Expected: pass.

- [x] **Step 2: Use timeout constant in TUI confirmation command**

In `internal/tui/update.go`, replace:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
```

with:

```go
ctx, cancel := context.WithTimeout(context.Background(), keepalive.ConfirmationTimeout)
```

Run:

```bash
gofmt -w internal/tui/update.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'TestWorkspaceKeepAliveActionsProduceRunnerAndConfirmationCommands|TestWorkspaceIgnoresStaleKeepAliveAsyncMessages'
```

Expected: pass.

## Gate D: Reminder And macOS Notification Cleanup

### Task D1: Remove standalone Reminder package

**Files:**
- Create: `internal/tui/reminder_runtime.go`
- Create: `internal/tui/reminder_runtime_test.go`
- Modify: `internal/jsonout/json.go`
- Modify: `internal/jsonout/json_test.go`
- Delete: `internal/reminder/reminder.go`
- Delete: `internal/reminder/reminder_test.go`

- [x] **Step 1: Move JSON Reminder event type into `jsonout`**

In `internal/jsonout/json.go`, remove the `internal/reminder` import. Add:

```go
type ReminderEvent struct {
	Kind             string
	SessionID        string
	ThresholdPercent int
	RemainingPercent float64
	OccurredAt       time.Time
}
```

Change `ReminderState.Fired` to:

```go
Fired []ReminderEvent
```

Update `internal/jsonout/json_test.go` to use `[]ReminderEvent`.

Run:

```bash
gofmt -w internal/jsonout/json.go internal/jsonout/json_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/jsonout
```

Expected: pass.

- [x] **Step 2: Create TUI Reminder Runtime from current TUI maps**

Create `internal/tui/reminder_runtime.go`:

```go
package tui

import (
	"time"

	"github.com/richardchen/cc-watch/internal/session"
)

type reminderRuntime struct {
	thresholds []int
	enabled    map[string]bool
	fired      map[string]map[int]bool
}

type reminderRuntimeEvent struct {
	sessionID        string
	thresholdPercent int
	remainingPercent float64
	occurredAt       time.Time
}

func newReminderRuntime(thresholds []int, enabled map[string]bool, fired map[string]map[int]bool) *reminderRuntime {
	return &reminderRuntime{
		thresholds: append([]int(nil), thresholds...),
		enabled:    cloneBoolMap(enabled),
		fired:      cloneReminderFired(fired),
	}
}

func (r *reminderRuntime) check(now time.Time, sessions []session.Session) []reminderRuntimeEvent {
	var events []reminderRuntimeEvent
	for _, s := range sessions {
		if !r.enabled[s.SessionID] {
			continue
		}
		status := s.StatusAt(now)
		if status.State != session.StatusActive || status.RemainingSeconds == nil || s.CacheWindow.TTLSeconds <= 0 {
			continue
		}
		remainingPercent := float64(*status.RemainingSeconds) / float64(s.CacheWindow.TTLSeconds) * 100
		for _, threshold := range r.thresholds {
			if remainingPercent > float64(threshold) || r.alreadyFired(s.SessionID, threshold) {
				continue
			}
			r.markFired(s.SessionID, threshold)
			events = append(events, reminderRuntimeEvent{
				sessionID:        s.SessionID,
				thresholdPercent: threshold,
				remainingPercent: remainingPercent,
				occurredAt:       now,
			})
		}
	}
	return events
}

func (r *reminderRuntime) alreadyFired(sessionID string, threshold int) bool {
	return r.fired[sessionID][threshold]
}

func (r *reminderRuntime) markFired(sessionID string, threshold int) {
	fired := r.fired[sessionID]
	if fired == nil {
		fired = map[int]bool{}
	}
	fired[threshold] = true
	r.fired[sessionID] = fired
}

func cloneReminderFired(src map[string]map[int]bool) map[string]map[int]bool {
	cloned := make(map[string]map[int]bool, len(src))
	for sessionID, thresholds := range src {
		cloned[sessionID] = make(map[int]bool, len(thresholds))
		for threshold, fired := range thresholds {
			cloned[sessionID][threshold] = fired
		}
	}
	return cloned
}
```

Add focused tests in `internal/tui/reminder_runtime_test.go` for enabled-only firing and once-per-threshold behavior.

Run:

```bash
gofmt -w internal/tui/reminder_runtime.go internal/tui/reminder_runtime_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run TestReminderRuntime
```

Expected: pass.

- [x] **Step 3: Route Reminder notification checks through runtime**

In `internal/tui/update.go`, change `reminderNotificationCommands` to create a runtime and translate events:

```go
runtime := newReminderRuntime(m.reminderThresholds, m.reminderEnabled, m.reminderFired)
events := runtime.check(m.now, m.sessions)
m.reminderFired = runtime.fired
for _, event := range events {
	commands = append(commands, m.notificationCommand(notify.Event{
		Kind:             notify.EventReminderThresholdCrossed,
		SessionID:        event.sessionID,
		ThresholdPercent: event.thresholdPercent,
	}))
}
```

Remove `reminderAlreadyFired` and `markReminderFired` if no callers remain.

Delete the obsolete `internal/reminder` files via patch.

Run:

```bash
rg -n 'internal/reminder|reminder\.' internal cmd --glob '*.go'
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/jsonout ./internal/tui
```

Expected: `rg` has no matches and tests pass.

### Task D2: Collapse notifications to macOS product scope

**Files:**
- Modify: `internal/notify/notifier.go`
- Modify: `internal/notify/notifier_test.go`
- Delete: `internal/notify/command.go`
- Delete: `internal/notify/macos.go`
- Delete: `internal/notify/linux.go`
- Delete: `internal/notify/noop.go`

- [x] **Step 1: Keep macOS command and command notifier in `notifier.go`**

Move these definitions into `internal/notify/notifier.go`:

```go
type CommandBuilder func(Notification) (string, []string)

type CommandNotifier struct {
	build  CommandBuilder
	runner Runner
}

func NewCommandNotifier(build CommandBuilder, runner Runner) CommandNotifier {
	return CommandNotifier{build: build, runner: runner}
}

func ExecRunner(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func MacOSCommand(notification Notification) (string, []string) {
	script := `display notification "` + escapeAppleScript(notification.Body) + `" with title "` + escapeAppleScript(notification.Title) + `"`
	return "osascript", []string{"-e", script}
}

func escapeAppleScript(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func (n CommandNotifier) Notify(event Event) Result {
	notification := FormatEvent(event)
	name, args := n.build(notification)
	if n.runner == nil {
		err := errors.New("notification runner unavailable")
		return Result{Degraded: true, Message: err.Error(), Err: err}
	}
	if err := n.runner(name, args...); err != nil {
		return Result{Degraded: true, Message: err.Error(), Err: err}
	}
	return Result{Delivered: true, Message: "delivered"}
}
```

Update imports in `notifier.go` to include:

```go
"os/exec"
"strings"
```

Run:

```bash
gofmt -w internal/notify/notifier.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/notify
```

Expected: fail with redeclared symbols until old files are deleted.

- [x] **Step 2: Remove Linux and unsupported-platform adapters**

Change `NewPlatformNotifier` to:

```go
func NewPlatformNotifier(_ string, runner Runner) Notifier {
	return NewCommandNotifier(MacOSCommand, runner)
}
```

This keeps a stable call site while reflecting product scope: only the macOS command is supported.

Delete the obsolete notification adapter files via patch:

```text
internal/notify/command.go
internal/notify/macos.go
internal/notify/linux.go
internal/notify/noop.go
```

Update tests to remove `TestLinuxCommandUsesSeparateArguments` and `TestUnsupportedNotifierReturnsDegradedState`. Keep macOS escaping and manager suppression tests.

Run:

```bash
gofmt -w internal/notify/notifier.go internal/notify/notifier_test.go
rg -n 'LinuxCommand|UnsupportedNotifier|notify-send' internal cmd --glob '*.go'
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/notify ./internal/tui ./internal/jsonout
```

Expected: `rg` has no matches and tests pass.

## Gate E: Route Module And Test Locality

### Task E1: Extract route-local action dispatch from `update.go`

**Files:**
- Create: `internal/tui/route_actions.go`
- Modify: `internal/tui/update.go`
- Test: `internal/tui/update_test.go`

- [x] **Step 1: Move focused action dispatch into route-specific helpers**

Create `internal/tui/route_actions.go` with:

```go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) activateFocusedAction(action string) (tea.Model, tea.Cmd) {
	switch m.route {
	case RouteList, RouteAmbiguous:
		return m.activateListAction(action)
	case RouteWorkspace:
		return m.activateWorkspaceAction(action)
	case RouteConfig:
		return m.activateConfigAction(action)
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateListAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "session":
		if selected := m.selectedSession(); selected != nil {
			m.route = RouteWorkspace
			m.selectedID = selected.SessionID
			m.lastAction = "open_session"
			return m, nil
		}
		m.lastAction = "activate_session"
		return m, nil
	case "reminder":
		m.toggleReminderForSelected()
		return m, nil
	case "keepalive":
		m.toggleKeepAliveForSelected()
		return m, nil
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateWorkspaceAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "reminder":
		m.toggleReminderForSelected()
		return m, nil
	case "keepalive":
		m.toggleKeepAliveForSelected()
		return m, nil
	case "keepalive_autosend":
		m.toggleKeepAliveAutoSendForSelected()
		return m, nil
	case "keepalive_send_now":
		return m.sendKeepAliveNow()
	case "keepalive_cancel", "keepalive_stop_waiting":
		m.cancelKeepAlive()
		return m, nil
	case "keepalive_acknowledge":
		if selected := m.selectedSession(); selected != nil {
			m.keepAliveManager.Acknowledge(selected.SessionID)
			m.lastAction = "acknowledge_keepalive"
			m.focusIndex = m.defaultFocusIndex()
		}
		return m, nil
	case "copy_id":
		m.lastAction = "copy_session_id"
		if selected := m.selectedSession(); selected != nil {
			m.setNotice("Session ID shown: "+selected.SessionID, RoleInfo, 3*time.Second)
		}
		return m, nil
	case "back":
		m.route = RouteList
		m.lastAction = "back_to_list"
		return m, nil
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateConfigAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "config_reminder_thresholds", "config_trigger", "config_countdown", "config_message", "config_max_sends":
		m.startConfigEdit(action)
		return m, nil
	case "config_autosend":
		m.toggleConfigAutoSend()
		return m, nil
	case "config_save":
		return m.saveConfig()
	case "config_reset":
		return m.resetConfigDefaults()
	case "config_cancel":
		return m.cancelConfig()
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateSharedAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "refresh":
		return m.Update(ManualRefreshMsg{})
	case "config":
		m.route = RouteConfig
		m.focusIndex = m.defaultFocusIndex()
		return m, nil
	case "help":
		m.helpOpen = !m.helpOpen
		m.lastAction = "toggle_help"
		return m, nil
	case "quit":
		m.lastAction = "quit"
		return m, tea.Quit
	default:
		m.lastAction = "activate_" + action
		return m, nil
	}
}
```

Update the existing action dispatch in `update.go` to call `activateFocusedAction(m.FocusedAction())`.

Run:

```bash
gofmt -w internal/tui/route_actions.go internal/tui/update.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'Test.*Focus|Test.*Shortcut|Test.*Config'
```

Expected: pass. This is route-local dispatch, not a renamed copy of the old global switch.

### Task E2: Extract Workspace KeepAlive behavior

**Files:**
- Create: `internal/tui/workspace_keepalive.go`
- Modify: `internal/tui/workspace.go`
- Modify: `internal/tui/update.go`
- Test: `internal/tui/update_keepalive_test.go`
- Test: `internal/tui/render_test.go`

- [x] **Step 1: Move KeepAlive Workspace rendering helpers**

Move these functions from `workspace.go` to `workspace_keepalive.go` without behavior changes:

```text
activeKeepAliveCard
keepAliveControlState
keepAliveControlDetail
keepAliveCard
keepAliveBadge
activeKeepAliveState
nextKeepAliveTime
autoSendWorkspaceDetail
autoSendWorkspaceDetailForState
workspaceCanSendKeepAlive
workspaceCanCancelKeepAlive
```

Run:

```bash
gofmt -w internal/tui/workspace.go internal/tui/workspace_keepalive.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'TestWorkspace.*KeepAlive|Test.*KeepAliveCard'
```

Expected: pass.

### Task E3: Split giant TUI tests by module seam

**Files:**
- Modify: `internal/tui/update_test.go`
- Modify: `internal/tui/render_test.go`
- Create: `internal/tui/update_refresh_test.go`
- Create: `internal/tui/update_keepalive_test.go`
- Create: `internal/tui/update_config_test.go`

- [x] **Step 1: Move refresh tests**

Move tests whose names contain `Refresh`, `Watcher`, or `DegradedMessage` from `update_test.go` into `update_refresh_test.go`. Keep shared helpers in `update_test.go` for this phase so this is a mechanical split.

Run:

```bash
gofmt -w internal/tui/update_test.go internal/tui/update_refresh_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'Test.*Refresh|Test.*Watcher|Test.*Degraded'
```

Expected: pass.

- [x] **Step 2: Move KeepAlive tests**

Move tests whose names contain `KeepAlive`, `Claude`, `Countdown`, `Runner`, or `Confirmation` from `update_test.go` into `update_keepalive_test.go`.

Run:

```bash
gofmt -w internal/tui/update_test.go internal/tui/update_keepalive_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'Test.*KeepAlive|Test.*Claude|Test.*Countdown|Test.*Runner|Test.*Confirmation'
```

Expected: pass.

- [x] **Step 3: Move Config tests**

Move tests whose names contain `Config` from `update_test.go` into `update_config_test.go`.

Run:

```bash
gofmt -w internal/tui/update_test.go internal/tui/update_config_test.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui -run 'Test.*Config'
```

Expected: pass.

- [x] **Step 4: Check test file sizes**

Run:

```bash
wc -l internal/tui/update_test.go internal/tui/render_test.go internal/tui/update_refresh_test.go internal/tui/update_keepalive_test.go internal/tui/update_config_test.go
```

Expected: no newly created test file exceeds 1000 lines. `render_test.go` may remain over 1000 in this phase only if render decomposition would mix behavior refactor with mechanical movement; record that in progress.

## Gate F: Session Snapshot Adapter Slimming

### Task F1: Move TUI conversion out of app orchestration

**Files:**
- Create: `internal/tui/snapshot_options.go`
- Modify: `internal/app/app.go`
- Modify: `internal/tui/model.go`
- Test: `internal/app/cli_test.go`
- Test: `internal/tui/update_test.go`

- [x] **Step 1: Create TUI snapshot adapter**

Create `internal/tui/snapshot_options.go`:

```go
package tui

import (
	"github.com/richardchen/cc-watch/internal/session"
	"github.com/richardchen/cc-watch/internal/snapshot"
)

type SnapshotOptionsInput struct {
	Result       snapshot.Result
	Dependencies Dependencies
	StartMode    StartMode
}

func OptionsFromSnapshot(input SnapshotOptionsInput) Options {
	result := input.Result
	refreshState := RefreshViewState{ProjectsDir: result.ProjectsDir}
	var selectedID string
	var ambiguousID string
	sessions := result.Sessions
	if result.Selected != nil {
		sessions = []session.Session{*result.Selected}
		selectedID = result.Selected.SessionID
	}
	if result.Error != nil {
		switch result.Error.Code {
		case "ambiguous_session_id":
			ambiguousID = result.Error.Query
			sessions = result.Candidates
		case "session_not_found":
			sessions = nil
			refreshState.EmptyState = EmptyNoSessions
		}
	}
	if refreshState.EmptyState == EmptyNone {
		refreshState.EmptyState = emptyStateFromSnapshot(result.EmptyState)
	}
	return Options{
		Now:                result.GeneratedAt,
		Dependencies:       input.Dependencies,
		Sessions:           sessions,
		SelectedID:         selectedID,
		AmbiguousID:        ambiguousID,
		ReminderEnabled:    reminderEnabledFromSnapshot(result.Reminder),
		ReminderThresholds: result.Config.ReminderThresholds,
		KeepAliveConfig:    normalizeKeepAliveConfig(result.Config.KeepAlive),
		Refresh:            refreshState,
		StartMode:          input.StartMode,
		StartDisplayTicker: true,
		StartRefreshTicker: input.StartMode != StartConfig,
		Config:             normalizeConfig(result.Config),
	}
}

func emptyStateFromSnapshot(state snapshot.EmptyState) EmptyState {
	switch state {
	case snapshot.EmptyProjectsDir:
		return EmptyProjectsDir
	case snapshot.EmptyNoSessions:
		return EmptyNoSessions
	default:
		return EmptyNone
	}
}

func reminderEnabledFromSnapshot(states map[string]snapshot.ReminderState) map[string]bool {
	enabled := make(map[string]bool, len(states))
	for id, state := range states {
		if state.Enabled {
			enabled[id] = true
		}
	}
	return enabled
}

func RefreshSnapshotFromSnapshotResult(result snapshot.Result) RefreshSnapshot {
	sessions := result.Sessions
	refreshState := RefreshViewState{
		ProjectsDir: result.ProjectsDir,
		EmptyState:  emptyStateFromSnapshot(result.EmptyState),
	}
	if result.Error != nil {
		switch result.Error.Code {
		case "ambiguous_session_id":
			sessions = result.Candidates
		case "session_not_found":
			sessions = nil
			refreshState.EmptyState = EmptyNoSessions
		}
	}
	return RefreshSnapshot{
		Sessions:   sessions,
		Refresh:    refreshState,
		HasRefresh: true,
	}
}
```

Run:

```bash
gofmt -w internal/tui/snapshot_options.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/tui
```

Expected: pass or fail on type mismatch. Fix only type mismatches in this new adapter.

- [x] **Step 2: Use adapter from app**

In `internal/app/app.go`, replace the session/empty-state/reminder mapping in `buildTUIOptionsFromSnapshot` with:

```go
options := tui.OptionsFromSnapshot(tui.SnapshotOptionsInput{
	Result:       result,
	Dependencies: tuiDependencies(cmd, deps, home),
	StartMode:    startMode,
})
options.LiveRefresh = liveRefresh
return options
```

Delete `tuiEmptyState` and `tuiReminderEnabled` from app if no callers remain.

Replace `tuiRefreshSnapshotFromResult(result)` call sites with:

```go
return tui.RefreshSnapshotFromSnapshotResult(result)
```

Delete `tuiRefreshSnapshotFromResult` from app after call sites move.

Run:

```bash
gofmt -w internal/app/app.go internal/tui/snapshot_options.go
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/app ./internal/tui
```

Expected: pass.

## Gate G: Documentation, Review, And Verification

### Task G1: Update docs and progress

**Files:**
- Modify: `CONTEXT.md`
- Modify: `docs/superpowers/progress/cc-watch-v2-progress.md`
- Modify: `docs/superpowers/plans/cc-watch-v2/PLAN.md`
- Modify: `docs/superpowers/plans/cc-watch-v2/phase-11.9-architecture-simplification.md`

- [x] **Step 1: Add new context terms**

Add terms only if they are used in the implementation:

```markdown
### Notification Runtime

The macOS notification path for Reminder and KeepAlive events. It owns event wording, osascript command construction, failure suppression, and notification degraded state.

### Live Refresh Adapter

The TUI adapter that turns Refresh Runtime watcher results, debounce decisions, and safety ticks into Bubble Tea messages. Refresh Runtime owns refresh policy; the adapter owns Bubble Tea message conversion.
```

Run:

```bash
wc -l CONTEXT.md
```

Expected: `CONTEXT.md` remains under 120 lines.

- [x] **Step 2: Mark plan checkboxes during execution**

As each task completes, change its checkbox from `[ ]` to `[x]`. Do not mark future tasks complete.

- [x] **Step 3: Update progress ledger**

Add a Phase 11.9 verification ledger row with the exact commands run and results. Update Last Context Handoff to state:

```markdown
Phase 11.9 deepened Refresh Runtime production wiring, kept live refresh in scope, improved KeepAlive Runtime locality, removed false Reminder/notification depth, split oversized TUI test surfaces, and preserved CLI/JSON/config/parser/KeepAlive safety contracts. Next phase is local macOS install planning; do not replace `$HOME/.local/bin/cc-watch` without explicit approval.
```

### Task G2: Final verification

**Files:**
- No new source files beyond previous tasks.

- [x] **Step 1: Focused package tests**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test ./internal/refresh ./internal/app ./internal/tui ./internal/keepalive ./internal/notify ./internal/jsonout ./internal/snapshot
```

Expected: pass.

- [x] **Step 2: Full suite**

Run:

```bash
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go test -count=1 ./...
```

Expected: pass.

- [x] **Step 3: Dead and live symbol scans**

Run:

```bash
rg -n 'internal/reminder|reminder\.|LinuxCommand|UnsupportedNotifier|notify-send' internal cmd --glob '*.go'
rg -n 'NewWatcher|NewFSNotifyFS|NewCoordinator|RefreshWatcherEventsMsg|RefreshDebounceElapsedMsg|LiveRefreshCommand' internal cmd --glob '*.go'
```

Expected: first command has no matches. Second command has matches in production code and tests.

- [x] **Step 4: Build throwaway binary**

Run:

```bash
mkdir -p /private/tmp/cc-watch-phase119
GOCACHE=/private/tmp/cc-watch-go-build GOMODCACHE=/private/tmp/cc-watch-go-mod go build -o /private/tmp/cc-watch-phase119/cc-watch ./cmd/cc-watch
```

Expected: command exits 0 and writes `/private/tmp/cc-watch-phase119/cc-watch`.

- [x] **Step 5: JSON smoke**

Run:

```bash
HOME="$PWD/internal/session/testdata/smoke-home" /private/tmp/cc-watch-phase119/cc-watch --json
```

Expected: exits 0, emits `schema_version: 1`, includes `sessions`, and has `"error": null`.

- [x] **Step 6: Whitespace and status**

Run:

```bash
git diff --check
git status --short
```

Expected: whitespace check has no output. Status shows only intended Phase 11.9 source/doc changes. No installed command path changed. No release artifacts were added to the repo. No real Claude send was run.

## Review Gates

After Gate B, dispatch a read-only reviewer focused on live Refresh Runtime correctness and the no-public-watch contract.

After Gate C, dispatch a read-only reviewer focused on KeepAlive safety and ADR conformance.

After Gate E, dispatch a strict code-quality reviewer focused on file-size, route locality, and spaghetti branching.

After final verification, dispatch one final read-only reviewer for product-scope and regression risk.

## Self-Review Notes

- Spec coverage: all architecture report candidates are covered. Refresh Runtime is Gate B, KeepAlive Runtime is Gate C, Reminder and macOS Notify are Gate D, Route Modules and tests are Gate E, Session Snapshot adapter slimming is Gate F.
- Go stack: kept because the product reality is a local macOS terminal binary with filesystem watching, subprocess control, native notification command execution, stable JSON, and simple local install.
- Live refresh: preserved and made first-class. Deleting refresh watcher/coordinator is forbidden in this phase.
- Scope control: Linux, Windows, Homebrew, GitHub Release, goreleaser, public watch mode, native macOS app, daemon behavior, and real Claude sends remain out of scope.
