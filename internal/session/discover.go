package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SessionFile struct {
	SessionID string
	ShortID   string
	Project   string
	Path      string
	ModTime   time.Time
}

type DiscoveryResult struct {
	ProjectsDir string
	Sessions    []SessionFile
	Degraded    bool
	ErrorCode   string
	Messages    []string
}

type ResolveError struct {
	Code       string
	Query      string
	Candidates []SessionFile
}

func (e *ResolveError) Error() string {
	switch e.Code {
	case "session_not_found":
		return fmt.Sprintf("partial id %q did not match any session", e.Query)
	case "ambiguous_session_id":
		return fmt.Sprintf("partial id %q matched multiple sessions", e.Query)
	default:
		return fmt.Sprintf("partial id %q failed with %s", e.Query, e.Code)
	}
}

func DiscoverHome(home string, limit int) (DiscoveryResult, error) {
	return DiscoverProjects(filepath.Join(home, ".claude", "projects"), limit)
}

func DiscoverProjects(projectsDir string, limit int) (DiscoveryResult, error) {
	result := DiscoveryResult{
		ProjectsDir: projectsDir,
	}

	info, err := os.Stat(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			result.Degraded = true
			result.ErrorCode = "projects_dir_missing"
			result.Messages = []string{"projects directory does not exist"}
			return result, nil
		}
		return result, err
	}
	if !info.IsDir() {
		result.Degraded = true
		result.ErrorCode = "projects_dir_missing"
		result.Messages = []string{"projects path is not a directory"}
		return result, nil
	}

	var sessions []SessionFile
	err = filepath.WalkDir(projectsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		id := stem(path)
		sessions = append(sessions, SessionFile{
			SessionID: id,
			ShortID:   shortID(id),
			Project:   projectName(path),
			Path:      path,
			ModTime:   info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return result, err
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	result.Sessions = sessions
	return result, nil
}

func ResolvePartialID(sessions []SessionFile, query string) (SessionFile, error) {
	normalized := strings.ToLower(query)
	var matches []SessionFile
	for _, session := range sessions {
		if strings.Contains(strings.ToLower(session.SessionID), normalized) {
			matches = append(matches, session)
		}
	}

	switch len(matches) {
	case 0:
		return SessionFile{}, &ResolveError{
			Code:  "session_not_found",
			Query: query,
		}
	case 1:
		return matches[0], nil
	default:
		return SessionFile{}, &ResolveError{
			Code:       "ambiguous_session_id",
			Query:      query,
			Candidates: matches,
		}
	}
}
