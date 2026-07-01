package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/keepalive"
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

	updated, cmd = model.Update(RefreshDebounceElapsedMsg{Now: now.Add(300 * time.Millisecond), Token: model.RefreshDebounceToken()})
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

func TestStaleWatcherDebounceElapsedDoesNotRefresh(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	calls := 0
	model := NewModel(Options{
		Now: now,
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				calls++
				return RefreshSnapshot{Sessions: []session.Session{{SessionID: "after-event", ShortID: "after"}}}
			},
		},
	})

	updated, _ := model.Update(RefreshWatcherEventsMsg{
		Events: []refresh.NormalizedEvent{{Kind: refresh.EventWritten, Path: "/tmp/one.jsonl"}},
		State:  refresh.State{Status: refresh.StatusOK, SafetyRefreshActive: true},
	})
	model = updated.(Model)
	firstToken := model.RefreshDebounceToken()

	updated, _ = model.Update(RefreshWatcherEventsMsg{
		Events: []refresh.NormalizedEvent{{Kind: refresh.EventWritten, Path: "/tmp/two.jsonl"}},
		State:  refresh.State{Status: refresh.StatusOK, SafetyRefreshActive: true},
	})
	model = updated.(Model)
	secondToken := model.RefreshDebounceToken()
	if secondToken == firstToken {
		t.Fatalf("second token = first token = %d, want fresh debounce token", secondToken)
	}

	updated, cmd := model.Update(RefreshDebounceElapsedMsg{Now: now.Add(300 * time.Millisecond), Token: firstToken})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("stale debounce returned command %#v, want nil", cmd)
	}
	if calls != 0 {
		t.Fatalf("stale debounce called RefreshSnapshot %d times, want 0", calls)
	}

	updated, cmd = model.Update(RefreshDebounceElapsedMsg{Now: now.Add(600 * time.Millisecond), Token: secondToken})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("latest debounce returned nil command")
	}
	_ = cmd()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestManualRefreshResetsNotificationSuppression(t *testing.T) {
	resets := 0
	model := NewModel(Options{
		Dependencies: Dependencies{
			ResetNotificationSuppression: func() { resets++ },
		},
	})

	updated, _ := model.Update(ManualRefreshMsg{})
	model = updated.(Model)

	if resets != 1 {
		t.Fatalf("notification suppression resets = %d, want 1", resets)
	}
	if model.LastAction() != "manual_refresh" {
		t.Fatalf("last action = %q, want manual_refresh", model.LastAction())
	}
}

func TestWatcherEventsArriveAsMessages(t *testing.T) {
	model := NewModel(Options{
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if source != refresh.SourceFsnotify {
					t.Fatalf("source = %q, want fsnotify", source)
				}
				return RefreshSnapshot{Sessions: []session.Session{{SessionID: "fsnotify", ShortID: "fsnotify"}}}
			},
		},
	})

	updated, cmd := model.Update(WatcherEventMsg{
		Path: "/tmp/session.jsonl",
		Op:   "write",
	})
	model = updated.(Model)

	if cmd == nil {
		t.Fatalf("WatcherEvent returned nil command, want refresh command")
	}
	msg := cmd()
	result, ok := msg.(RefreshResultMsg)
	if !ok {
		t.Fatalf("watcher command returned %#v, want RefreshResultMsg", msg)
	}
	if len(result.Sessions) != 1 || result.Sessions[0].SessionID != "fsnotify" {
		t.Fatalf("watcher refresh result = %#v", result)
	}
	events := model.WatcherEvents()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Path != "/tmp/session.jsonl" || events[0].Op != "write" {
		t.Fatalf("event = %#v", events[0])
	}
}

func TestSafetyRefreshProducesFreshGenerationCommand(t *testing.T) {
	model := NewModel(Options{
		RefreshGeneration: 4,
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if source != refresh.SourceSafety {
					t.Fatalf("source = %q, want safety", source)
				}
				if generation != 5 {
					t.Fatalf("generation = %d, want 5", generation)
				}
				return RefreshSnapshot{Sessions: []session.Session{{SessionID: "safety", ShortID: "safety"}}}
			},
		},
	})

	updated, cmd := model.Update(RefreshTickMsg{Now: time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)})
	model = updated.(Model)

	if cmd == nil {
		t.Fatal("safety refresh returned nil command")
	}
	if model.RefreshGeneration() != 5 {
		t.Fatalf("generation = %d, want 5", model.RefreshGeneration())
	}
	msg := cmd()
	result, ok := msg.(RefreshResultMsg)
	if !ok {
		t.Fatalf("safety command returned %#v, want RefreshResultMsg", msg)
	}
	if len(result.Sessions) != 1 || result.Sessions[0].SessionID != "safety" {
		t.Fatalf("safety refresh result = %#v", result)
	}
}

