package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
)

func TestSemanticStylesExposeRequiredRoles(t *testing.T) {
	styles := DefaultStyles()
	required := []StyleRole{
		RoleNeutral,
		RoleMuted,
		RoleSelectedFocus,
		RoleInfo,
		RoleWarning,
		RoleDanger,
		RoleSuccess,
		RoleDisabled,
		RoleDegraded,
	}

	for _, role := range required {
		if !styles.Has(role) {
			t.Fatalf("style role %q missing", role)
		}
	}
}

func TestStateBadgesIncludeTextNotOnlyColor(t *testing.T) {
	styles := DefaultStyles()
	for _, tc := range []struct {
		role  StyleRole
		label string
	}{
		{RoleInfo, "active"},
		{RoleWarning, "countdown"},
		{RoleDanger, "failed"},
		{RoleSuccess, "done"},
		{RoleDisabled, "disabled"},
		{RoleDegraded, "watcher degraded"},
	} {
		rendered := styles.Badge(tc.role, tc.label)
		if !strings.Contains(rendered, tc.label) {
			t.Fatalf("badge %q for %q missing text label: %q", tc.role, tc.label, rendered)
		}
	}
}

func TestViewIncludesRouteAndFocusLabels(t *testing.T) {
	model := NewModel(Options{})
	view := model.View()

	for _, want := range []string{"cc-cache list", "focus: session"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestViewIncludesWatcherRefreshAndEmptyStateBanners(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options Options
		want    []string
	}{
		{
			name: "watcher ok",
			options: Options{
				Refresh: RefreshViewState{Watcher: refresh.State{Status: refresh.StatusOK, SafetyRefreshActive: true}},
			},
			want: []string{"Watcher: ok"},
		},
		{
			name: "partial watcher degraded",
			options: Options{
				Refresh: RefreshViewState{Watcher: refresh.State{Status: refresh.StatusPartial, Messages: []string{"subdir permission denied"}, SafetyRefreshActive: true}},
			},
			want: []string{"Watcher: partial", "subdir permission denied", "Safety refresh: active"},
		},
		{
			name: "post start watcher failure",
			options: Options{
				Refresh: RefreshViewState{Watcher: refresh.State{Status: refresh.StatusDegraded, Messages: []string{"watcher closed"}, SafetyRefreshActive: true}},
			},
			want: []string{"Watcher: degraded", "watcher closed", "Safety refresh: active"},
		},
		{
			name: "no projects directory",
			options: Options{
				Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyProjectsDir},
			},
			want: []string{"No projects directory", "/tmp/home/.claude/projects"},
		},
		{
			name: "no sessions found",
			options: Options{
				Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
			},
			want: []string{"No sessions found", "sessions appear after Claude Code writes JSONL files"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := NewModel(tc.options).View()
			for _, want := range tc.want {
				if !strings.Contains(view, want) {
					t.Fatalf("view missing %q:\n%s", want, view)
				}
			}
		})
	}
}

func TestListViewSortsRowsByFileModificationTime(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:      now,
		Width:    120,
		Sessions: listViewSessions(now),
	})

	view := model.View()
	first := strings.Index(view, "newer-id")
	second := strings.Index(view, "middle-id")
	third := strings.Index(view, "older-id")
	if first == -1 || second == -1 || third == -1 {
		t.Fatalf("view missing sorted session ids:\n%s", view)
	}
	if !(first < second && second < third) {
		t.Fatalf("sessions are not sorted by file modification time:\n%s", view)
	}
}

