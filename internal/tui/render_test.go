package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/notify"
	"github.com/richardchen/cc-watch/internal/refresh"
	"github.com/richardchen/cc-watch/internal/session"
)

func TestSemanticStylesExposeRequiredRoles(t *testing.T) {
	styles := DefaultStyles()
	required := []StyleRole{
		RoleNeutral,
		RoleMuted,
		RoleExcerptLabel,
		RoleReminder,
		RoleKeepAlive,
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

func TestSemanticStylesUseMutedCoherentTerminalPalette(t *testing.T) {
	styles := DefaultStyles()
	for _, role := range []StyleRole{RoleIdentity, RoleInfo, RoleWarning, RoleDanger, RoleSuccess, RoleReminder, RoleKeepAlive, RoleFirstLabel, RoleLastLabel} {
		rendered := styles.Render(role, string(role))
		if strings.Contains(rendered, ";1m") || strings.Contains(rendered, "[1;") {
			t.Fatalf("role %q uses bold/saturated emphasis: %q", role, rendered)
		}
	}
	if got := fmt.Sprint(styles.roles[RoleFirstLabel].GetForeground()); got != "187" {
		t.Fatalf("first label foreground = %q, want light khaki 187", got)
	}
	if got := fmt.Sprint(styles.roles[RoleLastLabel].GetForeground()); got != "109" {
		t.Fatalf("last label foreground = %q, want cool muted blue-gray 109", got)
	}
	if got := fmt.Sprint(styles.roles[RoleIdentity].GetForeground()); got != "111" {
		t.Fatalf("identity/session accent foreground = %q, want saturated readable blue 111", got)
	}
	if got := fmt.Sprint(styles.roles[RoleSelectedFocus].GetForeground()); got != "111" {
		t.Fatalf("selected focus foreground = %q, want saturated readable blue 111", got)
	}
	if got := fmt.Sprint(styles.roles[RoleKeepAlive].GetForeground()); got != "147" {
		t.Fatalf("KeepAlive foreground = %q, want muted purple 147", got)
	}
	if got := fmt.Sprint(styles.roles[RoleWarning].GetForeground()); got == "214" {
		t.Fatalf("warning/degraded roles still use saturated orange tones")
	}
	if got := fmt.Sprint(styles.roles[RoleDegraded].GetForeground()); got == "202" {
		t.Fatalf("degraded role still uses saturated orange tone")
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

func TestVisualPrimitivesRenderPanelsAndProgressBars(t *testing.T) {
	panel := RenderPanel("Status", "Cache window: 1h\nTTL elapsed")
	for _, want := range []string{"╭", "╮", "╰", "╯", "Status", "Cache window: 1h"} {
		if !strings.Contains(panel, want) {
			t.Fatalf("panel missing %q:\n%s", want, panel)
		}
	}
	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	for _, line := range lines[1:] {
		if visibleWidth(stripANSI(line)) != visibleWidth(stripANSI(lines[0])) {
			t.Fatalf("panel frame width mismatch:\n%s", panel)
		}
	}

	bar := ProgressBar(67, 12)
	for _, want := range []string{"█", "░"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("progress bar missing %q: %q", want, bar)
		}
	}
	if visibleWidth(stripANSI(bar)) != 12 {
		t.Fatalf("progress bar visible width = %d, want 12: %q", visibleWidth(stripANSI(bar)), bar)
	}

}

func TestViewIncludesRouteAndFocusLabels(t *testing.T) {
	model := NewModel(Options{})
	view := model.View()

	for _, want := range []string{"Claude Code Watch"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"Sessions  ·", "  ·  live", "updated"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("view contains removed header metadata %q:\n%s", notWant, view)
		}
	}
}

func TestListViewUsesPolishedVisualHierarchy(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	last := now.Add(-30 * time.Minute)
	model := NewModel(Options{
		Now: now,
		Sessions: []session.Session{{
			SessionID:      "11111111-1111-1111-1111-111111111111",
			ShortID:        "11111111",
			Project:        "visual-project",
			FileModifiedAt: now,
			LastMessageAt:  &last,
			CacheWindow:    session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true},
			TokenStats:     session.TokenStats{HitRate: 88},
			Messages:       session.Messages{LastUserExcerpt: "last visual message"},
		}},
	})
	view := model.View()

	for _, want := range []string{"Claude Code Watch", "›", "visual-project", "● Active", "hit 88%", "last  last visual message"} {
		if !strings.Contains(view, want) {
			t.Fatalf("list view missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"╭─ System", "╭─ Sessions", "KA "} {
		if strings.Contains(view, notWant) {
			t.Fatalf("list view contains deprecated visual grammar %q:\n%s", notWant, view)
		}
	}
}

func TestAdaptiveListViewFitsEightyColumnsAndCapsExpiredProgress(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 23, 0, 0, time.UTC)
	last := now.Add(-9*time.Hour - 2*time.Minute)
	model := NewModel(Options{
		Now:    now,
		Width:  80,
		Height: 24,
		Sessions: []session.Session{{
			SessionID:      "1463d331-4237-4000-9000-aaaaaaaaaaaa",
			ShortID:        "1463d331",
			Project:        "Users-richardchen-workspace",
			FileModifiedAt: now,
			LastMessageAt:  &last,
			CacheWindow:    session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true},
			TokenStats:     session.TokenStats{HitRate: 95},
			Messages: session.Messages{
				FirstUserExcerpt: "<local-command-caveat>Caveat: The messages below were generated by Claude Code",
				LastUserExcerpt:  "Base directory for this skill: /Users/richardchen/.claude/plugins/cache/anthropic-agent-skills/examples",
			},
		}},
	})

	view := model.View()
	assertMaxLineWidth(t, view, 80)
	for _, want := range []string{"Claude Code Watch", "› #1", "1-hour cache", "× Expired   9h02m ago", "100%", "KeepAlive N/A"} {
		if !strings.Contains(view, want) {
			t.Fatalf("adaptive list missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"903%", "TTL 903", "╭─ System", "╭─ Sessions", "KA "} {
		if strings.Contains(view, notWant) {
			t.Fatalf("adaptive list contains forbidden copy %q:\n%s", notWant, view)
		}
	}
}

func TestListViewStaysReadableOnWideTerminals(t *testing.T) {
	now := time.Date(2026, 6, 30, 23, 38, 0, 0, time.UTC)
	view := NewModel(Options{
		Now:   now,
		Width: 160,
		Sessions: []session.Session{listViewSession(
			"ec29223a",
			"Users-richardchen-Documents-pkm-system-workspace",
			now,
			now.Add(-time.Minute),
			session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true},
			"<local-command-caveat>Caveat: The messages below were generated by the user while running local commands",
			"looks good - all recommendations taken",
		)},
	}).View()

	assertMaxLineWidth(t, view, 80)
	if strings.Contains(view, "duration ") || strings.Contains(view, "warnings ") {
		t.Fatalf("wide-only list details leaked into capped list view:\n%s", view)
	}
}

func TestExpiredListRowsSpaceStatusTimeAndDisableAutomationChips(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	last := now.Add(-2 * time.Hour)
	expired := listViewSession("expired-id", "expired-project", now, last, session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "expired")
	view := NewModel(Options{
		Now:             now,
		Width:           120,
		Sessions:        []session.Session{expired},
		ReminderEnabled: map[string]bool{expired.SessionID: true},
		KeepAliveEnabled: map[string]bool{
			expired.SessionID: true,
		},
	}).View()
	plain := stripANSI(view)

	for _, want := range []string{"× Expired   2h00m ago", "KeepAlive N/A"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expired list row missing %q:\n%s", want, view)
		}
	}
	styles := DefaultStyles()
	for _, want := range []string{styles.Render(RoleDisabled, "KeepAlive N/A")} {
		if !strings.Contains(view, want) {
			t.Fatalf("expired automation chip is not fully disabled %q:\n%s", want, view)
		}
	}
}

func TestStatusTimesDropLowValueUnitsAsTheyGrow(t *testing.T) {
	for _, tc := range []struct {
		seconds int
		want    string
	}{
		{59*60 + 7, "59m07s"},
		{2*3600 + 4*60 + 9, "2h04m"},
		{26*3600 + 12*60 + 30, "1d2h"},
	} {
		if got := formatStatusDuration(tc.seconds); got != tc.want {
			t.Fatalf("formatStatusDuration(%d) = %q, want %q", tc.seconds, got, tc.want)
		}
	}
}

func TestExpiredRowsWithTimestampUseStatusDurationScale(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	last := now.Add(-(3*24*time.Hour + 12*time.Hour + 33*time.Minute))
	view := NewModel(Options{
		Now:   now,
		Width: 140,
		Sessions: []session.Session{listViewSession(
			"expired-long",
			"project",
			now,
			last,
			session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true},
			"",
			"last",
		)},
	}).View()
	plain := stripANSI(view)

	if !strings.Contains(plain, "× Expired   3d12h ago") {
		t.Fatalf("long expired timestamp did not use day/hour scale:\n%s", view)
	}
	if strings.Contains(plain, "84h") || strings.Contains(plain, "33m") {
		t.Fatalf("long expired timestamp leaked hour-only/minute detail:\n%s", view)
	}
}

