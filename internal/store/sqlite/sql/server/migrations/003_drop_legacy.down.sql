-- Reverse migration 003: restore legacy tables and track columns.

-- Restore columns on tracks
ALTER TABLE tracks ADD COLUMN artist_id TEXT;
ALTER TABLE tracks ADD COLUMN album_id TEXT;
ALTER TABLE tracks ADD COLUMN acoustic_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN mb_recording_id TEXT DEFAULT '';
ALTER TABLE tracks ADD COLUMN artwork_id TEXT;
ALTER TABLE tracks ADD COLUMN enriched_at TEXT;

-- Recreate legacy tables
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
    FOREIGN KEY (artist_id) REFERENCES artists (id),
    FOREIGN KEY (artwork_id) REFERENCES artworks (id)
);

CREATE TABLE IF NOT EXISTS offline_packs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    track_id TEXT NOT NULL,
    local_path TEXT NOT NULL,
    downloaded_at TEXT NOT NULL,
    UNIQUE (user_id, track_id),
    FOREIGN KEY (user_id) REFERENCES users (id),
    FOREIGN KEY (track_id) REFERENCES tracks (id)
);

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks (artist_id);
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks (album_id);
CREATE INDEX IF NOT EXISTS idx_tracks_acoustic_fp ON tracks (acoustic_fingerprint);
CREATE INDEX IF NOT EXISTS idx_albums_artist ON albums (artist_id);
CREATE INDEX IF NOT EXISTS idx_offline_user ON offline_packs (user_id);
