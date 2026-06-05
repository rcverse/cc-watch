package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-cache/internal/config"
	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
)

func TestDisplayTickRecomputesTimeOnly(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	last := now.Add(-5 * time.Minute)
	deps := &fakeDeps{}
	model := NewModel(Options{
		Now: now,
		Dependencies: Dependencies{
			Discover: deps.discover,
			Parse:    deps.parse,
			Refresh:  deps.refresh,
		},
		Sessions: []session.Session{{
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ShortID:       "11111111",
			Project:       "tmp",
			LastMessageAt: &last,
			CacheWindow: session.CacheWindow{
				Tier:       session.Tier1Hour,
				Label:      "1h",
				TTLSeconds: 3600,
				Known:      true,
			},
		}},
		Countdowns: map[string]int{"11111111-1111-1111-1111-111111111111": 5},
	})

	updated, cmd := model.Update(DisplayTickMsg{Now: now.Add(time.Second)})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("DisplayTick returned command, want nil")
	}
	if deps.discoverCalls != 0 || deps.parseCalls != 0 || deps.refreshCalls != 0 {
		t.Fatalf("display tick called deps: discover=%d parse=%d refresh=%d", deps.discoverCalls, deps.parseCalls, deps.refreshCalls)
	}
	statuses := model.SessionStatuses()
	if *statuses["11111111-1111-1111-1111-111111111111"].RemainingSeconds != 3299 {
		t.Fatalf("remaining seconds = %d, want 3299", *statuses["11111111-1111-1111-1111-111111111111"].RemainingSeconds)
	}
	if model.Countdown("11111111-1111-1111-1111-111111111111") != 4 {
		t.Fatalf("countdown = %d, want 4", model.Countdown("11111111-1111-1111-1111-111111111111"))
	}
}

func TestDisplayTickFiresReminderNotificationThreshold(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	last := now.Add(-49 * time.Minute)
	var events []notify.Event
	model := NewModel(Options{
		Now:                now,
		ReminderThresholds: []int{20},
		ReminderEnabled:    map[string]bool{"reminder-id": true},
		Dependencies: Dependencies{
			NotifyEvent: func(event notify.Event) notify.Result {
				events = append(events, event)
				return notify.Result{Delivered: true, Message: "delivered"}
			},
		},
		Sessions: []session.Session{{
			SessionID:     "reminder-id",
			ShortID:       "reminder",
			LastMessageAt: &last,
			CacheWindow: session.CacheWindow{
				Label:      "1h",
				TTLSeconds: 3600,
				Known:      true,
			},
		}},
	})

	updated, cmd := model.Update(DisplayTickMsg{Now: now.Add(time.Second)})
	model = updated.(Model)

	if cmd == nil {
		t.Fatal("DisplayTick returned nil command, want notification command")
	}
	msg := cmd()
	result, ok := msg.(NotificationResultMsg)
	if !ok {
		t.Fatalf("notification command returned %#v, want NotificationResultMsg", msg)
	}
	if result.Event.Kind != notify.EventReminderThresholdCrossed || result.Event.ThresholdPercent != 20 {
		t.Fatalf("event = %#v, want reminder threshold 20", result.Event)
	}
	if len(events) != 1 {
		t.Fatalf("notify events = %d, want 1", len(events))
	}

	updated, cmd = model.Update(DisplayTickMsg{Now: now.Add(2 * time.Second)})
	if cmd != nil {
		t.Fatalf("second tick returned command %#v, want threshold one-shot", cmd())
	}
}

func TestDisplayTickEvaluatesKeepAliveMonitoringSessions(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	last := now.Add(-56 * time.Minute)
	cfg := config.Default().KeepAlive
	cfg.AutoSend = true
	cfg.CountdownSeconds = 30
	model := NewModel(Options{
		Now:             now,
		SelectedID:      "workspace-id",
		KeepAliveConfig: cfg,
		Sessions: []session.Session{{
			SessionID:     "workspace-id",
			ShortID:       "workspace",
			Project:       "workspace-api",
			JSONLPath:     "/tmp/workspace.jsonl",
			LastMessageAt: &last,
			CacheWindow:   session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true},
		}},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateMonitoringIdle, AutoSend: true, TriggerArmed: true, MaxSends: 1},
		},
	})

	updated, cmd := model.Update(DisplayTickMsg{Now: now})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("monitoring tick returned command before countdown elapses, want nil")
	}
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateCountdown {
		t.Fatalf("state = %q, want countdown", got)
	}
	if got := model.Countdown("workspace-id"); got != 30 {
		t.Fatalf("countdown = %d, want 30", got)
	}
}

