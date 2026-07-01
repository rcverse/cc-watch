package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/refresh"
)

type Watcher interface {
	Next() refresh.WatcherResult
}

func LiveRefreshCommand(watcher Watcher) tea.Cmd {
	if watcher == nil {
		return nil
	}
	return func() tea.Msg {
		result := watcher.Next()
		if result.Closed {
			return RefreshWatcherClosedMsg{State: result.State}
		}
		if result.Err != nil {
			return RefreshWatcherDegradedMsg{State: result.State}
		}
		return RefreshWatcherEventsMsg{Events: result.Events, State: result.State}
	}
}
