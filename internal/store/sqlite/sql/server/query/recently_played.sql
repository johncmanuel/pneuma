-- name: UpsertRecentAlbum :exec
INSERT INTO recent_albums (user_id, album_name, album_artist, first_track_id, played_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (user_id, album_name, album_artist) DO UPDATE SET
    first_track_id = excluded.first_track_id,
    played_at = excluded.played_at;

-- name: ListRecentAlbumsByUser :many
SELECT album_name, album_artist, first_track_id, played_at
FROM recent_albums
WHERE user_id = ?
ORDER BY played_at DESC
LIMIT 50;

-- name: UpsertRecentPlaylist :exec
INSERT INTO recent_playlists (user_id, playlist_id, played_at)
VALUES (?, ?, ?)
ON CONFLICT (user_id, playlist_id) DO UPDATE SET
    played_at = excluded.played_at;

-- name: ListRecentPlaylistsByUser :many
SELECT rp.playlist_id, p.name, COALESCE(p.artwork_path, '') AS artwork_path, rp.played_at
FROM recent_playlists rp
JOIN playlists p ON p.id = rp.playlist_id
WHERE rp.user_id = ?
ORDER BY rp.played_at DESC
LIMIT 50;

-- name: DeleteRecentPlaylist :exec
DELETE FROM recent_playlists
WHERE user_id = ? AND playlist_id = ?;

-- name: ClearRecentAlbumsByUser :exec
DELETE FROM recent_albums WHERE user_id = ?;

-- name: ClearRecentPlaylistsByUser :exec
DELETE FROM recent_playlists WHERE user_id = ?;
