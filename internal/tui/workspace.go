package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/session"
)

const rewindWindowActiveLimit = 6

type rewindWindowRow struct {
	message session.MessageWindow
	status  session.StatusState
	seconds int
}

func (m Model) workspaceView() string {
	selected := m.selectedSession()
	if selected == nil {
		var b strings.Builder
		b.WriteString("cc-watch workspace\nNo selected session.\nb/esc back  q quit\n")
		return b.String()
	}

	var b strings.Builder
	styles := DefaultStyles()
	title := fmt.Sprintf("Claude Code Watch / %s / %s", selected.Project, displayID(*selected))
	b.WriteString("\n")
	b.WriteString(truncateANSI(styles.Render(RoleIdentity, title), m.width))
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleSeparator, strings.Repeat("─", max(min(m.width, 76)-2, 12))))
	b.WriteString("\n")
	if banner := m.listDegradedBanner(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n")
	}
	panelGap := m.workspacePanelGap()
	b.WriteString(m.cacheStatusCard(*selected))
	b.WriteString(panelGap)
	b.WriteString(m.sessionInfoCard(*selected))
	if card := m.activeKeepAliveCard(*selected); card != "" {
		b.WriteString(panelGap)
		b.WriteString(card)
	}
	b.WriteString(panelGap)
	b.WriteString(m.workspaceControls(*selected))
	b.WriteString(m.workspaceFooter())
	return b.String()
}

func (m Model) cacheStatusCard(s session.Session) string {
	styles := DefaultStyles()
	status := s.StatusAt(m.now)
	var b strings.Builder
	state := workspaceStatusLabel(status)
	if state == "" {
		state = "UNKNOWN"
	}
	ttlLine := fmt.Sprintf("%s %s", padANSI(styles.Render(statusRole(status), state), 10), cacheTierText(s))
	if status.PercentElapsed != nil {
		percent := cappedPercent(*status.PercentElapsed)
		ttlLine = fmt.Sprintf("%s %s  %.0f%%  %s  %s", padANSI(styles.Render(statusRole(status), state), 10), ProgressBar(percent, 20), percent, cacheStatusTime(status), cacheTierText(s))
	}
	fmt.Fprintf(&b, "%s\n", truncateANSI(ttlLine, max(m.width-4, 20)))
	return m.renderWorkspacePanel("Cache Status", b.String())
}

func (m Model) sessionInfoCard(s session.Session) string {
	if m.sessionInfoExpanded {
		return m.sessionInfoDetailsCard(s)
	}
	if m.compactOperationalWorkspace() {
		return m.compactSessionInfoCard(s)
	}
	var b strings.Builder
	styles := DefaultStyles()
	fmt.Fprintf(&b, "%s   %s\n", styles.Render(RoleMuted, "Session ID"), truncateMiddle(s.SessionID, max(m.width-18, 24)))
	fmt.Fprintf(&b, "%s\n", workspaceMessageLine(styles.Render(RoleMuted, "Messages")+"     ", "first", s.Messages.FirstUserExcerpt, m.workspacePanelWidth()))
	fmt.Fprintf(&b, "%s\n", workspaceMessageLine("             ", "last", s.Messages.LastUserExcerpt, m.workspacePanelWidth()))
	fmt.Fprintf(&b, "%s       writes %d  reads %d  hit %s %.0f%%\n", styles.Render(RoleMuted, "Tokens"), s.TokenStats.CacheWrites, s.TokenStats.CacheReads, HitRateProgressBar(s.TokenStats.HitRate, 8), s.TokenStats.HitRate)
	fmt.Fprintf(&b, "%s         %s  %s\n", styles.Render(RoleMuted, "Gaps"), gapSummary(s), styles.Render(RoleMuted, "v details"))
	return m.renderWorkspacePanel("Session Info", b.String())
}

func (m Model) compactSessionInfoCard(s session.Session) string {
	var b strings.Builder
	styles := DefaultStyles()
	fmt.Fprintf(&b, "%s   %s\n", styles.Render(RoleMuted, "Session ID"), truncateMiddle(s.SessionID, max(m.width-18, 24)))
	fmt.Fprintf(&b, "%s       writes %d  reads %d  hit %.0f%%\n", styles.Render(RoleMuted, "Tokens"), s.TokenStats.CacheWrites, s.TokenStats.CacheReads, s.TokenStats.HitRate)
	return m.renderWorkspacePanel("Session Info", b.String())
}

