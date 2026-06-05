package keepalive

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrClaudeUnavailable = errors.New("claude command unavailable")
	ErrClaudeLimit       = errors.New("claude limit reached")
	ErrSubprocess        = errors.New("claude subprocess failed")
)

type RunRequest struct {
	SessionID string
	Message   string
}

type RunResult struct {
	StartedAt time.Time
	ExitCode  int
	Stdout    string
	Stderr    string
	Limit     bool
	Err       error
}

type ClaudeRunner interface {
	Available() error
	Send(context.Context, RunRequest) RunResult
}

type SubprocessRunner struct {
	LookPath func(string) (string, error)
	Command  func(context.Context, string, ...string) (string, string, int, error)
}

func NewSubprocessRunner() SubprocessRunner {
	return SubprocessRunner{
		LookPath: exec.LookPath,
		Command:  runCommand,
	}
}

func (r SubprocessRunner) Available() error {
	lookPath := r.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath("claude"); err != nil {
		return ErrClaudeUnavailable
	}
	return nil
}

func (r SubprocessRunner) Send(ctx context.Context, req RunRequest) RunResult {
	startedAt := time.Now()
	if err := r.Available(); err != nil {
		return RunResult{StartedAt: startedAt, Err: err}
	}
	command := r.Command
	if command == nil {
		command = runCommand
	}
	stdout, stderr, exitCode, err := command(ctx, "claude", "-r", req.SessionID, "-p", req.Message)
	result := RunResult{
		StartedAt: startedAt,
		ExitCode:  exitCode,
		Stdout:    stdout,
		Stderr:    stderr,
		Limit:     isClaudeLimit(stdout) || isClaudeLimit(stderr),
		Err:       err,
	}
	if result.Limit {
		result.Err = ErrClaudeLimit
	} else if err != nil || exitCode != 0 {
		result.Err = ErrSubprocess
	}
	return result
}

func (m *Manager) CheckAvailability(sessionID string, runner ClaudeRunner) error {
	state := m.State(sessionID)
	if state.State == StateOff || state.State == StateScopeComplete {
		return nil
	}
	if runner == nil {
		return nil
	}
	if err := runner.Available(); err != nil {
		state.State = StateErrorNoClaude
		state.AutoSend = false
		state.LastFailure = err.Error()
		state.InstanceToken = m.nextToken()
		m.states[sessionID] = state
		return err
	}
	return nil
}

func (m *Manager) Run(ctx context.Context, action Action, runner ClaudeRunner, now time.Time) RunResult {
	state := m.State(action.SessionID)
	if action.Kind != ActionStartRunner || state.State != StateSending || state.InstanceToken != action.InstanceToken {
		return RunResult{StartedAt: now}
	}
	if runner == nil {
		err := ErrClaudeUnavailable
		m.MarkNoClaude(action.SessionID, action.InstanceToken, err.Error())
		return RunResult{StartedAt: now, Err: err}
	}
	if err := runner.Available(); err != nil {
		m.MarkNoClaude(action.SessionID, action.InstanceToken, err.Error())
		return RunResult{StartedAt: now, Err: err}
	}
	result := runner.Send(ctx, RunRequest{SessionID: action.SessionID, Message: action.Message})
	if result.StartedAt.IsZero() {
		result.StartedAt = now
	}
	if result.Limit && result.Err == nil {
		result.Err = ErrClaudeLimit
	}
	if result.Err != nil || result.ExitCode != 0 || result.Limit {
		m.MarkSubprocessFailure(action.SessionID, action.InstanceToken, failureMessage(result))
		return result
	}
	m.MarkSendStarted(action.SessionID, action.InstanceToken, result.StartedAt)
	return result
}

func runCommand(ctx context.Context, name string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	return stdout.String(), stderr.String(), exitCode, err
}

func failureMessage(result RunResult) string {
	if result.Stderr != "" {
		return result.Stderr
	}
	if result.Stdout != "" {
		return result.Stdout
	}
	if result.Err != nil {
		return result.Err.Error()
	}
	return ErrSubprocess.Error()
}

func isClaudeLimit(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "limit") || strings.Contains(lower, "usage")
}
