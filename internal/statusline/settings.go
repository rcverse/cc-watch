package statusline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const wrapMarker = " statusline -- "

type State string

const (
	StateNotInstalled State = "not_installed"
	StateExisting     State = "existing"
	StateInstalled    State = "installed"
	StateManualReview State = "manual_review"
)

type Status struct {
	State           State
	Command         string
	PreviousCommand string
}

func Inspect(home string) (Status, error) {
	settings, _, err := readSettings(home)
	if err != nil {
		return Status{State: StateManualReview}, err
	}
	current := statuslineCommand(settings)
	return inspectCommand(current), nil
}

func Install(home, binaryPath string) error {
	settings, exists, err := readSettings(home)
	if err != nil {
		return err
	}
	status := inspectCommand(statuslineCommand(settings))
	switch status.State {
	case StateInstalled:
		if UsesRuntimeCommand(status.Command, binaryPath) {
			return nil
		}
		desired := RuntimeCommand(binaryPath)
		command := desired
		if status.PreviousCommand != "" {
			command = desired + " -- " + status.PreviousCommand
			if NeedsShell(status.PreviousCommand) {
				command = desired + " -- sh -c " + ShellQuote(status.PreviousCommand)
			}
		}
		setStatuslineCommand(settings, command)
		return writeSettings(home, settings, exists)
	case StateManualReview:
		return errors.New("statusline settings need manual review")
	}

	commandPrefix := RuntimeCommand(binaryPath)
	command := commandPrefix
	if status.State == StateExisting {
		command = commandPrefix + " -- " + status.Command
		if NeedsShell(status.Command) {
			command = commandPrefix + " -- sh -c " + ShellQuote(status.Command)
		}
	}
	setStatuslineCommand(settings, command)
	return writeSettings(home, settings, exists)
}

func RuntimeCommand(binaryPath string) string {
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		binaryPath = "cc-watch"
	}
	return binaryPath + " statusline"
}

func UsesRuntimeCommand(command, binaryPath string) bool {
	current := strings.TrimSpace(command)
	prefix := RuntimeCommand(binaryPath)
	return current == prefix || strings.HasPrefix(current, prefix+" -- ")
}

func Uninstall(home string) error {
	settings, exists, err := readSettings(home)
	if err != nil {
		return err
	}
	status := inspectCommand(statuslineCommand(settings))
	switch status.State {
	case StateNotInstalled, StateExisting:
		return nil
	case StateManualReview:
		return errors.New("statusline settings need manual review")
	}

	if status.PreviousCommand == "" {
		delete(settings, "statusLine")
	} else {
		setStatuslineCommand(settings, status.PreviousCommand)
	}
	return writeSettings(home, settings, exists)
}

func SettingsSnippet(command string) string {
	escaped, _ := json.Marshal(command)
	return fmt.Sprintf(`"statusLine": {"type": "command", "command": %s}`, escaped)
}

func NeedsShell(command string) bool {
	return strings.ContainsAny(command, "|&;<>()$`\\\n=")
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func inspectCommand(command string) Status {
	current := strings.TrimSpace(command)
	if current == "" {
		return Status{State: StateNotInstalled}
	}
	if idx := strings.Index(current, wrapMarker); idx >= 0 && isCcWatchBinary(current[:idx]) {
		previous := strings.TrimSpace(current[idx+len(wrapMarker):])
		return Status{State: StateInstalled, Command: current, PreviousCommand: previous}
	}
	if isBareCcWatchStatusline(current) {
		return Status{State: StateInstalled, Command: current}
	}
	if strings.Contains(current, "cc-watch statusline") {
		return Status{State: StateManualReview, Command: current}
	}
	return Status{State: StateExisting, Command: current}
}

func isBareCcWatchStatusline(command string) bool {
	parts := strings.Fields(command)
	return len(parts) == 2 && isCcWatchBinary(parts[0]) && parts[1] == "statusline"
}

func isCcWatchBinary(command string) bool {
	return filepath.Base(strings.TrimSpace(command)) == "cc-watch"
}

func readSettings(home string) (map[string]any, bool, error) {
	path := settingsPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, false, nil
		}
		return nil, false, err
	}
	settings := map[string]any{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, true, err
	}
	return settings, true, nil
}

func writeSettings(home string, settings map[string]any, backup bool) error {
	path := settingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if backup {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		backupPath := fmt.Sprintf("%s.cc-watch-%s.bak", path, time.Now().UTC().Format("20060102-150405"))
		if err := os.WriteFile(backupPath, data, 0o644); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func statuslineCommand(settings map[string]any) string {
	raw, ok := settings["statusLine"].(map[string]any)
	if !ok {
		return ""
	}
	command, _ := raw["command"].(string)
	return command
}

func setStatuslineCommand(settings map[string]any, command string) {
	statusLine, ok := settings["statusLine"].(map[string]any)
	if !ok {
		statusLine = map[string]any{}
	}
	statusLine["type"] = "command"
	statusLine["command"] = command
	settings["statusLine"] = statusLine
}

func settingsPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}
