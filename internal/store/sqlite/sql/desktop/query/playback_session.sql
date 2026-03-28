-- name: UpsertPlaybackSession :exec
INSERT INTO playback_session (
    track_id, position_ms, queue_json, queue_index, repeat_mode, shuffle, playing, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    track_id = excluded.track_id, position_ms = excluded.position_ms,
    queue_json = excluded.queue_json, queue_index = excluded.queue_index,
    repeat_mode = excluded.repeat_mode, shuffle = excluded.shuffle,
    playing = excluded.playing, updated_at = excluded.updated_at;

-- name: GetPlaybackSession :one
SELECT
    COALESCE(track_id, '') AS track_id,
    position_ms, queue_json, queue_index, repeat_mode, shuffle, playing, updated_at
FROM playback_session WHERE id = 'current' LIMIT 1;
