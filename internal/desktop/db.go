package desktop

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// openAppDB opens (or creates) the app-local SQLite database used to persist
// desktop client state: local folder list, track cache, recent albums, etc.
// The database is stored in the OS user-cache directory so it survives app
// updates but is clearly separate from user documents.
//
// Linux:   ~/.cache/pneuma/app.db
// macOS:   ~/Library/Caches/pneuma/app.db
// Windows: %LocalAppData%\pneuma\app.db
func openAppDB() (*sql.DB, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "pneuma")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir appdb: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "app.db"))
	if err != nil {
		return nil, err
	}

	// Generic key-value table (settings, small blobs, etc.).
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create kv table: %w", err)
	}

	// Migration: drop the old local_tracks table if it still has the
	// now-removed fingerprint / acoustic_fingerprint columns.  The table
	// is a pure scan cache so data loss is safe — next scan repopulates it.
	var hasOldSchema bool
	if err := db.QueryRow(
		`SELECT count(*) > 0 FROM pragma_table_info('local_tracks') WHERE name='fingerprint'`,
	).Scan(&hasOldSchema); err == nil && hasOldSchema {
		slog.Info("appdb: removing obsolete fingerprint columns — table will be rebuilt on next scan")
		_, _ = db.Exec(`DROP TABLE IF EXISTS local_tracks`)
		_, _ = db.Exec(`DELETE FROM kv WHERE key = 'local_dupes_cache'`)
	}
	// Also drop the old fingerprint indexes if they somehow survived.
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_lt_fp`)
	_, _ = db.Exec(`DROP INDEX IF EXISTS idx_lt_afp`)

	// Relational table for locally-scanned tracks (metadata only, no hashes).
	const createLocalTracks = `
CREATE TABLE IF NOT EXISTS local_tracks (
path         TEXT PRIMARY KEY,
folder       TEXT NOT NULL,
title        TEXT NOT NULL DEFAULT '',
artist       TEXT NOT NULL DEFAULT '',
album        TEXT NOT NULL DEFAULT '',
album_artist TEXT NOT NULL DEFAULT '',
genre        TEXT NOT NULL DEFAULT '',
year         INTEGER NOT NULL DEFAULT 0,
track_number INTEGER NOT NULL DEFAULT 0,
disc_number  INTEGER NOT NULL DEFAULT 0,
duration_ms  INTEGER NOT NULL DEFAULT 0,
has_artwork  INTEGER NOT NULL DEFAULT 0
)`
	if _, err = db.Exec(createLocalTracks); err != nil {
		db.Close()
		return nil, fmt.Errorf("create local_tracks table: %w", err)
	}

	// Index for fast per-folder deletion and loading.
	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_lt_folder ON local_tracks(folder)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create index: %w", err)
	}

	// One-time migration: clean up old KV-blob track cache if present.
	_, _ = db.Exec(`DELETE FROM kv WHERE key = 'local_tracks_cache'`)

	return db, nil
}

// closeAppDB is called from Shutdown.
func (a *App) closeAppDB() {
	if a.appDB != nil {
		if err := a.appDB.Close(); err != nil {
			slog.Warn("appDB close error", "err", err)
		}
		a.appDB = nil
	}
}

// AppDBGet returns the stored value for key, or "" if the key does not exist.
func (a *App) AppDBGet(key string) string {
	if a.appDB == nil {
		return ""
	}
	var val string
	if err := a.appDB.QueryRow(`SELECT value FROM kv WHERE key=?`, key).Scan(&val); err != nil {
		return ""
	}
	return val
}

// AppDBSet stores or replaces value for key (upsert).
func (a *App) AppDBSet(key, value string) error {
	if a.appDB == nil {
		return fmt.Errorf("appDB not initialised")
	}
	_, err := a.appDB.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}

// AppDBDelete removes key from the store. It is a no-op when the key does not exist.
func (a *App) AppDBDelete(key string) error {
	if a.appDB == nil {
		return nil
	}
	_, err := a.appDB.Exec(`DELETE FROM kv WHERE key=?`, key)
	return err
}
