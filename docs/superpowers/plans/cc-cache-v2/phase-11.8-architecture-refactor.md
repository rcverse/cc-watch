# cc-cache v2 Phase 11.8 Architecture Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the post-11.7 codebase into a smaller, sharper architecture before Phase 12 packaging, preserving public behavior while removing historical traces, duplicated session assembly, and shallow TUI seams.

**Architecture:** Introduce one deep `internal/snapshot` module that owns the live session/config/discovery state consumed by JSON and TUI startup. List refresh also uses snapshots; selected workspace refresh keeps the current selected-path fast path and parses `selected.JSONLPath` directly so it does not rediscover or replace the selected session by accident. Then delete historical dead code and narrow the TUI root so it consumes one refresh callback instead of rebuilding product state through broad callbacks.

**Tech Stack:** Go 1.23+, existing Bubble Tea/Lip Gloss TUI, existing `internal/session`, `internal/config`, `internal/jsonout`, `internal/keepalive`, `internal/notify`, and existing test harness with `go test`.

---

## Baseline

- Baseline branch/tag requirement has been satisfied before this plan: `phase-11.7-stable-before-11.8` points at the clean Phase 11.7 baseline commit.
- Implementation branch: `codex/phase-11.8-architecture-refactor`.
- Public behavior freeze:
  - Do not change CLI flags or exit-code contract.
  - Do not change JSON schema version or public field names.
  - Do not change config file schema.
  - Do not change parser metrics or fixture semantics.
  - Do not replace `$HOME/.local/bin/cc-cache`.
  - Do not run a real Claude KeepAlive send.
  - Do not start Phase 12 packaging.
  - Do not add dependencies.

## Live Behavior Source

Treat these as live, behavior-preserving requirements:

- `cc-cache` opens the TUI list using the latest sessions.
- `cc-cache --id <partial>` opens one selected workspace or ambiguous selection state.
- `cc-cache --json` emits the accepted schema from `docs/adr/2026-06-03-json-output-schema.md`.
- `cc-cache config` opens the config editor without session discovery.
- TUI refresh keeps list/session data current without exposing public `--watch`.
- Reminder never invokes Claude.
- KeepAlive remains visible, bounded, cancellable, and fake-runner-testable.

## Dead/Historical Trace Targets

Remove only after tests prove they are not live:

- `internal/app/dependencies.go` dependency-anchor file.
- `internal/app.Dependencies.StartWatcher`.
- `internal/app.Dependencies.StartNotifier`.
- `internal/app.Dependencies.NewKeepAliveRunner`.
- `internal/tui.Dependencies.Discover`.
- `internal/tui.Dependencies.Parse`.
- `internal/tui.Dependencies.Refresh`.
- `internal/tui.SafetyRefreshMsg` if no live runtime path needs it after refresh unification.
- `internal/tui.KeepAliveStatus` if help and workspace behavior can derive from `keepalive.SessionState`.
- `internal/tui.Model.evidenceOffset`, which is a historical field after Evidence was removed from the default and expanded TUI.

## File Structure

### Create

- `CONTEXT.md` - small cc-cache domain glossary used by future architecture work. Keep it under 80 lines.
- `internal/snapshot/snapshot.go` - public interface for building current session/config snapshots.
- `internal/snapshot/snapshot_test.go` - behavior-preserving tests for list, selected, ambiguous, missing, config warning, and runtime projection behavior.

### Modify

- `internal/app/app.go` - replace duplicated TUI/JSON session assembly with `snapshot.Build`.
- `internal/app/cli_test.go` - keep public CLI behavior assertions; add regressions proving JSON/TUI share snapshot behavior.
- `internal/jsonout/json.go` - consume snapshot-derived runtime state without `map[string]any` construction in app code; preserve external JSON.
- `internal/jsonout/json_test.go` - assert unchanged schema and typed KeepAlive scope.
- `internal/tui/model.go` - replace broad refresh dependencies with snapshot refresh callbacks; remove unused dependency hooks and historical state.
- `internal/tui/messages.go` - remove `SafetyRefreshMsg` when refresh ticks cover the same live path.
- `internal/tui/update.go` - route refresh through snapshot callbacks; remove dead update branches.
- `internal/tui/help.go` and `internal/tui/help_test.go` - derive KeepAlive help from live `keepalive.SessionState` rather than `KeepAliveStatus`.
- `internal/tui/list.go`, `internal/tui/workspace.go`, `internal/tui/render_test.go`, `internal/tui/update_test.go` - adjust tests only where the refactor changes internal seams; do not change visible behavior unless deleting dead paths exposes stale copy.
- `docs/superpowers/plans/cc-cache-v2/PLAN.md` - add Phase 11.8 to phase index.
- `docs/superpowers/progress/cc-cache-v2-progress.md` - mark current phase as Phase 11.8 after implementation starts and update verification ledger before stopping work.

### Delete

- `internal/app/dependencies.go` if `go test ./internal/app` still passes after removing dependency anchors.

## Target Snapshot Interface

Use this exact shape unless implementation reveals a compile-time simplification that reduces the interface further.