func TestListViewResponsivePriorityFields(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	longExcerpt := "this is a deliberately long excerpt that should truncate before action critical columns disappear"

	narrow := NewModel(Options{
		Now:   now,
		Width: 72,
		Sessions: []session.Session{listViewSession("narrow-id", "ops", now, now.Add(-4*time.Minute), session.CacheWindow{
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
		}, longExcerpt, "last message")},
	}).View()
	for _, want := range []string{"narrow-id", "ops", "1h", "active", "56m00s", "enter open", "r remind", "k keepalive"} {
		if !strings.Contains(narrow, want) {
			t.Fatalf("narrow view missing %q:\n%s", want, narrow)
		}
	}
	if strings.Contains(narrow, longExcerpt) {
		t.Fatalf("narrow view did not truncate long excerpt:\n%s", narrow)
	}

	medium := NewModel(Options{
		Now:   now,
		Width: 100,
		Sessions: []session.Session{listViewSession("medium-id", "workspace", now, now.Add(-30*time.Minute), session.CacheWindow{
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
		}, "first user message", "last user message")},
	}).View()
	for _, want := range []string{"medium-id", "workspace", "TTL 50%", "hit 75%", "last: \"last user message\""} {
		if !strings.Contains(medium, want) {
			t.Fatalf("medium view missing %q:\n%s", want, medium)
		}
	}
	if strings.Contains(medium, "first:") {
		t.Fatalf("medium view included first excerpt, want one excerpt only:\n%s", medium)
	}

	duration := 7200
	wideSession := listViewSession("wide-id", "wide-project", now, now.Add(-2*time.Minute), session.CacheWindow{
		Label:      "5m",
		TTLSeconds: 300,
		Known:      true,
	}, "first wide message", "last wide message")
	wideSession.DurationSeconds = &duration
	wideSession.Warnings = []session.ParseWarning{{Message: "bad timestamp"}}
	wide := NewModel(Options{
		Now:              now,
		Width:            140,
		Sessions:         []session.Session{wideSession},
		KeepAliveEnabled: map[string]bool{"wide-id": true},
	}).View()
	for _, want := range []string{"TTL 40%", "first: \"first wide message\"", "last: \"last wide message\"", "duration 2h00m00s", "warnings: 1", "KeepAlive on"} {
		if !strings.Contains(wide, want) {
			t.Fatalf("wide view missing %q:\n%s", want, wide)
		}
	}
}

func TestListEmptyStatesOnlyAdvertiseValidActions(t *testing.T) {
	view := NewModel(Options{
		Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
	}).View()

	for _, disallowed := range []string{"enter open", "r remind", "k keepalive"} {
		if strings.Contains(view, disallowed) {
			t.Fatalf("empty state advertised invalid action %q:\n%s", disallowed, view)
		}
	}
	for _, want := range []string{"refresh", "? help", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty state missing valid action %q:\n%s", want, view)
		}
	}
}