func TestInitStartsDisplayTickerWhenEnabled(t *testing.T) {
	if cmd := NewModel(Options{}).Init(); cmd != nil {
		t.Fatal("default Init returned ticker command")
	}
	if cmd := NewModel(Options{StartDisplayTicker: true}).Init(); cmd == nil {
		t.Fatal("Init with StartDisplayTicker returned nil command")
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
			RefreshSessions: func(source refresh.Source, generation int) []session.Session {
				if source != refresh.SourceFsnotify {
					t.Fatalf("source = %q, want fsnotify", source)
				}
				return []session.Session{{SessionID: "fsnotify", ShortID: "fsnotify"}}
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
			RefreshSessions: func(source refresh.Source, generation int) []session.Session {
				if source != refresh.SourceSafety {
					t.Fatalf("source = %q, want safety", source)
				}
				if generation != 5 {
					t.Fatalf("generation = %d, want 5", generation)
				}
				return []session.Session{{SessionID: "safety", ShortID: "safety"}}
			},
		},
	})

	updated, cmd := model.Update(SafetyRefreshMsg{})
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
	if !strings.Contains(model.View(), "No Claude Code session files found") {
		t.Fatalf("empty refresh did not render no-session state:\n%s", model.View())
	}
}

func TestSessionStateIsDeepCopiedAcrossModelBoundaries(t *testing.T) {
	input := session.Session{
		SessionID: "input",
		ShortID:   "input",
		Project:   "tmp",
		CacheWindow: session.CacheWindow{
			Evidence: []string{"original-evidence"},
		},
		Gaps: []session.Gap{{Seconds: 60}},
		Warnings: []session.ParseWarning{{
			Code:    session.WarningMalformedJSON,
			Message: "original-warning",
		}},
	}
	model := NewModel(Options{Sessions: []session.Session{input}})
	input.CacheWindow.Evidence[0] = "mutated-evidence"
	input.Gaps[0].Seconds = 999
	input.Warnings[0].Message = "mutated-warning"

	stored := model.Sessions()[0]
	if stored.CacheWindow.Evidence[0] != "original-evidence" {
		t.Fatalf("constructor kept aliased evidence: %#v", stored.CacheWindow.Evidence)
	}
	if stored.Gaps[0].Seconds != 60 {
		t.Fatalf("constructor kept aliased gaps: %#v", stored.Gaps)
	}
	if stored.Warnings[0].Message != "original-warning" {
		t.Fatalf("constructor kept aliased warnings: %#v", stored.Warnings)
	}

	refresh := session.Session{
		SessionID: "refresh",
		ShortID:   "refresh",
		Project:   "tmp",
		CacheWindow: session.CacheWindow{
			Evidence: []string{"refresh-evidence"},
		},
		Gaps:     []session.Gap{{Seconds: 120}},
		Warnings: []session.ParseWarning{{Message: "refresh-warning"}},
	}
	updated, _ := model.Update(RefreshResultMsg{Generation: 1, Sessions: []session.Session{refresh}})
	model = updated.(Model)
	refresh.CacheWindow.Evidence[0] = "mutated-refresh-evidence"
	refresh.Gaps[0].Seconds = 1000
	refresh.Warnings[0].Message = "mutated-refresh-warning"
	stored = model.Sessions()[0]
	if stored.CacheWindow.Evidence[0] != "refresh-evidence" || stored.Gaps[0].Seconds != 120 || stored.Warnings[0].Message != "refresh-warning" {
		t.Fatalf("refresh result aliased caller slices: %#v", stored)
	}

	exposed := model.Sessions()
	exposed[0].CacheWindow.Evidence[0] = "mutated-returned-evidence"
	exposed[0].Gaps[0].Seconds = 2000
	exposed[0].Warnings[0].Message = "mutated-returned-warning"
	stored = model.Sessions()[0]
	if stored.CacheWindow.Evidence[0] != "refresh-evidence" || stored.Gaps[0].Seconds != 120 || stored.Warnings[0].Message != "refresh-warning" {
		t.Fatalf("Sessions() exposed internal slices: %#v", stored)
	}
}

