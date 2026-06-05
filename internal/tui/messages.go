package tui

import (
	"time"

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
	Generation int
	Sessions   []session.Session
	Refresh    RefreshViewState
	HasRefresh bool
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
