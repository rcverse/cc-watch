package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/session"
)

const maxWorkspaceEvidenceLines = 16

func (m Model) workspaceView() string {
	selected := m.selectedSession()
	if selected == nil {
		var b strings.Builder
		b.WriteString("cc-cache workspace\nNo selected session.\nb/esc back  q quit\n")
		if m.helpOpen {
			b.WriteString("\n")
			b.WriteString(m.helpText())
		}
		return b.String()
	}

	var b strings.Builder
	styles := DefaultStyles()
	title := fmt.Sprintf("Claude Code Cache / %s / %s", selected.Project, displayID(*selected))
	b.WriteString(truncateANSI(styles.Render(RoleIdentity, title), m.width))
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleSeparator, strings.Repeat("─", maxInt(minInt(m.width, 76)-2, 12))))
	b.WriteString("\n")
	if banner := m.listDegradedBanner(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n")
	}
	if banner := m.actionBanner(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n")
	}
	b.WriteString(m.cacheStatusCard(*selected))
	b.WriteString(m.sessionInfoCard(*selected))
	if card := m.activeKeepAliveCard(*selected); card != "" {
		b.WriteString(card)
	}
	b.WriteString(m.workspaceControls(*selected))
	b.WriteString(m.workspaceFooter())
	if m.helpOpen {
		b.WriteString("\n")
		b.WriteString(m.helpText())
	}
	return b.String()
}

func (m Model) cacheStatusCard(s session.Session) string {
	styles := DefaultStyles()
	status := s.StatusAt(m.now)
	var b strings.Builder
	state := strings.ToUpper(string(status.State))
	if state == "" {
		state = "UNKNOWN"
	}
	ttlLine := fmt.Sprintf("%-10s %s", styles.Render(statusRole(status), state), cacheDisplay(s))
	if status.PercentElapsed != nil {
		percent := cappedPercent(*status.PercentElapsed)
		ttlLine = fmt.Sprintf("%-10s %s  %s %.0f%%  %s", styles.Render(statusRole(status), state), formatStatusTime(status), ProgressBar(percent, 20), percent, cacheDisplay(s))
	}
	fmt.Fprintf(&b, "%s\n", truncateANSI(ttlLine, maxInt(m.width-4, 20)))
	fmt.Fprintf(&b, "%s    %s\n", styles.Render(RoleMuted, "Session"), displayID(s))
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
	fmt.Fprintf(&b, "%s   %s\n", styles.Render(RoleMuted, "Session ID"), truncateMiddle(s.SessionID, maxInt(m.width-18, 24)))
	fmt.Fprintf(&b, "%s     first %s\n", styles.Render(RoleMuted, "Messages"), truncateEnd(emptyDash(displayExcerpt(s.Messages.FirstUserExcerpt)), maxInt(m.width-23, 18)))
	fmt.Fprintf(&b, "             last  %s\n", truncateEnd(emptyDash(displayExcerpt(s.Messages.LastUserExcerpt)), maxInt(m.width-23, 18)))
	fmt.Fprintf(&b, "%s       writes %d  reads %d  hit %s %.0f%%\n", styles.Render(RoleMuted, "Tokens"), s.TokenStats.CacheWrites, s.TokenStats.CacheReads, HitRateProgressBar(s.TokenStats.HitRate, 8), s.TokenStats.HitRate)
	fmt.Fprintf(&b, "%s         %s  %s\n", styles.Render(RoleMuted, "Gaps"), gapSummary(s), styles.Render(RoleMuted, "v details"))
	return m.renderWorkspacePanel("Session Info", b.String())
}

func (m Model) compactSessionInfoCard(s session.Session) string {
	var b strings.Builder
	styles := DefaultStyles()
	fmt.Fprintf(&b, "%s   %s\n", styles.Render(RoleMuted, "Session ID"), truncateMiddle(s.SessionID, maxInt(m.width-18, 24)))
	fmt.Fprintf(&b, "%s       writes %d  reads %d  hit %.0f%%\n", styles.Render(RoleMuted, "Tokens"), s.TokenStats.CacheWrites, s.TokenStats.CacheReads, s.TokenStats.HitRate)
	if !m.compactOperationalWorkspace() {
		fmt.Fprintf(&b, "%s         %s\n", styles.Render(RoleMuted, "Gaps"), gapSummary(s))
	}
	return m.renderWorkspacePanel("Session Info", b.String())
}

func (m Model) compactOperationalWorkspace() bool {
	state := m.activeKeepAliveState().State
	return m.width <= 90 && m.height <= 24 && (m.sessionInfoExpanded || (state != "" && state != keepalive.StateOff))
}

