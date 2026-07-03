package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/keepalive"
)

func (m Model) activateFocusedAction(action string) (tea.Model, tea.Cmd) {
	switch m.route {
	case RouteList, RouteAmbiguous:
		return m.activateListAction(action)
	case RouteWorkspace:
		return m.activateWorkspaceAction(action)
	case RouteConfig:
		return m.activateConfigAction(action)
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateListAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "session":
		if selected := m.selectedSession(); selected != nil {
			m.route = RouteWorkspace
			m.selectedID = selected.SessionID
			m.lastAction = "open_session"
			return m, nil
		}
		m.lastAction = "activate_session"
		return m, nil
	case "reminder":
		m.toggleReminderForSelected()
		return m, nil
	case "keepalive":
		return m, m.toggleKeepAliveForSelected()
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateWorkspaceAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "reminder":
		m.toggleReminderForSelected()
		return m, nil
	case "keepalive":
		return m, m.toggleKeepAliveForSelected()
	case "keepalive_autosend":
		return m, m.toggleKeepAliveAutoSendForSelected()
	case "keepalive_send_now":
		return m.sendKeepAliveNow()
	case "keepalive_cancel", "keepalive_stop_waiting":
		m.cancelKeepAlive()
		return m, nil
	case "keepalive_acknowledge":
		if selected := m.selectedSession(); selected != nil {
			m.keepAliveManager.Acknowledge(selected.SessionID)
			m.lastAction = "acknowledge_keepalive"
			m.focusIndex = m.defaultFocusIndex()
			// Only ActionScopeComplete's state is newly notification-worthy
			// here; a lingering (non-exhausted) error state is unchanged by
			// Acknowledge and would otherwise re-fire its Failure notice.
			if state := m.KeepAliveState(selected.SessionID); state.State == keepalive.StateScopeComplete {
				return m, m.keepAliveLifecycleNotification(selected.SessionID, state)
			}
		}
		return m, nil
	case "back":
		m.route = RouteList
		m.lastAction = "back_to_list"
		return m, nil
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateConfigAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "config_reminder_thresholds", "config_trigger", "config_countdown", "config_message", "config_max_sends":
		m.startConfigEdit(action)
		return m, nil
	case "config_autosend":
		m.toggleConfigAutoSend()
		return m, nil
	case "config_save":
		return m.saveConfig()
	case "config_reset":
		return m.resetConfigDefaults()
	case "config_cancel":
		return m.cancelConfig()
	default:
		return m.activateSharedAction(action)
	}
}

func (m Model) activateSharedAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "refresh":
		return m.Update(ManualRefreshMsg{})
	case "config":
		m.route = RouteConfig
		m.focusIndex = m.defaultFocusIndex()
		return m, nil
	case "quit":
		m.lastAction = "quit"
		return m, tea.Quit
	default:
		m.lastAction = "activate_" + action
		return m, nil
	}
}
