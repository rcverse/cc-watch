package tui

import (
	"errors"
	"time"

	"github.com/richardchen/cc-cache/internal/keepalive"
	"github.com/richardchen/cc-cache/internal/notify"
	"github.com/richardchen/cc-cache/internal/refresh"
	"github.com/richardchen/cc-cache/internal/session"
)

type DisplayTickMsg struct {
	Now time.Time
}

type WatcherEventMsg struct {
	Path string
	Op   string
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

type SafetyRefreshMsg struct{}

type NotificationResultMsg struct {
	Event  notify.Event
	Result notify.Result
}

type KeepAliveCountdownElapsedMsg struct {
	SessionID     string
	InstanceToken int64
	Now           time.Time
	Generation    int
	SelectedID    string
}

type KeepAliveRunnerResultMsg struct {
	SessionID          string
	InstanceToken      int64
	StartedAt          time.Time
	Err                error
	Reason             string
	Generation         int
	SelectedID         string
	ConfirmationTarget keepalive.ConfirmationTarget
}

type KeepAliveConfirmationResultMsg struct {
	SessionID     string
	InstanceToken int64
	ConfirmedAt   time.Time
	Err           error
	Generation    int
	SelectedID    string
}

var ErrKeepAliveStaleMessage = errors.New("stale keepalive message")