func (m Model) sessionInfoDetailsCard(s session.Session) string {
	var b strings.Builder
	title := "Session Info · details"
	if m.FocusedAction() == "details_scroll" {
		title = DefaultStyles().Render(RoleIdentity, "› "+title)
	}
	styles := DefaultStyles()
	fmt.Fprintf(&b, "%s   %s\n", styles.Render(RoleMuted, "Session ID"), truncateMiddle(s.SessionID, maxInt(m.width-18, 24)))
	fmt.Fprintf(&b, "%s        %s\n", styles.Render(RoleMuted, "JSONL"), truncateMiddle(s.JSONLPath, maxInt(m.width-18, 24)))
	fmt.Fprintf(&b, "%s      parsed %s · file modified %s\n", styles.Render(RoleMuted, "Updated"), m.now.Local().Format("15:04:05"), s.FileModifiedAt.Local().Format("15:04:05"))
	fmt.Fprintf(&b, "%s     first %s\n", styles.Render(RoleMuted, "Messages"), truncateEnd(emptyDash(displayExcerpt(s.Messages.FirstUserExcerpt)), maxInt(m.width-23, 18)))
	fmt.Fprintf(&b, "             last  %s\n", truncateEnd(emptyDash(displayExcerpt(s.Messages.LastUserExcerpt)), maxInt(m.width-23, 18)))
	fmt.Fprintf(&b, "%s  writes %d · reads %d · hit %s %.0f%% · output %d\n", styles.Render(RoleMuted, "Token Stats"), s.TokenStats.CacheWrites, s.TokenStats.CacheReads, HitRateProgressBar(s.TokenStats.HitRate, 10), s.TokenStats.HitRate, s.TokenStats.OutputTokens)
	fmt.Fprintf(&b, "%s\n", styles.Render(RoleMuted, "Mid-session Gaps >1min · "+m.gapSortLabel()))
	gaps := m.visibleDetailGaps(s)
	if len(gaps) == 0 {
		fmt.Fprintf(&b, "%s\n", styles.Render(RoleMuted, "No mid-session gaps found."))
	} else {
		for _, gap := range gaps {
			fmt.Fprintf(&b, "%s\n", truncateANSI(formatGapLine(gap, s, m.gapSortNewest), maxInt(m.width-4, 20)))
		}
	}
	if total := len(s.Gaps); total > len(gaps) {
		fmt.Fprintf(&b, "%s\n", styles.Render(RoleMuted, fmt.Sprintf("%d more gap(s); use ↑↓ while details is focused", total-len(gaps))))
	}
	if s.ResetCount > 0 {
		fmt.Fprintf(&b, "%s %d cache reset(s) - rebuilt from scratch %d time(s).\n", styles.Render(RoleWarning, "!"), s.ResetCount, s.ResetCount)
	}
	return m.renderWorkspacePanel(title, b.String())
}

func (m Model) renderWorkspacePanel(title string, body string) string {
	return RenderPanelWidth(DefaultStyles().Render(RoleIdentity, title), body, m.workspacePanelWidth())
}

