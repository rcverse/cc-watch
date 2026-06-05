package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/session"
	"github.com/richardchen/cc-cache/internal/tui"
)

func TestHelpExitsSuccessfully(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(--help) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage: cc-cache") {
		t.Fatalf("help output missing usage:\n%s", stdout.String())
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
	if !strings.Contains(stdout.String(), "cc-cache 2.0.0-dev") {
		t.Fatalf("version output = %q, want dev version", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestWatchIsExplicitlyRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--watch"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("Run(--watch) exit code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--watch is not part of cc-cache v2") {
		t.Fatalf("stderr missing watch rejection:\n%s", stderr.String())
	}
}

func TestJSONDispatchIsNonInteractive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	last := now.Add(-5 * time.Minute)
	deps := fakeDeps(t)
	deps.Now = func() time.Time { return now }
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{
			Sessions: []session.SessionFile{{
				SessionID: "11111111-1111-1111-1111-111111111111",
				ShortID:   "11111111",
				Project:   "tmp",
				Path:      "/tmp/session.jsonl",
				ModTime:   now,
			}},
		}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		return session.Session{
			SessionID:      "11111111-1111-1111-1111-111111111111",
			ShortID:        "11111111",
			Project:        "tmp",
			JSONLPath:      path,
			FileModifiedAt: now,
			LastMessageAt:  &last,
			CacheWindow: session.CacheWindow{
				Tier:       session.Tier1Hour,
				Label:      "1h",
				TTLSeconds: 3600,
				Known:      true,
			},
		}, nil
	}

	code := RunWithDeps([]string{"--json", "--id", "11111111", "--n", "3"}, &stdout, &stderr, deps.Dependencies)

	if code != 0 {
		t.Fatalf("Run(--json) exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if deps.tuiStarts != 0 || deps.watcherStarts != 0 || deps.notifierStarts != 0 || deps.keepAliveRunnerCreations != 0 {
		t.Fatalf("interactive side effects: tui=%d watcher=%d notifier=%d keepalive=%d", deps.tuiStarts, deps.watcherStarts, deps.notifierStarts, deps.keepAliveRunnerCreations)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doc["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %#v, want 1", doc["schema_version"])
	}
	if doc["selected_session"] == nil {
		t.Fatalf("selected_session = nil, want selected session JSON:\n%s", stdout.String())
	}
}

