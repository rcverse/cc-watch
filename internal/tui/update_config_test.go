package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/statusline"
)

func TestConfigFocusCycleOnlyVisitsVisibleRows(t *testing.T) {
	model := NewModel(Options{StartMode: StartConfig, Config: config.Default()})
	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		action := model.FocusedAction()
		seen[action] = true
		if !viewHasVisibleFocusMarker(model.View()) {
			t.Fatalf("config focus %q has no visible marker:\n%s", action, model.View())
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	for _, hidden := range []string{"help", "quit"} {
		if seen[hidden] {
			t.Fatalf("config focus reached hidden action %q; saw %#v", hidden, seen)
		}
	}
	for _, want := range []string{"config_save", "config_reset", "config_cancel"} {
		if !seen[want] {
			t.Fatalf("config focus did not reach visible action %q; saw %#v", want, seen)
		}
	}
}

func TestConfigEditorShowsStatuslineInstallState(t *testing.T) {
	model := NewModel(Options{StartMode: StartConfig, Config: config.Default()})
	view := model.View()

	for _, want := range []string{"Statusline", "State", "Not installed", "Install in Claude Code"} {
		if !strings.Contains(view, want) {
			t.Fatalf("config view missing %q:\n%s", want, view)
		}
	}
}

func TestConfigEditorCanInstallStatusline(t *testing.T) {
	status := statusline.Status{State: statusline.StateNotInstalled}
	installs := 0
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    config.Default(),
		Dependencies: Dependencies{
			InspectStatusline: func() (statusline.Status, error) {
				return status, nil
			},
			InstallStatusline: func() error {
				installs++
				status = statusline.Status{State: statusline.StateInstalled, Command: "cc-watch statusline"}
				return nil
			},
		},
	})

	model = moveConfigFocusTo(t, model, "config_statusline_action")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if installs != 1 {
		t.Fatalf("installs = %d, want 1", installs)
	}
	if !strings.Contains(model.View(), "Installed in Claude Code") || !strings.Contains(model.View(), "Uninstall from Claude Code") {
		t.Fatalf("installed statusline state missing:\n%s", model.View())
	}
}

func TestConfigEditorCanUninstallStatusline(t *testing.T) {
	status := statusline.Status{State: statusline.StateInstalled, Command: "cc-watch statusline"}
	uninstalls := 0
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    config.Default(),
		Dependencies: Dependencies{
			InspectStatusline: func() (statusline.Status, error) {
				return status, nil
			},
			UninstallStatusline: func() error {
				uninstalls++
				status = statusline.Status{State: statusline.StateNotInstalled}
				return nil
			},
		},
	})

	model = moveConfigFocusTo(t, model, "config_statusline_action")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if uninstalls != 1 {
		t.Fatalf("uninstalls = %d, want 1", uninstalls)
	}
	if !strings.Contains(model.View(), "Not installed") || !strings.Contains(model.View(), "Install in Claude Code") {
		t.Fatalf("uninstalled statusline state missing:\n%s", model.View())
	}
}

func TestConfigEditorStatuslineManualReviewDoesNotWrite(t *testing.T) {
	writes := 0
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    config.Default(),
		Dependencies: Dependencies{
			InspectStatusline: func() (statusline.Status, error) {
				return statusline.Status{State: statusline.StateManualReview}, nil
			},
			InstallStatusline: func() error {
				writes++
				return errors.New("must not write")
			},
			UninstallStatusline: func() error {
				writes++
				return errors.New("must not write")
			},
		},
	})

	model = moveConfigFocusTo(t, model, "config_statusline_action")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if writes != 0 {
		t.Fatalf("writes = %d, want 0 for manual review", writes)
	}
	if !strings.Contains(model.View(), "Needs manual review") || !strings.Contains(model.View(), "Run cc-watch statusline --check") {
		t.Fatalf("manual review copy missing:\n%s", model.View())
	}
}

func TestListConfigShortcutOpensConfigEditor(t *testing.T) {
	model := NewModel(Options{Sessions: listViewSessions(time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC))})

	updated, _ := model.Update(keyRunes("c"))
	model = updated.(Model)

	if model.route != RouteConfig {
		t.Fatalf("c from list route = %q, want config", model.route)
	}
	if !strings.Contains(model.View(), "Claude Code Watch / config") {
		t.Fatalf("config shortcut did not render config editor:\n%s", model.View())
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("esc from list-opened config returned command, want nil")
	}
	if model.route != RouteList {
		t.Fatalf("route = %q, want list", model.route)
	}
}

