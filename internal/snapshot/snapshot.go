package snapshot

import (
	"fmt"
	"time"

	"github.com/richardchen/cc-cache/internal/config"
	"github.com/richardchen/cc-cache/internal/session"
)

type Loaders struct {
	LoadConfig   func(home string) (config.LoadResult, error)
	DiscoverHome func(home string, limit int) (session.DiscoveryResult, error)
	ParseFile    func(path string) (session.Session, error)
}

type Request struct {
	Home   string
	Now    time.Time
	Limit  int
	ID     string
	Remind bool
}

type EmptyState string

const (
	EmptyNone        EmptyState = ""
	EmptyProjectsDir EmptyState = "projects_dir_missing"
	EmptyNoSessions  EmptyState = "no_sessions"
)

type Error struct {
	Code    string
	Message string
	Query   string
}

type ReminderState struct {
	Enabled    bool
	Thresholds []int
}

type KeepAliveState struct {
	Enabled  bool
	AutoSend bool
	Mode     string
	MaxSends int
	State    string
}

type Result struct {
	GeneratedAt    time.Time
	QueryID        string
	QueryLimit     int
	Config         config.Config
	ConfigWarnings []config.Warning
	ProjectsDir    string
	EmptyState     EmptyState
	Sessions       []session.Session
	Selected       *session.Session
	Candidates     []session.Session
	Reminder       map[string]ReminderState
	KeepAlive      map[string]KeepAliveState
	Error          *Error
}

type ErrorStage string

const (
	StageConfig    ErrorStage = "config"
	StageDiscovery ErrorStage = "discovery"
	StageParse     ErrorStage = "parse"
)

type BuildError struct {
	Stage ErrorStage
	Code  string
	Err   error
}

func (e *BuildError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s %s: %v", e.Stage, e.Code, e.Err)
}

