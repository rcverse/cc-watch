package app

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/notify"
	"github.com/richardchen/cc-watch/internal/refresh"
	"github.com/richardchen/cc-watch/internal/session"
	"github.com/richardchen/cc-watch/internal/tui"
)

func TestHelpExitsSuccessfully(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(--help) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") || !strings.Contains(stdout.String(), "cc-watch [--n N] [--id <partial-id>]") {
		t.Fatalf("help output missing usage:\n%s", stdout.String())
	}
	for _, notWant := range []string{"--watch", "Unsupported:"} {
		if strings.Contains(stdout.String(), notWant) {
			t.Fatalf("help output still advertises retired flag %q:\n%s", notWant, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "--remind") {
		t.Fatalf("help output still advertises obsolete --remind:\n%s", stdout.String())
	}
	for _, want := range []string{"TUI:", "Statusline:", "Safety:", "Examples:", "cc-watch --id d4b247b7", "See: cc-watch statusline --help"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestStatuslineHelpExitsSuccessfully(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"statusline", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(statusline --help) exit code = %d, want 0", code)
	}
	for _, want := range []string{"Usage:", "cc-watch statusline --check", "writes nothing", "KeepAlive at risk"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("statusline help missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionExitsSuccessfully(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(--version) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "cc-watch 2.0.0-dev") {
		t.Fatalf("version output = %q, want dev version", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRetiredWatchFlagIsUnknown(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--watch"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("Run(--watch) exit code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined: -watch") {
		t.Fatalf("stderr missing unknown-flag error:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "not part of cc-watch v2") {
		t.Fatalf("stderr still treats watch as known retired mode:\n%s", stderr.String())
	}
}

func TestRetiredRemindFlagIsUnknown(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--remind"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("Run(--remind) exit code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined: -remind") {
		t.Fatalf("stderr missing unknown-flag error:\n%s", stderr.String())
	}
}

func TestConfigDispatchStartsConfigEditor(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := fakeDeps(t)

	code := RunWithDeps([]string{"config"}, &stdout, &stderr, deps.Dependencies)

	if code != 0 {
		t.Fatalf("Run(config) exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if deps.tuiStarts != 1 {
		t.Fatalf("tui starts = %d, want 1", deps.tuiStarts)
	}
}

func TestTUIDispatchStartsListWithoutKeepAliveSideEffects(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := fakeDeps(t)

	code := RunWithDeps([]string{}, &stdout, &stderr, deps.Dependencies)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if deps.tuiStarts != 1 {
		t.Fatalf("tui starts = %d, want 1", deps.tuiStarts)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestTUIDispatchForwardsPublicCLICommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Command
	}{
		{
			name: "default cc-watch",
			args: nil,
			want: Command{Mode: ModeTUI, Limit: 25},
		},
		{
			name: "--n N",
			args: []string{"--n", "2"},
			want: Command{Mode: ModeTUI, Limit: 2},
		},
		{
			name: "--id partial",
			args: []string{"--id", "11111111"},
			want: Command{Mode: ModeTUI, Limit: 25, ID: "11111111"},
		},
		{
			name: "config",
			args: []string{"config"},
			want: Command{Mode: ModeConfig, Limit: 25},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			deps := fakeDeps(t)
			var got Command
			deps.StartTUI = func(cmd Command) error {
				deps.tuiStarts++
				got = cmd
				return nil
			}

			code := RunWithDeps(tt.args, &stdout, &stderr, deps.Dependencies)

			if code != 0 {
				t.Fatalf("RunWithDeps(%v) exit code = %d, want 0; stderr=%q", tt.args, code, stderr.String())
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("command = %#v, want %#v", got, tt.want)
			}
			if deps.tuiStarts != 1 {
				t.Fatalf("tui starts = %d, want 1", deps.tuiStarts)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

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

func TestRunTUIClosesLiveRefresh(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	deps := fakeDeps(t)
	deps.StartTUI = nil
	deps.Now = func() time.Time { return now }
	closed := false
	deps.NewLiveWatcher = func(projectsDir string) (tui.Watcher, func() error, error) {
		return fakeLiveWatcher{}, func() error {
			closed = true
			return nil
		}, nil
	}
	deps.RunTUIProgram = func(options tui.Options) error {
		if options.LiveRefresh == nil {
			t.Fatal("RunTUIProgram received nil LiveRefresh")
		}
		return nil
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

	if err := runTUI(Command{Mode: ModeTUI, Limit: 5}, deps.Dependencies); err != nil {
		t.Fatalf("runTUI returned error: %v", err)
	}
	if !closed {
		t.Fatal("live refresh closer was not called")
	}
}

type fakeLiveWatcher struct{}

func (fakeLiveWatcher) Next() refresh.WatcherResult {
	return refresh.WatcherResult{State: refresh.State{Status: refresh.StatusOK, SafetyRefreshActive: true}}
}

func TestTUIStartupWithIDSelectsMatchingSession(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	deps := fakeDeps(t)
	deps.Now = func() time.Time { return now }
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		if limit != 0 {
			t.Fatalf("DiscoverHome limit = %d, want 0 for ID resolution", limit)
		}
		return session.DiscoveryResult{
			Sessions: []session.SessionFile{
				{SessionID: "11111111-0000-0000-0000-000000000000", ShortID: "11111111", Project: "one", Path: "/tmp/one.jsonl", ModTime: now},
				{SessionID: "22222222-0000-0000-0000-000000000000", ShortID: "22222222", Project: "two", Path: "/tmp/two.jsonl", ModTime: now.Add(-time.Minute)},
			},
		}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		if path != "/tmp/one.jsonl" {
			t.Fatalf("ParseFile path = %q, want selected session path", path)
		}
		return session.Session{
			SessionID:      "11111111-0000-0000-0000-000000000000",
			ShortID:        "11111111",
			Project:        "one",
			JSONLPath:      path,
			FileModifiedAt: now,
		}, nil
	}

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5, ID: "11111111"}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}

	if options.SelectedID != "11111111-0000-0000-0000-000000000000" {
		t.Fatalf("selected ID = %q, want matching session", options.SelectedID)
	}
	if len(options.Sessions) != 1 || options.Sessions[0].ShortID != "11111111" {
		t.Fatalf("sessions = %#v, want selected session only", options.Sessions)
	}
}

func TestTUISelectedDispatchPreservesID(t *testing.T) {
	deps := testDepsWithTwoSessions(t)
	var tuiCommand Command
	deps.StartTUI = func(cmd Command) error {
		tuiCommand = cmd
		return nil
	}

	if code := RunWithDeps([]string{"--id", "2222"}, io.Discard, io.Discard, deps.Dependencies); code != 0 {
		t.Fatalf("TUI selected run exit = %d, want 0", code)
	}
	if tuiCommand.ID != "2222" {
		t.Fatalf("TUI command ID = %q, want 2222", tuiCommand.ID)
	}
}

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

func TestTUIIDNoMatchMapsToCurrentEmptyStateBehavior(t *testing.T) {
	deps := fakeDeps(t)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{Sessions: []session.SessionFile{{
			SessionID: "11111111-1111-1111-1111-111111111111",
			ShortID:   "11111111",
			Project:   "tmp",
			Path:      "/tmp/session.jsonl",
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
	view := model.View()
	if !strings.Contains(view, "matched more than one session") {
		t.Fatalf("ambiguous route did not render:\n%s", view)
	}
	for _, candidate := range []string{"11111111", "11112222"} {
		if !strings.Contains(view, candidate) {
			t.Fatalf("ambiguous route missing candidate %q:\n%s", candidate, view)
		}
	}
}

func TestWorkspaceManualRefreshParsesSelectedJSONLPathOnly(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{Sessions: []session.SessionFile{{
			SessionID: "11111111-1111-1111-1111-111111111111",
			ShortID:   "11111111",
			Project:   "tmp",
			Path:      "/tmp/selected.jsonl",
			ModTime:   now,
		}}}, nil
	}
	parseCalls := []string{}
	deps.ParseFile = func(path string) (session.Session, error) {
		parseCalls = append(parseCalls, path)
		return session.Session{
			SessionID:      "11111111-1111-1111-1111-111111111111",
			ShortID:        "11111111",
			Project:        "tmp",
			JSONLPath:      path,
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
	snapshot := options.Dependencies.RefreshSnapshot(&selected)
	if len(parseCalls) != 1 || parseCalls[0] != "/tmp/selected.jsonl" {
		t.Fatalf("refresh parse calls = %#v, want selected path only", parseCalls)
	}
	if !snapshot.SelectedOnly || snapshot.SelectedID != selected.SessionID {
		t.Fatalf("snapshot selected flags = %#v", snapshot)
	}
}

func TestWorkspaceManualRefreshParseFailurePreservesSelectedRefreshScope(t *testing.T) {
	deps := fakeDeps(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{Sessions: []session.SessionFile{{
			SessionID: "11111111-1111-1111-1111-111111111111",
			ShortID:   "11111111",
			Project:   "tmp",
			Path:      "/tmp/selected.jsonl",
			ModTime:   now,
		}}}, nil
	}
	parseErr := false
	deps.ParseFile = func(path string) (session.Session, error) {
		if parseErr {
			return session.Session{}, errors.New("read failed")
		}
		return session.Session{
			SessionID:      "11111111-1111-1111-1111-111111111111",
			ShortID:        "11111111",
			Project:        "tmp",
			JSONLPath:      path,
			FileModifiedAt: now,
		}, nil
	}

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5, ID: "11111111"}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions: %v", err)
	}
	selected := options.Sessions[0]
	parseErr = true
	snapshot := options.Dependencies.RefreshSnapshot(&selected)
	if !snapshot.SelectedOnly || snapshot.SelectedID != selected.SessionID {
		t.Fatalf("snapshot selected flags = %#v, want selected scope preserved", snapshot)
	}
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("snapshot sessions = %#v, want no replacement on parse failure", snapshot.Sessions)
	}
}

func TestTUIStartupWiresManualRefreshLoader(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	deps := fakeDeps(t)
	deps.Now = func() time.Time { return now }
	loads := 0
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		loads++
		return session.DiscoveryResult{
			ProjectsDir: "/tmp/home/.claude/projects",
			Sessions: []session.SessionFile{{
				SessionID: "refresh-id",
				ShortID:   "refresh",
				Project:   "tmp",
				Path:      "/tmp/refresh.jsonl",
				ModTime:   now.Add(time.Duration(loads) * time.Minute),
			}},
		}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		return session.Session{
			SessionID:      "refresh-id",
			ShortID:        "refresh",
			Project:        "tmp",
			JSONLPath:      path,
			FileModifiedAt: now.Add(time.Duration(loads) * time.Minute),
		}, nil
	}

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}
	if !options.StartRefreshTicker {
		t.Fatal("StartRefreshTicker = false, want autonomous refresh enabled")
	}
	model := tui.NewModel(options)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	model = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("u refresh returned nil command")
	}
	msg := cmd()
	result, ok := msg.(tui.RefreshResultMsg)
	if !ok {
		t.Fatalf("refresh command returned %#v, want RefreshResultMsg", msg)
	}
	if len(result.Sessions) != 1 || result.Sessions[0].SessionID != "refresh-id" {
		t.Fatalf("refresh result = %#v", result)
	}
}

func TestWorkspaceManualRefreshParsesOnlySelectedSessionPath(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	deps := fakeDeps(t)
	deps.Now = func() time.Time { return now }
	discoverCalls := 0
	parseCalls := map[string]int{}
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		discoverCalls++
		return session.DiscoveryResult{
			ProjectsDir: "/tmp/home/.claude/projects",
			Sessions: []session.SessionFile{
				{SessionID: "selected-id", ShortID: "selected", Project: "tmp", Path: "/tmp/selected.jsonl", ModTime: now.Add(time.Minute)},
				{SessionID: "other-id", ShortID: "other", Project: "tmp", Path: "/tmp/other.jsonl", ModTime: now},
			},
		}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		parseCalls[path]++
		id := "other-id"
		short := "other"
		if path == "/tmp/selected.jsonl" {
			id = "selected-id"
			short = "selected"
		}
		return session.Session{
			SessionID:      id,
			ShortID:        short,
			Project:        "tmp",
			JSONLPath:      path,
			FileModifiedAt: now,
		}, nil
	}

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}
	model := tui.NewModel(options)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(tui.Model)
	discoverCalls = 0
	parseCalls = map[string]int{}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	model = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("workspace refresh returned nil command")
	}
	msg := cmd()
	result, ok := msg.(tui.RefreshResultMsg)
	if !ok {
		t.Fatalf("refresh command returned %#v, want RefreshResultMsg", msg)
	}
	if !result.SelectedOnly || result.SelectedID != "selected-id" {
		t.Fatalf("refresh metadata selectedOnly=%v selectedID=%q", result.SelectedOnly, result.SelectedID)
	}
	if discoverCalls != 0 {
		t.Fatalf("workspace selected refresh rediscovered sessions %d time(s)", discoverCalls)
	}
	if parseCalls["/tmp/selected.jsonl"] != 1 || len(parseCalls) != 1 {
		t.Fatalf("parse calls = %#v, want selected path only", parseCalls)
	}
}

func TestTUIStartupWiresNotificationCallbacks(t *testing.T) {
	deps := fakeDeps(t)
	notifyCalls := 0
	resetCalls := 0
	deps.NotifyEvent = func(event notify.Event) notify.Result {
		notifyCalls++
		if event.Kind != notify.EventReminderThresholdCrossed {
			t.Fatalf("event kind = %q, want reminder threshold", event.Kind)
		}
		return notify.Result{Delivered: true, Message: "delivered"}
	}
	deps.ResetNotificationSuppression = func() { resetCalls++ }

	options, err := buildTUIOptions(Command{Mode: ModeTUI, Limit: 5}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}
	result := options.Dependencies.NotifyEvent(notify.Event{Kind: notify.EventReminderThresholdCrossed})
	options.Dependencies.ResetNotificationSuppression()

	if !result.Delivered {
		t.Fatalf("notify result = %#v, want delivered", result)
	}
	if notifyCalls != 1 {
		t.Fatalf("notify calls = %d, want 1", notifyCalls)
	}
	if resetCalls != 1 {
		t.Fatalf("reset calls = %d, want 1", resetCalls)
	}
}

func TestConfigEditorStartupLoadsAndSavesConfig(t *testing.T) {
	home := t.TempDir()
	deps := fakeDeps(t)
	deps.HomeDir = func() (string, error) { return home, nil }

	options, err := buildTUIOptions(Command{Mode: ModeConfig}, deps.Dependencies)
	if err != nil {
		t.Fatalf("buildTUIOptions returned error: %v", err)
	}
	model := tui.NewModel(options)
	if !strings.Contains(model.View(), "Claude Code Watch / config") || !strings.Contains(model.View(), "Reminder thresholds") {
		t.Fatalf("config editor did not render:\n%s", model.View())
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(tui.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(tui.Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = updated.(tui.Model)

	loaded, err := config.Load(home)
	if err != nil {
		t.Fatalf("config load after save: %v", err)
	}
	if loaded.KeepAlive.Scope.MaxSends != config.Default().KeepAlive.Scope.MaxSends {
		t.Fatalf("saved config changed max sends: %#v", loaded)
	}
}

func TestConfigEditorStartupDoesNotDiscoverOrParseSessions(t *testing.T) {
	deps := fakeDeps(t)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		t.Fatalf("config editor should not discover sessions")
		return session.DiscoveryResult{}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		t.Fatalf("config editor should not parse sessions")
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

func TestParseListFlags(t *testing.T) {
	cmd, err := ParseArgs([]string{"--n", "7", "--id", "abc"})
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	if cmd.Mode != ModeTUI {
		t.Fatalf("Mode = %q, want %q", cmd.Mode, ModeTUI)
	}
	if cmd.Limit != 7 {
		t.Fatalf("Limit = %d, want 7", cmd.Limit)
	}
	if cmd.ID != "abc" {
		t.Fatalf("ID = %q, want abc", cmd.ID)
	}
}

func TestParseStatuslineArgsGrammar(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Command
		wantErr string
	}{
		{
			name: "bare",
			args: []string{"statusline"},
			want: Command{Mode: ModeStatusline, Limit: DefaultLimit},
		},
		{
			name: "check",
			args: []string{"statusline", "--check"},
			want: Command{Mode: ModeStatusline, Limit: DefaultLimit, CheckConfig: true},
		},
		{
			name: "help",
			args: []string{"statusline", "--help"},
			want: Command{Mode: ModeStatuslineHelp, Limit: DefaultLimit},
		},
		{
			name: "wrapped command",
			args: []string{"statusline", "--", "ccstatusline", "--flag"},
			want: Command{Mode: ModeStatusline, Limit: DefaultLimit, WrappedCommand: []string{"ccstatusline", "--flag"}},
		},
		{
			name:    "dash-dash with no command",
			args:    []string{"statusline", "--"},
			wantErr: "statusline: no command given after --",
		},
		{
			name:    "check with trailing wrapped command",
			args:    []string{"statusline", "--check", "--", "cmd"},
			wantErr: statuslineUsageError,
		},
		{
			name:    "unrecognized flag",
			args:    []string{"statusline", "--bogus"},
			wantErr: statuslineUsageError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseArgs returned error: %v", err)
			}
			if cmd.Mode != tt.want.Mode || cmd.CheckConfig != tt.want.CheckConfig ||
				!reflect.DeepEqual(cmd.WrappedCommand, tt.want.WrappedCommand) {
				t.Fatalf("cmd = %#v, want %#v", cmd, tt.want)
			}
		})
	}
}

type fakeAppDeps struct {
	Dependencies
	tuiStarts int
}

func fakeDeps(t *testing.T) *fakeAppDeps {
	t.Helper()
	deps := &fakeAppDeps{}
	deps.HomeDir = func() (string, error) { return t.TempDir(), nil }
	deps.Now = func() time.Time { return time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC) }
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		t.Fatalf("ParseFile called unexpectedly with %q", path)
		return session.Session{}, nil
	}
	deps.StartTUI = func(Command) error {
		deps.tuiStarts++
		return nil
	}
	return deps
}

func testDepsWithTwoSessions(t *testing.T) *fakeAppDeps {
	t.Helper()
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	deps := fakeDeps(t)
	deps.Now = func() time.Time { return now }
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{
			ProjectsDir: "/home/me/.claude/projects",
			Sessions: []session.SessionFile{
				{SessionID: "11111111-1111-1111-1111-111111111111", ShortID: "11111111", Project: "one", Path: "/tmp/one.jsonl", ModTime: now},
				{SessionID: "22222222-2222-2222-2222-222222222222", ShortID: "22222222", Project: "two", Path: "/tmp/two.jsonl", ModTime: now.Add(-time.Minute)},
			},
		}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		switch path {
		case "/tmp/one.jsonl":
			return session.Session{
				SessionID:      "11111111-1111-1111-1111-111111111111",
				ShortID:        "11111111",
				Project:        "one",
				JSONLPath:      path,
				FileModifiedAt: now,
			}, nil
		case "/tmp/two.jsonl":
			return session.Session{
				SessionID:      "22222222-2222-2222-2222-222222222222",
				ShortID:        "22222222",
				Project:        "two",
				JSONLPath:      path,
				FileModifiedAt: now.Add(-time.Minute),
			}, nil
		default:
			t.Fatalf("ParseFile path = %q, want fixture path", path)
			return session.Session{}, nil
		}
	}
	return deps
}
