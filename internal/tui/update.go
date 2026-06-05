package tui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case DisplayTickMsg:
		m.now = typed.Now
		var keepAliveCommands []tea.Cmd
		for sessionID, seconds := range m.countdowns {
			if seconds > 0 {
				m.countdowns[sessionID] = seconds - 1
			}
			if m.countdowns[sessionID] <= 0 {
				state := m.KeepAliveState(sessionID)
				if state.State == keepalive.StateCountdown {
					delete(m.countdowns, sessionID)
					m.applyKeepAliveActions(m.keepAliveManager.CountdownElapsed(sessionID, state.InstanceToken, typed.Now), &keepAliveCommands)
				}
			}
		}
		commands := m.reminderNotificationCommands()
		commands = append(commands, keepAliveCommands...)
		commands = append(commands, m.applyKeepAliveActions(m.keepAliveManager.Check(typed.Now, m.sessions), nil)...)
		if m.startDisplayTicker {
			commands = append(commands, displayTickCommand())
		}
		return m, tea.Batch(commands...)
	case WatcherEventMsg:
		m.watcherEvents = append(m.watcherEvents, typed)
		return m.scheduleRefresh(refresh.SourceFsnotify, false)
	case RefreshResultMsg:
		if typed.Generation < m.refreshGeneration {
			return m, nil
		}
		m.refreshGeneration = typed.Generation
		previousSelectedID := m.SelectedSessionID()
		if typed.SelectedOnly {
			m.sessions = mergeSelectedRefresh(m.sessions, typed.Sessions, typed.SelectedID)
		} else {
			m.sessions = cloneSessions(typed.Sessions)
		}
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
	case KeepAliveCountdownElapsedMsg:
		if !m.keepAliveAsyncCurrent(typed.SessionID, typed.InstanceToken, typed.Generation, typed.SelectedID) {
			m.lastAction = ErrKeepAliveStaleMessage.Error()
			return m, nil
		}
		return m.withKeepAliveActions(m.keepAliveManager.CountdownElapsed(typed.SessionID, typed.InstanceToken, typed.Now))
	case KeepAliveRunnerResultMsg:
		if !m.keepAliveAsyncCurrent(typed.SessionID, typed.InstanceToken, typed.Generation, typed.SelectedID) {
			m.lastAction = ErrKeepAliveStaleMessage.Error()
			return m, nil
		}
		if typed.Err != nil {
			if errors.Is(typed.Err, keepalive.ErrClaudeUnavailable) {
				m.keepAliveManager.MarkNoClaude(typed.SessionID, typed.InstanceToken, typed.Reason)
			} else {
				m.keepAliveManager.MarkSubprocessFailure(typed.SessionID, typed.InstanceToken, typed.Reason)
			}
			return m, nil
		} else {
			m.keepAliveManager.MarkSendStarted(typed.SessionID, typed.InstanceToken, typed.StartedAt)
		}
		if m.KeepAliveState(typed.SessionID).State == keepalive.StateConfirming {
			return m, m.keepAliveConfirmationCommand(typed)
		}
		return m, nil
	case KeepAliveConfirmationResultMsg:
		if !m.keepAliveAsyncCurrent(typed.SessionID, typed.InstanceToken, typed.Generation, typed.SelectedID) {
			m.lastAction = ErrKeepAliveStaleMessage.Error()
			return m, nil
		}
		if typed.Err != nil {
			if errors.Is(typed.Err, keepalive.ErrConfirmationTimeout) {
				m.keepAliveManager.MarkConfirmationTimeout(typed.SessionID, typed.InstanceToken)
			} else {
				m.keepAliveManager.MarkSubprocessFailure(typed.SessionID, typed.InstanceToken, typed.Err.Error())
			}
			return m, nil
		}
		m.keepAliveManager.MarkSuccess(typed.SessionID, typed.InstanceToken, typed.ConfirmedAt)
		if m.KeepAliveState(typed.SessionID).State == keepalive.StateSuccess {
			m.lastAction = "keepalive_confirmed"
		}
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
	refreshSelectedSnapshot := m.deps.RefreshSelectedSnapshot
	selected := m.selectedSession()
	if refreshSessions == nil && refreshSnapshot == nil && refreshSelectedSnapshot == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		if m.route == RouteWorkspace && selected != nil && refreshSelectedSnapshot != nil {
			snapshot := refreshSelectedSnapshot(source, generation, *selected)
			return RefreshResultMsg{
				Generation:   generation,
				Sessions:     snapshot.Sessions,
				Refresh:      snapshot.Refresh,
				HasRefresh:   snapshot.HasRefresh,
				SelectedOnly: true,
				SelectedID:   selected.SessionID,
			}
		}
		if refreshSnapshot != nil {
			snapshot := refreshSnapshot(source, generation)
			return RefreshResultMsg{
				Generation:   generation,
				Sessions:     snapshot.Sessions,
				Refresh:      snapshot.Refresh,
				HasRefresh:   snapshot.HasRefresh,
				SelectedOnly: m.route == RouteWorkspace && m.SelectedSessionID() != "",
				SelectedID:   m.SelectedSessionID(),
			}
		}
		return RefreshResultMsg{
			Generation:   generation,
			Sessions:     refreshSessions(source, generation),
			SelectedOnly: m.route == RouteWorkspace && m.SelectedSessionID() != "",
			SelectedID:   m.SelectedSessionID(),
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

func mergeSelectedRefresh(existing []session.Session, refreshed []session.Session, selectedID string) []session.Session {
	if selectedID == "" {
		return cloneSessions(refreshed)
	}
	var replacement *session.Session
	for i := range refreshed {
		if refreshed[i].SessionID == selectedID || refreshed[i].ShortID == selectedID {
			copied := refreshed[i]
			replacement = &copied
			break
		}
	}
	if replacement == nil {
		return cloneSessions(existing)
	}
	merged := cloneSessions(existing)
	for i := range merged {
		if merged[i].SessionID == selectedID || merged[i].ShortID == selectedID {
			merged[i] = *replacement
			return cloneSessions(merged)
		}
	}
	merged = append(merged, *replacement)
	return cloneSessions(merged)
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
		if m.route == RouteWorkspace && m.FocusedAction() == "evidence" && m.evidenceCanScroll(1) {
			m.evidenceOffset++
			return m, nil
		}
		m.moveFocus(1)
		return m, nil
	case "up":
		if m.route == RouteWorkspace && m.FocusedAction() == "evidence" && m.evidenceCanScroll(-1) {
			m.evidenceOffset--
			return m, nil
		}
		m.moveFocus(-1)
		return m, nil
	case "enter":
		return m.activateFocused()
	case " ":
		if m.FocusedAction() == "reminder" || m.FocusedAction() == "keepalive" || m.FocusedAction() == "keepalive_autosend" {
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
			if msg.String() == "esc" && m.directWorkspace {
				m.lastAction = "quit"
				return m, tea.Quit
			}
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
		case m.route == RouteWorkspace && m.workspaceCanSendKeepAlive():
			return m.sendKeepAliveNow()
		}
		return m, nil
	case "x":
		if m.route == RouteWorkspace && m.workspaceCanCancelKeepAlive() {
			m.cancelKeepAlive()
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
	case "keepalive_autosend":
		m.toggleKeepAliveAutoSendForSelected()
		return m, nil
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
		}
		return m, nil
	case "copy_id":
		m.lastAction = "copy_session_id"
		return m, nil
	case "refresh":
		return m.Update(ManualRefreshMsg{})
	case "help":
		m.helpOpen = !m.helpOpen
		m.lastAction = "toggle_help"
		return m, nil
	case "back":
		m.route = RouteList
		m.lastAction = "back_to_list"
		return m, nil
	case "quit":
		m.lastAction = "quit"
		return m, tea.Quit
	default:
		m.lastAction = "activate_" + action
		return m, nil
	}
}

