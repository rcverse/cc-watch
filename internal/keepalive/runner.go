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

// SendTimeout bounds the real `claude -r ... -p ...` subprocess so a hang
// (e.g. an unanswerable non-interactive permission prompt) can't leave a
// send stuck in StateSending forever.
const SendTimeout = 30 * time.Second

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

type RunnerExecution struct {
	Result            RunResult
	ClaudeUnavailable bool
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
	execution := ExecuteRunner(ctx, action, runner, now)
	m.ApplyRunnerExecution(action, execution)
	return execution.Result
}

func ExecuteRunner(ctx context.Context, action Action, runner ClaudeRunner, now time.Time) RunnerExecution {
	if runner == nil {
		err := ErrClaudeUnavailable
		return RunnerExecution{
			Result:            RunResult{StartedAt: now, Err: err},
			ClaudeUnavailable: true,
		}
	}
	if err := runner.Available(); err != nil {
		return RunnerExecution{
			Result:            RunResult{StartedAt: now, Err: err},
			ClaudeUnavailable: true,
		}
	}
	result := runner.Send(ctx, RunRequest{SessionID: action.SessionID, Message: action.Message})
	if result.StartedAt.IsZero() {
		result.StartedAt = now
	}
	if result.Limit && result.Err == nil {
		result.Err = ErrClaudeLimit
	}
	return RunnerExecution{Result: result}
}

func (m *Manager) ApplyRunnerExecution(action Action, execution RunnerExecution) SessionState {
	state := m.State(action.SessionID)
	if action.Kind != ActionStartRunner || state.State != StateSending || state.InstanceToken != action.InstanceToken {
		return state
	}
	if execution.ClaudeUnavailable {
		reason := ErrClaudeUnavailable.Error()
		if execution.Result.Err != nil {
			reason = execution.Result.Err.Error()
		}
		m.MarkNoClaude(action.SessionID, action.InstanceToken, reason)
		return m.State(action.SessionID)
	}
	result := execution.Result
	if result.Err != nil || result.ExitCode != 0 || result.Limit {
		m.MarkSubprocessFailure(action.SessionID, action.InstanceToken, failureMessage(result), result.Limit)
		return m.State(action.SessionID)
	}
	m.MarkSendStarted(action.SessionID, action.InstanceToken, result.StartedAt)
	return m.State(action.SessionID)
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
		// A context-deadline kill leaves no exit code or output of its own
		// to explain the failure (Send() below normalizes err to the
		// generic ErrSubprocess), so record a distinguishable reason here.
		if ctx.Err() == context.DeadlineExceeded && stderr.Len() == 0 {
			stderr.WriteString("claude did not respond before the send timeout and was terminated")
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
	for _, marker := range []string{"limit", "usage", "quota", "too many requests"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
