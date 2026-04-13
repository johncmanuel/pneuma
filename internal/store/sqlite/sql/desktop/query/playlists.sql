-- name: CreateLocalPlaylist :exec
INSERT INTO local_playlists (id, name, description, artwork_path, remote_playlist_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetLocalPlaylistByID :one
SELECT id, name, description, artwork_path, remote_playlist_id, created_at, updated_at
FROM local_playlists WHERE id = ?;

-- name: GetLocalPlaylistByRemoteID :one
SELECT id, name, description, artwork_path, remote_playlist_id, created_at, updated_at
FROM local_playlists WHERE remote_playlist_id = ? LIMIT 1;

-- name: ListLocalPlaylists :many
SELECT lp.id, lp.name, lp.description, lp.artwork_path, lp.remote_playlist_id,
       lp.created_at, lp.updated_at,
       COUNT(li.playlist_id) AS item_count,
       CAST(COALESCE(SUM(CASE
           WHEN li.source = 'local_ref' THEN li.ref_duration_ms
           ELSE li.ref_duration_ms
       END), 0) AS INTEGER) AS total_duration_ms
FROM local_playlists lp
LEFT JOIN local_playlist_items li ON li.playlist_id = lp.id
GROUP BY lp.id
ORDER BY lp.updated_at DESC;

-- name: UpdateLocalPlaylist :exec
UPDATE local_playlists SET name = ?, description = ?, artwork_path = ?, remote_playlist_id = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateLocalPlaylistArtwork :exec
UPDATE local_playlists SET artwork_path = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteLocalPlaylist :exec
DELETE FROM local_playlists WHERE id = ?;

-- name: DeleteLocalPlaylistItems :exec
DELETE FROM local_playlist_items WHERE playlist_id = ?;

-- name: InsertLocalPlaylistItem :exec
INSERT INTO local_playlist_items (playlist_id, position, source, track_id, local_path, ref_title, ref_album, ref_album_artist, ref_duration_ms, added_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListLocalPlaylistItems :many
SELECT playlist_id, position, source, track_id, local_path,
       ref_title, ref_album, ref_album_artist, ref_duration_ms, added_at
FROM local_playlist_items
WHERE playlist_id = ?
ORDER BY position;

-- name: CountLocalPlaylistItems :one
SELECT COUNT(*) FROM local_playlist_items WHERE playlist_id = ?;

-- name: TouchLocalPlaylist :exec
UPDATE local_playlists SET updated_at = ? WHERE id = ?;