func TestHelpToggleAndQuitKeys(t *testing.T) {
	model := NewModel(Options{})

	updated, cmd := model.Update(keyRunes("?"))
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("? returned command, want nil")
	}
	if !model.HelpOpen() {
		t.Fatal("HelpOpen = false, want true after ?")
	}
	if !strings.Contains(model.View(), "arrows") || !strings.Contains(model.View(), "enter") {
		t.Fatalf("help view missing navigation copy:\n%s", model.View())
	}

	updated, cmd = model.Update(keyRunes("q"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("q returned nil command, want tea.Quit command")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("q command returned %#v, want tea.QuitMsg", msg)
	}
}

func TestAdvertisedShortcutsAreHandledForCurrentRoute(t *testing.T) {
	model := NewModel(Options{})
	for _, shortcut := range []string{"r", "k"} {
		updated, _ := model.Update(keyRunes(shortcut))
		model = updated.(Model)
		if model.LastAction() == "" {
			t.Fatalf("shortcut %q produced no action", shortcut)
		}
	}

	model = NewModel(Options{SelectedID: "11111111"})
	for _, shortcut := range []string{"r", "k", "c", "b"} {
		updated, _ := model.Update(keyRunes(shortcut))
		model = updated.(Model)
		if model.LastAction() == "" {
			t.Fatalf("workspace shortcut %q produced no action", shortcut)
		}
	}

	model = NewModel(Options{SelectedID: "11111111", KeepAliveStatus: KeepAliveCountdown})
	for _, shortcut := range []string{"s", "x"} {
		updated, _ := model.Update(keyRunes(shortcut))
		model = updated.(Model)
		if model.LastAction() == "" {
			t.Fatalf("countdown shortcut %q produced no action", shortcut)
		}
	}

	model = NewModel(Options{StartMode: StartConfig})
	for _, shortcut := range []string{"s", "d"} {
		updated, _ := model.Update(keyRunes(shortcut))
		model = updated.(Model)
		if model.LastAction() == "" {
			t.Fatalf("config shortcut %q produced no action", shortcut)
		}
	}
}

func TestManualRefreshBypassesDebounceWithFreshGeneration(t *testing.T) {
	model := NewModel(Options{
		RefreshGeneration: 2,
		Dependencies: Dependencies{
			RefreshSessions: func(source refresh.Source, generation int) []session.Session {
				if source != refresh.SourceManual {
					t.Fatalf("source = %q, want manual", source)
				}
				if generation != 3 {
					t.Fatalf("generation = %d, want 3", generation)
				}
				return []session.Session{{SessionID: "manual", ShortID: "manual"}}
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
	for _, want := range []string{"Watcher: partial", "new directory permission denied", "Safety refresh: active"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestFocusActionsAreReachable(t *testing.T) {
	model := NewModel(Options{})
	if model.FocusedAction() == "" {
		t.Fatal("initial focused action is empty")
	}

	seen := map[string]bool{model.FocusedAction(): true}
	for i := 0; i < 12; i++ {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
		seen[model.FocusedAction()] = true
	}

	for _, want := range []string{"session", "reminder", "keepalive", "refresh", "help", "quit"} {
		if !seen[want] {
			t.Fatalf("focus action %q was not reachable; saw %#v", want, seen)
		}
	}
}

func TestListCursorMovesRowsAndActionsLinearly(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
	})

	if model.SelectedSessionID() != "newer-id" {
		t.Fatalf("initial selected id = %q, want newer-id", model.SelectedSessionID())
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.SelectedSessionID() != "middle-id" {
		t.Fatalf("selected after down = %q, want middle-id", model.SelectedSessionID())
	}
	updated, _ = model.Update(keyRunes("k"))
	model = updated.(Model)
	if model.SelectedSessionID() != "middle-id" {
		t.Fatalf("k moved selection to %q, want middle-id to remain selected", model.SelectedSessionID())
	}

	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		seen[model.FocusedAction()] = true
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	for _, want := range []string{"session", "reminder", "keepalive", "refresh", "help", "quit"} {
		if !seen[want] {
			t.Fatalf("focus action %q was not reachable; saw %#v", want, seen)
		}
	}
}

func TestEmptyStateFocusOnlyReachesValidActions(t *testing.T) {
	model := NewModel(Options{
		Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
	})

	seen := map[string]bool{}
	for i := 0; i < 6; i++ {
		seen[model.FocusedAction()] = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}

	for _, disallowed := range []string{"session", "reminder", "keepalive"} {
		if seen[disallowed] {
			t.Fatalf("empty state reached invalid action %q; saw %#v", disallowed, seen)
		}
	}
	for _, want := range []string{"refresh", "help", "quit"} {
		if !seen[want] {
			t.Fatalf("empty state did not reach %q; saw %#v", want, seen)
		}
	}
}

func TestListEnterOpensFocusedSession(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
	})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("enter on session returned command, want nil")
	}
	if model.Route() != RouteWorkspace {
		t.Fatalf("route = %q, want workspace", model.Route())
	}
	if model.SelectedSessionID() != "newer-id" {
		t.Fatalf("selected id = %q, want newer-id", model.SelectedSessionID())
	}
	if model.LastAction() != "open_session" {
		t.Fatalf("last action = %q, want open_session", model.LastAction())
	}
}

func TestListSpaceDoesNotOpenSessionRow(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
	})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("space on session returned command, want nil")
	}
	if model.Route() != RouteList {
		t.Fatalf("route = %q, want list", model.Route())
	}
}