```go
package snapshot

import (
	"time"

	"github.com/richardchen/cc-cache/internal/config"
	"github.com/richardchen/cc-cache/internal/session"
)

type Loaders struct {
	LoadConfig   func(home string) (config.LoadResult, error)
	DiscoverHome func(home string, limit int) (session.DiscoveryResult, error)
	ParseFile    func(path string) (session.Session, error)
}

type Request struct {
	Home   string
	Now    time.Time
	Limit  int
	ID     string
	Remind bool
}

type EmptyState string

const (
	EmptyNone        EmptyState = ""
	EmptyProjectsDir EmptyState = "projects_dir_missing"
	EmptyNoSessions  EmptyState = "no_sessions"
)

type Error struct {
	Code    string
	Message string
	Query   string
}

type ReminderState struct {
	Enabled    bool
	Thresholds []int
}

type KeepAliveState struct {
	Enabled  bool
	AutoSend bool
	Mode     string
	MaxSends int
	State    string
}

type Result struct {
	GeneratedAt    time.Time
	QueryID        string
	QueryLimit     int
	Config         config.Config
	ConfigWarnings []config.Warning
	ProjectsDir    string
	EmptyState     EmptyState
	Sessions       []session.Session
	Selected       *session.Session
	Candidates     []session.Session
	Reminder       map[string]ReminderState
	KeepAlive      map[string]KeepAliveState
	Error          *Error
}

func Build(req Request, loaders Loaders) (Result, error)
func ConfigOnly(req Request, loaders Loaders) (Result, error)
```

Operational failures must use a typed error so callers do not guess the public error code:

```go
type ErrorStage string

const (
	StageConfig    ErrorStage = "config"
	StageDiscovery ErrorStage = "discovery"
	StageParse     ErrorStage = "parse"
)

type BuildError struct {
	Stage ErrorStage
	Code  string
	Err   error
}

func (e *BuildError) Error() string
func (e *BuildError) Unwrap() error
```

Stage mapping:

- `StageConfig` uses public code `config_error`.
- `StageDiscovery` preserves current JSON behavior and uses public code `parse_error`.
- `StageParse` uses public code `parse_error`.
- Partial-ID ambiguity and no-match are not `BuildError`; they stay in `Result.Error`.

Rules:

- `Build` owns config load, discovery limit selection, partial-ID resolution, parse execution, empty-state mapping, candidate projection, and initial Reminder/KeepAlive runtime projection.
- `ConfigOnly` loads config only and performs no session discovery or parsing.
- `Build` returns `*BuildError` only for failures that should abort the caller immediately: config load failures, discovery failures, and parse failures.
- Partial-ID no-match and ambiguity are represented as `Result.Error` so JSON and TUI can present them consistently.
- `Result.Sessions`, `Result.Candidates`, config warnings, and runtime maps must be deep-copied enough that callers cannot mutate snapshot internals accidentally.
- The snapshot module must not import `internal/app`, `internal/tui`, or Bubble Tea.

## State Mapping

| Current condition | Snapshot result |
|---|---|
| Config command | `ConfigOnly`, no discovery, `Sessions == nil`, `Error == nil` |
| Missing projects dir | `EmptyStateProjectsDir`, `Sessions == []`, `Error == nil` for TUI, JSON output remains empty success unless current JSON behavior says otherwise |
| No sessions | `EmptyStateNoSessions`, `Sessions == []`, `Error == nil` |
| `--id` exact/unique partial match | `Selected != nil`, `Sessions` contains selected session |
| `--id` ambiguous | `Error.Code == "ambiguous_session_id"`, `Candidates` contains safe candidate sessions |
| `--id` no match | `Error.Code == "session_not_found"`, `Candidates == nil` |
| Parser read failure | returned Go `error`; app maps to `parse_error` JSON or TUI startup error as today |
| Invalid config file with warnings | `Config` is default, `ConfigWarnings` records warnings |

## TUI Snapshot Mapping

`buildTUIOptions` must map `snapshot.Result` to `tui.Options` exactly as follows:

| Snapshot result | TUI options |
|---|---|
| `ConfigOnly` result | `StartMode: tui.StartConfig`, no sessions, no discovery-derived empty state |
| Normal list result | `Sessions: result.Sessions`, `Refresh.EmptyState: mapped result.EmptyState`, no selected ID |
| Selected result | `Sessions: []session.Session{*result.Selected}`, `SelectedID: result.Selected.SessionID` |
| `Result.Error.Code == "ambiguous_session_id"` | `AmbiguousID: result.Error.Query`, `Sessions: result.Candidates`, route becomes ambiguous through existing `routeFromOptions` |
| `Result.Error.Code == "session_not_found"` | Preserve current behavior: no selected session, `Refresh.EmptyState: tui.EmptyNoSessions` |
| `EmptyProjectsDir` | `Refresh.EmptyState: tui.EmptyProjectsDir`, `Refresh.ProjectsDir: result.ProjectsDir` |
| `EmptyNoSessions` | `Refresh.EmptyState: tui.EmptyNoSessions`, `Refresh.ProjectsDir: result.ProjectsDir` |

`buildTUIOptions` must not expose `snapshot.Result.Error` directly to the TUI. It maps query errors into the current route/empty-state behavior.

## Refresh Ownership

There are two refresh paths after this refactor:

