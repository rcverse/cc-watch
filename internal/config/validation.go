package config

import (
	"errors"
	"strings"
)

const keepAliveSafetyMarginSeconds = 30

type ValidationError struct {
	Messages []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Messages, "; ")
}

type KeepAliveSummary struct {
	SendPausedFor1Hour             bool
	SendPausedFor5Minute           bool
	EffectiveCountdown1Hour        int
	EffectiveCountdown5Minute      int
	EffectiveTriggerSeconds1Hour   int
	EffectiveTriggerSeconds5Minute int
	Warning                        string
}

func Validate(cfg Config) error {
	var messages []string
	if cfg.RecentSessions <= 0 {
		messages = append(messages, "recent_sessions must be positive")
	}
	if err := validateThresholds(cfg.ReminderThresholds); err != nil {
		messages = append(messages, err.Error())
	}
	if cfg.KeepAlive.TriggerBeforeExpiryMinutes <= 0 {
		messages = append(messages, "trigger_before_expiry_m must be positive")
	}
	if cfg.KeepAlive.CountdownSeconds <= 0 {
		messages = append(messages, "countdown_s must be positive")
	}
	if cfg.KeepAlive.Scope.MaxSends <= 0 {
		messages = append(messages, "scope.max_sends must be positive")
	}
	validateStatuslineElement(&messages, "usage", cfg.Statusline.Usage, StatuslineFormatFull, StatuslineFormatCompact)
	validateStatuslineElement(&messages, "warning", cfg.Statusline.Warning, StatuslineWarningFormatAlert, StatuslineWarningFormatVerbose)
	validateStatuslineElement(&messages, "cache", cfg.Statusline.Cache, StatuslineFormatFull, StatuslineFormatCompact)
	if !validStatuslineOrder(cfg.Statusline.Order) {
		messages = append(messages, "statusline.order must contain usage, warning, and cache exactly once")
	}
	if len(messages) > 0 {
		return ValidationError{Messages: messages}
	}
	return nil
}

func validateStatuslineElement(messages *[]string, name string, element StatuslineElementConfig, formats ...string) {
	if element.Layout != StatuslineLayoutSameLine && element.Layout != StatuslineLayoutNewLine {
		*messages = append(*messages, "statusline."+name+".layout must be same_line or new_line")
	}
	for _, format := range formats {
		if element.Format == format {
			return
		}
	}
	*messages = append(*messages, "statusline."+name+".format is invalid")
}

func validStatuslineOrder(order []string) bool {
	if len(order) != 3 {
		return false
	}
	seen := map[string]bool{}
	for _, element := range order {
		if element != StatuslineElementUsage && element != StatuslineElementWarning && element != StatuslineElementCache {
			return false
		}
		if seen[element] {
			return false
		}
		seen[element] = true
	}
	return len(seen) == 3
}

func EffectiveKeepAliveSummary(cfg Config) KeepAliveSummary {
	trigger := cfg.KeepAlive.TriggerBeforeExpiryMinutes * 60
	summary := KeepAliveSummary{
		EffectiveTriggerSeconds1Hour:   min(trigger, 3600/5),
		EffectiveTriggerSeconds5Minute: min(trigger, 300/5),
	}
	summary.EffectiveCountdown1Hour, summary.SendPausedFor1Hour = effectiveCountdown(cfg.KeepAlive.CountdownSeconds, summary.EffectiveTriggerSeconds1Hour)
	summary.EffectiveCountdown5Minute, summary.SendPausedFor5Minute = effectiveCountdown(cfg.KeepAlive.CountdownSeconds, summary.EffectiveTriggerSeconds5Minute)
	if summary.SendPausedFor1Hour || summary.SendPausedFor5Minute {
		summary.Warning = "automatic sends will pause for affected sessions because countdown plus safety margin does not fit"
	}
	return summary
}

func validateThresholds(thresholds []int) error {
	if len(thresholds) == 0 {
		return errors.New("reminder thresholds must not be empty")
	}
	previous := 100
	for _, threshold := range thresholds {
		if threshold < 1 || threshold > 99 {
			return errors.New("reminder thresholds must be whole numbers from 1 to 99")
		}
		if threshold >= previous {
			return errors.New("reminder thresholds must be in descending order")
		}
		previous = threshold
	}
	return nil
}

func effectiveCountdown(configuredCountdown, effectiveTrigger int) (int, bool) {
	if effectiveTrigger <= keepAliveSafetyMarginSeconds {
		return 0, true
	}
	latestSafeCountdown := effectiveTrigger - keepAliveSafetyMarginSeconds
	if configuredCountdown > latestSafeCountdown {
		return latestSafeCountdown, true
	}
	return configuredCountdown, false
}