func TestAmbiguousEscapeReturnsToList(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:         now,
		AmbiguousID: "d4b",
		Sessions:    listViewSessions(now),
	})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("esc on ambiguous returned command, want nil")
	}
	if model.Route() != RouteList {
		t.Fatalf("route = %q, want list", model.Route())
	}
	if model.LastAction() != "back_to_list" {
		t.Fatalf("last action = %q, want back_to_list", model.LastAction())
	}
}

func TestListFocusableActionsToggleAndActivate(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
		Dependencies: Dependencies{
			RefreshSessions: func(source refresh.Source, generation int) []session.Session {
				return listViewSessions(now.Add(time.Minute))
			},
		},
	})

	model = moveListFocusTo(t, model, "reminder")
	selectedID := model.SelectedSessionID()
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("enter on reminder returned command, want nil")
	}
	if !model.ReminderEnabled(selectedID) {
		t.Fatalf("ReminderEnabled(%s) = false, want true", selectedID)
	}

	model = moveListFocusTo(t, model, "keepalive")
	selectedID = model.SelectedSessionID()
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("enter on keepalive returned command, want nil")
	}
	if !model.KeepAliveEnabled(selectedID) {
		t.Fatalf("KeepAliveEnabled(%s) = false, want true", selectedID)
	}

	model = moveListFocusTo(t, model, "refresh")
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatalf("enter on refresh returned nil command, want manual refresh command")
	}
	if model.LastAction() != "manual_refresh" {
		t.Fatalf("last action = %q, want manual_refresh", model.LastAction())
	}

	model = moveListFocusTo(t, model, "help")
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("enter on help returned command, want nil")
	}
	if !model.HelpOpen() {
		t.Fatal("HelpOpen = false, want true")
	}

	model = moveListFocusTo(t, model, "quit")
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatalf("enter on quit returned nil command, want tea.Quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("quit command returned %#v, want tea.QuitMsg", msg)
	}
}

func TestListAcceleratorsToggleSelectedSession(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
	})

	updated, _ := model.Update(keyRunes("r"))
	model = updated.(Model)
	if !model.ReminderEnabled("newer-id") {
		t.Fatalf("r did not enable reminder for selected session")
	}
	updated, _ = model.Update(keyRunes("r"))
	model = updated.(Model)
	if model.ReminderEnabled("newer-id") {
		t.Fatalf("second r did not disable reminder for selected session")
	}

	updated, _ = model.Update(keyRunes("k"))
	model = updated.(Model)
	if !model.KeepAliveEnabled("newer-id") {
		t.Fatalf("k did not enable KeepAlive for selected session")
	}
	updated, _ = model.Update(keyRunes("k"))
	model = updated.(Model)
	if model.KeepAliveEnabled("newer-id") {
		t.Fatalf("second k did not disable KeepAlive for selected session")
	}
}

func TestWorkspaceFocusOrderAndFocusedActions(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
	})

	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		seen[model.FocusedAction()] = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	for _, want := range []string{"reminder", "keepalive", "keepalive_autosend", "copy_id", "refresh", "help", "back", "quit"} {
		if !seen[want] {
			t.Fatalf("workspace focus action %q was not reachable; saw %#v", want, seen)
		}
	}
}