func (m Model) withKeepAliveActions(actions []keepalive.Action) (tea.Model, tea.Cmd) {
	commands := m.applyKeepAliveActions(actions, nil)
	return m, tea.Batch(commands...)
}

func (m *Model) applyKeepAliveActions(actions []keepalive.Action, commands *[]tea.Cmd) []tea.Cmd {
	if commands == nil {
		local := []tea.Cmd{}
		commands = &local
	}
	for _, action := range actions {
		switch action.Kind {
		case keepalive.ActionCountdownStarted:
			m.countdowns[action.SessionID] = action.CountdownSeconds
		case keepalive.ActionManualPromptShown:
			m.lastAction = "keepalive_manual_prompt"
		case keepalive.ActionStartRunner:
			m.lastAction = "keepalive_runner_ready"
			*commands = append(*commands, m.keepAliveRunnerCommand(action))
		}
	}
	return *commands
}

func (m *Model) toggleKeepAliveAutoSendForSelected() {
	selected := m.selectedSession()
	if selected == nil {
		return
	}
	state := m.KeepAliveState(selected.SessionID)
	if state.State == keepalive.StateSending || state.State == keepalive.StateConfirming || isKeepAliveFailure(state.State) {
		m.lastAction = "keepalive_autosend_disabled"
		return
	}
	next := !state.AutoSend
	m.keepAliveManager.SetAutoSend(selected.SessionID, next)
	if next && m.deps.CheckClaudeAvailable != nil {
		if err := m.deps.CheckClaudeAvailable(); err != nil {
			m.keepAliveManager.MarkNoClaude(selected.SessionID, state.InstanceToken, err.Error())
			m.refresh.ClaudeUnavailableMessage = err.Error()
		}
	}
	m.lastAction = "toggle_keepalive_autosend"
}

func (m Model) workspaceCanSendKeepAlive() bool {
	state := m.activeKeepAliveState()
	return state.State == keepalive.StateCountdown || state.State == keepalive.StateManualReady || m.keepAliveStatus == KeepAliveCountdown || m.keepAliveStatus == KeepAliveFailure
}

