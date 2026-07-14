package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rcverse/cc-watch/internal/config"
	"github.com/rcverse/cc-watch/internal/statusline"
)

var configFocusActions = []string{
	"config_reminder_thresholds",
	"config_trigger",
	"config_countdown",
	"config_message",
	"config_max_sends",
	"config_statusline",
	"config_save",
	"config_reset",
	"config_cancel",
}

var statuslineFocusActions = []string{
	"config_statusline_usage",
	"config_statusline_warning",
	"config_statusline_cache",
	"config_statusline_sequence",
	"config_statusline_action",
	"config_save",
	"config_statusline_back",
}

func (m Model) configView() string {
	cfg := m.configDraft
	var b strings.Builder
	styles := DefaultStyles()
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleIdentity, "Claude Code Watch / config"))
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleSeparator, strings.Repeat("─", 46)))
	b.WriteString("\n")

	var settings strings.Builder
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_reminder_thresholds", "Reminder thresholds", thresholdsText(cfg.ReminderThresholds)+"%", "notify when cache is fading"))
	if message := m.configFieldError("config_reminder_thresholds"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_trigger", "KeepAlive trigger", fmt.Sprintf("%dm", cfg.KeepAlive.TriggerBeforeExpiryMinutes), "start before cache expiry"))
	if message := m.configFieldError("config_trigger"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_countdown", "Countdown", fmt.Sprintf("%ds", cfg.KeepAlive.CountdownSeconds), "wait before sending"))
	if message := m.configFieldError("config_countdown"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	if countdownWarnsFor5Minute(cfg) {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleWarning, "Warning: countdown may not fit the 5m cache trigger window."))
	}
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_message", "Message", truncateEnd(cfg.KeepAlive.Message, 38), "text sent to Claude Code"))
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_max_sends", "Max sends", fmt.Sprintf("%d", cfg.KeepAlive.Scope.MaxSends), "stop after this many automatic sends"))
	if message := m.configFieldError("config_max_sends"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	state, detail, _, _ := m.statuslineConfigCopy()
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_statusline", "Statusline", state, detail))
	b.WriteString(m.renderConfigPanel("Settings", settings.String()))

	if m.configEditing {
		var edit strings.Builder
		fmt.Fprintf(&edit, "%s %s\n", styles.Render(RoleInfo, configFieldLabel(m.configEditingField)), styles.Render(RoleMuted, "is active"))
		fmt.Fprintf(&edit, "%s  %s%s\n", styles.Render(RoleMuted, "Current input"), m.configInput, styles.Render(RoleIdentity, "▌"))
		fmt.Fprintf(&edit, "%s\n", styles.Render(RoleMuted, "Enter save field  Esc cancel edit"))
		b.WriteString(m.renderConfigPanel("Editing", edit.String()))
	}

	var preview strings.Builder
	preview.WriteString(configBehaviorSummary(cfg))
	b.WriteString(m.renderConfigPanel("Preview", preview.String()))

	var status strings.Builder
	if err := m.configEditorValidation(); err != nil {
		fmt.Fprintf(&status, "%s\n", styles.Render(RoleDanger, "✕ Validation failed: "+err.Error()))
	} else {
		fmt.Fprintf(&status, "%s\n", styles.Render(RoleSuccess, "✓ Validation OK"))
	}
	if m.configResetConfirm {
		status.WriteString("\n")
		status.WriteString("Reset defaults? This will replace KeepAlive defaults.\n")
		status.WriteString("Press d again to confirm, Esc to keep current settings.\n")
	}
	b.WriteString(m.renderConfigPanel("Status", status.String()))

	var actions strings.Builder
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_save", "Save", "", "write config"))
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_reset", "Reset defaults", "", "requires confirmation"))
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_cancel", "Cancel", "", "discard edits"))
	b.WriteString(m.renderConfigPanel("Actions", actions.String()))
	if m.notice.Message != "" {
		b.WriteString(m.renderConfigPanel("Notice", DefaultStyles().Render(m.notice.Role, m.notice.Message)))
	}
	b.WriteString(cueLine("↑↓ move  Enter edit/act  s save  d reset  Esc cancel"))
	return b.String()
}

func (m Model) statuslineView() string {
	var b strings.Builder
	styles := DefaultStyles()
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleIdentity, "Claude Code Watch / statusline"))
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleSeparator, strings.Repeat("─", 46)))
	b.WriteString("\n")

	state, detail, actionLabel, actionDetail := m.statuslineConfigCopy()
	var statuslinePanel strings.Builder
	fmt.Fprintf(&statuslinePanel, " %s\n", styles.Render(statuslineStateRole(state), "● "+state))
	fmt.Fprintf(&statuslinePanel, " %s\n", styles.Render(RoleMuted, detail))
	b.WriteString(m.renderConfigPanel("Statusline", statuslinePanel.String()))

	var elements strings.Builder
	for _, element := range []string{config.StatuslineElementUsage, config.StatuslineElementWarning, config.StatuslineElementCache} {
		cfg := statuslineElementConfig(m.configDraft.Statusline, element)
		fmt.Fprintf(&elements, "%s\n", m.statuslineElementRow("config_statusline_"+element, statusline.ElementLabel(element), statuslineElementValue(*cfg), statuslineElementDetail(element, *cfg)))
	}
	b.WriteString(m.renderConfigPanel("Elements", elements.String()))
	b.WriteString(m.renderConfigPanel("Sequence", m.configRow("config_statusline_sequence", "Order", statusline.OrderText(m.configDraft.Statusline.Order), "Enter to reorder elements")))
	b.WriteString(m.renderConfigPanel("Preview", styles.Render(RoleMuted, statusline.Preview(m.configDraft.Statusline))))
	if m.configChoiceField != "" {
		b.WriteString(m.renderConfigPanel("Choose "+configFieldLabel(m.configChoiceField), m.statuslineChoiceView()))
	}

	if m.configEditing {
		var edit strings.Builder
		fmt.Fprintf(&edit, "%s %s\n", styles.Render(RoleInfo, configFieldLabel(m.configEditingField)), styles.Render(RoleMuted, "is active"))
		fmt.Fprintf(&edit, "%s  %s%s\n", styles.Render(RoleMuted, "Current input"), m.configInput, styles.Render(RoleIdentity, "▌"))
		fmt.Fprintf(&edit, "%s\n", styles.Render(RoleMuted, "Enter save field  Esc cancel edit"))
		b.WriteString(m.renderConfigPanel("Editing", edit.String()))
	}

	var actions strings.Builder
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_statusline_action", actionLabel, "", actionDetail))
	if m.configStatuslineConfirm {
		fmt.Fprintf(&actions, "    %s\n", styles.Render(RoleWarning, "Press Enter again to "+strings.ToLower(actionLabel)+"; Esc cancels"))
	}
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_save", "Save", "", "write config"))
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_statusline_back", "Back", "", "return to Settings"))
	b.WriteString(m.renderConfigPanel("Actions", actions.String()))
	if m.notice.Message != "" {
		b.WriteString(m.renderConfigPanel("Notice", styles.Render(m.notice.Role, m.notice.Message)))
	}
	cue := "↑↓ move  Enter choose/act  s save  d reset  Esc back"
	if statuslineElementName(m.FocusedAction()) != "" {
		cue = "↑↓ move  Enter choose  Space toggle  s save  d reset  Esc back"
	}
	if m.configChoiceField != "" {
		cue = "↑↓ choose  Enter select  Esc cancel"
		if m.configChoiceField == "config_statusline_sequence" {
			cue = "↑↓ select  ←→ move  Enter done  Esc cancel"
		}
	}
	b.WriteString(cueLine(cue))
	return b.String()
}

