package tui

import (
	"strings"
	"testing"

	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/session"
)

func TestHelpIncludesGlobalShortcuts(t *testing.T) {
	view := openHelp(NewModel(Options{})).View()

	for _, want := range []string{"arrows", "enter", "space", "?", "q"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help missing %q:\n%s", want, view)
		}
	}
}

func TestHelpShowsOnlyCurrentlyValidRouteShortcuts(t *testing.T) {
	listHelp := openHelp(NewModel(Options{})).View()
	for _, want := range []string{"r toggle Reminder", "k toggle KeepAlive"} {
		if !strings.Contains(listHelp, want) {
			t.Fatalf("list help missing %q:\n%s", want, listHelp)
		}
	}
	for _, notWant := range []string{"send KeepAlive now", "cancel/dismiss", "reset defaults"} {
		if strings.Contains(listHelp, notWant) {
			t.Fatalf("list help contains invalid shortcut %q:\n%s", notWant, listHelp)
		}
	}

	workspaceHelp := openHelp(NewModel(Options{SelectedID: "11111111"})).View()
	for _, notWant := range []string{"send KeepAlive now", "cancel/dismiss"} {
		if strings.Contains(workspaceHelp, notWant) {
			t.Fatalf("normal workspace help contains inactive action %q:\n%s", notWant, workspaceHelp)
		}
	}

	configHelp := openHelp(NewModel(Options{StartMode: StartConfig})).View()
	for _, want := range []string{"s save valid config", "d reset defaults"} {
		if !strings.Contains(configHelp, want) {
			t.Fatalf("config help missing %q:\n%s", want, configHelp)
		}
	}
}

func TestHelpSummarizesDangerousKeepAliveState(t *testing.T) {
	for _, tc := range []struct {
		state keepalive.State
		want  []string
	}{
		{
			state: keepalive.StateCountdown,
			want:  []string{"KeepAlive countdown remains visible", "s send KeepAlive now", "x cancel/dismiss"},
		},
		{
			state: keepalive.StateConfirming,
			want:  []string{"KeepAlive confirming remains visible", "x cancel/dismiss"},
		},
		{
			state: keepalive.StateErrorNoClaude,
			want:  []string{"KeepAlive failure remains visible", "x cancel/dismiss"},
		},
	} {
		model := NewModel(Options{
			SelectedID: "11111111",
			Sessions:   []session.Session{{SessionID: "11111111", ShortID: "11111111"}},
			KeepAliveStates: map[string]keepalive.SessionState{
				"11111111": {SessionID: "11111111", State: tc.state, InstanceToken: 1, MaxSends: 1},
			},
		})
		view := openHelp(model).View()
		for _, want := range tc.want {
			if !strings.Contains(view, want) {
				t.Fatalf("%s help missing %q:\n%s", tc.state, want, view)
			}
		}
	}
}

func openHelp(model Model) Model {
	updated, _ := model.Update(keyRunes("?"))
	return updated.(Model)
}
