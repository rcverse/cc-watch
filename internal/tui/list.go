package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
)

func (m Model) listView() string {
	var b strings.Builder
	if m.route == RouteAmbiguous {
		return m.ambiguousListView()
	}

	b.WriteString(m.listHeader())
	if banner := m.listDegradedBanner(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n")
	}
	if banner := m.actionBanner(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	switch m.refresh.EmptyState {
	case EmptyLoading:
		b.WriteString(m.emptyStateBlock("Loading sessions", "Reading Claude's cache trail now.", ""))
	case EmptyProjectsDir:
		b.WriteString(m.emptyStateBlock("No projects directory", "Claude has not written cache history here yet.", m.refresh.ProjectsDir))
	case EmptyNoSessions:
		b.WriteString(m.emptyStateBlock("No sessions found", "Sessions appear after Claude Code writes JSONL files.", m.refresh.ProjectsDir))
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

func (m Model) listHeader() string {
	styles := DefaultStyles()
	status := m.refresh.Watcher.Status
	if status == "" {
		status = "ok"
	}
	parts := []string{
		styles.Render(RoleIdentity, "Claude Code Cache"),
		styles.Render(RoleMuted, fmt.Sprintf("%d sessions", len(m.sessions))),
	}
	if status != "" && status != refresh.StatusOK {
		parts = append(parts, styles.Render(RoleWarning, "watcher "+string(status)))
	} else {
		parts = append(parts, styles.Render(RoleMuted, "live"))
	}
	parts = append(parts, styles.Render(RoleMuted, "updated "+m.now.Local().Format("15:04")))
	line := strings.Join(parts, styles.Render(RoleSeparator, "  ·  "))
	return truncateANSI(line, m.width) + "\n" + styles.Render(RoleSeparator, strings.Repeat("─", maxInt(minInt(m.width, 68)-2, 12))) + "\n"
}

func (m Model) listDegradedBanner() string {
	var messages []string
	deliveredOnly := false
	if len(m.refresh.Watcher.Messages) > 0 {
		messages = append(messages, m.refresh.Watcher.Messages[0])
	}
	if m.refresh.NotificationDegraded != "" {
		messages = append(messages, "notifications degraded: "+m.refresh.NotificationDegraded)
	}
	if len(m.notificationStatuses) > 0 {
		status := m.notificationStatuses[0]
		switch {
		case status.Result.Degraded:
			messages = append(messages, "notification failed: "+status.Notification.Title)
			if status.Notification.Body != "" {
				messages = append(messages, status.Notification.Body)
			}
			if status.Result.Message != "" {
				messages = append(messages, "reason: "+status.Result.Message)
			}
		case status.Result.Delivered:
			deliveredOnly = len(messages) == 0
		}
	}
	if m.shouldShowClaudeUnavailable() {
		messages = append(messages, "claude unavailable: "+m.refresh.ClaudeUnavailableMessage)
	}
	if len(messages) == 0 {
		return ""
	}
	styles := DefaultStyles()
	prefix := "! "
	if deliveredOnly {
		prefix = "  "
	}
	var lines []string
	for _, message := range messages {
		lines = append(lines, styles.Render(RoleWarning, truncateEnd(prefix+message, maxInt(m.width-2, 20))))
	}
	return strings.Join(lines, "\n")
}

func (m Model) emptyStateBlock(title string, body string, path string) string {
	styles := DefaultStyles()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s\n", styles.Render(RoleIdentity, title))
	fmt.Fprintf(&b, "     %s\n", body)
	if path != "" {
		fmt.Fprintf(&b, "     %s  %s\n", styles.Render(RoleMuted, "path"), truncateMiddle(path, maxInt(m.width-17, 16)))
	}
	b.WriteString("\n")
	for _, action := range emptyFocusActions {
		fmt.Fprintf(&b, "%s\n", m.emptyActionRow(action))
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) emptyActionRow(action string) string {
	styles := DefaultStyles()
	marker := " "
	if m.FocusedAction() == action {
		marker = styles.Render(RoleIdentity, "›")
	}
	label := action
	switch action {
	case "refresh":
		label = "Refresh"
	case "help":
		label = "Help"
	case "quit":
		label = "Quit"
	}
	return fmt.Sprintf("  %s %-8s %s", marker, label, emptyActionDetail(action))
}

func emptyActionDetail(action string) string {
	switch action {
	case "refresh":
		return "check again"
	case "help":
		return "show key hints"
	case "quit":
		return "close cc-cache"
	default:
		return ""
	}
}

func (m Model) ambiguousListView() string {
	var b strings.Builder
	styles := DefaultStyles()
	b.WriteString(styles.Render(RoleIdentity, "Claude Code Cache / choose session"))
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleSeparator, strings.Repeat("─", maxInt(minInt(m.width, 68)-2, 12))))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  partial id %s matched more than one session\n\n", styles.Render(RoleWarning, m.refreshQuery()))
	for i, s := range m.sessions {
		b.WriteString(m.renderListRow(i, s))
	}
	b.WriteString("up/down move  enter open selected  esc list  q quit\n")
	return b.String()
}

func (m Model) renderListRow(index int, s session.Session) string {
	var b strings.Builder
	styles := DefaultStyles()
	selected := m.focusIndex == index
	marker := " "
	if selected {
		marker = styles.Render(RoleIdentity, "›")
	}
	status := s.StatusAt(m.now)
	id := displayID(s)
	if selected {
		id = styles.Render(RoleIdentity, id)
	}
	projectWidth := 18
	if m.width >= 100 {
		projectWidth = 28
	}
	if m.width >= 120 {
		projectWidth = 36
	}
	identity := fmt.Sprintf("  %s #%d  %s  %s  %s  %s",
		marker,
		index+1,
		id,
		styles.Render(RoleSeparator, "·"),
		styles.Render(RoleIdentity, truncateMiddle(s.Project, projectWidth)),
		cacheDisplay(s),
	)
	identity = appendChips(identity, []string{keepAliveChip(m, s), reminderChip(m, s)}, m.width)
	b.WriteString(truncateANSI(identity, m.width))
	b.WriteString("\n")

	statusLine := fmt.Sprintf("     %s  %s  hit %.0f%%",
		sessionStatusText(status, m.now),
		progressSummary(status),
		s.TokenStats.HitRate,
	)
	if m.width >= 120 {
		if s.DurationSeconds != nil {
			statusLine += "  duration " + formatDuration(*s.DurationSeconds)
		}
		if len(s.Warnings) > 0 {
			statusLine += fmt.Sprintf("  warnings %d", len(s.Warnings))
		}
	}
	b.WriteString(truncateANSI(statusLine, m.width))
	b.WriteString("\n")

	firstExcerpt := displayExcerpt(s.Messages.FirstUserExcerpt)
	lastExcerpt := displayExcerpt(s.Messages.LastUserExcerpt)
	if m.width >= 96 && firstExcerpt != "" {
		fmt.Fprintf(&b, "    %s  %s\n", styles.Render(RoleExcerptLabel, "first"), truncateEnd(firstExcerpt, maxInt(m.width-13, 18)))
	}
	if lastExcerpt != "" {
		label := "last"
		available := maxInt(m.width-13, 18)
		if m.width < 90 {
			label = "last"
			available = maxInt(m.width-13, 18)
		}
		fmt.Fprintf(&b, "    %s  %s\n", styles.Render(RoleExcerptLabel, label), truncateEnd(lastExcerpt, available))
	}
	if len(s.Warnings) > 0 && m.width < 120 {
		fmt.Fprintf(&b, "     %s\n", styles.Render(RoleWarning, fmt.Sprintf("! %d parse warning(s)", len(s.Warnings))))
	}
	b.WriteString("\n")
	return b.String()
}

func cacheDisplay(s session.Session) string {
	switch cacheLabel(s) {
	case "1h":
		return "1-hour cache"
	case "5m":
		return "5-min cache"
	case "TTL ?":
		return "TTL unknown"
	default:
		return cacheLabel(s) + " cache"
	}
}

func sessionStatusText(status session.Status, now time.Time) string {
	styles := DefaultStyles()
	when := formatStatusTime(status)
	switch status.State {
	case session.StatusActive:
		return styles.Render(RoleSuccess, "active") + " " + when + " left"
	case session.StatusExpired:
		if status.LastMessageAt != nil {
			when = formatDuration(int(now.Sub(*status.LastMessageAt).Seconds())) + " ago"
		}
		return styles.Render(RoleDanger, "expired") + " " + when
	case session.StatusUnknown:
		return styles.Render(RoleDisabled, "unknown") + " " + when
	default:
		return string(status.State) + " " + when
	}
}

func progressSummary(status session.Status) string {
	if status.PercentElapsed == nil {
		return "no TTL"
	}
	percent := cappedPercent(*status.PercentElapsed)
	return fmt.Sprintf("%s %.0f%%", ProgressBar(percent, 14), percent)
}

func cappedPercent(percent float64) float64 {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func reminderChip(m Model, s session.Session) string {
	styles := DefaultStyles()
	if m.reminderEnabled[s.SessionID] {
		return styles.Render(RoleReminder, "remind") + " " + styles.Render(RoleSuccess, "ON")
	}
	return styles.Render(RoleReminder, "remind") + " " + styles.Render(RoleMuted, "off")
}

func keepAliveChip(m Model, s session.Session) string {
	styles := DefaultStyles()
	state := m.KeepAliveState(s.SessionID).State
	switch state {
	case keepalive.StateCountdown:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "countdown")
	case keepalive.StateManualReady:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "ready")
	case keepalive.StateSending:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "sending")
	case keepalive.StateConfirming:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "confirming")
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleDanger, "failed")
	case keepalive.StateMonitoringIdle, keepalive.StateSuccess, keepalive.StateScopeComplete:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleSuccess, "ON")
	default:
		if m.keepAliveEnabled[s.SessionID] {
			return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleSuccess, "ON")
		}
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleMuted, "off")
	}
}

