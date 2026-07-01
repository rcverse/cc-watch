package tui

import (
	"testing"
	"time"

	"github.com/richardchen/cc-watch/internal/session"
)

func TestReminderRuntimeFiresEnabledSessionsOnly(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	enabled := reminderRuntimeSession("enabled", now.Add(-55*time.Minute), 3600)
	disabled := reminderRuntimeSession("disabled", now.Add(-55*time.Minute), 3600)
	runtime := newReminderRuntime([]int{20, 10}, map[string]bool{"enabled": true}, map[string]map[int]bool{})

	events := runtime.check(now, []session.Session{enabled, disabled})
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want two enabled-session thresholds: %#v", len(events), events)
	}
	for _, event := range events {
		if event.sessionID != "enabled" {
			t.Fatalf("event session = %q, want enabled", event.sessionID)
		}
	}
}

func TestReminderRuntimeFiresOncePerThreshold(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	s := reminderRuntimeSession("session-a", now.Add(-55*time.Minute), 3600)
	runtime := newReminderRuntime([]int{20, 10}, map[string]bool{"session-a": true}, map[string]map[int]bool{})

	first := runtime.check(now, []session.Session{s})
	if len(first) != 2 {
		t.Fatalf("first len = %d, want two thresholds: %#v", len(first), first)
	}
	second := runtime.check(now.Add(30*time.Second), []session.Session{s})
	if len(second) != 0 {
		t.Fatalf("second check fired duplicate events: %#v", second)
	}

	nextRuntime := newReminderRuntime([]int{20, 10}, map[string]bool{"session-a": true}, map[string]map[int]bool{})
	again := nextRuntime.check(now, []session.Session{s})
	if len(again) != 2 {
		t.Fatalf("new runtime len = %d, want thresholds to fire again", len(again))
	}
}

func reminderRuntimeSession(id string, lastMessageAt time.Time, ttlSeconds int) session.Session {
	return session.Session{
		SessionID:     id,
		ShortID:       id,
		Project:       "tmp",
		LastMessageAt: &lastMessageAt,
		CacheWindow: session.CacheWindow{
			Tier:       session.Tier1Hour,
			Label:      "1h",
			TTLSeconds: ttlSeconds,
			Known:      true,
		},
	}
}
