package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfigMatchesProductDefaults(t *testing.T) {
	cfg := Default()

	want := Config{
		RecentSessions:     10,
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

	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("Default() = %#v, want %#v", cfg, want)
	}
}

func TestLoadMigratesLegacyStatuslineSettingsToUsage(t *testing.T) {
	home := t.TempDir()
	path := ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"statusline":{"layout":"new_line","format":"compact"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if result.Statusline.Usage.Layout != StatuslineLayoutNewLine || result.Statusline.Usage.Format != StatuslineFormatCompact {
		t.Fatalf("usage = %#v, want migrated legacy layout and format", result.Statusline.Usage)
	}
	if result.Statusline.Cache != DefaultStatusline().Cache {
		t.Fatalf("cache = %#v, want new default cache element", result.Statusline.Cache)
	}
}

func TestLoadReadsConfigFromHome(t *testing.T) {
	home := t.TempDir()
	path := ConfigPath(home)
	writeConfigFile(t, path, Config{
		RecentSessions:     10,
		ReminderThresholds: []int{30, 15, 5},
		KeepAlive: KeepAliveConfig{
			TriggerBeforeExpiryMinutes: 7,
			CountdownSeconds:           45,
			Message:                    "still there?",
			Scope: ScopeConfig{
				MaxSends: 3,
			},
		},
	})

	result, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if result.KeepAlive.Message != "still there?" {
		t.Fatalf("Message = %q, want custom config", result.KeepAlive.Message)
	}
	if result.KeepAlive.Scope.MaxSends != 3 {
		t.Fatalf("MaxSends = %d, want 3", result.KeepAlive.Scope.MaxSends)
	}
	if result.RecentSessions != 10 {
		t.Fatalf("RecentSessions = %d, want default 10", result.RecentSessions)
	}
}

func TestLoadMergesPartialConfigWithDefaults(t *testing.T) {
	home := t.TempDir()
	path := ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"keep_alive":{"message":"custom"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if result.KeepAlive.Message != "custom" {
		t.Fatalf("Message = %q, want partial config override", result.KeepAlive.Message)
	}
	if result.KeepAlive.TriggerBeforeExpiryMinutes != Default().KeepAlive.TriggerBeforeExpiryMinutes {
		t.Fatalf("trigger = %d, want default", result.KeepAlive.TriggerBeforeExpiryMinutes)
	}
	if result.KeepAlive.Scope.MaxSends != Default().KeepAlive.Scope.MaxSends {
		t.Fatalf("max sends = %d, want default", result.KeepAlive.Scope.MaxSends)
	}
	if len(result.ReminderThresholds) != len(Default().ReminderThresholds) {
		t.Fatalf("reminder thresholds = %#v, want defaults", result.ReminderThresholds)
	}
	if !reflect.DeepEqual(result.Statusline, Default().Statusline) {
		t.Fatalf("statusline = %#v, want defaults", result.Statusline)
	}
}

func TestLoadInvalidJSONFallsBackToDefaults(t *testing.T) {
	home := t.TempDir()
	path := ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !reflect.DeepEqual(result, Default()) {
		t.Fatalf("Config = %#v, want defaults after invalid JSON", result)
	}
}

func TestSaveWritesOnlyValidConfig(t *testing.T) {
	home := t.TempDir()
	valid := Default()
	valid.KeepAlive.Scope.MaxSends = 2

	if err := Save(home, valid); err != nil {
		t.Fatalf("Save valid config returned error: %v", err)
	}
	loaded, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.KeepAlive.Scope.MaxSends != 2 {
		t.Fatalf("saved MaxSends = %d, want 2", loaded.KeepAlive.Scope.MaxSends)
	}

	invalid := valid
	invalid.KeepAlive.Scope.MaxSends = 0
	if err := Save(home, invalid); err == nil {
		t.Fatal("Save invalid config returned nil, want validation error")
	}
	after, err := Load(home)
	if err != nil {
		t.Fatalf("Load after invalid save returned error: %v", err)
	}
	if after.KeepAlive.Scope.MaxSends != 2 {
		t.Fatalf("invalid save changed MaxSends to %d, want existing value 2", after.KeepAlive.Scope.MaxSends)
	}
}

func TestValidateRejectsUnsafeValuesAndSummarizesAffectedAutosend(t *testing.T) {
	cfg := Default()
	cfg.ReminderThresholds = []int{10, 20}
	cfg.RecentSessions = 0
	cfg.KeepAlive.TriggerBeforeExpiryMinutes = 0
	cfg.KeepAlive.CountdownSeconds = 0
	cfg.KeepAlive.Scope.MaxSends = 0

	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate returned nil, want validation error")
	}
	message := err.Error()
	for _, want := range []string{"descending", "trigger", "countdown", "max_sends"} {
		if !strings.Contains(message, want) {
			t.Fatalf("validation error %q missing %q", message, want)
		}
	}

	cfg = Default()
	summary := EffectiveKeepAliveSummary(cfg)
	if summary.EffectiveTriggerSeconds1Hour != 300 {
		t.Fatalf("1h effective trigger = %d, want configured 300s", summary.EffectiveTriggerSeconds1Hour)
	}
	if summary.EffectiveTriggerSeconds5Minute != 60 {
		t.Fatalf("5m effective trigger = %d, want 20%% TTL trigger 60s", summary.EffectiveTriggerSeconds5Minute)
	}

	cfg.KeepAlive.CountdownSeconds = 280
	summary = EffectiveKeepAliveSummary(cfg)
	if !summary.SendPausedFor5Minute {
		t.Fatalf("SendPausedFor5Minute = false, want true: %#v", summary)
	}
	if summary.Warning == "" {
		t.Fatalf("Warning is empty, want visible warning summary")
	}
}

func writeConfigFile(t *testing.T, path string, cfg Config) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
