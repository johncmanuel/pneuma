package desktop

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"pneuma/internal/media"

	"github.com/dhowden/tag"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// LibraryManager is the single authority for translating filesystem state into
// LocalStore state. It handles full lifecycle synchronization: scanning folders,
// adding new files, and removing deleted files. All Wails-related library events are
// emitted here, giving callers (e.g. LocalWatcher) high leverage behind a small
// interface.
type LibraryManager struct {
	ctx   context.Context
	store *AppStore
}

// NewLibraryManager creates a LibraryManager with the given Wails context and AppStore.
func NewLibraryManager(ctx context.Context, store *AppStore) *LibraryManager {
	return &LibraryManager{ctx: ctx, store: store}
}

// SyncFolder recursively scans a directory for audio files,
// reads embedded tags, persists each track to the LocalStore, prunes
// any DB entries for files that no longer exist, and emits the following
// Wails events for frontend progress display:
//
//	"local:scan:start"    -> { folder string, total int }
//	"local:track:scanned" -> { folder string, done int, total int, track LocalTrack }
//	"local:track:removed" -> { paths []string }
//	"local:scan:done"     -> { folder string, count int }
func (lm *LibraryManager) SyncFolder(dir string) error {
	// count audio files first so we know the total
	var total int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if media.IsSupportedAudio(strings.ToLower(filepath.Ext(path))) {
			total++
		}
		return nil
	})

	wailsruntime.EventsEmit(lm.ctx, "local:scan:start", map[string]any{
		"folder": dir,
		"total":  total,
	})

	done := 0
	livePaths := make(map[string]struct{}, total)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !media.IsSupportedAudio(strings.ToLower(filepath.Ext(path))) {
			return nil
		}

		lt := LocalTrack{Path: path, Title: filepath.Base(path)}
		livePaths[path] = struct{}{}

		f, err := os.Open(path)
		if err != nil {
			done++
			_ = lm.store.upsertLocalTrack(lt, dir)
			wailsruntime.EventsEmit(lm.ctx, "local:track:scanned", map[string]any{
				"folder": dir, "done": done, "total": total, "track": lt,
			})
			return nil
		}

		m, tagErr := tag.ReadFrom(f)
		f.Close()

		if tagErr == nil {
			if m.Title() != "" {
				lt.Title = m.Title()
			}
			lt.Artist = m.Artist()
			lt.Album = m.Album()
			lt.AlbumArtist = m.AlbumArtist()
			lt.Genre = m.Genre()
			lt.Year = m.Year()
			tn, _ := m.Track()
			lt.TrackNumber = tn
			dn, _ := m.Disc()
			lt.DiscNumber = dn
			lt.HasArtwork = m.Picture() != nil
		}

		if info, statErr := os.Stat(path); statErr == nil {
			probeLocalDuration(path, info, &lt)
			if lt.DurationMs == 0 {
				parseDurationFallbackLocal(path, info, &lt)
			}
		}

		_ = lm.store.upsertLocalTrack(lt, dir)
		done++
		wailsruntime.EventsEmit(lm.ctx, "local:track:scanned", map[string]any{
			"folder": dir, "done": done, "total": total, "track": lt,
		})
		return nil
	})

	// prune DB entries for files that no longer exist on disk
	if stalePaths, pruneErr := lm.store.pruneStaleLocalTracks(dir, livePaths); pruneErr != nil {
		slog.Warn("sync: failed to prune stale tracks", "folder", dir, "err", pruneErr)
	} else if len(stalePaths) > 0 {
		wailsruntime.EventsEmit(lm.ctx, "local:track:removed", map[string]any{
			"paths": stalePaths,
		})
	}

	wailsruntime.EventsEmit(lm.ctx, "local:scan:done", map[string]any{
		"folder": dir,
		"count":  done,
	})
	return err
}

// HandleFileAdded reads metadata for one audio file, persists it to the
// LocalStore, emits "local:track:added", and returns the populated LocalTrack.
// Used by the LocalWatcher on Create events.
func (lm *LibraryManager) HandleFileAdded(path, folder string) (LocalTrack, error) {
	lt := LocalTrack{Path: path, Title: filepath.Base(path)}

	f, err := os.Open(path)
	if err != nil {
		upsertErr := lm.store.upsertLocalTrack(lt, folder)
		lm.emitTrackAdded(path, lt)
		return lt, upsertErr
	}

	m, tagErr := tag.ReadFrom(f)
	f.Close()

	if tagErr == nil {
		if m.Title() != "" {
			lt.Title = m.Title()
		}
		lt.Artist = m.Artist()
		lt.Album = m.Album()
		lt.AlbumArtist = m.AlbumArtist()
		lt.Genre = m.Genre()
		lt.Year = m.Year()
		tn, _ := m.Track()
		lt.TrackNumber = tn
		dn, _ := m.Disc()
		lt.DiscNumber = dn
		lt.HasArtwork = m.Picture() != nil
	}

	if info, err := os.Stat(path); err == nil {
		probeLocalDuration(path, info, &lt)
		if lt.DurationMs == 0 {
			parseDurationFallbackLocal(path, info, &lt)
		}
	}

	upsertErr := lm.store.upsertLocalTrack(lt, folder)
	lm.emitTrackAdded(path, lt)
	return lt, upsertErr
}

// HandleFileRemoved deletes a single track from the LocalStore and emits
// "local:track:removed". Used by the LocalWatcher on Remove/Rename events.
func (lm *LibraryManager) HandleFileRemoved(path string) {
	if err := lm.store.deleteLocalTrackByPath(path); err != nil {
		slog.Warn("library: failed to delete track from DB", "path", path, "err", err)
	}
	wailsruntime.EventsEmit(lm.ctx, "local:track:removed", map[string]any{"path": path})
}

// HandleFolderRemoved deletes all tracks under a directory prefix from the
// LocalStore and emits "local:track:removed". Used by the LocalWatcher when a
// watched directory is moved or deleted.
func (lm *LibraryManager) HandleFolderRemoved(path string) {
	n, err := lm.store.deleteLocalTracksByPathPrefix(path)
	if err != nil {
		slog.Warn("library: failed to delete tracks by prefix", "path", path, "err", err)
	}
	if n > 0 {
		wailsruntime.EventsEmit(lm.ctx, "local:track:removed", map[string]any{"path": path})
	}
}

// emitTrackAdded emits the "local:track:added" Wails event.
func (lm *LibraryManager) emitTrackAdded(path string, lt LocalTrack) {
	if lm.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(lm.ctx, "local:track:added", map[string]any{
		"path":  path,
		"track": lt,
	})
}
