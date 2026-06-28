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
	refreshCalls := 0
	model := NewModel(Options{
		Now: now,
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				refreshCalls++
				return RefreshSnapshot{}
			},
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
	if refreshCalls != 0 {
		t.Fatalf("display tick called refresh %d time(s)", refreshCalls)
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

func TestInitStartsDisplayTickerWhenEnabled(t *testing.T) {
	if cmd := NewModel(Options{}).Init(); cmd != nil {
		t.Fatal("default Init returned ticker command")
	}
	if cmd := NewModel(Options{StartDisplayTicker: true}).Init(); cmd == nil {
		t.Fatal("Init with StartDisplayTicker returned nil command")
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

	model = NewModel(Options{
		SelectedID: "11111111",
		Sessions:   []session.Session{{SessionID: "11111111", ShortID: "11111111"}},
		KeepAliveStates: map[string]keepalive.SessionState{
			"11111111": {SessionID: "11111111", State: keepalive.StateCountdown, InstanceToken: 1, MaxSends: 1},
		},
	})
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

func TestFocusActionAlwaysHasVisibleMarker(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	cfg.AutoSend = false
	cases := []struct {
		name    string
		options Options
	}{
		{
			name: "list",
			options: Options{
				Now:      now,
				Sessions: listViewSessions(now),
			},
		},
		{
			name: "workspace",
			options: Options{
				Now:        now,
				SelectedID: "workspace-id",
				Sessions:   []session.Session{workspaceSession(now)},
			},
		},
		{
			name: "keepalive active",
			options: Options{
				Now:             now,
				SelectedID:      "workspace-id",
				Sessions:        []session.Session{workspaceSession(now)},
				KeepAliveConfig: cfg,
				KeepAliveStates: map[string]keepalive.SessionState{
					"workspace-id": {SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, MaxSends: 1, InstanceToken: 8},
				},
			},
		},
		{
			name: "config",
			options: Options{
				StartMode: StartConfig,
				Config:    config.Default(),
			},
		},
		{
			name: "empty",
			options: Options{
				Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
			},
		},
		{
			name: "ambiguous",
			options: Options{
				Now:         now,
				AmbiguousID: "d4b",
				Sessions:    listViewSessions(now),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel(tc.options)
			if model.FocusedAction() == "" {
				t.Fatal("focused action is empty")
			}
			if !viewHasVisibleFocusMarker(model.View()) {
				t.Fatalf("focused action %q has no visible marker:\n%s", model.FocusedAction(), model.View())
			}
		})
	}
}

func TestListCursorMovesSessionsOnly(t *testing.T) {
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
	seenIDs := map[string]bool{}
	for i := 0; i < 8; i++ {
		seen[model.FocusedAction()] = true
		seenIDs[model.SelectedSessionID()] = true
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	if len(seen) != 1 || !seen["session"] {
		t.Fatalf("list cursor reached non-session focus actions: %#v", seen)
	}
	for _, want := range []string{"newer-id", "middle-id", "older-id"} {
		if !seenIDs[want] {
			t.Fatalf("list cursor did not reach session %q; saw %#v", want, seenIDs)
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
	if !viewHasVisibleFocusMarker(model.View()) {
		t.Fatalf("empty state focused action %q has no visible marker:\n%s", model.FocusedAction(), model.View())
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

func TestListDirectKeysToggleAndActivate(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				return RefreshSnapshot{Sessions: listViewSessions(now.Add(time.Minute))}
			},
		},
	})

	selectedID := model.SelectedSessionID()
	updated, cmd := model.Update(keyRunes("r"))
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("r returned command, want nil")
	}
	if !model.ReminderEnabled(selectedID) {
		t.Fatalf("ReminderEnabled(%s) = false, want true", selectedID)
	}

	selectedID = model.SelectedSessionID()
	updated, cmd = model.Update(keyRunes("k"))
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("k returned command, want nil")
	}
	if !model.KeepAliveEnabled(selectedID) {
		t.Fatalf("KeepAliveEnabled(%s) = false, want true", selectedID)
	}

	updated, cmd = model.Update(keyRunes("u"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatalf("u returned nil command, want manual refresh command")
	}
	if model.LastAction() != "manual_refresh" {
		t.Fatalf("last action = %q, want manual_refresh", model.LastAction())
	}

	updated, cmd = model.Update(keyRunes("?"))
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("? returned command, want nil")
	}
	if !model.HelpOpen() {
		t.Fatal("HelpOpen = false, want true")
	}

	updated, cmd = model.Update(keyRunes("q"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatalf("q returned nil command, want tea.Quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("quit command returned %#v, want tea.QuitMsg", msg)
	}
}

func TestListAcceleratorsToggleSelectedSession(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now: now,
		Sessions: []session.Session{
			listViewSession("top-id", "top", now, now.Add(-2*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
			listViewSession("target-id", "target", now.Add(-time.Minute), now.Add(-5*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
		},
	})
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.SelectedSessionID() != "target-id" {
		t.Fatalf("setup selected %q, want target-id", model.SelectedSessionID())
	}

	updated, _ = model.Update(keyRunes("r"))
	model = updated.(Model)
	if !model.ReminderEnabled("target-id") {
		t.Fatalf("r did not enable reminder for selected session")
	}
	if model.SelectedSessionID() != "target-id" || model.FocusedAction() != "session" {
		t.Fatalf("r changed list focus/selection to action %q selected %q", model.FocusedAction(), model.SelectedSessionID())
	}
	updated, _ = model.Update(keyRunes("r"))
	model = updated.(Model)
	if model.ReminderEnabled("target-id") {
		t.Fatalf("second r did not disable reminder for selected session")
	}

	updated, _ = model.Update(keyRunes("k"))
	model = updated.(Model)
	if !model.KeepAliveEnabled("target-id") {
		t.Fatalf("k did not enable KeepAlive for selected session")
	}
	if model.SelectedSessionID() != "target-id" || model.FocusedAction() != "session" {
		t.Fatalf("k changed list focus/selection to action %q selected %q", model.FocusedAction(), model.SelectedSessionID())
	}
	updated, _ = model.Update(keyRunes("k"))
	model = updated.(Model)
	if model.KeepAliveEnabled("target-id") {
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
	for _, want := range []string{"reminder", "keepalive", "keepalive_autosend", "copy_id", "back"} {
		if !seen[want] {
			t.Fatalf("workspace focus action %q was not reachable; saw %#v", want, seen)
		}
	}
	for _, hidden := range []string{"evidence", "refresh", "help", "quit"} {
		if seen[hidden] {
			t.Fatalf("workspace focus reached hidden action %q; saw %#v", hidden, seen)
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
	if model.FocusedAction() != "keepalive" {
		t.Fatalf("KeepAlive toggle moved focus to %q, want keepalive", model.FocusedAction())
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

func TestWorkspaceActionFeedbackForUpdateCopyAndCancelWatching(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(source refresh.Source, generation int, selected *session.Session) RefreshSnapshot {
				if selected == nil {
					t.Fatalf("selected refresh input = nil")
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

	updated, cmd := model.Update(keyRunes("u"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("manual update returned nil command")
	}
	if !strings.Contains(model.View(), "updating selected session") {
		t.Fatalf("manual update missing visible feedback:\n%s", model.View())
	}

	updated, _ = model.Update(keyRunes("c"))
	model = updated.(Model)
	if !strings.Contains(model.View(), "Session ID shown") || !strings.Contains(model.View(), "workspace-id") {
		t.Fatalf("copy id missing visible feedback:\n%s", model.View())
	}

	state := keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateMonitoringIdle, AutoSend: true, MaxSends: 1}
	model = NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": state,
		},
	})
	model = moveWorkspaceFocusTo(t, model, "keepalive")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.KeepAliveEnabled("workspace-id") {
		t.Fatalf("cancel watching left KeepAlive enabled: %#v", model.KeepAliveState("workspace-id"))
	}
	if !strings.Contains(model.View(), "KeepAlive cancelled") {
		t.Fatalf("cancel watching missing visible feedback:\n%s", model.View())
	}
}

func TestTransientNoticesClearAfterDisplayTick(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	expiredLast := now.Add(-2 * time.Hour)
	expired := workspaceSession(now)
	expired.LastMessageAt = &expiredLast
	expired.CacheWindow = session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true}
	model := NewModel(Options{
		Now:        now,
		SelectedID: expired.SessionID,
		Sessions:   []session.Session{expired},
	})

	updated, _ := model.Update(keyRunes("k"))
	model = updated.(Model)
	if !strings.Contains(model.View(), "KeepAlive unavailable after expiry") {
		t.Fatalf("missing transient expiry notice:\n%s", model.View())
	}

	updated, _ = model.Update(DisplayTickMsg{Now: now.Add(4 * time.Second)})
	model = updated.(Model)
	if strings.Contains(model.View(), "KeepAlive unavailable after expiry") {
		t.Fatalf("transient expiry notice did not clear:\n%s", model.View())
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
	if !strings.Contains(model.View(), "disabled after failure") {
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
		if !strings.Contains(model.View(), "disabled while") {
			t.Fatalf("%s view missing Auto-send disabled reason:\n%s", state, model.View())
		}
	}
}

func TestWorkspaceDetailsDisclosureDoesNotBecomeFocusRow(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	normal := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
	})
	if normal.FocusedAction() != "reminder" {
		t.Fatalf("normal focus = %q, want controls first", normal.FocusedAction())
	}

	overflowSession := workspaceSession(now)
	for i := 0; i < 12; i++ {
		overflowSession.Warnings = append(overflowSession.Warnings, session.ParseWarning{Message: "warning"})
		overflowSession.CacheWindow.Evidence = append(overflowSession.CacheWindow.Evidence, "extra")
		overflowSession.Gaps = append(overflowSession.Gaps, session.Gap{
			Seconds: float64(60 + i),
			From:    now.Add(-time.Duration(i+2) * time.Minute),
			To:      now.Add(-time.Duration(i+1) * time.Minute),
		})
	}
	overflow := NewModel(Options{
		Now:        now,
		Height:     24,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{overflowSession},
	})
	if overflow.FocusedAction() != "reminder" {
		t.Fatalf("overflow focus = %q, want reminder", overflow.FocusedAction())
	}
	updated, _ := overflow.Update(keyRunes("v"))
	overflow = updated.(Model)
	if overflow.FocusedAction() != "details_scroll" {
		t.Fatalf("details disclosure focus = %q, want details_scroll", overflow.FocusedAction())
	}
	if !strings.Contains(overflow.View(), "Session Info · details") {
		t.Fatalf("details disclosure did not render expanded session info:\n%s", overflow.View())
	}
	before := overflow.View()
	updated, _ = overflow.Update(tea.KeyMsg{Type: tea.KeyDown})
	overflow = updated.(Model)
	after := overflow.View()
	if before == after || !strings.Contains(after, "more gap(s)") {
		t.Fatalf("details scroll did not change bounded gap window:\nbefore:\n%s\nafter:\n%s", before, after)
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

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func viewHasVisibleFocusMarker(view string) bool {
	clean := stripANSI(view)
	return strings.Contains(clean, "›") || strings.Contains(clean, ">")
}

func focusedListLine(view string) string {
	for _, line := range strings.Split(view, "\n") {
		clean := stripANSI(line)
		if strings.Contains(clean, "› #") {
			return clean
		}
	}
	return ""
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

func moveConfigFocusTo(t *testing.T, model Model, action string) Model {
	t.Helper()
	for i := 0; i < 30; i++ {
		if model.FocusedAction() == action {
			return model
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	t.Fatalf("could not move config focus to %q; focused %q", action, model.FocusedAction())
	return model
}