func appendChips(base string, chips []string, width int) string {
	line := base
	separator := DefaultStyles().Render(RoleSeparator, "  ·  ")
	for _, chip := range chips {
		if chip == "" {
			continue
		}
		candidate := line + separator + chip
		if visibleWidth(stripANSI(candidate)) <= width {
			line = candidate
		}
	}
	return line
}

func displayExcerpt(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncateANSI(value string, max int) string {
	if max <= 0 || visibleWidth(stripANSI(value)) <= max {
		return value
	}
	return truncateEnd(stripANSI(value), max)
}

func (m Model) listFooter() string {
	if m.isEmptyListState() {
		return "enter act  u update  ? help  q quit\n"
	}
	switch {
	case m.width < 90:
		return "↑↓ select  enter open  r remind  k KeepAlive  u update  c config  ? help  q quit\n"
	default:
		return "↑↓ select  enter open  r remind  k KeepAlive  u update  c config  ? help  q quit\n"
	}
}

func (m Model) focusedAction() string {
	items := m.focusItems()
	if len(items) == 0 {
		return ""
	}
	return items[m.focusIndex%len(items)].action
}

func (m Model) listFocusCount() int {
	return len(m.focusItems())
}

func (m Model) focusItems() []focusItem {
	switch m.route {
	case RouteWorkspace:
		return focusItemsFromActions(m.workspaceFocusActions())
	case RouteConfig:
		return focusItemsFromActions(configFocusActions)
	case RouteList, RouteAmbiguous:
		if m.isEmptyListState() {
			return focusItemsFromActions(emptyFocusActions)
		}
		items := make([]focusItem, len(m.sessions))
		for i := range items {
			items[i] = focusItem{action: "session"}
		}
		return items
	default:
		return nil
	}
}

func focusItemsFromActions(actions []string) []focusItem {
	items := make([]focusItem, 0, len(actions))
	for _, action := range actions {
		items = append(items, focusItem{action: action})
	}
	return items
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
	if m.route != RouteList && m.route != RouteAmbiguous {
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
	if reason := m.keepAliveUnavailableReason(*selected); reason != "" {
		m.disableKeepAlive(selected.SessionID)
		m.lastAction = "keepalive_unavailable_expired"
		m.setNotice("KeepAlive "+reason, RoleWarning, 3*time.Second)
		return
	}
	currentAction := m.FocusedAction()
	state := m.KeepAliveState(selected.SessionID)
	if state.State != keepalive.StateOff && state.State != "" {
		m.keepAliveManager.Disable(selected.SessionID)
		m.keepAliveEnabled[selected.SessionID] = false
		m.setNotice("KeepAlive cancelled", RoleInfo, 3*time.Second)
		m.restoreFocusAction(currentAction)
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
	m.restoreFocusAction(currentAction)
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

func truncateEnd(value string, max int) string {
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
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
