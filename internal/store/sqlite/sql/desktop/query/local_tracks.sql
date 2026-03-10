-- name: UpsertLocalTrack :exec
INSERT INTO local_tracks (
    path, folder, title, artist, album, album_artist, genre,
    year, track_number, disc_number, duration_ms, has_artwork
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    folder       = excluded.folder,
    title        = excluded.title,
    artist       = excluded.artist,
    album        = excluded.album,
    album_artist = excluded.album_artist,
    genre        = excluded.genre,
    year         = excluded.year,
    track_number = excluded.track_number,
    disc_number  = excluded.disc_number,
    duration_ms  = excluded.duration_ms,
    has_artwork  = excluded.has_artwork;

-- name: DeleteLocalTracksByFolder :exec
DELETE FROM local_tracks WHERE folder = ?;

-- name: DeleteLocalTrackByPath :exec
DELETE FROM local_tracks WHERE path = ?;

-- name: DeleteLocalTracksByPathPrefix :execrows
DELETE FROM local_tracks WHERE path = ? OR path LIKE ?;

-- name: ListPathsByFolder :many
SELECT path FROM local_tracks WHERE folder = ?;

-- name: ListAllLocalTracks :many
SELECT path, folder, title, artist, album, album_artist, genre,
       year, track_number, disc_number, duration_ms, has_artwork
FROM local_tracks
ORDER BY folder, path;

-- name: CountAllLocalTracks :one
SELECT COUNT(*) FROM local_tracks;

-- name: AllLocalTracksPage :many
SELECT path, folder, title, artist, album, album_artist, genre,
       year, track_number, disc_number, duration_ms, has_artwork
FROM local_tracks
ORDER BY album COLLATE NOCASE, disc_number, track_number
LIMIT ? OFFSET ?;

-- name: SearchAllLocalTracks :many
SELECT path, folder, title, artist, album, album_artist, genre,
       year, track_number, disc_number, duration_ms, has_artwork
FROM local_tracks
WHERE title LIKE ? OR artist LIKE ? OR album LIKE ? OR path LIKE ?
ORDER BY title COLLATE NOCASE
LIMIT 50;