func (m Model) workspaceCanCancelKeepAlive() bool {
	state := m.activeKeepAliveState()
	return state.State == keepalive.StateCountdown || state.State == keepalive.StateManualReady || state.State == keepalive.StateSending || state.State == keepalive.StateConfirming || m.keepAliveStatus != KeepAliveInactive
}

func (m Model) sendKeepAliveNow() (tea.Model, tea.Cmd) {
	selected := m.selectedSession()
	if selected == nil {
		if m.keepAliveStatus == KeepAliveCountdown || m.keepAliveStatus == KeepAliveFailure {
			m.lastAction = "send_keepalive_now"
		}
		return m, nil
	}
	state := m.KeepAliveState(selected.SessionID)
	actions := m.keepAliveManager.SendNow(selected.SessionID, state.InstanceToken, m.now)
	commands := m.applyKeepAliveActions(actions, nil)
	m.lastAction = "send_keepalive_now"
	m.focusIndex = m.defaultFocusIndex()
	return m, tea.Batch(commands...)
}

func (m *Model) cancelKeepAlive() {
	selected := m.selectedSession()
	if selected == nil {
		if m.keepAliveStatus != KeepAliveInactive {
			m.lastAction = "cancel_keepalive"
		}
		return
	}
	state := m.KeepAliveState(selected.SessionID)
	m.keepAliveManager.Cancel(selected.SessionID, state.InstanceToken)
	m.lastAction = "cancel_keepalive"
	m.focusIndex = m.defaultFocusIndex()
}

func isKeepAliveFailure(state keepalive.State) bool {
	return state == keepalive.StateErrorNoClaude || state == keepalive.StateErrorSubprocess || state == keepalive.StateErrorTimeout
}

func (m Model) keepAliveAsyncCurrent(sessionID string, token int64, generation int, selectedID string) bool {
	if generation != m.refreshGeneration {
		return false
	}
	if selectedID != "" && selectedID != m.SelectedSessionID() {
		return false
	}
	state := m.KeepAliveState(sessionID)
	return state.InstanceToken == token
}

func (m Model) keepAliveRunnerCommand(action keepalive.Action) tea.Cmd {
	selected, ok := findSessionByID(m.sessions, action.SessionID)
	if !ok {
		return nil
	}
	runner := m.deps.KeepAliveRunner
	generation := m.refreshGeneration
	selectedID := m.SelectedSessionID()
	target := keepalive.NewConfirmationTarget(selected.JSONLPath, time.Time{})
	return func() tea.Msg {
		startedAt := m.now
		if runner == nil {
			err := keepalive.ErrClaudeUnavailable
			return KeepAliveRunnerResultMsg{SessionID: action.SessionID, InstanceToken: action.InstanceToken, StartedAt: startedAt, Err: err, Reason: err.Error(), Generation: generation, SelectedID: selectedID, ConfirmationTarget: target}
		}
		result := runner.Send(context.Background(), keepalive.RunRequest{SessionID: action.SessionID, Message: action.Message})
		if result.StartedAt.IsZero() {
			result.StartedAt = startedAt
		}
		target.After = result.StartedAt
		reason := ""
		if result.Err != nil || result.ExitCode != 0 || result.Limit {
			reason = keepAliveFailureReason(result)
		}
		return KeepAliveRunnerResultMsg{SessionID: action.SessionID, InstanceToken: action.InstanceToken, StartedAt: result.StartedAt, Err: result.Err, Reason: reason, Generation: generation, SelectedID: selectedID, ConfirmationTarget: target}
	}
}

func (m Model) keepAliveConfirmationCommand(msg KeepAliveRunnerResultMsg) tea.Cmd {
	confirm := m.deps.ConfirmKeepAlive
	generation := msg.Generation
	selectedID := msg.SelectedID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var result keepalive.ConfirmationResult
		var err error
		if confirm != nil {
			result, err = confirm(ctx, msg.ConfirmationTarget)
		} else {
			result, err = keepalive.WaitForConfirmation(ctx, msg.ConfirmationTarget.Check)
		}
		return KeepAliveConfirmationResultMsg{SessionID: msg.SessionID, InstanceToken: msg.InstanceToken, ConfirmedAt: result.ConfirmedAt, Err: err, Generation: generation, SelectedID: selectedID}
	}
}

func keepAliveFailureReason(result keepalive.RunResult) string {
	if result.Stderr != "" {
		return result.Stderr
	}
	if result.Stdout != "" {
		return result.Stdout
	}
	if result.Err != nil {
		return result.Err.Error()
	}
	return "claude subprocess failed"
}

func displayTickCommand() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return DisplayTickMsg{Now: t.UTC()}
	})
}

type availabilityChecker struct {
	err error
}

func (c availabilityChecker) Available() error {
	return c.err
}

func (c availabilityChecker) Send(context.Context, keepalive.RunRequest) keepalive.RunResult {
	return keepalive.RunResult{Err: c.err}
}