1. List refresh: call `snapshot.Build` with the original command request and apply the returned sessions/empty state.
2. Selected workspace refresh: preserve current behavior by parsing only `selected.JSONLPath` with `deps.ParseFile(selected.JSONLPath)`. Do not call `snapshot.Build` for selected workspace refresh because that would rediscover sessions and can change scope.

The narrowed TUI callback is still one interface:

```go
RefreshSnapshot func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot
```

but the app adapter behind it branches internally on `selected != nil`.

## Task 1: Snapshot Module Red Tests

**Files:**
- Create: `internal/snapshot/snapshot_test.go`

- [ ] **Step 1: Write failing snapshot tests**

Create `internal/snapshot/snapshot_test.go` with these tests:

```go
package snapshot

import (
	"errors"
	"testing"
	"time"

	"github.com/richardchen/cc-cache/internal/config"
	"github.com/richardchen/cc-cache/internal/session"
)

func TestBuildListSnapshotLoadsConfigDiscoversAndParsesSessions(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	loaders := fakeLoaders(t)
	loaders.files = []session.SessionFile{
		{SessionID: "11111111-1111-1111-1111-111111111111", ShortID: "11111111", Project: "alpha", Path: "/tmp/alpha.jsonl", ModTime: now},
		{SessionID: "22222222-2222-2222-2222-222222222222", ShortID: "22222222", Project: "beta", Path: "/tmp/beta.jsonl", ModTime: now},
	}

	result, err := Build(Request{Home: "/home/me", Now: now, Limit: 5}, loaders.Loaders())
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if loaders.discoveryLimit != 5 {
		t.Fatalf("discovery limit = %d, want 5", loaders.discoveryLimit)
	}
	if len(result.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(result.Sessions))
	}
	if result.Selected != nil {
		t.Fatalf("selected = %#v, want nil for list snapshot", result.Selected)
	}
	if result.EmptyState != EmptyNone {
		t.Fatalf("empty state = %q, want none", result.EmptyState)
	}
	if got := result.Reminder["11111111-1111-1111-1111-111111111111"].Enabled; got {
		t.Fatalf("reminder enabled = %v, want false without --remind", got)
	}
	if got := result.KeepAlive["11111111-1111-1111-1111-111111111111"].State; got != "off" {
		t.Fatalf("keepalive state = %q, want off", got)
	}
}

func TestBuildSelectedSnapshotResolvesPartialIDAndParsesOnlySelected(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	loaders := fakeLoaders(t)
	loaders.files = []session.SessionFile{
		{SessionID: "11111111-1111-1111-1111-111111111111", ShortID: "11111111", Project: "alpha", Path: "/tmp/alpha.jsonl", ModTime: now},
		{SessionID: "22222222-2222-2222-2222-222222222222", ShortID: "22222222", Project: "beta", Path: "/tmp/beta.jsonl", ModTime: now},
	}

	result, err := Build(Request{Home: "/home/me", Now: now, Limit: 5, ID: "2222", Remind: true}, loaders.Loaders())
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if loaders.discoveryLimit != 0 {
		t.Fatalf("discovery limit = %d, want 0 for selected lookup", loaders.discoveryLimit)
	}
	if len(loaders.parsedPaths) != 1 || loaders.parsedPaths[0] != "/tmp/beta.jsonl" {
		t.Fatalf("parsed paths = %#v, want selected file only", loaders.parsedPaths)
	}
	if result.Selected == nil || result.Selected.SessionID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("selected = %#v, want beta session", result.Selected)
	}
	if got := result.Reminder[result.Selected.SessionID].Enabled; !got {
		t.Fatalf("reminder enabled = %v, want true with --remind", got)
	}
}

func TestBuildAmbiguousAndNoMatchAreResultErrorsNotReturnedErrors(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	loaders := fakeLoaders(t)
	loaders.files = []session.SessionFile{
		{SessionID: "11111111-1111-1111-1111-111111111111", ShortID: "11111111", Project: "alpha", Path: "/tmp/alpha.jsonl", ModTime: now},
		{SessionID: "11112222-2222-2222-2222-222222222222", ShortID: "11112222", Project: "beta", Path: "/tmp/beta.jsonl", ModTime: now},
	}

	ambiguous, err := Build(Request{Home: "/home/me", Now: now, ID: "1111", Limit: 5}, loaders.Loaders())
	if err != nil {
		t.Fatalf("ambiguous Build returned Go error: %v", err)
	}
	if ambiguous.Error == nil || ambiguous.Error.Code != "ambiguous_session_id" {
		t.Fatalf("ambiguous error = %#v, want ambiguous_session_id", ambiguous.Error)
	}
	if len(ambiguous.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(ambiguous.Candidates))
	}

	missing, err := Build(Request{Home: "/home/me", Now: now, ID: "9999", Limit: 5}, loaders.Loaders())
	if err != nil {
		t.Fatalf("missing Build returned Go error: %v", err)
	}
	if missing.Error == nil || missing.Error.Code != "session_not_found" {
		t.Fatalf("missing error = %#v, want session_not_found", missing.Error)
	}
}

func TestConfigOnlyDoesNotDiscoverOrParse(t *testing.T) {
	loaders := fakeLoaders(t)
	result, err := ConfigOnly(Request{Home: "/home/me"}, loaders.Loaders())
	if err != nil {
		t.Fatalf("ConfigOnly returned error: %v", err)
	}
	if loaders.discoverCalled {
		t.Fatalf("ConfigOnly called discovery")
	}
	if len(loaders.parsedPaths) != 0 {
		t.Fatalf("ConfigOnly parsed paths: %#v", loaders.parsedPaths)
	}
	if result.Config.KeepAlive.Message == "" {
		t.Fatalf("expected loaded config defaults")
	}
}

func TestBuildMapsProjectsDirMissingToEmptyState(t *testing.T) {
	loaders := fakeLoaders(t)
	loaders.discovery = session.DiscoveryResult{ProjectsDir: "/home/me/.claude/projects", ErrorCode: "projects_dir_missing"}
	result, err := Build(Request{Home: "/home/me", Limit: 5}, loaders.Loaders())
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if result.EmptyState != EmptyProjectsDir {
		t.Fatalf("empty state = %q, want projects_dir_missing", result.EmptyState)
	}
	if len(result.Sessions) != 0 {
		t.Fatalf("sessions = %d, want 0", len(result.Sessions))
	}
}

func TestBuildPropagatesConfigWarnings(t *testing.T) {
	loaders := fakeLoaders(t)
	loaders.configWarnings = []config.Warning{{
		Code:    config.WarningInvalidJSON,
		Message: "bad config",
	}}
	result, err := Build(Request{Home: "/home/me", Limit: 5}, loaders.Loaders())
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(result.ConfigWarnings) != 1 || result.ConfigWarnings[0].Message != "bad config" {
		t.Fatalf("config warnings = %#v, want bad config warning", result.ConfigWarnings)
	}
}

func TestBuildReturnsOperationalErrors(t *testing.T) {
	loaders := fakeLoaders(t)
	loaders.discoverErr = errors.New("disk unavailable")
	_, err := Build(Request{Home: "/home/me", Limit: 5}, loaders.Loaders())
	var buildErr *BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("error = %T %v, want *BuildError", err, err)
	}
	if buildErr.Stage != StageDiscovery || buildErr.Code != "parse_error" {
		t.Fatalf("build error = %#v, want discovery parse_error", buildErr)
	}
}

func TestBuildSelectedParseFailureReturnsParseBuildError(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	loaders := fakeLoaders(t)
	loaders.files = []session.SessionFile{{
		SessionID: "11111111-1111-1111-1111-111111111111",
		ShortID: "11111111",
		Project: "alpha",
		Path: "/tmp/alpha.jsonl",
		ModTime: now,
	}}
	loaders.parseErr = errors.New("read failed")
	_, err := Build(Request{Home: "/home/me", Now: now, ID: "1111", Limit: 5}, loaders.Loaders())
	var buildErr *BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("error = %T %v, want *BuildError", err, err)
	}
	if buildErr.Stage != StageParse || buildErr.Code != "parse_error" {
		t.Fatalf("build error = %#v, want parse parse_error", buildErr)
	}
}

type snapshotFakeLoaders struct {
	t              *testing.T
	files          []session.SessionFile
	discovery      session.DiscoveryResult
	discoverErr    error
	parseErr       error
	configWarnings []config.Warning
	discoverCalled bool
	discoveryLimit int
	parsedPaths    []string
}

func fakeLoaders(t *testing.T) *snapshotFakeLoaders {
	t.Helper()
	return &snapshotFakeLoaders{t: t}
}

func (f *snapshotFakeLoaders) Loaders() Loaders {
	return Loaders{
		LoadConfig: func(home string) (config.LoadResult, error) {
			if home != "/home/me" {
				f.t.Fatalf("home = %q, want /home/me", home)
			}
			return config.LoadResult{Config: config.Default(), Warnings: f.configWarnings}, nil
		},
		DiscoverHome: func(home string, limit int) (session.DiscoveryResult, error) {
			f.discoverCalled = true
			f.discoveryLimit = limit
			if f.discoverErr != nil {
				return session.DiscoveryResult{}, f.discoverErr
			}
			if f.discovery.ProjectsDir != "" || f.discovery.ErrorCode != "" {
				return f.discovery, nil
			}
			return session.DiscoveryResult{ProjectsDir: "/home/me/.claude/projects", Sessions: f.files}, nil
		},
		ParseFile: func(path string) (session.Session, error) {
			f.parsedPaths = append(f.parsedPaths, path)
			if f.parseErr != nil {
				return session.Session{}, f.parseErr
			}
			for _, file := range f.files {
				if file.Path == path {
					return session.Session{
						SessionID:      file.SessionID,
						ShortID:        file.ShortID,
						Project:        file.Project,
						JSONLPath:      file.Path,
						FileModifiedAt: file.ModTime,
					}, nil
				}
			}
			f.t.Fatalf("unexpected parse path %q", path)
			return session.Session{}, nil
		},
	}
}
```

