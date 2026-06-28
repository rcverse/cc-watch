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
		m.clearExpiredNotice()
		m.enforceKeepAliveEligibility()
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
	case RefreshTickMsg:
		m.now = typed.Now
		decision := m.refreshCoordinator.OnSafetyTick(typed.Now)
		updated, cmd := m.scheduleRefreshDecision(decision)
		m = updated.(Model)
		if m.startRefreshTicker {
			return m, tea.Batch(cmd, refreshTickCommand(m.refreshTiming.SafetyInterval))
		}
		return m, cmd
	case WatcherEventMsg:
		m.watcherEvents = append(m.watcherEvents, typed)
		return m.scheduleRefresh(refresh.SourceFsnotify, false)
	case RefreshWatcherEventsMsg:
		m.refresh.Watcher = typed.State
		decision := m.refreshCoordinator.OnWatcherEvents(typed.Events)
		m.refreshDebounceToken = decision.DebounceToken
		if m.liveRefresh != nil {
			return m, tea.Batch(refreshDebounceCommand(m.refreshTiming.Debounce, m.refreshDebounceToken), m.liveRefresh)
		}
		return m, refreshDebounceCommand(m.refreshTiming.Debounce, m.refreshDebounceToken)
	case RefreshWatcherDegradedMsg:
		m.refresh.Watcher = typed.State
		if m.liveRefresh != nil {
			return m, m.liveRefresh
		}
		return m, nil
	case RefreshWatcherClosedMsg:
		m.refresh.Watcher = typed.State
		return m, nil
	case RefreshDebounceElapsedMsg:
		decision := m.refreshCoordinator.OnDebounceElapsed(typed.Now, typed.Token)
		return m.scheduleRefreshDecision(decision)
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
		if m.route == RouteList || m.route == RouteAmbiguous {
			m.focusIndex = m.selectedIndex
		}
		m.enforceKeepAliveEligibility()
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
		state := m.keepAliveManager.ApplyRunnerExecution(typed.Action, typed.Execution)
		if state.State == keepalive.StateConfirming {
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
		if m.route == RouteWorkspace {
			m.setNotice("updating selected session", RoleInfo, 3*time.Second)
		} else {
			m.setNotice("updating sessions", RoleInfo, 3*time.Second)
		}
		decision := m.refreshCoordinator.OnManualRefresh()
		return m.scheduleRefreshDecision(decision)
	case tea.KeyMsg:
		return m.updateKey(typed)
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
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
	runtime := newReminderRuntime(m.reminderThresholds, m.reminderEnabled, m.reminderFired)
	events := runtime.check(m.now, m.sessions)
	m.reminderFired = runtime.fired
	for _, event := range events {
		commands = append(commands, m.notificationCommand(notify.Event{
			Kind:             notify.EventReminderThresholdCrossed,
			SessionID:        event.sessionID,
			Project:          event.project,
			ThresholdPercent: event.thresholdPercent,
		}))
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
	decision := m.refreshCoordinator.NextRefresh(source)
	decision.BypassedDebounce = bypassedDebounce
	return m.scheduleRefreshDecision(decision)
}

func (m Model) scheduleRefreshDecision(decision refresh.Decision) (tea.Model, tea.Cmd) {
	if !decision.ShouldRefresh {
		return m, nil
	}
	m.refreshGeneration = decision.Generation
	m.lastRefreshSource = decision.Source
	m.lastBypassedDebounce = decision.BypassedDebounce
	return m.refreshCommand(decision.Source, decision.Generation)
}

func (m Model) refreshCommand(source refresh.Source, generation int) (tea.Model, tea.Cmd) {
	refreshSnapshot := m.deps.RefreshSnapshot
	selected := m.selectedSession()
	if refreshSnapshot == nil {
		return m, nil
	}
	var selectedForRefresh *session.Session
	if m.route == RouteWorkspace && selected != nil {
		copied := *selected
		selectedForRefresh = &copied
	}
	return m, func() tea.Msg {
		snapshot := refreshSnapshot(source, generation, selectedForRefresh)
		return RefreshResultMsg{
			Generation:   generation,
			Sessions:     snapshot.Sessions,
			Refresh:      snapshot.Refresh,
			HasRefresh:   snapshot.HasRefresh,
			SelectedOnly: snapshot.SelectedOnly,
			SelectedID:   snapshot.SelectedID,
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
	if m.route == RouteConfig && m.configEditing {
		return m.updateConfigEditing(msg)
	}
	switch msg.String() {
	case "?":
		m.helpOpen = !m.helpOpen
		m.lastAction = "toggle_help"
		return m, nil
	case "q", "ctrl+c":
		m.lastAction = "quit"
		return m, tea.Quit
	case "down":
		if m.route == RouteWorkspace && m.FocusedAction() == "details_scroll" && m.detailsCanScroll(1) {
			m.detailsOffset++
			return m, nil
		}
		m.moveFocus(1)
		return m, nil
	case "up":
		if m.route == RouteWorkspace && m.FocusedAction() == "details_scroll" && m.detailsCanScroll(-1) {
			m.detailsOffset--
			return m, nil
		}
		m.moveFocus(-1)
		return m, nil
	case "enter":
		return m.activateFocused()
	case " ":
		if m.FocusedAction() == "reminder" || m.FocusedAction() == "keepalive" || m.FocusedAction() == "keepalive_autosend" || m.FocusedAction() == "config_autosend" {
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
	case "u":
		if m.route == RouteList || m.route == RouteWorkspace {
			return m.Update(ManualRefreshMsg{})
		}
		return m, nil
	case "c":
		if m.route == RouteList || m.route == RouteAmbiguous {
			m.route = RouteConfig
			m.focusIndex = m.defaultFocusIndex()
			m.lastAction = "open_config"
		} else if m.route == RouteWorkspace {
			m.lastAction = "copy_session_id"
			if selected := m.selectedSession(); selected != nil {
				m.setNotice("Session ID shown: "+selected.SessionID, RoleInfo, 3*time.Second)
			}
		}
		return m, nil
	case "v":
		if m.route == RouteWorkspace {
			m.sessionInfoExpanded = !m.sessionInfoExpanded
			m.detailsOffset = 0
			m.focusIndex = m.defaultFocusIndex()
			m.lastAction = "toggle_session_info_details"
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
		} else if m.route == RouteConfig {
			return m.cancelConfig()
		}
		return m, nil
	case "s":
		switch {
		case m.route == RouteConfig:
			return m.saveConfig()
		case m.route == RouteWorkspace && m.sessionInfoExpanded:
			m.gapSortNewest = !m.gapSortNewest
			m.lastAction = "toggle_gap_sort"
			return m, nil
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
			return m.resetConfigDefaults()
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) activateFocused() (tea.Model, tea.Cmd) {
	return m.activateFocusedAction(m.FocusedAction())
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
	if reason := m.keepAliveUnavailableReason(*selected); reason != "" {
		m.disableKeepAlive(selected.SessionID)
		m.lastAction = "keepalive_unavailable_expired"
		m.setNotice("KeepAlive "+reason, RoleWarning, 3*time.Second)
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

func (m Model) sendKeepAliveNow() (tea.Model, tea.Cmd) {
	selected := m.selectedSession()
	if selected == nil {
		return m, nil
	}
	if reason := m.keepAliveUnavailableReason(*selected); reason != "" {
		m.disableKeepAlive(selected.SessionID)
		m.lastAction = "keepalive_unavailable_expired"
		m.setNotice("KeepAlive "+reason, RoleWarning, 3*time.Second)
		m.restoreFocusAction("keepalive")
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
		return
	}
	state := m.KeepAliveState(selected.SessionID)
	if state.State == keepalive.StateMonitoringIdle || state.State == keepalive.StateCancelledInstance {
		m.disableKeepAlive(selected.SessionID)
	} else {
		m.keepAliveManager.Cancel(selected.SessionID, state.InstanceToken)
	}
	m.lastAction = "cancel_keepalive"
	m.setNotice("KeepAlive cancelled", RoleInfo, 3*time.Second)
	m.restoreFocusAction("keepalive")
}

func (m *Model) enforceKeepAliveEligibility() {
	for _, s := range m.sessions {
		if m.keepAliveUnavailableReason(s) == "" {
			continue
		}
		state := m.KeepAliveState(s.SessionID)
		if state.State != "" && state.State != keepalive.StateOff {
			m.disableKeepAlive(s.SessionID)
		}
	}
}

func (m *Model) disableKeepAlive(sessionID string) {
	if m.keepAliveManager != nil {
		m.keepAliveManager.Disable(sessionID)
	}
	m.keepAliveEnabled[sessionID] = false
	delete(m.countdowns, sessionID)
}

func (m *Model) setNotice(message string, role StyleRole, ttl time.Duration) {
	m.notice = Notice{Message: message, Role: role, ExpiresAt: m.now.Add(ttl)}
}

func (m *Model) clearExpiredNotice() {
	if !m.notice.ExpiresAt.IsZero() && !m.now.Before(m.notice.ExpiresAt) {
		m.notice = Notice{}
	}
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
		execution := keepalive.ExecuteRunner(context.Background(), action, runner, startedAt)
		result := execution.Result
		target.After = result.StartedAt
		return KeepAliveRunnerResultMsg{
			SessionID:          action.SessionID,
			InstanceToken:      action.InstanceToken,
			StartedAt:          result.StartedAt,
			Err:                result.Err,
			Reason:             keepAliveFailureReason(result),
			Action:             action,
			Execution:          execution,
			Generation:         generation,
			SelectedID:         selectedID,
			ConfirmationTarget: target,
		}
	}
}

func (m Model) keepAliveConfirmationCommand(msg KeepAliveRunnerResultMsg) tea.Cmd {
	confirm := m.deps.ConfirmKeepAlive
	generation := msg.Generation
	selectedID := msg.SelectedID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), keepalive.ConfirmationTimeout)
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

func refreshDebounceCommand(delay time.Duration, token int) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return RefreshDebounceElapsedMsg{Now: t.UTC(), Token: token}
	})
}

func refreshTickCommand(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return RefreshTickMsg{Now: t.UTC()}
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
