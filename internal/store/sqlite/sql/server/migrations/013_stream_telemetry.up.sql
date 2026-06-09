CREATE TABLE IF NOT EXISTS stream_telemetry (
    id                  TEXT PRIMARY KEY,
    track_id            TEXT NOT NULL,
    user_id             TEXT NOT NULL DEFAULT '',
    device_id           TEXT NOT NULL DEFAULT '',
    stream_quality      TEXT NOT NULL DEFAULT 'original',
    latency_ms          INTEGER NOT NULL DEFAULT 0,
    stutter_count       INTEGER NOT NULL DEFAULT 0,
    stutter_duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_stream_telemetry_track_id ON stream_telemetry(track_id);
CREATE INDEX IF NOT EXISTS idx_stream_telemetry_created_at ON stream_telemetry(created_at);