- [ ] **Step 2: Run snapshot red test**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/snapshot
```

Expected: fail with `package github.com/richardchen/cc-cache/internal/snapshot is not in std` or undefined `Build`, `ConfigOnly`, and snapshot types.

## Task 2: Implement Snapshot Module

**Files:**
- Create: `internal/snapshot/snapshot.go`
- Test: `internal/snapshot/snapshot_test.go`

- [ ] **Step 1: Implement snapshot interface**

Create `internal/snapshot/snapshot.go` with the target interface from this plan and this implementation behavior:

```go
func Build(req Request, loaders Loaders) (Result, error) {
	if req.Limit <= 0 {
		req.Limit = 5
	}
	result, err := loadBase(req, loaders)
	if err != nil {
		return Result{}, &BuildError{Stage: StageConfig, Code: "config_error", Err: err}
	}
	discoveryLimit := req.Limit
	if req.ID != "" {
		discoveryLimit = 0
	}
	discovery, err := loaders.DiscoverHome(req.Home, discoveryLimit)
	if err != nil {
		return Result{}, &BuildError{Stage: StageDiscovery, Code: "parse_error", Err: err}
	}
	result.ProjectsDir = discovery.ProjectsDir
	if discovery.ErrorCode == "projects_dir_missing" {
		result.EmptyState = EmptyProjectsDir
	}
	if req.ID != "" {
		return buildSelected(req, loaders, result, discovery.Sessions)
	}
	for _, file := range discovery.Sessions {
		parsed, err := loaders.ParseFile(file.Path)
		if err != nil {
			return Result{}, &BuildError{Stage: StageParse, Code: "parse_error", Err: err}
		}
		result.Sessions = append(result.Sessions, parsed)
	}
	if len(result.Sessions) == 0 && result.EmptyState == EmptyNone {
		result.EmptyState = EmptyNoSessions
	}
	result.populateRuntime(req.Remind)
	return result, nil
}
```

The implementation may split helper functions, but keep them private and boring:

- `loadBase`
- `buildSelected`
- `sessionFilesToCandidates`
- `populateRuntime`
- `cloneSessions`
- `cloneWarnings`

Do not import `internal/jsonout` or `internal/tui`. Snapshot owns domain state only.

- [ ] **Step 2: Run snapshot tests**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/snapshot
```

