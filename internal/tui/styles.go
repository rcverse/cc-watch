package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	maxPanelBodyWidth  = 96
	maxVisualLineWidth = maxPanelBodyWidth + 4
)

type StyleRole string

const (
	RoleNeutral       StyleRole = "neutral"
	RoleMuted         StyleRole = "muted"
	RoleCacheTier     StyleRole = "cache_tier"
	RoleFirstLabel    StyleRole = "first_label"
	RoleLastLabel     StyleRole = "last_label"
	RoleExcerptText   StyleRole = "excerpt_text"
	RoleExcerptLabel  StyleRole = "excerpt_label"
	RoleReminder      StyleRole = "reminder"
	RoleKeepAlive     StyleRole = "keepalive"
	RoleSeparator     StyleRole = "separator"
	RoleIdentity      StyleRole = "identity"
	RoleSelectedFocus StyleRole = "selected_focus"
	RoleInfo          StyleRole = "info"
	RoleWarning       StyleRole = "warning"
	RoleDanger        StyleRole = "danger"
	RoleSuccess       StyleRole = "success"
	RoleDisabled      StyleRole = "disabled"
	RoleDegraded      StyleRole = "degraded"
)

type SemanticStyles struct {
	roles map[StyleRole]lipgloss.Style
}

func DefaultStyles() SemanticStyles {
	return SemanticStyles{roles: map[StyleRole]lipgloss.Style{
		RoleNeutral:       lipgloss.NewStyle(),
		RoleMuted:         lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		RoleCacheTier:     lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Italic(true),
		RoleFirstLabel:    lipgloss.NewStyle().Foreground(lipgloss.Color("187")),
		RoleLastLabel:     lipgloss.NewStyle().Foreground(lipgloss.Color("109")),
		RoleExcerptText:   lipgloss.NewStyle().Italic(true),
		RoleExcerptLabel:  lipgloss.NewStyle().Foreground(lipgloss.Color("144")),
		RoleReminder:      lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
		RoleKeepAlive:     lipgloss.NewStyle().Foreground(lipgloss.Color("147")),
		RoleSeparator:     lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
		RoleIdentity:      lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
		RoleSelectedFocus: lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Underline(true),
		RoleInfo:          lipgloss.NewStyle().Foreground(lipgloss.Color("109")),
		RoleWarning:       lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
		RoleDanger:        lipgloss.NewStyle().Foreground(lipgloss.Color("167")),
		RoleSuccess:       lipgloss.NewStyle().Foreground(lipgloss.Color("108")),
		RoleDisabled:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		RoleDegraded:      lipgloss.NewStyle().Foreground(lipgloss.Color("173")),
	}}
}

func (s SemanticStyles) Render(role StyleRole, value string) string {
	style, ok := s.roles[role]
	if !ok {
		style = lipgloss.NewStyle()
	}
	return style.Render(value)
}

func RenderPanelWidth(title string, body string, width int) string {
	return renderPanelWidth(title, body, width, RoleNeutral)
}

func RenderPanelWidthFocused(title string, body string, width int) string {
	return renderPanelWidth(title, body, width, RoleIdentity)
}

func renderPanelWidth(title string, body string, width int, borderRole StyleRole) string {
	if width < 24 {
		width = 24
	}
	titleWidth := visibleWidth(title)
	if width < titleWidth+1 {
		width = titleWidth + 1
	}
	styles := DefaultStyles()
	border := func(value string) string { return styles.Render(borderRole, value) }
	var b strings.Builder
	b.WriteString(border("╭─ "))
	b.WriteString(title)
	b.WriteString(border(" "))
	b.WriteString(border(strings.Repeat("─", max(width-titleWidth-1, 0))))
	b.WriteString(border("╮"))
	b.WriteString("\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		b.WriteString(border("│"))
		b.WriteString(" ")
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", max(width-visibleWidth(line), 0)))
		b.WriteString(" ")
		b.WriteString(border("│"))
		b.WriteString("\n")
	}
	b.WriteString(border("╰"))
	b.WriteString(border(strings.Repeat("─", width+2)))
	b.WriteString(border("╯"))
	b.WriteString("\n")
	return b.String()
}

func ProgressBar(percent float64, width int) string {
	return progressBarWithRole(percent, width, ttlPercentRole(percent))
}

func CountdownProgressBar(percent float64, width int) string {
	return progressBarWithRole(percent, width, countdownPercentRole(percent))
}

func HitRateProgressBar(percent float64, width int) string {
	return progressBarWithRole(percent, width, hitRatePercentRole(percent))
}

func progressBarWithRole(percent float64, width int, role StyleRole) string {
	if width < 1 {
		width = 1
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(math.Round(percent / 100 * float64(width)))
	if filled > width {
		filled = width
	}
	styles := DefaultStyles()
	return styles.roles[role].Render(strings.Repeat("█", filled)) + styles.roles[RoleDisabled].Render(strings.Repeat("░", width-filled))
}

func ttlPercentRole(percent float64) StyleRole {
	if percent >= 80 {
		return RoleDanger
	}
	if percent >= 50 {
		return RoleWarning
	}
	return RoleSuccess
}

func countdownPercentRole(percent float64) StyleRole {
	if percent >= 80 {
		return RoleSuccess
	}
	if percent >= 50 {
		return RoleWarning
	}
	return RoleDanger
}

func hitRatePercentRole(percent float64) StyleRole {
	if percent >= 80 {
		return RoleSuccess
	}
	if percent >= 50 {
		return RoleWarning
	}
	return RoleDanger
}

func stripANSI(s string) string {
	return ansi.Strip(s)
}

func visibleWidth(s string) int {
	return lipgloss.Width(s)
}
