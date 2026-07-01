package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
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

func TestListConfigShortcutOpensConfigEditor(t *testing.T) {
	model := NewModel(Options{Sessions: listViewSessions(time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC))})

	updated, _ := model.Update(keyRunes("c"))
	model = updated.(Model)

	if model.Route() != RouteConfig {
		t.Fatalf("c from list route = %q, want config", model.Route())
	}
	if !strings.Contains(model.View(), "Claude Code Watch / config") {
		t.Fatalf("config shortcut did not render config editor:\n%s", model.View())
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("esc from list-opened config returned command, want nil")
	}
	if model.Route() != RouteList {
		t.Fatalf("route = %q, want list", model.Route())
	}
	if model.LastAction() != "back_to_list" {
		t.Fatalf("last action = %q, want back_to_list", model.LastAction())
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

	model = moveConfigFocusTo(t, model, "config_autosend")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)
	if strings.Contains(model.View(), "Auto-send:             [x] enabled") {
		t.Fatalf("space did not toggle auto-send off:\n%s", model.View())
	}

	updated, _ = model.Update(keyRunes("s"))
	model = updated.(Model)
	if saves != 1 {
		t.Fatalf("saves = %d, want 1", saves)
	}
	if saved.ReminderThresholds[0] != 30 || saved.ReminderThresholds[1] != 15 || saved.KeepAlive.AutoSend {
		t.Fatalf("saved config = %#v", saved)
	}
	if model.LastAction() != "save_config" {
		t.Fatalf("last action = %q, want save_config", model.LastAction())
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("esc in config editor returned nil command, want quit")
	}
	if saves != 1 {
		t.Fatalf("esc wrote config; saves = %d", saves)
	}
	if model.LastAction() != "cancel_config" {
		t.Fatalf("last action = %q, want cancel_config", model.LastAction())
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
	if model.LastAction() != "save_config_invalid" {
		t.Fatalf("last action = %q, want save_config_invalid", model.LastAction())
	}
	if !strings.Contains(model.View(), "Cannot save.") {
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
