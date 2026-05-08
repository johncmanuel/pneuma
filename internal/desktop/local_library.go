package desktop

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"pneuma/internal/media"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ScanLocalFolderStream delegates to the Scanner, which handles file traversal,
// tag parsing, DB upserts, and Wails event emission.
func (a *App) ScanLocalFolderStream(dir string) error {
	return a.scanner.ScanFolderStream(dir)
}

// GetLocalTracks returns all cached tracks for the given folders from the local SQLite DB.
func (a *App) GetLocalTracks(folders []string) ([]LocalTrack, error) {
	return a.store.getLocalTracks(folders)
}

// GetLocalTracksPage returns a paginated slice of cached tracks for the given folders.
func (a *App) GetLocalTracksPage(folders []string, offset, limit int) ([]LocalTrack, int, error) {
	return a.store.getLocalTracksPage(folders, offset, limit)
}

// SearchLocalTracks performs a case-insensitive search across title, artist,
// album, and path columns of local_tracks, returning at most 50 results.
func (a *App) SearchLocalTracks(folders []string, query string) ([]LocalTrack, error) {
	return a.store.searchLocalTracks(folders, query)
}

// GetLocalTracksByPaths returns tracks for the given exact paths.
func (a *App) GetLocalTracksByPaths(paths []string) ([]LocalTrack, error) {
	return a.store.getLocalTracksByPaths(paths)
}

// GetLocalAlbumGroups returns paginated album groups computed via SQL GROUP BY.
// filter is an optional case-insensitive substring match on album name or artist.
func (a *App) GetLocalAlbumGroups(folders []string, filter string, offset, limit int) (*LocalAlbumGroupsResult, error) {
	return a.store.getLocalAlbumGroups(folders, filter, offset, limit)
}

// GetLocalAlbumTracks returns the tracks for a specific album by its group key,
// ordered by disc/track number.
func (a *App) GetLocalAlbumTracks(folders []string, albumName, albumArtist string) ([]LocalTrack, error) {
	return a.store.getLocalAlbumTracks(folders, albumName, albumArtist)
}

// ClearLocalFolder removes all cached tracks for a folder from the local DB.
func (a *App) ClearLocalFolder(folder string) error {
	return a.store.deleteLocalTracksByFolder(folder)
}

// GetRecentAlbums returns all recently played albums.
func (a *App) GetRecentAlbums() []RecentAlbum {
	if a.store == nil {
		return nil
	}
	return a.store.GetRecentAlbums()
}

// SetRecentAlbum upserts a recently played album.
func (a *App) SetRecentAlbum(album RecentAlbum) error {
	if a.store == nil {
		return nil
	}
	return a.store.SetRecentAlbum(album)
}

// GetRecentPlaylists returns all recently played playlists.
func (a *App) GetRecentPlaylists() []RecentPlaylist {
	if a.store == nil {
		return nil
	}
	return a.store.GetRecentPlaylists()
}

// SetRecentPlaylist upserts a recently played playlist.
func (a *App) SetRecentPlaylist(playlist RecentPlaylist) error {
	if a.store == nil {
		return nil
	}
	return a.store.SetRecentPlaylist(playlist)
}

// ClearAllRecent deletes all recently played albums and playlists.
func (a *App) ClearAllRecent() error {
	if a.store == nil {
		return nil
	}
	return a.store.ClearAllRecent()
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
			{DisplayName: "Audio Files", Pattern: media.DesktopFilterPattern()},
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
		if media.IsSupportedAudio(strings.ToLower(filepath.Ext(path))) {
			files = append(files, path)
		}
		return nil
	})
	return files, nil
}

// ResolvePlaylistItems attempts to match local_ref items to actual local files.
func (a *App) ResolvePlaylistItems(playlistID string) ([]LocalPlaylistItem, error) {
	items, err := a.GetLocalPlaylistItems(playlistID)
	if err != nil {
		return nil, err
	}

	if a.store == nil {
		return items, nil
	}

	allTracks, err := a.store.queries().ListAllLocalTracks(a.ctx)
	if err != nil {
		slog.Warn("ResolvePlaylistItems: failed to list tracks", "err", err)
		return items, nil
	}

	return resolvePlaylistItems(items, allTracks), nil
}