func TestListStatusLabelAlignsWithMessageLabels(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	view := NewModel(Options{
		Now:   now,
		Width: 120,
		Sessions: []session.Session{listViewSession(
			"align-id",
			"align-project",
			now,
			now.Add(-15*time.Minute),
			session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true},
			"first excerpt",
			"last excerpt",
		)},
	}).View()
	plain := stripANSI(view)

	rowColumn := lineIndexContaining(plain, "› #1")
	statusColumn := lineIndexContaining(plain, "● Active")
	firstColumn := lineIndexContaining(plain, "first")
	lastColumn := lineIndexContaining(plain, "last")
	if rowColumn != 0 {
		t.Fatalf("selected row marker column = %d, want flush-left:\n%s", rowColumn, view)
	}
	if statusColumn != firstColumn || statusColumn != lastColumn {
		t.Fatalf("status/first/last columns = %d/%d/%d, want aligned:\n%s", statusColumn, firstColumn, lastColumn, view)
	}
	if statusColumn != 2 {
		t.Fatalf("status/first/last column = %d, want tight two-column gutter:\n%s", statusColumn, view)
	}
}

func TestListRowsSnapIdentityAndStatusColumns(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	view := NewModel(Options{
		Now:   now,
		Width: 160,
		Sessions: []session.Session{
			listViewSession("short", "a", now, now.Add(-5*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "one"),
			listViewSession("longer-id", "Users-richardchen-course-custom-skills", now, now.Add(-84*time.Hour), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "two"),
		},
	}).View()
	plain := stripANSI(view)
	lines := strings.Split(plain, "\n")
	row1 := lineContaining(lines, "#1")
	row2 := lineContaining(lines, "#2")
	status1 := lineContaining(lines, "● Active")
	status2 := lineContaining(lines, "× Expired")

	for _, needle := range []string{"1-hour cache", "KeepAlive"} {
		if visibleIndex(row1, needle) != visibleIndex(row2, needle) {
			t.Fatalf("%q columns differ:\n%s", needle, view)
		}
	}
	if visibleIndex(status1, "█") != visibleIndex(status2, "█") {
		t.Fatalf("progress columns differ:\n%s", view)
	}
}

func TestListKeepsEveryVisibleRowExcerptAndAddsTitleBreathingRoom(t *testing.T) {
	now := time.Date(2026, 6, 29, 23, 5, 0, 0, time.UTC)
	last := now.Add(-8 * time.Hour)
	var sessions []session.Session
	for i := 0; i < 5; i++ {
		sessions = append(sessions, session.Session{
			SessionID:      fmt.Sprintf("session-%d", i),
			ShortID:        fmt.Sprintf("5ac2bc0%d", i),
			Project:        "Users-richardchen-course-custom-skills",
			FileModifiedAt: now,
			LastMessageAt:  &last,
			CacheWindow:    session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true},
			TokenStats:     session.TokenStats{HitRate: 92},
			Messages: session.Messages{
				FirstUserExcerpt: "Let's discuss the context-handoff skill because this needs more setup",
				LastUserExcerpt:  "Base directory for this skill: /Users/richardchen/.claude/plugins/cache/anthropic-agent-skills",
			},
		})
	}
	view := NewModel(Options{Now: now, Width: 180, Height: 30, Sessions: sessions}).View()
	plain := stripANSI(view)

	if !strings.HasPrefix(plain, "\nClaude Code Watch\n") {
		t.Fatalf("main title is missing top breathing room:\n%s", view)
	}
	assertMaxLines(t, view, 30)
	for _, want := range []string{"#1", "#4", "5ac2bc00", "Users-r...m-skills", "Page 1/2"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("list missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(plain, "#5") {
		t.Fatalf("first page should show four complete rows, not five compact rows:\n%s", view)
	}
	if strings.Count(plain, "first  Let's discuss") != 4 || strings.Count(plain, "last  Base directory") != 4 {
		t.Fatalf("list should keep excerpts for every visible row:\n%s", view)
	}
}

func TestWorkspaceAndConfigUseAdaptiveTerminalLayout(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	sessionID := "workspace-id"
	workspace := NewModel(Options{
		Now:        now,
		Sessions:   []session.Session{workspaceSession(now)},
		SelectedID: sessionID,
	})
	workspaceView := workspace.View()
	assertMaxLineWidth(t, workspaceView, 80)
	assertMaxLines(t, workspaceView, 24)
	for _, want := range []string{"Claude Code Watch / workspace-api / workspace", "Cache Status", "Session Info", "Messages", "Tokens", "Controls", "› Reminder", "KeepAlive", "Auto-send", "ON", "off", "█", "░"} {
		if !strings.Contains(workspaceView, want) {
			t.Fatalf("workspace view missing %q:\n%s", want, workspaceView)
		}
	}
	for _, notWant := range []string{"╭─ System", "╭─ Evidence", "Evidence", "KA "} {
		if strings.Contains(workspaceView, notWant) {
			t.Fatalf("workspace view contains deprecated visual grammar %q:\n%s", notWant, workspaceView)
		}
	}
	for _, line := range strings.Split(workspaceView, "\n") {
		if strings.Contains(line, "JSONL:") && visibleWidth(stripANSI(line)) > 80 {
			t.Fatalf("workspace JSONL line width = %d, want <= 80:\n%s", visibleWidth(stripANSI(line)), workspaceView)
		}
	}

	configView := NewModel(Options{StartMode: StartConfig, Config: config.Default()}).View()
	assertMaxLineWidth(t, configView, 80)
	for _, want := range []string{"Claude Code Watch / config", "Settings", "Preview", "Validation", "Actions", "› Reminder thresholds", "KeepAlive trigger", "Auto-send", "ON"} {
		if !strings.Contains(configView, want) {
			t.Fatalf("config view missing %q:\n%s", want, configView)
		}
	}
	for _, notWant := range []string{"Warning: Auto-send default may send a Claude message after countdown.", "╭─ Reminder", "╭─ KeepAlive automation", "What will happen", "KA "} {
		if strings.Contains(configView, notWant) {
			t.Fatalf("config view contains deprecated visual grammar %q:\n%s", notWant, configView)
		}
	}
}

func TestWorkspacePolishRemovesDuplicateSessionRowAndUsesMessageChips(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	s := workspaceSession(now)
	last := now.Add(-20 * time.Minute)
	s.LastMessageAt = &last
	view := NewModel(Options{
		Now:        now,
		Width:      132,
		Height:     36,
		Sessions:   []session.Session{s},
		SelectedID: "workspace-id",
	}).View()
	plain := stripANSI(view)

	for _, want := range []string{"first  can you check", "last  please continue", "ACTIVE", "█", "33%", "40m00s left", "1-hour cache"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("workspace polish missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(plain, "Session    workspace") {
		t.Fatalf("workspace still repeats short session row in Cache Status:\n%s", view)
	}
	assertOrder(t, plain, "█", "33%", "40m00s left", "1-hour cache")
}

func TestMessageExcerptsUseColoredLabelsAndItalicTextWithoutBackground(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	view := NewModel(Options{
		Now:        now,
		Width:      132,
		Sessions:   []session.Session{workspaceSession(now)},
		SelectedID: "workspace-id",
	}).View()

	if strings.Contains(view, "[first]") || strings.Contains(view, "[last]") || strings.Contains(view, "\x1b[") && strings.Contains(view, "48;") {
		t.Fatalf("message labels still look like background chips:\n%s", view)
	}
	if !DefaultStyles().Has(RoleExcerptText) || !strings.Contains(stripANSI(view), "first  can you check") || !strings.Contains(stripANSI(view), "last  please continue") {
		t.Fatalf("message excerpts are not routed through the excerpt text role:\n%s", view)
	}
}

func TestKeepAliveCountdownPlacesTimeAfterProgressBar(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	state := keepalive.SessionState{
		SessionID:     "workspace-id",
		State:         keepalive.StateCountdown,
		AutoSend:      true,
		ScopeUsed:     0,
		MaxSends:      3,
		InstanceToken: 7,
	}
	view := NewModel(Options{
		Now:             now,
		Width:           132,
		Height:          36,
		Sessions:        []session.Session{workspaceSession(now)},
		SelectedID:      "workspace-id",
		KeepAliveConfig: cfg,
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": state,
		},
		Countdowns: map[string]int{"workspace-id": 20},
	}).View()
	plain := stripANSI(view)

	for _, want := range []string{"Countdown", "█", "20s remaining", "Scope        0 / 3 sends"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("countdown view missing %q:\n%s", want, view)
		}
	}
	assertOrder(t, plain, "Countdown    █", "20s remaining", "Scope        0 / 3 sends")
}

