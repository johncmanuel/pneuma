-- All of the main tables

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    can_upload INTEGER NOT NULL DEFAULT 0,
    can_edit INTEGER NOT NULL DEFAULT 0,
    can_delete INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tracks (
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

CREATE TABLE IF NOT EXISTS playlists (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    artwork_path TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE TABLE IF NOT EXISTS playlist_items (
    playlist_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    -- 'remote' or 'local_ref', which is a remote track and local track respectively
    source TEXT NOT NULL DEFAULT 'remote',  
    -- server track UUID (for remote tracks only)
    track_id TEXT,                          
    -- display/matching metadata (for local tracks only)
    ref_title TEXT NOT NULL DEFAULT '',       
    ref_album TEXT NOT NULL DEFAULT '',
    ref_album_artist TEXT NOT NULL DEFAULT '',
    ref_duration_ms INTEGER NOT NULL DEFAULT 0,
    added_at TEXT NOT NULL,
    PRIMARY KEY (playlist_id, position),
    FOREIGN KEY (playlist_id) REFERENCES playlists (id) ON DELETE CASCADE,
    FOREIGN KEY (track_id) REFERENCES tracks (id)
);

CREATE TABLE IF NOT EXISTS watch_folders (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE TABLE IF NOT EXISTS playback_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    track_id TEXT,
    position_ms INTEGER DEFAULT 0,
    queue_json TEXT DEFAULT '[]',
    updated_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    detail TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks (path);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON playback_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_target ON audit_log (target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_playlists_user ON playlists (user_id);
CREATE INDEX IF NOT EXISTS idx_playlist_items_playlist ON playlist_items (playlist_id);