func (m Model) compactOperationalWorkspace() bool {
	state := m.activeKeepAliveState().State
	return m.width <= 90 && m.height <= 24 && (m.sessionInfoExpanded || (state != "" && state != keepalive.StateOff))
}

func (m Model) sessionInfoDetailsCard(s session.Session) string {
	var b strings.Builder
	title := "Session Info · details"
	styles := DefaultStyles()
	fmt.Fprintf(&b, "%s   %s\n", styles.Render(RoleMuted, "Session ID"), truncateMiddle(s.SessionID, max(m.width-18, 24)))
	fmt.Fprintf(&b, "%s       %s\n", styles.Render(RoleMuted, "JSONL"), truncateMiddle(s.JSONLPath, max(m.width-18, 24)))
	fmt.Fprintf(&b, "%s      parsed %s · file modified %s\n", styles.Render(RoleMuted, "Updated"), m.now.Local().Format("15:04:05"), s.FileModifiedAt.Local().Format("15:04:05"))
	fmt.Fprintf(&b, "%s\n", workspaceMessageLine(styles.Render(RoleMuted, "Messages")+"     ", "first", s.Messages.FirstUserExcerpt, m.workspacePanelWidth()))
	fmt.Fprintf(&b, "%s\n", workspaceMessageLine("             ", "last", s.Messages.LastUserExcerpt, m.workspacePanelWidth()))
	fmt.Fprintf(&b, "%s  writes %d · reads %d · hit %s %.0f%% · output %d\n", styles.Render(RoleMuted, "Token Stats"), s.TokenStats.CacheWrites, s.TokenStats.CacheReads, HitRateProgressBar(s.TokenStats.HitRate, 10), s.TokenStats.HitRate, s.TokenStats.OutputTokens)
	if section := m.rewindWindowsSection(s); section != "" {
		b.WriteString(section)
	}
	fmt.Fprintf(&b, "%s\n", styles.Render(RoleMuted, "Mid-session Gaps >1min · "+m.gapSortLabel()))
	gaps := m.visibleDetailGaps(s)
	if len(gaps) == 0 {
		fmt.Fprintf(&b, "%s\n", styles.Render(RoleMuted, "No mid-session gaps found."))
	} else {
		for _, gap := range gaps {
			fmt.Fprintf(&b, "%s\n", truncateANSI(formatGapLine(gap, s, m.gapSortNewest), max(m.width-4, 20)))
		}
	}
	if scroll := m.detailScrollAffordance(len(s.Gaps), len(gaps)); scroll != "" {
		fmt.Fprintf(&b, "%s\n", styles.Render(RoleMuted, scroll))
	}
	if s.ResetCount > 0 {
		fmt.Fprintf(&b, "%s %d cache reset(s) - rebuilt from scratch %d time(s).\n", styles.Render(RoleWarning, "!"), s.ResetCount, s.ResetCount)
	}
	return m.renderWorkspacePanelFocused(title, b.String(), m.FocusedAction() == "details_scroll")
}

func (m Model) rewindWindowsSection(s session.Session) string {
	styles := DefaultStyles()
	if len(s.RecentMessages) == 0 {
		return ""
	}
	if !s.CacheWindow.Known {
		return fmt.Sprintf("%s\n%s\n", styles.Render(RoleMuted, "Recent Message Cache Status"), styles.Render(RoleMuted, "TTL unknown; message cache status unavailable."))
	}
	rows := rewindWindowRows(s, m.now, rewindWindowActiveLimit)
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", styles.Render(RoleMuted, "Recent Message Cache Status"))
	for _, row := range rows {
		fmt.Fprintf(&b, "%s\n", formatRewindWindowRow(row, m.workspacePanelWidth()))
	}
	return b.String()
}

func rewindWindowRows(s session.Session, now time.Time, activeLimit int) []rewindWindowRow {
	if !s.CacheWindow.Known || s.CacheWindow.TTLSeconds <= 0 {
		return nil
	}
	var rows []rewindWindowRow
	var cutoff *rewindWindowRow
	for i := len(s.RecentMessages) - 1; i >= 0; i-- {
		message := s.RecentMessages[i]
		remaining := int(message.At.Add(time.Duration(s.CacheWindow.TTLSeconds) * time.Second).Sub(now).Seconds())
		if remaining >= 0 {
			if len(rows) < activeLimit {
				rows = append(rows, rewindWindowRow{message: message, status: session.StatusActive, seconds: remaining})
			}
			continue
		}
		if cutoff == nil {
			row := rewindWindowRow{message: message, status: session.StatusExpired, seconds: -remaining}
			cutoff = &row
		}
	}
	if cutoff != nil {
		rows = append(rows, *cutoff)
	}
	return rows
}