func TestConfigEditingUsesSeparatePromptPanel(t *testing.T) {
	now := time.Date(2026, 6, 29, 15, 0, 0, 0, time.UTC)
	updated, _ := NewModel(Options{Now: now, StartMode: StartConfig, Config: config.Default()}).Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	view := model.View()

	for _, want := range []string{"╭─ Settings", "╭─ Editing", "KeepAlive trigger", "Current input", "3", "↵ save field", "⎋ cancel edit"} {
		if !strings.Contains(stripANSI(view), want) {
			t.Fatalf("config edit view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(stripANSI(view), "Editing KeepAlive trigger: 3") {
		t.Fatalf("config edit prompt is still inline with options:\n%s", view)
	}
}

func TestViewIncludesWatcherRefreshAndEmptyStateBanners(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options Options
		want    []string
	}{
		{
			name: "live safety refresh",
			options: Options{
				Refresh: RefreshViewState{Watcher: refresh.State{Status: refresh.StatusOK, SafetyRefreshActive: true}},
			},
			want: []string{"Claude Code Watch"},
		},
		{
			name: "partial watcher degraded",
			options: Options{
				Refresh: RefreshViewState{Watcher: refresh.State{Status: refresh.StatusPartial, Messages: []string{"subdir permission denied"}, SafetyRefreshActive: true}},
			},
			want: []string{"watcher partial", "subdir permission denied"},
		},
		{
			name: "post start watcher failure",
			options: Options{
				Refresh: RefreshViewState{Watcher: refresh.State{Status: refresh.StatusDegraded, Messages: []string{"watcher closed"}, SafetyRefreshActive: true}},
			},
			want: []string{"watcher degraded", "watcher closed"},
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
			want: []string{"No sessions found", "Sessions appear after Claude Code writes JSONL files."},
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
	for _, want := range []string{"narrow-id", "ops", "1-hour cache", "● Active", "56m00s left", "↵ open", "r remind", "k KeepAlive"} {
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
	for _, want := range []string{"medium-id", "workspace", "1-hour cache", "50%", "hit 75%", "last  last user message"} {
		if !strings.Contains(medium, want) {
			t.Fatalf("medium view missing %q:\n%s", want, medium)
		}
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
	for _, want := range []string{"wide-id", "wide-project", "5-min cache", "40%", "last  last wide message", "! 1 parse warning(s)", "KeepAlive ON"} {
		if !strings.Contains(wide, want) {
			t.Fatalf("wide view missing %q:\n%s", want, wide)
		}
	}
	assertMaxLineWidth(t, wide, 80)
}

func TestListViewSanitizesMultilineExcerptsAndKeepsRowsBounded(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	multiline := "Research PKM\n<task-id>a6f6</task-id>\n<tool-use-id>tool-123</tool-use-id>\t\twith   spacing"
	model := NewModel(Options{
		Now:   now,
		Width: 80,
		Sessions: []session.Session{listViewSession("task-id", "ops", now, now.Add(-4*time.Minute), session.CacheWindow{
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
		}, multiline, multiline)},
	})

	view := model.View()
	assertMaxLineWidth(t, view, 80)
	if strings.Contains(view, "\n<task-id>") || strings.Contains(view, "\n<tool-use-id>") {
		t.Fatalf("multiline excerpt leaked raw lines:\n%s", view)
	}
	for _, want := range []string{"last  Research PKM <task-id>a6f6</task-id> <tool-use-id>", "u update", "c config"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sanitized list view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "  refresh  ") || strings.Contains(view, "focused  r remind") {
		t.Fatalf("list footer contains stale refresh/action prose:\n%s", view)
	}
}

func TestListEmptyStatesOnlyAdvertiseValidActions(t *testing.T) {
	view := NewModel(Options{
		Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
	}).View()

	for _, disallowed := range []string{"↵ open", "r remind", "k KeepAlive"} {
		if strings.Contains(view, disallowed) {
			t.Fatalf("empty state advertised invalid action %q:\n%s", disallowed, view)
		}
	}
	for _, want := range []string{"Refresh", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty state missing valid action %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Help") || strings.Contains(view, "? help") {
		t.Fatalf("empty state still advertises removed help page:\n%s", view)
	}
}

func TestListViewPaginatesFourCompleteSessionsWithPrevNextNavigation(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	var sessions []session.Session
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("page-%02d", i)
		sessions = append(sessions, listViewSession(id, fmt.Sprintf("project-%02d", i), now.Add(time.Duration(-i)*time.Minute), now.Add(-5*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", fmt.Sprintf("last %02d", i)))
	}
	model := NewModel(Options{Now: now, Width: 120, Sessions: sessions})
	view := model.View()

	for _, want := range []string{"#1", "page-01", "#4", "page-04", "Page 1/3", "Prev", "Next >", "←/→ page"} {
		if !strings.Contains(view, want) {
			t.Fatalf("page 1 view missing %q:\n%s", want, view)
		}
	}
	if lineIndexContaining(stripANSI(view), "Page 1/3") != 0 {
		t.Fatalf("pagination line should align with title:\n%s", view)
	}
	if strings.Contains(view, "n/p page") {
		t.Fatalf("page footer still advertises n/p instead of arrows:\n%s", view)
	}
	for _, notWant := range []string{"#5", "page-05"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("page 1 leaked page 2 row %q:\n%s", notWant, view)
		}
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)
	view = model.View()
	for _, want := range []string{"#5", "page-05", "#8", "page-08", "Page 2/3", "< Prev", "Next >"} {
		if !strings.Contains(view, want) {
			t.Fatalf("page 2 view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "#4  page-04") {
		t.Fatalf("page 2 leaked previous page row:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)
	if !strings.Contains(model.View(), "Page 1/3") || !strings.Contains(model.View(), "page-01") {
		t.Fatalf("left did not return to page 1:\n%s", model.View())
	}

	for i := 0; i < 4; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	view = model.View()
	if model.SelectedSessionID() != "page-05" || !strings.Contains(view, "Page 2/3") || !strings.Contains(view, "page-05") {
		t.Fatalf("down did not cross to next visible page; selected=%q:\n%s", model.SelectedSessionID(), view)
	}
}

func TestListFooterOnlyAdvertisesPagingWhenAnotherPageExists(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	sessions := []session.Session{
		listViewSession("page-01", "project-01", now, now.Add(-5*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "last 01"),
	}
	view := NewModel(Options{Now: now, Width: 120, Sessions: sessions}).View()
	if strings.Contains(view, "n/p page") || strings.Contains(view, "Page 1/") {
		t.Fatalf("single-page list advertised paging:\n%s", view)
	}
}

func TestConfigEditorRendersFieldsSummaryWarningsAndValidation(t *testing.T) {
	cfg := config.Default()
	cfg.KeepAlive.CountdownSeconds = 120
	model := NewModel(Options{
		StartMode: StartConfig,
		Config:    cfg,
	})

	view := model.View()
	for _, want := range []string{
		"Claude Code Watch / config",
		"Reminder thresholds",
		"20, 10%",
		"KeepAlive trigger",
		"5m",
		"Countdown",
		"120s",
		"Message",
		"Keep-alive check. Reply \"yes\" only.",
		"Auto-send",
		"ON",
		"send after countdown",
		"Max sends",
		"Preview",
		"1h cache:",
		"5m cache:",
		"Validation",
		"Cannot save.",
		"countdown may not fit the 5m cache trigger window",
		"↑↓ move  ↵ edit  space toggle  s save  d reset  ⎋ cancel",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("config view missing %q:\n%s", want, view)
		}
	}
}

func TestConfigEditorRendersAutosendWarning(t *testing.T) {
	cfg := config.Default()
	model := NewModel(Options{StartMode: StartConfig, Config: cfg})
	view := model.View()

	if !strings.Contains(view, "Auto-send is ON") || !strings.Contains(view, "Claude message after countdown") {
		t.Fatalf("config view missing relocated auto-send warning:\n%s", view)
	}
	if strings.Contains(view, "Warning: Auto-send default may send a Claude message after countdown.") {
		t.Fatalf("config view still renders old inline auto-send warning:\n%s", view)
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
			want: []string{"No projects directory", "/tmp/home/.claude/projects", "Claude has not written cache history here yet."},
		},
		{
			name: "no sessions",
			options: Options{
				Refresh: RefreshViewState{ProjectsDir: "/tmp/home/.claude/projects", EmptyState: EmptyNoSessions},
			},
			want: []string{"No sessions found", "/tmp/home/.claude/projects", "Sessions appear after Claude Code writes JSONL files."},
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
			want: []string{"Claude Code Watch / choose session", "partial id", "matched more than one session", "d4b247b7", "d4b901aa"},
		},
		{
			name: "degraded watcher notification and claude",
			options: Options{
				Now:   now,
				Width: 120,
				Refresh: RefreshViewState{
					Watcher:                  refresh.State{Status: refresh.StatusPartial, Messages: []string{"permission denied"}, SafetyRefreshActive: true},
					NotificationDegraded:     "osascript failed",
					ClaudeUnavailableMessage: "claude not found",
				},
				Sessions:         []session.Session{listViewSession("armed-id", "armed", now, now.Add(-1*time.Minute), session.CacheWindow{Label: "1h", TTLSeconds: 3600, Known: true}, "", "")},
				KeepAliveEnabled: map[string]bool{"armed-id": true},
			},
			want: []string{"watcher partial", "permission denied", "notifications degraded: osascript failed", "claude unavailable: claude not found"},
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
			want: []string{"warn-id", "! 2 parse warning(s)"},
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
			Message:  "osascript failed",
		},
	})
	model = updated.(Model)

	view := model.View()
	for _, want := range []string{
		"notification failed: Reminder alarm",
		"osascript failed",
		"No Claude message was sent",
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
	for _, notWant := range []string{"notification delivered:", "may be sent after 30s unless canceled"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("success notification leaked into TUI banner %q:\n%s", notWant, view)
		}
	}
}

func TestWorkspaceRendersCanonicalInfoAndControlsSeparately(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:                now,
		Width:              120,
		SelectedID:         "workspace-id",
		ReminderThresholds: []int{20, 10},
		Sessions:           []session.Session{workspaceSession(now)},
	})

	view := model.View()
	for _, want := range []string{
		"Claude Code Watch / workspace-api / workspace",
		"Cache Status",
		"Session Info",
		"Session ID",
		"Messages",
		"Tokens",
		"Gaps",
		"Controls",
		"› Reminder",
		"off",
		"notify at 20%, 10%",
		"KeepAlive",
		"5m before expiry · 1 send",
		"Auto-send",
		"ON",
		"send after countdown",
		"Back",
		"session list",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("workspace view missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "╭─ Controls") {
		t.Fatalf("workspace controls should use the same framed section language:\n%s", view)
	}

	for _, notWant := range []string{"Evidence", "manual refresh", "Copy ID", "alert at", "trigger 5m before expiry", "sends Claude message", "return to session list"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("workspace contains stale copy %q:\n%s", notWant, view)
		}
	}

	styles := DefaultStyles()
	for _, want := range []string{
		styles.Render(RoleMuted, "notify at 20%, 10%"),
		styles.Render(RoleMuted, "5m before expiry · 1 send"),
		styles.Render(RoleMuted, "send after countdown"),
		styles.Render(RoleMuted, "session list"),
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("control detail should use muted helper styling %q:\n%s", want, view)
		}
	}

	assertOrder(t, view, "Cache Status", "Session Info", "Messages", "Tokens", "Gaps", "Controls")
}

func TestExpiredWorkspaceUsesNeutralUnavailableNA(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	expiredLast := now.Add(-2 * time.Hour)
	expired := workspaceSession(now)
	expired.LastMessageAt = &expiredLast
	expired.CacheWindow = session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true}
	view := NewModel(Options{
		Now:        now,
		Width:      120,
		SelectedID: expired.SessionID,
		Sessions:   []session.Session{expired},
	}).View()
	plain := stripANSI(view)

	for _, want := range []string{"× EXPIRED", "Reminder", "KeepAlive", "Auto-send", "N/A  after expiry"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expired workspace missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"unavailable", "Auto-send   ON", "disabled while KeepAlive is N/A", "\x1b[38;5;214mN/A", "\x1b[38;5;202mN/A"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("expired workspace still uses old unavailable/orange grammar %q:\n%s", notWant, view)
		}
	}
	if !strings.Contains(view, DefaultStyles().Render(RoleMuted, "after expiry")) {
		t.Fatalf("expired workspace detail should use muted helper styling:\n%s", view)
	}
}

func TestUnknownStatusUsesUnicodeMarkerWithText(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	unknown := listViewSession("unknown-id", "unknown", now, time.Time{}, session.CacheWindow{Label: "TTL ?", Known: false}, "", "")
	unknown.LastMessageAt = nil
	list := NewModel(Options{
		Now:      now,
		Width:    120,
		Sessions: []session.Session{unknown},
	}).View()
	if !strings.Contains(stripANSI(list), "○ Unknown   no timestamp") {
		t.Fatalf("list unknown status missing Unicode marker plus text:\n%s", list)
	}

	workspace := NewModel(Options{
		Now:        now,
		Width:      120,
		SelectedID: unknown.SessionID,
		Sessions:   []session.Session{unknown},
	}).View()
	if !strings.Contains(stripANSI(workspace), "○ UNKNOWN") {
		t.Fatalf("workspace unknown status missing Unicode marker plus text:\n%s", workspace)
	}
}

func TestWorkspacePanelsUseClosedConsistentFrames(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	view := NewModel(Options{
		Now:        now,
		Width:      160,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
	}).View()

	var widths []int
	for _, line := range strings.Split(view, "\n") {
		if strings.HasPrefix(stripANSI(line), "╭") || strings.HasPrefix(stripANSI(line), "│") || strings.HasPrefix(stripANSI(line), "╰") {
			widths = append(widths, visibleWidth(stripANSI(line)))
		}
	}
	if len(widths) == 0 {
		t.Fatalf("workspace rendered no framed panels:\n%s", view)
	}
	for _, width := range widths {
		if width != widths[0] {
			t.Fatalf("workspace panel widths differ: %v\n%s", widths, view)
		}
		if width > 80 {
			t.Fatalf("workspace panel width = %d, want <= 80:\n%s", width, view)
		}
	}
}

func TestConfigPanelsStayReadableOnWideTerminals(t *testing.T) {
	view := NewModel(Options{StartMode: StartConfig, Config: config.Default(), Width: 160}).View()

	var widths []int
	for _, line := range strings.Split(view, "\n") {
		if strings.HasPrefix(stripANSI(line), "╭") || strings.HasPrefix(stripANSI(line), "│") || strings.HasPrefix(stripANSI(line), "╰") {
			widths = append(widths, visibleWidth(stripANSI(line)))
		}
	}
	if len(widths) == 0 {
		t.Fatalf("config rendered no framed panels:\n%s", view)
	}
	for _, width := range widths {
		if width > 80 {
			t.Fatalf("config panel width = %d, want <= 80:\n%s", width, view)
		}
	}
}

func TestWorkspaceRendersCanonicalCacheStatusAndSessionInfoCards(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	activeLast := now.Add(-108 * time.Second)
	expiredLast := now.Add(-2 * time.Hour)
	active := workspaceSession(now)
	active.LastMessageAt = &activeLast
	active.CacheWindow = session.CacheWindow{Tier: session.Tier1Hour, Label: "1h", TTLSeconds: 3600, Known: true, Evidence: []string{"ephemeral_1h_input_tokens"}}
	active.TokenStats.HitRate = 95
	expired := active
	expired.SessionID = "expired-id"
	expired.ShortID = "expired"
	expired.LastMessageAt = &expiredLast

	model := NewModel(Options{
		Now:        now,
		Width:      120,
		SelectedID: active.SessionID,
		Sessions:   []session.Session{active},
	})
	view := model.View()
	for _, want := range []string{"Cache Status", "Session Info", "Session ID", "Messages", "Tokens", "Gaps", "1-hour cache", "95%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("workspace canonical card missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Evidence") {
		t.Fatalf("workspace renders Evidence as user-facing copy:\n%s", view)
	}
	if ttlPercentRole(3) != RoleSuccess || ttlPercentRole(100) != RoleDanger {
		t.Fatalf("TTL percent roles are not low-good/high-danger")
	}
	if hitRatePercentRole(95) != RoleSuccess || hitRatePercentRole(10) != RoleDanger {
		t.Fatalf("hit-rate percent roles are not high-good/low-danger")
	}

	expiredView := NewModel(Options{
		Now:        now,
		Width:      120,
		SelectedID: expired.SessionID,
		Sessions:   []session.Session{expired},
	}).View()
	for _, want := range []string{"Cache Status", "EXPIRED", "100%"} {
		if !strings.Contains(expiredView, want) {
			t.Fatalf("expired workspace missing %q:\n%s", want, expiredView)
		}
	}
}

func TestSessionInfoDetailsDisclosureAndGapSorting(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	s := workspaceSession(now)
	s.Gaps = []session.Gap{
		{Seconds: 64, From: now.Add(-4 * time.Minute), To: now.Add(-3*time.Minute - 4*time.Second)},
		{Seconds: 184, From: now.Add(-30 * time.Minute), To: now.Add(-26*time.Minute - 56*time.Second), Reset: true},
		{Seconds: 117, From: now.Add(-45 * time.Minute), To: now.Add(-43*time.Minute - 3*time.Second), Reset: true},
		{Seconds: 42, From: now.Add(-55 * time.Minute), To: now.Add(-54*time.Minute - 18*time.Second)},
	}
	s.ResetCount = 2
	model := NewModel(Options{
		Now:        now,
		Width:      120,
		Height:     24,
		SelectedID: s.SessionID,
		Sessions:   []session.Session{s},
	})
	initialFocus := model.FocusedAction()
	if strings.Contains(model.View(), "Session Info · details") {
		t.Fatalf("details are open by default:\n%s", model.View())
	}

	updated, _ := model.Update(keyRunes("v"))
	model = updated.(Model)
	if model.FocusedAction() != "details_scroll" {
		t.Fatalf("v changed focus from %q to %q, want details_scroll", initialFocus, model.FocusedAction())
	}
	details := model.View()
	for _, want := range []string{"Session Info · details", "JSONL", "Updated", "Token Stats", "Mid-session Gaps >1min · ↕ longest", "! RESET", "184s", "b/⎋ back"} {
		if !strings.Contains(details, want) {
			t.Fatalf("details view missing %q:\n%s", want, details)
		}
	}
	if strings.Contains(details, "Mid-session Gaps >1min                                  ↕") {
		t.Fatalf("details sort label is visually detached:\n%s", details)
	}
	if !strings.Contains(details, "3 more gap(s); use ↑↓") {
		t.Fatalf("details view missing scroll affordance after one visible gap:\n%s", details)
	}
	assertMaxLines(t, details, 24)

	updated, _ = model.Update(keyRunes("s"))
	model = updated.(Model)
	newest := model.View()
	for _, want := range []string{"Session Info · details", "Mid-session Gaps >1min · ↕ newest"} {
		if !strings.Contains(newest, want) {
			t.Fatalf("newest details missing %q:\n%s", want, newest)
		}
	}
	if !strings.Contains(newest, "64s") || !strings.Contains(newest, "3 more gap(s); use ↑↓") {
		t.Fatalf("newest details missing visible newest gap or scroll affordance:\n%s", newest)
	}
}

func TestSessionInfoDetailsWithoutGapsShowsEmptySentenceAndKeepsControlFocus(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	s := workspaceSession(now)
	s.Gaps = nil
	s.ResetCount = 0
	model := NewModel(Options{
		Now:        now,
		Width:      120,
		Height:     24,
		SelectedID: s.SessionID,
		Sessions:   []session.Session{s},
	})

	updated, _ := model.Update(keyRunes("v"))
	model = updated.(Model)

	if model.FocusedAction() == "details_scroll" {
		t.Fatalf("non-scrollable details took focus:\n%s", model.View())
	}
	view := model.View()
	if strings.Contains(view, "› Session Info · details") {
		t.Fatalf("non-scrollable details heading rendered as focused:\n%s", view)
	}
	if !strings.Contains(view, "No mid-session gaps found.") {
		t.Fatalf("details missing no-gap sentence:\n%s", view)
	}
}

func TestWorkspaceKeepAliveCardStatesRenderSafetyContract(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cfg := config.Default().KeepAlive
	cfg.AutoSend = true

	for _, tc := range []struct {
		name  string
		state keepalive.SessionState
		want  []string
	}{
		{
			name:  "watching auto-send on",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateMonitoringIdle, AutoSend: true, MaxSends: 1},
			want:  []string{"KeepAlive · watching", "Next", "Countdown at", "Msg Preview", "Scope", "0 / 1 sends · auto-send"},
		},
		{
			name:  "watching auto-send off",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateMonitoringIdle, AutoSend: false, MaxSends: 1},
			want:  []string{"KeepAlive · watching", "Next", "Manual prompt at", "Msg Preview", "Scope", "0 / 1 sends · auto-send"},
		},
		{
			name:  "countdown",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateCountdown, AutoSend: true, ScopeUsed: 0, MaxSends: 1, InstanceToken: 7},
			want:  []string{"KeepAlive · countdown", "Next", "Send now or cancel before countdown ends", "Msg Preview", "Scope", "24s remaining"},
		},
		{
			name:  "manual prompt",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, MaxSends: 1, InstanceToken: 8},
			want:  []string{"KeepAlive · manual prompt", "Next", "Send now or dismiss", "Msg Preview", "Scope", "auto-send off"},
		},
		{
			name:  "confirming",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateConfirming, AutoSend: true, ScopeUsed: 1, MaxSends: 1, InstanceToken: 9},
			want:  []string{"KeepAlive · confirming", "Next", "Watching this session JSONL", "Msg Preview", "Scope", "awaiting confirmation"},
		},
		{
			name:  "success",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateSuccess, AutoSend: true, ScopeUsed: 1, MaxSends: 1, InstanceToken: 10, LastResult: "confirmed at 12:00:30"},
			want:  []string{"KeepAlive · done", "Next", "Monitoring complete", "Msg Preview", "Scope", "confirmed at 12:00:30"},
		},
		{
			name:  "failure",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateErrorNoClaude, AutoSend: false, ScopeUsed: 1, MaxSends: 1, InstanceToken: 11, LastFailure: "claude command not found"},
			want:  []string{"KeepAlive · failed", "Next", "Use manual fallback", "Msg Preview", "Scope", "failed: claude command not found", "Fallback", "claude -r workspace-id -p"},
		},
		{
			name:  "scope complete",
			state: keepalive.SessionState{SessionID: "workspace-id", State: keepalive.StateScopeComplete, AutoSend: false, ScopeUsed: 1, MaxSends: 1, InstanceToken: 12},
			want:  []string{"KeepAlive · scope complete", "Next", "Turn KeepAlive off", "Msg Preview", "Scope", "no more automatic sends"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel(Options{
				Now:             now,
				Width:           120,
				SelectedID:      "workspace-id",
				Sessions:        []session.Session{workspaceSession(now)},
				KeepAliveConfig: cfg,
				KeepAliveStates: map[string]keepalive.SessionState{"workspace-id": tc.state},
				Countdowns:      map[string]int{"workspace-id": 24},
			})
			view := model.View()
			for _, want := range tc.want {
				if !strings.Contains(view, want) {
					t.Fatalf("%s view missing %q:\n%s", tc.name, want, view)
				}
			}
			for _, notWant := range []string{"State    ", "Evidence "} {
				if strings.Contains(view, notWant) {
					t.Fatalf("%s view contains stale row %q:\n%s", tc.name, notWant, view)
				}
			}
		})
	}
}

