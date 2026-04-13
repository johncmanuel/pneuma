package desktop

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"pneuma/internal/store/sqlite"
	"pneuma/internal/store/sqlite/dbconv"
	"pneuma/internal/store/sqlite/desktopdb"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "modernc.org/sqlite"
)

// pretty similar logic to store.go, but probably won't take time to dedupe the
// logic atm

const (
	DesktopProdAppDirName = "pneuma"
	DesktopDevAppDirName  = "pneuma-dev"
	DesktopDBName         = "app.db"
	DesktopMigrationsDir  = "sql/desktop/migrations"
)

// openAppDBRaw opens the SQLite database at path, creates the directory if needed,
// and applies the standard connection pragmas via DSN.
func openAppDBRaw(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir appdb: %w", err)
	}

	pragmas := []string{
		"_pragma=journal_mode(WAL)",   // ensure concurrent reads/writes
		"_pragma=cache_size(-2000)",   // cap the page cache to ~2 MB in memory
		"_pragma=synchronous(NORMAL)", // use fewer fsyncs for better performance
		// no need for storing temp tables in memory since desktop app is local
	}

	dsn := fmt.Sprintf(
		"file:%s?%s",
		path, strings.Join(pragmas, "&"),
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open %s: %w", path, err)
	}

	// want to keep connections low to optimize for memory usage, but if general performance bottlenecks,
	// look here first and modify as needed.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return db, nil
}

// newAppDBMigrator builds a *migrate.Migrate instance backed by the embedded
// migration files and the provided DB connection. The caller is responsible
// for calling m.Close() when done.
func newAppDBMigrator(db *sql.DB) (*migrate.Migrate, error) {
	sourceDriver, migrErr := iofs.New(sqlite.DesktopMigrations, DesktopMigrationsDir)
	if migrErr != nil {
		return nil, fmt.Errorf("desktop migration source: %w", migrErr)
	}

	dbDriver, migrErr := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if migrErr != nil {
		return nil, fmt.Errorf("desktop migration db driver: %w", migrErr)
	}

	m, migrErr := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if migrErr != nil {
		return nil, fmt.Errorf("desktop migrate new: %w", migrErr)
	}

	return m, nil
}

// migrateDBFromCacheToConfig moves the desktop database from the old cache
// directory to the config directory for existing users. Returns the new path.
func migrateDBFromCacheToConfig(profile desktopProfile) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}

	newPath := filepath.Join(configDir, desktopAppDir(profile), DesktopDBName)

	if profile != desktopProfileProd {
		return newPath, nil
	}

	if _, err := os.Stat(newPath); err == nil {
		return newPath, nil
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return newPath, nil
	}

	oldPath := filepath.Join(cacheDir, DesktopProdAppDirName, DesktopDBName)

	if _, err := os.Stat(oldPath); err != nil {
		return newPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
		return "", fmt.Errorf("mkdir config dir: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("move db to config dir: %w", err)
	}

	slog.Info("migrated desktop database from cache to config dir", "from", oldPath, "to", newPath)

	return newPath, nil
}

// openAppDB opens (or creates) the app-local SQLite database used to persist
// desktop client state: local folder list, track cache, recent albums, etc.
// The database is stored in the OS user-config directory.
func openAppDB(profile desktopProfile) (*sql.DB, error) {
	dbPath, err := migrateDBFromCacheToConfig(profile)
	if err != nil {
		return nil, err
	}

	db, err := openAppDBRaw(dbPath)
	if err != nil {
		return nil, err
	}

	m, migrErr := newAppDBMigrator(db)
	if migrErr != nil {
		db.Close()
		return nil, migrErr
	}

	if migrErr = m.Up(); migrErr != nil && !errors.Is(migrErr, migrate.ErrNoChange) {
		db.Close()
		return nil, fmt.Errorf("apply desktop migrations: %w", migrErr)
	}

	slog.Info("desktop database opened and migrated", "profile", profile, "path", dbPath)

	return db, nil
}

// closeAppDB is called from Shutdown.
func (a *App) closeAppDB() {
	if a.appDB != nil {
		if err := a.appDB.Close(); err != nil {
			slog.Warn("appDB close error", "err", err)
		}
		a.appDB = nil
		a.dq = nil
	}
}

// RecentAlbum represents a recently played album.
type RecentAlbum struct {
	Key            string
	Name           string
	Artist         string
	IsLocal        bool
	FirstTrackID   string
	FirstLocalPath string
	PlayedAt       int64
}

// RecentPlaylist represents a recently played playlist.
type RecentPlaylist struct {
	ID          string
	Name        string
	ArtworkPath string
	PlayedAt    int64
}

// GetRecentAlbums returns all recently played albums, ordered by played_at DESC.
func (a *App) GetRecentAlbums() []RecentAlbum {
	if a.dq == nil {
		return nil
	}

	albums, err := a.dq.GetRecentAlbums(context.Background())
	if err != nil {
		return nil
	}

	result := make([]RecentAlbum, len(albums))
	for i, al := range albums {
		result[i] = RecentAlbum{
			Key:            al.Key,
			Name:           al.Name,
			Artist:         al.Artist,
			IsLocal:        al.IsLocal,
			FirstTrackID:   al.FirstTrackID.String,
			FirstLocalPath: al.FirstLocalPath.String,
			PlayedAt:       al.PlayedAt,
		}
	}
	return result
}

// SetRecentAlbum upserts a recently played album.
func (a *App) SetRecentAlbum(album RecentAlbum) error {
	if a.dq == nil {
		return fmt.Errorf("appDB not initialised")
	}

	return a.dq.SetRecentAlbum(context.Background(), desktopdb.SetRecentAlbumParams{
		Key:            album.Key,
		Name:           album.Name,
		Artist:         album.Artist,
		IsLocal:        album.IsLocal,
		FirstTrackID:   dbconv.NullStr(album.FirstTrackID),
		FirstLocalPath: dbconv.NullStr(album.FirstLocalPath),
		PlayedAt:       album.PlayedAt,
	})
}

// GetRecentPlaylists returns all recently played playlists, ordered by played_at DESC.
func (a *App) GetRecentPlaylists() []RecentPlaylist {
	if a.dq == nil {
		return nil
	}

	playlists, err := a.dq.GetRecentPlaylists(context.Background())
	if err != nil {
		return nil
	}

	result := make([]RecentPlaylist, len(playlists))
	for i, pl := range playlists {
		result[i] = RecentPlaylist{
			ID:          pl.ID,
			Name:        pl.Name,
			ArtworkPath: pl.ArtworkPath.String,
			PlayedAt:    pl.PlayedAt,
		}
	}
	return result
}

// SetRecentPlaylist upserts a recently played playlist.
func (a *App) SetRecentPlaylist(playlist RecentPlaylist) error {
	if a.dq == nil {
		return fmt.Errorf("appDB not initialised")
	}

	return a.dq.SetRecentPlaylist(context.Background(), desktopdb.SetRecentPlaylistParams{
		ID:          playlist.ID,
		Name:        playlist.Name,
		ArtworkPath: dbconv.NullStr(playlist.ArtworkPath),
		PlayedAt:    playlist.PlayedAt,
	})
}

// ClearAllRecent deletes all recently played albums and playlists.
func (a *App) ClearAllRecent() error {
	if a.dq == nil {
		return fmt.Errorf("appDB not initialised")
	}

	if err := a.dq.DeleteAllRecentAlbums(context.Background()); err != nil {
		return err
	}

	if err := a.dq.DeleteAllRecentPlaylists(context.Background()); err != nil {
		return err
	}

	return nil
}
