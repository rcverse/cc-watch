package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
)

var configFocusActions = []string{
	"config_reminder_thresholds",
	"config_trigger",
	"config_countdown",
	"config_message",
	"config_autosend",
	"config_max_sends",
	"config_save",
	"config_reset",
	"config_cancel",
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
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_reminder_thresholds", "Reminder thresholds", thresholdsText(cfg.ReminderThresholds)+"%", "alerts only; no Claude message"))
	if message := m.configFieldError("config_reminder_thresholds"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_trigger", "KeepAlive trigger", fmt.Sprintf("%dm", cfg.KeepAlive.TriggerBeforeExpiryMinutes), "before cache expiry"))
	if message := m.configFieldError("config_trigger"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_countdown", "Countdown", fmt.Sprintf("%ds", cfg.KeepAlive.CountdownSeconds), "cancel window before any send"))
	if message := m.configFieldError("config_countdown"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	if countdownWarnsFor5Minute(cfg) {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleWarning, "Warning: countdown may not fit the 5m cache trigger window."))
	}
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_message", "Message", truncateEnd(cfg.KeepAlive.Message, 38), "Claude prompt text"))
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_autosend", "Auto-send", onOffText(cfg.KeepAlive.AutoSend, true), autoSendConfigDetail(cfg.KeepAlive.AutoSend)))
	fmt.Fprintf(&settings, "%s\n", m.configRow("config_max_sends", "Max sends", fmt.Sprintf("%d", cfg.KeepAlive.Scope.MaxSends), "per session scope"))
	if message := m.configFieldError("config_max_sends"); message != "" {
		fmt.Fprintf(&settings, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	b.WriteString(m.renderConfigPanel("Settings", settings.String()))
	if m.configEditing {
		var edit strings.Builder
		fmt.Fprintf(&edit, "%s %s\n", styles.Render(RoleInfo, configFieldLabel(m.configEditingField)), styles.Render(RoleMuted, "is active"))
		fmt.Fprintf(&edit, "%s  %s%s\n", styles.Render(RoleMuted, "Current input"), m.configInput, styles.Render(RoleIdentity, "▌"))
		fmt.Fprintf(&edit, "%s\n", styles.Render(RoleMuted, "↵ save field  ⎋ cancel edit"))
		b.WriteString(m.renderConfigPanel("Editing", edit.String()))
	}

	var preview strings.Builder
	preview.WriteString(configBehaviorSummary(cfg))
	if cfg.KeepAlive.AutoSend {
		fmt.Fprintf(&preview, "%s\n", styles.Render(RoleWarning, "Auto-send is ON: Claude message after countdown unless canceled."))
	}
	b.WriteString(m.renderConfigPanel("Preview", preview.String()))

	var validation strings.Builder
	if err := m.configEditorValidation(); err != nil {
		fmt.Fprintf(&validation, "%s\n", styles.Render(RoleDanger, "✗ Cannot save. "+err.Error()))
	} else {
		fmt.Fprintf(&validation, "%s\n", styles.Render(RoleSuccess, "✓ OK"))
	}
	if m.configSaveError != "" {
		fmt.Fprintf(&validation, "%s\n", styles.Render(RoleDanger, "✗ Save failed: "+m.configSaveError))
	}
	if m.configResetConfirm {
		validation.WriteString("\n")
		validation.WriteString("Reset defaults? This will replace KeepAlive defaults.\n")
		validation.WriteString("Press d again to confirm, ⎋ to keep current settings.\n")
	}
	b.WriteString(m.renderConfigPanel("Validation", validation.String()))

	var actions strings.Builder
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_save", "Save", "", "write config"))
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_reset", "Reset defaults", "", "requires confirmation"))
	fmt.Fprintf(&actions, "%s\n", m.configRow("config_cancel", "Cancel", "", "discard edits"))
	b.WriteString(m.renderConfigPanel("Actions", actions.String()))
	b.WriteString(cueLine("↑↓ move  ↵ edit  space toggle  s save  d reset  ⎋ cancel"))
	return b.String()
}

func (m Model) configRow(action string, label string, value string, detail string) string {
	styles := DefaultStyles()
	marker := " "
	if m.FocusedAction() == action {
		marker = styles.Render(RoleIdentity, "›")
	}
	return truncateANSI(fmt.Sprintf("  %s %-20s %-10s %s", marker, label, value, styles.Render(RoleMuted, detail)), m.configPanelBodyWidth())
}

func (m Model) renderConfigPanel(title string, body string) string {
	width := m.configPanelBodyWidth()
	return RenderPanelWidth(DefaultStyles().Render(RoleIdentity, title), truncateBodyLines(body, width), width)
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

func autoSendConfigDetail(enabled bool) string {
	if enabled {
		return "send after countdown"
	}
	return "manual prompt only"
}

func (m Model) updateConfigEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.configEditing = false
		m.configEditingField = ""
		m.configInput = ""
		m.configInputFresh = false
		m.lastAction = "cancel_config_edit"
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
	m.configResetConfirm = false
	m.configInput = m.configFieldValue(field)
	m.configInputFresh = true
	m.clearConfigFieldError(field)
	m.lastAction = "edit_" + field
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
	m.lastAction = "commit_config_edit"
}

func (m *Model) toggleConfigAutoSend() {
	m.configDraft.KeepAlive.AutoSend = !m.configDraft.KeepAlive.AutoSend
	m.configResetConfirm = false
	m.lastAction = "toggle_config_autosend"
}

func (m Model) saveConfig() (tea.Model, tea.Cmd) {
	if err := m.configEditorValidation(); err != nil {
		m.lastAction = "save_config_invalid"
		return m, nil
	}
	if m.deps.SaveConfig != nil {
		if err := m.deps.SaveConfig(m.configDraft); err != nil {
			m.configSaveError = err.Error()
			m.lastAction = "save_config_failed"
			return m, nil
		}
	}
	m.configOriginal = m.configDraft
	m.configSaveError = ""
	m.configResetConfirm = false
	m.lastAction = "save_config"
	return m, nil
}

func (m Model) resetConfigDefaults() (tea.Model, tea.Cmd) {
	if !m.configResetConfirm {
		m.configResetConfirm = true
		m.lastAction = "reset_defaults_confirm"
		return m, nil
	}
	defaults := config.Default()
	if m.deps.SaveConfig != nil {
		if err := m.deps.SaveConfig(defaults); err != nil {
			m.configSaveError = err.Error()
			m.lastAction = "reset_defaults_failed"
			return m, nil
		}
	}
	m.configDraft = defaults
	m.configOriginal = defaults
	m.configFieldErrors = map[string]string{}
	m.configSaveError = ""
	m.configResetConfirm = false
	m.configEditing = false
	m.configEditingField = ""
	m.configInput = ""
	m.configInputFresh = false
	m.lastAction = "reset_defaults"
	return m, nil
}

func (m Model) cancelConfig() (tea.Model, tea.Cmd) {
	m.configDraft = m.configOriginal
	m.configEditing = false
	m.configEditingField = ""
	m.configInput = ""
	m.configInputFresh = false
	m.configFieldErrors = map[string]string{}
	m.configResetConfirm = false
	if m.configReturnRoute != "" {
		m.route = m.configReturnRoute
		m.configReturnRoute = ""
		m.focusIndex = m.defaultFocusIndex()
		m.lastAction = "back_to_list"
		return m, nil
	}
	m.lastAction = "cancel_config"
	return m, tea.Quit
}

func configBehaviorSummary(cfg config.Config) string {
	summary := config.EffectiveKeepAliveSummary(cfg)
	styles := DefaultStyles()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s %s left · %s\n", padANSI(styles.Render(RoleCacheTier, "1h cache"), 10), styles.Render(RoleInfo, formatStatusDuration(summary.EffectiveTriggerSeconds1Hour)), countdownOutcome(cfg.KeepAlive.AutoSend, summary.EffectiveCountdown1Hour, summary.AutoSendDisabledFor1Hour))
	fmt.Fprintf(&b, "  %s %s left · %s\n", padANSI(styles.Render(RoleCacheTier, "5m cache"), 10), styles.Render(RoleInfo, formatStatusDuration(summary.EffectiveTriggerSeconds5Minute)), countdownOutcome(cfg.KeepAlive.AutoSend, summary.EffectiveCountdown5Minute, summary.AutoSendDisabledFor5Minute))
	fmt.Fprintf(&b, "  %s stop after %s attempted or successful send\n", padANSI(styles.Render(RoleMuted, "Scope"), 10), styles.Render(RoleInfo, fmt.Sprintf("%d", cfg.KeepAlive.Scope.MaxSends)))
	return b.String()
}

func countdownOutcome(autoSend bool, countdown int, disabled bool) string {
	if !autoSend {
		return "manual prompt only; no auto-send"
	}
	if disabled {
		return DefaultStyles().Render(RoleWarning, "auto-send disabled for affected sessions")
	}
	return fmt.Sprintf("auto-send after %s unless canceled", DefaultStyles().Render(RoleInfo, formatStatusDuration(countdown)))
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
	if cfg.KeepAlive.AutoSend && summary.AutoSendDisabledFor5Minute {
		return config.ValidationError{Messages: []string{"countdown may not fit the 5m cache trigger window"}}
	}
	return nil
}

func countdownWarnsFor5Minute(cfg config.Config) bool {
	summary := config.EffectiveKeepAliveSummary(cfg)
	return cfg.KeepAlive.AutoSend && summary.AutoSendDisabledFor5Minute
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
