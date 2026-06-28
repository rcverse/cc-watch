package tui

import (
	"time"

	"github.com/richardchen/cc-cache/internal/session"
)

type reminderRuntime struct {
	thresholds []int
	enabled    map[string]bool
	fired      map[string]map[int]bool
}

type reminderRuntimeEvent struct {
	sessionID        string
	project          string
	thresholdPercent int
	remainingPercent float64
	occurredAt       time.Time
}

func newReminderRuntime(thresholds []int, enabled map[string]bool, fired map[string]map[int]bool) reminderRuntime {
	return reminderRuntime{
		thresholds: append([]int(nil), thresholds...),
		enabled:    enabled,
		fired:      fired,
	}
}

func (r reminderRuntime) check(now time.Time, sessions []session.Session) []reminderRuntimeEvent {
	var events []reminderRuntimeEvent
	for _, s := range sessions {
		if !r.enabled[s.SessionID] {
			continue
		}
		status := s.StatusAt(now)
		if status.State != session.StatusActive || status.RemainingSeconds == nil || s.CacheWindow.TTLSeconds <= 0 {
			continue
		}
		remainingPercent := float64(*status.RemainingSeconds) / float64(s.CacheWindow.TTLSeconds) * 100
		for _, threshold := range r.thresholds {
			if remainingPercent > float64(threshold) || r.alreadyFired(s.SessionID, threshold) {
				continue
			}
			r.markFired(s.SessionID, threshold)
			events = append(events, reminderRuntimeEvent{
				sessionID:        s.SessionID,
				project:          s.Project,
				thresholdPercent: threshold,
				remainingPercent: remainingPercent,
				occurredAt:       now,
			})
		}
	}
	return events
}

func (r reminderRuntime) alreadyFired(sessionID string, threshold int) bool {
	return r.fired[sessionID][threshold]
}

func (r reminderRuntime) markFired(sessionID string, threshold int) {
	fired := r.fired[sessionID]
	if fired == nil {
		fired = map[int]bool{}
	}
	fired[threshold] = true
	r.fired[sessionID] = fired
}
