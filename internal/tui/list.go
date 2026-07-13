package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rcverse/cc-watch/internal/keepalive"
	"github.com/rcverse/cc-watch/internal/refresh"
	"github.com/rcverse/cc-watch/internal/session"
)

const listPageSize = 4

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

	switch m.refresh.EmptyState {
	case EmptyLoading:
		b.WriteString(m.emptyStateBlock("Loading sessions", "Reading Claude's cache trail now.", ""))
	case EmptyProjectsDir:
		b.WriteString(m.emptyStateBlock("No projects directory", "Claude has not written cache history here yet.", m.refresh.ProjectsDir))
	case EmptyNoSessions:
		b.WriteString(m.emptyStateBlock("No sessions found", "Sessions appear after Claude Code writes JSONL files.", m.refresh.ProjectsDir))
	default:
		start, end := m.listVisibleRange()
		for i := start; i < end; i++ {
			s := m.sessions[i]
			b.WriteString(m.renderListRow(i, s))
		}
		b.WriteString(m.listPaginationLine())
	}

	b.WriteString(m.listFooter())
	return b.String()
}

func (m Model) listHeader() string {
	styles := DefaultStyles()
	width := m.listWidth()
	title := truncateANSI(styles.Render(RoleIdentity, "Claude Code Watch"), width)
	return "\n" + title + "\n" + styles.Render(RoleSeparator, strings.Repeat("─", max(min(width, 68)-2, 12))) + "\n"
}

func (m Model) listDegradedBanner() string {
	var messages []string
	deliveredOnly := false
	if len(m.refresh.Watcher.Messages) > 0 {
		message := m.refresh.Watcher.Messages[0]
		if m.refresh.Watcher.Status != "" && m.refresh.Watcher.Status != refresh.StatusOK {
			message = "watcher " + string(m.refresh.Watcher.Status) + ": " + message
		}
		messages = append(messages, message)
	}
	if m.refresh.NotificationDegraded != "" {
		messages = append(messages, "notifications degraded: "+m.refresh.NotificationDegraded)
	}
	if m.lastNotification != nil {
		status := *m.lastNotification
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
	width := m.listWidth()
	for _, message := range messages {
		lines = append(lines, styles.Render(RoleWarning, truncateEnd(prefix+message, max(width-2, 20))))
	}
	return strings.Join(lines, "\n")
}

func (m Model) emptyStateBlock(title string, body string, path string) string {
	styles := DefaultStyles()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s\n", styles.Render(RoleIdentity, title))
	fmt.Fprintf(&b, "     %s\n", body)
	if path != "" {
		fmt.Fprintf(&b, "     %s  %s\n", styles.Render(RoleMuted, "path"), truncateMiddle(path, max(m.listWidth()-17, 16)))
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
	case "quit":
		label = "Quit"
	}
	return fmt.Sprintf("  %s %-8s %s", marker, label, emptyActionDetail(action))
}

func emptyActionDetail(action string) string {
	switch action {
	case "refresh":
		return "check again"
	case "quit":
		return "close cc-watch"
	default:
		return ""
	}
}

func (m Model) ambiguousListView() string {
	var b strings.Builder
	styles := DefaultStyles()
	width := m.listWidth()
	b.WriteString(styles.Render(RoleIdentity, "Claude Code Watch / choose session"))
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleSeparator, strings.Repeat("─", max(min(width, 68)-2, 12))))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  partial id %s matched more than one session\n\n", styles.Render(RoleWarning, m.refreshQuery()))
	for i, s := range m.sessions {
		b.WriteString(m.renderListRow(i, s))
	}
	b.WriteString(cueLine("↑↓ move  ↵ open selected  ⎋ list  q quit"))
	return b.String()
}

