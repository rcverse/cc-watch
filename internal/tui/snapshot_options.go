package tui

import (
	"github.com/rcverse/cc-watch/internal/session"
	"github.com/rcverse/cc-watch/internal/snapshot"
)

type SnapshotOptionsInput struct {
	Result       snapshot.Result
	Dependencies Dependencies
	StartMode    StartMode
}

func OptionsFromSnapshot(input SnapshotOptionsInput) Options {
	result := input.Result
	refreshState := RefreshViewState{
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
			refreshState.EmptyState = EmptyNoSessions
		}
	}
	if refreshState.EmptyState == EmptyNone {
		refreshState.EmptyState = emptyStateFromSnapshot(result.EmptyState)
	}

	return Options{
		Now:                result.GeneratedAt,
		Dependencies:       input.Dependencies,
		Sessions:           sessions,
		SelectedID:         selectedID,
		AmbiguousID:        ambiguousID,
		ReminderThresholds: result.Config.ReminderThresholds,
		KeepAliveConfig:    result.Config.KeepAlive,
		Refresh:            refreshState,
		StartMode:          input.StartMode,
		StartDisplayTicker: true,
		StartRefreshTicker: input.StartMode != StartConfig,
		Config:             result.Config,
	}
}

func RefreshSnapshotFromSnapshotResult(result snapshot.Result) RefreshSnapshot {
	sessions := result.Sessions
	refreshState := RefreshViewState{
		ProjectsDir: result.ProjectsDir,
		EmptyState:  emptyStateFromSnapshot(result.EmptyState),
	}
	if result.Error != nil {
		switch result.Error.Code {
		case "ambiguous_session_id":
			sessions = result.Candidates
		case "session_not_found":
			sessions = nil
			refreshState.EmptyState = EmptyNoSessions
		}
	}
	return RefreshSnapshot{
		Sessions:   sessions,
		Refresh:    refreshState,
		HasRefresh: true,
	}
}

func emptyStateFromSnapshot(state snapshot.EmptyState) EmptyState {
	switch state {
	case snapshot.EmptyProjectsDir:
		return EmptyProjectsDir
	case snapshot.EmptyNoSessions:
		return EmptyNoSessions
	default:
		return EmptyNone
	}
}
