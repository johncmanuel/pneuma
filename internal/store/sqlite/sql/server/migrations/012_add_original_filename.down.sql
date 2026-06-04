-- Recreate tracks table without original_filename

CREATE TABLE IF NOT EXISTS tracks_clean (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
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
    replay_gain_track REAL DEFAULT 0,
    replay_gain_album REAL DEFAULT 0,
    uploaded_by_user_id TEXT DEFAULT '' REFERENCES users (id),
    deleted_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO tracks_clean (
    id, path, title, album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint,
    replay_gain_track, replay_gain_album, uploaded_by_user_id,
    deleted_at, created_at, updated_at
)
SELECT
    id, path, title, album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint,
    replay_gain_track, replay_gain_album, uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks;

DROP TABLE tracks;
ALTER TABLE tracks_clean RENAME TO tracks;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks (path);

CREATE INDEX IF NOT EXISTS idx_tracks_active_title 
ON tracks (title COLLATE NOCASE) 
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tracks_active_album_artist_sort 
ON tracks (album_artist COLLATE NOCASE, album_name COLLATE NOCASE, disc_number, track_number) 
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tracks_fingerprint ON tracks (fingerprint) WHERE deleted_at IS NULL;