func TestConfigDispatchIsParsedButNotWired(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"config"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("Run(config) exit code = 0, want non-zero until Phase 10")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "config editor is not wired until Phase 10") {
		t.Fatalf("stderr missing config not-wired message:\n%s", stderr.String())
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
	if deps.keepAliveRunnerCreations != 0 {
		t.Fatalf("KeepAlive runner creations = %d, want 0", deps.keepAliveRunnerCreations)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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
	model := tui.NewModel(options)
	for _, key := range []tea.KeyMsg{{Type: tea.KeyDown}, {Type: tea.KeyDown}, {Type: tea.KeyDown}} {
		updated, _ := model.Update(key)
		model = updated.(tui.Model)
	}
	if model.FocusedAction() != "refresh" {
		t.Fatalf("focused action = %q, want refresh", model.FocusedAction())
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("refresh action returned nil command")
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
	for _, key := range []tea.KeyMsg{{Type: tea.KeyDown}, {Type: tea.KeyDown}, {Type: tea.KeyDown}, {Type: tea.KeyDown}} {
		updated, _ = model.Update(key)
		model = updated.(tui.Model)
	}
	if model.FocusedAction() != "refresh" {
		t.Fatalf("workspace focused action = %q, want refresh", model.FocusedAction())
	}

	discoverCalls = 0
	parseCalls = map[string]int{}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
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

func TestParseListFlags(t *testing.T) {
	cmd, err := ParseArgs([]string{"--n", "7", "--id", "abc", "--remind"})
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
	if !cmd.Remind {
		t.Fatal("Remind = false, want true")
	}
}

func TestJSONNoMatchReturnsContractError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := fakeDeps(t)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{
			Sessions: []session.SessionFile{{
				SessionID: "11111111-1111-1111-1111-111111111111",
				ShortID:   "11111111",
				Project:   "tmp",
				Path:      "/tmp/session.jsonl",
			}},
		}, nil
	}

	code := RunWithDeps([]string{"--json", "--id", "zzz"}, &stdout, &stderr, deps.Dependencies)

	if code == 0 {
		t.Fatal("Run(--json --id zzz) exit code = 0, want non-zero")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	errObj := doc["error"].(map[string]any)
	if errObj["code"] != "session_not_found" {
		t.Fatalf("error code = %#v, want session_not_found", errObj["code"])
	}
}

func TestJSONAmbiguousIDReturnsCandidates(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := fakeDeps(t)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		return session.DiscoveryResult{
			Sessions: []session.SessionFile{
				{SessionID: "11111111-0000-0000-0000-000000000000", ShortID: "11111111", Project: "one"},
				{SessionID: "11112222-0000-0000-0000-000000000000", ShortID: "11112222", Project: "two"},
			},
		}, nil
	}

	code := RunWithDeps([]string{"--json", "--id", "1111"}, &stdout, &stderr, deps.Dependencies)

	if code == 0 {
		t.Fatal("Run ambiguous JSON query exit code = 0, want non-zero")
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doc["error"].(map[string]any)["code"] != "ambiguous_session_id" {
		t.Fatalf("error = %#v", doc["error"])
	}
	if len(doc["sessions"].([]any)) != 2 {
		t.Fatalf("sessions = %#v, want 2 candidates", doc["sessions"])
	}
	candidate := doc["sessions"].([]any)[0].(map[string]any)
	if _, ok := candidate["cache_window"]; ok {
		t.Fatalf("ambiguous candidate contains full session fields: %#v", candidate)
	}
}

func TestJSONIDResolutionIgnoresListLimit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := fakeDeps(t)
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	deps.DiscoverHome = func(home string, limit int) (session.DiscoveryResult, error) {
		if limit != 0 {
			t.Fatalf("DiscoverHome limit = %d, want 0 for ID resolution", limit)
		}
		return session.DiscoveryResult{
			Sessions: []session.SessionFile{
				{SessionID: "newer", ShortID: "newer", Project: "new", Path: "/tmp/newer.jsonl", ModTime: now},
				{SessionID: "older", ShortID: "older", Project: "old", Path: "/tmp/older.jsonl", ModTime: now.Add(-time.Hour)},
			},
		}, nil
	}
	deps.ParseFile = func(path string) (session.Session, error) {
		if path != "/tmp/older.jsonl" {
			t.Fatalf("ParseFile path = %q, want older session", path)
		}
		return session.Session{
			SessionID:      "older",
			ShortID:        "older",
			Project:        "old",
			JSONLPath:      path,
			FileModifiedAt: now,
		}, nil
	}

	code := RunWithDeps([]string{"--json", "--id", "older", "--n", "1"}, &stdout, &stderr, deps.Dependencies)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0; stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
}

func TestJSONHomeErrorUsesContractShape(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := fakeDeps(t)
	deps.HomeDir = func() (string, error) { return "", errors.New("home unavailable") }

	code := RunWithDeps([]string{"--json"}, &stdout, &stderr, deps.Dependencies)

	if code == 0 {
		t.Fatal("Run exit code = 0, want non-zero")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doc["error"].(map[string]any)["code"] != "config_error" {
		t.Fatalf("error = %#v, want config_error", doc["error"])
	}
}

func TestJSONInvalidConfigWarningIsVisible(t *testing.T) {
	var stdout, stderr bytes.Buffer
	home := t.TempDir()
	configPath := filepath.Join(home, ".config", "cc-cache", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{bad-json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	deps := fakeDeps(t)
	deps.HomeDir = func() (string, error) { return home, nil }
	deps.ParseFile = func(path string) (session.Session, error) {
		t.Fatalf("ParseFile called unexpectedly with %q", path)
		return session.Session{}, nil
	}

	code := RunWithDeps([]string{"--json"}, &stdout, &stderr, deps.Dependencies)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0; stderr=%q stdout=%s", code, stderr.String(), stdout.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	warnings := doc["config"].(map[string]any)["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("config warnings = %#v, want one warning", warnings)
	}
}

type fakeAppDeps struct {
	Dependencies
	tuiStarts                int
	watcherStarts            int
	notifierStarts           int
	keepAliveRunnerCreations int
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
	deps.StartWatcher = func() error {
		deps.watcherStarts++
		return nil
	}
	deps.StartNotifier = func() error {
		deps.notifierStarts++
		return nil
	}
	deps.NewKeepAliveRunner = func() error {
		deps.keepAliveRunnerCreations++
		return nil
	}
	return deps
}
