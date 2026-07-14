package config

type Config struct {
	ReminderThresholds []int            `json:"reminder_thresholds"`
	KeepAlive          KeepAliveConfig  `json:"keep_alive"`
	Statusline         StatuslineConfig `json:"statusline"`
}

const (
	StatuslineLayoutSameLine       = "same_line"
	StatuslineLayoutNewLine        = "new_line"
	StatuslineFormatFull           = "full"
	StatuslineFormatCompact        = "compact"
	StatuslineWarningFormatAlert   = "alert_only"
	StatuslineWarningFormatVerbose = "verbose"

	StatuslineElementUsage   = "usage"
	StatuslineElementWarning = "warning"
	StatuslineElementCache   = "cache"
)

type StatuslineElementConfig struct {
	Enabled bool   `json:"enabled"`
	Layout  string `json:"layout"`
	Format  string `json:"format"`
}

type StatuslineConfig struct {
	Usage   StatuslineElementConfig `json:"usage"`
	Warning StatuslineElementConfig `json:"warning"`
	Cache   StatuslineElementConfig `json:"cache"`
	Order   []string                `json:"order"`
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
		Statusline: DefaultStatusline(),
	}
}

func DefaultStatusline() StatuslineConfig {
	return StatuslineConfig{
		Usage: StatuslineElementConfig{
			Enabled: true,
			Layout:  StatuslineLayoutSameLine,
			Format:  StatuslineFormatFull,
		},
		Warning: StatuslineElementConfig{
			Enabled: true,
			Layout:  StatuslineLayoutSameLine,
			Format:  StatuslineWarningFormatAlert,
		},
		Cache: StatuslineElementConfig{
			Enabled: true,
			Layout:  StatuslineLayoutNewLine,
			Format:  StatuslineFormatFull,
		},
		Order: []string{StatuslineElementUsage, StatuslineElementWarning, StatuslineElementCache},
	}
}

func NormalizeStatusline(cfg StatuslineConfig) StatuslineConfig {
	defaults := DefaultStatusline()
	if cfg.Usage.Layout == "" {
		cfg.Usage.Layout = defaults.Usage.Layout
	}
	if cfg.Usage.Format == "" {
		cfg.Usage.Format = defaults.Usage.Format
	}
	if cfg.Warning.Layout == "" {
		cfg.Warning.Layout = defaults.Warning.Layout
	}
	if cfg.Warning.Format == "" {
		cfg.Warning.Format = defaults.Warning.Format
	}
	if cfg.Cache.Layout == "" {
		cfg.Cache.Layout = defaults.Cache.Layout
	}
	if cfg.Cache.Format == "" {
		cfg.Cache.Format = defaults.Cache.Format
	}
	if len(cfg.Order) == 0 {
		cfg.Order = append([]string(nil), defaults.Order...)
	}
	return cfg
}