func TestWorkspaceEnterAndSpaceToggleFocusedControls(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	cfg.AutoSend = true
	model := NewModel(Options{
		Now:             now,
		SelectedID:      "workspace-id",
		Sessions:        []session.Session{workspaceSession(now)},
		KeepAliveConfig: cfg,
	})

	model = moveWorkspaceFocusTo(t, model, "reminder")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("enter on reminder returned command, want nil")
	}
	if !model.ReminderEnabled("workspace-id") {
		t.Fatalf("enter did not toggle Reminder for selected session")
	}

	model = moveWorkspaceFocusTo(t, model, "keepalive")
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("space on KeepAlive returned command, want nil")
	}
	if !model.KeepAliveEnabled("workspace-id") {
		t.Fatalf("space did not toggle KeepAlive for selected session")
	}

	model = moveWorkspaceFocusTo(t, model, "keepalive_autosend")
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("enter on Auto-send returned command, want nil")
	}
	if model.KeepAliveState("workspace-id").AutoSend {
		t.Fatalf("enter did not toggle Auto-send off for selected session")
	}
}

func TestWorkspaceAutoSendTogglePreflightsClaudeAvailability(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	cfg.AutoSend = false
	model := NewModel(Options{
		Now:             now,
		SelectedID:      "workspace-id",
		Sessions:        []session.Session{workspaceSession(now)},
		KeepAliveConfig: cfg,
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, InstanceToken: 5, MaxSends: 1},
		},
		Dependencies: Dependencies{
			CheckClaudeAvailable: func() error { return errForTest("claude command not found") },
		},
	})

	model = moveWorkspaceFocusTo(t, model, "keepalive_autosend")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("auto-send toggle returned command, want nil")
	}
	state := model.KeepAliveState("workspace-id")
	if state.State != keepalive.StateErrorNoClaude || state.AutoSend {
		t.Fatalf("state = %#v, want no-claude and auto-send stopped", state)
	}
	if !strings.Contains(model.View(), "claude unavailable: claude command not found") {
		t.Fatalf("view missing claude unavailable banner:\n%s", model.View())
	}
}

func TestWorkspaceManualRefreshIsFocusableAndUsesSelectedIDSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int) RefreshSnapshot {
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

	model = moveWorkspaceFocusTo(t, model, "refresh")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("enter on workspace manual refresh returned nil command")
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
			RefreshSelectedSnapshot: func(source refresh.Source, generation int, selected session.Session) RefreshSnapshot {
				selectedCalls++
				if selected.SessionID != "workspace-id" {
					t.Fatalf("selected refresh got %q, want workspace-id", selected.SessionID)
				}
				return RefreshSnapshot{Sessions: []session.Session{workspaceSession(now.Add(time.Minute))}, HasRefresh: true}
			},
			RefreshSnapshot: func(source refresh.Source, generation int) RefreshSnapshot {
				fullCalls++
				return RefreshSnapshot{Sessions: []session.Session{listViewSession("other-id", "other", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "")}, HasRefresh: true}
			},
		},
	})

	model = moveWorkspaceFocusTo(t, model, "refresh")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
			RefreshSnapshot: func(source refresh.Source, generation int) RefreshSnapshot {
				return RefreshSnapshot{
					Sessions: []session.Session{
						listViewSession("other-id", "other-mutated", now.Add(time.Hour), now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
						workspaceSession(now.Add(time.Minute)),
					},
					HasRefresh: true,
				}
			},
		},
	})

	model = moveWorkspaceFocusTo(t, model, "refresh")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
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

func TestWorkspaceFailureKeepsAutosendFocusableWithDisabledReason(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {
				SessionID:   "workspace-id",
				State:       keepalive.StateErrorNoClaude,
				AutoSend:    false,
				LastFailure: "claude command not found",
				MaxSends:    1,
			},
		},
	})

	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		seen[model.FocusedAction()] = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	if !seen["keepalive_autosend"] {
		t.Fatalf("failure focus did not include disabled Auto-send setting; saw %#v", seen)
	}
	if !strings.Contains(model.View(), "Auto-send stopped") {
		t.Fatalf("failure view missing disabled reason:\n%s", model.View())
	}
}

func TestWorkspaceSendingAndConfirmingShowAutosendDisabledReason(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for _, state := range []keepalive.State{keepalive.StateSending, keepalive.StateConfirming} {
		model := NewModel(Options{
			Now:        now,
			SelectedID: "workspace-id",
			Sessions:   []session.Session{workspaceSession(now)},
			KeepAliveStates: map[string]keepalive.SessionState{
				"workspace-id": {SessionID: "workspace-id", State: state, AutoSend: true, InstanceToken: 7, MaxSends: 1},
			},
		})
		if !strings.Contains(model.View(), "Auto-send disabled while") {
			t.Fatalf("%s view missing Auto-send disabled reason:\n%s", state, model.View())
		}
	}
}

