package tui

import (
	"math"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type StyleRole string

const (
	RoleNeutral       StyleRole = "neutral"
	RoleMuted         StyleRole = "muted"
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
		RoleMuted:         lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		RoleSelectedFocus: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("24")),
		RoleInfo:          lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		RoleWarning:       lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		RoleDanger:        lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		RoleSuccess:       lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		RoleDisabled:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		RoleDegraded:      lipgloss.NewStyle().Foreground(lipgloss.Color("202")),
	}}
}

func (s SemanticStyles) Has(role StyleRole) bool {
	_, ok := s.roles[role]
	return ok
}

func (s SemanticStyles) Badge(role StyleRole, label string) string {
	style, ok := s.roles[role]
	if !ok {
		style = lipgloss.NewStyle()
	}
	return style.Render("[" + label + "]")
}

func RenderPanel(title string, body string) string {
	width := maxLineWidth(title, body)
	if width < 24 {
		width = 24
	}
	var b strings.Builder
	b.WriteString("╭─ ")
	b.WriteString(title)
	b.WriteString(" ")
	b.WriteString(strings.Repeat("─", maxInt(width-visibleWidth(title)-3, 0)))
	b.WriteString("╮\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		b.WriteString("│ ")
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", maxInt(width-visibleWidth(stripANSI(line)), 0)))
		b.WriteString(" │\n")
	}
	b.WriteString("╰")
	b.WriteString(strings.Repeat("─", width+2))
	b.WriteString("╯\n")
	return b.String()
}

func ProgressBar(percent float64, width int) string {
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
	role := RoleSuccess
	if percent >= 80 {
		role = RoleDanger
	} else if percent >= 50 {
		role = RoleWarning
	}
	styles := DefaultStyles()
	return styles.roles[role].Render(strings.Repeat("█", filled)) + styles.roles[RoleDisabled].Render(strings.Repeat("░", width-filled))
}

func Divider(label string) string {
	if label == "" {
		return "────────"
	}
	return "── " + label + " " + strings.Repeat("─", 8)
}

func Header(title string, details ...string) string {
	parts := []string{title}
	for _, detail := range details {
		if detail != "" {
			parts = append(parts, detail)
		}
	}
	return strings.Join(parts, "  │  ") + "\n"
}

func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func visibleWidth(s string) int {
	return lipgloss.Width(s)
}

func maxLineWidth(title string, body string) int {
	width := visibleWidth(stripANSI(title))
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if w := visibleWidth(stripANSI(line)); w > width {
			width = w
		}
	}
	return width
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
