package tools

import (
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

type FileSystemWatcher interface {
	NotifyChan() chan struct{}
	Stop()
	StartWatch() error
	Exited() bool
}

type fileSystemWatcher struct {
	cfg        *WatcherConfig
	watchDir   string
	notifyChan chan struct{}
	stopChan   chan struct{}
	exited     *atomic.Bool
}

type WatcherConfig struct {
	operation fsnotify.Op
	fileName  string
}

type WatchOption func(*WatcherConfig)

func WithFileWatches(file string) WatchOption {
	return func(wc *WatcherConfig) {
		wc.fileName = file
	}
}

func WithOperationFilter(op fsnotify.Op) WatchOption {
	return func(wc *WatcherConfig) {
		wc.operation = op
	}
}

func NewFileSystemWatcher(watchDirPath string, opts ...WatchOption) FileSystemWatcher {
	w := &fileSystemWatcher{
		cfg:        &WatcherConfig{},
		watchDir:   watchDirPath,
		notifyChan: make(chan struct{}),
		stopChan:   make(chan struct{}),
		exited:     new(atomic.Bool),
	}

	for _, opt := range opts {
		opt(w.cfg)
	}
	return w
}

func (w *fileSystemWatcher) StartWatch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add(w.watchDir)
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			w.exited.Store(true)
			close(w.notifyChan)
			watcher.Close()
		}()
		for {
			select {
			case event := <-watcher.Events:
				logrus.Debugf("event: %+v: filtering for: %s \n", event, w.cfg.fileName)
				if w.matchesFilter(event) {
					return
				}
			case err := <-watcher.Errors:
				logrus.Errorf("watch error: %s", err.Error())
			case <-w.stopChan:
				return
			}
		}
	}()
	return nil
}

func (w *fileSystemWatcher) Stop() {
	close(w.stopChan)
}

func (w *fileSystemWatcher) Exited() bool {
	return w.exited.Load()
}

func (w *fileSystemWatcher) NotifyChan() chan struct{} {
	return w.notifyChan
}

func (w *fileSystemWatcher) matchesFilter(event fsnotify.Event) bool {
	return event.Op&w.cfg.operation == w.cfg.operation &&
		w.cfg.fileName == event.Name
}
