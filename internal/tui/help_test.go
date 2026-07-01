package tui

import (
	"strings"
	"testing"

	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/session"
)

func TestQuestionMarkDoesNotOpenInAppHelpOverlay(t *testing.T) {
	for _, model := range []Model{
		NewModel(Options{}),
		NewModel(Options{StartMode: StartConfig}),
		NewModel(Options{
			SelectedID: "11111111",
			Sessions:   []session.Session{{SessionID: "11111111", ShortID: "11111111"}},
			KeepAliveStates: map[string]keepalive.SessionState{
				"11111111": {SessionID: "11111111", State: keepalive.StateCountdown, InstanceToken: 1, MaxSends: 1},
			},
		}),
	} {
		before := model.View()
		updated, cmd := model.Update(keyRunes("?"))
		model = updated.(Model)
		if cmd != nil {
			t.Fatalf("? returned command, want nil")
		}
		after := model.View()
		if strings.Contains(after, "\nHelp\n") || strings.Contains(after, "toggle help") {
			t.Fatalf("removed help overlay rendered:\n%s", after)
		}
		if before != after {
			t.Fatalf("? changed visible view; before:\n%s\nafter:\n%s", before, after)
		}
	}
}
