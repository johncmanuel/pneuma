package desktop

import (
	"context"
	"database/sql"
	"fmt"

	"pneuma/internal/store/sqlite/desktopdb"
)

// LocalStore defines the persistent storage operations for the desktop app's
// local library: tracks, album groups, playlists, and recently played items.
type LocalStore interface {
	// Track operations

	upsertLocalTrack(lt LocalTrack, folder string) error
	deleteLocalTracksByFolder(folder string) error
	deleteLocalTrackByPath(path string) error
	pruneStaleLocalTracks(folder string, livePaths map[string]struct{}) ([]string, error)
	deleteLocalTracksByPathPrefix(prefix string) (int64, error)
	getLocalTracks(folders []string) ([]LocalTrack, error)
	getLocalTracksPage(folders []string, offset, limit int) ([]LocalTrack, int, error)
	searchLocalTracks(folders []string, query string) ([]LocalTrack, error)
	getLocalTracksByPaths(paths []string) ([]LocalTrack, error)
	getLocalAlbumGroups(folders []string, filter string, offset, limit int) (*LocalAlbumGroupsResult, error)
	getLocalAlbumTracks(folders []string, albumName, albumArtist string) ([]LocalTrack, error)

	// Recent items

	GetRecentAlbums() []RecentAlbum
	SetRecentAlbum(album RecentAlbum) error
	GetRecentPlaylists() []RecentPlaylist
	SetRecentPlaylist(playlist RecentPlaylist) error
	ClearAllRecent() error

	// Playlist operations delegated via App methods in db_playlists.go
	queries() *desktopdb.Queries
}

// AppStore is the concrete LocalStore backed by a SQLite database via sqlc.
type AppStore struct {
	db *sql.DB
	dq *desktopdb.Queries
}

// NewAppStore creates an AppStore from an open *sql.DB.
func NewAppStore(db *sql.DB) *AppStore {
	return &AppStore{
		db: db,
		dq: desktopdb.New(db),
	}
}

// queries returns the underlying sqlc queries that haven't been migrated to AppStore yet.
func (s *AppStore) queries() *desktopdb.Queries {
	return s.dq
}

// closeDB closes the underlying database connection.
func (s *AppStore) closeDB() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// checkDB returns an error if the store has not been initialised.
func (s *AppStore) checkDB() error {
	if s.dq == nil {
		return fmt.Errorf("appDB not initialised")
	}
	return nil
}

// checkDBCtx is checkDB paired with a ready-to-use background context.
func (s *AppStore) checkDBCtx() (context.Context, error) {
	if err := s.checkDB(); err != nil {
		return nil, err
	}
	return context.Background(), nil
}
