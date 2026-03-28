CREATE TABLE IF NOT EXISTS playback_session (
    id TEXT PRIMARY KEY DEFAULT 'current',
    track_id TEXT,
    position_ms INTEGER DEFAULT 0,
    queue_json TEXT DEFAULT '[]',
    queue_index INTEGER DEFAULT 0,
    repeat_mode INTEGER DEFAULT 0,
    shuffle INTEGER DEFAULT 0,
    playing INTEGER DEFAULT 0,
    updated_at TEXT NOT NULL
);
