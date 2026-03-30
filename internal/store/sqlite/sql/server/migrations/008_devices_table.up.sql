CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_active TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE INDEX IF NOT EXISTS idx_devices_user ON devices (user_id);

CREATE TABLE IF NOT EXISTS playback_sessions_clean (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    track_id TEXT,
    position_ms INTEGER DEFAULT 0,
    queue_json TEXT DEFAULT '[]',
    queue_index INTEGER DEFAULT 0,
    repeat_mode INTEGER DEFAULT 0,
    shuffle INTEGER DEFAULT 0,
    playing INTEGER DEFAULT 0,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

DROP TABLE playback_sessions;
ALTER TABLE playback_sessions_clean RENAME TO playback_sessions;

CREATE INDEX IF NOT EXISTS idx_sessions_device ON playback_sessions (device_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON playback_sessions (user_id);