Expected: pass.

- [ ] **Step 3: Run existing data packages**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/session ./internal/config ./internal/jsonout
```

Expected: pass.

## Task 3: Route App JSON Through Snapshot

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/cli_test.go`
- Modify: `internal/jsonout/json.go`
- Modify: `internal/jsonout/json_test.go`

- [ ] **Step 1: Add app regression tests for shared snapshot behavior**

Add tests to `internal/app/cli_test.go`:

```go
func TestJSONAndTUIUseSameSelectedDiscoverySemantics(t *testing.T) {
	deps := testDepsWithTwoSessions(t)
	var tuiCommand Command
	deps.StartTUI = func(cmd Command) error {
		tuiCommand = cmd
		return nil
	}

	if code := RunWithDeps([]string{"--id", "2222"}, io.Discard, io.Discard, deps); code != 0 {
		t.Fatalf("TUI selected run exit = %d, want 0", code)
	}
	if tuiCommand.ID != "2222" {
		t.Fatalf("TUI command ID = %q, want 2222", tuiCommand.ID)
	}

	var stdout bytes.Buffer
	if code := RunWithDeps([]string{"--json", "--id", "2222"}, &stdout, io.Discard, deps); code != 0 {
		t.Fatalf("JSON selected run exit = %d, want 0; output=%s", code, stdout.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	selected := doc["selected_session"].(map[string]any)
	if selected["session_id"] != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("selected session id = %v", selected["session_id"])
	}
}
```

Use existing `cli_test.go` fixture helpers where possible. If `testDepsWithTwoSessions` does not already exist, create it as a local test helper that mirrors existing fake dependency patterns in that file.

Also add this config-mode regression:

```go
func TestConfigModeDoesNotDiscoverOrParseSessionsThroughSnapshot(t *testing.T) {
	deps := fakeDeps(t)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		t.Fatalf("DiscoverHome called in config mode")
		return session.DiscoveryResult{}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		t.Fatalf("ParseFile called in config mode with %q", path)
		return session.Session{}, nil
	}
	options, err := buildTUIOptions(Command{Mode: ModeConfig}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}
	if options.StartMode != tui.StartConfig {
		t.Fatalf("start mode = %q, want config", options.StartMode)
	}
}
```

- [ ] **Step 2: Refactor `runJSON` to use `snapshot.Build`**

In `internal/app/app.go`:

- Import `github.com/richardchen/cc-cache/internal/snapshot`.
- Replace JSON discovery/parse/config duplication with:

```go
result, err := snapshot.Build(snapshot.Request{
	Home: home,
	Now: now,
	Limit: cmd.Limit,
	ID: cmd.ID,
	Remind: cmd.Remind,
}, snapshot.Loaders{
	LoadConfig: config.Load,
	DiscoverHome: deps.DiscoverHome,
	ParseFile: deps.ParseFile,
})
```

- Map returned operational errors to the same JSON error codes currently used.
- Use the standard pointer target form for `errors.As`:

```go
var buildErr *snapshot.BuildError
if errors.As(err, &buildErr) {
	return writeJSONError(stdout, now, cmd, nil, buildErr.Code, buildErr.Error(), cmd.ID)
}
```

