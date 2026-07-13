package tui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rcverse/cc-watch/internal/keepalive"
	"github.com/rcverse/cc-watch/internal/notify"
	"github.com/rcverse/cc-watch/internal/refresh"
	"github.com/rcverse/cc-watch/internal/session"
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
		updated, cmd := m.scheduleRefresh()
		m = updated.(Model)
		if m.startRefreshTicker {
			return m, tea.Batch(cmd, refreshTickCommand(m.refreshTiming.SafetyInterval))
		}
		return m, cmd
	case RefreshWatcherChangedMsg:
		m.refresh.Watcher = typed.State
		m.refreshDebounceToken = m.refreshCoordinator.OnWatcherEvent()
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
		decision := m.refreshCoordinator.OnDebounceElapsed(typed.Token)
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
		if !m.keepAliveAsyncCurrent(typed.SessionID, typed.InstanceToken, typed.SelectedID) {
			return m, nil
		}
		return m.withKeepAliveActions(m.keepAliveManager.CountdownElapsed(typed.SessionID, typed.InstanceToken, typed.Now))
	case KeepAliveRunnerResultMsg:
		if !m.keepAliveAsyncCurrent(typed.SessionID, typed.InstanceToken, typed.SelectedID) {
			return m, nil
		}
		state := m.keepAliveManager.ApplyRunnerExecution(typed.Action, typed.Execution)
		if state.State == keepalive.StateConfirming {
			return m, m.keepAliveConfirmationCommand(typed)
		}
		return m, m.keepAliveLifecycleNotification(typed.SessionID, state)
	case KeepAliveConfirmationResultMsg:
		if !m.keepAliveAsyncCurrent(typed.SessionID, typed.InstanceToken, typed.SelectedID) {
			return m, nil
		}
		if typed.Err != nil {
			if errors.Is(typed.Err, keepalive.ErrConfirmationTimeout) {
				m.keepAliveManager.MarkConfirmationTimeout(typed.SessionID, typed.InstanceToken)
			} else {
				m.keepAliveManager.MarkSubprocessFailure(typed.SessionID, typed.InstanceToken, typed.Err.Error(), false)
			}
			return m, m.keepAliveLifecycleNotification(typed.SessionID, m.KeepAliveState(typed.SessionID))
		}
		m.keepAliveManager.MarkSuccess(typed.SessionID, typed.InstanceToken)
		state := m.KeepAliveState(typed.SessionID)
		m.setNotice("✓ KeepAlive sent and confirmed", RoleSuccess, 3*time.Second)
		commands := []tea.Cmd{
			m.keepAliveSuccessNotification(typed.SessionID),
		}
		if state.State == keepalive.StateScopeComplete {
			commands = append(commands, m.keepAliveLifecycleNotification(typed.SessionID, state))
		}
		return m, tea.Batch(commands...)
	case ManualRefreshMsg:
		m.refresh.NotificationDegraded = ""
		if m.deps.ResetNotificationSuppression != nil {
			m.deps.ResetNotificationSuppression()
		}
		if m.route == RouteWorkspace {
			m.setNotice("updating selected session", RoleInfo, 3*time.Second)
		} else {
			m.setNotice("updating sessions", RoleInfo, 3*time.Second)
		}
		return m.scheduleRefresh()
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
			ShortID:          event.shortID,
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
	m.lastNotification = &status
	if msg.Result.Degraded {
		m.refresh.NotificationDegraded = msg.Result.Message
	} else if msg.Result.Delivered {
		m.refresh.NotificationDegraded = ""
	}
}

func (m Model) scheduleRefresh() (tea.Model, tea.Cmd) {
	return m.scheduleRefreshDecision(m.refreshCoordinator.Refresh())
}

func (m Model) scheduleRefreshDecision(decision refresh.Decision) (tea.Model, tea.Cmd) {
	if !decision.ShouldRefresh {
		return m, nil
	}
	m.refreshGeneration = decision.Generation
	return m.refreshCommand(decision.Generation)
}

func (m Model) refreshCommand(generation int) (tea.Model, tea.Cmd) {
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
		snapshot := refreshSnapshot(selectedForRefresh)
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
	if m.configChoiceField != "" {
		return m.updateConfigChoice(msg)
	}
	if (m.route == RouteConfig || m.route == RouteStatusline) && m.configEditing {
		return m.updateConfigEditing(msg)
	}
	switch msg.String() {
	case "?":
		return m, nil
	case "n", "right", "pagedown":
		if m.route == RouteList {
			m.moveListPage(1)
		}
		return m, nil
	case "p", "left", "pageup":
		if m.route == RouteList {
			m.moveListPage(-1)
		}
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	case "down":
		if m.route == RouteWorkspace && m.FocusedAction() == "details_scroll" {
			if m.detailsCanScroll(1) {
				m.detailsOffset++
			}
			return m, nil
		}
		if m.route == RouteConfig || m.route == RouteStatusline {
			m.configStatuslineConfirm = false
		}
		m.moveFocus(1)
		return m, nil
	case "up":
		if m.route == RouteWorkspace && m.FocusedAction() == "details_scroll" {
			if m.detailsCanScroll(-1) {
				m.detailsOffset--
			}
			return m, nil
		}
		if m.route == RouteConfig || m.route == RouteStatusline {
			m.configStatuslineConfirm = false
		}
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
			return m, m.toggleKeepAliveForSelected()
		}
		return m, nil
	case "u":
		if m.route == RouteList || m.route == RouteWorkspace {
			return m.Update(ManualRefreshMsg{})
		}
		return m, nil
	case "c":
		if m.route == RouteList || m.route == RouteAmbiguous {
			m.configReturnRoute = m.route
			m.route = RouteConfig
			m.focusIndex = m.defaultFocusIndex()
		} else if m.route == RouteWorkspace {
			return m, nil
		}
		return m, nil
	case "v":
		if m.route == RouteWorkspace {
			m.sessionInfoExpanded = !m.sessionInfoExpanded
			m.detailsOffset = 0
			m.focusIndex = m.defaultFocusIndex()
		}
		return m, nil
	case "b", "esc":
		if m.route == RouteWorkspace {
			m.route = RouteList
		} else if m.route == RouteAmbiguous {
			m.route = RouteList
		} else if m.route == RouteStatusline {
			m.route = RouteConfig
			m.configStatuslineConfirm = false
			m.focusIndex = m.defaultFocusIndex()
		} else if m.route == RouteConfig {
			return m.cancelConfig()
		}
		return m, nil
	case "s":
		switch {
		case m.route == RouteConfig || m.route == RouteStatusline:
			return m.saveConfig()
		case m.route == RouteWorkspace && m.sessionInfoExpanded:
			m.gapSortNewest = !m.gapSortNewest
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
		if m.route == RouteConfig || m.route == RouteStatusline {
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
		case keepalive.ActionStartRunner:
			*commands = append(*commands, m.keepAliveRunnerCommand(action))
		case keepalive.ActionScopeComplete:
			if cmd := m.keepAliveLifecycleNotification(action.SessionID, m.KeepAliveState(action.SessionID)); cmd != nil {
				*commands = append(*commands, cmd)
			}
		}
	}
	return *commands
}

