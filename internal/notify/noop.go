package notify

import (
	"errors"
	"fmt"
)

type UnsupportedNotifier struct {
	GOOS string
}

func (n UnsupportedNotifier) Notify(Event) Result {
	message := fmt.Sprintf("notifications unsupported on %s", n.GOOS)
	return Result{
		Degraded: true,
		Message:  message,
		Err:      errors.New(message),
	}
}
