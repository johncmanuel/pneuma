-- Recreate playback_sessions to ensure user_id is properly declared UNIQUE, not device_id, which
-- was the case in the original schema.

CREATE TABLE IF NOT EXISTS playback_sessions_clean (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    track_id TEXT,
    position_ms INTEGER DEFAULT 0,
    queue_json TEXT DEFAULT '[]',
    updated_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- Copy existing session data.
-- Handle the case where multiple device sessions existed for a single user in the
-- legacy schema. Keep the most recently updated session per user.
INSERT INTO playback_sessions_clean (
    id, user_id, track_id, position_ms, queue_json, updated_at
)
SELECT
    id, user_id, track_id, position_ms, queue_json, updated_at
FROM (
    SELECT
        id, user_id, track_id, position_ms, queue_json, updated_at,
        ROW_NUMBER() OVER(PARTITION BY user_id ORDER BY updated_at DESC) as rnk
    FROM playback_sessions
)
WHERE rnk = 1;

DROP TABLE playback_sessions;
ALTER TABLE playback_sessions_clean RENAME TO playback_sessions;

CREATE INDEX IF NOT EXISTS idx_sessions_user ON playback_sessions (user_id);