- Convert `snapshot.Result` into `jsonout.State` through a private app helper named `jsonStateFromSnapshot`.
- Preserve `writeJSONError` for hard operational errors.

- [ ] **Step 3: Refactor JSON runtime-state types away from app `map[string]any` construction**

In `internal/jsonout/json.go`, replace `KeepAliveState.Scope any` with:

```go
type KeepAliveScope struct {
	Mode     string `json:"mode"`
	MaxSends int    `json:"max_sends"`
}

type KeepAliveState struct {
	Available  bool
	Enabled    *bool
	AutoSend   *bool
	State      string
	Scope      *KeepAliveScope
	LastResult any
}
```

Keep the external JSON field name `scope` and shape unchanged.

Add a JSON regression in `internal/jsonout/json_test.go`:

```go
func TestKeepAliveScopeUsesPublicSnakeCaseKeys(t *testing.T) {
	enabled := false
	autoSend := true
	data, err := Marshal(State{
		GeneratedAt: time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC),
		Sessions: []session.Session{{
			SessionID: "scope-id",
			ShortID: "scope-id",
			CacheWindow: session.CacheWindow{Known: true, Label: "1h", TTLSeconds: 3600},
		}},
		KeepAlive: map[string]KeepAliveState{
			"scope-id": {
				Available: true,
				Enabled: &enabled,
				AutoSend: &autoSend,
				State: "off",
				Scope: &KeepAliveScope{Mode: "per_session", MaxSends: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	sessionDoc := doc["sessions"].([]any)[0].(map[string]any)
	keepAlive := sessionDoc["keep_alive"].(map[string]any)
	scope := keepAlive["scope"].(map[string]any)
	if _, ok := scope["MaxSends"]; ok {
		t.Fatalf("scope contains Go field name MaxSends: %#v", scope)
	}
	if scope["mode"] != "per_session" || scope["max_sends"].(float64) != 1 {
		t.Fatalf("scope = %#v, want snake-case mode/max_sends", scope)
	}
}
```

- [ ] **Step 4: Run JSON/app focused tests**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/app ./internal/jsonout
```

Expected: pass with unchanged JSON schema assertions.

## Task 4: Route TUI Startup And Refresh Through Snapshot

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/update_test.go`
- Modify: `internal/tui/render_test.go`

- [ ] **Step 1: Add TUI snapshot mapping regressions**

In `internal/app/cli_test.go`, add a no-match startup regression:

```go
func TestTUIIDNoMatchMapsToCurrentEmptyStateBehavior(t *testing.T) {
	deps := fakeDeps(t)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{Sessions: []session.SessionFile{{
			SessionID: "11111111-1111-1111-1111-111111111111",
			ShortID: "11111111",
			Project: "tmp",
			Path: "/tmp/session.jsonl",
		}}}, nil
	}

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5, ID: "zzz"}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}
	if options.SelectedID != "" || options.AmbiguousID != "" {
		t.Fatalf("selected=%q ambiguous=%q, want neither", options.SelectedID, options.AmbiguousID)
	}
	if options.Refresh.EmptyState != tui.EmptyNoSessions {
		t.Fatalf("empty state = %q, want no sessions", options.Refresh.EmptyState)
	}
}
```

Also add an ambiguous startup regression:

```go
func TestTUIAmbiguousIDMapsToAmbiguousRouteCandidates(t *testing.T) {
	deps := fakeDeps(t)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{Sessions: []session.SessionFile{
			{SessionID: "11111111-0000-0000-0000-000000000000", ShortID: "11111111", Project: "one"},
			{SessionID: "11112222-0000-0000-0000-000000000000", ShortID: "11112222", Project: "two"},
		}}, nil
	}

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5, ID: "1111"}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}
	model := tui.NewModel(options)
	if model.Route() != tui.RouteAmbiguous {
		t.Fatalf("route = %q, want ambiguous", model.Route())
	}
	if len(model.Sessions()) != 2 {
		t.Fatalf("candidate sessions = %d, want 2", len(model.Sessions()))
	}
}
```

- [ ] **Step 2: Add TUI selected-refresh regression test**

In `internal/tui/update_test.go`, add a test that proves refresh receives a snapshot and preserves selected session behavior:

```go
func TestRefreshSnapshotAppliesSelectedSessionSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	initial := workspaceSession(now)
	refreshed := initial
	refreshed.Messages.LastUserExcerpt = "fresh selected session"
	model := NewModel(Options{
		Now: now,
		Sessions: []session.Session{initial},
		SelectedID: initial.SessionID,
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if selected == nil || selected.SessionID != initial.SessionID {
					t.Fatalf("selected snapshot input = %#v", selected)
				}
				return RefreshSnapshot{
					Sessions: []session.Session{refreshed},
					Refresh: RefreshViewState{EmptyState: EmptyNone},
					HasRefresh: true,
					SelectedOnly: true,
					SelectedID: initial.SessionID,
				}
			},
		},
	})

	updated, cmd := model.Update(ManualRefreshMsg{})
	if cmd == nil {
		t.Fatalf("manual refresh returned nil command")
	}
	msg := cmd()
	result, ok := msg.(RefreshResultMsg)
	if !ok {
		t.Fatalf("message = %#v, want RefreshResultMsg", msg)
	}
	updated, _ = updated.Update(result)
	got := updated.(Model).Sessions()[0].Messages.LastUserExcerpt
	if got != "fresh selected session" {
		t.Fatalf("last excerpt = %q, want refreshed selected session", got)
	}
}
```

