package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type WarningCode string

const (
	WarningInvalidJSON   WarningCode = "invalid_json"
	WarningInvalidConfig WarningCode = "invalid_config"
)

type Warning struct {
	Code    WarningCode
	Message string
}

type LoadResult struct {
	Config   Config
	Warnings []Warning
}

func ConfigPath(home string) string {
	return filepath.Join(home, ".config", "cc-watch", "config.json")
}

func Load(home string) (LoadResult, error) {
	path := ConfigPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LoadResult{Config: Default()}, nil
		}
		return LoadResult{}, err
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return LoadResult{
			Config: Default(),
			Warnings: []Warning{{
				Code:    WarningInvalidJSON,
				Message: err.Error(),
			}},
		}, nil
	}
	if err := Validate(cfg); err != nil {
		return LoadResult{
			Config: Default(),
			Warnings: []Warning{{
				Code:    WarningInvalidConfig,
				Message: err.Error(),
			}},
		}, nil
	}
	return LoadResult{Config: cfg}, nil
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

func Reset(home string) error {
	return Save(home, Default())
}
