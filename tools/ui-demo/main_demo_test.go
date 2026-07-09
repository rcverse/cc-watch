//go:build demo

package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDemoModelRendersRealRoutes(t *testing.T) {
	model := newDemoModel()

	for _, tc := range []struct {
		key  string
		want string
	}{
		{key: "1", want: "Claude Code Watch"},
		{key: "2", want: "KeepAlive"},
		{key: "3", want: "choose session"},
		{key: "4", want: "Settings"},
	} {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		model = updated.(demoModel)
		if view := model.View(); !strings.Contains(view, tc.want) {
			t.Fatalf("route %q missing %q:\n%s", tc.key, tc.want, view)
		}
	}
}

func TestDemoModelTimeTravelCanReachKeepAliveCountdown(t *testing.T) {
	model := newDemoModel()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	model = updated.(demoModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = updated.(demoModel)

	if view := model.View(); !strings.Contains(view, "KeepAlive · ✓ Armed") || !strings.Contains(view, "Sending in") {
		t.Fatalf("jump to KA trigger did not render countdown:\n%s", view)
	}
}

func TestDemoConfigSavePersistsAcrossRouteRebuild(t *testing.T) {
	model := newDemoModel()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	model = updated.(demoModel)

	for i := 0; i < 4; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(demoModel)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(demoModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	model = updated.(demoModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(demoModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = updated.(demoModel)
	if !strings.Contains(model.View(), "✓ Saved") {
		t.Fatalf("save did not show notice:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	model = updated.(demoModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	model = updated.(demoModel)

	if !strings.Contains(model.View(), "7") {
		t.Fatalf("saved config did not survive demo route rebuild:\n%s", model.View())
	}
}
