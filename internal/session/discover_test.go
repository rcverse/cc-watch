package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverHomeFindsJSONLRecursivelyAndSortsByModTime(t *testing.T) {
	home := t.TempDir()
	older := writeSessionFile(t, home, "-tmp-alpha", "aaaaaaaa-0000-0000-0000-000000000000", time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC))
	newer := writeSessionFile(t, home, "-tmp-beta", "bbbbbbbb-0000-0000-0000-000000000000", time.Date(2026, 6, 3, 11, 0, 0, 0, time.UTC))

	result, err := DiscoverHome(home, 10)
	if err != nil {
		t.Fatalf("DiscoverHome returned error: %v", err)
	}
	if result.Degraded {
		t.Fatalf("DiscoverHome degraded unexpectedly: %+v", result)
	}
	if len(result.Sessions) != 2 {
		t.Fatalf("len(Sessions) = %d, want 2", len(result.Sessions))
	}
	if result.Sessions[0].Path != newer || result.Sessions[1].Path != older {
		t.Fatalf("sort order = %#v, want newest first", result.Sessions)
	}
	if result.Sessions[0].Project != "tmp-beta" {
		t.Fatalf("Project = %q, want tmp-beta", result.Sessions[0].Project)
	}
	if result.Sessions[0].ShortID != "bbbbbbbb" {
		t.Fatalf("ShortID = %q, want bbbbbbbb", result.Sessions[0].ShortID)
	}
}

func TestDiscoverHomeHonorsLimit(t *testing.T) {
	home := t.TempDir()
	writeSessionFile(t, home, "-tmp-alpha", "aaaaaaaa-0000-0000-0000-000000000000", time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC))
	writeSessionFile(t, home, "-tmp-beta", "bbbbbbbb-0000-0000-0000-000000000000", time.Date(2026, 6, 3, 11, 0, 0, 0, time.UTC))

	result, err := DiscoverHome(home, 1)
	if err != nil {
		t.Fatalf("DiscoverHome returned error: %v", err)
	}
	if len(result.Sessions) != 1 {
		t.Fatalf("len(Sessions) = %d, want 1", len(result.Sessions))
	}
	if result.Sessions[0].SessionID != "bbbbbbbb-0000-0000-0000-000000000000" {
		t.Fatalf("SessionID = %q, want newest session", result.Sessions[0].SessionID)
	}
}

func TestDiscoverHomeMissingProjectsDirectoryIsStructuredDegradedState(t *testing.T) {
	home := t.TempDir()

	result, err := DiscoverHome(home, 5)
	if err != nil {
		t.Fatalf("DiscoverHome returned error: %v", err)
	}
	if !result.Degraded {
		t.Fatal("Degraded = false, want true")
	}
	if result.ErrorCode != "projects_dir_missing" {
		t.Fatalf("ErrorCode = %q, want projects_dir_missing", result.ErrorCode)
	}
	if len(result.Sessions) != 0 {
		t.Fatalf("len(Sessions) = %d, want 0", len(result.Sessions))
	}
}

func TestResolvePartialIDMatchesFilenameStemSubstring(t *testing.T) {
	sessions := []SessionFile{
		{SessionID: "aaaaaaaa-0000-0000-0000-000000000000", Path: "/tmp/aaaaaaaa-0000-0000-0000-000000000000.jsonl"},
		{SessionID: "bbbbbbbb-0000-0000-0000-000000000000", Path: "/tmp/bbbbbbbb-0000-0000-0000-000000000000.jsonl"},
	}

	match, err := ResolvePartialID(sessions, "BBBB")
	if err != nil {
		t.Fatalf("ResolvePartialID returned error: %v", err)
	}
	if match.SessionID != sessions[1].SessionID {
		t.Fatalf("match = %+v, want second session", match)
	}
}

func TestResolvePartialIDNoMatch(t *testing.T) {
	_, err := ResolvePartialID([]SessionFile{{SessionID: "aaaaaaaa"}}, "zzz")
	var resolveErr *ResolveError
	if !errors.As(err, &resolveErr) {
		t.Fatalf("err = %v, want ResolveError", err)
	}
	if resolveErr.Code != "session_not_found" {
		t.Fatalf("Code = %q, want session_not_found", resolveErr.Code)
	}
	if len(resolveErr.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want empty", resolveErr.Candidates)
	}
}

func TestResolvePartialIDAmbiguous(t *testing.T) {
	sessions := []SessionFile{
		{SessionID: "11111111-0000-0000-0000-000000000000"},
		{SessionID: "11112222-0000-0000-0000-000000000000"},
	}

	_, err := ResolvePartialID(sessions, "1111")
	var resolveErr *ResolveError
	if !errors.As(err, &resolveErr) {
		t.Fatalf("err = %v, want ResolveError", err)
	}
	if resolveErr.Code != "ambiguous_session_id" {
		t.Fatalf("Code = %q, want ambiguous_session_id", resolveErr.Code)
	}
	if len(resolveErr.Candidates) != 2 {
		t.Fatalf("len(Candidates) = %d, want 2", len(resolveErr.Candidates))
	}
}

func writeSessionFile(t *testing.T, home, project, sessionID string, modTime time.Time) string {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	if err := os.WriteFile(path, []byte(`{"timestamp":"2026-06-03T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	return path
}
