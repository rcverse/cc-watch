//go:build demo

package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rcverse/cc-watch/internal/session"
)

func TestDemoSessionsShowDistinctCacheStates(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.Local)
	sessions := demoSessions(now)

	if got := sessions[1].StatusAt(now).State; got != session.StatusActive {
		t.Fatalf("fading demo session state = %q, want active", got)
	}
	if got := sessions[2].StatusAt(now).State; got != session.StatusExpired {
		t.Fatalf("expired demo session state = %q, want expired", got)
	}
	if sessions[3].CacheUnknownReason != session.CacheUnknownAfterModel {
		t.Fatalf("unknown demo reason = %q, want after model", sessions[3].CacheUnknownReason)
	}
}

func TestDemoWorkspaceIncludesCacheHistory(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.Local)
	demo := demoWorkspaceSession(now)

	if got := len(demo.RecentMessages); got != 3 {
		t.Fatalf("demo recent messages = %d, want 3", got)
	}
	if got := len(demo.Gaps); got != 2 {
		t.Fatalf("demo gaps = %d, want 2", got)
	}
	if demo.ResetCount != 1 {
		t.Fatalf("demo reset count = %d, want 1", demo.ResetCount)
	}
	if !demo.Gaps[0].Reset {
		t.Fatal("demo longest gap should demonstrate a cache reset")
	}
	if demo.CurrentModel == "" || demo.CurrentContextTokens == 0 || len(demo.ModelsUsed) != 2 {
		t.Fatalf("demo model/context metadata is incomplete: %#v", demo)
	}
}

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

func TestDemoModelFiveSecondStepMovesKeepAliveCountdown(t *testing.T) {
	model := newDemoModel()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	model = updated.(demoModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = updated.(demoModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(".")})
	model = updated.(demoModel)

	if !strings.Contains(model.View(), "Sending in 25s") {
		t.Fatalf("five-second demo step did not move countdown:\n%s", model.View())
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
