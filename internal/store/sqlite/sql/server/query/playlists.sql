-- name: CreatePlaylist :exec
INSERT INTO playlists (id, user_id, name, description, artwork_path, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetPlaylistByID :one
SELECT id, user_id, name, description, artwork_path, created_at, updated_at
FROM playlists WHERE id = ?;

-- name: ListPlaylistsByUser :many
SELECT p.id, p.user_id, p.name, p.description, p.artwork_path, p.created_at, p.updated_at,
       COUNT(pi.playlist_id) AS item_count,
       CAST(COALESCE(SUM(CASE
           WHEN pi.source = 'remote' THEN COALESCE(t.duration_ms, 0)
           ELSE pi.ref_duration_ms
       END), 0) AS INTEGER) AS total_duration_ms
FROM playlists p
LEFT JOIN playlist_items pi ON pi.playlist_id = p.id
LEFT JOIN tracks t ON t.id = pi.track_id
WHERE p.user_id = ?
GROUP BY p.id
ORDER BY p.updated_at DESC;

-- name: UpdatePlaylist :exec
UPDATE playlists SET name = ?, description = ?, artwork_path = ?, updated_at = ?
WHERE id = ?;

-- name: DeletePlaylist :exec
DELETE FROM playlists WHERE id = ?;

-- name: DeletePlaylistItems :exec
DELETE FROM playlist_items WHERE playlist_id = ?;

-- name: InsertPlaylistItem :exec
INSERT INTO playlist_items (playlist_id, position, source, track_id, ref_title, ref_album, ref_album_artist, ref_duration_ms, added_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPlaylistItems :many
SELECT pi.playlist_id, pi.position, pi.source,
       COALESCE(pi.track_id, '') AS track_id,
       pi.ref_title, pi.ref_album, pi.ref_album_artist, pi.ref_duration_ms, pi.added_at,
       CAST(
           pi.source = 'remote' AND (pi.track_id IS NULL OR t.id IS NULL OR t.deleted_at IS NOT NULL)
       AS BOOLEAN) AS missing
FROM playlist_items pi
LEFT JOIN tracks t ON t.id = pi.track_id
WHERE pi.playlist_id = ?
ORDER BY pi.position;

-- name: CountPlaylistItems :one
SELECT COUNT(*) FROM playlist_items WHERE playlist_id = ?;

-- name: SumPlaylistDuration :one
SELECT CAST(COALESCE(SUM(CASE
    WHEN pi.source = 'remote' THEN COALESCE(t.duration_ms, 0)
    ELSE pi.ref_duration_ms
END), 0) AS INTEGER) AS total_duration_ms
FROM playlist_items pi
LEFT JOIN tracks t ON t.id = pi.track_id
WHERE pi.playlist_id = ?;

-- name: TouchPlaylist :exec
UPDATE playlists SET updated_at = ? WHERE id = ?;
