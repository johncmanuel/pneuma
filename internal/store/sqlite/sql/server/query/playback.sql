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

-- name: UpsertOfflinePack :exec
INSERT INTO offline_packs (id, user_id, track_id, local_path, downloaded_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (user_id, track_id) DO UPDATE SET
    local_path = excluded.local_path, downloaded_at = excluded.downloaded_at;

-- name: DeleteOfflinePack :exec
DELETE FROM offline_packs
WHERE user_id = ? AND track_id = ?;

-- name: ListOfflinePacks :many
SELECT
    id,
    user_id,
    track_id,
    local_path,
    downloaded_at
FROM offline_packs
WHERE user_id = ?
ORDER BY downloaded_at DESC;