// keepAliveLifecycleNotification maps a KeepAlive state to the matching
// notify.Event and returns the tea.Cmd to dispatch it, or nil if this state
// doesn't warrant an OS notification (e.g. transient states stay TUI-only).
func (m Model) keepAliveLifecycleNotification(sessionID string, state keepalive.SessionState) tea.Cmd {
	if m.deps.NotifyEvent == nil {
		return nil
	}
	event, ok := keepAliveNotifyEvent(sessionID, state)
	if !ok {
		return nil
	}
	if selected, found := findSessionByID(m.sessions, sessionID); found {
		event.ShortID = selected.ShortID
		event.Project = selected.Project
	}
	return m.notificationCommand(event)
}

func keepAliveNotifyEvent(sessionID string, state keepalive.SessionState) (notify.Event, bool) {
	switch state.State {
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return notify.Event{Kind: notify.EventKeepAliveFailure, SessionID: sessionID, Reason: state.LastFailure, RateLimited: state.RateLimited}, true
	case keepalive.StateScopeComplete:
		return notify.Event{Kind: notify.EventKeepAliveScopeComplete, SessionID: sessionID}, true
	default:
		return notify.Event{}, false
	}
}

func (m Model) keepAliveSuccessNotification(sessionID string) tea.Cmd {
	if m.deps.NotifyEvent == nil {
		return nil
	}
	event := notify.Event{Kind: notify.EventKeepAliveSuccess, SessionID: sessionID}
	if selected, found := findSessionByID(m.sessions, sessionID); found {
		event.ShortID = selected.ShortID
		event.Project = selected.Project
	}
	return m.notificationCommand(event)
}

func (m Model) sendKeepAliveNow() (tea.Model, tea.Cmd) {
	selected := m.selectedSession()
	if selected == nil {
		return m, nil
	}
	if reason := m.keepAliveUnavailableReason(*selected); reason != "" {
		m.disableKeepAlive(selected.SessionID)
		m.setNotice("KeepAlive N/A "+reason, RoleMuted, 3*time.Second)
		m.restoreFocusAction("keepalive")
		return m, nil
	}
	state := m.KeepAliveState(selected.SessionID)
	actions := m.keepAliveManager.SendNow(selected.SessionID, state.InstanceToken)
	commands := m.applyKeepAliveActions(actions, nil)
	m.focusIndex = m.defaultFocusIndex()
	return m, tea.Batch(commands...)
}

func (m *Model) cancelKeepAlive() {
	selected := m.selectedSession()
	if selected == nil {
		return
	}
	state := m.KeepAliveState(selected.SessionID)
	if state.State == keepalive.StateMonitoringIdle {
		m.disableKeepAlive(selected.SessionID)
	} else {
		m.keepAliveManager.Cancel(selected.SessionID, state.InstanceToken)
	}
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

func (m Model) keepAliveAsyncCurrent(sessionID string, token int64, selectedID string) bool {
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
	selectedID := m.SelectedSessionID()
	action.Dir = selected.Cwd
	target := keepalive.NewConfirmationTarget(selected.JSONLPath, time.Time{})
	return func() tea.Msg {
		startedAt := m.now
		ctx, cancel := context.WithTimeout(context.Background(), keepalive.SendTimeout)
		defer cancel()
		execution := keepalive.ExecuteRunner(ctx, action, runner, startedAt)
		keepalive.LogSend(action, execution)
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
			SelectedID:         selectedID,
			ConfirmationTarget: target,
		}
	}
}

func (m Model) keepAliveConfirmationCommand(msg KeepAliveRunnerResultMsg) tea.Cmd {
	confirm := m.deps.ConfirmKeepAlive
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
		keepalive.LogConfirm(msg.SessionID, msg.InstanceToken, msg.ConfirmationTarget, result, err)
		return KeepAliveConfirmationResultMsg{SessionID: msg.SessionID, InstanceToken: msg.InstanceToken, ConfirmedAt: result.ConfirmedAt, Err: err, SelectedID: selectedID}
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
