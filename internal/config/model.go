package config

type Config struct {
	ReminderThresholds []int           `json:"reminder_thresholds"`
	KeepAlive          KeepAliveConfig `json:"keep_alive"`
}

type KeepAliveConfig struct {
	TriggerBeforeExpiryMinutes int         `json:"trigger_before_expiry_m"`
	CountdownSeconds           int         `json:"countdown_s"`
	Message                    string      `json:"message"`
	AutoSend                   bool        `json:"auto_send"`
	Scope                      ScopeConfig `json:"scope"`
}

type ScopeConfig struct {
	Mode     string `json:"mode"`
	MaxSends int    `json:"max_sends"`
}

type SessionDefaults struct {
	ReminderThresholds []int
	KeepAliveAutoSend  bool
	KeepAliveMaxSends  int
}

func Default() Config {
	return Config{
		ReminderThresholds: []int{20, 10},
		KeepAlive: KeepAliveConfig{
			TriggerBeforeExpiryMinutes: 5,
			CountdownSeconds:           30,
			Message:                    `Keep-alive check. Reply "yes" only.`,
			AutoSend:                   true,
			Scope: ScopeConfig{
				Mode:     "max_sends",
				MaxSends: 1,
			},
		},
	}
}

func NewSessionDefaults(cfg Config) SessionDefaults {
	return SessionDefaults{
		ReminderThresholds: append([]int(nil), cfg.ReminderThresholds...),
		KeepAliveAutoSend:  cfg.KeepAlive.AutoSend,
		KeepAliveMaxSends:  cfg.KeepAlive.Scope.MaxSends,
	}
}
