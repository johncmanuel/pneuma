CREATE TABLE recent_albums (
    key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    artist TEXT NOT NULL,
    is_local INTEGER NOT NULL DEFAULT 0,
    first_track_id TEXT,
    first_local_path TEXT,
    played_at INTEGER NOT NULL
);

CREATE INDEX idx_recent_albums_played_at ON recent_albums(played_at DESC);

CREATE TABLE recent_playlists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    artwork_path TEXT,
    played_at INTEGER NOT NULL
);

CREATE INDEX idx_recent_playlists_played_at ON recent_playlists(played_at DESC);