func TestRefreshResultsReplaceSessionStateAndIgnoreStaleGenerations(t *testing.T) {
	model := NewModel(Options{
		Sessions:          []session.Session{{SessionID: "old", ShortID: "old", Project: "old"}},
		RefreshGeneration: 2,
	})

	updated, _ := model.Update(RefreshResultMsg{
		Generation: 1,
		Sessions:   []session.Session{{SessionID: "stale", ShortID: "stale", Project: "stale"}},
	})
	model = updated.(Model)
	if model.Sessions()[0].SessionID != "old" {
		t.Fatalf("stale refresh replaced sessions: %#v", model.Sessions())
	}

	updated, _ = model.Update(RefreshResultMsg{
		Generation: 2,
		Sessions:   []session.Session{{SessionID: "new", ShortID: "new", Project: "new"}},
	})
	model = updated.(Model)
	if model.Sessions()[0].SessionID != "new" {
		t.Fatalf("current refresh did not replace sessions: %#v", model.Sessions())
	}
}

func TestRefreshResultPreservesSelectedSessionAcrossReorder(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
	})
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.SelectedSessionID() != "middle-id" {
		t.Fatalf("selected before refresh = %q, want middle-id", model.SelectedSessionID())
	}

	updated, _ = model.Update(RefreshResultMsg{
		Generation: 1,
		Sessions: []session.Session{
			listViewSession("inserted-id", "inserted", now.Add(2*time.Hour), now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
			listViewSession("middle-id", "mid", now.Add(-3*time.Hour), now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
			listViewSession("newer-id", "new", now.Add(-4*time.Hour), now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
		},
	})
	model = updated.(Model)

	if model.SelectedSessionID() != "middle-id" {
		t.Fatalf("selected after reorder refresh = %q, want middle-id", model.SelectedSessionID())
	}
	if focused := focusedListLine(model.View()); !strings.Contains(focused, "middle-id") {
		t.Fatalf("visible focus after reorder = %q, want middle-id in:\n%s", focused, model.View())
	}
}

func TestRefreshResultUpdatesEmptyState(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:     now,
		Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
	})

	updated, _ := model.Update(RefreshResultMsg{
		Generation: 1,
		Sessions: []session.Session{
			listViewSession("found-id", "found", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
		},
		Refresh:    RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNone},
		HasRefresh: true,
	})
	model = updated.(Model)
	if strings.Contains(model.View(), "No sessions found") {
		t.Fatalf("populated refresh still renders empty state:\n%s", model.View())
	}

	updated, _ = model.Update(RefreshResultMsg{
		Generation: 2,
		Sessions:   []session.Session{},
		Refresh:    RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
		HasRefresh: true,
	})
	model = updated.(Model)
	if !strings.Contains(model.View(), "No sessions found") {
		t.Fatalf("empty refresh did not render no-session state:\n%s", model.View())
	}
}

func TestManualRefreshBypassesDebounceWithFreshGeneration(t *testing.T) {
	model := NewModel(Options{
		RefreshGeneration: 2,
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if source != refresh.SourceManual {
					t.Fatalf("source = %q, want manual", source)
				}
				if generation != 3 {
					t.Fatalf("generation = %d, want 3", generation)
				}
				return RefreshSnapshot{Sessions: []session.Session{{SessionID: "manual", ShortID: "manual"}}}
			},
		},
	})

	updated, cmd := model.Update(ManualRefreshMsg{})
	model = updated.(Model)

	if cmd != nil {
		msg := cmd()
		result, ok := msg.(RefreshResultMsg)
		if !ok {
			t.Fatalf("manual refresh command returned %#v, want RefreshResultMsg", msg)
		}
		if result.Generation != 3 || len(result.Sessions) != 1 || result.Sessions[0].SessionID != "manual" {
			t.Fatalf("manual refresh result = %#v", result)
		}
	} else {
		t.Fatal("manual refresh returned nil command, want reparse command")
	}
	if model.RefreshGeneration() != 3 {
		t.Fatalf("generation = %d, want 3", model.RefreshGeneration())
	}
	if model.LastRefreshSource() != refresh.SourceManual {
		t.Fatalf("source = %q, want manual", model.LastRefreshSource())
	}
	if !model.LastRefreshBypassedDebounce() {
		t.Fatal("manual refresh did not bypass debounce")
	}
}

func TestRefreshResultHandlesDeletedAndNewSessions(t *testing.T) {
	model := NewModel(Options{
		RefreshGeneration: 1,
		Sessions:          []session.Session{{SessionID: "deleted", ShortID: "deleted"}},
	})

	updated, _ := model.Update(RefreshResultMsg{Generation: 2, Sessions: []session.Session{}})
	model = updated.(Model)
	if len(model.Sessions()) != 0 {
		t.Fatalf("deleted session remained after refresh: %#v", model.Sessions())
	}

	updated, _ = model.Update(RefreshResultMsg{Generation: 3, Sessions: []session.Session{{SessionID: "new", ShortID: "new"}}})
	model = updated.(Model)
	if got := model.Sessions(); len(got) != 1 || got[0].SessionID != "new" {
		t.Fatalf("new session not reflected: %#v", got)
	}
}

