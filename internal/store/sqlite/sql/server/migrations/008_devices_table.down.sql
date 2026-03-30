-- Rollback 008: Drop devices table and restore playback_sessions relying on user_id

CREATE TABLE IF NOT EXISTS playback_sessions_legacy (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    track_id TEXT,
    position_ms INTEGER DEFAULT 0,
    queue_json TEXT DEFAULT '[]',
    queue_index INTEGER DEFAULT 0,
    repeat_mode INTEGER DEFAULT 0,
    shuffle INTEGER DEFAULT 0,
    playing INTEGER DEFAULT 0,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Note: playback sessions are cleared upon rollback as well.
DROP TABLE playback_sessions;
ALTER TABLE playback_sessions_legacy RENAME TO playback_sessions;

CREATE INDEX IF NOT EXISTS idx_sessions_user ON playback_sessions (user_id);

DROP INDEX IF EXISTS idx_devices_user;
DROP TABLE IF EXISTS devices;
