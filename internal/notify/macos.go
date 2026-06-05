package notify

import "strings"

func MacOSCommand(notification Notification) (string, []string) {
	script := `display notification "` + escapeAppleScript(notification.Body) + `" with title "` + escapeAppleScript(notification.Title) + `"`
	return "osascript", []string{"-e", script}
}

func escapeAppleScript(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