func (m Model) statuslineChoiceView() string {
	styles := DefaultStyles()
	values := m.statuslineChoiceValues(m.configChoiceField)
	var b strings.Builder
	for index, value := range values {
		marker := " "
		role := RoleMuted
		if index == m.configChoiceIndex {
			marker = styles.Render(RoleIdentity, "›")
			role = RoleIdentity
		}
		label := statuslineChoiceLabel(m.configChoiceField, value)
		if m.configChoiceField == "config_statusline_sequence" {
			label = fmt.Sprintf("%d  %s", index+1, label)
		}
		fmt.Fprintf(&b, "  %s %s\n", marker, styles.Render(role, label))
	}
	return b.String()
}

func (m Model) configRow(action string, label string, value string, detail string) string {
	styles := DefaultStyles()
	marker := " "
	if m.FocusedAction() == action {
		marker = styles.Render(RoleIdentity, "›")
	}
	width := m.configPanelBodyWidth()
	valueWidth := visibleWidth(value)
	detailWidth := max(width-27-valueWidth, 8)
	if value != "" && valueWidth > 24 && visibleWidth(detail) > detailWidth {
		first := truncateANSI(fmt.Sprintf("  %s %-20s %s", marker, label, styles.Render(RoleMuted, detail)), width)
		second := truncateANSI(fmt.Sprintf("    %-20s %s", "", value), width)
		return first + "\n" + second
	}
	detailText := padANSI(truncateANSI(styles.Render(RoleMuted, detail), detailWidth), detailWidth)
	return truncateANSI(fmt.Sprintf("  %s %-20s %s %s", marker, label, detailText, value), width)
}