func (e *BuildError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Build(req Request, loaders Loaders) (Result, error) {
	if req.Limit <= 0 {
		req.Limit = 5
	}
	result, err := loadBase(req, loaders)
	if err != nil {
		return Result{}, &BuildError{Stage: StageConfig, Code: "config_error", Err: err}
	}
	discoveryLimit := req.Limit
	if req.ID != "" {
		discoveryLimit = 0
	}
	discovery, err := loaders.DiscoverHome(req.Home, discoveryLimit)
	if err != nil {
		return Result{}, &BuildError{Stage: StageDiscovery, Code: "parse_error", Err: err}
	}
	result.ProjectsDir = discovery.ProjectsDir
	if discovery.ErrorCode == "projects_dir_missing" {
		result.EmptyState = EmptyProjectsDir
	}
	if req.ID != "" {
		return buildSelected(req, loaders, result, discovery.Sessions)
	}
	for _, file := range discovery.Sessions {
		parsed, err := loaders.ParseFile(file.Path)
		if err != nil {
			return Result{}, &BuildError{Stage: StageParse, Code: "parse_error", Err: err}
		}
		result.Sessions = append(result.Sessions, cloneSession(parsed))
	}
	if len(result.Sessions) == 0 && result.EmptyState == EmptyNone {
		result.EmptyState = EmptyNoSessions
	}
	result.populateRuntime(req.Remind)
	return result, nil
}

func ConfigOnly(req Request, loaders Loaders) (Result, error) {
	result, err := loadBase(req, loaders)
	if err != nil {
		return Result{}, &BuildError{Stage: StageConfig, Code: "config_error", Err: err}
	}
	return result, nil
}

func loadBase(req Request, loaders Loaders) (Result, error) {
	cfgResult, err := loaders.LoadConfig(req.Home)
	if err != nil {
		return Result{}, err
	}
	generatedAt := req.Now
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	return Result{
		GeneratedAt:    generatedAt,
		QueryID:        req.ID,
		QueryLimit:     limit,
		Config:         cloneConfig(cfgResult.Config),
		ConfigWarnings: cloneWarnings(cfgResult.Warnings),
		Reminder:       map[string]ReminderState{},
		KeepAlive:      map[string]KeepAliveState{},
	}, nil
}

func buildSelected(req Request, loaders Loaders, result Result, files []session.SessionFile) (Result, error) {
	selectedFile, err := session.ResolvePartialID(files, req.ID)
	if err != nil {
		if resolveErr, ok := err.(*session.ResolveError); ok {
			result.Error = &Error{
				Code:    resolveErr.Code,
				Message: resolveErr.Error(),
				Query:   resolveErr.Query,
			}
			result.Candidates = sessionFilesToCandidates(resolveErr.Candidates)
			result.populateRuntime(req.Remind)
			return result, nil
		}
		result.Error = &Error{
			Code:    "session_not_found",
			Message: err.Error(),
			Query:   req.ID,
		}
		result.populateRuntime(req.Remind)
		return result, nil
	}
	parsed, err := loaders.ParseFile(selectedFile.Path)
	if err != nil {
		return Result{}, &BuildError{Stage: StageParse, Code: "parse_error", Err: err}
	}
	selected := cloneSession(parsed)
	result.Sessions = []session.Session{selected}
	result.Selected = &result.Sessions[0]
	result.populateRuntime(req.Remind)
	return result, nil
}

func sessionFilesToCandidates(files []session.SessionFile) []session.Session {
	candidates := make([]session.Session, 0, len(files))
	for _, file := range files {
		candidates = append(candidates, session.Session{
			SessionID:      file.SessionID,
			ShortID:        file.ShortID,
			Project:        file.Project,
			JSONLPath:      file.Path,
			FileModifiedAt: file.ModTime,
		})
	}
	return candidates
}

func (r *Result) populateRuntime(remind bool) {
	r.Reminder = map[string]ReminderState{}
	r.KeepAlive = map[string]KeepAliveState{}
	sessions := r.Sessions
	if len(sessions) == 0 && len(r.Candidates) > 0 {
		sessions = r.Candidates
	}
	for _, s := range sessions {
		r.Reminder[s.SessionID] = ReminderState{
			Enabled:    remind,
			Thresholds: append([]int(nil), r.Config.ReminderThresholds...),
		}
		r.KeepAlive[s.SessionID] = KeepAliveState{
			Enabled:  false,
			AutoSend: r.Config.KeepAlive.AutoSend,
			Mode:     r.Config.KeepAlive.Scope.Mode,
			MaxSends: r.Config.KeepAlive.Scope.MaxSends,
			State:    "off",
		}
	}
}

func cloneConfig(cfg config.Config) config.Config {
	cfg.ReminderThresholds = append([]int(nil), cfg.ReminderThresholds...)
	return cfg
}

func cloneSessions(sessions []session.Session) []session.Session {
	cloned := make([]session.Session, 0, len(sessions))
	for _, s := range sessions {
		cloned = append(cloned, cloneSession(s))
	}
	return cloned
}

func cloneSession(s session.Session) session.Session {
	s.CacheWindow.Evidence = append([]string(nil), s.CacheWindow.Evidence...)
	s.Gaps = append([]session.Gap(nil), s.Gaps...)
	s.Warnings = append([]session.ParseWarning(nil), s.Warnings...)
	if s.StartedAt != nil {
		startedAt := *s.StartedAt
		s.StartedAt = &startedAt
	}
	if s.EndedAt != nil {
		endedAt := *s.EndedAt
		s.EndedAt = &endedAt
	}
	if s.DurationSeconds != nil {
		durationSeconds := *s.DurationSeconds
		s.DurationSeconds = &durationSeconds
	}
	if s.LastMessageAt != nil {
		lastMessageAt := *s.LastMessageAt
		s.LastMessageAt = &lastMessageAt
	}
	return s
}

func cloneWarnings(warnings []config.Warning) []config.Warning {
	return append([]config.Warning(nil), warnings...)
}
