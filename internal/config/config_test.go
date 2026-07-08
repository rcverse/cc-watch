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

	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("Default() = %#v, want %#v", cfg, want)
	}
}

func TestLoadReadsConfigFromHome(t *testing.T) {
	home := t.TempDir()
	path := ConfigPath(home)
	writeConfigFile(t, path, Config{
		ReminderThresholds: []int{30, 15, 5},
		KeepAlive: KeepAliveConfig{
			TriggerBeforeExpiryMinutes: 7,
			CountdownSeconds:           45,
			Message:                    "still there?",
			AutoSend:                   false,
			Scope: ScopeConfig{
				Mode:     "max_sends",
				MaxSends: 3,
			},
		},
	})

	result, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", result.Warnings)
	}
	if result.Config.KeepAlive.Message != "still there?" {
		t.Fatalf("Message = %q, want custom config", result.Config.KeepAlive.Message)
	}
	if result.Config.KeepAlive.Scope.MaxSends != 3 {
		t.Fatalf("MaxSends = %d, want 3", result.Config.KeepAlive.Scope.MaxSends)
	}
}

func TestLoadMergesPartialConfigWithDefaults(t *testing.T) {
	home := t.TempDir()
	path := ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"keep_alive":{"auto_send":false}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Load(home)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if result.Config.KeepAlive.AutoSend {
		t.Fatalf("AutoSend = true, want partial config override false")
	}
	if result.Config.KeepAlive.TriggerBeforeExpiryMinutes != Default().KeepAlive.TriggerBeforeExpiryMinutes {
		t.Fatalf("trigger = %d, want default", result.Config.KeepAlive.TriggerBeforeExpiryMinutes)
	}
	if result.Config.KeepAlive.Scope.MaxSends != Default().KeepAlive.Scope.MaxSends {
		t.Fatalf("max sends = %d, want default", result.Config.KeepAlive.Scope.MaxSends)
	}
	if len(result.Config.ReminderThresholds) != len(Default().ReminderThresholds) {
		t.Fatalf("reminder thresholds = %#v, want defaults", result.Config.ReminderThresholds)
	}
}

func TestLoadInvalidJSONFallsBackWithVisibleWarning(t *testing.T) {
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
	if !reflect.DeepEqual(result.Config, Default()) {
		t.Fatalf("Config = %#v, want defaults after invalid JSON", result.Config)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one visible warning", result.Warnings)
	}
	if result.Warnings[0].Code != WarningInvalidJSON {
		t.Fatalf("Warning code = %q, want %q", result.Warnings[0].Code, WarningInvalidJSON)
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
	if loaded.Config.KeepAlive.Scope.MaxSends != 2 {
		t.Fatalf("saved MaxSends = %d, want 2", loaded.Config.KeepAlive.Scope.MaxSends)
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
	if after.Config.KeepAlive.Scope.MaxSends != 2 {
		t.Fatalf("invalid save changed MaxSends to %d, want existing value 2", after.Config.KeepAlive.Scope.MaxSends)
	}
}

func TestResetRestoresDefaults(t *testing.T) {
	home := t.TempDir()
	custom := Default()
	custom.ReminderThresholds = []int{40, 20}
	if err := Save(home, custom); err != nil {
		t.Fatalf("Save custom config: %v", err)
	}

	if err := Reset(home); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	loaded, err := Load(home)
	if err != nil {
		t.Fatalf("Load after reset returned error: %v", err)
	}
	if !reflect.DeepEqual(loaded.Config, Default()) {
		t.Fatalf("Config = %#v, want defaults after reset", loaded.Config)
	}
}

func TestValidateRejectsUnsafeValuesAndSummarizesAffectedAutosend(t *testing.T) {
	cfg := Default()
	cfg.ReminderThresholds = []int{10, 20}
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
	if !summary.AutoSendDisabledFor5Minute {
		t.Fatalf("AutoSendDisabledFor5Minute = false, want true: %#v", summary)
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
