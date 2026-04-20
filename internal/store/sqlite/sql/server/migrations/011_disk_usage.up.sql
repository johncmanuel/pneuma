CREATE TABLE IF NOT EXISTS disk_usage (
    id TEXT PRIMARY KEY,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    free_bytes INTEGER NOT NULL DEFAULT 0,
    tracks_bytes INTEGER NOT NULL DEFAULT 0,
    db_bytes INTEGER NOT NULL DEFAULT 0,
    transcode_cache_bytes INTEGER NOT NULL DEFAULT 0,
    artwork_cache_bytes INTEGER NOT NULL DEFAULT 0,
    playlist_art_bytes INTEGER NOT NULL DEFAULT 0,
    recorded_at TEXT NOT NULL
);
