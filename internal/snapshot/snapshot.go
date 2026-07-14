package snapshot

import (
	"fmt"
	"time"

	"github.com/rcverse/cc-watch/internal/config"
	"github.com/rcverse/cc-watch/internal/session"
)

type Loaders struct {
	LoadConfig   func(home string) (config.Config, error)
	DiscoverHome func(home string, limit int) (session.DiscoveryResult, error)
	ParseFile    func(path string) (session.Session, error)
}

type Request struct {
	Home  string
	Now   time.Time
	Limit int
	ID    string
}

type EmptyState string

const (
	EmptyNone        EmptyState = ""
	EmptyProjectsDir EmptyState = "projects_dir_missing"
	EmptyNoSessions  EmptyState = "no_sessions"
)

type Error struct {
	Code  string
	Query string
}

type Result struct {
	GeneratedAt time.Time
	Config      config.Config
	ProjectsDir string
	EmptyState  EmptyState
	Sessions    []session.Session
	Selected    *session.Session
	Candidates  []session.Session
	Error       *Error
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
	cfg, err := loaders.LoadConfig(req.Home)
	if err != nil {
		return Result{}, err
	}
	generatedAt := req.Now
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	return Result{
		GeneratedAt: generatedAt,
		Config:      cloneConfig(cfg),
	}, nil
}

func buildSelected(req Request, loaders Loaders, result Result, files []session.SessionFile) (Result, error) {
	selectedFile, err := session.ResolvePartialID(files, req.ID)
	if err != nil {
		if resolveErr, ok := err.(*session.ResolveError); ok {
			result.Error = &Error{
				Code:  resolveErr.Code,
				Query: resolveErr.Query,
			}
			result.Candidates = sessionFilesToCandidates(resolveErr.Candidates)
			return result, nil
		}
		result.Error = &Error{
			Code:  "session_not_found",
			Query: req.ID,
		}
		return result, nil
	}
	parsed, err := loaders.ParseFile(selectedFile.Path)
	if err != nil {
		return Result{}, &BuildError{Stage: StageParse, Code: "parse_error", Err: err}
	}
	selected := cloneSession(parsed)
	result.Sessions = []session.Session{selected}
	result.Selected = &result.Sessions[0]
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

func cloneConfig(cfg config.Config) config.Config {
	cfg.ReminderThresholds = append([]int(nil), cfg.ReminderThresholds...)
	return cfg
}

func cloneSession(s session.Session) session.Session {
	s.Gaps = append([]session.Gap(nil), s.Gaps...)
	if s.DurationSeconds != nil {
		durationSeconds := *s.DurationSeconds
		s.DurationSeconds = &durationSeconds
	}
	if s.CacheAnchorAt != nil {
		cacheAnchorAt := *s.CacheAnchorAt
		s.CacheAnchorAt = &cacheAnchorAt
	}
	return s
}
