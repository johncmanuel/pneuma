-- name: InsertStreamTelemetry :exec
INSERT INTO stream_telemetry (
    id, track_id, user_id, device_id, stream_quality,
    latency_ms, stutter_count, stutter_duration_ms, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetStreamTelemetryStats :many
SELECT
    st.track_id,
    t.title AS track_title,
    t.album_artist AS track_artist,
    t.codec AS track_codec,
    t.file_size_bytes AS track_file_size,
    t.duration_ms AS track_duration_ms,
    st.stream_quality,
    COUNT(*) AS sample_count,
    CAST(AVG(st.latency_ms) AS INTEGER) AS avg_latency_ms,
    MIN(st.latency_ms) AS min_latency_ms,
    MAX(st.latency_ms) AS max_latency_ms,
    CAST(SUM(st.stutter_count) AS INTEGER) AS total_stutters,
    CAST(AVG(st.stutter_duration_ms) AS INTEGER) AS avg_stutter_duration_ms
FROM stream_telemetry st
LEFT JOIN tracks t ON t.id = st.track_id AND t.deleted_at IS NULL
GROUP BY st.track_id, st.stream_quality
ORDER BY avg_latency_ms DESC;

-- name: ClearStreamTelemetry :exec
DELETE FROM stream_telemetry;
