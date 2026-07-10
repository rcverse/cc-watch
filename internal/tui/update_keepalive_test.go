package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/session"
)

func TestDisplayTickEvaluatesKeepAliveMonitoringSessions(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	last := now.Add(-56 * time.Minute)
	cfg := config.Default().KeepAlive
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
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateMonitoringIdle, TriggerArmed: true, MaxSends: 1},
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
	if got := model.countdowns["workspace-id"]; got != 30 {
		t.Fatalf("countdown = %d, want 30", got)
	}
}

func TestExpiredSessionDoesNotEnableKeepAlive(t *testing.T) {
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

	updated, cmd := model.Update(keyRunes("k"))
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("expired k returned command, want nil")
	}
	if model.KeepAliveEnabled(expired.SessionID) {
		t.Fatalf("expired session enabled KeepAlive")
	}
	if !strings.Contains(model.View(), "N/A after expiry") {
		t.Fatalf("expired workspace missing KeepAlive disabled reason:\n%s", model.View())
	}
}

func TestExpiredSessionDisablesExistingKeepAliveAndCannotSend(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	expiredLast := now.Add(-2 * time.Hour)
	expired := workspaceSession(now)
	expired.LastMessageAt = &expiredLast
	expired.CacheWindow = session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true}
	model := NewModel(Options{
		Now:        now,
		SelectedID: expired.SessionID,
		Sessions:   []session.Session{expired},
		KeepAliveStates: map[string]keepalive.SessionState{
			expired.SessionID: {SessionID: expired.SessionID, State: keepalive.StateMonitoringIdle, MaxSends: 1},
		},
	})

	updated, cmd := model.Update(DisplayTickMsg{Now: now.Add(time.Second)})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("expired display tick returned command, want nil")
	}
	if state := model.KeepAliveState(expired.SessionID); state.State != keepalive.StateOff {
		t.Fatalf("expired monitoring state = %#v, want off", state)
	}
	if strings.Contains(model.View(), "Send now") {
		t.Fatalf("expired session still exposes KeepAlive send UI:\n%s", model.View())
	}

	model = NewModel(Options{
		Now:        now,
		SelectedID: expired.SessionID,
		Sessions:   []session.Session{expired},
		KeepAliveStates: map[string]keepalive.SessionState{
			expired.SessionID: {SessionID: expired.SessionID, State: keepalive.StatePaused, InstanceToken: 7, MaxSends: 1},
		},
	})
	updated, cmd = model.Update(DisplayTickMsg{Now: now.Add(time.Second)})
	model = updated.(Model)
	if cmd != nil {
		t.Fatalf("expired paused display tick produced cmd=%v, want nil", cmd)
	}
	if state := model.KeepAliveState(expired.SessionID); state.State != keepalive.StateOff {
		t.Fatalf("expired paused state = %#v, want off", state)
	}
}

func TestWorkspaceIgnoresStaleKeepAliveAsyncMessages(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	state := keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateCountdown, InstanceToken: 41, MaxSends: 1}
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
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateMonitoringIdle {
		t.Fatalf("x state = %q, want monitoring idle", got)
	}

	for _, msg := range []tea.Msg{
		KeepAliveCountdownElapsedMsg{SessionID: "workspace-id", InstanceToken: 41, Now: now.Add(30 * time.Second)},
		KeepAliveRunnerResultMsg{SessionID: "workspace-id", InstanceToken: 41, StartedAt: now.Add(time.Second)},
		KeepAliveConfirmationResultMsg{SessionID: "workspace-id", InstanceToken: 41, ConfirmedAt: now.Add(time.Minute)},
	} {
		updated, _ = model.Update(msg)
		model = updated.(Model)
		if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateMonitoringIdle {
			t.Fatalf("stale msg %#v changed state to %q, want monitoring idle", msg, got)
		}
	}

	switched, _ := model.Update(RefreshResultMsg{
		Generation: 1,
		Sessions:   []session.Session{listViewSession("other-id", "other", now, now, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "")},
	})
	model = switched.(Model)
	updated, _ = model.Update(KeepAliveConfirmationResultMsg{SessionID: "workspace-id", InstanceToken: 41, ConfirmedAt: now.Add(time.Minute)})
	model = updated.(Model)
	if got := model.KeepAliveState("workspace-id").State; got == keepalive.StateScopeComplete {
		t.Fatalf("stale confirmation after session switch changed state to %q", got)
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
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateCountdown, InstanceToken: 21, MaxSends: 1},
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
	if got := model.KeepAliveState("workspace-id").State; got != keepalive.StateScopeComplete {
		t.Fatalf("state = %q, want limit reached", got)
	}
	if !strings.Contains(model.View(), "✓ KeepAlive sent and confirmed") {
		t.Fatalf("view missing success notice:\n%s", model.View())
	}
}