func (m Model) statuslineElementRow(action string, label string, value string, detail string) string {
	styles := DefaultStyles()
	marker := " "
	if m.FocusedAction() == action {
		marker = styles.Render(RoleIdentity, "›")
	}
	const detailWidth = 22
	const detailGap = 4
	if m.configPanelBodyWidth() < 2+1+1+20+1+detailWidth+detailGap+visibleWidth(value) {
		return m.configRow(action, label, value, detail)
	}
	detailText := padANSI(styles.Render(RoleMuted, detail), detailWidth)
	return fmt.Sprintf("  %s %-20s %s    %s", marker, label, detailText, value)
}

func (m Model) renderConfigPanel(title string, body string) string {
	width := m.configPanelBodyWidth()
	return RenderPanelWidth(DefaultStyles().Render(RoleIdentity, title), truncateBodyLines(body, width), width)
}

func (m Model) statuslineConfigCopy() (state string, detail string, actionLabel string, actionDetail string) {
	status, err := m.deps.InspectStatusline()
	if err != nil {
		return "Needs manual review", "Run cc-watch statusline --check", "Review", "show manual instructions"
	}
	switch status.State {
	case statusline.StateInstalled:
		if m.statuslineNeedsReinstall(status) {
			return "Needs reinstall", "current command path may be stale or unavailable", "Reinstall", "repair cc-watch integration"
		}
		return "Installed", "appears after Claude Code's next message", "Uninstall", "remove cc-watch integration"
	case statusline.StateExisting:
		return "Not installed", "Claude Code statusline is not using cc-watch", "Install", "keep the current statusline and add cc-watch"
	case statusline.StateManualReview:
		return "Needs manual review", "Run cc-watch statusline --check", "Review", "show manual instructions"
	default:
		return "Not installed", "Claude Code statusline is not using cc-watch", "Install", "add cc-watch integration"
	}
}

func statuslineStateRole(state string) StyleRole {
	switch state {
	case "Not installed":
		return RoleIdentity
	case "Installed":
		return RoleSuccess
	case "Needs reinstall":
		return RoleWarning
	default:
		return RoleDegraded
	}
}

func (m Model) statuslineChoiceValues(field string) []string {
	switch field {
	case "config_statusline_sequence":
		return append([]string(nil), m.configDraft.Statusline.Order...)
	case "config_statusline_usage_format", "config_statusline_warning_format", "config_statusline_cache_format":
		return statuslineFormatChoiceValues(field)
	default:
		return nil
	}
}

func statuslineFormatChoiceValues(field string) []string {
	formats := []string{config.StatuslineFormatFull, config.StatuslineFormatCompact}
	if field == "config_statusline_warning_format" {
		formats = []string{config.StatuslineWarningFormatAlert, config.StatuslineWarningFormatVerbose}
	}
	values := make([]string, 0, len(formats)*2)
	for _, layout := range []string{config.StatuslineLayoutSameLine, config.StatuslineLayoutNewLine} {
		for _, format := range formats {
			values = append(values, format+"|"+layout)
		}
	}
	return values
}

