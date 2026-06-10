package tui

import (
	"fmt"
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
	b.WriteString(Header(fmt.Sprintf("cc-cache / %s / %s", selected.Project, displayID(*selected)), "focus: "+m.FocusedAction()))
	b.WriteString(RenderPanel("System", strings.TrimRight(m.listSystemState(), "\n")))
	b.WriteString("\nSession Evidence\n")
	b.WriteString(RenderPanel("Evidence", strings.TrimRight(m.workspaceEvidence(*selected), "\n")))
	b.WriteString("\nControls\n")
	b.WriteString(RenderPanel("Controls", strings.TrimRight(m.workspaceControls(*selected), "\n")))
	b.WriteString(m.workspaceFooter())
	if m.helpOpen {
		b.WriteString("\n")
		b.WriteString(m.helpText())
	}
	return b.String()
}

func (m Model) workspaceEvidence(s session.Session) string {
	lines := workspaceEvidenceLines(s, m.now)
	total := len(lines)
	offset := m.evidenceOffset
	overflow := len(lines) > maxWorkspaceEvidenceLines
	if overflow {
		maxOffset := len(lines) - maxWorkspaceEvidenceLines
		if offset > maxOffset {
			offset = maxOffset
		}
		lines = lines[offset:]
		if len(lines) > maxWorkspaceEvidenceLines {
			lines = lines[:maxWorkspaceEvidenceLines]
		}
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if overflow {
		fmt.Fprintf(&b, "Evidence scroll %d-%d of %d\n", offset+1, minInt(offset+len(lines), total), total)
	}
	return b.String()
}

func workspaceEvidenceLines(s session.Session, now time.Time) []string {
	var lines []string
	status := s.StatusAt(now)
	lines = append(lines, "Status")
	if status.PercentElapsed != nil {
		lines = append(lines, fmt.Sprintf("  cache window: %s TTL, %s, %s  %s %.0f%%", cacheLabel(s), status.State, formatStatusTime(status), ProgressBar(*status.PercentElapsed, 16), *status.PercentElapsed))
	} else {
		lines = append(lines, fmt.Sprintf("  cache window: %s TTL, %s, %s", cacheLabel(s), status.State, formatStatusTime(status)))
	}
	lines = append(lines, fmt.Sprintf("  Project: %s", s.Project))
	lines = append(lines, fmt.Sprintf("  Full session ID: %s", s.SessionID))
	lines = append(lines, fmt.Sprintf("  JSONL: %s", truncateMiddle(s.JSONLPath, 66)))
	lines = append(lines, fmt.Sprintf("  File refresh: %s", formatModified(s.FileModifiedAt)))
	if len(s.CacheWindow.Evidence) > 0 {
		lines = append(lines, fmt.Sprintf("  Evidence: %s", strings.Join(s.CacheWindow.Evidence, ", ")))
		if len(s.CacheWindow.Evidence) > 3 {
			for _, evidence := range s.CacheWindow.Evidence {
				lines = append(lines, fmt.Sprintf("    - %s", evidence))
			}
		}
	}

	lines = append(lines, "Messages")
	lines = append(lines, fmt.Sprintf("  First user: %s", emptyDash(s.Messages.FirstUserExcerpt)))
	lines = append(lines, fmt.Sprintf("  Last user: %s", emptyDash(s.Messages.LastUserExcerpt)))

	lines = append(lines, "Token Stats")
	lines = append(lines, fmt.Sprintf("  Cache writes: %d   Cache reads: %d   Hit rate: %.0f%%", s.TokenStats.CacheWrites, s.TokenStats.CacheReads, s.TokenStats.HitRate))

	lines = append(lines, "Gaps")
	if len(s.Gaps) == 0 {
		lines = append(lines, "  0 cache resets detected")
	} else {
		lines = append(lines, fmt.Sprintf("  %d gap(s); latest %.0fs", len(s.Gaps), s.Gaps[len(s.Gaps)-1].Seconds))
	}
	if len(s.Warnings) > 0 {
		lines = append(lines, fmt.Sprintf("  Parse warnings: %d", len(s.Warnings)))
		if len(s.Warnings) > 3 {
			for _, warning := range s.Warnings {
				lines = append(lines, fmt.Sprintf("    - %s", emptyDash(warning.Message)))
			}
		}
	}
	return lines
}

func (m Model) workspaceControls(s session.Session) string {
	var b strings.Builder
	reminder := "[ ]"
	if m.reminderEnabled[s.SessionID] {
		reminder = "[x]"
	}
	fmt.Fprintf(&b, "%s Reminder   alert at %s   Sends no Claude message.\n", reminder, thresholdSummary(m.reminderThresholds))

	state := m.KeepAliveState(s.SessionID)
	if state.State == keepalive.StateOff || state.State == "" {
		checked := "[x]"
		if !state.AutoSend {
			checked = "[ ]"
		}
		fmt.Fprintf(&b, "[ ] KeepAlive  %s auto-send · trigger %dm · scope %s   Off. No message.\n", checked, m.keepAliveConfig.TriggerBeforeExpiryMinutes, scopeLabel(m.keepAliveConfig.Scope.MaxSends))
	} else {
		b.WriteString(m.keepAliveCard(s, state))
	}
	b.WriteString("Actions: copy ID · manual refresh · help · back · quit\n")
	return b.String()
}

func (m Model) keepAliveCard(s session.Session, state keepalive.SessionState) string {
	var b strings.Builder
	badge := keepAliveBadge(state.State)
	checked := "[x]"
	if state.State == keepalive.StateOff {
		checked = "[ ]"
	}
	fmt.Fprintf(&b, "%s KeepAlive -------------------------------- %s ----\n", checked, badge)
	switch state.State {
	case keepalive.StateMonitoringIdle:
		fmt.Fprintf(&b, "  State    Watching cache expiry\n")
		if state.AutoSend {
			fmt.Fprintf(&b, "  Next     Countdown at %s if session still active\n", m.nextKeepAliveTime(s))
			fmt.Fprintf(&b, "  Claude message  May send after countdown unless canceled\n")
		} else {
			fmt.Fprintf(&b, "  Next     Manual prompt at %s if session still active\n", m.nextKeepAliveTime(s))
			fmt.Fprintf(&b, "  Claude message  Will not send automatically\n")
		}
		fmt.Fprintf(&b, "  Message  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "  Scope    %d / %d sends · auto-send %s\n", state.ScopeUsed, maxSends(state), autoSendBox(state.AutoSend))
	case keepalive.StateCountdown:
		fmt.Fprintf(&b, "  State    Countdown %ds\n", m.countdowns[s.SessionID])
		if seconds := m.countdowns[s.SessionID]; seconds > 0 {
			percent := float64(seconds) / float64(maxInt(m.keepAliveConfig.CountdownSeconds, 1)) * 100
			fmt.Fprintf(&b, "  Progress %s %ds remaining\n", ProgressBar(percent, 16), seconds)
		}
		fmt.Fprintf(&b, "  Next     Send now or cancel before countdown ends\n")
		fmt.Fprintf(&b, "  Claude message  Will send at zero if not canceled\n")
		fmt.Fprintf(&b, "  Message  %q\n", m.keepAliveConfig.Message)
		fmt.Fprintf(&b, "  Target   %s · %s\n", displayID(s), s.Project)
		fmt.Fprintf(&b, "  Scope    %d / %d sends · will count as %d if sent\n", state.ScopeUsed, maxSends(state), state.ScopeUsed+1)
		fmt.Fprintf(&b, "  Controls Send now · Cancel this instance\n")
	case keepalive.StateManualReady:
		fmt.Fprintf(&b, "  State    Manual send available\n")
		fmt.Fprintf(&b, "  Next     Send now or dismiss\n")
		fmt.Fprintf(&b, "  Claude message  Not sent\n")
		fmt.Fprintf(&b, "  Message  %q\n", m.keepAliveConfig.Message)
		if state.SafetyDisabled {
			fmt.Fprintf(&b, "  Reason   Auto-send disabled because safety margin cannot be preserved\n")
		}
		fmt.Fprintf(&b, "  Scope    %d / %d sends\n", state.ScopeUsed, maxSends(state))
		fmt.Fprintf(&b, "  Controls Send now · Dismiss\n")
	case keepalive.StateSending:
		fmt.Fprintf(&b, "  State    Sending message\n")
		fmt.Fprintf(&b, "  Next     Waiting for Claude CLI\n")
		fmt.Fprintf(&b, "  Claude message  Send started\n")
		fmt.Fprintf(&b, "  Auto-send disabled while sending; wait or stop waiting first\n")
		fmt.Fprintf(&b, "  Controls Stop waiting\n")
	case keepalive.StateConfirming:
		fmt.Fprintf(&b, "  State    Confirming result\n")
		fmt.Fprintf(&b, "  Next     Watching this session JSONL\n")
		fmt.Fprintf(&b, "  Claude message  Sent, awaiting evidence\n")
		fmt.Fprintf(&b, "  Evidence watching %s for a new entry after send time\n", s.JSONLPath)
		fmt.Fprintf(&b, "  Auto-send disabled while confirming; wait or stop waiting first\n")
		fmt.Fprintf(&b, "  Controls Stop waiting\n")
	case keepalive.StateSuccess:
		fmt.Fprintf(&b, "  State    Cache refreshed\n")
		fmt.Fprintf(&b, "  Next     Monitoring complete or re-armed by scope\n")
		fmt.Fprintf(&b, "  Claude message  Sent and confirmed\n")
		fmt.Fprintf(&b, "  Evidence %s\n", successEvidence(state))
		fmt.Fprintf(&b, "  Scope    %d / %d sends used\n", state.ScopeUsed, maxSends(state))
		fmt.Fprintf(&b, "  Controls Acknowledge · Refresh\n")
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		reason := state.LastFailure
		if reason == "" {
			reason = string(state.State)
		}
		fmt.Fprintf(&b, "  State    Failed: %s\n", reason)
		fmt.Fprintf(&b, "  Next     Use manual fallback or re-enable after fixing\n")
		fmt.Fprintf(&b, "  Claude message  Stopped or not confirmed\n")
		fmt.Fprintf(&b, "  Scope    %d / %d sends used. Auto-send stopped for this session.\n", state.ScopeUsed, maxSends(state))
		fmt.Fprintf(&b, "  Auto-send disabled until this failure is acknowledged or KeepAlive is reset\n")
		fmt.Fprintf(&b, "  Manual fallback:\n")
		fmt.Fprintf(&b, "    %s\n", keepalive.ManualFallbackCommand(s.SessionID, m.keepAliveConfig.Message).Display)
		fmt.Fprintf(&b, "  Controls Copy command · Refresh · Turn Off\n")
	case keepalive.StateScopeComplete:
		fmt.Fprintf(&b, "  State    Scope complete\n")
		fmt.Fprintf(&b, "  Next     Turn KeepAlive off or wait for a new eligible cache window\n")
		fmt.Fprintf(&b, "  Claude message  No more automatic sends\n")
		fmt.Fprintf(&b, "  Scope    %d / %d sends used\n", state.ScopeUsed, maxSends(state))
		fmt.Fprintf(&b, "  Controls Acknowledge · Turn Off\n")
	case keepalive.StateCancelledInstance:
		fmt.Fprintf(&b, "  State    Cancelled this instance\n")
		fmt.Fprintf(&b, "  Next     Waiting for a new eligible cache window\n")
		fmt.Fprintf(&b, "  Claude message  Will not send for this instance\n")
		fmt.Fprintf(&b, "  Controls Turn Off · Refresh\n")
	}
	return b.String()
}

func (m Model) workspaceFooter() string {
	if m.FocusedAction() == "evidence" {
		return "arrows scroll evidence   enter/space no action   b back   ? help   q quit\n"
	}
	switch m.activeKeepAliveState().State {
	case keepalive.StateCountdown:
		return "arrows choose action   enter act   s send now   x cancel instance   b back   ? help   q quit\n"
	case keepalive.StateManualReady:
		return "arrows choose action   enter act   s send now   x dismiss   b back   ? help   q quit\n"
	case keepalive.StateConfirming, keepalive.StateSending:
		return "arrows choose action   enter act   x stop waiting   b back   ? help   q quit\n"
	default:
		return "arrows move focus   enter/space act   r remind   k keepalive   c copy id   b/esc back   ? help   q quit\n"
	}
}

func (m Model) workspaceFocusActions() []string {
	state := m.activeKeepAliveState()
	prefix := []string{}
	if selected := m.selectedSession(); selected != nil && len(workspaceEvidenceLines(*selected, m.now)) > maxWorkspaceEvidenceLines {
		prefix = append(prefix, "evidence")
	}
	var actions []string
	switch state.State {
	case keepalive.StateCountdown, keepalive.StateManualReady:
		actions = []string{"keepalive_send_now", "keepalive_cancel", "reminder", "keepalive", "keepalive_autosend", "copy_id", "refresh", "help", "back", "quit"}
	case keepalive.StateSending, keepalive.StateConfirming:
		actions = []string{"keepalive_stop_waiting", "reminder", "keepalive", "keepalive_autosend", "copy_id", "refresh", "help", "back", "quit"}
	case keepalive.StateSuccess, keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout, keepalive.StateScopeComplete:
		actions = []string{"keepalive_acknowledge", "keepalive", "keepalive_autosend", "copy_id", "refresh", "help", "back", "quit"}
	default:
		actions = []string{"reminder", "keepalive", "keepalive_autosend", "copy_id", "refresh", "help", "back", "quit"}
	}
	return append(prefix, actions...)
}

func (m Model) defaultFocusIndex() int {
	if m.route != RouteWorkspace {
		return 0
	}
	actions := m.workspaceFocusActions()
	if len(actions) == 0 {
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

func (m Model) evidenceCanScroll(delta int) bool {
	selected := m.selectedSession()
	if selected == nil {
		return false
	}
	lineCount := len(workspaceEvidenceLines(*selected, m.now))
	if lineCount <= maxWorkspaceEvidenceLines {
		return false
	}
	next := m.evidenceOffset + delta
	return next >= 0 && next <= lineCount-maxWorkspaceEvidenceLines
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
