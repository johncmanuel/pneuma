-- name: GetRecentAlbums :many
SELECT key, name, artist, is_local, first_track_id, first_local_path, played_at 
FROM recent_albums 
ORDER BY played_at DESC;

-- name: GetRecentAlbum :one
SELECT key, name, artist, is_local, first_track_id, first_local_path, played_at 
FROM recent_albums 
WHERE key = ?;

-- name: SetRecentAlbum :exec
INSERT INTO recent_albums (key, name, artist, is_local, first_track_id, first_local_path, played_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
    name = excluded.name,
    artist = excluded.artist,
    is_local = excluded.is_local,
    first_track_id = excluded.first_track_id,
    first_local_path = excluded.first_local_path,
    played_at = excluded.played_at;

-- name: DeleteRecentAlbum :exec
DELETE FROM recent_albums WHERE key = ?;

-- name: DeleteAllRecentAlbums :exec
DELETE FROM recent_albums;

-- name: GetRecentPlaylists :many
SELECT id, name, artwork_path, played_at 
FROM recent_playlists 
ORDER BY played_at DESC;

-- name: GetRecentPlaylist :one
SELECT id, name, artwork_path, played_at 
FROM recent_playlists 
WHERE id = ?;

-- name: SetRecentPlaylist :exec
INSERT INTO recent_playlists (id, name, artwork_path, played_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    artwork_path = excluded.artwork_path,
    played_at = excluded.played_at;

-- name: DeleteRecentPlaylist :exec
DELETE FROM recent_playlists WHERE id = ?;

-- name: DeleteAllRecentPlaylists :exec
DELETE FROM recent_playlists;