func (m Model) renderListRow(index int, s session.Session) string {
	var b strings.Builder
	styles := DefaultStyles()
	width := m.listWidth()
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
	if width >= 100 {
		projectWidth = 28
	}
	if width >= 120 {
		projectWidth = 36
	}
	rowNumber := fmt.Sprintf("#%d", index+1)
	project := styles.Render(RoleIdentity, truncateMiddle(s.Project, projectWidth))
	identity := fmt.Sprintf("%s %s %s  %s  %s  %s",
		marker,
		padANSI(rowNumber, 4),
		padANSI(id, 12),
		styles.Render(RoleSeparator, "·"),
		padANSI(project, projectWidth),
		padANSI(cacheDisplay(s), 12),
	)
	identity = appendChips(identity, []string{keepAliveChip(m, s), reminderChip(m, s)}, width)
	b.WriteString(truncateANSI(identity, width))
	b.WriteString("\n")

	statusLine := fmt.Sprintf("  %s  %s  hit %.0f%%",
		padANSI(sessionStatusText(status, m.now), 24),
		progressSummary(status),
		s.TokenStats.HitRate,
	)
	if width >= 120 {
		if s.DurationSeconds != nil {
			statusLine += "  duration " + formatDuration(*s.DurationSeconds)
		}
		if s.WarningCount > 0 {
			statusLine += fmt.Sprintf("  warnings %d", s.WarningCount)
		}
	}
	b.WriteString(truncateANSI(statusLine, width))
	b.WriteString("\n")

	firstExcerpt := displayExcerpt(s.Messages.FirstUserExcerpt)
	lastExcerpt := displayExcerpt(s.Messages.LastUserExcerpt)
	if firstExcerpt != "" {
		fmt.Fprintf(&b, "  %s %s\n", messageChip("first"), messageText(truncateEnd(firstExcerpt, max(width-16, 18))))
	}
	if lastExcerpt != "" {
		available := max(width-13, 18)
		if width < 90 {
			available = max(width-13, 18)
		}
		fmt.Fprintf(&b, "  %s %s\n", messageChip("last"), messageText(truncateEnd(lastExcerpt, available)))
	}
	if s.WarningCount > 0 && width < 120 {
		fmt.Fprintf(&b, "     %s\n", styles.Render(RoleWarning, fmt.Sprintf("! %d parse warning(s)", s.WarningCount)))
	}
	b.WriteString("\n")
	return b.String()
}

func (m Model) listVisibleRange() (int, int) {
	if len(m.sessions) == 0 {
		return 0, 0
	}
	selected := m.selectedIndex
	if selected < 0 {
		selected = 0
	}
	if selected >= len(m.sessions) {
		selected = len(m.sessions) - 1
	}
	start := selected / listPageSize * listPageSize
	end := start + listPageSize
	if end > len(m.sessions) {
		end = len(m.sessions)
	}
	return start, end
}

func (m Model) listPaginationLine() string {
	if len(m.sessions) <= listPageSize {
		return ""
	}
	start, _ := m.listVisibleRange()
	page := start/listPageSize + 1
	pages := (len(m.sessions) + listPageSize - 1) / listPageSize
	styles := DefaultStyles()
	prev := styles.Render(RoleMuted, "Prev")
	if page > 1 {
		prev = styles.Render(RoleIdentity, "< Prev")
	}
	next := styles.Render(RoleMuted, "Next")
	if page < pages {
		next = styles.Render(RoleIdentity, "Next >")
	}
	line := fmt.Sprintf("Page %d/%d  %s  %s", page, pages, prev, next)
	return truncateANSI(line, m.listWidth()) + "\n\n"
}

func (m Model) listWidth() int {
	return max(min(m.width, maxVisualLineWidth), 24)
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
		return padANSI(styles.Render(RoleSuccess, "● Active"), 12) + when + " left"
	case session.StatusExpired:
		if status.LastMessageAt != nil {
			when = formatStatusDuration(int(now.Sub(*status.LastMessageAt).Seconds())) + " ago"
		}
		return padANSI(styles.Render(RoleDanger, "× Expired"), 12) + when
	case session.StatusUnknown:
		return padANSI(styles.Render(RoleMuted, "○ Unknown"), 12) + when
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
	if s.StatusAt(m.now).State == session.StatusExpired {
		return styles.Render(RoleDisabled, "remind N/A")
	}
	if m.reminderEnabled[s.SessionID] {
		return styles.Render(RoleReminder, "remind") + " " + styles.Render(RoleSuccess, "ON")
	}
	return styles.Render(RoleReminder, "remind") + " " + styles.Render(RoleMuted, "off")
}