func TestWorkspaceEvidenceFocusOnlyWhenOverflowing(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	normal := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
	})
	if normal.FocusedAction() == "evidence" {
		t.Fatalf("normal evidence focused, want controls first")
	}

	overflowSession := workspaceSession(now)
	for i := 0; i < 12; i++ {
		overflowSession.Warnings = append(overflowSession.Warnings, session.ParseWarning{Message: "warning"})
		overflowSession.CacheWindow.Evidence = append(overflowSession.CacheWindow.Evidence, "extra")
	}
	overflow := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{overflowSession},
	})
	if overflow.FocusedAction() != "evidence" {
		t.Fatalf("overflow focus = %q, want evidence", overflow.FocusedAction())
	}
	if !strings.Contains(overflow.View(), "Evidence scroll") || !strings.Contains(overflow.View(), "arrows scroll evidence") {
		t.Fatalf("overflow evidence missing scroll affordance:\n%s", overflow.View())
	}
}

func TestWorkspaceShortcutAvailabilityAndDefaultActiveFocus(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		state  keepalive.SessionState
		focus  string
		sendOK bool
		xOK    bool
	}{
		{name: "countdown", state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateCountdown, AutoSend: true, InstanceToken: 1, MaxSends: 1}, focus: "keepalive_send_now", sendOK: true, xOK: true},
		{name: "manual", state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, InstanceToken: 2, MaxSends: 1}, focus: "keepalive_send_now", sendOK: true, xOK: true},
		{name: "confirming", state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateConfirming, AutoSend: true, InstanceToken: 3, MaxSends: 1}, focus: "keepalive_stop_waiting", sendOK: false, xOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel(Options{
				Now:        now,
				SelectedID: "workspace-id",
				Sessions:   []session.Session{workspaceSession(now)},
				KeepAliveStates: map[string]keepalive.SessionState{
					"workspace-id": tc.state,
				},
			})
			if model.FocusedAction() != tc.focus {
				t.Fatalf("initial active focus = %q, want %q", model.FocusedAction(), tc.focus)
			}
			updated, _ := model.Update(keyRunes("s"))
			afterS := updated.(Model)
			if tc.sendOK && afterS.LastAction() != "send_keepalive_now" {
				t.Fatalf("s in %s last action = %q, want send", tc.name, afterS.LastAction())
			}
			if !tc.sendOK && afterS.LastAction() == "send_keepalive_now" {
				t.Fatalf("s in %s unexpectedly sent", tc.name)
			}
			updated, _ = model.Update(keyRunes("x"))
			afterX := updated.(Model)
			if tc.xOK && afterX.LastAction() != "cancel_keepalive" {
				t.Fatalf("x in %s last action = %q, want cancel", tc.name, afterX.LastAction())
			}
		})
	}

	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
	})
	updated, _ := model.Update(keyRunes("s"))
	if updated.(Model).LastAction() == "send_keepalive_now" {
		t.Fatalf("s sent while no KeepAlive send action was available")
	}
	updated, _ = model.Update(keyRunes("x"))
	if updated.(Model).LastAction() == "cancel_keepalive" {
		t.Fatalf("x canceled while no KeepAlive instance was available")
	}
}

