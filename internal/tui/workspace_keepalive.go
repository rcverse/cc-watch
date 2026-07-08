package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/session"
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

func autoSendWorkspaceDetail(enabled bool) string {
	if enabled {
		return "send after countdown"
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
			fmt.Fprintf(&b, "Next         Message at %s · countdown starts %s before\n", m.keepAliveSendTime(s), formatStatusDuration(m.keepAliveConfig.CountdownSeconds))
		} else {
			fmt.Fprintf(&b, "Next         Manual prompt at %s · auto-send off\n", m.nextKeepAliveTime(s))
		}
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends · auto-send %s\n", state.ScopeUsed, maxSends(state), onOffPlain(state.AutoSend))
	case keepalive.StateCountdown:
		fmt.Fprintf(&b, "Next         Send now or cancel before countdown ends\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		if seconds := m.countdowns[s.SessionID]; seconds > 0 {
			percent := float64(seconds) / float64(max(m.keepAliveConfig.CountdownSeconds, 1)) * 100
			fmt.Fprintf(&b, "Countdown    %s %ds remaining\n", ProgressBar(percent, 12), seconds)
			fmt.Fprintf(&b, "Scope        %d / %d sends\n", state.ScopeUsed, maxSends(state))
		} else {
			fmt.Fprintf(&b, "Scope        %d / %d sends\n", state.ScopeUsed, maxSends(state))
		}
	case keepalive.StateManualReady:
		fmt.Fprintf(&b, "Next         Send now or dismiss\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends · auto-send off\n", state.ScopeUsed, maxSends(state))
		if state.SafetyDisabled {
			fmt.Fprintf(&b, "Reason       Auto-send disabled because safety margin cannot be preserved\n")
		}
	case keepalive.StateSending:
		fmt.Fprintf(&b, "Next         Waiting for Claude CLI\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends · send started\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateConfirming:
		fmt.Fprintf(&b, "Next         Watching this session JSONL\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends · awaiting confirmation\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateSuccess:
		fmt.Fprintf(&b, "Next         Monitoring complete or re-armed by scope\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends used · %s\n", state.ScopeUsed, maxSends(state), successEvidence(state))
	case keepalive.StateErrorNoClaude, keepalive.StateErrorSubprocess, keepalive.StateErrorTimeout:
		reason := state.LastFailure
		if reason == "" {
			reason = string(state.State)
		}
		fmt.Fprintf(&b, "Next         Use manual fallback or re-enable after fixing\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends used · failed: %s\n", state.ScopeUsed, maxSends(state), reason)
		fmt.Fprintf(&b, "Fallback     %s\n", keepalive.ManualFallbackCommand(s.SessionID, m.keepAliveConfig.Message, s.Cwd).Display)
	case keepalive.StateScopeComplete:
		fmt.Fprintf(&b, "Next         Turn KeepAlive off or wait for a new eligible cache window\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends used · no more automatic sends\n", state.ScopeUsed, maxSends(state))
	case keepalive.StateCancelledInstance:
		fmt.Fprintf(&b, "Next         Waiting for a new eligible cache window\n")
		fmt.Fprintf(&b, "Msg Preview  %s\n", messageText(fmt.Sprintf("%q", m.keepAliveConfig.Message)))
		fmt.Fprintf(&b, "Scope        %d / %d sends · cancelled\n", state.ScopeUsed, maxSends(state))
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

func (m Model) workspaceCanSendKeepAlive() bool {
	state := m.activeKeepAliveState()
	return state.State == keepalive.StateCountdown || state.State == keepalive.StateManualReady || isKeepAliveFailure(state.State)
}

func (m Model) workspaceCanCancelKeepAlive() bool {
	state := m.activeKeepAliveState()
	return state.State == keepalive.StateCountdown || state.State == keepalive.StateManualReady || state.State == keepalive.StateSending || state.State == keepalive.StateConfirming || isKeepAliveFailure(state.State)
}