func TestWorkspaceKeepAliveCardIsAdditiveAndControlsOwnFocus(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		Width:      120,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, MaxSends: 1, InstanceToken: 8},
		},
	})

	view := model.View()
	for _, want := range []string{"Cache Status", "Session Info", "Controls", "KeepAlive · manual prompt", "Next", "Msg Preview", "Scope", "› Send now", "Dismiss"} {
		if !strings.Contains(view, want) {
			t.Fatalf("additive KeepAlive card missing %q:\n%s", want, view)
		}
	}
	assertOrder(t, view, "Session Info", "KeepAlive · manual prompt", "Controls")
	for _, notWant := range []string{"State    ", "Evidence ", "Actions", "Cancel watching"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("additive KeepAlive card contains stale row %q:\n%s", notWant, view)
		}
	}

	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		seen[model.FocusedAction()] = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	for _, want := range []string{"keepalive_send_now", "keepalive_cancel", "reminder", "keepalive", "keepalive_autosend", "back"} {
		if !seen[want] {
			t.Fatalf("KeepAlive focus did not reach %q; saw %#v", want, seen)
		}
	}
	for _, hidden := range []string{"copy_id", "evidence", "refresh", "help", "quit"} {
		if seen[hidden] {
			t.Fatalf("KeepAlive focus reached hidden action %q; saw %#v", hidden, seen)
		}
	}
}