func TestWorkspaceIgnoresStaleKeepAliveAsyncMessages(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	state := keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateCountdown, AutoSend: true, InstanceToken: 41, MaxSends: 1}
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": state,
		},
	})

	updated, _ := model.Update(keyRunes("x"))
	model = updated.(Model)
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateCancelledInstance {
		t.Fatalf("x state = %q, want cancelled", got)
	}

	for _, msg := range []tea.Msg{
		KeepAliveCountdownElapsedMsg{SessionID: "workspace-id", InstanceToken: 41, Now: now.Add(30 * time.Second)},
		KeepAliveRunnerResultMsg{SessionID: "workspace-id", InstanceToken: 41, StartedAt: now.Add(time.Second)},
		KeepAliveConfirmationResultMsg{SessionID: "workspace-id", InstanceToken: 41, ConfirmedAt: now.Add(time.Minute)},
	} {
		updated, _ = model.Update(msg)
		model = updated.(Model)
		if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateCancelledInstance {
			t.Fatalf("stale msg %#v changed state to %q, want cancelled", msg, got)
		}
	}

	switched, _ := model.Update(RefreshResultMsg{
		Generation: 1,
		Sessions:   []session.Session{listViewSession("other-id", "other", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "")},
	})
	model = switched.(Model)
	updated, _ = model.Update(KeepAliveConfirmationResultMsg{SessionID: "workspace-id", InstanceToken: 41, ConfirmedAt: now.Add(time.Minute)})
	model = updated.(Model)
	if model.LastAction() == "keepalive_confirmed" {
		t.Fatalf("stale confirmation after session switch was applied")
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

func TestWorkspaceKeepAliveActionsProduceRunnerAndConfirmationCommands(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	runner := fakeKeepAliveRunner{startedAt: now.Add(time.Second)}
	confirmCalls := 0
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, InstanceToken: 21, MaxSends: 1},
		},
		Dependencies: Dependencies{
			KeepAliveRunner: runner,
			ConfirmKeepAlive: func(ctx context.Context, target keepalive.ConfirmationTarget) (keepalive.ConfirmationResult, error) {
				confirmCalls++
				if target.Path == "" {
					t.Fatalf("confirmation target path is empty")
				}
				return keepalive.ConfirmationResult{Confirmed: true, ConfirmedAt: now.Add(2 * time.Second)}, nil
			},
		},
	})

	updated, cmd := model.Update(keyRunes("s"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("send now returned nil command, want runner command")
	}
	runnerMsg, ok := cmd().(KeepAliveRunnerResultMsg)
	if !ok {
		t.Fatalf("runner command returned %#v", runnerMsg)
	}
	updated, cmd = model.Update(runnerMsg)
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("runner success returned nil command, want confirmation command")
	}
	confirmMsg, ok := cmd().(KeepAliveConfirmationResultMsg)
	if !ok {
		t.Fatalf("confirmation command returned %#v", confirmMsg)
	}
	if confirmCalls != 1 {
		t.Fatalf("confirmation calls = %d, want 1", confirmCalls)
	}
	updated, _ = model.Update(confirmMsg)
	model = updated.(Model)
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateSuccess {
		t.Fatalf("state = %q, want success", got)
	}
}

func TestInitialRoutes(t *testing.T) {
	if route := NewModel(Options{}).Route(); route != RouteList {
		t.Fatalf("default route = %q, want list", route)
	}
	if route := NewModel(Options{SelectedID: "11111111"}).Route(); route != RouteWorkspace {
		t.Fatalf("selected id route = %q, want workspace", route)
	}
	if route := NewModel(Options{AmbiguousID: "111"}).Route(); route != RouteAmbiguous {
		t.Fatalf("ambiguous id route = %q, want ambiguous", route)
	}
	if route := NewModel(Options{StartMode: StartConfig}).Route(); route != RouteConfig {
		t.Fatalf("config route = %q, want config", route)
	}
}

type fakeDeps struct {
	discoverCalls int
	parseCalls    int
	refreshCalls  int
}

type errForTest string

func (e errForTest) Error() string {
	return string(e)
}

type fakeKeepAliveRunner struct {
	startedAt time.Time
	err       error
}

func (r fakeKeepAliveRunner) Available() error {
	return r.err
}

func (r fakeKeepAliveRunner) Send(context.Context, keepalive.RunRequest) keepalive.RunResult {
	return keepalive.RunResult{StartedAt: r.startedAt, Err: r.err}
}

func (d *fakeDeps) discover() {
	d.discoverCalls++
}

func (d *fakeDeps) parse() {
	d.parseCalls++
}

func (d *fakeDeps) refresh() {
	d.refreshCalls++
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func moveListFocusTo(t *testing.T, model Model, action string) Model {
	t.Helper()
	for i := 0; i < 20; i++ {
		if model.FocusedAction() == action {
			return model
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	t.Fatalf("could not move focus to %q; focused %q", action, model.FocusedAction())
	return model
}

func moveWorkspaceFocusTo(t *testing.T, model Model, action string) Model {
	t.Helper()
	for i := 0; i < 30; i++ {
		if model.FocusedAction() == action {
			return model
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	t.Fatalf("could not move workspace focus to %q; focused %q", action, model.FocusedAction())
	return model
}
