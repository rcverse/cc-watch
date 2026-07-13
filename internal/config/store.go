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
	if cfg.Statusline.Layout == "" {
		cfg.Statusline.Layout = StatuslineLayoutSameLine
	}
	if cfg.Statusline.Format == "" {
		cfg.Statusline.Format = StatuslineFormatFull
	}
	if err := Validate(cfg); err != nil {
		return Default(), nil
	}
	return cfg, nil
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
