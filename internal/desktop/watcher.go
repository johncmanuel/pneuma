package desktop

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pneuma/internal/media"
)

// LocalWatcher monitors the local filesystem for changes, debouncing rapid
// OS events and triggering the Scanner or LocalStore accordingly.
type LocalWatcher struct {
	watcher        *fsnotify.Watcher
	watchedRoots   []string
	pendingCreates map[string]*time.Timer
	mu             sync.RWMutex

	ctx     context.Context
	store   *AppStore
	scanner *Scanner
}

// NewLocalWatcher creates the fsnotify watcher and starts the event loop.
func NewLocalWatcher(ctx context.Context, store *AppStore, scanner *Scanner) (*LocalWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	lw := &LocalWatcher{
		watcher:        w,
		watchedRoots:   make([]string, 0),
		pendingCreates: make(map[string]*time.Timer),
		ctx:            ctx,
		store:          store,
		scanner:        scanner,
	}

	go lw.runLocalWatcher()

	return lw, nil
}

// Close stops the background event loop and closes the fsnotify watcher.
func (lw *LocalWatcher) Close() error {
	if lw.watcher != nil {
		return lw.watcher.Close()
	}
	return nil
}

// WatchFolder recursively adds dir (and all its subdirectories) to the
// fsnotify watcher so that file removals are detected.
func (lw *LocalWatcher) WatchFolder(dir string) error {
	if lw.watcher == nil {
		return nil
	}

	// Track the root folder so Create events can upsert to the right folder.
	lw.mu.Lock()
	alreadyRoot := false

	for _, r := range lw.watchedRoots {
		if r == dir {
			alreadyRoot = true
			break
		}
	}

	if !alreadyRoot {
		lw.watchedRoots = append(lw.watchedRoots, dir)
	}

	lw.mu.Unlock()

	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		// skip unreadable paths
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if addErr := lw.watcher.Add(path); addErr != nil {
				slog.Warn("watcher: failed to watch dir", "path", path, "err", addErr)
			}
		}
		return nil
	})
}

// UnwatchFolder removes dir (and all subdirectories currently in the
// watch list) from the fsnotify watcher.
func (lw *LocalWatcher) UnwatchFolder(dir string) error {
	if lw.watcher == nil {
		return nil
	}
	// Remove from root list.
	lw.mu.Lock()
	roots := lw.watchedRoots[:0]
	for _, r := range lw.watchedRoots {
		if r != dir {
			roots = append(roots, r)
		}
	}
	lw.watchedRoots = roots
	lw.mu.Unlock()

	for _, watched := range lw.watcher.WatchList() {
		if watched == dir || strings.HasPrefix(watched, dir+string(filepath.Separator)) {
			if err := lw.watcher.Remove(watched); err != nil {
				slog.Warn("watcher: failed to unwatch dir", "path", watched, "err", err)
			}
		}
	}
	return nil
}

// rootFolderFor returns the registered root folder that contains path,
// or "" if none is found.
func (lw *LocalWatcher) rootFolderFor(path string) string {
	lw.mu.RLock()
	defer lw.mu.RUnlock()
	best := ""
	for _, r := range lw.watchedRoots {
		if (path == r || strings.HasPrefix(path, r+string(filepath.Separator))) && len(r) > len(best) {
			best = r
		}
	}
	return best
}

// runLocalWatcher is the background goroutine that processes fsnotify events.
func (lw *LocalWatcher) runLocalWatcher() {
	for {
		select {
		case event, ok := <-lw.watcher.Events:
			if !ok {
				return
			}
			lw.handleWatcherEvent(event)

		case err, ok := <-lw.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("local file watcher error", "err", err)
		}
	}
}

// handleWatcherEvent processes a single fsnotify event.
func (lw *LocalWatcher) handleWatcherEvent(event fsnotify.Event) {
	path := event.Name

	switch {
	// handle events where a file or directory is created
	case event.Has(fsnotify.Create):
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		if info.IsDir() {
			if addErr := lw.watcher.Add(path); addErr != nil {
				slog.Warn("watcher: failed to add new dir", "path", path, "err", addErr)
			}
		} else {
			ext := strings.ToLower(filepath.Ext(path))
			if !media.IsSupportedAudio(ext) {
				return
			}
			lw.mu.Lock()

			// about 600ms delay to allow for file to be fully written
			delayMs := 600 * time.Millisecond

			if t, exists := lw.pendingCreates[path]; exists {
				t.Reset(delayMs)
			} else {
				lw.pendingCreates[path] = time.AfterFunc(delayMs, func() {
					lw.mu.Lock()
					delete(lw.pendingCreates, path)
					lw.mu.Unlock()

					folder := lw.rootFolderFor(path)
					if folder == "" {
						return
					}
					lt, err := lw.scanner.ScanAndUpsertSingleFile(path, folder)
					if err != nil {
						slog.Warn("watcher: failed to upsert new file", "path", path, "err", err)
						return
					}
					if lw.ctx != nil {
						runtime.EventsEmit(lw.ctx, "local:track:added", map[string]any{
							"path":  path,
							"track": lt,
						})
					}
				})
			}
			lw.mu.Unlock()
		}

	// handle events where a file or directory is removed or renamed
	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		ext := strings.ToLower(filepath.Ext(path))
		if media.IsSupportedAudio(ext) {
			if err := lw.store.deleteLocalTrackByPath(path); err != nil {
				slog.Warn("watcher: failed to delete track from DB", "path", path, "err", err)
			}
			if lw.ctx != nil {
				runtime.EventsEmit(lw.ctx, "local:track:removed", map[string]any{"path": path})
			}
		} else if ext == "" || !strings.Contains(filepath.Base(path), ".") {
			// directory was moved/deleted, delete all tracks under it.
			n, err := lw.store.deleteLocalTracksByPathPrefix(path)
			if err != nil {
				slog.Warn("watcher: failed to delete tracks by prefix", "path", path, "err", err)
			}
			if n > 0 && lw.ctx != nil {
				runtime.EventsEmit(lw.ctx, "local:track:removed", map[string]any{"path": path})
			}
		}
	}
}
