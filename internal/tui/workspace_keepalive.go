package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcverse/cc-watch/internal/keepalive"
	"github.com/rcverse/cc-watch/internal/session"
)

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

func keepAliveControlState(state keepalive.SessionState) string {
	styles := DefaultStyles()
	switch state.State {
	case keepalive.StateMonitoringIdle, keepalive.StateCountdown, keepalive.StateSending, keepalive.StateConfirming:
		return styles.Render(RoleSuccess, "ON")
	case keepalive.StateScopeComplete:
		return styles.Render(RoleWarning, "limit")
	case keepalive.StatePaused:
		return styles.Render(RoleWarning, "paused")
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return styles.Render(RoleDanger, "failed")
	default:
		return offText()
	}
}

func (m Model) keepAliveControlDetail(s session.Session, state keepalive.SessionState) string {
	switch state.State {
	case keepalive.StateMonitoringIdle, keepalive.StateCountdown, keepalive.StateSending, keepalive.StateConfirming:
		return "message at " + stripANSI(m.keepAliveSendTime(s))
	case keepalive.StateScopeComplete:
		return "automatic sends paused"
	case keepalive.StatePaused:
		return "automatic send paused"
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return "fix issue or reset limit"
	default:
		return fmt.Sprintf("%dm before expiry · %s", m.keepAliveConfig.TriggerBeforeExpiryMinutes, sendsLabel(m.keepAliveConfig.Scope.MaxSends))
	}
}

func (m Model) keepAliveCard(s session.Session, state keepalive.SessionState) string {
	var b strings.Builder
	badge := keepAliveBadge(state.State)
	switch state.State {
	case keepalive.StateMonitoringIdle:
		fmt.Fprintf(&b, "Next         Message will be sent at %s\n", m.keepAliveSendTime(s))
		if state.LastResult != "" {
			fmt.Fprintf(&b, "Last         %s\n", state.LastResult)
		}
		fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateCountdown:
		if seconds := m.countdowns[s.SessionID]; seconds > 0 {
			percent := float64(seconds) / float64(max(m.keepAliveConfig.CountdownSeconds, 1)) * 100
			fmt.Fprintf(&b, "Next         Sending in %ds at %s\n", seconds, m.keepAliveSendTime(s))
			fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
			fmt.Fprintf(&b, "Countdown    %s %ds remaining\n", ProgressBar(percent, 12), seconds)
			fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
		} else {
			fmt.Fprintf(&b, "Next         Message will be sent at %s\n", m.keepAliveSendTime(s))
			fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
			fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
		}
	case keepalive.StatePaused:
		fmt.Fprintf(&b, "Next         Automatic send paused\n")
		reason := state.LastFailure
		if reason == "" {
			reason = "Cache is too close to expiry for a safe automatic send"
		}
		fmt.Fprintf(&b, "Last         %s\n", reason)
		fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateSending:
		fmt.Fprintf(&b, "Next         Sending message now\n")
		fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateConfirming:
		fmt.Fprintf(&b, "Next         Checking for confirmation\n")
		fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		reason := state.LastFailure
		if reason == "" {
			reason = string(state.State)
		}
		fmt.Fprintf(&b, "Next         Fix the issue, then turn KeepAlive off/on or reset limit\n")
		fmt.Fprintf(&b, "Last         %s\n", reason)
		fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
		fmt.Fprintf(&b, "Fallback     %s\n", keepalive.ManualFallbackCommand(s.SessionID, m.keepAliveConfig.Message, s.Cwd))
	case keepalive.StateScopeComplete:
		fmt.Fprintf(&b, "Next         Automatic sends paused\n")
		if state.LastResult != "" {
			fmt.Fprintf(&b, "Last         %s\n", state.LastResult)
		}
		fmt.Fprintf(&b, "Message      %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Sends        %d / %d used\n", state.ScopeUsed, maxSends(state))
	}
	return m.renderWorkspacePanel("KeepAlive · "+badge, b.String())
}

func (m Model) activeKeepAliveState() keepalive.SessionState {
	selected := m.selectedSession()
	if selected == nil {
		return keepalive.SessionState{State: keepalive.StateOff}
	}
	return m.KeepAliveState(selected.SessionID)
}

func (m Model) nextKeepAliveTime(s session.Session) string {
	return m.keepAliveTimeAt(s, 0)
}

func (m Model) keepAliveSendTime(s session.Session) string {
	return m.keepAliveTimeAt(s, m.keepAliveConfig.CountdownSeconds)
}

func (m Model) keepAliveTimeAt(s session.Session, offsetSeconds int) string {
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
	at := s.LastMessageAt.Add(time.Duration(ttl-trigger+offsetSeconds) * time.Second).Local().Format("15:04:05")
	return DefaultStyles().Render(RoleInfo, at)
}

func keepAliveBadge(state keepalive.State) string {
	switch state {
	case keepalive.StateMonitoringIdle, keepalive.StateCountdown, keepalive.StateSending, keepalive.StateConfirming:
		return "✓ Armed"
	case keepalive.StatePaused:
		return "! Paused"
	case keepalive.StateScopeComplete:
		return "! Limit reached"
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		return "✕ Failed"
	default:
		return string(state)
	}
}

func (m Model) workspaceCanSendKeepAlive() bool {
	state := m.activeKeepAliveState()
	return state.State == keepalive.StateCountdown || isKeepAliveFailure(state.State)
}

func (m Model) workspaceCanCancelKeepAlive() bool {
	state := m.activeKeepAliveState()
	return state.State == keepalive.StateCountdown || state.State == keepalive.StateSending || state.State == keepalive.StateConfirming || isKeepAliveFailure(state.State)
}
