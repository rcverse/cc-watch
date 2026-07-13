package config

type Config struct {
	ReminderThresholds []int            `json:"reminder_thresholds"`
	KeepAlive          KeepAliveConfig  `json:"keep_alive"`
	Statusline         StatuslineConfig `json:"statusline"`
}

const (
	StatuslineLayoutSameLine = "same_line"
	StatuslineLayoutNewLine  = "new_line"
	StatuslineFormatFull     = "full"
	StatuslineFormatCompact  = "compact"
)

type StatuslineConfig struct {
	Layout string `json:"layout"`
	Format string `json:"format"`
}

type KeepAliveConfig struct {
	TriggerBeforeExpiryMinutes int         `json:"trigger_before_expiry_m"`
	CountdownSeconds           int         `json:"countdown_s"`
	Message                    string      `json:"message"`
	Scope                      ScopeConfig `json:"scope"`
}

type ScopeConfig struct {
	MaxSends int `json:"max_sends"`
}

func Default() Config {
	return Config{
		ReminderThresholds: []int{20, 10},
		KeepAlive: KeepAliveConfig{
			TriggerBeforeExpiryMinutes: 5,
			CountdownSeconds:           30,
			Message:                    `Keep-alive check. Reply "yes" only.`,
			Scope: ScopeConfig{
				MaxSends: 5,
			},
		},
		Statusline: StatuslineConfig{
			Layout: StatuslineLayoutSameLine,
			Format: StatuslineFormatFull,
		},
	}
}
