package desktop

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// scanAndUpsertSingleFile reads metadata for one audio file and persists it
// to the local DB. Used by the fsnotify watcher on Create events.
// Returns the populated LocalTrack so the caller can include it in events.
func (a *App) scanAndUpsertSingleFile(path, folder string) (LocalTrack, error) {
	lt := LocalTrack{Path: path, Title: filepath.Base(path)}

	f, err := os.Open(path)
	if err != nil {
		return lt, a.upsertLocalTrack(lt, folder)
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

	return lt, a.upsertLocalTrack(lt, folder)
}

// ScanLocalFolderStream recursively scans a directory for audio files,
// reading embedded tags and persisting each track to the local SQLite DB.
// Instead of returning the full list, progress is streamed to the frontend
// via Wails events so the UI can show "42 / 200 songs scanned":
//
//	"local:scan:start"    → { folder string, total int }
//	"local:track:scanned" → { folder string, done int, total int, track LocalTrack }
//	"local:scan:done"     → { folder string, count int }
//
// The method itself returns nil on success or an error.
func (a *App) ScanLocalFolderStream(dir string) error {
	// count audio files so we know the total
	var total int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if audioExts[strings.ToLower(filepath.Ext(path))] {
			total++
		}
		return nil
	})

	wailsruntime.EventsEmit(a.ctx, "local:scan:start", map[string]any{
		"folder": dir,
		"total":  total,
	})

	// read metadata, upsert to DB, emit per-file progress
	done := 0
	livePaths := make(map[string]struct{}, total)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !audioExts[ext] {
			return nil
		}

		lt := LocalTrack{Path: path, Title: filepath.Base(path)}

		livePaths[path] = struct{}{}

		f, err := os.Open(path)
		if err != nil {
			done++
			_ = a.upsertLocalTrack(lt, dir)
			wailsruntime.EventsEmit(a.ctx, "local:track:scanned", map[string]any{
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

		probeLocalDuration(path, info, &lt)
		if lt.DurationMs == 0 {
			parseDurationFallbackLocal(path, info, &lt)
		}

		_ = a.upsertLocalTrack(lt, dir)

		done++
		wailsruntime.EventsEmit(a.ctx, "local:track:scanned", map[string]any{
			"folder": dir, "done": done, "total": total, "track": lt,
		})
		return nil
	})

	// prune DB entries for files that no longer exist on disk
	if stalePaths, pruneErr := a.pruneStaleLocalTracks(dir, livePaths); pruneErr != nil {
		slog.Warn("scan: failed to prune stale tracks", "folder", dir, "err", pruneErr)
	} else if len(stalePaths) > 0 {
		wailsruntime.EventsEmit(a.ctx, "local:track:removed", map[string]any{
			"paths": stalePaths,
		})
	}

	wailsruntime.EventsEmit(a.ctx, "local:scan:done", map[string]any{
		"folder": dir,
		"count":  done,
	})
	return err
}

// GetLocalTracks returns all cached tracks for the given folders from the
// local SQLite DB.
func (a *App) GetLocalTracks(folders []string) ([]LocalTrack, error) {
	return a.getLocalTracks(folders)
}

// GetLocalTracksPage returns a paginated slice of cached tracks for the given folders.
func (a *App) GetLocalTracksPage(folders []string, offset, limit int) ([]LocalTrack, int, error) {
	return a.getLocalTracksPage(folders, offset, limit)
}

// SearchLocalTracks performs a case-insensitive search across title, artist,
// album, and path columns of local_tracks, returning at most 50 results.
func (a *App) SearchLocalTracks(folders []string, query string) ([]LocalTrack, error) {
	return a.searchLocalTracks(folders, query)
}

// GetLocalTracksByPaths returns tracks for the given exact paths.
func (a *App) GetLocalTracksByPaths(paths []string) ([]LocalTrack, error) {
	return a.getLocalTracksByPaths(paths)
}

// GetLocalAlbumGroups returns paginated album groups computed via SQL GROUP BY.
// filter is an optional case-insensitive substring match on album name or artist.
func (a *App) GetLocalAlbumGroups(folders []string, filter string, offset, limit int) (*LocalAlbumGroupsResult, error) {
	return a.getLocalAlbumGroups(folders, filter, offset, limit)
}

// GetLocalAlbumTracks returns the tracks for a specific album by its group key,
// ordered by disc/track number.
func (a *App) GetLocalAlbumTracks(folders []string, albumName, albumArtist string) ([]LocalTrack, error) {
	return a.getLocalAlbumTracks(folders, albumName, albumArtist)
}

// ClearLocalFolder removes all cached tracks for a folder from the local DB.
func (a *App) ClearLocalFolder(folder string) error {
	return a.deleteLocalTracksByFolder(folder)
}

// ChooseLocalFolder opens a directory picker and returns only the chosen path.
// NOTE: Does not perform any scans!
// The frontend stores the path and calls ScanLocalFolderStream separately.
func (a *App) ChooseLocalFolder() (string, error) {
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Add Local Music Folder",
	})

	if err != nil {
		return "", err
	}
	return dir, nil
}

// OpenLocalFiles opens a native file dialog for selecting audio files.
func (a *App) OpenLocalFiles() ([]string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Audio Files",
		Filters: []runtime.FileFilter{
			// TODO: define single source of truth for audio file extensions
			{DisplayName: "Audio Files", Pattern: "*.mp3;*.flac;*.ogg;*.opus;*.m4a;*.aac;*.wav;*.aiff"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}

	return []string{path}, nil
}

// OpenLocalFolder opens a native directory dialog and returns all audio files found.
func (a *App) OpenLocalFolder() ([]string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Music Folder",
	})
	if err != nil {
		return nil, err
	}

	if dir == "" {
		return nil, nil
	}

	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if audioExts[ext] {
			files = append(files, path)
		}
		return nil
	})
	return files, nil
}
