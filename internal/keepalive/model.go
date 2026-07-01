package keepalive

import (
	"time"

	"github.com/richardchen/cc-watch/internal/config"
)

type State string

const (
	StateOff               State = "off"
	StateMonitoringIdle    State = "monitoring_idle"
	StateCountdown         State = "countdown"
	StateManualReady       State = "manual_ready"
	StateSending           State = "sending"
	StateConfirming        State = "confirming"
	StateSuccess           State = "success"
	StateErrorNoClaude     State = "error_no_claude"
	StateErrorSubprocess   State = "error_subprocess"
	StateErrorTimeout      State = "error_timeout"
	StateCancelledInstance State = "cancelled_instance"
	StateScopeComplete     State = "scope_complete"
)

type ActionKind string

const (
	ActionCountdownStarted  ActionKind = "countdown_started"
	ActionManualPromptShown ActionKind = "manual_prompt_shown"
	ActionStartRunner       ActionKind = "start_runner"
)

type Action struct {
	Kind             ActionKind
	SessionID        string
	InstanceToken    int64
	CountdownSeconds int
	Message          string
}

type SessionState struct {
	SessionID        string
	State            State
	AutoSend         bool
	ScopeUsed        int
	MaxSends         int
	InstanceToken    int64
	TriggerArmed     bool
	LatestSafeSendAt time.Time
	LastResult       string
	LastFailure      string
	SafetyDisabled   bool
}

type Manager struct {
	cfg     config.KeepAliveConfig
	states  map[string]SessionState
	tokenID int64
}

func NewManager(cfg config.KeepAliveConfig) *Manager {
	return &Manager{
		cfg:    cfg,
		states: map[string]SessionState{},
	}
}

func (m *Manager) State(sessionID string) SessionState {
	state, ok := m.states[sessionID]
	if !ok {
		return SessionState{
			SessionID:    sessionID,
			State:        StateOff,
			AutoSend:     m.cfg.AutoSend,
			MaxSends:     m.cfg.Scope.MaxSends,
			TriggerArmed: true,
		}
	}
	return state
}

func (m *Manager) SetState(state SessionState) {
	if state.SessionID == "" {
		return
	}
	if state.MaxSends == 0 {
		state.MaxSends = m.cfg.Scope.MaxSends
	}
	m.states[state.SessionID] = state
	if state.InstanceToken > m.tokenID {
		m.tokenID = state.InstanceToken
	}
}

func (m *Manager) nextToken() int64 {
	m.tokenID++
	return m.tokenID
}

func (m *Manager) initialState(sessionID string) SessionState {
	state := m.State(sessionID)
	if state.State == StateOff && state.InstanceToken == 0 {
		state.AutoSend = m.cfg.AutoSend
		state.MaxSends = m.cfg.Scope.MaxSends
		state.TriggerArmed = true
	}
	if state.MaxSends == 0 {
		state.MaxSends = m.cfg.Scope.MaxSends
	}
	return state
}

func (s SessionState) scopeExhausted() bool {
	return s.MaxSends > 0 && s.ScopeUsed >= s.MaxSends
}