func TestListRowsUseDomainSemanticRoles(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	styles := DefaultStyles()
	for _, role := range []StyleRole{RoleExcerptLabel, RoleReminder, RoleKeepAlive} {
		if !styles.Has(role) {
			t.Fatalf("domain role %q missing", role)
		}
	}
	model := NewModel(Options{
		Now:   now,
		Width: 140,
		Sessions: []session.Session{listViewSession("semantic-id", "semantic", now, now.Add(-5*time.Minute), session.CacheWindow{
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
		}, "first semantic message", "last semantic message")},
		KeepAliveEnabled: map[string]bool{"semantic-id": true},
		ReminderEnabled:  map[string]bool{"semantic-id": true},
	})

	view := model.View()
	for _, want := range []string{"first", "last", "KeepAlive ON"} {
		if !strings.Contains(stripANSI(view), want) {
			t.Fatalf("semantic list view missing %q:\n%s", want, view)
		}
	}
}

func TestWorkspaceKeepAliveManualReadyFitsEightyByTwentyFour(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	view := NewModel(Options{
		Now:        now,
		Width:      80,
		Height:     24,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, MaxSends: 1, InstanceToken: 8},
		},
	}).View()

	assertMaxLineWidth(t, view, 80)
	assertMaxLines(t, view, 24)
	for _, want := range []string{"Claude Code Watch / workspace-api / workspace", "KeepAlive · manual prompt", "ready", "Send now", "Dismiss"} {
		if !strings.Contains(view, want) {
			t.Fatalf("manual-ready workspace missing %q:\n%s", want, view)
		}
	}
}

