package jsonout

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/reminder"
	"github.com/richardchen/cc-cache/internal/session"
)

func TestSuccessOutputUsesStableTopLevelContract(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	data, err := Marshal(State{
		GeneratedAt: now,
		Query:       Query{Limit: 5},
		Sessions:    []session.Session{},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var doc map[string]any
	decodeJSON(t, data, &doc)
	if doc["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %#v, want 1", doc["schema_version"])
	}
	if doc["generated_at"] != "2026-06-03T12:00:00Z" {
		t.Fatalf("generated_at = %#v", doc["generated_at"])
	}
	if doc["error"] != nil {
		t.Fatalf("error = %#v, want null", doc["error"])
	}
	for _, key := range []string{"query", "refresh", "notifications", "sessions", "selected_session"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("top-level key %q missing from %#v", key, doc)
		}
	}
	if bytes.Contains(data, []byte("\x1b[")) {
		t.Fatalf("JSON contains ANSI escape codes: %q", data)
	}
}

func TestSessionObjectShapeIncludesParserReminderAndKeepAliveState(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	last := now.Add(-5 * time.Minute)
	start := now.Add(-20 * time.Minute)
	duration := 900
	s := session.Session{
		SessionID:       "11111111-1111-1111-1111-111111111111",
		ShortID:         "11111111",
		Project:         "tmp-cc-cache",
		JSONLPath:       "/tmp/home/.claude/projects/-tmp-cc-cache/11111111-1111-1111-1111-111111111111.jsonl",
		FileModifiedAt:  now,
		StartedAt:       &start,
		EndedAt:         &last,
		DurationSeconds: &duration,
		LastMessageAt:   &last,
		CacheWindow: session.CacheWindow{
			Tier:       session.Tier1Hour,
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
			Evidence:   []string{"ephemeral_1h_input_tokens"},
		},
		Messages: session.Messages{
			FirstUserExcerpt: "start",
			LastUserExcerpt:  "continue",
		},
		TokenStats: session.TokenStats{
			CacheWrites:  100,
			CacheReads:   900,
			OutputTokens: 50,
			HitRate:      90,
		},
		Gaps: []session.Gap{{
			Seconds: 120,
			From:    now.Add(-30 * time.Minute),
			To:      now.Add(-28 * time.Minute),
			Reset:   false,
		}},
		Warnings: []session.ParseWarning{{
			Code:    session.WarningMalformedJSON,
			Line:    2,
			Message: "ignored malformed line",
		}},
	}

	data, err := Marshal(State{
		GeneratedAt: now,
		Query:       Query{ID: "11111111", Limit: 5},
		Sessions:    []session.Session{s},
		Selected:    &s,
		Reminder: map[string]ReminderState{
			s.SessionID: {
				Available:  true,
				Enabled:    boolPtr(true),
				Thresholds: []int{20, 10},
				Fired: []reminder.Event{{
					Kind:             "reminder_threshold_crossed",
					SessionID:        s.SessionID,
					ThresholdPercent: 20,
					RemainingPercent: 8.33,
					OccurredAt:       now,
				}},
			},
		},
		KeepAlive: map[string]KeepAliveState{
			s.SessionID: {
				Available: true,
				Enabled:   boolPtr(false),
				AutoSend:  boolPtr(true),
				State:     "off",
				Scope:     map[string]any{"mode": "max_sends", "max_sends": 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var doc map[string]any
	decodeJSON(t, data, &doc)
	selected := doc["selected_session"].(map[string]any)
	if selected["session_id"] != s.SessionID {
		t.Fatalf("selected session_id = %#v", selected["session_id"])
	}
	for _, key := range []string{"short_id", "project", "jsonl_path", "file_modified_at", "cache_window", "status", "messages", "token_stats", "gaps", "warnings", "reminder", "keep_alive"} {
		if _, ok := selected[key]; !ok {
			t.Fatalf("session key %q missing from %#v", key, selected)
		}
	}
	if selected["started_at"] != "2026-06-03T11:40:00Z" {
		t.Fatalf("started_at = %#v", selected["started_at"])
	}
	if selected["ended_at"] != "2026-06-03T11:55:00Z" {
		t.Fatalf("ended_at = %#v", selected["ended_at"])
	}
	if selected["duration_seconds"] != float64(900) {
		t.Fatalf("duration_seconds = %#v", selected["duration_seconds"])
	}
	reminderObject := selected["reminder"].(map[string]any)
	if reminderObject["available"] != true || reminderObject["enabled"] != true {
		t.Fatalf("reminder object = %#v", reminderObject)
	}
	keepAlive := selected["keep_alive"].(map[string]any)
	if keepAlive["available"] != true || keepAlive["state"] != "off" {
		t.Fatalf("keep_alive object = %#v", keepAlive)
	}
	warnings := selected["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1", len(warnings))
	}
}

func TestRefreshAndNotificationDegradedStatesAreEncoded(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	data, err := Marshal(State{
		GeneratedAt: now,
		Query:       Query{Limit: 5},
		Refresh: RefreshState{
			Mode:                "snapshot",
			WatcherStatus:       "degraded",
			WatcherDegraded:     true,
			WatcherMessages:     []string{"partial watch failed"},
			SafetyRefreshActive: true,
			LastRefreshAt:       now,
		},
		Notifications: NotificationState{
			Status:   "degraded",
			Degraded: true,
			Recent:   []string{"notify-send unavailable"},
		},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var doc map[string]any
	decodeJSON(t, data, &doc)
	refresh := doc["refresh"].(map[string]any)
	watcher := refresh["watcher"].(map[string]any)
	if watcher["degraded"] != true {
		t.Fatalf("watcher degraded = %#v, want true", watcher["degraded"])
	}
	notifications := doc["notifications"].(map[string]any)
	if notifications["degraded"] != true {
		t.Fatalf("notifications degraded = %#v, want true", notifications["degraded"])
	}
}

func TestRefreshPartialDegradedStateIsEncoded(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	data, err := Marshal(State{
		GeneratedAt: now,
		Refresh: FromRefreshState(refresh.State{
			Status:              refresh.StatusPartial,
			Messages:            []string{"new directory permission denied"},
			SafetyRefreshActive: true,
		}, now),
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var doc map[string]any
	decodeJSON(t, data, &doc)
	refresh := doc["refresh"].(map[string]any)
	watcher := refresh["watcher"].(map[string]any)
	if watcher["status"] != "partial" {
		t.Fatalf("watcher status = %#v, want partial", watcher["status"])
	}
	if watcher["degraded"] != true {
		t.Fatalf("watcher degraded = %#v, want true", watcher["degraded"])
	}
	if refresh["safety_refresh_active"] != true {
		t.Fatalf("safety_refresh_active = %#v, want true", refresh["safety_refresh_active"])
	}
}

func TestAllowedErrorShapesForNoMatchAndAmbiguousID(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	for _, code := range AllowedErrorCodes() {
		if code == "" {
			t.Fatal("AllowedErrorCodes contains empty code")
		}
	}

	noMatch, err := Marshal(State{
		GeneratedAt: now,
		Query:       Query{ID: "zzz", Limit: 5},
		Error: &Error{
			Code:    "session_not_found",
			Message: "partial id did not match any session",
			Query:   "zzz",
		},
	})
	if err != nil {
		t.Fatalf("Marshal no-match returned error: %v", err)
	}
	var noMatchDoc map[string]any
	decodeJSON(t, noMatch, &noMatchDoc)
	if noMatchDoc["error"].(map[string]any)["code"] != "session_not_found" {
		t.Fatalf("no-match error = %#v", noMatchDoc["error"])
	}

	candidate := session.Session{
		SessionID: "11111111-1111-1111-1111-111111111111",
		ShortID:   "11111111",
		Project:   "tmp-cc-cache",
	}
	ambiguous, err := Marshal(State{
		GeneratedAt: now,
		Query:       Query{ID: "111", Limit: 5},
		Sessions:    []session.Session{candidate},
		Error: &Error{
			Code:    "ambiguous_session_id",
			Message: "partial id matched multiple sessions",
			Query:   "111",
		},
	})
	if err != nil {
		t.Fatalf("Marshal ambiguous returned error: %v", err)
	}
	var ambiguousDoc map[string]any
	decodeJSON(t, ambiguous, &ambiguousDoc)
	sessions := ambiguousDoc["sessions"].([]any)
	if len(sessions) != 1 {
		t.Fatalf("candidate sessions len = %d, want 1", len(sessions))
	}
	candidateDoc := sessions[0].(map[string]any)
	if _, ok := candidateDoc["cache_window"]; ok {
		t.Fatalf("candidate includes full session fields, want safe summary only: %#v", candidateDoc)
	}
	for _, key := range []string{"session_id", "short_id", "project"} {
		if _, ok := candidateDoc[key]; !ok {
			t.Fatalf("candidate key %q missing from %#v", key, candidateDoc)
		}
	}
	if ambiguousDoc["error"].(map[string]any)["code"] != "ambiguous_session_id" {
		t.Fatalf("ambiguous error = %#v", ambiguousDoc["error"])
	}
}

func TestConfigWarningsAreVisible(t *testing.T) {
	data, err := Marshal(State{
		GeneratedAt:    time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
		ConfigWarnings: []string{"invalid config JSON; using defaults"},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var doc map[string]any
	decodeJSON(t, data, &doc)
	configDoc := doc["config"].(map[string]any)
	warnings := configDoc["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("config warnings = %#v, want one warning", warnings)
	}
}

func TestRejectsUnsupportedErrorCode(t *testing.T) {
	_, err := Marshal(State{
		GeneratedAt: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
		Error: &Error{
			Code:    "unsupported",
			Message: "nope",
		},
	})
	if err == nil {
		t.Fatal("Marshal returned nil error for unsupported error code")
	}
	if !strings.Contains(err.Error(), "unsupported error code") {
		t.Fatalf("error = %v, want unsupported error code", err)
	}
}

func decodeJSON(t *testing.T, data []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("Decode JSON failed: %v\n%s", err, data)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
