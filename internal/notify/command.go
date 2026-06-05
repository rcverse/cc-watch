package notify

import (
	"errors"
	"os/exec"
)

type CommandBuilder func(Notification) (string, []string)

type CommandNotifier struct {
	build  CommandBuilder
	runner Runner
}

func NewCommandNotifier(build CommandBuilder, runner Runner) CommandNotifier {
	return CommandNotifier{build: build, runner: runner}
}

func ExecRunner(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func (n CommandNotifier) Notify(event Event) Result {
	notification := FormatEvent(event)
	name, args := n.build(notification)
	if n.runner == nil {
		err := errors.New("notification runner unavailable")
		return Result{Degraded: true, Message: err.Error(), Err: err}
	}
	if err := n.runner(name, args...); err != nil {
		return Result{Degraded: true, Message: err.Error(), Err: err}
	}
	return Result{Delivered: true, Message: "delivered"}
}
