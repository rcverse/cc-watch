//go:build demo

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rcverse/cc-watch/internal/config"
	"github.com/rcverse/cc-watch/internal/keepalive"
	"github.com/rcverse/cc-watch/internal/notify"
	"github.com/rcverse/cc-watch/internal/session"
	"github.com/rcverse/cc-watch/internal/tui"
)

const demoSessionID = "demo-workspace-11111111"

type demoModel struct {
	inner           tui.Model
	now             time.Time
	route           tui.Route
	state           *demoState
	showCues        bool
	reminderEnabled bool
}

type demoState struct {
	cfg     config.Config
	manager *keepalive.Manager
	clock   *demoClock
}

type demoClock struct {
	now time.Time
}

func main() {
	if _, err := tea.NewProgram(newDemoModel()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newDemoModel() demoModel {
	cfg := config.Default()
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.Local)
	model := demoModel{
		now:             now,
		route:           tui.RouteList,
		state:           &demoState{cfg: cfg, manager: keepalive.NewManager(cfg.KeepAlive), clock: &demoClock{now: now}},
		showCues:        os.Getenv("CC_WATCH_DEMO_CUES") != "off",
		reminderEnabled: os.Getenv("CC_WATCH_DEMO_REMINDER") != "off",
	}
	if os.Getenv("CC_WATCH_DEMO_START") == "workspace" {
		model.route = tui.RouteWorkspace
	}
	model.rebuild()
	return model
}

func (m demoModel) Init() tea.Cmd {
	return nil
}

func (m demoModel) View() string {
	if !m.showCues {
		return m.inner.View()
	}
	return m.inner.View() + "\n" + "demo: 1 list  2 workspace  3 ambiguous  4 config  j KA trigger  J expiry  . +5s  , -5s\n"
}

func (m demoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "1":
			m.route = tui.RouteList
			m.rebuild()
			return m, nil
		case "2":
			m.route = tui.RouteWorkspace
			m.rebuild()
			return m, nil
		case "3":
			m.route = tui.RouteAmbiguous
			m.rebuild()
			return m, nil
		case "4":
			m.route = tui.RouteConfig
			m.rebuild()
			return m, nil
		case "j":
			m.route = tui.RouteWorkspace
			m.rebuild()
			return m.tick(m.keepAliveTriggerTime())
		case "J":
			m.route = tui.RouteWorkspace
			m.rebuild()
			return m.tick(m.cacheExpiryTime())
		case ".":
			return m.advance(5 * time.Second)
		case ",":
			return m.advance(-5 * time.Second)
		}
		if key.String() == "r" && m.route != tui.RouteConfig {
			m.reminderEnabled = !m.reminderEnabled
		}
	}
	updated, cmd := m.inner.Update(msg)
	if inner, ok := updated.(tui.Model); ok {
		m.inner = inner
	}
	return m, cmd
}

func (m *demoModel) rebuild() {
	sessions := demoSessions(m.now)
	if os.Getenv("CC_WATCH_DEMO_KEEPALIVE") != "off" && m.state.manager.State(demoSessionID).State == keepalive.StateOff {
		m.state.manager.Enable(sessions[0], m.now)
	}
	options := tui.Options{
		Now:              m.now,
		Width:            120,
		Height:           29,
		Config:           m.state.cfg,
		KeepAliveConfig:  m.state.cfg.KeepAlive,
		KeepAliveManager: m.state.manager,
		ReminderEnabled:  map[string]bool{demoSessionID: m.reminderEnabled},
		Sessions:         sessions,
		Refresh:          tui.RefreshViewState{ProjectsDir: "/demo/.claude/projects"},
		Dependencies: tui.Dependencies{
			RefreshSnapshot: func(selected *session.Session) tui.RefreshSnapshot {
				if selected == nil {
					return tui.RefreshSnapshot{}
				}
				refreshed := *selected
				anchor := m.state.clock.now
				refreshed.CacheAnchorAt = &anchor
				return tui.RefreshSnapshot{
					Sessions:     []session.Session{refreshed},
					HasRefresh:   true,
					SelectedOnly: true,
					SelectedID:   selected.SessionID,
				}
			},
			CheckClaudeAvailable: func() error { return nil },
			KeepAliveRunner:      fakeRunner{},
			ConfirmKeepAlive: func(context.Context, keepalive.ConfirmationTarget) (keepalive.ConfirmationResult, error) {
				return keepalive.ConfirmationResult{Confirmed: true, ConfirmedAt: m.state.clock.now}, nil
			},
			SaveConfig: func(next config.Config) error {
				m.state.cfg = next
				m.state.manager = keepalive.NewManager(next.KeepAlive)
				return nil
			},
			NotifyEvent: func(notify.Event) notify.Result { return notify.Result{Suppressed: true} },
		},
	}
	switch m.route {
	case tui.RouteWorkspace:
		options.SelectedID = demoSessionID
	case tui.RouteAmbiguous:
		options.AmbiguousID = "demo"
	case tui.RouteConfig:
		options.StartMode = tui.StartConfig
	}
	m.inner = tui.NewModel(options)
}