func TestListViewRequiredStates(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		options Options
		want    []string
	}{
		{
			name:    "loading",
			options: Options{Refresh: RefreshViewState{EmptyState: EmptyLoading}},
			want:    []string{"Loading sessions"},
		},
		{
			name: "missing projects directory",
			options: Options{
				Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyProjectsDir},
			},
			want: []string{"No ~/.claude/projects directory exists yet", "/tmp/home/.claude/projects", "cannot discover sessions"},
		},
		{
			name: "no sessions",
			options: Options{
				Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
			},
			want: []string{"No Claude Code session files found", "/tmp/home/.claude/projects", "Sessions appear here after Claude Code writes JSONL files"},
		},
		{
			name: "ambiguous partial id",
			options: Options{
				Now:         now,
				Width:       120,
				AmbiguousID: "d4b",
				Sessions: []session.Session{
					listViewSession("d4b247b7", "workspace-api", now, now.Add(-1*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", ""),
					listViewSession("d4b901aa", "docs-review", now.Add(-time.Hour), now.Add(-2*time.Hour), session.CacheWindow{Label: "5m", TTLSeconds: 300, Known: true}, "", ""),
				},
			},
			want: []string{"ambiguous session id: d4b", "matched more than one session", "d4b247b7", "d4b901aa"},
		},
		{
			name: "degraded watcher notification and claude",
			options: Options{
				Now:   now,
				Width: 120,
				Refresh: RefreshViewState{
					Watcher:                  refresh.State{Status: refresh.StatusPartial, Messages: []string{"permission denied"}, SafetyRefreshActive: true},
					NotificationDegraded:     "notify-send failed",
					ClaudeUnavailableMessage: "claude not found",
				},
				Sessions:         []session.Session{listViewSession("armed-id", "armed", now, now.Add(-1*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "")},
				KeepAliveEnabled: map[string]bool{"armed-id": true},
			},
			want: []string{"Watcher: partial", "permission denied", "Safety refresh: active", "Notify degraded: notify-send failed", "claude unavailable: claude not found"},
		},
		{
			name: "parse warnings",
			options: Options{
				Now:   now,
				Width: 120,
				Sessions: []session.Session{func() session.Session {
					s := listViewSession("warn-id", "parser", now, now.Add(-1*time.Minute), session.CacheWindow{Label: "TTL ?", Known: false}, "", "")
					s.Warnings = []session.ParseWarning{{Message: "bad json"}, {Message: "bad timestamp"}}
					return s
				}()},
			},
			want: []string{"warn-id", "warnings: 2"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := NewModel(tc.options).View()
			for _, want := range tc.want {
				if !strings.Contains(view, want) {
					t.Fatalf("view missing %q:\n%s", want, view)
				}
			}
		})
	}
}

func TestNotificationDeliveryFailureIsVisibleAsDegradedStateAndStatus(t *testing.T) {
	model := NewModel(Options{})
	updated, _ := model.Update(NotificationResultMsg{
		Event: notify.Event{Kind: notify.EventReminderThresholdCrossed, ThresholdPercent: 20},
		Result: notify.Result{
			Degraded: true,
			Message:  "notify-send failed",
		},
	})
	model = updated.(Model)

	view := model.View()
	for _, want := range []string{
		"Notify degraded: notify-send failed",
		"Notification delivery failed: notify-send failed",
		"Event happened: Reminder alarm",
		"Sends no Claude message",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestNotificationDeliverySuccessRecordsStatusWithoutDegradedBanner(t *testing.T) {
	model := NewModel(Options{})
	updated, _ := model.Update(NotificationResultMsg{
		Event: notify.Event{Kind: notify.EventKeepAliveCountdownStarted, CountdownSeconds: 30},
		Result: notify.Result{
			Delivered: true,
			Message:   "delivered",
		},
	})
	model = updated.(Model)

	view := model.View()
	if strings.Contains(view, "Notify degraded") {
		t.Fatalf("success view contains degraded banner:\n%s", view)
	}
	for _, want := range []string{"Notification delivered: KeepAlive countdown", "may be sent after 30s unless canceled"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func listViewSessions(now time.Time) []session.Session {
	return []session.Session{
		listViewSession("older-id", "old", now.Add(-2*time.Hour), now.Add(-2*time.Hour), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "old first", "old last"),
		listViewSession("newer-id", "new", now, now.Add(-5*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "new first", "new last"),
		listViewSession("middle-id", "mid", now.Add(-time.Hour), now.Add(-10*time.Minute), session.CacheWindow{Label: "5m", TTLSeconds: 300, Known: true}, "mid first", "mid last"),
	}
}

func listViewSession(id string, project string, modified time.Time, last time.Time, cache session.CacheWindow, first string, final string) session.Session {
	return session.Session{
		SessionID:      id,
		ShortID:        id,
		Project:        project,
		FileModifiedAt: modified,
		LastMessageAt:  &last,
		CacheWindow:    cache,
		Messages: session.Messages{
			FirstUserExcerpt: first,
			LastUserExcerpt:  final,
		},
		TokenStats: session.TokenStats{
			CacheWrites: 3,
			CacheReads:  9,
			HitRate:     75,
		},
	}
}
