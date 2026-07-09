package notify

import (
	"errors"
	"strings"
	"testing"
)

func TestAppleScriptTitleAndBodyEscaping(t *testing.T) {
	notification := Notification{
		Title: `Reminder "alarm" \ title`,
		Body:  `Cache window crossed "20%" \ Sends no Claude message.`,
	}

	name, args := MacOSCommand(notification)

	if name != "osascript" {
		t.Fatalf("command = %q, want osascript", name)
	}
	script := strings.Join(args, " ")
	for _, unsafe := range []string{`Reminder "alarm"`, `crossed "20%"`} {
		if strings.Contains(script, unsafe) {
			t.Fatalf("AppleScript contains unescaped string %q:\n%s", unsafe, script)
		}
	}
	for _, want := range []string{`Reminder \"alarm\" \\ title`, `crossed \"20%\" \\ Sends no Claude message.`} {
		if !strings.Contains(script, want) {
			t.Fatalf("AppleScript missing escaped text %q:\n%s", want, script)
		}
	}
}

func TestRepeatedIdenticalFailureIsSuppressedUntilDistinctEventOrManualRefresh(t *testing.T) {
	failing := &fakeNotifier{err: errors.New("osascript failed")}
	manager := NewManager(failing)
	event := Event{Kind: EventReminderThresholdCrossed, SessionID: "one", ThresholdPercent: 20}

	first := manager.Notify(event)
	second := manager.Notify(event)
	if first.Suppressed {
		t.Fatalf("first failure was suppressed: %#v", first)
	}
	if !second.Suppressed {
		t.Fatalf("second identical failure was not suppressed: %#v", second)
	}
	if len(manager.Attempts()) != 1 {
		t.Fatalf("attempts = %d, want 1 after suppressed duplicate", len(manager.Attempts()))
	}
	if failing.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1 after suppressed duplicate", failing.calls)
	}

	distinct := manager.Notify(Event{Kind: EventKeepAliveSuccess, SessionID: "one"})
	if distinct.Suppressed {
		t.Fatalf("distinct event failure was suppressed: %#v", distinct)
	}
	if len(manager.Attempts()) != 2 {
		t.Fatalf("attempts = %d, want 2 after distinct event", len(manager.Attempts()))
	}
	if failing.calls != 2 {
		t.Fatalf("notifier calls = %d, want 2 after distinct event", failing.calls)
	}

	manager.ResetSuppression()
	afterReset := manager.Notify(Event{Kind: EventKeepAliveSuccess, SessionID: "one"})
	if afterReset.Suppressed {
		t.Fatalf("failure after manual reset was suppressed: %#v", afterReset)
	}
	if failing.calls != 3 {
		t.Fatalf("notifier calls = %d, want 3 after manual reset", failing.calls)
	}
}

func TestSuccessfulDistinctEventResetsFailureSuppression(t *testing.T) {
	notifier := &sequenceNotifier{results: []Result{
		{Degraded: true, Message: "osascript failed", Err: errors.New("osascript failed")},
		{Delivered: true, Message: "delivered"},
		{Degraded: true, Message: "osascript failed", Err: errors.New("osascript failed")},
	}}
	manager := NewManager(notifier)
	failedEvent := Event{Kind: EventReminderThresholdCrossed, SessionID: "one", ThresholdPercent: 20}
	distinctEvent := Event{Kind: EventKeepAliveSuccess, SessionID: "one"}

	first := manager.Notify(failedEvent)
	second := manager.Notify(distinctEvent)
	third := manager.Notify(failedEvent)

	if first.Suppressed || second.Suppressed || third.Suppressed {
		t.Fatalf("unexpected suppression sequence: first=%#v second=%#v third=%#v", first, second, third)
	}
	if notifier.calls != 3 {
		t.Fatalf("notifier calls = %d, want 3", notifier.calls)
	}
}

func TestEventWordingSeparatesReminderAlarmFromKeepAliveAutomation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		event Event
		want  []string
	}{
		{
			name:  "reminder alarm",
			event: Event{Kind: EventReminderThresholdCrossed, ThresholdPercent: 20, ShortID: "abc12345", Project: "demo"},
			want:  []string{"cc-watch · Reminder", "abc12345 · demo", "20%", "No message sent"},
		},
		{
			name:  "success",
			event: Event{Kind: EventKeepAliveSuccess, ShortID: "abc12345", Project: "demo"},
			want:  []string{"cc-watch · KeepAlive", "abc12345 · demo", "sent and confirmed"},
		},
		{
			name:  "failure generic",
			event: Event{Kind: EventKeepAliveFailure, Reason: "confirmation timed out", ShortID: "abc12345", Project: "demo"},
			want:  []string{"cc-watch · KeepAlive", "abc12345 · demo", "send failed", "confirmation timed out", "Automatic sends paused"},
		},
		{
			name:  "failure rate limited",
			event: Event{Kind: EventKeepAliveFailure, Reason: "rate limit exceeded", RateLimited: true, ShortID: "abc12345", Project: "demo"},
			want:  []string{"cc-watch · KeepAlive", "abc12345 · demo", "rate-limited", "automatic sends paused"},
		},
		{
			name:  "scope complete",
			event: Event{Kind: EventKeepAliveScopeComplete, ShortID: "abc12345", Project: "demo"},
			want:  []string{"cc-watch · KeepAlive", "abc12345 · demo", "No more automatic sends"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notification := FormatEvent(tc.event)
			combined := notification.Title + "\n" + notification.Subtitle + "\n" + notification.Body
			for _, want := range tc.want {
				if !strings.Contains(combined, want) {
					t.Fatalf("wording missing %q:\n%s", want, combined)
				}
			}
		})
	}
}

func TestFailureNotificationPlaysAlertSound(t *testing.T) {
	notification := FormatEvent(Event{Kind: EventKeepAliveFailure, Reason: "boom"})
	if notification.Sound != "Basso" {
		t.Fatalf("failure sound = %q, want Basso", notification.Sound)
	}
	other := FormatEvent(Event{Kind: EventKeepAliveSuccess})
	if other.Sound != "" {
		t.Fatalf("success sound = %q, want silent", other.Sound)
	}
}

func TestTruncateForNotificationCollapsesWhitespaceAndCaps(t *testing.T) {
	got := truncateForNotification("line one\nline two\nline three", 13)
	if got != "line one line…" {
		t.Fatalf("truncateForNotification = %q", got)
	}
	short := truncateForNotification("short", 80)
	if short != "short" {
		t.Fatalf("truncateForNotification short = %q, want unchanged", short)
	}
}

type fakeNotifier struct {
	err   error
	calls int
}

type sequenceNotifier struct {
	results []Result
	calls   int
}

func (n *sequenceNotifier) Notify(Event) Result {
	if n.calls >= len(n.results) {
		n.calls++
		return Result{Delivered: true, Message: "delivered"}
	}
	result := n.results[n.calls]
	n.calls++
	return result
}

func (n *fakeNotifier) Notify(Event) Result {
	n.calls++
	if n.err != nil {
		return Result{Degraded: true, Message: n.err.Error(), Err: n.err}
	}
	return Result{Delivered: true, Message: "delivered"}
}