func TestWorkspaceKeepAliveRunnerUsesRuntimePolicy(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	manager := keepalive.NewManager(cfg)
	s := workspaceSession(now)
	manager.SetState(keepalive.SessionState{
		SessionID:     s.SessionID,
		State:         keepalive.StateCountdown,
		MaxSends:      1,
		InstanceToken: 7,
	})
	model := NewModel(Options{
		Now:              now,
		Sessions:         []session.Session{s},
		SelectedID:       s.SessionID,
		KeepAliveConfig:  cfg,
		KeepAliveManager: manager,
		Dependencies: Dependencies{
			KeepAliveRunner: fakeKeepAliveRunner{startedAt: now.Add(time.Second)},
			ConfirmKeepAlive: func(context.Context, keepalive.ConfirmationTarget) (keepalive.ConfirmationResult, error) {
				return keepalive.ConfirmationResult{ConfirmedAt: now.Add(2 * time.Second)}, nil
			},
		},
	})

	updated, cmd := model.sendKeepAliveNow()
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("sendKeepAliveNow returned nil command")
	}
	runnerMsg := cmd().(KeepAliveRunnerResultMsg)
	updated, cmd = model.Update(runnerMsg)
	model = updated.(Model)
	if got := model.KeepAliveState(s.SessionID).State; got != keepalive.StateConfirming {
		t.Fatalf("state after runner = %q, want confirming", got)
	}
	if cmd == nil {
		t.Fatal("runner result returned nil command, want confirmation command")
	}
}

func TestConfigEditorResetRequiresRepeatConfirmation(t *testing.T) {
	cfg := config.Default()
	cfg.ReminderThresholds = []int{30, 15}
	saves := 0
	var saved config.Config
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    cfg,
		Dependencies: Dependencies{
			SaveConfig: func(next config.Config) error {
				saves++
				saved = next
				return nil
			},
		},
	})

	updated, _ := model.Update(keyRunes("d"))
	model = updated.(Model)
	if !strings.Contains(model.View(), "Reset defaults?") {
		t.Fatalf("first d did not show reset confirmation:\n%s", model.View())
	}
	if !strings.Contains(model.View(), "30, 15%") {
		t.Fatalf("first d reset before confirmation:\n%s", model.View())
	}

	updated, _ = model.Update(keyRunes("d"))
	model = updated.(Model)
	if strings.Contains(model.View(), "Reset defaults?") {
		t.Fatalf("second d did not clear reset confirmation:\n%s", model.View())
	}
	if !strings.Contains(model.View(), "20, 10%") {
		t.Fatalf("second d did not reset draft:\n%s", model.View())
	}
	if saves != 1 || saved.ReminderThresholds[0] != 20 || saved.ReminderThresholds[1] != 10 {
		t.Fatalf("reset save = calls %d config %#v", saves, saved)
	}
}

func TestConfigEditorSaveDoesNotMutateActiveKeepAliveState(t *testing.T) {
	cfg := config.Default()
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    cfg,
		Sessions:  []session.Session{workspaceSession(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateMonitoringIdle, MaxSends: 1},
		},
		Dependencies: Dependencies{
			SaveConfig: func(config.Config) error { return nil },
		},
	})

	model = moveConfigFocusTo(t, model, "config_max_sends")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	updated, _ = model.Update(keyRunes("7"))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	updated, _ = model.Update(keyRunes("s"))
	model = updated.(Model)

	if state := model.KeepAliveState("workspace-id"); state.MaxSends != 1 {
		t.Fatalf("config save mutated active KeepAlive state: %#v", state)
	}
}
