-- Desktop app-local schema (cache DB).

CREATE TABLE IF NOT EXISTS kv (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

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
);

CREATE INDEX IF NOT EXISTS idx_lt_folder ON local_tracks(folder);

-- Local playlists: stored on the user's desktop, can optionally link to a
-- remote (server) playlist via remote_playlist_id for sync.
CREATE TABLE IF NOT EXISTS local_playlists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    artwork_path TEXT NOT NULL DEFAULT '',
    remote_playlist_id TEXT NOT NULL DEFAULT '',  -- linked server playlist ID (empty = local-only)
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_playlist_items (
    playlist_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'remote',  -- 'remote' or 'local_ref'
    track_id TEXT NOT NULL DEFAULT '',       -- server track UUID (for remote)
    local_path TEXT NOT NULL DEFAULT '',     -- local filesystem path (for local_ref, never uploaded)
    ref_title TEXT NOT NULL DEFAULT '',
    ref_album TEXT NOT NULL DEFAULT '',
    ref_album_artist TEXT NOT NULL DEFAULT '',
    ref_duration_ms INTEGER NOT NULL DEFAULT 0,
    added_at TEXT NOT NULL,
    PRIMARY KEY (playlist_id, position),
    FOREIGN KEY (playlist_id) REFERENCES local_playlists (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_lp_remote ON local_playlists(remote_playlist_id);
CREATE INDEX IF NOT EXISTS idx_lpi_playlist ON local_playlist_items(playlist_id);
