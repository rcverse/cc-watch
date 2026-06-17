# cc-cache v2 Phase 11.9 Architecture Simplification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce shallow package/file scattering before Phase 12 while preserving CLI behavior, JSON schema, config schema, parser semantics, and KeepAlive safety.

**Architecture:** Keep deep modules that earn their interface: `internal/session`, `internal/snapshot`, `internal/config`, `internal/keepalive`, `internal/jsonout`, and `internal/tui`. Remove or collapse shallow modules where the interface is nearly as complex as the implementation: the unused refresh coordinator/watcher runtime, the standalone reminder package, and tiny notification platform files. Prefer fewer live packages/files over speculative seams, but do not merge safety-sensitive KeepAlive or public JSON contract code.

**Tech Stack:** Go 1.23+, existing Go tests, Bubble Tea/Lip Gloss TUI, standard library subprocess/notification command construction, no new dependencies.

---

## Baseline And Constraints

- Start from completed Phase 11.8 on branch `codex/phase-11.8-architecture-refactor`.
- Do not change public CLI flags or exit-code behavior.
- Do not change JSON `schema_version` or public field names.
- Do not change config file schema.
- Do not change parser metrics or fixture semantics.
- Do not run a real Claude KeepAlive send.
- Do not replace `$HOME/.local/bin/cc-cache`.
- Do not start Phase 12 packaging, installer, release, or Homebrew work.
- Do not add dependencies.

## Target Shape

After Phase 11.9:

- `internal/refresh` contains only the small refresh status/source vocabulary still consumed by JSON/TUI, or is removed if those types are moved without cycles.
- `internal/reminder` is deleted. Reminder threshold runtime lives in `internal/tui`; JSON reminder event shape lives in `internal/jsonout`.
- `internal/notify` remains as one package, but its tiny platform-command implementations are consolidated into one production file.
- `internal/app`, `internal/tui`, `internal/snapshot`, `internal/session`, `internal/config`, `internal/jsonout`, and `internal/keepalive` remain separate packages.
- `docs/superpowers/progress/cc-cache-v2-progress.md` records Phase 11.9 verification and still stops before Phase 12.

## File Structure

### Create

- `internal/tui/reminder_runtime.go` - TUI-local Reminder runtime state and threshold event calculation.
- `internal/tui/reminder_runtime_test.go` - focused tests moved from `internal/reminder`.
- `internal/refresh/state.go` - retained refresh `Source`, `Status`, and `State` vocabulary if `refresh.go` is deleted.

### Modify

- `internal/jsonout/json.go` - replace `reminder.Event` import with local `ReminderEvent`.
- `internal/jsonout/json_test.go` - update tests to use `jsonout.ReminderEvent`.
- `internal/tui/model.go` - add a `reminderManager *reminderRuntime` field if runtime locality is cleaner than the existing maps.
- `internal/tui/update.go` - route Reminder threshold checks through the TUI-local runtime.
- `internal/tui/update_test.go` - keep visible Reminder behavior assertions passing.
- `internal/notify/notifier.go` - include command notifier, macOS command, Linux command, and unsupported notifier implementations.
- `internal/notify/notifier_test.go` - keep existing command escaping, Linux argument, unsupported notifier, suppression, and wording tests.
- `docs/superpowers/plans/cc-cache-v2/PLAN.md` - add Phase 11.9 to the phase index.
- `docs/superpowers/progress/cc-cache-v2-progress.md` - mark Phase 11.9 active/in-progress during execution and complete after verification.

### Delete

- `internal/reminder/reminder.go`
- `internal/reminder/reminder_test.go`
- `internal/refresh/refresh.go` if production code only needs `Source`, `Status`, and `State`
- `internal/refresh/watcher.go` if no production app path starts a watcher
- `internal/refresh/fsnotify.go` if no production app path starts a watcher
- `internal/refresh/refresh_test.go` after equivalent retained-type tests are covered elsewhere
- `internal/notify/command.go`
- `internal/notify/macos.go`
- `internal/notify/linux.go`
- `internal/notify/noop.go`

## Task 1: Confirm Shallow Runtime Targets

**Files:**
- Modify: none
- Test: none

- [ ] **Step 1: Confirm current package/file counts**

Run:

```bash
find internal -maxdepth 2 -type f -name '*.go' | sort | xargs wc -l
```

Expected: output includes `internal/reminder`, `internal/refresh`, and four tiny `internal/notify` implementation files.

- [ ] **Step 2: Confirm refresh watcher/coordinator are not used by production code**

Run:

```bash
rg -n "NewCoordinator|Coordinator|NewWatcher|NewFSNotifyFS|WatcherEventsMsg|WatcherDegradedMsg|RawEvent|NormalizedEvent" internal cmd --glob '*.go'
```

Expected: matches are limited to `internal/refresh/*` and tests. `internal/app` must not start a watcher.

- [ ] **Step 3: Confirm reminder package is not used by app or TUI**

Run:

```bash
rg -n 'internal/reminder|reminder\.' internal cmd --glob '*.go'
```

Expected before cleanup: matches only in `internal/jsonout/json.go`, `internal/jsonout/json_test.go`, and `internal/reminder/*`.

- [ ] **Step 4: Confirm notify tiny files are simple implementations**

Run:

```bash
wc -l internal/notify/command.go internal/notify/macos.go internal/notify/linux.go internal/notify/noop.go internal/notify/notifier.go
```

Expected: `macos.go`, `linux.go`, and `noop.go` are small enough that keeping separate files adds navigation cost without meaningful locality.

## Task 2: Remove Standalone Reminder Package

**Files:**
- Create: `internal/tui/reminder_runtime.go`
- Create: `internal/tui/reminder_runtime_test.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/jsonout/json.go`
- Modify: `internal/jsonout/json_test.go`
- Delete: `internal/reminder/reminder.go`
- Delete: `internal/reminder/reminder_test.go`

- [ ] **Step 1: Write failing JSON test for local Reminder event type**

In `internal/jsonout/json_test.go`, replace the `internal/reminder` import with no reminder import, and change the fired event fixture inside `TestSessionObjectShapeIncludesParserReminderAndKeepAliveState` to:

```go
Fired: []ReminderEvent{{
	Kind:             "reminder_threshold_crossed",
	SessionID:        s.SessionID,
	ThresholdPercent: 20,
	RemainingPercent: 8.33,
	OccurredAt:       now,
}},
```

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/jsonout -run TestSessionObjectShapeIncludesParserReminderAndKeepAliveState
```

Expected: fail with `undefined: ReminderEvent`.

- [ ] **Step 2: Add `jsonout.ReminderEvent`**

In `internal/jsonout/json.go`, remove:

```go
"github.com/richardchen/cc-cache/internal/reminder"
```

Replace:

```go
Fired      []reminder.Event
```

with:

```go
Fired      []ReminderEvent
```

Add this type near `ReminderState`:

```go
type ReminderEvent struct {
	Kind             string
	SessionID        string
	ThresholdPercent int
	RemainingPercent float64
	OccurredAt       time.Time
}
```

Run:

```bash
gofmt -w internal/jsonout/json.go internal/jsonout/json_test.go
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/jsonout -run TestSessionObjectShapeIncludesParserReminderAndKeepAliveState
```

Expected: pass.

- [ ] **Step 3: Write TUI-local Reminder runtime tests**

Create `internal/tui/reminder_runtime_test.go`:

```go
package tui

import (
	"testing"
	"time"

	"github.com/richardchen/cc-cache/internal/session"
)

func TestReminderRuntimeToggleControlsPerSessionReminder(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	s := reminderRuntimeActiveSession("session-a", now.Add(-55*time.Minute), 3600)
	runtime := newReminderRuntime([]int{20, 10})

	if runtime.enabled(s.SessionID) {
		t.Fatal("new runtime has reminder enabled, want disabled")
	}
	if events := runtime.check(now, []session.Session{s}); len(events) != 0 {
		t.Fatalf("disabled session emitted events: %#v", events)
	}

	runtime.enable(s.SessionID)
	if !runtime.enabled(s.SessionID) {
		t.Fatal("enable did not mark session enabled")
	}
	if events := runtime.check(now, []session.Session{s}); len(events) == 0 {
		t.Fatal("enabled session emitted no events")
	}

	runtime.disable(s.SessionID)
	if runtime.enabled(s.SessionID) {
		t.Fatal("disable left session enabled")
	}
}