func TestWorkspaceWrapsLongMessagesAndUsesSectionedKeepAliveCard(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	longSession := workspaceSession(now)
	longSession.Messages.FirstUserExcerpt = strings.Repeat("first excerpt that should not run beyond the terminal ", 6)
	longSession.Messages.LastUserExcerpt = strings.Repeat("last excerpt that should remain bounded and readable ", 6)
	view := NewModel(Options{
		Now:        now,
		Width:      80,
		Height:     24,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{longSession},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateManualReady, AutoSend: false, MaxSends: 1, InstanceToken: 8, SafetyDisabled: true},
		},
	}).View()

	assertMaxLineWidth(t, view, 80)
	assertMaxLines(t, view, 24)
	for _, want := range []string{"Session Info", "Controls", "KeepAlive · manual prompt", "Reason", "› Send now"} {
		if !strings.Contains(view, want) {
			t.Fatalf("workspace missing visual separator %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"── Session", "── Messages", "── Controls", "── KeepAlive"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("workspace contains heavy section rule %q:\n%s", notWant, view)
		}
	}
	for _, notWant := range []string{"[x] KeepAlive", "--------------------------------", "first excerpt that should not run beyond the terminal first excerpt that should not run beyond the terminal"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("workspace contains deprecated or overflowing text %q:\n%s", notWant, view)
		}
	}
}

