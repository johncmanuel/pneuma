-- name: UpsertPlaybackSession :exec
INSERT INTO playback_sessions (
    id, device_id, user_id, track_id, position_ms, queue_json, queue_index, repeat_mode, shuffle, playing, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (device_id) DO UPDATE SET
    track_id = excluded.track_id, position_ms = excluded.position_ms,
    queue_json = excluded.queue_json, queue_index = excluded.queue_index,
    repeat_mode = excluded.repeat_mode, shuffle = excluded.shuffle,
    playing = excluded.playing, updated_at = excluded.updated_at;

-- name: PlaybackSessionByDevice :one
SELECT
    id,
    device_id,
    user_id,
    COALESCE(track_id, '') AS track_id,
    position_ms,
    queue_json,
    queue_index,
    repeat_mode,
    shuffle,
    playing,
    updated_at
FROM playback_sessions
WHERE device_id = ? LIMIT 1;
