package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case DisplayTickMsg:
		m.now = typed.Now
		for sessionID, seconds := range m.countdowns {
			if seconds > 0 {
				m.countdowns[sessionID] = seconds - 1
			}
		}
		return m, tea.Batch(m.reminderNotificationCommands()...)
	case WatcherEventMsg:
		m.watcherEvents = append(m.watcherEvents, typed)
		return m.scheduleRefresh(refresh.SourceFsnotify, false)
	case RefreshResultMsg:
		if typed.Generation < m.refreshGeneration {
			return m, nil
		}
		m.refreshGeneration = typed.Generation
		previousSelectedID := m.SelectedSessionID()
		m.sessions = cloneSessions(typed.Sessions)
		if typed.HasRefresh {
			m.refresh = defaultRefresh(typed.Refresh)
		} else if m.refresh.EmptyState != EmptyNone && len(m.sessions) > 0 {
			m.refresh.EmptyState = EmptyNone
		}
		switch {
		case len(m.sessions) == 0:
			m.selectedIndex = 0
			m.selectedID = ""
		case previousSelectedID != "":
			if index, ok := findSessionIndex(m.sessions, previousSelectedID); ok {
				m.selectedIndex = index
				m.selectedID = m.sessions[index].SessionID
			} else {
				m.clampSelectedIndex()
			}
		default:
			m.clampSelectedIndex()
		}
		return m, nil
	case RefreshDegradedMsg:
		m.refresh.Watcher = typed.State
		return m, nil
	case NotificationResultMsg:
		m.applyNotificationResult(typed)
		return m, nil
	case ManualRefreshMsg:
		m.refresh.NotificationDegraded = ""
		if m.deps.ResetNotificationSuppression != nil {
			m.deps.ResetNotificationSuppression()
		}
		m.lastAction = "manual_refresh"
		return m.scheduleRefresh(refresh.SourceManual, true)
	case SafetyRefreshMsg:
		m.lastAction = "safety_refresh"
		return m.scheduleRefresh(refresh.SourceSafety, false)
	case tea.KeyMsg:
		return m.updateKey(typed)
	case tea.WindowSizeMsg:
		m.width = typed.Width
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) reminderNotificationCommands() []tea.Cmd {
	if m.deps.NotifyEvent == nil {
		return nil
	}
	var commands []tea.Cmd
	for _, s := range m.sessions {
		if !m.reminderEnabled[s.SessionID] {
			continue
		}
		status := s.StatusAt(m.now)
		if status.State != session.StatusActive || status.RemainingSeconds == nil || s.CacheWindow.TTLSeconds <= 0 {
			continue
		}
		remainingPercent := float64(*status.RemainingSeconds) / float64(s.CacheWindow.TTLSeconds) * 100
		for _, threshold := range m.reminderThresholds {
			if remainingPercent > float64(threshold) || m.reminderAlreadyFired(s.SessionID, threshold) {
				continue
			}
			m.markReminderFired(s.SessionID, threshold)
			event := notify.Event{
				Kind:             notify.EventReminderThresholdCrossed,
				SessionID:        s.SessionID,
				Project:          s.Project,
				ThresholdPercent: threshold,
			}
			commands = append(commands, m.notificationCommand(event))
		}
	}
	return commands
}

func (m Model) notificationCommand(event notify.Event) tea.Cmd {
	return func() tea.Msg {
		return NotificationResultMsg{
			Event:  event,
			Result: m.deps.NotifyEvent(event),
		}
	}
}

func (m Model) reminderAlreadyFired(sessionID string, threshold int) bool {
	return m.reminderFired[sessionID][threshold]
}

func (m *Model) markReminderFired(sessionID string, threshold int) {
	fired := m.reminderFired[sessionID]
	if fired == nil {
		fired = map[int]bool{}
	}
	fired[threshold] = true
	m.reminderFired[sessionID] = fired
}

func (m *Model) applyNotificationResult(msg NotificationResultMsg) {
	if msg.Result.Suppressed {
		return
	}
	notification := notify.FormatEvent(msg.Event)
	status := NotificationStatus{
		Event:        msg.Event,
		Notification: notification,
		Result:       msg.Result,
	}
	m.notificationStatuses = append([]NotificationStatus{status}, m.notificationStatuses...)
	if len(m.notificationStatuses) > 5 {
		m.notificationStatuses = m.notificationStatuses[:5]
	}
	if msg.Result.Degraded {
		m.refresh.NotificationDegraded = msg.Result.Message
	} else if msg.Result.Delivered {
		m.refresh.NotificationDegraded = ""
	}
}