func TestWorkspaceShowsClaudeUnavailableBeforeCountdownCanSend(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	model := NewModel(Options{
		Now:        now,
		SelectedID: "workspace-id",
		Sessions:   []session.Session{workspaceSession(now)},
		Refresh:    RefreshViewState{ClaudeUnavailableMessage: "claude command not found"},
		KeepAliveStates: map[string]keepalive.SessionState{
			"workspace-id": {SessionID: "workspace-id", State: keepalive.StateErrorNoClaude, AutoSend: false, LastFailure: "claude command not found", MaxSends: 1},
		},
	})

	view := model.View()
	for _, want := range []string{"claude unavailable: claude command not found", "KeepAlive · failed", "failed: claude command not found", "Fallback"} {
		if !strings.Contains(view, want) {
			t.Fatalf("workspace unavailable view missing %q:\n%s", want, view)
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

func workspaceSession(now time.Time) session.Session {
	last := now.Add(-54 * time.Minute)
	duration := 3*3600 + 8*60
	return session.Session{
		SessionID:       "workspace-id",
		ShortID:         "workspace",
		Project:         "workspace-api",
		JSONLPath:       "/tmp/home/.claude/projects/workspace-api/workspace-id.jsonl",
		FileModifiedAt:  now,
		LastMessageAt:   &last,
		StartedAt:       &last,
		EndedAt:         &now,
		DurationSeconds: &duration,
		CacheWindow: session.CacheWindow{
			Tier:       session.Tier1Hour,
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
			Evidence:   []string{"ephemeral_1h_input_tokens"},
		},
		Messages: session.Messages{
			FirstUserExcerpt: "can you check whether this session is cached for 5m or 1h?",
			LastUserExcerpt:  "please continue the implementation",
		},
		TokenStats: session.TokenStats{
			CacheWrites: 100,
			CacheReads:  900,
			HitRate:     90,
		},
		Gaps: []session.Gap{{Seconds: 60, From: last, To: last.Add(time.Minute)}},
	}
}

func assertOrder(t *testing.T, text string, values ...string) {
	t.Helper()
	previous := -1
	for _, value := range values {
		index := strings.Index(text, value)
		if index == -1 {
			t.Fatalf("missing %q in:\n%s", value, text)
		}
		if index < previous {
			t.Fatalf("%q appeared out of order in:\n%s", value, text)
		}
		previous = index
	}
}

func assertMaxLineWidth(t *testing.T, text string, maxWidth int) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if width := visibleWidth(stripANSI(line)); width > maxWidth {
			t.Fatalf("line width = %d, want <= %d\nline: %s\nview:\n%s", width, maxWidth, line, text)
		}
	}
}

func assertMaxLines(t *testing.T, text string, maxLines int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > maxLines {
		t.Fatalf("line count = %d, want <= %d\nview:\n%s", len(lines), maxLines, text)
	}
}

func lineIndexContaining(text string, needle string) int {
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, needle); i >= 0 {
			return i
		}
	}
	return -1
}

func lineContaining(lines []string, needle string) string {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func visibleIndex(line string, needle string) int {
	index := strings.Index(line, needle)
	if index < 0 {
		return -1
	}
	return visibleWidth(line[:index])
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