func TestConfigEditorPrefillsCurrentValueAndPreservesMessageOnEmptyEdit(t *testing.T) {
	cfg := config.Default()
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

	model = moveConfigFocusTo(t, model, "config_message")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !strings.Contains(model.View(), cfg.KeepAlive.Message) || !strings.Contains(model.View(), "↵ save field") {
		t.Fatalf("message edit did not prefill current value with guidance:\n%s", model.View())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	updated, _ = model.Update(keyRunes("s"))
	model = updated.(Model)

	if saves != 1 {
		t.Fatalf("saves = %d, want 1", saves)
	}
	if saved.KeepAlive.Message != cfg.KeepAlive.Message {
		t.Fatalf("message = %q, want preserved %q", saved.KeepAlive.Message, cfg.KeepAlive.Message)
	}
}

func TestConfigEditorFocusEditToggleSaveAndCancel(t *testing.T) {
	cfg := config.Default()
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

	model = moveConfigFocusTo(t, model, "config_reminder_thresholds")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	for _, key := range []tea.KeyMsg{
		keyRunes("3"), keyRunes("0"), keyRunes(","), keyRunes("1"), keyRunes("5"),
		{Type: tea.KeyEnter},
	} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	if !strings.Contains(model.View(), "30, 15%") {
		t.Fatalf("threshold edit not reflected:\n%s", model.View())
	}

	updated, _ = model.Update(keyRunes("s"))
	model = updated.(Model)
	if saves != 1 {
		t.Fatalf("saves = %d, want 1", saves)
	}
	if saved.ReminderThresholds[0] != 30 || saved.ReminderThresholds[1] != 15 {
		t.Fatalf("saved config = %#v", saved)
	}
	if !strings.Contains(model.View(), "✓ Saved") {
		t.Fatalf("save success missing notice:\n%s", model.View())
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("esc in config editor returned nil command, want quit")
	}
	if saves != 1 {
		t.Fatalf("esc wrote config; saves = %d", saves)
	}
}

func TestConfigEditorInvalidConfigCannotSave(t *testing.T) {
	cfg := config.Default()
	saves := 0
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    cfg,
		Dependencies: Dependencies{
			SaveConfig: func(config.Config) error {
				saves++
				return nil
			},
		},
	})

	model = moveConfigFocusTo(t, model, "config_countdown")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	for _, key := range []tea.KeyMsg{keyRunes("1"), keyRunes("2"), keyRunes("0"), {Type: tea.KeyEnter}} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	updated, _ = model.Update(keyRunes("s"))
	model = updated.(Model)

	if saves != 0 {
		t.Fatalf("invalid config saved %d time(s)", saves)
	}
	if !strings.Contains(model.View(), "✕ Cannot save") || !strings.Contains(model.View(), "✕ Validation failed:") {
		t.Fatalf("invalid config view missing summary:\n%s", model.View())
	}
}

func TestConfigEditorMalformedFieldInputBlocksSave(t *testing.T) {
	cfg := config.Default()
	saves := 0
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    cfg,
		Dependencies: Dependencies{
			SaveConfig: func(config.Config) error {
				saves++
				return nil
			},
		},
	})

	model = moveConfigFocusTo(t, model, "config_reminder_thresholds")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	for _, key := range []tea.KeyMsg{keyRunes("1"), keyRunes(","), keyRunes("x"), {Type: tea.KeyEnter}} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	if !strings.Contains(model.View(), "Error: reminder thresholds") {
		t.Fatalf("malformed thresholds missing field error:\n%s", model.View())
	}
	updated, _ = model.Update(keyRunes("s"))
	model = updated.(Model)
	if saves != 0 {
		t.Fatalf("malformed thresholds saved %d time(s)", saves)
	}

	model = moveConfigFocusTo(t, model, "config_trigger")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	for _, key := range []tea.KeyMsg{keyRunes("abc"), {Type: tea.KeyEnter}} {
		updated, _ = model.Update(key)
		model = updated.(Model)
	}
	if !strings.Contains(model.View(), "Error: trigger must be positive.") {
		t.Fatalf("malformed trigger missing field error:\n%s", model.View())
	}
}

func TestConfigEditorEditsTriggerMessageAndMaxSends(t *testing.T) {
	cfg := config.Default()
	var saved config.Config
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    cfg,
		Dependencies: Dependencies{
			SaveConfig: func(next config.Config) error {
				saved = next
				return nil
			},
		},
	})

	for _, edit := range []struct {
		action string
		keys   []tea.KeyMsg
	}{
		{action: "config_trigger", keys: []tea.KeyMsg{keyRunes("4"), {Type: tea.KeyEnter}}},
		{action: "config_message", keys: []tea.KeyMsg{keyRunes("still here?"), {Type: tea.KeyEnter}}},
		{action: "config_max_sends", keys: []tea.KeyMsg{keyRunes("2"), {Type: tea.KeyEnter}}},
	} {
		model = moveConfigFocusTo(t, model, edit.action)
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model = updated.(Model)
		for _, key := range edit.keys {
			updated, _ = model.Update(key)
			model = updated.(Model)
		}
	}
	updated, _ := model.Update(keyRunes("s"))
	model = updated.(Model)

	if saved.KeepAlive.TriggerBeforeExpiryMinutes != 4 || saved.KeepAlive.Message != "still here?" || saved.KeepAlive.Scope.MaxSends != 2 {
		t.Fatalf("saved config = %#v", saved)
	}
}
