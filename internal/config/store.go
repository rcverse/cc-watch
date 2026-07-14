package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ConfigPath(home string) string {
	return filepath.Join(home, ".config", "cc-watch", "config.json")
}

func Load(home string) (Config, error) {
	path := ConfigPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Config{}, err
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), nil
	}
	if !hasModernStatuslineConfig(data) {
		cfg.Statusline = migrateLegacyStatusline(data)
	} else {
		cfg.Statusline = NormalizeStatusline(cfg.Statusline)
	}
	if err := Validate(cfg); err != nil {
		return Default(), nil
	}
	return cfg, nil
}

func hasModernStatuslineConfig(data []byte) bool {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return false
	}
	var statusline map[string]json.RawMessage
	if err := json.Unmarshal(root["statusline"], &statusline); err != nil {
		return false
	}
	for _, field := range []string{"usage", "warning", "cache", "order"} {
		if _, ok := statusline[field]; ok {
			return true
		}
	}
	return false
}

func migrateLegacyStatusline(data []byte) StatuslineConfig {
	cfg := DefaultStatusline()
	var legacy struct {
		Statusline struct {
			Layout string `json:"layout"`
			Format string `json:"format"`
		} `json:"statusline"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return cfg
	}
	if legacy.Statusline.Layout == StatuslineLayoutSameLine || legacy.Statusline.Layout == StatuslineLayoutNewLine {
		cfg.Usage.Layout = legacy.Statusline.Layout
	}
	if legacy.Statusline.Format == StatuslineFormatFull || legacy.Statusline.Format == StatuslineFormatCompact {
		cfg.Usage.Format = legacy.Statusline.Format
	}
	return cfg
}

func Save(home string, cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	path := ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
