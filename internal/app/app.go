package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-cache/internal/config"
	"github.com/richardchen/cc-cache/internal/jsonout"
	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
	"github.com/richardchen/cc-cache/internal/snapshot"
	"github.com/richardchen/cc-cache/internal/tui"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithDeps(args, stdout, stderr, DefaultDependencies())
}

type Dependencies struct {
	HomeDir                      func() (string, error)
	Now                          func() time.Time
	DiscoverHome                 func(home string, limit int) (session.DiscoveryResult, error)
	ParseFile                    func(path string) (session.Session, error)
	StartTUI                     func(Command) error
	NotifyEvent                  func(notify.Event) notify.Result
	ResetNotificationSuppression func()
}

func DefaultDependencies() Dependencies {
	return Dependencies{
		HomeDir:      os.UserHomeDir,
		Now:          func() time.Time { return time.Now().UTC() },
		DiscoverHome: session.DiscoverHome,
		ParseFile:    session.ParseFile,
	}
}

// RunWithDeps dispatches CLI modes. Phase 3 wires JSON as a non-interactive snapshot path.
func RunWithDeps(args []string, stdout io.Writer, stderr io.Writer, deps Dependencies) int {
	cmd, err := ParseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	switch cmd.Mode {
	case ModeHelp:
		WriteHelp(stdout)
		return 0
	case ModeVersion:
		fmt.Fprintf(stdout, "cc-cache %s\n", Version)
		return 0
	case ModeJSON:
		return runJSON(cmd, stdout, stderr, deps)
	case ModeConfig:
		deps = fillDependencies(deps)
		var err error
		if deps.StartTUI != nil {
			err = deps.StartTUI(cmd)
		} else {
			err = runTUI(cmd, deps)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case ModeTUI:
		deps = fillDependencies(deps)
		var err error
		if deps.StartTUI != nil {
			err = deps.StartTUI(cmd)
		} else {
			err = runTUI(cmd, deps)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown mode %q\n", cmd.Mode)
		return 2
	}
}

func runTUI(cmd Command, deps Dependencies) error {
	options, err := buildTUIOptions(cmd, deps)
	if err != nil {
		return err
	}
	model := tui.NewModel(options)
	_, err = tea.NewProgram(model).Run()
	return err
}

func buildTUIOptions(cmd Command, deps Dependencies) (tui.Options, error) {
	now := deps.Now()
	home, err := deps.HomeDir()
	if err != nil {
		return tui.Options{}, err
	}
	if cmd.Mode == ModeConfig {
		result, err := snapshot.ConfigOnly(snapshot.Request{Home: home, Now: now, Limit: cmd.Limit}, snapshot.Loaders{
			LoadConfig: config.Load,
		})
		if err != nil {
			return tui.Options{}, err
		}
		return buildTUIOptionsFromSnapshot(cmd, deps, home, result, tui.StartConfig), nil
	}

	result, err := snapshot.Build(snapshot.Request{
		Home:   home,
		Now:    now,
		Limit:  cmd.Limit,
		ID:     cmd.ID,
		Remind: cmd.Remind,
	}, snapshot.Loaders{
		LoadConfig:   config.Load,
		DiscoverHome: deps.DiscoverHome,
		ParseFile:    deps.ParseFile,
	})
	if err != nil {
		return tui.Options{}, err
	}
	return buildTUIOptionsFromSnapshot(cmd, deps, home, result, tui.StartList), nil
}

func buildTUIOptionsFromSnapshot(cmd Command, deps Dependencies, home string, result snapshot.Result, startMode tui.StartMode) tui.Options {
	refreshState := tui.RefreshViewState{
		ProjectsDir: result.ProjectsDir,
	}

	var selectedID string
	var ambiguousID string
	sessions := result.Sessions
	if result.Selected != nil {
		sessions = []session.Session{*result.Selected}
		selectedID = result.Selected.SessionID
	}
	if result.Error != nil {
		switch result.Error.Code {
		case "ambiguous_session_id":
			ambiguousID = result.Error.Query
			sessions = result.Candidates
		case "session_not_found":
			sessions = nil
			refreshState.EmptyState = tui.EmptyNoSessions
		}
	}
	if refreshState.EmptyState == tui.EmptyNone {
		refreshState.EmptyState = tuiEmptyState(result.EmptyState)
	}

	return tui.Options{
		Now:                result.GeneratedAt,
		Dependencies:       tuiDependencies(cmd, deps, home),
		Sessions:           sessions,
		SelectedID:         selectedID,
		AmbiguousID:        ambiguousID,
		ReminderEnabled:    tuiReminderEnabled(result.Reminder),
		ReminderThresholds: result.Config.ReminderThresholds,
		KeepAliveConfig:    result.Config.KeepAlive,
		Refresh:            refreshState,
		StartMode:          startMode,
		StartDisplayTicker: true,
		StartRefreshTicker: startMode != tui.StartConfig,
		Config:             result.Config,
	}
}

func tuiEmptyState(state snapshot.EmptyState) tui.EmptyState {
	switch state {
	case snapshot.EmptyProjectsDir:
		return tui.EmptyProjectsDir
	case snapshot.EmptyNoSessions:
		return tui.EmptyNoSessions
	default:
		return tui.EmptyNone
	}
}

func tuiReminderEnabled(states map[string]snapshot.ReminderState) map[string]bool {
	enabled := make(map[string]bool, len(states))
	for id, state := range states {
		if state.Enabled {
			enabled[id] = true
		}
	}
	return enabled
}

func tuiDependencies(cmd Command, deps Dependencies, home string) tui.Dependencies {
	notifyEvent := deps.NotifyEvent
	resetNotificationSuppression := deps.ResetNotificationSuppression
	if notifyEvent == nil || resetNotificationSuppression == nil {
		manager := notify.NewManager(notify.NewPlatformNotifier("", notify.ExecRunner))
		if notifyEvent == nil {
			notifyEvent = manager.Notify
		}
		if resetNotificationSuppression == nil {
			resetNotificationSuppression = manager.ResetSuppression
		}
	}
	runner := keepalive.NewSubprocessRunner()
	return tui.Dependencies{
		RefreshSnapshot: func(_ refresh.Source, _ int, selected *session.Session) tui.RefreshSnapshot {
			if selected != nil {
				parsed, err := deps.ParseFile(selected.JSONLPath)
				if err != nil {
					return tui.RefreshSnapshot{
						SelectedOnly: true,
						SelectedID:   selected.SessionID,
					}
				}
				return tui.RefreshSnapshot{
					Sessions:     []session.Session{parsed},
					Refresh:      tui.RefreshViewState{EmptyState: tui.EmptyNone},
					HasRefresh:   true,
					SelectedOnly: true,
					SelectedID:   selected.SessionID,
				}
			}
			result, err := snapshot.Build(snapshot.Request{
				Home:   home,
				Now:    deps.Now(),
				Limit:  cmd.Limit,
				ID:     cmd.ID,
				Remind: cmd.Remind,
			}, snapshot.Loaders{
				LoadConfig:   config.Load,
				DiscoverHome: deps.DiscoverHome,
				ParseFile:    deps.ParseFile,
			})
			if err != nil {
				return tui.RefreshSnapshot{}
			}
			return tuiRefreshSnapshotFromResult(result)
		},
		CheckClaudeAvailable: func() error {
			return runner.Available()
		},
		KeepAliveRunner: runner,
		SaveConfig: func(next config.Config) error {
			home, err := deps.HomeDir()
			if err != nil {
				return err
			}
			return config.Save(home, next)
		},
		NotifyEvent:                  notifyEvent,
		ResetNotificationSuppression: resetNotificationSuppression,
	}
}

func tuiRefreshSnapshotFromResult(result snapshot.Result) tui.RefreshSnapshot {
	sessions := result.Sessions
	refreshState := tui.RefreshViewState{
		ProjectsDir: result.ProjectsDir,
		EmptyState:  tuiEmptyState(result.EmptyState),
	}
	if result.Error != nil {
		switch result.Error.Code {
		case "ambiguous_session_id":
			sessions = result.Candidates
		case "session_not_found":
			sessions = nil
			refreshState.EmptyState = tui.EmptyNoSessions
		}
	}
	return tui.RefreshSnapshot{
		Sessions:   sessions,
		Refresh:    refreshState,
		HasRefresh: true,
	}
}

func runJSON(cmd Command, stdout io.Writer, _ io.Writer, deps Dependencies) int {
	deps = fillDependencies(deps)
	now := deps.Now()
	home, err := deps.HomeDir()
	if err != nil {
		return writeJSONError(stdout, now, cmd, nil, "config_error", err.Error(), cmd.ID)
	}
	result, err := snapshot.Build(snapshot.Request{
		Home:   home,
		Now:    now,
		Limit:  cmd.Limit,
		ID:     cmd.ID,
		Remind: cmd.Remind,
	}, snapshot.Loaders{
		LoadConfig:   config.Load,
		DiscoverHome: deps.DiscoverHome,
		ParseFile:    deps.ParseFile,
	})
	if err != nil {
		var buildErr *snapshot.BuildError
		if errors.As(err, &buildErr) {
			return writeJSONError(stdout, now, cmd, nil, buildErr.Code, buildErr.Error(), cmd.ID)
		}
		return writeJSONError(stdout, now, cmd, nil, "parse_error", err.Error(), cmd.ID)
	}
	state := jsonStateFromSnapshot(result)
	exitCode := 0
	if result.Error != nil {
		exitCode = 1
	}
	return writeJSON(stdout, state, exitCode)
}

func fillDependencies(deps Dependencies) Dependencies {
	defaults := DefaultDependencies()
	if deps.HomeDir == nil {
		deps.HomeDir = defaults.HomeDir
	}
	if deps.Now == nil {
		deps.Now = defaults.Now
	}
	if deps.DiscoverHome == nil {
		deps.DiscoverHome = defaults.DiscoverHome
	}
	if deps.ParseFile == nil {
		deps.ParseFile = defaults.ParseFile
	}
	return deps
}

func writeJSONError(stdout io.Writer, now time.Time, cmd Command, candidates []session.Session, code, message, query string) int {
	return writeJSON(stdout, jsonout.State{
		GeneratedAt: now,
		Query:       jsonout.Query{ID: cmd.ID, Limit: cmd.Limit},
		Sessions:    candidates,
		Error: &jsonout.Error{
			Code:    code,
			Message: message,
			Query:   query,
		},
	}, 1)
}

func configWarningMessages(warnings []config.Warning) []string {
	messages := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		messages = append(messages, fmt.Sprintf("%s: %s", warning.Code, warning.Message))
	}
	return messages
}

func jsonStateFromSnapshot(result snapshot.Result) jsonout.State {
	sessions := result.Sessions
	if result.Error != nil {
		sessions = result.Candidates
	}
	return jsonout.State{
		GeneratedAt:    result.GeneratedAt,
		Query:          jsonout.Query{ID: result.QueryID, Limit: result.QueryLimit},
		ConfigWarnings: configWarningMessages(result.ConfigWarnings),
		Sessions:       sessions,
		Selected:       result.Selected,
		Reminder:       jsonReminderStates(result.Reminder),
		KeepAlive:      jsonKeepAliveStates(result.KeepAlive),
		Error:          jsonErrorFromSnapshot(result.Error),
	}
}

func jsonErrorFromSnapshot(err *snapshot.Error) *jsonout.Error {
	if err == nil {
		return nil
	}
	return &jsonout.Error{
		Code:    err.Code,
		Message: err.Message,
		Query:   err.Query,
	}
}

func jsonReminderStates(states map[string]snapshot.ReminderState) map[string]jsonout.ReminderState {
	result := make(map[string]jsonout.ReminderState, len(states))
	for id, state := range states {
		enabled := state.Enabled
		result[id] = jsonout.ReminderState{
			Available:  true,
			Enabled:    &enabled,
			Thresholds: append([]int(nil), state.Thresholds...),
		}
	}
	return result
}

func jsonKeepAliveStates(states map[string]snapshot.KeepAliveState) map[string]jsonout.KeepAliveState {
	result := make(map[string]jsonout.KeepAliveState, len(states))
	for id, state := range states {
		enabled := state.Enabled
		autoSend := state.AutoSend
		result[id] = jsonout.KeepAliveState{
			Available: true,
			Enabled:   &enabled,
			AutoSend:  &autoSend,
			State:     state.State,
			Scope: &jsonout.KeepAliveScope{
				Mode:     state.Mode,
				MaxSends: state.MaxSends,
			},
		}
	}
	return result
}

func writeJSON(stdout io.Writer, state jsonout.State, exitCode int) int {
	data, err := jsonout.Marshal(state)
	if err != nil {
		fmt.Fprintln(stdout, `{"schema_version":1,"error":{"code":"config_error","message":"failed to encode json","query":""}}`)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return exitCode
}