func statuslineChoiceLabel(field, value string) string {
	if field == "config_statusline_sequence" {
		return statusline.ElementLabel(value)
	}
	if strings.HasSuffix(field, "_format") {
		format, layout := splitStatuslineFormatChoice(value)
		if strings.HasPrefix(field, "config_statusline_warning") {
			if format == config.StatuslineWarningFormatVerbose {
				return "Verbose · " + statuslineLayoutLabel(layout)
			}
			return "Alert only · " + statuslineLayoutLabel(layout)
		}
		return statuslineFormatLabel(format) + " · " + statuslineLayoutLabel(layout)
	}
	return value
}

func splitStatuslineFormatChoice(value string) (string, string) {
	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 {
		return value, config.StatuslineLayoutSameLine
	}
	return parts[0], parts[1]
}

func statuslineElementChoiceValue(element config.StatuslineElementConfig) string {
	return element.Format + "|" + element.Layout
}

func statuslineLayoutLabel(value string) string {
	if value == config.StatuslineLayoutNewLine {
		return "New line"
	}
	return "Same line"
}

func statuslineFormatLabel(value string) string {
	if value == config.StatuslineFormatCompact {
		return "Compact"
	}
	return "Full"
}

func statuslineElementConfig(cfg config.StatuslineConfig, name string) *config.StatuslineElementConfig {
	switch name {
	case config.StatuslineElementUsage:
		return &cfg.Usage
	case config.StatuslineElementWarning:
		return &cfg.Warning
	case config.StatuslineElementCache:
		return &cfg.Cache
	default:
		return &config.StatuslineElementConfig{}
	}
}

func statuslineElementValue(element config.StatuslineElementConfig) string {
	return onOffText(element.Enabled, false)
}

func statuslineElementDetail(name string, element config.StatuslineElementConfig) string {
	format := statuslineFormatLabel(element.Format)
	if name == config.StatuslineElementWarning {
		if element.Format == config.StatuslineWarningFormatVerbose {
			format = "Verbose"
		} else {
			format = "Alert only"
		}
	}
	return statuslineLayoutLabel(element.Layout) + " · " + format
}

func statuslineElementName(action string) string {
	for _, name := range []string{config.StatuslineElementUsage, config.StatuslineElementWarning, config.StatuslineElementCache} {
		if action == "config_statusline_"+name {
			return name
		}
	}
	return ""
}

func (m *Model) toggleStatuslineElement(name string) {
	switch name {
	case config.StatuslineElementUsage:
		m.configDraft.Statusline.Usage.Enabled = !m.configDraft.Statusline.Usage.Enabled
	case config.StatuslineElementWarning:
		m.configDraft.Statusline.Warning.Enabled = !m.configDraft.Statusline.Warning.Enabled
	case config.StatuslineElementCache:
		m.configDraft.Statusline.Cache.Enabled = !m.configDraft.Statusline.Cache.Enabled
	}
}

func statuslineElementConfigForDraft(cfg *config.StatuslineConfig, name string) *config.StatuslineElementConfig {
	switch name {
	case config.StatuslineElementUsage:
		return &cfg.Usage
	case config.StatuslineElementWarning:
		return &cfg.Warning
	case config.StatuslineElementCache:
		return &cfg.Cache
	default:
		return nil
	}
}

func (m Model) activateStatuslineConfigAction() (tea.Model, tea.Cmd) {
	status, err := m.deps.InspectStatusline()
	if err != nil || status.State == statusline.StateManualReview {
		m.configStatuslineConfirm = false
		m.setNotice("Run cc-watch statusline --check for manual instructions", RoleWarning, 5*time.Second)
		return m, nil
	}
	if !m.configStatuslineConfirm {
		m.configStatuslineConfirm = true
		return m, nil
	}
	reinstall := m.statuslineNeedsReinstall(status)
	m.configStatuslineConfirm = false
	if status.State == statusline.StateInstalled && !reinstall {
		if err := m.deps.UninstallStatusline(); err != nil {
			m.setNotice("✕ Could not uninstall statusline", RoleDanger, 3*time.Second)
			return m, nil
		}
		m.setNotice("✓ Statusline uninstalled from Claude Code", RoleSuccess, 3*time.Second)
		return m, nil
	}
	if m.deps.InstallStatusline == nil {
		m.setNotice("✕ Statusline install is unavailable", RoleDanger, 3*time.Second)
		return m, nil
	}
	if err := m.deps.InstallStatusline(m.configDraft); err != nil {
		m.setNotice("✕ Could not install statusline", RoleDanger, 3*time.Second)
		return m, nil
	}
	if reinstall {
		m.setNotice("✓ Statusline command repaired", RoleSuccess, 3*time.Second)
	} else {
		m.setNotice("✓ Statusline installed in Claude Code", RoleSuccess, 3*time.Second)
	}
	return m, nil
}

