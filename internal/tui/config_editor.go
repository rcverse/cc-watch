package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-cache/internal/config"
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
	b.WriteString(styles.Render(RoleIdentity, "Claude Code Cache / config"))
	b.WriteString("\n")
	b.WriteString(styles.Render(RoleSeparator, strings.Repeat("─", 46)))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "%s\n", m.configRow("config_reminder_thresholds", "Reminder thresholds", thresholdsText(cfg.ReminderThresholds)+"%", "alerts only; no Claude message"))
	if message := m.configFieldError("config_reminder_thresholds"); message != "" {
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	fmt.Fprintf(&b, "%s\n", m.configRow("config_trigger", "KeepAlive trigger", fmt.Sprintf("%dm", cfg.KeepAlive.TriggerBeforeExpiryMinutes), "before cache expiry"))
	if message := m.configFieldError("config_trigger"); message != "" {
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	fmt.Fprintf(&b, "%s\n", m.configRow("config_countdown", "Countdown", fmt.Sprintf("%ds", cfg.KeepAlive.CountdownSeconds), "cancel window before any send"))
	if message := m.configFieldError("config_countdown"); message != "" {
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	if countdownWarnsFor5Minute(cfg) {
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleWarning, "Warning: countdown may not fit the 5m cache trigger window."))
	}
	fmt.Fprintf(&b, "%s\n", m.configRow("config_message", "Message", truncateEnd(cfg.KeepAlive.Message, 38), "Claude prompt text"))
	fmt.Fprintf(&b, "%s\n", m.configRow("config_autosend", "Auto-send", onOffText(cfg.KeepAlive.AutoSend, true), autoSendConfigDetail(cfg.KeepAlive.AutoSend)))
	if cfg.KeepAlive.AutoSend {
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleWarning, "Warning: Auto-send default may send a Claude message after countdown."))
	}
	fmt.Fprintf(&b, "%s\n", m.configRow("config_max_sends", "Max sends", fmt.Sprintf("%d", cfg.KeepAlive.Scope.MaxSends), "per session scope"))
	if message := m.configFieldError("config_max_sends"); message != "" {
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleDanger, "Error: "+message))
	}
	if m.configEditing {
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleInfo, fmt.Sprintf("Editing %s: %s", configFieldLabel(m.configEditingField), m.configInput)))
		fmt.Fprintf(&b, "    %s\n", styles.Render(RoleMuted, "Enter saves field · Esc cancels edit"))
	}

	b.WriteString("\nWhat will happen\n")
	b.WriteString(configBehaviorSummary(cfg))

	b.WriteString("\nValidation\n")
	if err := m.configEditorValidation(); err != nil {
		fmt.Fprintf(&b, "  %s\n", styles.Render(RoleDanger, "Cannot save. "+err.Error()))
	} else {
		fmt.Fprintf(&b, "  %s\n", styles.Render(RoleSuccess, "OK"))
	}
	if m.configSaveError != "" {
		fmt.Fprintf(&b, "  %s\n", styles.Render(RoleDanger, "Save failed: "+m.configSaveError))
	}
	if m.configResetConfirm {
		b.WriteString("\n  Reset defaults? This will replace KeepAlive defaults.\n")
		b.WriteString("  Press d again to confirm, esc to keep current settings.\n")
	}
	b.WriteString("\nActions\n")
	fmt.Fprintf(&b, "%s\n", m.configRow("config_save", "Save", "", "write config"))
	fmt.Fprintf(&b, "%s\n", m.configRow("config_reset", "Reset defaults", "", "requires confirmation"))
	fmt.Fprintf(&b, "%s\n", m.configRow("config_cancel", "Cancel", "", "discard edits"))
	b.WriteString("up/down move  enter edit  space toggle  s save  d reset(confirm)  esc cancel\n")
	if m.helpOpen {
		b.WriteString("\n")
		b.WriteString(m.helpText())
	}
	return b.String()
}

func (m Model) configRow(action string, label string, value string, detail string) string {
	styles := DefaultStyles()
	marker := " "
	if m.FocusedAction() == action {
		marker = styles.Render(RoleIdentity, "›")
	}
	return truncateANSI(fmt.Sprintf("  %s %-20s %-10s %s", marker, label, value, detail), m.width)
}

func autoSendConfigDetail(enabled bool) string {
	if enabled {
		return "sends Claude message after countdown"
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
	m.lastAction = "cancel_config"
	return m, tea.Quit
}

func configBehaviorSummary(cfg config.Config) string {
	summary := config.EffectiveKeepAliveSummary(cfg)
	var b strings.Builder
	fmt.Fprintf(&b, "  1h cache: countdown starts at %02dm left, %s\n", summary.EffectiveTriggerSeconds1Hour/60, countdownOutcome(cfg.KeepAlive.AutoSend, summary.EffectiveCountdown1Hour, summary.AutoSendDisabledFor1Hour))
	fmt.Fprintf(&b, "  5m cache: countdown starts at %02dm left, %s\n", summary.EffectiveTriggerSeconds5Minute/60, countdownOutcome(cfg.KeepAlive.AutoSend, summary.EffectiveCountdown5Minute, summary.AutoSendDisabledFor5Minute))
	fmt.Fprintf(&b, "  Scope: stop after %d attempted or successful send\n", cfg.KeepAlive.Scope.MaxSends)
	return b.String()
}

func countdownOutcome(autoSend bool, countdown int, disabled bool) string {
	if !autoSend {
		return "manual prompt only; no auto-send"
	}
	if disabled {
		return "auto-send disabled for affected sessions"
	}
	return fmt.Sprintf("auto-send after %ds unless canceled", countdown)
}

func autoSendDefaultText(enabled bool) string {
	if enabled {
		return "[x] enabled, sends Claude message"
	}
	return "[ ] disabled, manual prompt only"
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
