package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
			return m, nil
		}
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
	case "details_scroll":
		return m, nil
	case "reminder":
		m.toggleReminderForSelected()
		return m, nil
	case "keepalive":
		return m, m.toggleKeepAliveForSelected()
	case "keepalive_reset_limit":
		if selected := m.selectedSession(); selected != nil {
			m.keepAliveManager.ResetLimit(selected.SessionID)
			m.setNotice("✓ KeepAlive limit reset", RoleSuccess, 3*time.Second)
			m.focusIndex = m.defaultFocusIndex()
		}
		return m, nil
	case "keepalive_send_now":
		return m.sendKeepAliveNow()
	case "keepalive_cancel", "keepalive_stop_waiting":
		m.cancelKeepAlive()
		return m, nil
	case "back":
		m.route = RouteList
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
	case "config_statusline_action":
		return m.activateStatuslineConfigAction()
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
		return m, tea.Quit
	default:
		return m, nil
	}
}
