package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardchen/cc-watch/internal/config"
	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/notify"
	"github.com/richardchen/cc-watch/internal/refresh"
	"github.com/richardchen/cc-watch/internal/session"
	"github.com/richardchen/cc-watch/internal/snapshot"
	"github.com/richardchen/cc-watch/internal/statusline"
	"github.com/richardchen/cc-watch/internal/tui"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithDeps(args, stdout, stderr, DefaultDependencies())
}

type Dependencies struct {
	HomeDir                      func() (string, error)
	Now                          func() time.Time
	DiscoverHome                 func(home string, limit int) (session.DiscoveryResult, error)
	ParseFile                    func(path string) (session.Session, error)
	NewLiveWatcher               func(projectsDir string) (tui.Watcher, func() error, error)
	RunTUIProgram                func(tui.Options) error
	NotifyEvent                  func(notify.Event) notify.Result
	ResetNotificationSuppression func()
	Stdin                        io.Reader
	RunStatuslineCommand         StatuslineRunner
}

func DefaultDependencies() Dependencies {
	return Dependencies{
		HomeDir:              os.UserHomeDir,
		Now:                  func() time.Time { return time.Now().UTC() },
		DiscoverHome:         session.DiscoverHome,
		ParseFile:            session.ParseFile,
		Stdin:                os.Stdin,
		RunStatuslineCommand: runStatuslineCommand,
	}
}

// RunWithDeps dispatches CLI modes.
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
		fmt.Fprintf(stdout, "cc-watch %s\n", Version)
		return 0
	case ModeStatuslineHelp:
		WriteStatuslineHelp(stdout)
		return 0
	case ModeStatusline:
		deps = fillDependencies(deps)
		return runStatusline(cmd, deps, deps.Stdin, stdout, stderr)
	case ModeConfig, ModeTUI:
		deps = fillDependencies(deps)
		if err := runTUI(cmd, deps); err != nil {
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
	if options.CloseLiveRefresh != nil {
		defer func() { _ = options.CloseLiveRefresh() }()
	}
	if deps.RunTUIProgram != nil {
		return deps.RunTUIProgram(options)
	}
	model := tui.NewModel(options)
	_, err = tea.NewProgram(model).Run()
	return err
}

func buildTUIOptions(cmd Command, deps Dependencies) (tui.Options, error) {
	deps = fillDependencies(deps)
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
		Home:  home,
		Now:   now,
		Limit: cmd.Limit,
		ID:    cmd.ID,
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
	options := tui.OptionsFromSnapshot(tui.SnapshotOptionsInput{
		Result:       result,
		Dependencies: tuiDependencies(cmd, deps, home),
		StartMode:    startMode,
	})
	if startMode != tui.StartConfig && result.ProjectsDir != "" {
		watcher, closer, err := deps.NewLiveWatcher(result.ProjectsDir)
		if err != nil {
			options.Refresh.Watcher = refresh.State{
				Status:              refresh.StatusDegraded,
				Messages:            []string{err.Error()},
				SafetyRefreshActive: true,
			}
		} else {
			options.LiveRefresh = tui.LiveRefreshCommand(watcher)
			options.CloseLiveRefresh = closer
		}
	}
	return options
}

func defaultLiveWatcher(projectsDir string) (tui.Watcher, func() error, error) {
	fs, err := refresh.NewFSNotifyFS()
	if err != nil {
		return nil, nil, err
	}
	watcher, err := refresh.NewWatcher(projectsDir, fs)
	if err != nil {
		_ = fs.Close()
		return nil, nil, err
	}
	return watcher, fs.Close, nil
}

func tuiDependencies(cmd Command, deps Dependencies, home string) tui.Dependencies {
	notifyEvent := deps.NotifyEvent
	resetNotificationSuppression := deps.ResetNotificationSuppression
	if notifyEvent == nil || resetNotificationSuppression == nil {
		manager := notify.NewManager(notify.NewPlatformNotifier(notify.ExecRunner))
		if notifyEvent == nil {
			notifyEvent = manager.Notify
		}
		if resetNotificationSuppression == nil {
			resetNotificationSuppression = manager.ResetSuppression
		}
	}
	runner := keepalive.NewSubprocessRunner()
	if os.Getenv("CC_WATCH_KEEPALIVE_LOG") != "off" {
		keepalive.LogDir = filepath.Join(home, ".config", "cc-watch")
	}
	return tui.Dependencies{
		RefreshSnapshot: func(selected *session.Session) tui.RefreshSnapshot {
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
				Home:  home,
				Now:   deps.Now(),
				Limit: cmd.Limit,
				ID:    cmd.ID,
			}, snapshot.Loaders{
				LoadConfig:   config.Load,
				DiscoverHome: deps.DiscoverHome,
				ParseFile:    deps.ParseFile,
			})
			if err != nil {
				return tui.RefreshSnapshot{}
			}
			return tui.RefreshSnapshotFromSnapshotResult(result)
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
		InspectStatusline: func() (statusline.Status, error) {
			return statusline.Inspect(home)
		},
		InstallStatusline: func() error {
			return statusline.Install(home)
		},
		UninstallStatusline: func() error {
			return statusline.Uninstall(home)
		},
		NotifyEvent:                  notifyEvent,
		ResetNotificationSuppression: resetNotificationSuppression,
	}
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
	if deps.NewLiveWatcher == nil {
		deps.NewLiveWatcher = defaultLiveWatcher
	}
	if deps.Stdin == nil {
		deps.Stdin = defaults.Stdin
	}
	if deps.RunStatuslineCommand == nil {
		deps.RunStatuslineCommand = defaults.RunStatuslineCommand
	}
	return deps
}
