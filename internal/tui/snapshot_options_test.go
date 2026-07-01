package tui

import (
	"testing"
	"time"

	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/session"
	"github.com/richardchen/cc-watch/internal/snapshot"
)

func TestOptionsFromSnapshotMapsSelectedWorkspace(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	selected := session.Session{
		SessionID: "selected-id",
		ShortID:   "selected",
		JSONLPath: "/tmp/selected.jsonl",
	}
	options := OptionsFromSnapshot(SnapshotOptionsInput{
		Result: snapshot.Result{
			GeneratedAt: now,
			Config:      config.Default(),
			ProjectsDir: "/tmp/home/.claude/projects",
			Sessions: []session.Session{{
				SessionID: "other-id",
				ShortID:   "other",
			}},
			Selected: &selected,
		},
		StartMode: StartList,
	})

	if options.Now != now {
		t.Fatalf("Now = %v, want %v", options.Now, now)
	}
	if options.SelectedID != "selected-id" {
		t.Fatalf("SelectedID = %q, want selected-id", options.SelectedID)
	}
	if len(options.Sessions) != 1 || options.Sessions[0].SessionID != "selected-id" {
		t.Fatalf("Sessions = %#v, want selected only", options.Sessions)
	}
	if options.StartRefreshTicker != true {
		t.Fatalf("StartRefreshTicker = false, want true")
	}
}

func TestOptionsFromSnapshotMapsAmbiguousCandidatesAndReminders(t *testing.T) {
	candidates := []session.Session{
		{SessionID: "candidate-1", ShortID: "11111111"},
		{SessionID: "candidate-2", ShortID: "11112222"},
	}
	options := OptionsFromSnapshot(SnapshotOptionsInput{
		Result: snapshot.Result{
			Config:     config.Default(),
			Error:      &snapshot.Error{Code: "ambiguous_session_id", Query: "1111"},
			Candidates: candidates,
			Reminder: map[string]snapshot.ReminderState{
				"candidate-1": {Enabled: true},
				"candidate-2": {Enabled: true},
				"disabled":    {Enabled: false},
			},
		},
		StartMode: StartList,
	})

	if options.AmbiguousID != "1111" {
		t.Fatalf("AmbiguousID = %q, want query", options.AmbiguousID)
	}
	if len(options.Sessions) != 2 {
		t.Fatalf("Sessions len = %d, want candidates", len(options.Sessions))
	}
	if !options.ReminderEnabled["candidate-1"] || !options.ReminderEnabled["candidate-2"] {
		t.Fatalf("ReminderEnabled = %#v, want enabled candidates", options.ReminderEnabled)
	}
	if options.ReminderEnabled["disabled"] {
		t.Fatalf("disabled reminder projected as enabled: %#v", options.ReminderEnabled)
	}
}

func TestOptionsFromSnapshotMapsConfigModeWithoutRefreshTicker(t *testing.T) {
	options := OptionsFromSnapshot(SnapshotOptionsInput{
		Result:    snapshot.Result{Config: config.Default()},
		StartMode: StartConfig,
	})

	if options.StartMode != StartConfig {
		t.Fatalf("StartMode = %q, want config", options.StartMode)
	}
	if options.StartRefreshTicker {
		t.Fatalf("StartRefreshTicker = true, want false")
	}
}

func TestRefreshSnapshotFromSnapshotResultMapsNoMatchEmptyState(t *testing.T) {
	refresh := RefreshSnapshotFromSnapshotResult(snapshot.Result{
		ProjectsDir: "/tmp/home/.claude/projects",
		Error:       &snapshot.Error{Code: "session_not_found", Query: "missing"},
		Sessions: []session.Session{{
			SessionID: "old",
		}},
	})

	if !refresh.HasRefresh {
		t.Fatalf("HasRefresh = false, want true")
	}
	if len(refresh.Sessions) != 0 {
		t.Fatalf("Sessions = %#v, want empty for not found", refresh.Sessions)
	}
	if refresh.Refresh.EmptyState != EmptyNoSessions {
		t.Fatalf("EmptyState = %q, want no sessions", refresh.Refresh.EmptyState)
	}
}
