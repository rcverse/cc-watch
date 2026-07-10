package tui

import (
	"errors"
	"time"

	"github.com/richardchen/cc-watch/internal/keepalive"
	"github.com/richardchen/cc-watch/internal/notify"
	"github.com/richardchen/cc-watch/internal/refresh"
	"github.com/richardchen/cc-watch/internal/session"
)

type DisplayTickMsg struct {
	Now time.Time
}

type RefreshTickMsg struct {
	Now time.Time
}

type RefreshWatcherEventsMsg struct {
	Events []refresh.NormalizedEvent
	State  refresh.State
}

type RefreshWatcherDegradedMsg struct {
	State refresh.State
}

type RefreshWatcherClosedMsg struct {
	State refresh.State
}

type RefreshDebounceElapsedMsg struct {
	Now   time.Time
	Token int
}

type RefreshResultMsg struct {
	Generation   int
	Sessions     []session.Session
	Refresh      RefreshViewState
	HasRefresh   bool
	SelectedOnly bool
	SelectedID   string
}

type RefreshDegradedMsg struct {
	State refresh.State
}

type ManualRefreshMsg struct{}

type NotificationResultMsg struct {
	Event  notify.Event
	Result notify.Result
}

type KeepAliveCountdownElapsedMsg struct {
	SessionID     string
	InstanceToken int64
	Now           time.Time
	SelectedID    string
}

type KeepAliveRunnerResultMsg struct {
	SessionID          string
	InstanceToken      int64
	StartedAt          time.Time
	Err                error
	Reason             string
	Action             keepalive.Action
	Execution          keepalive.RunnerExecution
	SelectedID         string
	ConfirmationTarget keepalive.ConfirmationTarget
}

type KeepAliveConfirmationResultMsg struct {
	SessionID     string
	InstanceToken int64
	ConfirmedAt   time.Time
	Err           error
	SelectedID    string
}

var ErrKeepAliveStaleMessage = errors.New("stale keepalive message")
