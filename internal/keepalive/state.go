package keepalive

import (
	"time"

	"github.com/richardchen/cc-watch/internal/session"
)

func (m *Manager) Enable(s session.Session, now time.Time) []Action {
	state := m.initialState(s.SessionID)
	state.State = StateMonitoringIdle
	m.states[s.SessionID] = state
	return m.evaluate(s, now)
}

func (m *Manager) Disable(sessionID string) {
	state := m.State(sessionID)
	state.State = StateOff
	state.InstanceToken = m.nextToken()
	m.states[sessionID] = state
}

func (m *Manager) Check(now time.Time, sessions []session.Session) []Action {
	var actions []Action
	for _, s := range sessions {
		state := m.State(s.SessionID)
		if state.State != StateMonitoringIdle {
			continue
		}
		actions = append(actions, m.evaluate(s, now)...)
	}
	return actions
}

func (m *Manager) Refresh(s session.Session, now time.Time) []Action {
	state := m.State(s.SessionID)
	switch state.State {
	case StateOff, StateScopeComplete:
		return nil
	}
	state.State = StateMonitoringIdle
	state.InstanceToken = m.nextToken()
	m.states[s.SessionID] = state
	return m.evaluate(s, now)
}

func (m *Manager) CountdownElapsed(sessionID string, token int64, now time.Time) []Action {
	state := m.State(sessionID)
	if state.State != StateCountdown || state.InstanceToken != token {
		return nil
	}
	if !state.AutoSend {
		return m.enterManualReady(state, false)
	}
	if !state.LatestSafeSendAt.IsZero() && now.After(state.LatestSafeSendAt) {
		state.AutoSend = false
		return m.enterManualReady(state, true)
	}
	return m.beginSend(state)
}

func (m *Manager) SendNow(sessionID string, token int64, now time.Time) []Action {
	_ = now
	state := m.State(sessionID)
	if state.InstanceToken != token {
		return nil
	}
	if state.State != StateCountdown && state.State != StateManualReady {
		return nil
	}
	return m.beginSend(state)
}

func (m *Manager) Cancel(sessionID string, token int64) {
	state := m.State(sessionID)
	if state.InstanceToken != token {
		return
	}
	switch state.State {
	case StateCountdown, StateManualReady, StateSending, StateConfirming:
		state.State = StateCancelledInstance
		state.InstanceToken = m.nextToken()
		state.TriggerArmed = false
		m.states[sessionID] = state
	}
}

func (m *Manager) Dismiss(sessionID string, token int64) {
	m.Cancel(sessionID, token)
}

func (m *Manager) MarkSendStarted(sessionID string, token int64, startedAt time.Time) {
	_ = startedAt
	state := m.State(sessionID)
	if state.State != StateSending || state.InstanceToken != token {
		return
	}
	state.State = StateConfirming
	m.states[sessionID] = state
}

func (m *Manager) MarkNoClaude(sessionID string, token int64, reason string) {
	m.markFailure(sessionID, token, StateErrorNoClaude, reason)
}

func (m *Manager) MarkSubprocessFailure(sessionID string, token int64, reason string) {
	m.markFailure(sessionID, token, StateErrorSubprocess, reason)
}

func (m *Manager) MarkConfirmationTimeout(sessionID string, token int64) {
	m.markFailure(sessionID, token, StateErrorTimeout, "confirmation timed out")
}

func (m *Manager) MarkSuccess(sessionID string, token int64, confirmedAt time.Time) {
	_ = confirmedAt
	state := m.State(sessionID)
	if state.State != StateConfirming || state.InstanceToken != token {
		return
	}
	state.State = StateSuccess
	state.LastResult = "success"
	m.states[sessionID] = state
}

func (m *Manager) Acknowledge(sessionID string) {
	state := m.State(sessionID)
	switch state.State {
	case StateSuccess:
		if state.scopeExhausted() {
			state.State = StateScopeComplete
		} else {
			state.State = StateMonitoringIdle
			state.InstanceToken = m.nextToken()
			state.TriggerArmed = false
		}
	case StateErrorNoClaude, StateErrorSubprocess, StateErrorTimeout:
		if state.scopeExhausted() {
			state.State = StateScopeComplete
		}
	case StateScopeComplete:
		state.State = StateOff
	}
	m.states[sessionID] = state
}

func (m *Manager) evaluate(s session.Session, now time.Time) []Action {
	state := m.State(s.SessionID)
	if state.scopeExhausted() {
		state.State = StateScopeComplete
		m.states[s.SessionID] = state
		return nil
	}

	timing := EvaluateTiming(s, now, m.cfg)
	if !timing.InsideTrigger {
		state.State = StateMonitoringIdle
		state.TriggerArmed = true
		m.states[s.SessionID] = state
		return nil
	}
	if !state.TriggerArmed {
		state.State = StateMonitoringIdle
		m.states[s.SessionID] = state
		return nil
	}
	state.InstanceToken = m.nextToken()
	state.TriggerArmed = false
	state.LatestSafeSendAt = latestSafeSendAt(s)
	if state.AutoSend && timing.AutoSendAllowed {
		state.State = StateCountdown
		state.SafetyDisabled = false
		m.states[s.SessionID] = state
		return []Action{{
			Kind:             ActionCountdownStarted,
			SessionID:        s.SessionID,
			InstanceToken:    state.InstanceToken,
			CountdownSeconds: timing.EffectiveCountdownSeconds,
			Message:          m.cfg.Message,
		}}
	}

	state.SafetyDisabled = state.AutoSend && !timing.AutoSendAllowed
	return m.enterManualReady(state, state.SafetyDisabled)
}

func (m *Manager) beginSend(state SessionState) []Action {
	if state.scopeExhausted() {
		state.State = StateScopeComplete
		m.states[state.SessionID] = state
		return nil
	}
	state.ScopeUsed++
	state.State = StateSending
	state.TriggerArmed = false
	m.states[state.SessionID] = state
	return []Action{{
		Kind:          ActionStartRunner,
		SessionID:     state.SessionID,
		InstanceToken: state.InstanceToken,
		Message:       m.cfg.Message,
	}}
}

func (m *Manager) markFailure(sessionID string, token int64, failedState State, reason string) {
	state := m.State(sessionID)
	if state.InstanceToken != token {
		return
	}
	switch failedState {
	case StateErrorTimeout:
		if state.State != StateConfirming {
			return
		}
	case StateErrorNoClaude, StateErrorSubprocess:
		if state.State != StateSending && state.State != StateConfirming && state.State != StateCountdown && state.State != StateManualReady {
			return
		}
	}
	state.State = failedState
	state.AutoSend = false
	state.TriggerArmed = false
	state.LastFailure = reason
	m.states[sessionID] = state
}

func (m *Manager) SetAutoSend(sessionID string, enabled bool) {
	state := m.State(sessionID)
	state.AutoSend = enabled
	m.states[sessionID] = state
}

func (m *Manager) enterManualReady(state SessionState, safetyDisabled bool) []Action {
	state.State = StateManualReady
	state.SafetyDisabled = safetyDisabled
	state.TriggerArmed = false
	m.states[state.SessionID] = state
	return []Action{{
		Kind:          ActionManualPromptShown,
		SessionID:     state.SessionID,
		InstanceToken: state.InstanceToken,
		Message:       m.cfg.Message,
	}}
}

func latestSafeSendAt(s session.Session) time.Time {
	if s.LastMessageAt == nil {
		return time.Time{}
	}
	ttl, _ := effectiveTTL(s)
	return s.LastMessageAt.Add(time.Duration(ttl-safetyMarginSeconds) * time.Second)
}
