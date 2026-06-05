package reminder

import (
	"testing"
	"time"

	"github.com/richardchen/cc-cache/internal/config"
	"github.com/richardchen/cc-cache/internal/session"
)

func TestRuntimeToggleControlsPerSessionReminder(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	s := activeSession("session-a", now.Add(-55*time.Minute), 3600)
	manager := New(config.Default())

	if manager.Enabled(s.SessionID) {
		t.Fatal("new manager has reminder enabled, want disabled")
	}
	if events := manager.Check(now, []session.Session{s}); len(events) != 0 {
		t.Fatalf("Check disabled session emitted events: %#v", events)
	}

	manager.Enable(s.SessionID)
	if !manager.Enabled(s.SessionID) {
		t.Fatal("Enable did not mark session enabled")
	}
	if events := manager.Check(now, []session.Session{s}); len(events) == 0 {
		t.Fatal("Check enabled session emitted no events")
	}

	manager.Disable(s.SessionID)
	if manager.Enabled(s.SessionID) {
		t.Fatal("Disable left session enabled")
	}
}

func TestThresholdsComeFromGlobalConfig(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	s := activeSession("session-a", now.Add(-45*time.Minute), 3600)
	cfg := config.Default()
	cfg.ReminderThresholds = []int{30, 25}
	manager := New(cfg)
	manager.Enable(s.SessionID)

	events := manager.Check(now, []session.Session{s})
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2: %#v", len(events), events)
	}
	if events[0].ThresholdPercent != 30 || events[1].ThresholdPercent != 25 {
		t.Fatalf("thresholds = %d,%d; want 30,25", events[0].ThresholdPercent, events[1].ThresholdPercent)
	}
}

func TestFiresOncePerThresholdCrossingPerActiveSessionInstance(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	s := activeSession("session-a", now.Add(-55*time.Minute), 3600)
	manager := New(config.Default())
	manager.Enable(s.SessionID)

	first := manager.Check(now, []session.Session{s})
	if len(first) != 2 {
		t.Fatalf("first len = %d, want 2: %#v", len(first), first)
	}
	second := manager.Check(now.Add(30*time.Second), []session.Session{s})
	if len(second) != 0 {
		t.Fatalf("second check fired duplicate events: %#v", second)
	}

	nextProcess := New(config.Default())
	nextProcess.Enable(s.SessionID)
	again := nextProcess.Check(now, []session.Session{s})
	if len(again) != 2 {
		t.Fatalf("new runtime instance len = %d, want thresholds to fire again", len(again))
	}
}

func TestRemindEnablesLoadedSessionsOnly(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	loaded := activeSession("loaded", now.Add(-55*time.Minute), 3600)
	later := activeSession("later", now.Add(-55*time.Minute), 3600)
	manager := New(config.Default())

	manager.EnableLoadedSessions([]session.Session{loaded})

	if !manager.Enabled("loaded") {
		t.Fatal("loaded session was not enabled")
	}
	if manager.Enabled("later") {
		t.Fatal("later session enabled before it was loaded")
	}
	events := manager.Check(now, []session.Session{loaded, later})
	if len(events) != 2 {
		t.Fatalf("events = %#v, want only loaded session thresholds", events)
	}
	for _, event := range events {
		if event.SessionID != "loaded" {
			t.Fatalf("event session = %q, want loaded only", event.SessionID)
		}
	}
}

func TestReminderNeverInvokesClaudeOrKeepAliveBehavior(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	s := activeSession("session-a", now.Add(-55*time.Minute), 3600)
	manager := New(config.Default())
	manager.Enable(s.SessionID)

	events := manager.Check(now, []session.Session{s})
	if len(events) == 0 {
		t.Fatal("Check emitted no reminder events")
	}
	for _, event := range events {
		if event.Kind != "reminder_threshold_crossed" {
			t.Fatalf("event kind = %q, want reminder only", event.Kind)
		}
		if event.ClaudeSubprocessStarted {
			t.Fatalf("reminder event unexpectedly started Claude subprocess: %#v", event)
		}
	}
}

func activeSession(id string, lastMessageAt time.Time, ttlSeconds int) session.Session {
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
