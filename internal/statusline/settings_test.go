package statusline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectMissingSettingsIsNotInstalled(t *testing.T) {
	status, err := Inspect(t.TempDir())
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if status.State != StateNotInstalled {
		t.Fatalf("state = %q, want %q", status.State, StateNotInstalled)
	}
}

func TestInstallMissingSettingsAddsBareCommand(t *testing.T) {
	home := t.TempDir()

	if err := Install(home, "cc-watch"); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	if !strings.Contains(settings, `"command": "cc-watch statusline"`) {
		t.Fatalf("settings = %s, want bare cc-watch statusline command", settings)
	}
}

func TestInstallUsesProvidedBinaryPath(t *testing.T) {
	home := t.TempDir()

	if err := Install(home, "/Users/example/.local/bin/cc-watch"); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	if !strings.Contains(settings, `"command": "/Users/example/.local/bin/cc-watch statusline"`) {
		t.Fatalf("settings = %s, want command to use the provided executable path", settings)
	}
}

func TestInstallRepairsAnExistingWrapperWithAStaleBinaryPath(t *testing.T) {
	home := t.TempDir()
	writeSettingsFile(t, home, `{"statusLine":{"type":"command","command":"cc-watch statusline -- ccstatusline"}}`)

	if err := Install(home, "/Users/example/.local/bin/cc-watch"); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	if !strings.Contains(settings, `"command": "/Users/example/.local/bin/cc-watch statusline -- ccstatusline"`) {
		t.Fatalf("settings = %s, want existing wrapper repaired with the provided path", settings)
	}
}

func TestUsesRuntimeCommandRecognizesBareAndWrappedCommands(t *testing.T) {
	binary := "/Users/example/.local/bin/cc-watch"
	for _, command := range []string{
		"/Users/example/.local/bin/cc-watch statusline",
		"/Users/example/.local/bin/cc-watch statusline -- ccstatusline",
	} {
		if !UsesRuntimeCommand(command, binary) {
			t.Fatalf("UsesRuntimeCommand(%q, %q) = false, want true", command, binary)
		}
	}
	if UsesRuntimeCommand("cc-watch statusline", binary) {
		t.Fatal("UsesRuntimeCommand accepted a path-dependent command, want repair")
	}
}

func TestInstallExistingCommandPreservesItBehindCcWatch(t *testing.T) {
	home := t.TempDir()
	writeSettingsFile(t, home, `{"theme":"dark","statusLine":{"type":"command","command":"~/.claude/statusline.sh"}}`)

	if err := Install(home, "cc-watch"); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	if !strings.Contains(settings, `"theme": "dark"`) {
		t.Fatalf("settings = %s, want unrelated settings preserved", settings)
	}
	if !strings.Contains(settings, `"command": "cc-watch statusline -- ~/.claude/statusline.sh"`) {
		t.Fatalf("settings = %s, want existing command preserved after --", settings)
	}
	if backups := backupFiles(t, home); len(backups) != 1 {
		t.Fatalf("backups = %#v, want one backup before write", backups)
	}
}

func TestInstallExistingCommandPreservesStatuslineOptions(t *testing.T) {
	home := t.TempDir()
	writeSettingsFile(t, home, `{"statusLine":{"type":"command","command":"~/.claude/statusline.sh","padding":2,"refreshInterval":5,"hideVimModeIndicator":true}}`)

	if err := Install(home, "cc-watch"); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	for _, want := range []string{`"padding": 2`, `"refreshInterval": 5`, `"hideVimModeIndicator": true`} {
		if !strings.Contains(settings, want) {
			t.Fatalf("settings = %s, want existing statusline option %s preserved", settings, want)
		}
	}
}

func TestInstallExistingShellCommandKeepsShellSemantics(t *testing.T) {
	home := t.TempDir()
	writeSettingsFile(t, home, `{"statusLine":{"type":"command","command":"echo 'hi' | sed s/i/o/"}}`)

	if err := Install(home, "cc-watch"); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	settings := readSettingsFile(t, home)
	want := `"command": "cc-watch statusline -- sh -c 'echo '\\''hi'\\'' | sed s/i/o/'"`
	if !strings.Contains(settings, want) {
		t.Fatalf("settings = %s, want shell-preserving command %s", settings, want)
	}
}

func TestUninstallBareCommandRemovesStatusLine(t *testing.T) {
	home := t.TempDir()
	writeSettingsFile(t, home, `{"theme":"dark","statusLine":{"type":"command","command":"cc-watch statusline"}}`)

	if err := Uninstall(home); err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	if strings.Contains(settings, "statusLine") || !strings.Contains(settings, `"theme": "dark"`) {
		t.Fatalf("settings = %s, want statusLine removed and other settings preserved", settings)
	}
}

func TestUninstallInstalledCommandRestoresPreviousCommand(t *testing.T) {
	home := t.TempDir()
	writeSettingsFile(t, home, `{"statusLine":{"type":"command","command":"cc-watch statusline -- ~/.claude/statusline.sh"}}`)

	if err := Uninstall(home); err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	if !strings.Contains(settings, `"command": "~/.claude/statusline.sh"`) {
		t.Fatalf("settings = %s, want previous command restored", settings)
	}
}

func TestUninstallInstalledCommandPreservesStatuslineOptions(t *testing.T) {
	home := t.TempDir()
	writeSettingsFile(t, home, `{"statusLine":{"type":"command","command":"cc-watch statusline -- ~/.claude/statusline.sh","padding":2,"refreshInterval":5}}`)

	if err := Uninstall(home); err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	settings := readSettingsFile(t, home)
	for _, want := range []string{`"command": "~/.claude/statusline.sh"`, `"padding": 2`, `"refreshInterval": 5`} {
		if !strings.Contains(settings, want) {
			t.Fatalf("settings = %s, want %s preserved", settings, want)
		}
	}
}

func TestUninstallAmbiguousCommandRefusesToWrite(t *testing.T) {
	home := t.TempDir()
	original := `{"statusLine":{"type":"command","command":"sh -c 'cc-watch statusline -- old'"}}`
	writeSettingsFile(t, home, original)

	err := Uninstall(home)
	if err == nil {
		t.Fatal("Uninstall returned nil error, want refusal for ambiguous command")
	}
	if got := readSettingsFile(t, home); got != original {
		t.Fatalf("settings changed: got %s want %s", got, original)
	}
}

func writeSettingsFile(t *testing.T, home, contents string) {
	t.Helper()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func readSettingsFile(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

func backupFiles(t *testing.T, home string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(home, ".claude", "settings.json.cc-watch-*.bak"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	return matches
}
