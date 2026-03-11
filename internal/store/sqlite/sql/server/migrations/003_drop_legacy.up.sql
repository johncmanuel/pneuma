-- Remove legacy normalisation tables and track columns that are no longer used.
-- Uses IF EXISTS for all DROP operations so this migration is safe to run on
-- databases created from the clean 001 baseline (where these objects don't exist).

-- Drop legacy indexes
DROP INDEX IF EXISTS idx_tracks_artist;
DROP INDEX IF EXISTS idx_tracks_album;
DROP INDEX IF EXISTS idx_tracks_acoustic_fp;
DROP INDEX IF EXISTS idx_albums_artist;
DROP INDEX IF EXISTS idx_offline_user;

-- Drop legacy tables
DROP TABLE IF EXISTS offline_packs;
DROP TABLE IF EXISTS albums;
DROP TABLE IF EXISTS artworks;
DROP TABLE IF EXISTS artists;

-- Recreate tracks without the legacy columns.
-- Using table-recreation (the SQLite-recommended approach for removing columns)
-- so this migration works whether the legacy columns are present or not.
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
    id, path, title,
    COALESCE(album_artist, ''), COALESCE(album_name, ''), COALESCE(genre, ''),
    COALESCE(year, 0), COALESCE(track_number, 0), COALESCE(disc_number, 0),
    COALESCE(duration_ms, 0), COALESCE(bitrate_kbps, 0), COALESCE(sample_rate_hz, 0),
    COALESCE(codec, ''), COALESCE(file_size_bytes, 0), last_modified,
    COALESCE(fingerprint, ''), COALESCE(replay_gain_track, 0), COALESCE(replay_gain_album, 0),
    COALESCE(uploaded_by_user_id, ''), deleted_at, created_at, updated_at
FROM tracks;

DROP TABLE tracks;
ALTER TABLE tracks_clean RENAME TO tracks;

-- Recreate the path index (other legacy indexes were dropped above)
CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks (path);