func (m demoModel) advance(delta time.Duration) (tea.Model, tea.Cmd) {
	steps := int(delta / time.Second)
	if steps <= 0 {
		return m.tick(m.now.Add(delta))
	}
	var commands []tea.Cmd
	for i := 0; i < steps; i++ {
		updated, cmd := m.tick(m.now.Add(time.Second))
		m = updated.(demoModel)
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}
	return m, tea.Batch(commands...)
}

func (m demoModel) tick(next time.Time) (tea.Model, tea.Cmd) {
	m.now = next
	m.state.clock.now = next
	updated, cmd := m.inner.Update(tui.DisplayTickMsg{Now: m.now})
	if inner, ok := updated.(tui.Model); ok {
		m.inner = inner
	}
	return m, cmd
}

func (m demoModel) keepAliveTriggerTime() time.Time {
	s := demoWorkspaceSession(m.now)
	return s.CacheAnchorAt.Add(time.Duration(s.CacheWindow.TTLSeconds-m.state.cfg.KeepAlive.TriggerBeforeExpiryMinutes*60) * time.Second)
}

func (m demoModel) cacheExpiryTime() time.Time {
	s := demoWorkspaceSession(m.now)
	return s.CacheAnchorAt.Add(time.Duration(s.CacheWindow.TTLSeconds) * time.Second)
}

type fakeRunner struct{}

func (fakeRunner) Available() error { return nil }

func (fakeRunner) Send(_ context.Context, req keepalive.RunRequest) keepalive.RunResult {
	return keepalive.RunResult{StartedAt: time.Now(), Stdout: "demo sent " + req.SessionID}
}

func demoSessions(now time.Time) []session.Session {
	active := demoWorkspaceSession(now)
	fading := active
	fading.SessionID = "demo-fading-22222222"
	fading.ShortID = "demo-fading"
	fadingLast := now.Add(-58 * time.Minute)
	fading.CacheAnchorAt = &fadingLast
	fading.Project = "fading-cache"

	expired := active
	expired.SessionID = "demo-expired-33333333"
	expired.ShortID = "demo-expired"
	expiredLast := now.Add(-2 * time.Hour)
	expired.CacheAnchorAt = &expiredLast
	expired.Project = "expired-cache"

	unknown := active
	unknown.SessionID = "demo-unknown-44444444"
	unknown.ShortID = "demo-unknown"
	unknown.Project = "unknown-cache"
	unknown.CacheWindow = session.CacheWindow{Known: false, Label: "unknown"}

	return []session.Session{active, fading, expired, unknown}
}

func demoWorkspaceSession(now time.Time) session.Session {
	last := now.Add(-10 * time.Minute)
	start := last.Add(-2 * time.Hour)
	end := now
	duration := int(end.Sub(start).Seconds())
	return session.Session{
		SessionID:       demoSessionID,
		ShortID:         "demo-workspace",
		Project:         "demo-workspace",
		JSONLPath:       "/demo/.claude/projects/demo-workspace/demo-workspace-11111111.jsonl",
		FileModifiedAt:  now,
		CacheAnchorAt:   &last,
		DurationSeconds: &duration,
		Cwd:             "/demo/workspace",
		CacheWindow: session.CacheWindow{
			Tier:       session.Tier1Hour,
			Label:      "1h",
			TTLSeconds: 3600,
			Known:      true,
		},
		Messages: session.Messages{
			FirstUserExcerpt: "check which cache window is active",
			LastUserExcerpt:  "prepare the README demo",
		},
		RecentMessages: []session.MessageWindow{
			{At: now.Add(-95 * time.Minute), Excerpt: "inspect the cache window"},
			{At: now.Add(-31 * time.Minute), Excerpt: "show the recent cache state"},
			{At: now.Add(-4 * time.Minute), Excerpt: "prepare the README demo"},
		},
		TokenStats: session.TokenStats{
			CacheWrites:  120,
			CacheReads:   960,
			OutputTokens: 240,
			HitRate:      89,
		},
		Gaps: []session.Gap{
			{Seconds: 4500, From: now.Add(-180 * time.Minute), To: now.Add(-105 * time.Minute), Reset: true},
			{Seconds: 510, From: now.Add(-75 * time.Minute), To: now.Add(-66*time.Minute - 30*time.Second)},
		},
		ResetCount: 1,
	}
}
