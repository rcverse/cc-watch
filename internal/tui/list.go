package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/session"
)

func (m Model) listView() string {
	var b strings.Builder
	if m.route == RouteAmbiguous {
		return m.ambiguousListView()
	}

	fmt.Fprintf(&b, "cc-cache list  %d sessions\n", len(m.sessions))
	fmt.Fprintf(&b, "focus: %s\n", m.FocusedAction())
	b.WriteString(m.listSystemState())

	switch m.refresh.EmptyState {
	case EmptyLoading:
		b.WriteString("Loading sessions...\n")
	case EmptyProjectsDir:
		b.WriteString("No projects directory.\n")
		fmt.Fprintf(&b, "No ~/.claude/projects directory exists yet: %s\n", m.refresh.ProjectsDir)
		b.WriteString("cc-cache cannot discover sessions until Claude Code creates that directory.\n")
	case EmptyNoSessions:
		b.WriteString("No sessions found.\n")
		b.WriteString("No Claude Code session files found.\n")
		fmt.Fprintf(&b, "cc-cache looks for JSONL files under: %s\n", m.refresh.ProjectsDir)
		b.WriteString("Sessions appear here after Claude Code writes JSONL files.\n")
		b.WriteString("sessions appear after Claude Code writes JSONL files\n")
	default:
		for i, s := range m.sessions {
			b.WriteString(m.renderListRow(i, s))
		}
	}

	b.WriteString(m.listFooter())
	if m.helpOpen {
		b.WriteString("\n")
		b.WriteString(m.helpText())
	}
	return b.String()
}

func (m Model) ambiguousListView() string {
	var b strings.Builder
	fmt.Fprintf(&b, "cc-cache ambiguous session id: %s\n", m.refreshQuery())
	b.WriteString("The partial id matched more than one session. Choose one to open.\n")
	b.WriteString(m.listSystemState())
	for i, s := range m.sessions {
		marker := " "
		if m.focusIndex == i {
			marker = ">"
		}
		status := s.StatusAt(m.now)
		fmt.Fprintf(&b, "%s %s  %s  modified %s  %s  %s\n", marker, displayID(s), s.Project, formatModified(s.FileModifiedAt), status.State, formatStatusTime(status))
	}
	b.WriteString("up/down move  enter open selected  esc list  q quit\n")
	return b.String()
}

func (m Model) listSystemState() string {
	var b strings.Builder
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
	if m.refresh.NotificationDegraded != "" {
		fmt.Fprintf(&b, "Notify degraded: %s\n", m.refresh.NotificationDegraded)
	}
	if len(m.notificationStatuses) > 0 {
		status := m.notificationStatuses[0]
		switch {
		case status.Result.Degraded:
			fmt.Fprintf(&b, "Notification delivery failed: %s\n", status.Result.Message)
			fmt.Fprintf(&b, "Event happened: %s - %s\n", status.Notification.Title, status.Notification.Body)
		case status.Result.Delivered:
			fmt.Fprintf(&b, "Notification delivered: %s - %s\n", status.Notification.Title, status.Notification.Body)
		}
	}
	if m.shouldShowClaudeUnavailable() {
		fmt.Fprintf(&b, "claude unavailable: %s\n", m.refresh.ClaudeUnavailableMessage)
	}
	return b.String()
}

func (m Model) renderListRow(index int, s session.Session) string {
	var b strings.Builder
	marker := " "
	if m.focusIndex == index {
		marker = ">"
	}
	status := s.StatusAt(m.now)
	fmt.Fprintf(&b, "%s %s %s  %s %s  %s",
		marker,
		displayID(s),
		truncateMiddle(s.Project, 18),
		cacheLabel(s),
		status.State,
		formatStatusTime(status),
	)
	if m.width < 90 {
		fmt.Fprintf(&b, "  %s", compactKeepAliveSummary(m, s))
		if len(s.Warnings) > 0 {
			fmt.Fprintf(&b, "  warn %d", len(s.Warnings))
		}
		b.WriteString("\n")
		b.WriteString(m.renderNarrowEvidence(s, status))
		return b.String()
	}

	if status.PercentElapsed != nil {
		fmt.Fprintf(&b, "  TTL %.0f%%", *status.PercentElapsed)
	}
	fmt.Fprintf(&b, "  hit %.0f%%", s.TokenStats.HitRate)
	if len(s.Warnings) > 0 {
		fmt.Fprintf(&b, "  warnings: %d", len(s.Warnings))
	}
	if m.width >= 120 {
		if s.DurationSeconds != nil {
			fmt.Fprintf(&b, "  duration %s", formatDuration(*s.DurationSeconds))
		}
		if summary := keepAliveSummary(m, s); summary != "" {
			fmt.Fprintf(&b, "  %s", summary)
		}
	}
	b.WriteString("\n")

	if m.width >= 120 {
		if s.Messages.FirstUserExcerpt != "" {
			fmt.Fprintf(&b, "  first: %q\n", truncateEnd(s.Messages.FirstUserExcerpt, m.width-12))
		}
	}
	if s.Messages.LastUserExcerpt != "" {
		fmt.Fprintf(&b, "  last: %q\n", truncateEnd(s.Messages.LastUserExcerpt, m.width-11))
	}
	return b.String()
}