func formatRewindWindowRow(row rewindWindowRow, width int) string {
	styles := DefaultStyles()
	role := RoleSuccess
	chip := "● ACTIVE"
	status := formatStatusDuration(row.seconds) + " left"
	if row.status == session.StatusExpired {
		role = RoleDanger
		chip = "× EXPIRED"
		status = "expired " + formatStatusDuration(row.seconds)
	}
	prefixWidth := len("× EXPIRED") + 2 + len("15:04:05") + 2 + 13 + 2
	excerpt := truncateEnd(displayExcerpt(row.message.Excerpt), max(width-prefixWidth, 8))
	return fmt.Sprintf("%s  %s  %-13s  %s", styles.Render(role, chip), row.message.At.Local().Format("15:04:05"), status, messageText(excerpt))
}

func (m Model) renderWorkspacePanel(title string, body string) string {
	return m.renderWorkspacePanelFocused(title, body, false)
}

func (m Model) renderWorkspacePanelFocused(title string, body string, focused bool) string {
	width := m.workspacePanelWidth()
	title = DefaultStyles().Render(RoleIdentity, title)
	if focused {
		return RenderPanelWidthFocused(title, truncateBodyLines(body, width), width)
	}
	return RenderPanelWidth(title, truncateBodyLines(body, width), width)
}

func (m Model) workspacePanelWidth() int {
	return max(min(m.width-4, maxPanelBodyWidth), 24)
}

func (m Model) workspacePanelGap() string {
	if m.height >= 30 {
		return "\n"
	}
	return ""
}

func cacheStatusTime(status session.Status) string {
	switch status.State {
	case session.StatusActive:
		return formatStatusTime(status) + " left"
	default:
		return formatStatusTime(status)
	}
}

func workspaceStatusLabel(status session.Status) string {
	switch status.State {
	case session.StatusActive:
		return "● ACTIVE"
	case session.StatusExpired:
		return "× EXPIRED"
	case session.StatusUnknown:
		return "○ UNKNOWN"
	default:
		return strings.ToUpper(string(status.State))
	}
}

func cacheTierText(s session.Session) string {
	return DefaultStyles().Render(RoleCacheTier, cacheDisplay(s))
}

func messageLabel(label string) string {
	styles := DefaultStyles()
	if label == "last" {
		return styles.Render(RoleLastLabel, label)
	}
	return styles.Render(RoleFirstLabel, label)
}

func messageChip(label string) string {
	return padANSI(messageLabel(label), 5)
}

func workspaceMessageLine(prefix string, label string, text string, width int) string {
	chip := messageChip(label)
	available := max(width-visibleWidth(stripANSI(prefix))-visibleWidth(stripANSI(chip))-1, 8)
	excerpt := truncateEnd(emptyDash(displayExcerpt(text)), available)
	return fmt.Sprintf("%s%s %s", prefix, chip, messageText(excerpt))
}

func messageText(value string) string {
	return DefaultStyles().Render(RoleExcerptText, value)
}

