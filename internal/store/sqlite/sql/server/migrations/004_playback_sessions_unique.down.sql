-- Rollback 004: Restore legacy playback_sessions table requiring device_id.
-- If migration didn't work out for some reason, bring back device_id and use a random UUID.
-- for placeholder

CREATE TABLE IF NOT EXISTS playback_sessions_legacy (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    track_id TEXT,
    position_ms INTEGER DEFAULT 0,
    queue_json TEXT DEFAULT '[]',
    updated_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Copy session data and generate a new UUID for device_id since it was lost in the up migration.
INSERT INTO playback_sessions_legacy (
    id, device_id, user_id, track_id, position_ms, queue_json, updated_at
)
SELECT
    id,
    lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-a' || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))),
    user_id, track_id, position_ms, queue_json, updated_at
FROM playback_sessions;

DROP TABLE playback_sessions;
ALTER TABLE playback_sessions_legacy RENAME TO playback_sessions;

CREATE INDEX IF NOT EXISTS idx_sessions_device ON playback_sessions (device_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON playback_sessions (user_id);
