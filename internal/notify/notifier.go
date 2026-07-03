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
	EventKeepAliveManualPromptShown EventKind = "keepalive_manual_prompt_shown"
	EventKeepAliveSuccess           EventKind = "keepalive_success"
	EventKeepAliveFailure           EventKind = "keepalive_failure"
	EventKeepAliveScopeComplete     EventKind = "keepalive_scope_complete"
)

type Event struct {
	Kind             EventKind
	SessionID        string
	ShortID          string
	Project          string
	ThresholdPercent int
	CountdownSeconds int
	Reason           string
	RateLimited      bool
}

type Notification struct {
	Title    string
	Subtitle string
	Body     string
	Sound    string
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
	if notification.Subtitle != "" {
		script += ` subtitle "` + escapeAppleScript(notification.Subtitle) + `"`
	}
	if notification.Sound != "" {
		script += ` sound name "` + escapeAppleScript(notification.Sound) + `"`
	}
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
	subtitle := subtitleFor(event)
	switch event.Kind {
	case EventReminderThresholdCrossed:
		return Notification{
			Title:    "cc-watch · Reminder",
			Subtitle: subtitle,
			Body:     fmt.Sprintf("%d%% cache remaining. No message sent — reminder only.", event.ThresholdPercent),
		}
	case EventKeepAliveManualPromptShown:
		return Notification{
			Title:    "cc-watch · KeepAlive",
			Subtitle: subtitle,
			Body:     "Cache is about to expire. Auto-send is off — open cc-watch to send or dismiss.",
		}
	case EventKeepAliveSuccess:
		return Notification{
			Title:    "cc-watch · KeepAlive",
			Subtitle: subtitle,
			Body:     "Keep-alive sent and confirmed. Cache window extended.",
		}
	case EventKeepAliveFailure:
		body := "Claude account is rate-limited — can't send. Auto-send is off until you re-enable it."
		if !event.RateLimited {
			reason := event.Reason
			if reason == "" {
				reason = "result not confirmed"
			}
			body = "Keep-alive send failed: " + truncateForNotification(reason, 80) + ". Auto-send is off until you re-enable it."
		}
		return Notification{
			Title:    "cc-watch · KeepAlive",
			Subtitle: subtitle,
			Body:     body,
			Sound:    "Basso",
		}
	case EventKeepAliveScopeComplete:
		return Notification{
			Title:    "cc-watch · KeepAlive",
			Subtitle: subtitle,
			Body:     "No more automatic sends left for this session.",
		}
	default:
		return Notification{
			Title: "cc-watch",
			Body:  "A cc-watch event occurred.",
		}
	}
}

func subtitleFor(event Event) string {
	switch {
	case event.ShortID != "" && event.Project != "":
		return event.ShortID + " · " + event.Project
	case event.ShortID != "":
		return event.ShortID
	default:
		return event.Project
	}
}

// truncateForNotification collapses whitespace (so multi-line CLI output
// doesn't garble a single-line banner) and caps length for display.
func truncateForNotification(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

func NewPlatformNotifier(runner Runner) Notifier {
	return NewCommandNotifier(MacOSCommand, runner)
}

func eventKey(event Event) string {
	return fmt.Sprintf("%s:%s:%d:%d:%s", event.Kind, event.SessionID, event.ThresholdPercent, event.CountdownSeconds, event.Reason)
}
