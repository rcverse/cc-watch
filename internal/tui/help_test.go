package tui

import (
	"strings"
	"testing"
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
		status KeepAliveStatus
		want   []string
	}{
		{
			status: KeepAliveCountdown,
			want:   []string{"KeepAlive countdown remains visible", "s send KeepAlive now", "x cancel/dismiss"},
		},
		{
			status: KeepAliveConfirming,
			want:   []string{"KeepAlive confirming remains visible", "x cancel/dismiss"},
		},
		{
			status: KeepAliveFailure,
			want:   []string{"KeepAlive failure remains visible", "x cancel/dismiss"},
		},
	} {
		model := NewModel(Options{
			SelectedID:      "11111111",
			KeepAliveStatus: tc.status,
		})
		view := openHelp(model).View()
		for _, want := range tc.want {
			if !strings.Contains(view, want) {
				t.Fatalf("%s help missing %q:\n%s", tc.status, want, view)
			}
		}
	}
}

func openHelp(model Model) Model {
	updated, _ := model.Update(keyRunes("?"))
	return updated.(Model)
}