func (m Model) workspacePanelWidth() int {
	return maxInt(m.width-4, 24)
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

func (m Model) workspaceSection(title string) string {
	styles := DefaultStyles()
	return styles.Render(RoleMuted, title)
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
		limit = 2
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

func formatGapLine(gap session.Gap, s session.Session, newest bool) string {
	prefix := "-"
	label := "pause"
	if gap.Reset {
		prefix = "!"
		label = "RESET"
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
	return fmt.Sprintf("%s %-5s %5.0fs    %s -> %s%s", prefix, label, gap.Seconds, gap.From.Local().Format("15:04:05"), gap.To.Local().Format("15:04:05"), tag)
}

func (m Model) workspaceControls(s session.Session) string {
	var b strings.Builder
	b.WriteString(m.workspaceSection("Controls"))
	b.WriteString("\n")
	for _, action := range m.workspaceControlActions(s) {
		fmt.Fprintf(&b, "%s\n", m.controlRow(action.id, action.label, action.value, action.detail))
	}
	return b.String()
}

func (m Model) activeKeepAliveCard(s session.Session) string {
	if m.keepAliveUnavailableReason(s) != "" {
		return ""
	}
	state := m.KeepAliveState(s.SessionID)
	if state.State == "" || state.State == keepalive.StateOff {
		return ""
	}
	return m.keepAliveCard(s, state)
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
	if m.reminderEnabled[s.SessionID] {
		reminderState = onText(false)
	}
	actions = append(actions, workspaceControlAction{id: "reminder", label: "Reminder", value: reminderState, detail: fmt.Sprintf("alert at %s · sends no Claude message", thresholdSummary(m.reminderThresholds))})

	if reason := m.keepAliveUnavailableReason(s); reason != "" {
		actions = append(actions,
			workspaceControlAction{id: "keepalive", label: "KeepAlive", value: DefaultStyles().Render(RoleDisabled, "unavailable"), detail: reason},
			workspaceControlAction{id: "keepalive_autosend", label: "Auto-send", value: onOffText(state.AutoSend, true), detail: "disabled while KeepAlive is unavailable"},
		)
	} else if state.State == keepalive.StateOff || state.State == "" {
		actions = append(actions,
			workspaceControlAction{id: "keepalive", label: "KeepAlive", value: offText(), detail: fmt.Sprintf("trigger %dm before expiry · scope %s", m.keepAliveConfig.TriggerBeforeExpiryMinutes, scopeLabel(m.keepAliveConfig.Scope.MaxSends))},
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
			workspaceControlAction{id: "copy_id", label: "Copy ID", detail: "show full session id"},
			workspaceControlAction{id: "back", label: "Back", detail: "return to session list"},
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
	return fmt.Sprintf("  %s %-11s %-10s %s", marker, label, state, detail)
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

func autoSendWorkspaceDetail(enabled bool) string {
	if enabled {
		return "sends Claude message after countdown"
	}
	return "manual prompt only"
}

func autoSendWorkspaceDetailForState(state keepalive.SessionState) string {
	switch state.State {
	case keepalive.StateSending:
		return "disabled while sending"
	case keepalive.StateConfirming:
		return "disabled while confirming"
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return "disabled after failure"
	default:
		return autoSendWorkspaceDetail(state.AutoSend)
	}
}

func keepAliveControlState(state keepalive.SessionState) string {
	styles := DefaultStyles()
	switch state.State {
	case keepalive.StateMonitoringIdle:
		return styles.Render(RoleSuccess, "armed")
	case keepalive.StateCountdown:
		return styles.Render(RoleWarning, "countdown")
	case keepalive.StateManualReady:
		return styles.Render(RoleWarning, "ready")
	case keepalive.StateSending:
		return styles.Render(RoleWarning, "sending")
	case keepalive.StateConfirming:
		return styles.Render(RoleWarning, "confirming")
	case keepalive.StateSuccess:
		return styles.Render(RoleSuccess, "done")
	case keepalive.StateScopeComplete:
		return styles.Render(RoleMuted, "complete")
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return styles.Render(RoleDanger, "failed")
	default:
		return offText()
	}
}

func keepAliveControlDetail(state keepalive.SessionState) string {
	if state.State == keepalive.StateMonitoringIdle {
		if state.AutoSend {
			return "countdown will start near expiry"
		}
		return "manual prompt near expiry"
	}
	return "see KeepAlive state below"
}

func (m Model) keepAliveCard(s session.Session, state keepalive.SessionState) string {
	var b strings.Builder
	badge := keepAliveBadge(state.State)
	switch state.State {
	case keepalive.StateMonitoringIdle:
		if state.AutoSend {
			fmt.Fprintf(&b, "Next         Countdown at %s if session is still active\n", m.nextKeepAliveTime(s))
		} else {
			fmt.Fprintf(&b, "Next         Manual prompt at %s if session is still active\n", m.nextKeepAliveTime(s))
		}
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends · auto-send %s\n", state.ScopeUsed, maxSends(state), autoSendBox(state.AutoSend))
	case keepalive.StateCountdown:
		fmt.Fprintf(&b, "Next         Send now or cancel before countdown ends\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		if seconds := m.countdowns[s.SessionID]; seconds > 0 {
			percent := float64(seconds) / float64(maxInt(m.keepAliveConfig.CountdownSeconds, 1)) * 100
			fmt.Fprintf(&b, "Scope        %d / %d sends · %s %ds remaining\n", state.ScopeUsed, maxSends(state), ProgressBar(percent, 12), seconds)
		} else {
			fmt.Fprintf(&b, "Scope        %d / %d sends\n", state.ScopeUsed, maxSends(state))
		}
	case keepalive.StateManualReady:
		fmt.Fprintf(&b, "Next         Send now or dismiss\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends · auto-send off\n", state.ScopeUsed, maxSends(state))
		if state.SafetyDisabled {
			fmt.Fprintf(&b, "Reason       Auto-send disabled because safety margin cannot be preserved\n")
		}
	case keepalive.StateSending:
		fmt.Fprintf(&b, "Next         Waiting for Claude CLI\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends · send started\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateConfirming:
		fmt.Fprintf(&b, "Next         Watching this session JSONL\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends · awaiting confirmation\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateSuccess:
		fmt.Fprintf(&b, "Next         Monitoring complete or re-armed by scope\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends used · %s\n", state.ScopeUsed, maxSends(state), successEvidence(state))
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		reason := state.LastFailure
		if reason == "" {
			reason = string(state.State)
		}
		fmt.Fprintf(&b, "Next         Use manual fallback or re-enable after fixing\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends used · failed: %s\n", state.ScopeUsed, maxSends(state), reason)
		fmt.Fprintf(&b, "Fallback     %s\n", keepalive.ManualFallbackCommand(s.SessionID, m.keepAliveConfig.Message).Display)
	case keepalive.StateScopeComplete:
		fmt.Fprintf(&b, "Next         Turn KeepAlive off or wait for a new eligible cache window\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends used · no more automatic sends\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateCancelledInstance:
		fmt.Fprintf(&b, "Next         Waiting for a new eligible cache window\n")
		fmt.Fprintf(&b, "Msg Preview  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "Scope        %d / %d sends · cancelled\n", state.ScopeUsed, maxSends(state))
	}
	return m.renderWorkspacePanel("KeepAlive · "+badge, b.String())
}

func (m Model) workspaceFooter() string {
	if m.sessionInfoExpanded {
		if m.width <= 90 {
			return "v collapse  s sort  u update  q quit\n"
		}
		return "v collapse   s sort gaps   u update   q quit\n"
	}
	switch m.activeKeepAliveState().State {
	case keepalive.StateCountdown:
		if m.width <= 90 {
			return "up/down choose  enter act  s send now  x cancel  b back  ? help  q quit\n"
		}
		return "arrows choose action   enter act   s send now   x cancel instance   b back   ? help   q quit\n"
	case keepalive.StateManualReady:
		if m.width <= 90 {
			return "up/down choose  enter act  s send now  x dismiss  b back  ? help  q quit\n"
		}
		return "arrows choose action   enter act   s send now   x dismiss   b back   ? help   q quit\n"
	case keepalive.StateConfirming, keepalive.StateSending:
		if m.width <= 90 {
			return "up/down choose  enter act  x stop  b back  ? help  q quit\n"
		}
		return "arrows choose action   enter act   x stop waiting   b back   ? help   q quit\n"
	default:
		if m.width <= 90 {
			return "up/down focus  enter act  r remind  k KeepAlive  v details  u update  q quit\n"
		}
		return "arrows move focus   enter/space act   r remind   k KeepAlive   v details   u update   b/esc back   ? help   q quit\n"
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
		var filtered []string
		if m.detailsScrollable() {
			filtered = append(filtered, "details_scroll")
		}
		for _, action := range actions {
			if action != "copy_id" && action != "back" {
				filtered = append(filtered, action)
			}
		}
		return filtered
	}
	if m.compactOperationalWorkspace() {
		filtered := actions[:0]
		for _, action := range actions {
			if action != "copy_id" && action != "back" {
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
		return "unavailable after expiry"
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
	return DefaultStyles().Render(m.notice.Role, "  "+m.notice.Message)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m Model) activeKeepAliveState() keepalive.SessionState {
	selected := m.selectedSession()
	if selected == nil {
		return keepalive.SessionState{State: keepalive.StateOff}
	}
	return m.KeepAliveState(selected.SessionID)
}

func (m Model) nextKeepAliveTime(s session.Session) string {
	if s.LastMessageAt == nil {
		return "unknown"
	}
	ttl := s.CacheWindow.TTLSeconds
	if !s.CacheWindow.Known || ttl <= 0 {
		ttl = 300
	}
	trigger := m.keepAliveConfig.TriggerBeforeExpiryMinutes * 60
	ttlTrigger := ttl / 5
	if ttlTrigger < trigger {
		trigger = ttlTrigger
	}
	return s.LastMessageAt.Add(time.Duration(ttl-trigger) * time.Second).Format("15:04:05")
}

func keepAliveBadge(state keepalive.State) string {
	switch state {
	case keepalive.StateMonitoringIdle:
		return "watching"
	case keepalive.StateCountdown:
		return "countdown"
	case keepalive.StateManualReady:
		return "manual prompt"
	case keepalive.StateSending:
		return "sending"
	case keepalive.StateConfirming:
		return "confirming"
	case keepalive.StateSuccess:
		return "done"
	case keepalive.StateScopeComplete:
		return "scope complete"
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return "failed"
	default:
		return string(state)
	}
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

func autoSendBox(enabled bool) string {
	if enabled {
		return "[x] on"
	}
	return "[ ] off"
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
