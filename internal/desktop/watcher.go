package desktop

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// initLocalWatcher creates the fsnotify watcher and starts the event loop.
func (a *App) initLocalWatcher() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("local file watcher unavailable", "err", err)
		return
	}
	a.localWatcher = w
	a.pendingCreates = make(map[string]*time.Timer)
	go a.runLocalWatcher()
}

// stopLocalWatcher closes the fsnotify watcher.
func (a *App) stopLocalWatcher() {
	if a.localWatcher != nil {
		a.localWatcher.Close()
	}
}

// WatchLocalFolder recursively adds dir (and all its subdirectories) to the
// fsnotify watcher so that file removals are detected.
func (a *App) WatchLocalFolder(dir string) error {
	if a.localWatcher == nil {
		return nil
	}

	// Track the root folder so Create events can upsert to the right folder.
	a.mu.Lock()
	alreadyRoot := false

	for _, r := range a.watchedRoots {
		if r == dir {
			alreadyRoot = true
			break
		}
	}

	if !alreadyRoot {
		a.watchedRoots = append(a.watchedRoots, dir)
	}

	a.mu.Unlock()

	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		// skip unreadable paths
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if addErr := a.localWatcher.Add(path); addErr != nil {
				slog.Warn("watcher: failed to watch dir", "path", path, "err", addErr)
			}
		}
		return nil
	})
}

// UnwatchLocalFolder removes dir (and all subdirectories currently in the
// watch list) from the fsnotify watcher.
func (a *App) UnwatchLocalFolder(dir string) error {
	if a.localWatcher == nil {
		return nil
	}
	// Remove from root list.
	a.mu.Lock()
	roots := a.watchedRoots[:0]
	for _, r := range a.watchedRoots {
		if r != dir {
			roots = append(roots, r)
		}
	}
	a.watchedRoots = roots
	a.mu.Unlock()

	for _, watched := range a.localWatcher.WatchList() {
		if watched == dir || strings.HasPrefix(watched, dir+string(filepath.Separator)) {
			if err := a.localWatcher.Remove(watched); err != nil {
				slog.Warn("watcher: failed to unwatch dir", "path", watched, "err", err)
			}
		}
	}
	return nil
}

// rootFolderFor returns the registered root folder that contains path,
// or "" if none is found.
func (a *App) rootFolderFor(path string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	best := ""
	for _, r := range a.watchedRoots {
		if (path == r || strings.HasPrefix(path, r+string(filepath.Separator))) && len(r) > len(best) {
			best = r
		}
	}
	return best
}

// runLocalWatcher is the background goroutine that processes fsnotify events.
func (a *App) runLocalWatcher() {
	for {
		select {
		case event, ok := <-a.localWatcher.Events:
			if !ok {
				return
			}
			a.handleWatcherEvent(event)

		case err, ok := <-a.localWatcher.Errors:
			if !ok {
				return
			}
			slog.Warn("local file watcher error", "err", err)
		}
	}
}

// handleWatcherEvent processes a single fsnotify event.
func (a *App) handleWatcherEvent(event fsnotify.Event) {
	path := event.Name

	switch {
	// handle events where a file or directory is created
	case event.Has(fsnotify.Create):
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		if info.IsDir() {
			if addErr := a.localWatcher.Add(path); addErr != nil {
				slog.Warn("watcher: failed to add new dir", "path", path, "err", addErr)
			}
		} else {
			ext := strings.ToLower(filepath.Ext(path))
			if !audioExts[ext] {
				return
			}
			a.mu.Lock()

			// about 600ms delay to allow for file to be fully written
			delayMs := 600 * time.Millisecond

			if t, exists := a.pendingCreates[path]; exists {
				t.Reset(delayMs)
			} else {
				a.pendingCreates[path] = time.AfterFunc(delayMs, func() {
					a.mu.Lock()
					delete(a.pendingCreates, path)
					a.mu.Unlock()

					folder := a.rootFolderFor(path)
					if folder == "" {
						return
					}
					lt, err := a.scanAndUpsertSingleFile(path, folder)
					if err != nil {
						slog.Warn("watcher: failed to upsert new file", "path", path, "err", err)
						return
					}
					if a.ctx != nil {
						runtime.EventsEmit(a.ctx, "local:track:added", map[string]any{
							"path":  path,
							"track": lt,
						})
					}
				})
			}
			a.mu.Unlock()
		}

	// handle events where a file or directory is removed or renamed
	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		ext := strings.ToLower(filepath.Ext(path))
		if audioExts[ext] {
			if err := a.deleteLocalTrackByPath(path); err != nil {
				slog.Warn("watcher: failed to delete track from DB", "path", path, "err", err)
			}
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "local:track:removed", map[string]any{"path": path})
			}
		} else if ext == "" || !strings.Contains(filepath.Base(path), ".") {
			// directory was moved/deleted, delete all tracks under it.
			n, err := a.deleteLocalTracksByPathPrefix(path)
			if err != nil {
				slog.Warn("watcher: failed to delete tracks by prefix", "path", path, "err", err)
			}
			if n > 0 && a.ctx != nil {
				runtime.EventsEmit(a.ctx, "local:track:removed", map[string]any{"path": path})
			}
		}
	}
}
