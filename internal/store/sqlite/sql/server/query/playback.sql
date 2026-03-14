-- name: UpsertPlaybackSession :exec
INSERT INTO playback_sessions (
    id, user_id, track_id, position_ms, queue_json, updated_at
)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET
    track_id = excluded.track_id, position_ms = excluded.position_ms,
    queue_json = excluded.queue_json, updated_at = excluded.updated_at;

-- name: PlaybackSessionByUser :one
SELECT
    id,
    user_id,
    COALESCE(track_id, '') AS track_id,
    position_ms,
    queue_json,
    updated_at
FROM playback_sessions
WHERE user_id = ? LIMIT 1;
