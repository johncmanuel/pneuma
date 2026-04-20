-- name: InsertDiskUsage :exec
INSERT INTO disk_usage (
    id, total_bytes, free_bytes, tracks_bytes, db_bytes,
    transcode_cache_bytes, artwork_cache_bytes, playlist_art_bytes, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetLatestDiskUsage :one
SELECT
    id, total_bytes, free_bytes, tracks_bytes, db_bytes,
    transcode_cache_bytes, artwork_cache_bytes, playlist_art_bytes, recorded_at
FROM disk_usage
ORDER BY recorded_at DESC
LIMIT 1;

-- name: PruneDiskUsageHistory :exec
DELETE FROM disk_usage
WHERE id NOT IN (
    SELECT id FROM disk_usage ORDER BY recorded_at DESC LIMIT ?
);
