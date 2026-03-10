package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"pneuma/internal/store/sqlite"
	"pneuma/internal/store/sqlite/desktopdb"

	_ "modernc.org/sqlite"
)

// openAppDB opens (or creates) the app-local SQLite database used to persist
// desktop client state: local folder list, track cache, recent albums, etc.
// The database is stored in the OS user-cache directory so it survives app
// updates but is clearly separate from user documents.
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

	// want to keep connections low to optimize for memory usage, but if general performance bottlenecks,
	// look here first and modify as needed.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Memory-conscious SQLite settings:
	//   WAL mode   – readers don't block the writer; safer than DELETE journal.
	//   cache_size – cap the page cache to ~2 MB (default is -2000 KiB pages).
	//   synchronous NORMAL – safe for WAL; skips the extra fsync on each commit.
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA cache_size=-2000`,
		`PRAGMA synchronous=NORMAL`,
	} {
		if _, err = db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("appdb pragma (%s): %w", pragma, err)
		}
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

	// Create/ensure all tables and indexes from the canonical schema.
	if _, err = db.Exec(sqlite.DesktopSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply desktop schema: %w", err)
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
		a.dq = nil
	}
}

// AppDBGet returns the stored value for key, or "" if the key does not exist.
func (a *App) AppDBGet(key string) string {
	if a.dq == nil {
		return ""
	}
	val, err := a.dq.GetKV(context.Background(), key)
	if err != nil {
		return ""
	}
	return val
}

// AppDBSet stores or replaces value for key (upsert).
func (a *App) AppDBSet(key, value string) error {
	if a.dq == nil {
		return fmt.Errorf("appDB not initialised")
	}
	return a.dq.SetKV(context.Background(), desktopdb.SetKVParams{Key: key, Value: value})
}

// AppDBDelete removes key from the store. It is a no-op when the key does not exist.
func (a *App) AppDBDelete(key string) error {
	if a.dq == nil {
		return nil
	}
	return a.dq.DeleteKV(context.Background(), key)
}
