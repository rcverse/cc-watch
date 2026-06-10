package tui

import "strings"

func (m Model) helpText() string {
	var b strings.Builder
	b.WriteString("Help\n")
	b.WriteString("arrows move focus\n")
	b.WriteString("enter activate focused action\n")
	b.WriteString("space toggle focused checkbox\n")
	b.WriteString("? toggle help\n")
	b.WriteString("q quit\n")

	switch m.route {
	case RouteList:
		b.WriteString("r toggle Reminder for selected session\n")
		b.WriteString("k toggle KeepAlive for selected session\n")
	case RouteWorkspace:
		b.WriteString("r toggle Reminder\n")
		b.WriteString("k toggle KeepAlive monitoring\n")
		b.WriteString("c copy or show full session ID\n")
		b.WriteString("b/esc back to list\n")
		b.WriteString(m.keepAliveHelpText())
	case RouteConfig:
		b.WriteString("s save valid config\n")
		b.WriteString("d reset defaults with confirmation\n")
		b.WriteString("esc cancel without writing\n")
	case RouteAmbiguous:
		b.WriteString("enter choose matching session\n")
	}
	return b.String()
}

func (m Model) keepAliveHelpText() string {
	switch m.keepAliveStatus {
	case KeepAliveCountdown:
		return "KeepAlive countdown remains visible\ns send KeepAlive now\nx cancel/dismiss current KeepAlive instance\n"
	case KeepAliveConfirming:
		return "KeepAlive confirming remains visible\nx cancel/dismiss current KeepAlive instance\n"
	case KeepAliveFailure:
		return "KeepAlive failure remains visible\nx cancel/dismiss current KeepAlive instance\n"
	default:
		return ""
	}
}