func TestRefreshTickReparsesListAndWorkspaceSessions(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	listCalls := 0
	list := NewModel(Options{
		Now:      now,
		Sessions: []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				listCalls++
				if source != refresh.SourceSafety {
					t.Fatalf("list refresh source = %q, want safety", source)
				}
				return RefreshSnapshot{Sessions: []session.Session{workspaceSession(now.Add(time.Minute))}, HasRefresh: true}
			},
		},
	})
	updated, cmd := list.Update(RefreshTickMsg{Now: now.Add(time.Minute)})
	list = updated.(Model)
	if cmd == nil {
		t.Fatal("refresh tick returned nil list command")
	}
	if msg := cmd(); msg.(RefreshResultMsg).Generation != list.RefreshGeneration() {
		t.Fatalf("refresh result generation did not match model generation")
	}
	if listCalls != 1 || list.LastRefreshSource() != refresh.SourceSafety {
		t.Fatalf("list refresh calls=%d source=%q", listCalls, list.LastRefreshSource())
	}

	selectedCalls := 0
	workspace := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				selectedCalls++
				if selected == nil || source != refresh.SourceSafety || selected.SessionID != "workspace-id" {
					t.Fatalf("workspace refresh source=%q selected=%#v", source, selected)
				}
				return RefreshSnapshot{
					Sessions:     []session.Session{workspaceSession(now.Add(time.Minute))},
					HasRefresh:   true,
					SelectedOnly: true,
					SelectedID:   selected.SessionID,
				}
			},
		},
	})
	updated, cmd = workspace.Update(RefreshTickMsg{Now: now.Add(time.Minute)})
	workspace = updated.(Model)
	if cmd == nil {
		t.Fatal("refresh tick returned nil workspace command")
	}
	result := cmd().(RefreshResultMsg)
	if !result.SelectedOnly || result.SelectedID != "workspace-id" {
		t.Fatalf("workspace refresh result selectedOnly=%v selectedID=%q", result.SelectedOnly, result.SelectedID)
	}
	if selectedCalls != 1 || workspace.LastRefreshSource() != refresh.SourceSafety {
		t.Fatalf("workspace refresh calls=%d source=%q", selectedCalls, workspace.LastRefreshSource())
	}
}

func TestRefreshSnapshotAppliesSelectedSessionSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	initial := workspaceSession(now)
	refreshed := initial
	refreshed.Messages.LastUserExcerpt = "fresh selected session"
	model := NewModel(Options{
		Now:        now,
		Sessions:   []session.Session{initial},
		SelectedID: initial.SessionID,
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if selected == nil || selected.SessionID != initial.SessionID {
					t.Fatalf("selected snapshot input = %#v", selected)
				}
				return RefreshSnapshot{
					Sessions:     []session.Session{refreshed},
					Refresh:      RefreshViewState{EmptyState: EmptyNone},
					HasRefresh:   true,
					SelectedOnly: true,
					SelectedID:   initial.SessionID,
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

func TestDisplayTickDoesNotRefreshData(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	refreshCalls := 0
	model := NewModel(Options{
		Now:      now,
		Sessions: []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				refreshCalls++
				return RefreshSnapshot{Sessions: []session.Session{workspaceSession(now.Add(time.Minute))}, HasRefresh: true}
			},
		},
	})
	updated, cmd := model.Update(DisplayTickMsg{Now: now.Add(time.Second)})
	model = updated.(Model)
	if refreshCalls != 0 || model.LastRefreshSource() != "" {
		t.Fatalf("display tick refreshed data: calls=%d source=%q", refreshCalls, model.LastRefreshSource())
	}
	if cmd != nil {
		if _, ok := cmd().(RefreshResultMsg); ok {
			t.Fatalf("display tick produced refresh result")
		}
	}
}

func TestRefreshDegradedMessageUpdatesTUIState(t *testing.T) {
	model := NewModel(Options{})

	updated, _ := model.Update(RefreshDegradedMsg{
		State: refresh.State{
			Status:              refresh.StatusPartial,
			Messages:            []string{"new directory permission denied"},
			SafetyRefreshActive: true,
		},
	})
	model = updated.(Model)

	view := model.View()
	for _, want := range []string{"watcher partial", "new directory permission denied"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestWorkspaceManualRefreshIsFocusableAndUsesSelectedIDSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if source != refresh.SourceManual {
					t.Fatalf("source = %q, want manual", source)
				}
				return RefreshSnapshot{
					Sessions:   []session.Session{workspaceSession(now.Add(time.Minute))},
					Refresh:    RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects"},
					HasRefresh: true,
				}
			},
		},
	})

	updated, cmd := model.Update(keyRunes("u"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("u on workspace manual refresh returned nil command")
	}
	msg := cmd()
	result, ok := msg.(RefreshResultMsg)
	if !ok {
		t.Fatalf("manual refresh command returned %#v, want RefreshResultMsg", msg)
	}
	if len(result.Sessions) != 1 || result.Sessions[0].SessionID != "workspace-id" {
		t.Fatalf("manual workspace refresh result = %#v, want only selected session", result.Sessions)
	}
	if model.LastAction() != "manual_refresh" || !model.LastRefreshBypassedDebounce() {
		t.Fatalf("manual refresh state = action %q bypass %v", model.LastAction(), model.LastRefreshBypassedDebounce())
	}
}

