package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rcverse/cc-watch/internal/config"
	"github.com/rcverse/cc-watch/internal/keepalive"
	"github.com/rcverse/cc-watch/internal/notify"
	"github.com/rcverse/cc-watch/internal/session"
)

func TestDisplayTickRecomputesTimeOnly(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	last := now.Add(-5 * time.Minute)
	refreshCalls := 0
	model := NewModel(Options{
		Now: now,
		Dependencies: Dependencies{
			RefreshSnapshot: func(selected *session.Session) RefreshSnapshot {
				refreshCalls++
				return RefreshSnapshot{}
			},
		},
		Sessions: []session.Session{{
			SessionID:     "11111111-1111-1111-1111-111111111111",
			ShortID:       "11111111",
			Project:       "tmp",
			CacheAnchorAt: &last,
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
	status := model.sessions[0].StatusAt(model.now)
	if *status.RemainingSeconds != 3299 {
		t.Fatalf("remaining seconds = %d, want 3299", *status.RemainingSeconds)
	}
	if model.countdowns["11111111-1111-1111-1111-111111111111"] != 4 {
		t.Fatalf("countdown = %d, want 4", model.countdowns["11111111-1111-1111-1111-111111111111"])
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
			CacheAnchorAt: &last,
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
	if result.Event.Kind != notify.EventReminderThresholdCrossed || result.Event.ThresholdPercent != 20 || result.Event.ShortID != "reminder" {
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

func TestSessionGapsAreDeepCopiedAcrossModelBoundaries(t *testing.T) {
	input := session.Session{
		SessionID: "input",
		ShortID:   "input",
		Project:   "tmp",
		Gaps:      []session.Gap{{Seconds: 60}},
	}
	model := NewModel(Options{Sessions: []session.Session{input}})
	input.Gaps[0].Seconds = 999

	stored := model.sessions[0]
	if stored.Gaps[0].Seconds != 60 {
		t.Fatalf("constructor kept aliased gaps: %#v", stored.Gaps)
	}

	refresh := session.Session{
		SessionID: "refresh",
		ShortID:   "refresh",
		Project:   "tmp",
		Gaps:      []session.Gap{{Seconds: 120}},
	}
	updated, _ := model.Update(RefreshResultMsg{Generation: 1, Sessions: []session.Session{refresh}})
	model = updated.(Model)
	refresh.Gaps[0].Seconds = 1000
	stored = model.sessions[0]
	if stored.Gaps[0].Seconds != 120 {
		t.Fatalf("refresh result aliased caller slices: %#v", stored)
	}

}

func TestQuitKey(t *testing.T) {
	model := NewModel(Options{})

	updated, cmd := model.Update(keyRunes("q"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("q returned nil command, want tea.Quit command")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Fatalf("q command returned %#v, want tea.QuitMsg", msg)
	}
}

func TestQuitKeyIsTopLevelOnlyAndCtrlCIsGlobal(t *testing.T) {
	for _, model := range []Model{
		NewModel(Options{StartMode: StartConfig}),
		openStatuslineSettings(t, NewModel(Options{StartMode: StartConfig})),
	} {
		updated, cmd := model.Update(keyRunes("q"))
		if cmd != nil {
			t.Fatalf("q on %s returned quit command", model.route)
		}
		if updated.(Model).route != model.route {
			t.Fatalf("q on %s changed route to %s", model.route, updated.(Model).route)
		}

		updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Fatalf("ctrl-c on %s returned nil command", model.route)
		}
		if msg := cmd(); msg != (tea.QuitMsg{}) {
			t.Fatalf("ctrl-c on %s returned %#v, want tea.QuitMsg", model.route, msg)
		}
		_ = updated
	}
}

func TestFocusActionAlwaysHasVisibleMarker(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
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
				Now:              now,
				SelectedID:       "workspace-id",
				Sessions:         []session.Session{workspaceSession(now)},
				KeepAliveConfig:  cfg,
				KeepAliveManager: keepAliveManagerInState(keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StatePaused, MaxSends: 1, InstanceToken: 8}),
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
	for _, want := range []string{"refresh", "quit"} {
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
	if model.route != RouteWorkspace {
		t.Fatalf("route = %q, want workspace", model.route)
	}
	if model.SelectedSessionID() != "newer-id" {
		t.Fatalf("selected id = %q, want newer-id", model.SelectedSessionID())
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
	if model.route != RouteList {
		t.Fatalf("route = %q, want list", model.route)
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
	if model.route != RouteList {
		t.Fatalf("route = %q, want list", model.route)
	}
}

func TestListDirectKeysToggleAndActivate(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Sessions: listViewSessions(now),
		Dependencies: Dependencies{
			RefreshSnapshot: func(selected *session.Session) RefreshSnapshot {
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
	if !model.reminderEnabled[selectedID] {
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
	if !model.reminderEnabled["target-id"] {
		t.Fatalf("r did not enable reminder for selected session")
	}
	if model.SelectedSessionID() != "target-id" || model.FocusedAction() != "session" {
		t.Fatalf("r changed list focus/selection to action %q selected %q", model.FocusedAction(), model.SelectedSessionID())
	}
	updated, _ = model.Update(keyRunes("r"))
	model = updated.(Model)
	if model.reminderEnabled["target-id"] {
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

func TestDirectKeepAliveShortcutStartsCountdownWithoutNotification(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	s := workspaceSession(now)
	last := now.Add(-56 * time.Minute)
	s.CacheAnchorAt = &last
	model := NewModel(Options{
		Now:             now,
		Sessions:        []session.Session{s},
		KeepAliveConfig: cfg,
		Dependencies: Dependencies{
			NotifyEvent: func(event notify.Event) notify.Result {
				return notify.Result{Delivered: true, Message: "delivered"}
			},
		},
	})

	updated, cmd := model.Update(keyRunes("k"))
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("k returned command %#v, want countdown to stay TUI-only", cmd())
	}
	if !model.KeepAliveEnabled("workspace-id") {
		t.Fatalf("k did not enable KeepAlive for workspace-id")
	}
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateCountdown {
		t.Fatalf("state = %q, want countdown", got)
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
	for _, want := range []string{"reminder", "keepalive", "back"} {
		if !seen[want] {
			t.Fatalf("workspace focus action %q was not reachable; saw %#v", want, seen)
		}
	}
	for _, hidden := range []string{"copy_id", "evidence", "refresh", "quit"} {
		if seen[hidden] {
			t.Fatalf("workspace focus reached hidden action %q; saw %#v", hidden, seen)
		}
	}
}

func TestWorkspaceEnterAndSpaceToggleFocusedControls(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
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
	if !model.reminderEnabled["workspace-id"] {
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

	backModel := NewModel(Options{
		Now:        now,
		Width:      120,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
	})
	backModel = moveWorkspaceFocusTo(t, backModel, "back")
	updated, cmd = backModel.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd != nil || updated.(Model).route != RouteList {
		t.Fatalf("space on Back = route %q, command %v; want list with no command", updated.(Model).route, cmd)
	}

}

func TestWorkspaceResetLimitRearmsKeepAlive(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveManager: keepAliveManagerInState(keepalive.SessionState{
			SessionID: "workspace-id",
			State:     keepalive.StateScopeComplete,
			ScopeUsed: 5,
			MaxSends:  5,
		}),
	})
	model = moveWorkspaceFocusTo(t, model, "keepalive_reset_limit")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("reset limit returned command, want nil")
	}
	got := model.KeepAliveState("workspace-id")
	if got.State != keepalive.StateMonitoringIdle || got.ScopeUsed != 0 {
		t.Fatalf("state = %#v, want armed zero sends", got)
	}
	if !strings.Contains(model.View(), "✓ KeepAlive limit reset") {
		t.Fatalf("reset limit missing notice:\n%s", model.View())
	}
}

func TestWorkspaceActionFeedbackForUpdateAndCancelWatching(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		Dependencies: Dependencies{
			RefreshSnapshot: func(selected *session.Session) RefreshSnapshot {
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
	if strings.Contains(model.View(), "Session ID shown") {
		t.Fatalf("workspace c still triggers Copy ID feedback/action:\n%s", model.View())
	}

	state := keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateMonitoringIdle, MaxSends: 1}
	model = NewModel(Options{
		Now:              now,
		SelectedID:       "workspace-id",
		Sessions:         []session.Session{workspaceSession(now)},
		KeepAliveManager: keepAliveManagerInState(state),
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
	expired.CacheAnchorAt = &expiredLast
	expired.CacheWindow = session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true}
	model := NewModel(Options{
		Now:        now,
		SelectedID: expired.SessionID,
		Sessions:   []session.Session{expired},
	})

	updated, _ := model.Update(keyRunes("k"))
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "Notice") || !strings.Contains(view, "KeepAlive N/A after expiry") {
		t.Fatalf("missing transient expiry notice:\n%s", view)
	}
	assertOrder(t, view, "Controls", "Notice", "KeepAlive N/A after expiry")
	if strings.Index(view, "KeepAlive N/A after expiry") < strings.Index(view, "Cache Status") {
		t.Fatalf("transient notice rendered above workspace content:\n%s", view)
	}

	updated, _ = model.Update(DisplayTickMsg{Now: now.Add(4 * time.Second)})
	model = updated.(Model)
	view = model.View()
	if strings.Contains(view, "KeepAlive N/A after expiry") || strings.Contains(view, "Notice") {
		t.Fatalf("transient expiry notice did not clear:\n%s", view)
	}
}

func TestExpiredWorkspaceReminderIsNAAndCannotToggle(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	expiredLast := now.Add(-2 * time.Hour)
	expired := workspaceSession(now)
	expired.CacheAnchorAt = &expiredLast
	expired.CacheWindow = session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true}
	model := NewModel(Options{
		Now:             now,
		SelectedID:      expired.SessionID,
		Sessions:        []session.Session{expired},
		ReminderEnabled: map[string]bool{expired.SessionID: true},
	})

	view := stripANSI(model.View())
	if !strings.Contains(view, "Reminder") || !strings.Contains(view, "N/A  after expiry") {
		t.Fatalf("expired workspace should show Reminder N/A:\n%s", model.View())
	}

	model = moveWorkspaceFocusTo(t, model, "reminder")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("expired reminder toggle returned command, want nil")
	}
	if model.reminderEnabled[expired.SessionID] {
		t.Fatalf("expired reminder stayed enabled after blocked toggle")
	}
	if !strings.Contains(model.View(), "Reminder N/A after expiry") {
		t.Fatalf("expired reminder toggle missing notice:\n%s", model.View())
	}
}

func TestWorkspaceFailureOffersResetLimit(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveManager: keepAliveManagerInState(keepalive.SessionState{
			SessionID:   "workspace-id",
			State:       keepalive.StateErrorNoClaude,
			LastFailure: "claude command not found",
			ScopeUsed:   1,
			MaxSends:    1,
		}),
	})

	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		seen[model.FocusedAction()] = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	if !seen["keepalive_reset_limit"] {
		t.Fatalf("failure focus did not include reset limit; saw %#v", seen)
	}
	if !strings.Contains(model.View(), "Reset limit") {
		t.Fatalf("failure view missing reset limit:\n%s", model.View())
	}
}

func TestWorkspaceSendingAndConfirmingShowTransientKeepAliveStatus(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for _, state := range []keepalive.State{keepalive.StateSending, keepalive.StateConfirming} {
		model := NewModel(Options{
			Now:              now,
			SelectedID:       "workspace-id",
			Sessions:         []session.Session{workspaceSession(now)},
			KeepAliveManager: keepAliveManagerInState(keepalive.SessionState{SessionID: "workspace-id", State: state, InstanceToken: 7, MaxSends: 1}),
		})
		if !strings.Contains(model.View(), "KeepAlive · ✓ Armed") {
			t.Fatalf("%s view missing armed chip:\n%s", state, model.View())
		}
		if !strings.Contains(model.View(), "Sending message now") && !strings.Contains(model.View(), "Checking for confirmation") {
			t.Fatalf("%s view missing transient send/check text:\n%s", state, model.View())
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
	overflowSession.WarningCount = 12
	for i := 0; i < 12; i++ {
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
	if before == after || overflow.FocusedAction() != "details_scroll" {
		t.Fatalf("expanded details did not scroll in place:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	before = overflow.View()
	updated, _ = overflow.Update(tea.KeyMsg{Type: tea.KeySpace})
	overflow = updated.(Model)
	if overflow.View() != before {
		t.Fatalf("space should not change expanded details scroll mode:\nbefore:\n%s\nafter:\n%s", before, overflow.View())
	}
	for i := 0; i < 20; i++ {
		updated, _ = overflow.Update(tea.KeyMsg{Type: tea.KeyDown})
		overflow = updated.(Model)
	}
	if overflow.FocusedAction() != "details_scroll" || strings.Contains(overflow.View(), "› Session Info · details") {
		t.Fatalf("details scroll boundary moved focus out of details:\n%s", overflow.View())
	}
}

func TestWorkspaceFooterNamesBackAndActionKeys(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for _, width := range []int{80, 120} {
		model := NewModel(Options{
			Now:        now,
			Width:      width,
			SelectedID: "workspace-id",
			Sessions:   []session.Session{workspaceSession(now)},
		})
		view := stripANSI(model.View())
		for _, want := range []string{"Enter/Space act", "Esc back", "q quit"} {
			if !strings.Contains(view, want) {
				t.Fatalf("width %d workspace footer missing %q:\n%s", width, want, view)
			}
		}
		if strings.Contains(view, "⎋") || strings.Contains(view, "b/") {
			t.Fatalf("width %d workspace footer contains an unclear back cue:\n%s", width, view)
		}
	}

	failure := NewModel(Options{
		Now:        now,
		Width:      120,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveManager: keepAliveManagerInState(keepalive.SessionState{
			SessionID: "workspace-id",
			State:     keepalive.StateErrorSubprocess,
			ScopeUsed: 1,
			MaxSends:  5,
		}),
	})
	view := stripANSI(failure.View())
	for _, want := range []string{"s send now", "x cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("failure workspace footer missing %q:\n%s", want, view)
		}
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
		{name: "countdown", state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateCountdown, InstanceToken: 1, MaxSends: 1}, focus: "keepalive_send_now", sendOK: true, xOK: true},
		{name: "confirming", state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateConfirming, InstanceToken: 3, MaxSends: 1}, focus: "keepalive_stop_waiting", sendOK: false, xOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel(Options{
				Now:              now,
				SelectedID:       "workspace-id",
				Sessions:         []session.Session{workspaceSession(now)},
				KeepAliveManager: keepAliveManagerInState(tc.state),
			})
			if model.FocusedAction() != tc.focus {
				t.Fatalf("initial active focus = %q, want %q", model.FocusedAction(), tc.focus)
			}
			updated, cmd := model.Update(keyRunes("s"))
			if tc.sendOK && cmd == nil {
				t.Fatalf("s in %s returned nil command, want send", tc.name)
			}
			if !tc.sendOK && cmd != nil {
				t.Fatalf("s in %s returned command, want inert", tc.name)
			}
			updated, _ = model.Update(keyRunes("x"))
			afterX := updated.(Model)
			if tc.xOK && afterX.KeepAliveState("workspace-id").State != keepalive.StateMonitoringIdle {
				t.Fatalf("x in %s did not stop the active send", tc.name)
			}
		})
	}

	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
	})
	updated, cmd := model.Update(keyRunes("s"))
	if cmd != nil {
		t.Fatal("s produced command while no KeepAlive send action was available")
	}
	updated, cmd = model.Update(keyRunes("x"))
	if cmd != nil || updated.(Model).KeepAliveEnabled("workspace-id") {
		t.Fatal("x changed state while no KeepAlive instance was available")
	}
}

func TestInitialRoutes(t *testing.T) {
	if route := NewModel(Options{}).route; route != RouteList {
		t.Fatalf("default route = %q, want list", route)
	}
	if route := NewModel(Options{SelectedID: "11111111"}).route; route != RouteWorkspace {
		t.Fatalf("selected id route = %q, want workspace", route)
	}
	if route := NewModel(Options{AmbiguousID: "111"}).route; route != RouteAmbiguous {
		t.Fatalf("ambiguous id route = %q, want ambiguous", route)
	}
	if route := NewModel(Options{StartMode: StartConfig}).route; route != RouteConfig {
		t.Fatalf("config route = %q, want config", route)
	}
}

type errForTest string

func (e errForTest) Error() string {
	return string(e)
}

func keepAliveManagerInState(desired keepalive.SessionState) *keepalive.Manager {
	cfg := config.Default().KeepAlive
	if desired.MaxSends > 0 {
		cfg.Scope.MaxSends = desired.MaxSends
	}
	if desired.State == keepalive.StateScopeComplete {
		cfg.Scope.MaxSends = 1
	}
	manager := keepalive.NewManager(cfg)
	if desired.State == "" || desired.State == keepalive.StateOff {
		return manager
	}

	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	remaining := 5 * time.Minute
	if desired.State == keepalive.StateMonitoringIdle && desired.ScopeUsed == 0 {
		remaining = 10 * time.Minute
	}
	if desired.State == keepalive.StatePaused {
		remaining = 20 * time.Second
	}
	s := activeSessionForTUITest(desired.SessionID, now, time.Hour, remaining)
	actions := manager.Enable(s, now)
	if desired.State == keepalive.StatePaused || desired.State == keepalive.StateMonitoringIdle && desired.ScopeUsed == 0 {
		return manager
	}
	if len(actions) == 0 {
		return manager
	}
	if desired.State == keepalive.StateCountdown {
		return manager
	}
	send := manager.SendNow(desired.SessionID, actions[0].InstanceToken)
	if len(send) == 0 {
		return manager
	}
	if desired.State == keepalive.StateSending {
		return manager
	}
	manager.MarkSendStarted(desired.SessionID, send[0].InstanceToken)
	switch desired.State {
	case keepalive.StateConfirming:
	case keepalive.StateErrorNoClaude:
		manager.MarkNoClaude(desired.SessionID, send[0].InstanceToken, desired.LastFailure)
	case keepalive.StateErrorSubprocess:
		manager.MarkSubprocessFailure(desired.SessionID, send[0].InstanceToken, desired.LastFailure, desired.RateLimited)
	case keepalive.StateErrorTimeout:
		manager.MarkConfirmationTimeout(desired.SessionID, send[0].InstanceToken)
	case keepalive.StateScopeComplete, keepalive.StateMonitoringIdle:
		manager.MarkSuccess(desired.SessionID, send[0].InstanceToken)
	}
	return manager
}

func activeSessionForTUITest(id string, now time.Time, ttl, remaining time.Duration) session.Session {
	last := now.Add(-(ttl - remaining))
	return session.Session{
		SessionID:     id,
		CacheAnchorAt: &last,
		CacheWindow: session.CacheWindow{
			TTLSeconds: int(ttl.Seconds()),
			Known:      true,
		},
	}
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