func keepAliveChip(m Model, s session.Session) string {
	styles := DefaultStyles()
	if s.StatusAt(m.now).State == session.StatusExpired {
		return styles.Render(RoleDisabled, "KeepAlive N/A")
	}
	state := m.KeepAliveState(s.SessionID).State
	switch state {
	case keepalive.StateCountdown:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "countdown")
	case keepalive.StatePaused:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "paused")
	case keepalive.StateSending:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "sending")
	case keepalive.StateConfirming:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleWarning, "confirming")
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleDanger, "failed")
	case keepalive.StateMonitoringIdle, keepalive.StateScopeComplete:
		return styles.Render(RoleKeepAlive, "KeepAlive") + " " + styles.Render(RoleSuccess, "ON")
	default:
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
	return ansi.Truncate(value, max, "...")
}

func (m Model) listFooter() string {
	if m.isEmptyListState() {
		return cueLine("↵ act  u update  q quit")
	}
	pageCue := ""
	if len(m.sessions) > listPageSize {
		pageCue = "  ←/→ page"
	}
	return cueLine("↑↓ select" + pageCue + "  ↵ open  r remind  k KeepAlive  u update  c config  q quit")
}

func cueLine(text string) string {
	return DefaultStyles().Render(RoleMuted, text) + "\n"
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
	case RouteStatusline:
		return focusItemsFromActions(statuslineFocusActions)
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

func (m *Model) moveListPage(delta int) {
	if len(m.sessions) <= listPageSize {
		return
	}
	start, _ := m.listVisibleRange()
	next := start + delta*listPageSize
	if next < 0 {
		next = 0
	}
	if next >= len(m.sessions) {
		lastPage := (len(m.sessions) - 1) / listPageSize * listPageSize
		next = lastPage
	}
	m.selectedIndex = next
	m.focusIndex = next
	m.selectedID = m.sessions[next].SessionID
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
	selected := m.selectedSession()
	if selected == nil {
		return
	}
	if selected.StatusAt(m.now).State == session.StatusExpired {
		m.reminderEnabled[selected.SessionID] = false
		m.setNotice("Reminder N/A after expiry", RoleMuted, 3*time.Second)
		return
	}
	m.reminderEnabled[selected.SessionID] = !m.reminderEnabled[selected.SessionID]
}

func (m *Model) toggleKeepAliveForSelected() tea.Cmd {
	selected := m.selectedSession()
	if selected == nil {
		return nil
	}
	if reason := m.keepAliveUnavailableReason(*selected); reason != "" {
		m.disableKeepAlive(selected.SessionID)
		m.setNotice("KeepAlive N/A "+reason, RoleMuted, 3*time.Second)
		return nil
	}
	currentAction := m.FocusedAction()
	state := m.KeepAliveState(selected.SessionID)
	if state.State != keepalive.StateOff && state.State != "" {
		m.keepAliveManager.Disable(selected.SessionID)
		m.setNotice("KeepAlive cancelled", RoleInfo, 3*time.Second)
		m.restoreFocusAction(currentAction)
		return nil
	}
	actions := m.keepAliveManager.Enable(*selected, m.now)
	m.restoreFocusAction(currentAction)
	// CheckAvailability can overwrite the state actions above were computed
	// from, so treat the two as mutually exclusive rather than notifying
	// for both.
	if m.deps.CheckClaudeAvailable != nil {
		if err := m.deps.CheckClaudeAvailable(); err != nil {
			m.keepAliveManager.CheckAvailability(selected.SessionID, availabilityChecker{err: err})
			m.refresh.ClaudeUnavailableMessage = err.Error()
			return m.keepAliveLifecycleNotification(selected.SessionID, m.KeepAliveState(selected.SessionID))
		}
	}
	return tea.Batch(m.applyKeepAliveActions(actions, nil)...)
}

func (m Model) shouldShowClaudeUnavailable() bool {
	if m.refresh.ClaudeUnavailableMessage == "" {
		return false
	}
	if m.route == RouteWorkspace {
		state := m.activeKeepAliveState()
		if state.State != "" && state.State != keepalive.StateOff {
			return true
		}
	}
	for _, s := range m.sessions {
		if m.KeepAliveEnabled(s.SessionID) {
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
		return formatStatusDuration(*status.RemainingSeconds)
	case status.ExpiredSeconds != nil:
		return formatStatusDuration(*status.ExpiredSeconds) + " ago"
	case status.LastMessageAt == nil:
		return "no timestamp"
	default:
		return "no TTL"
	}
}

func formatStatusDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	days := seconds / 86400
	hours := seconds % 86400 / 3600
	minutes := seconds % 3600 / 60
	remainingSeconds := seconds % 60
	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm%02ds", minutes, remainingSeconds)
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
