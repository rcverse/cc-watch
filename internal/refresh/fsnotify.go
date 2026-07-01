package refresh

import (
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type FSNotifyFS struct {
	watcher *fsnotify.Watcher
	events  chan RawEvent
	errs    chan error
}

func NewFSNotifyFS() (*FSNotifyFS, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	fs := &FSNotifyFS{
		watcher: watcher,
		events:  make(chan RawEvent, 64),
		errs:    make(chan error, 16),
	}
	go fs.forward()
	return fs, nil
}

func (fs *FSNotifyFS) WalkDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	return dirs, err
}

func (fs *FSNotifyFS) Watch(path string) error {
	return fs.watcher.Add(path)
}

func (fs *FSNotifyFS) IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (fs *FSNotifyFS) FatalWatchError(_ string) bool {
	return false
}

func (fs *FSNotifyFS) Events() <-chan RawEvent {
	return fs.events
}

func (fs *FSNotifyFS) Errors() <-chan error {
	return fs.errs
}

func (fs *FSNotifyFS) Close() error {
	return fs.watcher.Close()
}

func (fs *FSNotifyFS) forward() {
	forwardWatchStreams(fs.watcher.Events, fs.watcher.Errors, fs.events, fs.errs)
}

func forwardWatchStreams(watcherEvents <-chan fsnotify.Event, watcherErrs <-chan error, events chan<- RawEvent, errs chan<- error) {
	defer close(events)
	defer close(errs)
	for watcherEvents != nil || watcherErrs != nil {
		select {
		case event, ok := <-watcherEvents:
			if !ok {
				watcherEvents = nil
				continue
			}
			select {
			case events <- RawEvent{Path: event.Name, Op: fsnotifyOp(event.Op)}:
			default:
			}
		case err, ok := <-watcherErrs:
			if !ok {
				watcherErrs = nil
				continue
			}
			select {
			case errs <- err:
			default:
			}
		}
	}
}

func fsnotifyOp(op fsnotify.Op) Op {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return OpCreate
	case op&fsnotify.Write == fsnotify.Write:
		return OpWrite
	case op&fsnotify.Rename == fsnotify.Rename:
		return OpRename
	case op&fsnotify.Remove == fsnotify.Remove:
		return OpDelete
	default:
		return OpWrite
	}
}