func (m Model) statuslineNeedsReinstall(status statusline.Status) bool {
	return status.State == statusline.StateInstalled && !statusline.UsesRuntimeCommand(status.Command, m.deps.StatuslineCommand)
}

func (m Model) configPanelBodyWidth() int {
	return max(min(m.width-4, maxPanelBodyWidth), 24)
}

func truncateBodyLines(body string, width int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i, line := range lines {
		lines[i] = truncateANSI(line, width)
	}
	return strings.Join(lines, "\n")
}

func (m Model) updateConfigEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.configEditing = false
		m.configEditingField = ""
		m.configInput = ""
		m.configInputFresh = false
		return m, nil
	case tea.KeyEnter:
		m.commitConfigInput()
		return m, nil
	case tea.KeyBackspace:
		if m.configInputFresh {
			m.configInput = ""
			m.configInputFresh = false
			return m, nil
		}
		if len(m.configInput) > 0 {
			m.configInput = m.configInput[:len(m.configInput)-1]
		}
		return m, nil
	case tea.KeyRunes:
		if m.configInputFresh {
			m.configInput = string(msg.Runes)
			m.configInputFresh = false
		} else {
			m.configInput += string(msg.Runes)
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) startConfigEdit(field string) {
	m.configEditing = true
	m.configEditingField = field
	m.configChoiceField = ""
	m.configChoiceIndex = 0
	m.configResetConfirm = false
	m.configStatuslineConfirm = false
	m.configInput = m.configFieldValue(field)
	m.configInputFresh = true
	m.clearConfigFieldError(field)
}

func (m *Model) startConfigChoice(field string) {
	values := m.statuslineChoiceValues(field)
	m.configChoiceField = field
	m.configChoiceIndex = 0
	m.configChoiceOrder = nil
	if field == "config_statusline_sequence" {
		m.configChoiceOrder = append([]string(nil), m.configDraft.Statusline.Order...)
	}
	for index, value := range values {
		if value == m.configFieldValue(field) {
			m.configChoiceIndex = index
			break
		}
	}
	m.configEditing = false
	m.configEditingField = ""
	m.configInput = ""
	m.configInputFresh = false
	m.configResetConfirm = false
	m.configStatuslineConfirm = false
	m.clearConfigFieldError(field)
}

