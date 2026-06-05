package notify

import (
	"errors"
	"fmt"
	"runtime"
)

type EventKind string

const (
	EventReminderThresholdCrossed   EventKind = "reminder_threshold_crossed"
	EventKeepAliveCountdownStarted  EventKind = "keepalive_countdown_started"
	EventKeepAliveManualPromptShown EventKind = "keepalive_manual_prompt_shown"
	EventKeepAliveSent              EventKind = "keepalive_sent"
	EventKeepAliveSuccess           EventKind = "keepalive_success"
	EventKeepAliveFailure           EventKind = "keepalive_failure"
	EventKeepAliveScopeComplete     EventKind = "keepalive_scope_complete"
)

type Event struct {
	Kind             EventKind
	SessionID        string
	Project          string
	ThresholdPercent int
	CountdownSeconds int
	Reason           string
}

type Notification struct {
	Title string
	Body  string
}

type Result struct {
	Delivered  bool
	Degraded   bool
	Suppressed bool
	Message    string
	Err        error
}

type Attempt struct {
	Event        Event
	Notification Notification
	Result       Result
}

type Notifier interface {
	Notify(Event) Result
}

type Runner func(name string, args ...string) error

type Manager struct {
	notifier           Notifier
	attempts           []Attempt
	lastFailureKey     string
	lastFailureEventID string
}

func NewManager(notifier Notifier) *Manager {
	return &Manager{notifier: notifier}
}

func (m *Manager) Notify(event Event) Result {
	notification := FormatEvent(event)
	eventID := eventKey(event)
	if m.lastFailureKey != "" && eventID == m.lastFailureEventID {
		return Result{
			Degraded:   true,
			Suppressed: true,
			Message:    m.lastFailureKey,
			Err:        errors.New(m.lastFailureKey),
		}
	}
	if m.notifier == nil {
		result := Result{Degraded: true, Message: "notifications unavailable", Err: errors.New("notifications unavailable")}
		m.record(event, notification, result)
		m.lastFailureKey = result.Message
		m.lastFailureEventID = eventID
		return result
	}

	result := m.notifier.Notify(event)
	if result.Err != nil || result.Degraded {
		failureKey := result.Message
		if result.Err != nil {
			failureKey = result.Err.Error()
		}
		m.lastFailureKey = failureKey
		m.lastFailureEventID = eventID
	} else if result.Delivered {
		m.ResetSuppression()
	}
	m.record(event, notification, result)
	return result
}

func (m *Manager) Attempts() []Attempt {
	return append([]Attempt(nil), m.attempts...)
}

func (m *Manager) ResetSuppression() {
	m.lastFailureKey = ""
	m.lastFailureEventID = ""
}

func (m *Manager) record(event Event, notification Notification, result Result) {
	m.attempts = append(m.attempts, Attempt{
		Event:        event,
		Notification: notification,
		Result:       result,
	})
}

func FormatEvent(event Event) Notification {
	switch event.Kind {
	case EventReminderThresholdCrossed:
		return Notification{
			Title: "Reminder alarm",
			Body:  fmt.Sprintf("Cache window crossed %d%% remaining. Sends no Claude message.", event.ThresholdPercent),
		}
	case EventKeepAliveCountdownStarted:
		return Notification{
			Title: "KeepAlive countdown",
			Body:  fmt.Sprintf("A Claude message may be sent after %ds unless canceled.", event.CountdownSeconds),
		}
	case EventKeepAliveManualPromptShown:
		return Notification{
			Title: "KeepAlive manual prompt",
			Body:  "No Claude message was sent. Send manually or dismiss.",
		}
	case EventKeepAliveSent:
		return Notification{
			Title: "KeepAlive send started",
			Body:  "Claude message send started for this session.",
		}
	case EventKeepAliveSuccess:
		return Notification{
			Title: "KeepAlive sent and confirmed",
			Body:  "Claude message was sent and session JSONL evidence confirmed the result.",
		}
	case EventKeepAliveFailure:
		reason := event.Reason
		if reason == "" {
			reason = "result not confirmed"
		}
		return Notification{
			Title: "KeepAlive stopped",
			Body:  "Claude message was not confirmed. " + reason,
		}
	case EventKeepAliveScopeComplete:
		return Notification{
			Title: "KeepAlive scope complete",
			Body:  "No more automatic sends remain for this session.",
		}
	default:
		return Notification{
			Title: "cc-cache event",
			Body:  "A cc-cache event occurred.",
		}
	}
}

func NewPlatformNotifier(goos string, runner Runner) Notifier {
	if goos == "" {
		goos = runtime.GOOS
	}
	switch goos {
	case "darwin":
		return NewCommandNotifier(MacOSCommand, runner)
	case "linux":
		return NewCommandNotifier(LinuxCommand, runner)
	default:
		return UnsupportedNotifier{GOOS: goos}
	}
}

func eventKey(event Event) string {
	return fmt.Sprintf("%s:%s:%d:%d:%s", event.Kind, event.SessionID, event.ThresholdPercent, event.CountdownSeconds, event.Reason)
}
