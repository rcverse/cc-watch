package tui

import (
	"fmt"
	"strings"
)

func (m Model) View() string {
	if m.route == RouteList || m.route == RouteAmbiguous {
		return m.listView()
	}
	if m.route == RouteWorkspace {
		return m.workspaceView()
	}
	if m.route == RouteConfig {
		return m.configView()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "cc-watch %s\n", m.route)
	fmt.Fprintf(&b, "focus: %s\n", m.FocusedAction())
	b.WriteString(m.refreshBanner())
	if len(m.watcherEvents) > 0 {
		fmt.Fprintf(&b, "watcher events: %d\n", len(m.watcherEvents))
	}
	return b.String()
}

func (m Model) refreshBanner() string {
	var b strings.Builder
	if m.refresh.EmptyState == EmptyProjectsDir {
		fmt.Fprintf(&b, "No projects directory: %s\n", m.refresh.ProjectsDir)
		return b.String()
	}
	if m.refresh.EmptyState == EmptyNoSessions {
		fmt.Fprintf(&b, "No sessions found: %s\n", m.refresh.ProjectsDir)
		b.WriteString("sessions appear after Claude Code writes JSONL files\n")
		return b.String()
	}

	status := m.refresh.Watcher.Status
	if status == "" {
		status = "ok"
	}
	fmt.Fprintf(&b, "Watcher: %s\n", status)
	if len(m.refresh.Watcher.Messages) > 0 {
		fmt.Fprintf(&b, "%s\n", m.refresh.Watcher.Messages[0])
	}
	if m.refresh.Watcher.SafetyRefreshActive {
		b.WriteString("Safety refresh: active\n")
	}
	return b.String()
}
