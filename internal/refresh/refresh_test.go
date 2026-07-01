package refresh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/richardchen/cc-watch/internal/session"
)

func TestWatcherSetupRegistersRootAndExistingProjectDirectories(t *testing.T) {
	projectsDir := "/tmp/home/.claude/projects"
	fs := &fakeWatchFS{
		dirs: []string{
			projectsDir,
			projectsDir + "/-tmp-alpha",
			projectsDir + "/-tmp-alpha/nested",
			projectsDir + "/-tmp-beta",
		},
	}

	watcher, err := NewWatcher(projectsDir, fs)
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	for _, want := range []string{projectsDir, projectsDir + "/-tmp-alpha", projectsDir + "/-tmp-alpha/nested", projectsDir + "/-tmp-beta"} {
		if !watcher.Watched(want) {
			t.Fatalf("watcher did not register %q; watched=%#v", want, watcher.WatchedPaths())
		}
	}
	if watcher.State().Status != StatusOK {
		t.Fatalf("status = %q, want ok: %#v", watcher.State().Status, watcher.State())
	}
}

func TestWatcherSetupFailureAndPartialFailureProduceDegradedState(t *testing.T) {
	projectsDir := "/tmp/home/.claude/projects"
	fs := &fakeWatchFS{
		dirs:       []string{projectsDir, projectsDir + "/-tmp-alpha", projectsDir + "/-tmp-denied"},
		watchErrs:  map[string]error{projectsDir + "/-tmp-denied": errors.New("permission denied")},
		fatalWatch: map[string]bool{projectsDir: false},
	}

	watcher, err := NewWatcher(projectsDir, fs)
	if err != nil {
		t.Fatalf("NewWatcher returned error for partial failure: %v", err)
	}
	state := watcher.State()
	if state.Status != StatusPartial {
		t.Fatalf("status = %q, want partial: %#v", state.Status, state)
	}
	if !strings.Contains(strings.Join(state.Messages, "\n"), "-tmp-denied") || !strings.Contains(strings.Join(state.Messages, "\n"), "permission denied") {
		t.Fatalf("messages do not include denied path and reason: %#v", state.Messages)
	}

	_, err = NewWatcher(projectsDir, &fakeWatchFS{
		dirs:       []string{projectsDir},
		watchErrs:  map[string]error{projectsDir: errors.New("root unavailable")},
		fatalWatch: map[string]bool{projectsDir: true},
	})
	if err == nil {
		t.Fatal("NewWatcher returned nil error for root watch failure")
	}
}

func TestWatcherNormalizesEventsAndAddsCreatedDirectories(t *testing.T) {
	projectsDir := "/tmp/home/.claude/projects"
	fs := &fakeWatchFS{
		dirs: []string{projectsDir},
		isDir: map[string]bool{
			projectsDir + "/-tmp-new": true,
		},
	}
	watcher, err := NewWatcher(projectsDir, fs)
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	events := []RawEvent{
		{Path: projectsDir + "/-tmp-new", Op: OpCreate},
		{Path: projectsDir + "/-tmp-new/session.jsonl", Op: OpWrite},
		{Path: projectsDir + "/-tmp-new/session.jsonl", Op: OpRename},
		{Path: projectsDir + "/-tmp-new/session.jsonl", Op: OpDelete},
	}
	normalized := watcher.Normalize(events)

	if !watcher.Watched(projectsDir + "/-tmp-new") {
		t.Fatalf("created directory was not watched: %#v", watcher.WatchedPaths())
	}
	if len(normalized) != 4 {
		t.Fatalf("len(normalized) = %d, want 4", len(normalized))
	}
	for i, want := range []EventKind{EventCreated, EventWritten, EventRenamed, EventDeleted} {
		if normalized[i].Kind != want {
			t.Fatalf("event %d kind = %q, want %q", i, normalized[i].Kind, want)
		}
	}
}

