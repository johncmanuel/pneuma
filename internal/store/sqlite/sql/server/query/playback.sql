-- name: UpsertPlaybackSession :exec
INSERT INTO playback_sessions (
    id, device_id, user_id, track_id, position_ms, queue_json, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (device_id) DO UPDATE SET
    track_id = excluded.track_id, position_ms = excluded.position_ms,
    queue_json = excluded.queue_json, updated_at = excluded.updated_at;

-- name: PlaybackSessionByDevice :one
SELECT
    id,
    device_id,
    user_id,
    COALESCE(track_id, '') AS track_id,
    position_ms,
    queue_json,
    updated_at
FROM playback_sessions
WHERE device_id = ? LIMIT 1;

-- name: PlaybackSessionsByUser :many
SELECT
    ps.id,
    ps.device_id,
    ps.user_id,
    COALESCE(ps.track_id, '') AS track_id,
    ps.position_ms,
    ps.queue_json,
    ps.updated_at
FROM playback_sessions ps
JOIN devices d ON d.id = ps.device_id
WHERE ps.user_id = ?
ORDER BY ps.updated_at DESC;