This test proves the TUI callback receives the selected session. The app adapter test below proves selected workspace refresh parses `selected.JSONLPath` directly.

- [ ] **Step 3: Add app selected-refresh-by-path regression**

In `internal/app/cli_test.go`, add:

```go
func TestWorkspaceManualRefreshParsesSelectedJSONLPathOnly(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{Sessions: []session.SessionFile{{
			SessionID: "11111111-1111-1111-1111-111111111111",
			ShortID: "11111111",
			Project: "tmp",
			Path: "/tmp/selected.jsonl",
			ModTime: now,
		}}}, nil
	}
	parseCalls := []string{}
	deps.ParseFile = func(path string) (session.Session, error) {
		parseCalls = append(parseCalls, path)
		return session.Session{
			SessionID: "11111111-1111-1111-1111-111111111111",
			ShortID: "11111111",
			Project: "tmp",
			JSONLPath: path,
			FileModifiedAt: now,
		}, nil
	}

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5, ID: "11111111"}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions: %v", err)
	}
	if len(parseCalls) != 1 || parseCalls[0] != "/tmp/selected.jsonl" {
		t.Fatalf("startup parse calls = %#v", parseCalls)
	}
	parseCalls = nil
	selected := options.Sessions[0]
	snapshot := options.Dependencies.RefreshSnapshot(refresh.SourceManual, 2, &selected)
	if len(parseCalls) != 1 || parseCalls[0] != "/tmp/selected.jsonl" {
		t.Fatalf("refresh parse calls = %#v, want selected path only", parseCalls)
	}
	if !snapshot.SelectedOnly || snapshot.SelectedID != selected.SessionID {
		t.Fatalf("snapshot selected flags = %#v", snapshot)
	}
}
```

- [ ] **Step 4: Narrow TUI refresh dependency interface**

Replace the three refresh callbacks in `internal/tui/model.go`:

```go
RefreshSessions func(source refresh.Source, generation int) []session.Session
RefreshSnapshot func(source refresh.Source, generation int) RefreshSnapshot
RefreshSelectedSnapshot func(source refresh.Source, generation int, selected session.Session) RefreshSnapshot
```

with one callback:

```go
RefreshSnapshot func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot
```

Add these fields to `RefreshSnapshot`:

```go
SelectedOnly bool
SelectedID string
```

- [ ] **Step 5: Refactor `scheduleRefresh`**

`scheduleRefresh` should:

- Increment `refreshGeneration`.
- Capture selected session pointer only when route is workspace.
- Call `m.deps.RefreshSnapshot(source, generation, selected)`.
- Return `RefreshResultMsg` from the snapshot without fallback callback branches.

Delete the fallback branches for `RefreshSessions` and `RefreshSelectedSnapshot`.

- [ ] **Step 6: Refactor `tuiDependencies` to build snapshots**

In `internal/app/app.go`, make `tuiDependencies` return one `RefreshSnapshot` callback:

- When `selected == nil`, call `snapshot.Build` with the original command limit, ID, and Remind values.
- When `selected != nil`, parse only `selected.JSONLPath` by using `deps.ParseFile(selected.JSONLPath)` and return `SelectedOnly: true`, `SelectedID: selected.SessionID`.
- Do not rediscover sessions in the selected workspace refresh path.

- [ ] **Step 7: Run focused TUI/app tests**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/tui ./internal/app
```

Expected: pass.

## Task 5: Delete Historical Dead Code

**Files:**
- Delete: `internal/app/dependencies.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/cli_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/help.go`
- Modify: `internal/tui/help_test.go`
- Modify: `internal/tui/update_test.go`

- [ ] **Step 1: Delete app dependency anchors and unused app dependency fields**

Remove:

```go
StartWatcher                 func() error
StartNotifier                func() error
NewKeepAliveRunner           func() error
```

from `internal/app.Dependencies`.

Remove tests that only assert those unused hooks are not called. Keep tests that prove JSON mode does not start TUI or KeepAlive automation.

- [ ] **Step 2: Delete unused TUI dependency hooks**

Remove from `internal/tui.Dependencies`:

```go
Discover func()
Parse    func()
Refresh  func()
```

Update any tests that used these as counters to assert the new snapshot callback instead.

- [ ] **Step 3: Remove `SafetyRefreshMsg` if tests prove it is only a test seam**

Delete:

```go
type SafetyRefreshMsg struct{}
```

and the `case SafetyRefreshMsg:` update branch.

Replace tests that send `SafetyRefreshMsg{}` with `RefreshTickMsg{Now: now}` or direct `ManualRefreshMsg{}` depending on the behavior under test.

- [ ] **Step 4: Remove legacy `KeepAliveStatus`**

Delete `KeepAliveStatus` constants and `Options.KeepAliveStatus`.

Rewrite `keepAliveHelpText` to inspect `m.activeKeepAliveState().State`:

```go
func (m Model) keepAliveHelpText() string {
	switch m.activeKeepAliveState().State {
	case keepalive.StateCountdown:
		return "KeepAlive countdown remains visible\ns send KeepAlive now\nx cancel/dismiss current KeepAlive instance\n"
	case keepalive.StateConfirming, keepalive.StateSending:
		return "KeepAlive confirming remains visible\nx cancel/dismiss current KeepAlive instance\n"
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return "KeepAlive failure remains visible\nx cancel/dismiss current KeepAlive instance\n"
	default:
		return ""
	}
}
```

Update help tests to seed `KeepAliveStates` instead of `KeepAliveStatus`.

- [ ] **Step 5: Remove historical `evidenceOffset`**

Delete `evidenceOffset` from `Model`. No replacement field should be added.

- [ ] **Step 6: Verify dead-code deletion**

Run:

```bash
rg -n "dependencyAnchors|StartWatcher|StartNotifier|NewKeepAliveRunner|Discover\\s+func|Parse\\s+func|Refresh\\s+func|SafetyRefreshMsg|KeepAliveStatus|evidenceOffset" internal cmd
```

Expected: no matches except this plan file if searching the whole repo.

- [ ] **Step 7: Run focused tests**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/app ./internal/tui
```