func TestReminderRuntimeThresholdsComeFromConfig(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	s := reminderRuntimeActiveSession("session-a", now.Add(-45*time.Minute), 3600)
	runtime := newReminderRuntime([]int{30, 25})
	runtime.enable(s.SessionID)

	events := runtime.check(now, []session.Session{s})
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2: %#v", len(events), events)
	}
	if events[0].thresholdPercent != 30 || events[1].thresholdPercent != 25 {
		t.Fatalf("thresholds = %d,%d; want 30,25", events[0].thresholdPercent, events[1].thresholdPercent)
	}
}

func TestReminderRuntimeFiresOncePerThreshold(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	s := reminderRuntimeActiveSession("session-a", now.Add(-55*time.Minute), 3600)
	runtime := newReminderRuntime([]int{20, 10})
	runtime.enable(s.SessionID)

	first := runtime.check(now, []session.Session{s})
	if len(first) != 2 {
		t.Fatalf("first len = %d, want 2: %#v", len(first), first)
	}
	second := runtime.check(now.Add(30*time.Second), []session.Session{s})
	if len(second) != 0 {
		t.Fatalf("second check fired duplicate events: %#v", second)
	}
}

func reminderRuntimeActiveSession(id string, lastMessageAt time.Time, ttlSeconds int) session.Session {
	return session.Session{
		SessionID:     id,
		ShortID:       id,
		Project:       "tmp",
		LastMessageAt: &lastMessageAt,
		CacheWindow: session.CacheWindow{
			Tier:       session.Tier1Hour,
			Label:      "1h",
			TTLSeconds: ttlSeconds,
			Known:      true,
		},
	}
}
```

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/tui -run 'TestReminderRuntime'
```

Expected: fail with `undefined: newReminderRuntime`.

- [ ] **Step 4: Implement TUI-local Reminder runtime**

Create `internal/tui/reminder_runtime.go`:

```go
package tui

import (
	"time"

	"github.com/richardchen/cc-cache/internal/session"
)

type reminderRuntime struct {
	thresholds []int
	sessions   map[string]reminderSessionState
}

type reminderSessionState struct {
	enabled bool
	fired   map[int]bool
}

type reminderRuntimeEvent struct {
	sessionID        string
	thresholdPercent int
	remainingPercent float64
	occurredAt       time.Time
}

func newReminderRuntime(thresholds []int) *reminderRuntime {
	return &reminderRuntime{
		thresholds: append([]int(nil), thresholds...),
		sessions:   map[string]reminderSessionState{},
	}
}

func (r *reminderRuntime) enable(sessionID string) {
	state := r.sessions[sessionID]
	state.enabled = true
	if state.fired == nil {
		state.fired = map[int]bool{}
	}
	r.sessions[sessionID] = state
}

func (r *reminderRuntime) disable(sessionID string) {
	state := r.sessions[sessionID]
	state.enabled = false
	if state.fired == nil {
		state.fired = map[int]bool{}
	}
	r.sessions[sessionID] = state
}

func (r *reminderRuntime) enabled(sessionID string) bool {
	return r.sessions[sessionID].enabled
}

func (r *reminderRuntime) check(now time.Time, sessions []session.Session) []reminderRuntimeEvent {
	var events []reminderRuntimeEvent
	for _, s := range sessions {
		state := r.sessions[s.SessionID]
		if !state.enabled || state.fired == nil {
			continue
		}
		status := s.StatusAt(now)
		if status.State != session.StatusActive || status.RemainingSeconds == nil || s.CacheWindow.TTLSeconds <= 0 {
			continue
		}
		remainingPercent := float64(*status.RemainingSeconds) / float64(s.CacheWindow.TTLSeconds) * 100
		for _, threshold := range r.thresholds {
			if remainingPercent > float64(threshold) || state.fired[threshold] {
				continue
			}
			events = append(events, reminderRuntimeEvent{
				sessionID:        s.SessionID,
				thresholdPercent: threshold,
				remainingPercent: remainingPercent,
				occurredAt:       now,
			})
			state.fired[threshold] = true
		}
		r.sessions[s.SessionID] = state
	}
	return events
}
```

Run:

```bash
gofmt -w internal/tui/reminder_runtime.go internal/tui/reminder_runtime_test.go
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/tui -run 'TestReminderRuntime'
```