func (m Model) updateConfigChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	values := m.statuslineChoiceValues(m.configChoiceField)
	if len(values) == 0 {
		m.configChoiceField = ""
		m.configChoiceIndex = 0
		m.configChoiceOrder = nil
		return m, nil
	}
	switch msg.String() {
	case "up":
		if m.configChoiceField == "config_statusline_sequence" {
			if m.configChoiceIndex > 0 {
				m.configChoiceIndex--
			}
		} else {
			m.configChoiceIndex = (m.configChoiceIndex + len(values) - 1) % len(values)
		}
	case "down":
		if m.configChoiceField == "config_statusline_sequence" {
			if m.configChoiceIndex < len(values)-1 {
				m.configChoiceIndex++
			}
		} else {
			m.configChoiceIndex = (m.configChoiceIndex + 1) % len(values)
		}
	case "left":
		if m.configChoiceField == "config_statusline_sequence" {
			if m.configChoiceIndex > 0 {
				m.configDraft.Statusline.Order[m.configChoiceIndex], m.configDraft.Statusline.Order[m.configChoiceIndex-1] = m.configDraft.Statusline.Order[m.configChoiceIndex-1], m.configDraft.Statusline.Order[m.configChoiceIndex]
				m.configChoiceIndex--
			}
		} else {
			m.configChoiceIndex = (m.configChoiceIndex + len(values) - 1) % len(values)
		}
	case "right":
		if m.configChoiceField == "config_statusline_sequence" {
			if m.configChoiceIndex < len(values)-1 {
				m.configDraft.Statusline.Order[m.configChoiceIndex], m.configDraft.Statusline.Order[m.configChoiceIndex+1] = m.configDraft.Statusline.Order[m.configChoiceIndex+1], m.configDraft.Statusline.Order[m.configChoiceIndex]
				m.configChoiceIndex++
			}
		} else {
			m.configChoiceIndex = (m.configChoiceIndex + 1) % len(values)
		}
	case "enter":
		if name := statuslineFormatElementName(m.configChoiceField); name != "" {
			if element := statuslineElementConfigForDraft(&m.configDraft.Statusline, name); element != nil {
				element.Format, element.Layout = splitStatuslineFormatChoice(values[m.configChoiceIndex])
			}
		}
		m.clearConfigFieldError(m.configChoiceField)
		m.configChoiceField = ""
		m.configChoiceIndex = 0
		m.configChoiceOrder = nil
	case "esc":
		if m.configChoiceField == "config_statusline_sequence" && m.configChoiceOrder != nil {
			m.configDraft.Statusline.Order = append([]string(nil), m.configChoiceOrder...)
		}
		m.configChoiceField = ""
		m.configChoiceIndex = 0
		m.configChoiceOrder = nil
	}
	return m, nil
}

func statuslineFormatElementName(field string) string {
	if !strings.HasPrefix(field, "config_statusline_") || !strings.HasSuffix(field, "_format") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(field, "config_statusline_"), "_format")
}

func (m Model) configFieldValue(field string) string {
	switch field {
	case "config_reminder_thresholds":
		return thresholdsText(m.configDraft.ReminderThresholds)
	case "config_trigger":
		return strconv.Itoa(m.configDraft.KeepAlive.TriggerBeforeExpiryMinutes)
	case "config_countdown":
		return strconv.Itoa(m.configDraft.KeepAlive.CountdownSeconds)
	case "config_message":
		return m.configDraft.KeepAlive.Message
	case "config_max_sends":
		return strconv.Itoa(m.configDraft.KeepAlive.Scope.MaxSends)
	case "config_statusline_usage_format":
		return statuslineElementChoiceValue(m.configDraft.Statusline.Usage)
	case "config_statusline_warning_format":
		return statuslineElementChoiceValue(m.configDraft.Statusline.Warning)
	case "config_statusline_cache_format":
		return statuslineElementChoiceValue(m.configDraft.Statusline.Cache)
	case "config_statusline_sequence":
		return strings.Join(m.configDraft.Statusline.Order, ",")
	default:
		return ""
	}
}

func configFieldLabel(field string) string {
	switch field {
	case "config_reminder_thresholds":
		return "Reminder thresholds"
	case "config_trigger":
		return "KeepAlive trigger"
	case "config_countdown":
		return "Countdown"
	case "config_message":
		return "Message"
	case "config_max_sends":
		return "Max sends"
	case "config_statusline_usage_format":
		return "Usage format"
	case "config_statusline_warning_format":
		return "KA warning format"
	case "config_statusline_cache_format":
		return "Cache timing format"
	case "config_statusline_sequence":
		return "Sequence"
	default:
		return field
	}
}

