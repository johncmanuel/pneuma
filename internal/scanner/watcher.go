package scanner

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"pneuma/internal/media"
)

// Watcher monitors directories for audio file changes using OS-level events.
// It delegates all file processing to the manager, keeping itself a thin
// event-dispatch layer responsible only for debouncing and routing fsnotify
// events.
type Watcher struct {
	// mgr is the scanner's manager that handles file operations.
	mgr *Manager
	// watcher is the fsnotify watcher that monitors directories for changes.
	watcher *fsnotify.Watcher
	// mu protects access to the pending map.
	mu sync.Mutex
	// pending is a map of paths that have been recently modified.
	pending map[string]time.Time
	// interval is the interval between flushes.
	interval time.Duration
	// log is the logger for the watcher.
	log *slog.Logger
}

// NewWatcher creates a Watcher that delegates file operations to the Manager.
func NewWatcher(mgr *Manager, interval time.Duration) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		mgr:      mgr,
		watcher:  fw,
		pending:  make(map[string]time.Time),
		interval: interval,
		log:      slog.Default().With("component", "watcher"),
	}, nil
}

// Add registers a directory with the OS watcher.
func (w *Watcher) Add(dir string) error {
	return w.watcher.Add(dir)
}

// Start begins processing file events. It blocks until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	defer w.watcher.Close()

	w.log.Info("watcher started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info("watcher stopped")
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.log.Error("fsnotify error", "err", err)
		case <-ticker.C:
			w.flush(ctx)
		}
	}
}

// handleEvent processes a single fsnotify event, adding create/write events
// to the pending map and handling deletes/renames immediately.
func (w *Watcher) handleEvent(e fsnotify.Event) {
	path := e.Name

	ext := strings.ToLower(filepath.Ext(path))
	if !media.IsSupportedAudio(ext) {
		return
	}

	switch {
	case e.Op&(fsnotify.Create|fsnotify.Write) != 0:
		w.mu.Lock()
		if _, exists := w.pending[path]; !exists {
			w.pending[path] = time.Now()
		}
		w.mu.Unlock()
	case e.Op&fsnotify.Remove != 0:
		w.mgr.HandleFileRemoved(context.Background(), path)
	case e.Op&fsnotify.Rename != 0:
		w.mgr.HandleFileRemoved(context.Background(), path)
	}
}

// flush processes pending file events that have been stable for at least 1 second.
func (w *Watcher) flush(ctx context.Context) {
	w.mu.Lock()
	now := time.Now()
	ready := make([]string, 0)

	for path, t := range w.pending {
		if now.Sub(t) >= time.Second {
			ready = append(ready, path)
			delete(w.pending, path)
		}
	}

	w.mu.Unlock()

	for _, path := range ready {
		w.mgr.HandleFileAdded(ctx, path)
	}
}