Expected: pass.

- [ ] **Step 5: Delete standalone reminder package**

Run:

```bash
rm internal/reminder/reminder.go internal/reminder/reminder_test.go
rmdir internal/reminder
```

Run:

```bash
rg -n 'internal/reminder|reminder\.' internal cmd --glob '*.go'
```

Expected: no matches.

- [ ] **Step 6: Run focused tests**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/jsonout ./internal/tui
```

Expected: pass.

## Task 3: Prune Unused Refresh Runtime

**Files:**
- Create: `internal/refresh/state.go`
- Modify: `internal/jsonout/json.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/update_test.go`
- Delete: `internal/refresh/refresh.go`
- Delete: `internal/refresh/watcher.go`
- Delete: `internal/refresh/fsnotify.go`
- Delete: `internal/refresh/refresh_test.go`

- [ ] **Step 1: Add retained refresh vocabulary file**

Create `internal/refresh/state.go`:

```go
package refresh

type Source string

const (
	SourceFsnotify Source = "fsnotify"
	SourceSafety   Source = "safety"
	SourceManual   Source = "manual"
)

type Status string

const (
	StatusOK       Status = "ok"
	StatusPartial  Status = "partial"
	StatusDegraded Status = "degraded"
)

type State struct {
	Status              Status
	Messages            []string
	SafetyRefreshActive bool
}
```

Run:

```bash
gofmt -w internal/refresh/state.go
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/refresh
```

Expected: fail with redeclared `Source`, `Status`, and `State` because `refresh.go` and `watcher.go` still define them.

- [ ] **Step 2: Delete unused refresh runtime files**

Run:

```bash
rm internal/refresh/refresh.go internal/refresh/watcher.go internal/refresh/fsnotify.go internal/refresh/refresh_test.go
```

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/refresh
```

Expected: `? github.com/richardchen/cc-cache/internal/refresh [no test files]`.

- [ ] **Step 3: Confirm production imports still compile**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/jsonout ./internal/tui ./internal/app
```

Expected: pass.

- [ ] **Step 4: Confirm removed refresh runtime symbols are gone**

Run:

```bash
rg -n "NewCoordinator|Coordinator|NewWatcher|NewFSNotifyFS|WatcherEventsMsg|WatcherDegradedMsg|RawEvent|NormalizedEvent|WatchFS" internal cmd --glob '*.go'
```

Expected: no matches.

## Task 4: Consolidate Notification Platform Files

**Files:**
- Modify: `internal/notify/notifier.go`
- Delete: `internal/notify/command.go`
- Delete: `internal/notify/macos.go`
- Delete: `internal/notify/linux.go`
- Delete: `internal/notify/noop.go`
- Test: `internal/notify/notifier_test.go`

- [ ] **Step 1: Run existing notify tests before consolidation**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/notify
```

Expected: pass.

- [ ] **Step 2: Move command notifier code into `notifier.go`**

Append these definitions to `internal/notify/notifier.go` after `NewPlatformNotifier`:

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

Add these imports to the existing import block:

```go
"os/exec"
"strings"
```

Run:

```bash
gofmt -w internal/notify/notifier.go
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/notify
```

Expected: fail with redeclared command notifier symbols until old files are deleted.

- [ ] **Step 3: Move platform command and unsupported notifier code into `notifier.go`**

Append these definitions to `internal/notify/notifier.go`:

```go
func MacOSCommand(notification Notification) (string, []string) {
	script := `display notification "` + escapeAppleScript(notification.Body) + `" with title "` + escapeAppleScript(notification.Title) + `"`
	return "osascript", []string{"-e", script}
}

func escapeAppleScript(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func LinuxCommand(notification Notification) (string, []string) {
	return "notify-send", []string{notification.Title, notification.Body}
}

type UnsupportedNotifier struct {
	GOOS string
}

func (n UnsupportedNotifier) Notify(Event) Result {
	message := fmt.Sprintf("notifications unsupported on %s", n.GOOS)
	return Result{
		Degraded: true,
		Message:  message,
		Err:      errors.New(message),
	}
}
```

Run:

```bash
gofmt -w internal/notify/notifier.go
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/notify
```

Expected: fail with redeclared platform symbols until old files are deleted.