func (m *Model) commitConfigInput() {
	input := strings.TrimSpace(m.configInput)
	switch m.configEditingField {
	case "config_reminder_thresholds":
		thresholds, err := parseThresholds(input)
		if err != nil {
			m.setConfigFieldError("config_reminder_thresholds", "reminder thresholds must be whole numbers from 1 to 99 in descending order.")
		} else {
			m.configDraft.ReminderThresholds = thresholds
			m.clearConfigFieldError("config_reminder_thresholds")
		}
	case "config_trigger":
		value, err := strconv.Atoi(input)
		if err != nil {
			m.setConfigFieldError("config_trigger", "trigger must be positive.")
		} else {
			m.configDraft.KeepAlive.TriggerBeforeExpiryMinutes = value
			m.clearConfigFieldError("config_trigger")
		}
	case "config_countdown":
		value, err := strconv.Atoi(input)
		if err != nil {
			m.setConfigFieldError("config_countdown", "countdown must be positive.")
		} else {
			m.configDraft.KeepAlive.CountdownSeconds = value
			m.clearConfigFieldError("config_countdown")
		}
	case "config_message":
		if input == "" {
			m.setConfigFieldError("config_message", "message cannot be empty.")
		} else {
			m.configDraft.KeepAlive.Message = input
			m.clearConfigFieldError("config_message")
		}
	case "config_max_sends":
		value, err := strconv.Atoi(input)
		if err != nil {
			m.setConfigFieldError("config_max_sends", "max sends must be positive.")
		} else {
			m.configDraft.KeepAlive.Scope.MaxSends = value
			m.clearConfigFieldError("config_max_sends")
		}
	}
	m.configEditing = false
	m.configEditingField = ""
	m.configInput = ""
	m.configInputFresh = false
	m.configResetConfirm = false
}

func (m Model) saveConfig() (tea.Model, tea.Cmd) {
	m.configStatuslineConfirm = false
	if err := m.configEditorValidation(); err != nil {
		m.setNotice("✕ Cannot save", RoleDanger, 3*time.Second)
		return m, nil
	}
	if m.deps.SaveConfig != nil {
		if err := m.deps.SaveConfig(m.configDraft); err != nil {
			m.setNotice("✕ Cannot save: "+err.Error(), RoleDanger, 3*time.Second)
			return m, nil
		}
	}
	m.configOriginal = m.configDraft
	m.configResetConfirm = false
	m.setNotice("✓ Saved", RoleSuccess, 3*time.Second)
	return m, nil
}

func (m Model) resetConfigDefaults() (tea.Model, tea.Cmd) {
	m.configStatuslineConfirm = false
	if !m.configResetConfirm {
		m.configResetConfirm = true
		return m, nil
	}
	defaults := config.Default()
	if m.deps.SaveConfig != nil {
		if err := m.deps.SaveConfig(defaults); err != nil {
			m.setNotice("✕ Cannot save: "+err.Error(), RoleDanger, 3*time.Second)
			return m, nil
		}
	}
	m.configDraft = defaults
	m.configOriginal = defaults
	m.configFieldErrors = map[string]string{}
	m.configResetConfirm = false
	m.configEditing = false
	m.configEditingField = ""
	m.configInput = ""
	m.configInputFresh = false
	m.configChoiceField = ""
	m.configChoiceIndex = 0
	m.configChoiceOrder = nil
	m.setNotice("✓ Saved", RoleSuccess, 3*time.Second)
	return m, nil
}

func (m Model) cancelConfig() (tea.Model, tea.Cmd) {
	m.configDraft = m.configOriginal
	m.configEditing = false
	m.configEditingField = ""
	m.configInput = ""
	m.configInputFresh = false
	m.configChoiceField = ""
	m.configChoiceIndex = 0
	m.configChoiceOrder = nil
	m.configFieldErrors = map[string]string{}
	m.configResetConfirm = false
	m.configStatuslineConfirm = false
	if m.configReturnRoute != "" {
		m.route = m.configReturnRoute
		m.configReturnRoute = ""
		m.focusIndex = m.defaultFocusIndex()
		return m, nil
	}
	return m, tea.Quit
}

func configBehaviorSummary(cfg config.Config) string {
	summary := config.EffectiveKeepAliveSummary(cfg)
	styles := DefaultStyles()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s %s left · %s\n", padANSI(styles.Render(RoleCacheTier, "1h cache"), 10), styles.Render(RoleInfo, formatStatusDuration(summary.EffectiveTriggerSeconds1Hour)), countdownOutcome(summary.EffectiveCountdown1Hour, summary.SendPausedFor1Hour))
	fmt.Fprintf(&b, "  %s %s left · %s\n", padANSI(styles.Render(RoleCacheTier, "5m cache"), 10), styles.Render(RoleInfo, formatStatusDuration(summary.EffectiveTriggerSeconds5Minute)), countdownOutcome(summary.EffectiveCountdown5Minute, summary.SendPausedFor5Minute))
	fmt.Fprintf(&b, "  %s stop after %s automatic sends\n", padANSI(styles.Render(RoleMuted, "Sends"), 10), styles.Render(RoleInfo, fmt.Sprintf("%d", cfg.KeepAlive.Scope.MaxSends)))
	return b.String()
}