func (m Model) scheduleRefresh(source refresh.Source, bypassedDebounce bool) (tea.Model, tea.Cmd) {
	m.refreshGeneration++
	m.lastRefreshSource = source
	m.lastBypassedDebounce = bypassedDebounce
	generation := m.refreshGeneration
	refreshSessions := m.deps.RefreshSessions
	refreshSnapshot := m.deps.RefreshSnapshot
	if refreshSessions == nil && refreshSnapshot == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		if refreshSnapshot != nil {
			snapshot := refreshSnapshot(source, generation)
			return RefreshResultMsg{
				Generation: generation,
				Sessions:   snapshot.Sessions,
				Refresh:    snapshot.Refresh,
				HasRefresh: snapshot.HasRefresh,
			}
		}
		return RefreshResultMsg{
			Generation: generation,
			Sessions:   refreshSessions(source, generation),
		}
	}
}

func (m *Model) clampSelectedIndex() {
	if len(m.sessions) == 0 {
		m.selectedIndex = 0
		m.selectedID = ""
		return
	}
	if m.selectedIndex >= len(m.sessions) {
		m.selectedIndex = len(m.sessions) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	m.selectedID = m.sessions[m.selectedIndex].SessionID
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.helpOpen = !m.helpOpen
		m.lastAction = "toggle_help"
		return m, nil
	case "q", "ctrl+c":
		m.lastAction = "quit"
		return m, tea.Quit
	case "down":
		m.moveFocus(1)
		return m, nil
	case "up":
		m.moveFocus(-1)
		return m, nil
	case "enter":
		return m.activateFocused()
	case " ":
		if m.FocusedAction() == "reminder" || m.FocusedAction() == "keepalive" {
			return m.activateFocused()
		}
		return m, nil
	case "r":
		if m.route == RouteList || m.route == RouteWorkspace {
			m.toggleReminderForSelected()
		}
		return m, nil
	case "k":
		if m.route == RouteList || m.route == RouteWorkspace {
			m.toggleKeepAliveForSelected()
		}
		return m, nil
	case "c":
		if m.route == RouteWorkspace {
			m.lastAction = "copy_session_id"
		}
		return m, nil
	case "b", "esc":
		if m.route == RouteWorkspace {
			m.route = RouteList
			m.lastAction = "back_to_list"
		} else if m.route == RouteAmbiguous {
			m.route = RouteList
			m.lastAction = "back_to_list"
		}
		return m, nil
	case "s":
		switch {
		case m.route == RouteConfig:
			m.lastAction = "save_config"
		case m.route == RouteWorkspace && (m.keepAliveStatus == KeepAliveCountdown || m.keepAliveStatus == KeepAliveFailure):
			m.lastAction = "send_keepalive_now"
		}
		return m, nil
	case "x":
		if m.route == RouteWorkspace && m.keepAliveStatus != KeepAliveInactive {
			m.lastAction = "cancel_keepalive"
		}
		return m, nil
	case "d":
		if m.route == RouteConfig {
			m.lastAction = "reset_defaults"
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) activateFocused() (tea.Model, tea.Cmd) {
	action := m.FocusedAction()
	switch action {
	case "session":
		if m.route == RouteList || m.route == RouteAmbiguous {
			if selected := m.selectedSession(); selected != nil {
				m.route = RouteWorkspace
				m.selectedID = selected.SessionID
				m.lastAction = "open_session"
				return m, nil
			}
		}
		m.lastAction = "activate_session"
		return m, nil
	case "reminder":
		m.toggleReminderForSelected()
		return m, nil
	case "keepalive":
		m.toggleKeepAliveForSelected()
		return m, nil
	case "refresh":
		return m.Update(ManualRefreshMsg{})
	case "help":
		m.helpOpen = !m.helpOpen
		m.lastAction = "toggle_help"
		return m, nil
	case "quit":
		m.lastAction = "quit"
		return m, tea.Quit
	default:
		m.lastAction = "activate_" + action
		return m, nil
	}
}