func (m Model) renderNarrowEvidence(s session.Session, status session.Status) string {
	var parts []string
	if status.PercentElapsed != nil {
		parts = append(parts, fmt.Sprintf("TTL %.0f%%", *status.PercentElapsed))
	}
	if s.TokenStats.HitRate > 0 {
		parts = append(parts, fmt.Sprintf("hit %.0f%%", s.TokenStats.HitRate))
	}
	if s.Messages.LastUserExcerpt != "" {
		parts = append(parts, fmt.Sprintf("last: %q", truncateEnd(s.Messages.LastUserExcerpt, 28)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  " + strings.Join(parts, "  ") + "\n"
}

func (m Model) listFooter() string {
	if m.isEmptyListState() {
		return "refresh  ? help  q quit\n"
	}
	switch {
	case m.width < 90:
		return "cursor move  enter open  r remind  k keepalive  ? help  q quit\n"
	default:
		return "enter open focused  r remind  k keepalive  refresh  ? help  q quit\n"
	}
}

func (m Model) focusedAction() string {
	if m.route == RouteWorkspace {
		actions := m.workspaceFocusActions()
		if len(actions) == 0 {
			return ""
		}
		return actions[m.focusIndex%len(actions)]
	}
	if m.route == RouteList || m.route == RouteAmbiguous {
		if m.isEmptyListState() {
			if len(emptyFocusActions) == 0 {
				return ""
			}
			return emptyFocusActions[m.focusIndex%len(emptyFocusActions)]
		}
		if len(m.sessions) == 0 {
			if len(rootFocusActions) == 0 {
				return ""
			}
			return rootFocusActions[m.focusIndex%len(rootFocusActions)]
		}
		if m.focusIndex < len(m.sessions) {
			return "session"
		}
		actionIndex := m.focusIndex - len(m.sessions)
		if len(m.sessions) == 0 {
			actionIndex = m.focusIndex
		}
		if actionIndex >= 0 && actionIndex < len(rootFocusActions)-1 {
			return rootFocusActions[actionIndex+1]
		}
		if actionIndex >= 0 && actionIndex < len(rootFocusActions) {
			return rootFocusActions[actionIndex]
		}
	}
	if len(rootFocusActions) == 0 {
		return ""
	}
	return rootFocusActions[m.focusIndex%len(rootFocusActions)]
}

func (m Model) listFocusCount() int {
	if m.route == RouteWorkspace {
		return len(m.workspaceFocusActions())
	}
	if m.route != RouteList && m.route != RouteAmbiguous {
		return len(rootFocusActions)
	}
	if m.isEmptyListState() {
		return len(emptyFocusActions)
	}
	if len(m.sessions) == 0 {
		return len(rootFocusActions)
	}
	return len(m.sessions) + len(rootFocusActions) - 1
}

func (m *Model) moveFocus(delta int) {
	count := m.listFocusCount()
	if count == 0 {
		return
	}
	m.focusIndex = (m.focusIndex + delta + count) % count
	if m.route == RouteWorkspace {
		return
	}
	if m.focusIndex < len(m.sessions) {
		m.selectedIndex = m.focusIndex
		m.selectedID = m.sessions[m.selectedIndex].SessionID
	}
}

func (m Model) selectedSession() *session.Session {
	if len(m.sessions) == 0 {
		return nil
	}
	index := m.selectedIndex
	if index < 0 || index >= len(m.sessions) {
		index = 0
	}
	return &m.sessions[index]
}

func (m Model) isEmptyListState() bool {
	return m.refresh.EmptyState != EmptyNone
}

func (m *Model) toggleReminderForSelected() {
	m.lastAction = "toggle_reminder"
	selected := m.selectedSession()
	if selected == nil {
		return
	}
	m.reminderEnabled[selected.SessionID] = !m.reminderEnabled[selected.SessionID]
}

func (m *Model) toggleKeepAliveForSelected() {
	m.lastAction = "toggle_keepalive"
	selected := m.selectedSession()
	if selected == nil {
		return
	}
	state := m.KeepAliveState(selected.SessionID)
	if state.State != keepalive.StateOff && state.State != "" {
		m.keepAliveManager.Disable(selected.SessionID)
		m.keepAliveEnabled[selected.SessionID] = false
		m.focusIndex = m.defaultFocusIndex()
		return
	}
	actions := m.keepAliveManager.Enable(*selected, m.now)
	m.keepAliveEnabled[selected.SessionID] = true
	if m.deps.CheckClaudeAvailable != nil && m.KeepAliveState(selected.SessionID).AutoSend {
		if err := m.deps.CheckClaudeAvailable(); err != nil {
			m.keepAliveManager.CheckAvailability(selected.SessionID, availabilityChecker{err: err})
			m.refresh.ClaudeUnavailableMessage = err.Error()
		}
	}
	m.applyKeepAliveActions(actions, nil)
	m.focusIndex = m.defaultFocusIndex()
}

func (m Model) shouldShowClaudeUnavailable() bool {
	if m.refresh.ClaudeUnavailableMessage == "" {
		return false
	}
	if m.keepAliveStatus != KeepAliveInactive {
		return true
	}
	if m.route == RouteWorkspace {
		state := m.activeKeepAliveState()
		if state.State != "" && state.State != keepalive.StateOff {
			return true
		}
	}
	for _, enabled := range m.keepAliveEnabled {
		if enabled {
			return true
		}
	}
	return false
}

func (m Model) refreshQuery() string {
	if m.ambiguousID != "" {
		return m.ambiguousID
	}
	if m.selectedID != "" {
		return m.selectedID
	}
	return ""
}

func displayID(s session.Session) string {
	if s.ShortID != "" {
		return s.ShortID
	}
	return truncateMiddle(s.SessionID, 8)
}

func cacheLabel(s session.Session) string {
	if s.CacheWindow.Label != "" {
		return s.CacheWindow.Label
	}
	if !s.CacheWindow.Known {
		return "TTL ?"
	}
	return string(s.CacheWindow.Tier)
}

func formatStatusTime(status session.Status) string {
	switch {
	case status.RemainingSeconds != nil:
		return formatDuration(*status.RemainingSeconds)
	case status.ExpiredSeconds != nil:
		return formatDuration(*status.ExpiredSeconds) + " ago"
	case status.LastMessageAt == nil:
		return "no timestamp"
	default:
		return "no TTL"
	}
}

func formatDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	duration := time.Duration(seconds) * time.Second
	hours := int(duration / time.Hour)
	duration -= time.Duration(hours) * time.Hour
	minutes := int(duration / time.Minute)
	duration -= time.Duration(minutes) * time.Minute
	remainingSeconds := int(duration / time.Second)
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, remainingSeconds)
	}
	return fmt.Sprintf("%dm%02ds", minutes, remainingSeconds)
}

func formatModified(modified time.Time) string {
	if modified.IsZero() {
		return "unknown"
	}
	return modified.Format("15:04")
}

func compactKeepAliveSummary(m Model, s session.Session) string {
	if m.keepAliveStatus == KeepAliveCountdown {
		if seconds := m.countdowns[s.SessionID]; seconds > 0 {
			return fmt.Sprintf("KA %ds", seconds)
		}
	}
	if m.keepAliveEnabled[s.SessionID] {
		return "KA on"
	}
	return ""
}

func keepAliveSummary(m Model, s session.Session) string {
	if m.keepAliveStatus == KeepAliveCountdown {
		if seconds := m.countdowns[s.SessionID]; seconds > 0 {
			return fmt.Sprintf("KeepAlive countdown %ds", seconds)
		}
	}
	if m.keepAliveEnabled[s.SessionID] {
		return "KeepAlive on"
	}
	return ""
}

func truncateEnd(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func truncateMiddle(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	front := (max - 3) / 2
	back := max - 3 - front
	return value[:front] + "..." + value[len(value)-back:]
}
