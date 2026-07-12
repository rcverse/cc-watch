package snapshot

import (
	"errors"
	"testing"
	"time"

	"github.com/rcverse/cc-watch/internal/config"
	"github.com/rcverse/cc-watch/internal/session"
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
}

func TestBuildSelectedSnapshotResolvesPartialIDAndParsesOnlySelected(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	loaders := fakeLoaders(t)
	loaders.files = []session.SessionFile{
		{SessionID: "11111111-1111-1111-1111-111111111111", ShortID: "11111111", Project: "alpha", Path: "/tmp/alpha.jsonl", ModTime: now},
		{SessionID: "22222222-2222-2222-2222-222222222222", ShortID: "22222222", Project: "beta", Path: "/tmp/beta.jsonl", ModTime: now},
	}

	result, err := Build(Request{Home: "/home/me", Now: now, Limit: 5, ID: "2222"}, loaders.Loaders())
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
		ShortID:   "11111111",
		Project:   "alpha",
		Path:      "/tmp/alpha.jsonl",
		ModTime:   now,
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
		LoadConfig: func(home string) (config.Config, error) {
			if home != "/home/me" {
				f.t.Fatalf("home = %q, want /home/me", home)
			}
			return config.Default(), nil
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