func padANSI(value string, width int) string {
	padding := width - visibleWidth(stripANSI(value))
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func statusRole(status session.Status) StyleRole {
	switch status.State {
	case session.StatusActive:
		return RoleSuccess
	case session.StatusExpired:
		return RoleDanger
	default:
		return RoleWarning
	}
}

func gapSummary(s session.Session) string {
	if len(s.Gaps) == 0 {
		return "0 gaps · 0 resets"
	}
	longest := s.Gaps[0]
	latest := s.Gaps[0]
	for _, gap := range s.Gaps[1:] {
		if gap.Seconds > longest.Seconds {
			longest = gap
		}
		if gap.To.After(latest.To) {
			latest = gap
		}
	}
	return fmt.Sprintf("%d gaps · %d resets · longest %.0fs · latest %.0fs", len(s.Gaps), s.ResetCount, longest.Seconds, latest.Seconds)
}

func (m Model) gapSortLabel() string {
	if m.gapSortNewest {
		return "↕ newest"
	}
	return "↕ longest"
}

func (m Model) sortedGaps(s session.Session) []session.Gap {
	gaps := append([]session.Gap(nil), s.Gaps...)
	if m.gapSortNewest {
		sort.SliceStable(gaps, func(i, j int) bool {
			return gaps[i].To.After(gaps[j].To)
		})
		return gaps
	}
	sort.SliceStable(gaps, func(i, j int) bool {
		return gaps[i].Seconds > gaps[j].Seconds
	})
	return gaps
}

func (m Model) visibleDetailGaps(s session.Session) []session.Gap {
	gaps := m.sortedGaps(s)
	limit := m.detailGapLimit(s)
	if limit >= len(gaps) {
		return gaps
	}
	offset := m.detailsOffset
	if offset < 0 {
		offset = 0
	}
	if maxOffset := len(gaps) - limit; offset > maxOffset {
		offset = maxOffset
	}
	return gaps[offset : offset+limit]
}

func (m Model) detailGapLimit(s session.Session) int {
	limit := 3
	if m.height <= 24 {
		limit = 1
	}
	if m.compactOperationalWorkspace() {
		limit = 1
	}
	if m.height >= 30 {
		limit = 6
	}
	if limit > len(s.Gaps) {
		return len(s.Gaps)
	}
	if limit < 0 {
		return 0
	}
	return limit
}

func (m Model) detailsCanScroll(delta int) bool {
	if !m.sessionInfoExpanded {
		return false
	}
	selected := m.selectedSession()
	if selected == nil {
		return false
	}
	limit := m.detailGapLimit(*selected)
	if len(selected.Gaps) <= limit {
		return false
	}
	next := m.detailsOffset + delta
	return next >= 0 && next <= len(selected.Gaps)-limit
}

func (m Model) detailScrollAffordance(total int, visible int) string {
	above := m.detailsOffset
	below := total - m.detailsOffset - visible
	switch {
	case above > 0 && below > 0:
		return fmt.Sprintf("↑ %d above · ↓ %d below; use ↑↓ scroll", above, below)
	case below > 0:
		return fmt.Sprintf("↓ %d more gap(s) below; use ↑↓ scroll", below)
	case above > 0:
		return fmt.Sprintf("↑ %d earlier gap(s) above; use ↑↓ scroll", above)
	default:
		return ""
	}
}

func formatGapLine(gap session.Gap, s session.Session, newest bool) string {
	styles := DefaultStyles()
	prefix := "•"
	label := "pause"
	role := RoleInfo
	if gap.Reset {
		prefix = "!"
		label = "RESET"
		role = RoleDanger
	}
	tag := ""
	if !newest {
		longest := true
		for _, candidate := range s.Gaps {
			if candidate.Seconds > gap.Seconds {
				longest = false
				break
			}
		}
		if longest {
			tag = "   longest"
		}
	} else {
		latest := true
		for _, candidate := range s.Gaps {
			if candidate.To.After(gap.To) {
				latest = false
				break
			}
		}
		if latest {
			tag = "   latest"
		}
	}
	return fmt.Sprintf("%s %5.0fs    %s -> %s%s", styles.Render(role, fmt.Sprintf("%s %-5s", prefix, label)), gap.Seconds, gap.From.Local().Format("15:04:05"), gap.To.Local().Format("15:04:05"), tag)
}

func (m Model) workspaceControls(s session.Session) string {
	var b strings.Builder
	for _, action := range m.workspaceControlActions(s) {
		fmt.Fprintf(&b, "%s\n", m.controlRow(action.id, action.label, action.value, action.detail))
	}
	if m.notice.Message != "" {
		fmt.Fprintf(&b, "\n%s\n", m.controlRow("", "Notice", "", m.notice.Message))
	}
	return m.renderWorkspacePanel("Controls", b.String())
}

type workspaceControlAction struct {
	id     string
	label  string
	value  string
	detail string
}

func (m Model) workspaceControlActions(s session.Session) []workspaceControlAction {
	state := m.KeepAliveState(s.SessionID)
	var actions []workspaceControlAction
	if m.keepAliveUnavailableReason(s) == "" {
		switch state.State {
		case keepalive.StateCountdown:
			actions = append(actions,
				workspaceControlAction{id: "keepalive_send_now", label: "Send now", detail: "send KeepAlive message now"},
				workspaceControlAction{id: "keepalive_cancel", label: "Dismiss", detail: "cancel this countdown"},
			)
		case keepalive.StateManualReady:
			actions = append(actions,
				workspaceControlAction{id: "keepalive_send_now", label: "Send now", detail: "send KeepAlive message now"},
				workspaceControlAction{id: "keepalive_cancel", label: "Dismiss", detail: "close this prompt"},
			)
		case keepalive.StateSending, keepalive.StateConfirming:
			actions = append(actions, workspaceControlAction{id: "keepalive_stop_waiting", label: "Stop waiting", detail: "cancel confirmation wait"})
		case keepalive.StateSuccess, keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout, keepalive.StateScopeComplete:
			actions = append(actions, workspaceControlAction{id: "keepalive_acknowledge", label: "Acknowledge", detail: "clear KeepAlive status"})
		}
	}

	reminderState := offText()
	reminderDetail := fmt.Sprintf("notify at %s", thresholdSummary(m.reminderThresholds))
	if s.StatusAt(m.now).State == session.StatusExpired {
		reminderState = DefaultStyles().Render(RoleDisabled, "N/A")
		reminderDetail = "after expiry"
	} else if m.reminderEnabled[s.SessionID] {
		reminderState = onText(false)
	}
	actions = append(actions, workspaceControlAction{id: "reminder", label: "Reminder", value: reminderState, detail: reminderDetail})

	if reason := m.keepAliveUnavailableReason(s); reason != "" {
		actions = append(actions,
			workspaceControlAction{id: "keepalive", label: "KeepAlive", value: DefaultStyles().Render(RoleDisabled, "N/A"), detail: reason},
			workspaceControlAction{id: "keepalive_autosend", label: "Auto-send", value: DefaultStyles().Render(RoleDisabled, "N/A"), detail: reason},
		)
	} else if state.State == keepalive.StateOff || state.State == "" {
		actions = append(actions,
			workspaceControlAction{id: "keepalive", label: "KeepAlive", value: offText(), detail: fmt.Sprintf("%dm before expiry · %s", m.keepAliveConfig.TriggerBeforeExpiryMinutes, scopeLabel(m.keepAliveConfig.Scope.MaxSends))},
			workspaceControlAction{id: "keepalive_autosend", label: "Auto-send", value: onOffText(state.AutoSend, true), detail: autoSendWorkspaceDetail(state.AutoSend)},
		)
	} else {
		actions = append(actions,
			workspaceControlAction{id: "keepalive", label: "KeepAlive", value: keepAliveControlState(state), detail: keepAliveControlDetail(state)},
			workspaceControlAction{id: "keepalive_autosend", label: "Auto-send", value: onOffText(state.AutoSend, true), detail: autoSendWorkspaceDetailForState(state)},
		)
	}

	if !m.compactOperationalWorkspace() && !m.sessionInfoExpanded {
		actions = append(actions,
			workspaceControlAction{id: "back", label: "Back", detail: "session list"},
		)
	}
	return actions
}

func (m Model) controlRow(action string, label string, state string, detail string) string {
	styles := DefaultStyles()
	marker := " "
	if m.FocusedAction() == action {
		marker = styles.Render(RoleIdentity, "›")
	}
	return fmt.Sprintf("  %s %-9s %s %s", marker, label, padANSI(state, 4), styles.Render(RoleMuted, detail))
}

func onText(risky bool) string {
	role := RoleSuccess
	if risky {
		role = RoleWarning
	}
	return DefaultStyles().Render(role, "ON")
}

func offText() string {
	return DefaultStyles().Render(RoleMuted, "off")
}

func onOffText(enabled bool, risky bool) string {
	if enabled {
		return onText(risky)
	}
	return offText()
}

func (m Model) workspaceFooter() string {
	if m.sessionInfoExpanded {
		if m.FocusedAction() == "details_scroll" {
			if m.width <= 90 {
				return cueLine("↑↓ scroll  v collapse  s sort  b/⎋ back  q quit")
			}
			return cueLine("↑↓ scroll details  v collapse  s sort gaps  u update  b/⎋ back  q quit")
		}
		if m.width <= 90 {
			return cueLine("v collapse  s sort  u update  b/⎋ back  q quit")
		}
		return cueLine("v collapse  s sort gaps  u update  b/⎋ back  q quit")
	}
	switch m.activeKeepAliveState().State {
	case keepalive.StateCountdown:
		if m.width <= 90 {
			return cueLine("↑↓ choose  ↵ act  s send now  x cancel  b/⎋ back  q quit")
		}
		return cueLine("↑↓ choose action  ↵ act  s send now  x cancel instance  b/⎋ back  q quit")
	case keepalive.StateManualReady:
		if m.width <= 90 {
			return cueLine("↑↓ choose  ↵ act  s send now  x dismiss  b/⎋ back  q quit")
		}
		return cueLine("↑↓ choose action  ↵ act  s send now  x dismiss  b/⎋ back  q quit")
	case keepalive.StateConfirming, keepalive.StateSending:
		if m.width <= 90 {
			return cueLine("↑↓ choose  ↵ act  x stop  b/⎋ back  q quit")
		}
		return cueLine("↑↓ choose action  ↵ act  x stop waiting  b/⎋ back  q quit")
	default:
		if m.width <= 90 {
			return cueLine("↑↓ focus  ↵ act  r remind  k KeepAlive  v details  u update  q quit")
		}
		return cueLine("↑↓ focus  ↵/space act  r remind  k KeepAlive  v details  u update  b/⎋ back  q quit")
	}
}

func (m Model) workspaceFocusActions() []string {
	selected := m.selectedSession()
	if selected == nil {
		return nil
	}
	controlActions := m.workspaceControlActions(*selected)
	actions := make([]string, 0, len(controlActions))
	for _, action := range controlActions {
		actions = append(actions, action.id)
	}
	if m.sessionInfoExpanded {
		if m.detailsScrollable() {
			return []string{"details_scroll"}
		}
		return nil
	}
	if m.compactOperationalWorkspace() {
		filtered := actions[:0]
		for _, action := range actions {
			if action != "back" {
				filtered = append(filtered, action)
			}
		}
		return filtered
	}
	return actions
}

func (m Model) defaultFocusIndex() int {
	if m.route != RouteWorkspace {
		return 0
	}
	actions := m.workspaceFocusActions()
	if len(actions) == 0 {
		return 0
	}
	if m.sessionInfoExpanded {
		if index := indexOfAction(actions, "details_scroll"); index >= 0 {
			return index
		}
		return 0
	}
	switch m.activeKeepAliveState().State {
	case keepalive.StateCountdown, keepalive.StateManualReady:
		return indexOfAction(actions, "keepalive_send_now")
	case keepalive.StateSending, keepalive.StateConfirming:
		return indexOfAction(actions, "keepalive_stop_waiting")
	default:
		return 0
	}
}

func (m Model) detailsScrollable() bool {
	selected := m.selectedSession()
	if selected == nil {
		return false
	}
	return len(selected.Gaps) > m.detailGapLimit(*selected)
}

func (m Model) keepAliveUnavailableReason(s session.Session) string {
	status := s.StatusAt(m.now)
	if status.State == session.StatusExpired {
		return "after expiry"
	}
	return ""
}

func (m *Model) restoreFocusAction(action string) {
	if action == "" {
		m.focusIndex = m.defaultFocusIndex()
		return
	}
	actions := m.focusItems()
	for i, item := range actions {
		if item.action == action {
			m.focusIndex = i
			return
		}
	}
	m.focusIndex = m.defaultFocusIndex()
}

func (m Model) actionBanner() string {
	if m.notice.Message == "" {
		return ""
	}
	return DefaultStyles().Render(m.notice.Role, m.notice.Message)
}

func thresholdSummary(thresholds []int) string {
	values := make([]string, 0, len(thresholds))
	for _, threshold := range thresholds {
		values = append(values, fmt.Sprintf("%d%%", threshold))
	}
	return strings.Join(values, ", ")
}

func scopeLabel(maxSends int) string {
	if maxSends == 1 {
		return "1 send"
	}
	return fmt.Sprintf("%d sends", maxSends)
}

func maxSends(state keepalive.SessionState) int {
	if state.MaxSends <= 0 {
		return 1
	}
	return state.MaxSends
}

func onOffPlain(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func successEvidence(state keepalive.SessionState) string {
	if state.LastResult != "" {
		return state.LastResult
	}
	return "Cache refreshed"
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func indexOfAction(actions []string, action string) int {
	for i, candidate := range actions {
		if candidate == action {
			return i
		}
	}
	return 0
}
