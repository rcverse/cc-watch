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
	rawEvents := make(chan RawEvent)
	rawErrs := make(chan error)
	done := make(chan struct{})
	go func() {
		forwardWatchStreams(rawEvents, rawErrs, fs.events, fs.errs)
		close(done)
	}()
	watcherEvents := fs.watcher.Events
	watcherErrs := fs.watcher.Errors
	for watcherEvents != nil || watcherErrs != nil {
		select {
		case event, ok := <-watcherEvents:
			if !ok {
				close(rawEvents)
				watcherEvents = nil
				continue
			}
			select {
			case rawEvents <- RawEvent{Path: event.Name, Op: fsnotifyOp(event.Op)}:
			default:
			}
		case err, ok := <-watcherErrs:
			if !ok {
				close(rawErrs)
				watcherErrs = nil
				continue
			}
			select {
			case rawErrs <- err:
			default:
			}
		}
	}
	<-done
}

func forwardWatchStreams(rawEvents <-chan RawEvent, rawErrs <-chan error, events chan<- RawEvent, errs chan<- error) {
	defer close(events)
	defer close(errs)
	for rawEvents != nil || rawErrs != nil {
		select {
		case event, ok := <-rawEvents:
			if !ok {
				rawEvents = nil
				continue
			}
			select {
			case events <- event:
			default:
			}
		case err, ok := <-rawErrs:
			if !ok {
				rawErrs = nil
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
