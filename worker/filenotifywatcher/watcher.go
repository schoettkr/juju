// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package filenotifywatcher

import (
	"path/filepath"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/worker/v3"
	"github.com/juju/worker/v3/catacomb"
	"k8s.io/utils/inotify"
)

const (
	defaultWatcherPath = "/var/lib/juju/agents/watchers"
)

// FileWatcher is an interface that allows a worker to watch a file for changes.
type FileWatcher interface {
	worker.Worker
	// Changes returns a channel that will receive a value whenever the
	// watched file changes.
	Changes() <-chan bool
}

// INotifyWatcher is an interface that allows a worker to watch a file for
// changes using inotify.
type INotifyWatcher interface {
	// Watch adds the given file or directory (non-recursively) to the watch.
	Watch(path string) error

	// Event returns the next event.
	Events() <-chan *inotify.Event

	// Error returns the next error.
	Errors() <-chan error

	// Close removes all watches and closes the events channel.
	Close() error
}

type option struct {
	path      string
	logger    Logger
	watcherFn func() (INotifyWatcher, error)
}

type Option func(*option)

// WithPath is an option for NewWatcher that specifies the path to watch.
func WithPath(path string) Option {
	return func(o *option) {
		o.path = path
	}
}

// WithLogger is an option for NewWatcher that specifies the logger to use.
func WithLogger(logger Logger) Option {
	return func(o *option) {
		o.logger = logger
	}
}

// WithINotifyWatcherFn is an option for NewWatcher that specifies the inotify
// watcher to use.
func WithINotifyWatcherFn(watcherFn func() (INotifyWatcher, error)) Option {
	return func(o *option) {
		o.watcherFn = watcherFn
	}
}

func newOption() *option {
	return &option{
		path:      defaultWatcherPath,
		logger:    loggo.GetLogger("juju.worker.filenotifywatcher"),
		watcherFn: newWatcher,
	}
}

type Watcher struct {
	catacomb catacomb.Catacomb

	namespace string
	changes   chan bool

	watchPath string
	watcher   INotifyWatcher

	logger Logger
}

func NewWatcher(namespace string, opts ...Option) (FileWatcher, error) {
	o := newOption()
	for _, opt := range opts {
		opt(o)
	}

	watcher, err := o.watcherFn()
	if err != nil {
		return nil, errors.Annotatef(err, "creating watcher for namespace %q", namespace)
	}
	if err := watcher.Watch(o.path); err != nil {
		return nil, errors.Annotatef(err, "watching namespace %q", namespace)
	}

	w := &Watcher{
		namespace: namespace,
		changes:   make(chan bool),
		watcher:   watcher,
		watchPath: filepath.Join(o.path, namespace),
		logger:    o.logger,
	}

	if err := catacomb.Invoke(catacomb.Plan{
		Site: &w.catacomb,
		Work: w.loop,
	}); err != nil {
		return nil, errors.Trace(err)
	}

	return w, nil
}

// Kill is part of the worker.Worker interface.
func (w *Watcher) Kill() {
	w.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (w *Watcher) Wait() error {
	return w.catacomb.Wait()
}

// Changes returns the changes for the given namespace.
func (w *Watcher) Changes() <-chan bool {
	return w.changes
}

func (w *Watcher) loop() error {
	defer func() {
		_ = w.watcher.Close()
		close(w.changes)
	}()

	for {
		select {
		case <-w.catacomb.Dying():
			return w.catacomb.ErrDying()
		case event := <-w.watcher.Events():
			// Ignore events for other files in the directory.
			if event.Name != w.watchPath {
				continue
			}
			// If the event is not a create or delete event, ignore it.
			if maskType(event.Mask) == unknown {
				continue
			}

			// TODO (stickupkid): Read the file to see how much we want to
			// step for. For now we just send a bool to indicate we should
			// step.
			created := event.Mask&inotify.InCreate != 0
			w.changes <- created

		case err := <-w.watcher.Errors():
			w.logger.Errorf("error watching namespace %q with %v", w.namespace, err)
		}
	}
}

// eventType normalizes the inotify event type, to known types.
type eventType int

const (
	unknown eventType = iota
	created
	deleted
)

// makeType returns the event type for the given mask.
// It expects that created and deleted can never be set at the same time.
func maskType(m uint32) eventType {
	if m&inotify.InCreate != 0 {
		return created
	}
	if m&inotify.InDelete != 0 {
		return deleted
	}
	return unknown
}
