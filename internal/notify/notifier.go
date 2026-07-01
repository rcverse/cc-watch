package notify

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

type CommandBuilder func(Notification) (string, []string)

type CommandNotifier struct {
	build  CommandBuilder
	runner Runner
}

type Manager struct {
	notifier           Notifier
	attempts           []Attempt
	lastFailureKey     string
	lastFailureEventID string
}

func NewManager(notifier Notifier) *Manager {
	return &Manager{notifier: notifier}
}

func NewCommandNotifier(build CommandBuilder, runner Runner) CommandNotifier {
	return CommandNotifier{build: build, runner: runner}
}

func ExecRunner(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func MacOSCommand(notification Notification) (string, []string) {
	script := `display notification "` + escapeAppleScript(notification.Body) + `" with title "` + escapeAppleScript(notification.Title) + `"`
	return "osascript", []string{"-e", script}
}

func escapeAppleScript(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func (n CommandNotifier) Notify(event Event) Result {
	notification := FormatEvent(event)
	name, args := n.build(notification)
	if n.runner == nil {
		err := errors.New("notification runner unavailable")
		return Result{Degraded: true, Message: err.Error(), Err: err}
	}
	if err := n.runner(name, args...); err != nil {
		return Result{Degraded: true, Message: err.Error(), Err: err}
	}
	return Result{Delivered: true, Message: "delivered"}
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
			Body:  fmt.Sprintf("Reminder: %d%% cache remaining. No Claude message was sent.", event.ThresholdPercent),
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
			Title: "cc-watch event",
			Body:  "A cc-watch event occurred.",
		}
	}
}

func NewPlatformNotifier(goos string, runner Runner) Notifier {
	return NewCommandNotifier(MacOSCommand, runner)
}

func eventKey(event Event) string {
	return fmt.Sprintf("%s:%s:%d:%d:%s", event.Kind, event.SessionID, event.ThresholdPercent, event.CountdownSeconds, event.Reason)
}