- [ ] **Step 4: Delete tiny notification implementation files**

Run:

```bash
rm internal/notify/command.go internal/notify/macos.go internal/notify/linux.go internal/notify/noop.go
```

Run:

```bash
gofmt -w internal/notify/notifier.go
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/notify
```

Expected: pass.

## Task 5: Documentation And Progress Wiring

**Files:**
- Modify: `docs/superpowers/plans/cc-cache-v2/PLAN.md`
- Modify: `docs/superpowers/progress/cc-cache-v2-progress.md`
- Modify: `CONTEXT.md`

- [ ] **Step 1: Add Phase 11.9 to plan index**

In `docs/superpowers/plans/cc-cache-v2/PLAN.md`, add this line between Phase 11.8 and Phase 12:

```markdown
- [Phase 11.9: Architecture Simplification](phase-11.9-architecture-simplification.md)
```

- [ ] **Step 2: Update progress current state during execution**

At the start of execution, set `docs/superpowers/progress/cc-cache-v2-progress.md` current state to:

```markdown
- Current phase: Phase 11.9 - Architecture Simplification
- Current phase file: `docs/superpowers/plans/cc-cache-v2/phase-11.9-architecture-simplification.md`
- Current step: Phase 11.9 implementation in progress
- Status: in progress
- Last updated: 2026-06-17
```

Add this unchecked phase row between Phase 11.8 and Phase 12:

```markdown
- [ ] Phase 11.9 - Architecture Simplification
```

- [ ] **Step 3: Add glossary note for simplified modules**

Append this section to `CONTEXT.md`:

```markdown
### Architecture Simplification

A cleanup pass that removes shallow modules and historical runtime seams after tests prove the behavior is either dead or better owned by an existing deep module.
```

Run:

```bash
wc -l CONTEXT.md
```

Expected: line count remains under 80.

## Task 6: Final Verification

**Files:**
- No new source files beyond earlier tasks.

- [ ] **Step 1: Run focused package tests**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test ./internal/app ./internal/jsonout ./internal/tui ./internal/notify ./internal/refresh ./internal/snapshot
```

Expected: pass.

- [ ] **Step 2: Run full suite**

Run:

```bash
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go test -count=1 ./...
```

Expected: pass.

- [ ] **Step 3: Run dead-symbol scans**

Run:

```bash
rg -n 'internal/reminder|reminder\.|NewCoordinator|Coordinator|NewWatcher|NewFSNotifyFS|WatcherEventsMsg|WatcherDegradedMsg|RawEvent|NormalizedEvent|WatchFS' internal cmd --glob '*.go'
```

Expected: no matches.

- [ ] **Step 4: Run whitespace check**

Run:

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 5: Build throwaway binary**

Run:

```bash
mkdir -p /private/tmp/cc-cache-phase119
GOCACHE=/private/tmp/cc-cache-go-build GOMODCACHE=/private/tmp/cc-cache-go-mod go build -o /private/tmp/cc-cache-phase119/cc-cache ./cmd/cc-cache
```

Expected: command exits 0 and writes `/private/tmp/cc-cache-phase119/cc-cache`.

- [ ] **Step 6: JSON smoke**

Run:

```bash
HOME="$PWD/internal/session/testdata/smoke-home" /private/tmp/cc-cache-phase119/cc-cache --json
```

Expected: exits 0, emits `schema_version: 1`, includes `sessions`, and has `"error": null`.

- [ ] **Step 7: Confirm no forbidden actions occurred**

Run:

```bash
git status --short
```

Expected: only intended source/doc changes. No `$HOME/.local/bin/cc-cache` replacement, no release artifacts in repo, no real Claude send.

## Self-Review Notes

- Spec coverage: the plan addresses the user concern about scattered files by deleting one shallow package, pruning dead refresh runtime files, and consolidating tiny notify files. It deliberately keeps deep modules that provide locality for parser, snapshot, config, TUI, JSON contract, and KeepAlive safety.
- Placeholder scan: no task uses open-ended implementation placeholders; every code-changing step names the exact file, code shape, command, and expected result.
- Type consistency: `jsonout.ReminderEvent`, `reminderRuntime`, `refresh.Source`, `refresh.Status`, and `refresh.State` are named consistently across tasks.
