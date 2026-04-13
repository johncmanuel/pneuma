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
    COALESCE(position_ms, 0) AS position_ms,
    queue_json,
    COALESCE(queue_index, 0) AS queue_index,
    COALESCE(repeat_mode, 0) AS repeat_mode,
    CAST(COALESCE(shuffle, 0) AS BOOLEAN) AS shuffle,
    CAST(COALESCE(playing, 0) AS BOOLEAN) AS playing,
    updated_at
FROM playback_session WHERE id = 'current' LIMIT 1;