func TestWorkspaceManualRefreshPrefersSelectedPathSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	selectedCalls := 0
	fullCalls := 0
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if selected != nil {
					selectedCalls++
					if selected.SessionID != "workspace-id" {
						t.Fatalf("selected refresh got %q, want workspace-id", selected.SessionID)
					}
					return RefreshSnapshot{
						Sessions:     []session.Session{workspaceSession(now.Add(time.Minute))},
						HasRefresh:   true,
						SelectedOnly: true,
						SelectedID:   selected.SessionID,
					}
				}
				fullCalls++
				return RefreshSnapshot{Sessions: []session.Session{listViewSession("other-id", "other", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "")}, HasRefresh: true}
			},
		},
	})

	updated, cmd := model.Update(keyRunes("u"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("workspace refresh returned nil command")
	}
	result := cmd().(RefreshResultMsg)
	if !result.SelectedOnly || result.SelectedID != "workspace-id" {
		t.Fatalf("selected refresh metadata = selectedOnly %v selectedID %q", result.SelectedOnly, result.SelectedID)
	}
	if selectedCalls != 1 || fullCalls != 0 {
		t.Fatalf("refresh calls selected=%d full=%d, want selected-only IO", selectedCalls, fullCalls)
	}
}

func TestWorkspaceManualRefreshUpdatesOnlySelectedSessionWhenSnapshotReturnsList(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions: []session.Session{
			workspaceSession(now),
			listViewSession("other-id", "other", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
		},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if selected == nil || selected.SessionID != "workspace-id" {
					t.Fatalf("selected refresh input = %#v", selected)
				}
				return RefreshSnapshot{
					Sessions: []session.Session{
						listViewSession("other-id", "other-mutated", now.Add(time.Hour), now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
						workspaceSession(now.Add(time.Minute)),
					},
					HasRefresh:   true,
					SelectedOnly: true,
					SelectedID:   selected.SessionID,
				}
			},
		},
	})

	updated, cmd := model.Update(keyRunes("u"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("workspace refresh returned nil command")
	}
	result := cmd().(RefreshResultMsg)
	updated, _ = model.Update(result)
	model = updated.(Model)

	got := model.Sessions()
	if len(got) != 2 {
		t.Fatalf("sessions length = %d, want 2 selected plus existing other: %#v", len(got), got)
	}
	for _, s := range got {
		if s.SessionID == "other-id" && s.Project != "other" {
			t.Fatalf("workspace manual refresh mutated non-selected session: %#v", s)
		}
	}
}

func TestWorkspaceIgnoresKeepAliveAsyncAfterRefreshGenerationOrSelectionChanges(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions: []session.Session{
			workspaceSession(now),
			listViewSession("other-id", "other", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
		},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateConfirming, AutoSend: true, InstanceToken: 11, MaxSends: 1},
		},
	})

	updated, _ := model.Update(KeepAliveConfirmationResultMsg{SessionID: "workspace-id", InstanceToken: 11, ConfirmedAt: now, Generation: 1, SelectedID: "workspace-id"})
	model = updated.(Model)
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateConfirming {
		t.Fatalf("stale generation changed state to %q, want confirming", got)
	}

	model = NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions: []session.Session{
			workspaceSession(now),
			listViewSession("other-id", "other", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
		},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateConfirming, AutoSend: true, InstanceToken: 12, MaxSends: 1},
		},
	})
	model.selectedIndex = 1
	model.selectedID = "other-id"
	updated, _ = model.Update(KeepAliveConfirmationResultMsg{SessionID: "workspace-id", InstanceToken: 12, ConfirmedAt: now, SelectedID: "workspace-id"})
	model = updated.(Model)
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateConfirming {
		t.Fatalf("stale selected session changed state to %q, want confirming", got)
	}
}
