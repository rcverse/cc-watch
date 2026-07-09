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
	if !state.LatestSafeSendAt.IsZero() && now.After(state.LatestSafeSendAt) {
		state.State = StatePaused
		state.SafetyDisabled = true
		state.LastFailure = "send window passed"
		m.states[sessionID] = state
		return nil
	}
	return m.beginSend(state)
}

func (m *Manager) SendNow(sessionID string, token int64) []Action {
	state := m.State(sessionID)
	if state.InstanceToken != token {
		return nil
	}
	if state.State != StateCountdown {
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
	case StateCountdown, StateSending, StateConfirming:
		state.State = StateMonitoringIdle
		state.InstanceToken = m.nextToken()
		state.TriggerArmed = false
		m.states[sessionID] = state
	}
}

func (m *Manager) Dismiss(sessionID string, token int64) {
	m.Cancel(sessionID, token)
}

func (m *Manager) MarkSendStarted(sessionID string, token int64) {
	state := m.State(sessionID)
	if state.State != StateSending || state.InstanceToken != token {
		return
	}
	state.State = StateConfirming
	m.states[sessionID] = state
}

func (m *Manager) MarkNoClaude(sessionID string, token int64, reason string) {
	m.markFailure(sessionID, token, StateErrorNoClaude, reason, false)
}

func (m *Manager) MarkSubprocessFailure(sessionID string, token int64, reason string, limited bool) {
	m.markFailure(sessionID, token, StateErrorSubprocess, reason, limited)
}

func (m *Manager) MarkConfirmationTimeout(sessionID string, token int64) {
	m.markFailure(sessionID, token, StateErrorTimeout, "confirmation timed out", false)
}

func (m *Manager) MarkSuccess(sessionID string, token int64) {
	state := m.State(sessionID)
	if state.State != StateConfirming || state.InstanceToken != token {
		return
	}
	state.LastResult = "Sent and confirmed"
	if state.scopeExhausted() {
		state.State = StateScopeComplete
	} else {
		state.State = StateMonitoringIdle
		state.TriggerArmed = false
	}
	m.states[sessionID] = state
}

func (m *Manager) ResetLimit(sessionID string) {
	state := m.State(sessionID)
	state.ScopeUsed = 0
	state.State = StateMonitoringIdle
	state.TriggerArmed = true
	state.LastFailure = ""
	state.RateLimited = false
	state.InstanceToken = m.nextToken()
	m.states[sessionID] = state
}

func (m *Manager) evaluate(s session.Session, now time.Time) []Action {
	state := m.State(s.SessionID)
	if state.scopeExhausted() {
		state.State = StateScopeComplete
		m.states[s.SessionID] = state
		return []Action{{Kind: ActionScopeComplete, SessionID: s.SessionID, InstanceToken: state.InstanceToken}}
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
	if timing.SendAllowed {
		state.State = StateCountdown
		state.SafetyDisabled = false
		state.LastFailure = ""
		m.states[s.SessionID] = state
		return []Action{{
			Kind:             ActionCountdownStarted,
			SessionID:        s.SessionID,
			InstanceToken:    state.InstanceToken,
			CountdownSeconds: timing.EffectiveCountdownSeconds,
			Message:          m.cfg.Message,
		}}
	}

	state.State = StatePaused
	state.SafetyDisabled = true
	state.LastFailure = "countdown does not fit the safe send window"
	m.states[s.SessionID] = state
	return nil
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

func (m *Manager) markFailure(sessionID string, token int64, failedState State, reason string, limited bool) {
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
		if state.State != StateSending && state.State != StateConfirming && state.State != StateCountdown {
			return
		}
	}
	state.State = failedState
	state.TriggerArmed = false
	state.LastFailure = reason
	state.RateLimited = limited
	m.states[sessionID] = state
}

func latestSafeSendAt(s session.Session) time.Time {
	if s.LastMessageAt == nil {
		return time.Time{}
	}
	ttl, _ := effectiveTTL(s)
	return s.LastMessageAt.Add(time.Duration(ttl-sendDeadlineMarginSeconds) * time.Second)
}