func TestWatcherNextReturnsDomainResultWithoutTeaDependency(t *testing.T) {
	events := make(chan RawEvent, 1)
	errs := make(chan error, 1)
	projectsDir := t.TempDir()
	fs := &fakeWatchFS{
		dirs:   []string{projectsDir},
		events: events,
		errs:   errs,
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

func TestNewDirectoryWatchFailureDegradesWithoutStoppingSafetyRefresh(t *testing.T) {
	projectsDir := "/tmp/home/.claude/projects"
	newDir := projectsDir + "/-tmp-new"
	fs := &fakeWatchFS{
		dirs:      []string{projectsDir},
		isDir:     map[string]bool{newDir: true},
		watchErrs: map[string]error{newDir: errors.New("permission denied")},
	}
	watcher, err := NewWatcher(projectsDir, fs)
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	_ = watcher.Normalize([]RawEvent{{Path: newDir, Op: OpCreate}})
	state := watcher.State()
	if state.Status != StatusPartial {
		t.Fatalf("status = %q, want partial after new dir failure", state.Status)
	}
	if !state.SafetyRefreshActive {
		t.Fatal("SafetyRefreshActive = false, want true after watcher degradation")
	}
}

func TestWatcherCloseOrErrorAfterStartupDegradesState(t *testing.T) {
	watcher, err := NewWatcher("/tmp/projects", &fakeWatchFS{dirs: []string{"/tmp/projects"}})
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	watcher.MarkRuntimeError(errors.New("watcher closed"))

	state := watcher.State()
	if state.Status != StatusDegraded {
		t.Fatalf("status = %q, want degraded", state.Status)
	}
	if !strings.Contains(strings.Join(state.Messages, "\n"), "watcher closed") {
		t.Fatalf("messages = %#v, want runtime error", state.Messages)
	}
}

func TestWatcherNextClosedChannelsReturnClosedResult(t *testing.T) {
	events := make(chan RawEvent)
	errs := make(chan error)
	close(events)
	close(errs)
	watcher, err := NewWatcher("/tmp/projects", &fakeWatchFS{
		dirs:   []string{"/tmp/projects"},
		events: events,
		errs:   errs,
	})
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

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
}

func TestFSNotifyCloseClosesForwardedEventsEvenWithoutReceiver(t *testing.T) {
	fs, err := NewFSNotifyFS()
	if err != nil {
		t.Fatalf("NewFSNotifyFS returned error: %v", err)
	}
	dir := t.TempDir()
	if err := fs.Watch(dir); err != nil {
		t.Fatalf("Watch returned error: %v", err)
	}
	for i := 0; i < 64; i++ {
		path := filepath.Join(dir, "session.jsonl")
		if err := os.WriteFile(path, []byte("event"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	if err := fs.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-fs.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("Events channel did not close after Close")
		}
	}
}

func TestForwarderClosesBothOutputsWhenEitherInputClosesFirst(t *testing.T) {
	tests := []struct {
		name       string
		closeFirst string
	}{
		{name: "events first", closeFirst: "events"},
		{name: "errors first", closeFirst: "errors"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawEvents := make(chan fsnotify.Event)
			rawErrs := make(chan error)
			events := make(chan RawEvent, 1)
			errs := make(chan error, 1)
			done := make(chan struct{})
			go func() {
				forwardWatchStreams(rawEvents, rawErrs, events, errs)
				close(done)
			}()

			if tt.closeFirst == "events" {
				close(rawEvents)
				close(rawErrs)
			} else {
				close(rawErrs)
				close(rawEvents)
			}

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("forwarder did not exit")
			}
			assertClosedRawEventChannel(t, events)
			assertClosedErrorChannel(t, errs)
		})
	}
}

func assertClosedRawEventChannel(t *testing.T, ch <-chan RawEvent) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("events channel still open")
		}
	default:
		t.Fatal("events channel not closed")
	}
}

func assertClosedErrorChannel(t *testing.T, ch <-chan error) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("errors channel still open")
		}
	default:
		t.Fatal("errors channel not closed")
	}
}

func TestCoordinatorDebounceSafetyManualAndGenerationOrdering(t *testing.T) {
	parser := &fakeParser{
		results: map[string][]session.Session{
			"fsnotify": {{SessionID: "debounced"}},
			"safety":   {{SessionID: "safety"}},
			"manual":   {{SessionID: "manual"}},
		},
	}
	coordinator := NewCoordinator(Options{
		Debounce:        200 * time.Millisecond,
		SafetyInterval:  time.Minute,
		Parser:          parser.Parse,
		ProjectsDir:     "/tmp/projects",
		InitialNow:      time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		InitialSessions: []session.Session{{SessionID: "old"}},
	})

	first := coordinator.OnWatcherEvents([]NormalizedEvent{{Kind: EventWritten, Path: "/tmp/a.jsonl"}})
	second := coordinator.OnWatcherEvents([]NormalizedEvent{{Kind: EventWritten, Path: "/tmp/b.jsonl"}})
	if first.ShouldRefresh || second.ShouldRefresh {
		t.Fatalf("debounced watcher events refreshed immediately: first=%#v second=%#v", first, second)
	}
	if coordinator.PendingDebounceCount() != 1 {
		t.Fatalf("pending debounce count = %d, want coalesced 1", coordinator.PendingDebounceCount())
	}
	if first.DebounceToken == second.DebounceToken {
		t.Fatalf("debounce token did not advance: first=%d second=%d", first.DebounceToken, second.DebounceToken)
	}

	stale := coordinator.OnDebounceElapsed(time.Date(2026, 6, 4, 12, 0, 1, 0, time.UTC), first.DebounceToken)
	if stale.ShouldRefresh {
		t.Fatalf("stale debounce decision = %#v, want no refresh", stale)
	}

	debounced := coordinator.OnDebounceElapsed(time.Date(2026, 6, 4, 12, 0, 1, 0, time.UTC), second.DebounceToken)
	if !debounced.ShouldRefresh || debounced.Source != SourceFsnotify || debounced.Generation != 1 {
		t.Fatalf("debounced decision = %#v, want fsnotify generation 1", debounced)
	}
	safety := coordinator.OnSafetyTick(time.Date(2026, 6, 4, 12, 1, 0, 0, time.UTC))
	if !safety.ShouldRefresh || safety.Source != SourceSafety || safety.Generation != 2 {
		t.Fatalf("safety decision = %#v, want safety generation 2", safety)
	}
	manual := coordinator.OnManualRefresh()
	if !manual.ShouldRefresh || manual.Source != SourceManual || manual.Generation != 3 {
		t.Fatalf("manual decision = %#v, want manual generation 3", manual)
	}
	if !manual.BypassedDebounce {
		t.Fatalf("manual BypassedDebounce = false, want true")
	}

	coordinator.ApplyResult(Result{Generation: manual.Generation, Sessions: parser.Parse("manual")})
	coordinator.ApplyResult(Result{Generation: debounced.Generation, Sessions: parser.Parse("fsnotify")})
	if got := coordinator.Sessions(); len(got) != 1 || got[0].SessionID != "manual" {
		t.Fatalf("stale debounced result overwrote manual result: %#v", got)
	}
}

