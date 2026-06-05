package notify

func LinuxCommand(notification Notification) (string, []string) {
	return "notify-send", []string{notification.Title, notification.Body}
}
