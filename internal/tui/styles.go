package tui

import "github.com/charmbracelet/lipgloss"

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