func TestOlderParseResultCannotResurrectDeletedOrRenamedSession(t *testing.T) {
	coordinator := NewCoordinator(Options{
		Parser:          (&fakeParser{}).Parse,
		ProjectsDir:     "/tmp/projects",
		InitialSessions: []session.Session{{SessionID: "deleted"}},
	})

	old := coordinator.NextRefresh(SourceFsnotify)
	newer := coordinator.NextRefresh(SourceManual)
	coordinator.ApplyResult(Result{Generation: newer.Generation, Sessions: []session.Session{}})
	coordinator.ApplyResult(Result{Generation: old.Generation, Sessions: []session.Session{{SessionID: "deleted"}}})

	if got := coordinator.Sessions(); len(got) != 0 {
		t.Fatalf("older result resurrected deleted session: %#v", got)
	}
}

func TestOlderIssuedResultCannotApplyBeforeNewerResultReturns(t *testing.T) {
	coordinator := NewCoordinator(Options{
		Parser:          (&fakeParser{}).Parse,
		ProjectsDir:     "/tmp/projects",
		InitialSessions: []session.Session{{SessionID: "old"}},
	})

	fsnotify := coordinator.NextRefresh(SourceFsnotify)
	manual := coordinator.NextRefresh(SourceManual)
	coordinator.ApplyResult(Result{Generation: fsnotify.Generation, Sessions: []session.Session{{SessionID: "stale"}}})
	if got := coordinator.Sessions(); len(got) != 1 || got[0].SessionID != "old" {
		t.Fatalf("older issued result applied before newer result returned: %#v", got)
	}

	coordinator.ApplyResult(Result{Generation: manual.Generation, Sessions: []session.Session{{SessionID: "manual"}}})
	if got := coordinator.Sessions(); len(got) != 1 || got[0].SessionID != "manual" {
		t.Fatalf("current generation result did not apply: %#v", got)
	}
}

func TestWatcherNextReturnsEventAndErrorResults(t *testing.T) {
	projectsDir := "/tmp/home/.claude/projects"
	events := make(chan RawEvent, 1)
	errs := make(chan error, 1)
	fs := &fakeWatchFS{
		dirs:   []string{projectsDir},
		events: events,
		errs:   errs,
	}
	watcher, err := NewWatcher(projectsDir, fs)
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	events <- RawEvent{Path: projectsDir + "/session.jsonl", Op: OpWrite}
	result := watcher.Next()
	if len(result.Events) != 1 || result.Events[0].Kind != EventWritten {
		t.Fatalf("events = %#v, want written event", result.Events)
	}

	errs <- errors.New("watcher closed")
	result = watcher.Next()
	if result.Err == nil || result.Err.Error() != "watcher closed" {
		t.Fatalf("Err = %v, want watcher error", result.Err)
	}
	if result.State.Status != StatusDegraded {
		t.Fatalf("state = %#v, want degraded", result.State)
	}
}

type fakeWatchFS struct {
	dirs       []string
	isDir      map[string]bool
	watchErrs  map[string]error
	fatalWatch map[string]bool
	events     <-chan RawEvent
	errs       <-chan error
}

func (fs *fakeWatchFS) WalkDirs(root string) ([]string, error) {
	return append([]string(nil), fs.dirs...), nil
}

func (fs *fakeWatchFS) Watch(path string) error {
	if err := fs.watchErrs[path]; err != nil {
		return err
	}
	return nil
}

func (fs *fakeWatchFS) IsDir(path string) bool {
	return fs.isDir[path]
}

func (fs *fakeWatchFS) FatalWatchError(path string) bool {
	return fs.fatalWatch[path]
}

func (fs *fakeWatchFS) Events() <-chan RawEvent {
	return fs.events
}

func (fs *fakeWatchFS) Errors() <-chan error {
	return fs.errs
}

type fakeParser struct {
	results map[string][]session.Session
}

func (p *fakeParser) Parse(source string) []session.Session {
	return append([]session.Session(nil), p.results[source]...)
}
