package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database connection.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite database at path and applies the schema.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open %s: %w", path, err)
	}

	// Performance pragmas.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-32000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("pragma: %w", err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB (for testing).
func (s *Store) DB() *sql.DB {
	return s.db
}

func migrate(db *sql.DB) error {
	// Apply base schema (idempotent CREATE TABLE IF NOT EXISTS).
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// Run incremental numbered migrations.
	return runMigrations(db)
}

// runMigrations applies migrations sequentially based on a schema_version table.
func runMigrations(db *sql.DB) error {
	// Ensure the version table exists.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var current int
	err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current)
	if err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migration %d begin: %w", m.version, err)
		}
		for _, stmt := range m.stmts {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("migration %d exec: %w", m.version, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d version: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d commit: %w", m.version, err)
		}
	}
	return nil
}

type migration struct {
	version int
	stmts   []string
}

// migrations is the ordered list of incremental schema changes.
var migrations = []migration{
	{
		version: 1,
		stmts: []string{
			`ALTER TABLE users ADD COLUMN can_upload INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE users ADD COLUMN can_edit INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE users ADD COLUMN can_delete INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE tracks ADD COLUMN uploaded_by_user_id TEXT DEFAULT '' REFERENCES users(id)`,
			`ALTER TABLE tracks ADD COLUMN deleted_at TEXT`,
			`CREATE TABLE IF NOT EXISTS audit_log (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				action TEXT NOT NULL,
				target_type TEXT NOT NULL,
				target_id TEXT NOT NULL,
				detail TEXT DEFAULT '',
				created_at TEXT NOT NULL,
				FOREIGN KEY (user_id) REFERENCES users(id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_target ON audit_log(target_type, target_id)`,
		},
	},
	{
		version: 2,
		stmts: []string{
			`ALTER TABLE tracks ADD COLUMN acoustic_fingerprint TEXT NOT NULL DEFAULT ''`,
			`CREATE INDEX IF NOT EXISTS idx_tracks_acoustic_fp ON tracks(acoustic_fingerprint)`,
		},
	},
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	is_admin INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS artists (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	mb_artist_id TEXT DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS artworks (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	width INTEGER DEFAULT 0,
	height INTEGER DEFAULT 0,
	format TEXT DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS albums (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	artist_id TEXT,
	year INTEGER DEFAULT 0,
	mb_release_id TEXT DEFAULT '',
	artwork_id TEXT,
	created_at TEXT NOT NULL,
	FOREIGN KEY (artist_id) REFERENCES artists(id),
	FOREIGN KEY (artwork_id) REFERENCES artworks(id)
);

CREATE TABLE IF NOT EXISTS tracks (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL DEFAULT '',
	artist_id TEXT,
	album_id TEXT,
	album_artist TEXT DEFAULT '',
	album_name TEXT DEFAULT '',
	genre TEXT DEFAULT '',
	year INTEGER DEFAULT 0,
	track_number INTEGER DEFAULT 0,
	disc_number INTEGER DEFAULT 0,
	duration_ms INTEGER DEFAULT 0,
	bitrate_kbps INTEGER DEFAULT 0,
	sample_rate_hz INTEGER DEFAULT 0,
	codec TEXT DEFAULT '',
	file_size_bytes INTEGER DEFAULT 0,
	last_modified TEXT NOT NULL,
	fingerprint TEXT DEFAULT '',
	mb_recording_id TEXT DEFAULT '',
	replay_gain_track REAL DEFAULT 0,
	replay_gain_album REAL DEFAULT 0,
	artwork_id TEXT,
	enriched_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (artist_id) REFERENCES artists(id),
	FOREIGN KEY (album_id) REFERENCES albums(id),
	FOREIGN KEY (artwork_id) REFERENCES artworks(id)
);

CREATE TABLE IF NOT EXISTS playlists (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS playlist_tracks (
	playlist_id TEXT NOT NULL,
	track_id TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (playlist_id, track_id),
	FOREIGN KEY (playlist_id) REFERENCES playlists(id),
	FOREIGN KEY (track_id) REFERENCES tracks(id)
);

CREATE TABLE IF NOT EXISTS watch_folders (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	user_id TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS devices (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL,
	last_seen_at TEXT,
	created_at TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS playback_sessions (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL UNIQUE,
	user_id TEXT NOT NULL,
	track_id TEXT,
	position_ms INTEGER DEFAULT 0,
	queue_json TEXT DEFAULT '[]',
	updated_at TEXT NOT NULL,
	FOREIGN KEY (device_id) REFERENCES devices(id),
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS offline_packs (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	track_id TEXT NOT NULL,
	local_path TEXT NOT NULL,
	downloaded_at TEXT NOT NULL,
	UNIQUE(user_id, track_id),
	FOREIGN KEY (user_id) REFERENCES users(id),
	FOREIGN KEY (track_id) REFERENCES tracks(id)
);

CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist_id);
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album_id);
CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks(path);
CREATE INDEX IF NOT EXISTS idx_albums_artist ON albums(artist_id);
CREATE INDEX IF NOT EXISTS idx_devices_user ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_device ON playback_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON playback_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_offline_user ON offline_packs(user_id);
`
