-- name: UpsertTrack :exec
INSERT INTO tracks (
    id, path, title, album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint,
    uploaded_by_user_id,
    created_at, updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(path) DO UPDATE SET
    title=excluded.title,
    album_artist=excluded.album_artist, album_name=excluded.album_name,
    genre=excluded.genre,
    year=excluded.year, track_number=excluded.track_number,
    disc_number=excluded.disc_number, duration_ms=excluded.duration_ms,
    bitrate_kbps=excluded.bitrate_kbps, sample_rate_hz=excluded.sample_rate_hz,
    codec=excluded.codec, file_size_bytes=excluded.file_size_bytes,
    last_modified=excluded.last_modified, fingerprint=excluded.fingerprint,
    uploaded_by_user_id=excluded.uploaded_by_user_id,
    updated_at=excluded.updated_at;

-- name: TrackByPath :one
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.path = ? LIMIT 1;

-- name: TrackByID :one
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.id = ? LIMIT 1;

-- name: ListTracks :many
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL ORDER BY title COLLATE NOCASE;

-- name: ListTracksPage :many
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL ORDER BY title COLLATE NOCASE LIMIT ? OFFSET ?;

-- name: ListTracksByIDs :many
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks
WHERE tracks.id IN (sqlc.slice('ids'));

-- name: CountTracks :one
SELECT COUNT(*) FROM tracks WHERE tracks.deleted_at IS NULL;

-- name: TrackByFingerprint :one
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.fingerprint = ? AND fingerprint != '' LIMIT 1;

-- name: TrackDuplicateByMeta :one
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks
WHERE tracks.deleted_at IS NULL
  AND LOWER(title)        = LOWER(sqlc.arg(title))
  AND LOWER(album_artist) = LOWER(sqlc.arg(album_artist))
  AND LOWER(album_name)   = LOWER(sqlc.arg(album_name))
  AND ABS(duration_ms - sqlc.arg(duration_ms)) <= 2000
  AND path != sqlc.arg(exclude_path)
LIMIT 1;

-- name: DeleteTrackByPath :exec
DELETE FROM tracks WHERE tracks.path = ?;

-- name: SoftDeleteTrack :exec
UPDATE tracks SET deleted_at = ?, updated_at = ? WHERE id = ?;

-- name: RestoreTrack :exec
UPDATE tracks SET deleted_at = NULL, updated_at = ? WHERE id = ?;

-- name: ReplaceTrackFile :exec
UPDATE tracks
SET path = ?, fingerprint = ?, file_size_bytes = ?, last_modified = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteDuplicateFingerprints :execresult
DELETE FROM tracks
WHERE fingerprint != '' AND fingerprint IS NOT NULL
AND rowid NOT IN (
    SELECT MIN(rowid) FROM tracks
    WHERE fingerprint != '' AND fingerprint IS NOT NULL
    GROUP BY fingerprint
);

-- name: SearchTracksByIDs :many
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks
WHERE tracks.deleted_at IS NULL
  AND tracks.id IN (sqlc.slice('ids'));

-- name: ListTracksByAlbumUnorganized :many
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL AND TRIM(COALESCE(album_name,''))=''
ORDER BY disc_number, track_number, title COLLATE NOCASE;

-- name: ListTracksByAlbumName :many
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL AND album_name = ?
ORDER BY disc_number, track_number, title COLLATE NOCASE;

-- name: ListTracksByAlbumNameAndArtist :many
SELECT id, path, title,
    COALESCE(album_artist,'') AS album_artist, COALESCE(album_name,'') AS album_name,
    COALESCE(genre,'') AS genre, COALESCE(year,0) AS year,
    COALESCE(track_number,0) AS track_number, COALESCE(disc_number,0) AS disc_number,
    COALESCE(duration_ms,0) AS duration_ms, COALESCE(bitrate_kbps,0) AS bitrate_kbps,
    COALESCE(sample_rate_hz,0) AS sample_rate_hz, COALESCE(codec,'') AS codec,
    COALESCE(file_size_bytes,0) AS file_size_bytes, last_modified,
    COALESCE(fingerprint,'') AS fingerprint,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id,
    deleted_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL AND album_name = ? AND COALESCE(album_artist,'') = ?
ORDER BY disc_number, track_number, title COLLATE NOCASE;

-- name: SumTrackBytes :one
SELECT CAST(COALESCE(SUM(file_size_bytes), 0) AS INTEGER) FROM tracks WHERE deleted_at IS NULL;