Expected: pass.

## Task 6: Create Domain Glossary

**Files:**
- Create: `CONTEXT.md`

- [ ] **Step 1: Add small domain glossary**

Create `CONTEXT.md`:

```markdown
# cc-cache Domain Context

## Terms

### Session Snapshot

The current parsed view of Claude Code cache sessions plus config-derived runtime defaults. It is the source consumed by JSON output, TUI startup, and TUI refresh.

### Cache Status

The user-facing state of one session cache window: active, expired, or unknown, with TTL timing and cache tier.

### Session Info

The user-facing session detail summary: IDs, message excerpts, token stats, and mid-session gaps.

### Refresh Runtime

The internal TUI mechanism that updates session snapshots from manual update, filesystem events, and safety ticks. It is not a public `--watch` mode.

### KeepAlive Runtime

The bounded, visible, cancellable automation state for optionally sending a configured Claude message to one selected session.

### Route Module

A TUI route's local focus, render, and action behavior. List, Workspace, Ambiguous, and Config are route modules.
```

- [ ] **Step 2: Verify glossary is concise**

Run:

```bash
wc -l CONTEXT.md
```

Expected: line count under 80.

## Task 7: Documentation And Progress Wiring

**Files:**
- Modify: `docs/superpowers/plans/cc-cache-v2/PLAN.md`
- Modify: `docs/superpowers/progress/cc-cache-v2-progress.md`

- [ ] **Step 1: Add Phase 11.8 to plan index**

In `docs/superpowers/plans/cc-cache-v2/PLAN.md`, add Phase 11.8 between Phase 11.7 and Phase 12:

```markdown
- [Phase 11.8: Architecture Refactor](phase-11.8-architecture-refactor.md)
```

- [ ] **Step 2: Update progress current state**

In `docs/superpowers/progress/cc-cache-v2-progress.md`, set:

```markdown
- Current phase: Phase 11.8 - Architecture Refactor
- Current phase file: `docs/superpowers/plans/cc-cache-v2/phase-11.8-architecture-refactor.md`
- Current step: Phase 11.8 implementation in progress
- Status: in progress
- Last updated: 2026-06-16
```

Add Phase 11.8 to the phase checklist between 11.7 and 12.

- [ ] **Step 3: Add verification ledger row before stopping**

After implementation verification succeeds, add a ledger row summarizing:

- snapshot tests;
- app/jsonout focused tests;
- app/tui focused tests;
- full `go test -count=1 ./...`;
- `git diff --check`;
- build of a throwaway binary under `/private/tmp`.

## Task 8: Final Verification

**Files:**
- No new files.

- [ ] **Step 1: Run all focused tests**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/snapshot ./internal/app ./internal/jsonout ./internal/tui
```

Expected: pass.

- [ ] **Step 2: Run full suite**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test -count=1 ./...
```

Expected: pass.

- [ ] **Step 3: Run whitespace check**

Run:

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 4: Build throwaway binary**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go build -o /private/tmp/cc-cache-phase118/cc-cache ./cmd/cc-cache
```

Expected: command exits 0 and writes `/private/tmp/cc-cache-phase118/cc-cache`.

- [ ] **Step 5: JSON smoke**

Run:

```bash
HOME="$PWD/internal/session/testdata/smoke-home" /private/tmp/cc-cache-phase118/cc-cache --json
```

Expected: exits 0, emits `schema_version: 1`, and includes `sessions` plus `error: null`.

- [ ] **Step 6: Confirm no forbidden actions occurred**

Run:

```bash
git status --short
```

Expected: only intended source/doc changes. No `$HOME/.local/bin/cc-cache` replacement, no release artifacts in repo, no real Claude send.

## Implementation Order

1. Snapshot red tests.
2. Snapshot implementation.
3. App/JSON consumption.
4. TUI refresh consumption.
5. Dead-code deletion.
6. Glossary.
7. Plan/progress updates.
8. Full verification.

Do not start Task 5 deletion until Tasks 2-4 pass. Deletion before the snapshot seam is stable risks removing test scaffolding without reducing implementation complexity.
