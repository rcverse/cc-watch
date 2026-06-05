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
	AutoSendDisabledFor1Hour       bool
	AutoSendDisabledFor5Minute     bool
	EffectiveCountdown1Hour        int
	EffectiveCountdown5Minute      int
	EffectiveTriggerSeconds1Hour   int
	EffectiveTriggerSeconds5Minute int
	Warning                        string
}

func Validate(cfg Config) error {
	var messages []string
	if err := validateThresholds(cfg.ReminderThresholds); err != nil {
		messages = append(messages, err.Error())
	}
	if cfg.KeepAlive.TriggerBeforeExpiryMinutes <= 0 {
		messages = append(messages, "trigger_before_expiry_m must be positive")
	}
	if cfg.KeepAlive.CountdownSeconds <= 0 {
		messages = append(messages, "countdown_s must be positive")
	}
	if cfg.KeepAlive.Scope.Mode != "max_sends" {
		messages = append(messages, "scope.mode must be max_sends")
	}
	if cfg.KeepAlive.Scope.MaxSends <= 0 {
		messages = append(messages, "scope.max_sends must be positive")
	}
	if len(messages) > 0 {
		return ValidationError{Messages: messages}
	}
	return nil
}

func EffectiveKeepAliveSummary(cfg Config) KeepAliveSummary {
	trigger := cfg.KeepAlive.TriggerBeforeExpiryMinutes * 60
	summary := KeepAliveSummary{
		EffectiveTriggerSeconds1Hour:   minInt(trigger, 3600/5),
		EffectiveTriggerSeconds5Minute: minInt(trigger, 300/5),
	}
	summary.EffectiveCountdown1Hour, summary.AutoSendDisabledFor1Hour = effectiveCountdown(cfg.KeepAlive.CountdownSeconds, summary.EffectiveTriggerSeconds1Hour)
	summary.EffectiveCountdown5Minute, summary.AutoSendDisabledFor5Minute = effectiveCountdown(cfg.KeepAlive.CountdownSeconds, summary.EffectiveTriggerSeconds5Minute)
	if cfg.KeepAlive.AutoSend && (summary.AutoSendDisabledFor1Hour || summary.AutoSendDisabledFor5Minute) {
		summary.Warning = "auto-send will be disabled for affected sessions because countdown plus safety margin does not fit"
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
