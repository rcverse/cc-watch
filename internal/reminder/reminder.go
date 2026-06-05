package reminder

import (
	"time"

	"github.com/richardchen/cc-cache/internal/config"
	"github.com/richardchen/cc-cache/internal/session"
)

type Event struct {
	Kind                    string
	SessionID               string
	ThresholdPercent        int
	RemainingPercent        float64
	OccurredAt              time.Time
	ClaudeSubprocessStarted bool
}

type Manager struct {
	thresholds []int
	sessions   map[string]sessionState
}

type sessionState struct {
	enabled bool
	fired   map[int]bool
}

func New(cfg config.Config) *Manager {
	return &Manager{
		thresholds: append([]int(nil), cfg.ReminderThresholds...),
		sessions:   map[string]sessionState{},
	}
}

func (m *Manager) Enable(sessionID string) {
	state := m.sessions[sessionID]
	state.enabled = true
	if state.fired == nil {
		state.fired = map[int]bool{}
	}
	m.sessions[sessionID] = state
}

func (m *Manager) EnableLoadedSessions(sessions []session.Session) {
	for _, s := range sessions {
		m.Enable(s.SessionID)
	}
}

func (m *Manager) Disable(sessionID string) {
	state := m.sessions[sessionID]
	state.enabled = false
	if state.fired == nil {
		state.fired = map[int]bool{}
	}
	m.sessions[sessionID] = state
}

func (m *Manager) Enabled(sessionID string) bool {
	return m.sessions[sessionID].enabled
}

func (m *Manager) Check(now time.Time, sessions []session.Session) []Event {
	var events []Event
	for _, s := range sessions {
		state := m.sessions[s.SessionID]
		if !state.enabled || state.fired == nil {
			continue
		}
		status := s.StatusAt(now)
		if status.State != session.StatusActive || status.RemainingSeconds == nil {
			continue
		}
		remainingPercent := float64(*status.RemainingSeconds) / float64(s.CacheWindow.TTLSeconds) * 100
		for _, threshold := range m.thresholds {
			if remainingPercent > float64(threshold) || state.fired[threshold] {
				continue
			}
			events = append(events, Event{
				Kind:             "reminder_threshold_crossed",
				SessionID:        s.SessionID,
				ThresholdPercent: threshold,
				RemainingPercent: remainingPercent,
				OccurredAt:       now,
			})
			state.fired[threshold] = true
		}
		m.sessions[s.SessionID] = state
	}
	return events
}