func countdownOutcome(countdown int, disabled bool) string {
	if disabled {
		return DefaultStyles().Render(RoleWarning, "automatic send paused for affected sessions")
	}
	return fmt.Sprintf("send after %s unless canceled", DefaultStyles().Render(RoleInfo, formatStatusDuration(countdown)))
}

func thresholdsText(thresholds []int) string {
	parts := make([]string, 0, len(thresholds))
	for _, threshold := range thresholds {
		parts = append(parts, strconv.Itoa(threshold))
	}
	return strings.Join(parts, ", ")
}

func parseThresholds(input string) ([]int, error) {
	parts := strings.Split(input, ",")
	thresholds := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		thresholds = append(thresholds, value)
	}
	return thresholds, nil
}

func (m Model) configEditorValidation() error {
	if len(m.configFieldErrors) > 0 {
		messages := make([]string, 0, len(m.configFieldErrors))
		for _, message := range m.configFieldErrors {
			messages = append(messages, strings.TrimSuffix(message, "."))
		}
		return config.ValidationError{Messages: messages}
	}
	cfg := m.configDraft
	if err := config.Validate(cfg); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.KeepAlive.Message) == "" {
		return config.ValidationError{Messages: []string{"message cannot be empty"}}
	}
	summary := config.EffectiveKeepAliveSummary(cfg)
	if summary.SendPausedFor5Minute {
		return config.ValidationError{Messages: []string{"countdown may not fit the 5m cache trigger window"}}
	}
	return nil
}

func countdownWarnsFor5Minute(cfg config.Config) bool {
	summary := config.EffectiveKeepAliveSummary(cfg)
	return summary.SendPausedFor5Minute
}

func (m Model) configFieldError(field string) string {
	if message := m.configFieldErrors[field]; message != "" {
		return message
	}
	cfg := m.configDraft
	switch field {
	case "config_reminder_thresholds":
		err := config.Validate(config.Config{ReminderThresholds: cfg.ReminderThresholds, KeepAlive: config.Default().KeepAlive})
		if err != nil && strings.Contains(err.Error(), "reminder thresholds") {
			return "reminder thresholds must be whole numbers from 1 to 99 in descending order."
		}
	case "config_trigger":
		if cfg.KeepAlive.TriggerBeforeExpiryMinutes <= 0 {
			return "trigger must be positive."
		}
	case "config_countdown":
		if cfg.KeepAlive.CountdownSeconds <= 0 {
			return "countdown must be positive."
		}
	case "config_message":
		if strings.TrimSpace(cfg.KeepAlive.Message) == "" {
			return "message cannot be empty."
		}
	case "config_max_sends":
		if cfg.KeepAlive.Scope.MaxSends <= 0 {
			return "max sends must be positive."
		}
	case "config_statusline_usage_format", "config_statusline_warning_format", "config_statusline_cache_format":
		name := strings.TrimSuffix(strings.TrimPrefix(field, "config_statusline_"), "_format")
		element := statuslineElementConfigForDraft(&cfg.Statusline, name)
		if element == nil {
			return "unknown statusline element."
		}
		if name == config.StatuslineElementWarning {
			if element.Format != config.StatuslineWarningFormatAlert && element.Format != config.StatuslineWarningFormatVerbose {
				return "format must be alert_only or verbose."
			}
		} else if element.Format != config.StatuslineFormatFull && element.Format != config.StatuslineFormatCompact {
			return "format must be full or compact."
		}
	}
	return ""
}

func (m *Model) setConfigFieldError(field, message string) {
	if m.configFieldErrors == nil {
		m.configFieldErrors = map[string]string{}
	}
	m.configFieldErrors[field] = message
}

func (m *Model) clearConfigFieldError(field string) {
	if m.configFieldErrors == nil {
		return
	}
	delete(m.configFieldErrors, field)
}
