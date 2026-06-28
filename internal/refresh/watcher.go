package refresh

import (
	"errors"
	"fmt"
)

type Status string

const (
	StatusOK       Status = "ok"
	StatusPartial  Status = "partial"
	StatusDegraded Status = "degraded"
)

type State struct {
	Status              Status
	Messages            []string
	SafetyRefreshActive bool
}

type WatchFS interface {
	WalkDirs(root string) ([]string, error)
	Watch(path string) error
	IsDir(path string) bool
	FatalWatchError(path string) bool
	Events() <-chan RawEvent
	Errors() <-chan error
}

type Watcher struct {
	fs      WatchFS
	watched map[string]bool
	state   State
}

type Op string

const (
	OpCreate Op = "create"
	OpWrite  Op = "write"
	OpRename Op = "rename"
	OpDelete Op = "delete"
)

type RawEvent struct {
	Path string
	Op   Op
}

type EventKind string

const (
	EventCreated EventKind = "created"
	EventWritten EventKind = "written"
	EventRenamed EventKind = "renamed"
	EventDeleted EventKind = "deleted"
)

type NormalizedEvent struct {
	Kind EventKind
	Path string
}

type WatcherResult struct {
	Events []NormalizedEvent
	State  State
	Err    error
	Closed bool
}

var ErrWatcherClosed = errors.New("refresh watcher closed")

func NewWatcher(projectsDir string, fs WatchFS) (*Watcher, error) {
	watcher := &Watcher{
		fs:      fs,
		watched: map[string]bool{},
		state: State{
			Status:              StatusOK,
			SafetyRefreshActive: true,
		},
	}

	dirs, err := fs.WalkDirs(projectsDir)
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if err := watcher.addWatch(dir); err != nil && fs.FatalWatchError(dir) {
			return nil, err
		}
	}
	return watcher, nil
}

func (w *Watcher) Normalize(events []RawEvent) []NormalizedEvent {
	normalized := make([]NormalizedEvent, 0, len(events))
	for _, event := range events {
		if event.Op == OpCreate && w.fs.IsDir(event.Path) {
			_ = w.addWatch(event.Path)
		}
		normalized = append(normalized, NormalizedEvent{
			Kind: normalizeKind(event.Op),
			Path: event.Path,
		})
	}
	return normalized
}

func (w *Watcher) Watched(path string) bool {
	return w.watched[path]
}

func (w *Watcher) WatchedPaths() []string {
	paths := make([]string, 0, len(w.watched))
	for path := range w.watched {
		paths = append(paths, path)
	}
	return paths
}

func (w *Watcher) State() State {
	return State{
		Status:              w.state.Status,
		Messages:            append([]string(nil), w.state.Messages...),
		SafetyRefreshActive: w.state.SafetyRefreshActive,
	}
}

func (w *Watcher) Next() WatcherResult {
	select {
	case event, ok := <-w.fs.Events():
		if !ok {
			w.MarkRuntimeError(ErrWatcherClosed)
			return WatcherResult{State: w.State(), Err: ErrWatcherClosed, Closed: true}
		}
		return WatcherResult{
			Events: w.Normalize([]RawEvent{event}),
			State:  w.State(),
		}
	case err, ok := <-w.fs.Errors():
		if !ok {
			w.MarkRuntimeError(ErrWatcherClosed)
			return WatcherResult{State: w.State(), Err: ErrWatcherClosed, Closed: true}
		}
		w.MarkRuntimeError(err)
		return WatcherResult{State: w.State(), Err: err}
	}
}

func (w *Watcher) MarkRuntimeError(err error) {
	w.state.Status = StatusDegraded
	w.state.Messages = append(w.state.Messages, err.Error())
	w.state.SafetyRefreshActive = true
}

func (w *Watcher) addWatch(path string) error {
	if err := w.fs.Watch(path); err != nil {
		w.state.Status = StatusPartial
		w.state.Messages = append(w.state.Messages, fmt.Sprintf("%s: %s", path, err))
		w.state.SafetyRefreshActive = true
		return err
	}
	w.watched[path] = true
	return nil
}

func normalizeKind(op Op) EventKind {
	switch op {
	case OpCreate:
		return EventCreated
	case OpWrite:
		return EventWritten
	case OpRename:
		return EventRenamed
	case OpDelete:
		return EventDeleted
	default:
		return EventWritten
	}
}
