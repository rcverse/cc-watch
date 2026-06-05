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
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
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
	StartWatcher                 func() error
	StartNotifier                func() error
	NewKeepAliveRunner           func() error
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
		fmt.Fprintln(stderr, "config editor is not wired until Phase 10")
		return 1
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

	discoveryLimit := cmd.Limit
	if cmd.ID != "" {
		discoveryLimit = 0
	}
	discovery, err := deps.DiscoverHome(home, discoveryLimit)
	if err != nil {
		return tui.Options{}, err
	}

	refreshState := tui.RefreshViewState{
		ProjectsDir: discovery.ProjectsDir,
	}
	switch discovery.ErrorCode {
	case "projects_dir_missing":
		refreshState.EmptyState = tui.EmptyProjectsDir
	}

	var sessions []session.Session
	var selectedID string
	var ambiguousID string
	if cmd.ID != "" {
		selectedFile, err := session.ResolvePartialID(discovery.Sessions, cmd.ID)
		if err != nil {
			var resolveErr *session.ResolveError
			if errors.As(err, &resolveErr) && resolveErr.Code == "ambiguous_session_id" {
				ambiguousID = cmd.ID
				sessions = sessionFilesToSessions(resolveErr.Candidates)
			} else {
				refreshState.EmptyState = tui.EmptyNoSessions
			}
		} else {
			selected, err := deps.ParseFile(selectedFile.Path)
			if err != nil {
				return tui.Options{}, err
			}
			selectedID = selected.SessionID
			sessions = []session.Session{selected}
		}
	} else {
		for _, file := range discovery.Sessions {
			parsed, err := deps.ParseFile(file.Path)
			if err != nil {
				return tui.Options{}, err
			}
			sessions = append(sessions, parsed)
		}
		if len(sessions) == 0 && refreshState.EmptyState == tui.EmptyNone {
			refreshState.EmptyState = tui.EmptyNoSessions
		}
	}

	reminders := map[string]bool{}
	if cmd.Remind {
		for _, s := range sessions {
			reminders[s.SessionID] = true
		}
	}

	options := tui.Options{
		Now:             now,
		Dependencies:    tuiDependencies(cmd, deps),
		Sessions:        sessions,
		SelectedID:      selectedID,
		AmbiguousID:     ambiguousID,
		ReminderEnabled: reminders,
		Refresh:         refreshState,
	}
	return options, nil
}

func tuiDependencies(cmd Command, deps Dependencies) tui.Dependencies {
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
	return tui.Dependencies{
		RefreshSnapshot: func(_ refresh.Source, _ int) tui.RefreshSnapshot {
			options, err := buildTUIOptions(Command{Mode: cmd.Mode, Limit: cmd.Limit, ID: cmd.ID, Remind: cmd.Remind}, deps)
			if err != nil {
				return tui.RefreshSnapshot{}
			}
			return tui.RefreshSnapshot{
				Sessions:   options.Sessions,
				Refresh:    options.Refresh,
				HasRefresh: true,
			}
		},
		NotifyEvent:                  notifyEvent,
		ResetNotificationSuppression: resetNotificationSuppression,
	}
}

func runJSON(cmd Command, stdout io.Writer, _ io.Writer, deps Dependencies) int {
	deps = fillDependencies(deps)
	now := deps.Now()
	home, err := deps.HomeDir()
	if err != nil {
		return writeJSONError(stdout, now, cmd, nil, "config_error", err.Error(), cmd.ID)
	}
	cfgResult, err := config.Load(home)
	if err != nil {
		return writeJSONError(stdout, now, cmd, nil, "config_error", err.Error(), cmd.ID)
	}
	discoveryLimit := cmd.Limit
	if cmd.ID != "" {
		discoveryLimit = 0
	}
	discovery, err := deps.DiscoverHome(home, discoveryLimit)
	if err != nil {
		return writeJSONError(stdout, now, cmd, nil, "parse_error", err.Error(), cmd.ID)
	}
	configWarnings := configWarningMessages(cfgResult.Warnings)

	if cmd.ID != "" {
		selectedFile, err := session.ResolvePartialID(discovery.Sessions, cmd.ID)
		if err != nil {
			var resolveErr *session.ResolveError
			if errors.As(err, &resolveErr) {
				return writeJSONError(stdout, now, cmd, sessionFilesToSessions(resolveErr.Candidates), resolveErr.Code, resolveErr.Error(), resolveErr.Query)
			}
			return writeJSONError(stdout, now, cmd, nil, "session_not_found", err.Error(), cmd.ID)
		}
		selected, err := deps.ParseFile(selectedFile.Path)
		if err != nil {
			return writeJSONError(stdout, now, cmd, nil, "parse_error", err.Error(), cmd.ID)
		}
		return writeJSON(stdout, jsonout.State{
			GeneratedAt:    now,
			Query:          jsonout.Query{ID: cmd.ID, Limit: cmd.Limit},
			ConfigWarnings: configWarnings,
			Sessions:       []session.Session{selected},
			Selected:       &selected,
			Reminder:       reminderStates([]session.Session{selected}, cmd.Remind, cfgResult.Config.ReminderThresholds),
		}, 0)
	}

	sessions := make([]session.Session, 0, len(discovery.Sessions))
	for _, file := range discovery.Sessions {
		parsed, err := deps.ParseFile(file.Path)
		if err != nil {
			return writeJSONError(stdout, now, cmd, nil, "parse_error", err.Error(), file.SessionID)
		}
		sessions = append(sessions, parsed)
	}
	return writeJSON(stdout, jsonout.State{
		GeneratedAt:    now,
		Query:          jsonout.Query{Limit: cmd.Limit},
		ConfigWarnings: configWarnings,
		Sessions:       sessions,
		Reminder:       reminderStates(sessions, cmd.Remind, cfgResult.Config.ReminderThresholds),
	}, 0)
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

func writeJSON(stdout io.Writer, state jsonout.State, exitCode int) int {
	data, err := jsonout.Marshal(state)
	if err != nil {
		fmt.Fprintln(stdout, `{"schema_version":1,"error":{"code":"config_error","message":"failed to encode json","query":""}}`)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return exitCode
}

func sessionFilesToSessions(files []session.SessionFile) []session.Session {
	sessions := make([]session.Session, 0, len(files))
	for _, file := range files {
		sessions = append(sessions, session.Session{
			SessionID:      file.SessionID,
			ShortID:        file.ShortID,
			Project:        file.Project,
			JSONLPath:      file.Path,
			FileModifiedAt: file.ModTime,
		})
	}
	return sessions
}

func reminderStates(sessions []session.Session, enabled bool, thresholds []int) map[string]jsonout.ReminderState {
	if !enabled {
		return nil
	}
	states := make(map[string]jsonout.ReminderState, len(sessions))
	for _, s := range sessions {
		sessionEnabled := true
		states[s.SessionID] = jsonout.ReminderState{
			Available:  true,
			Enabled:    &sessionEnabled,
			Thresholds: append([]int(nil), thresholds...),
		}
	}
	return states
}
