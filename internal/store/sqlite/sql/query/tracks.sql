-- name: UpsertTrack :exec
INSERT INTO tracks (
    id, path, title, artist_id, album_id, album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, artwork_id, uploaded_by_user_id,
    enriched_at, created_at, updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(path) DO UPDATE SET
    title=excluded.title,
    artist_id=excluded.artist_id, album_id=excluded.album_id,
    album_artist=excluded.album_artist, album_name=excluded.album_name,
    genre=excluded.genre,
    year=excluded.year, track_number=excluded.track_number,
    disc_number=excluded.disc_number, duration_ms=excluded.duration_ms,
    bitrate_kbps=excluded.bitrate_kbps, sample_rate_hz=excluded.sample_rate_hz,
    codec=excluded.codec, file_size_bytes=excluded.file_size_bytes,
    last_modified=excluded.last_modified, fingerprint=excluded.fingerprint,
    acoustic_fingerprint=excluded.acoustic_fingerprint,
    mb_recording_id=excluded.mb_recording_id,
    replay_gain_track=excluded.replay_gain_track,
    replay_gain_album=excluded.replay_gain_album,
    artwork_id=excluded.artwork_id,
    uploaded_by_user_id=excluded.uploaded_by_user_id,
    enriched_at=excluded.enriched_at,
    updated_at=excluded.updated_at;

-- name: TrackByPath :one
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.path = ? LIMIT 1;

-- name: TrackByID :one
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.id = ? LIMIT 1;

-- name: ListTracks :many
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL ORDER BY title COLLATE NOCASE;

-- name: ListTracksPage :many
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL ORDER BY title COLLATE NOCASE LIMIT ? OFFSET ?;

-- name: CountTracks :one
SELECT COUNT(*) FROM tracks WHERE tracks.deleted_at IS NULL;

-- name: TrackByFingerprint :one
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.fingerprint = ? AND fingerprint != '' LIMIT 1;

-- name: TrackByAcousticFingerprint :one
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.acoustic_fingerprint = ? AND acoustic_fingerprint != '' AND deleted_at IS NULL LIMIT 1;

-- name: TrackDuplicateByMeta :one
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
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

-- name: DeleteDuplicateFingerprints :execresult
DELETE FROM tracks
WHERE fingerprint != '' AND fingerprint IS NOT NULL
AND rowid NOT IN (
    SELECT MIN(rowid) FROM tracks
    WHERE fingerprint != '' AND fingerprint IS NOT NULL
    GROUP BY fingerprint
);

-- name: SearchTracks :many
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks
WHERE tracks.deleted_at IS NULL
  AND (title LIKE sqlc.arg(pattern)
    OR album_name LIKE sqlc.arg(pattern)
    OR album_artist LIKE sqlc.arg(pattern)
    OR COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') LIKE sqlc.arg(pattern)
    OR genre LIKE sqlc.arg(pattern))
ORDER BY title COLLATE NOCASE LIMIT 200;

-- name: ListTracksByAlbumUnorganized :many
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL AND TRIM(COALESCE(album_name,''))=''
ORDER BY disc_number, track_number, title COLLATE NOCASE;

-- name: ListTracksByAlbumName :many
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL AND album_name = ?
ORDER BY disc_number, track_number, title COLLATE NOCASE;

-- name: ListTracksByAlbumNameAndArtist :many
SELECT id, path, title, COALESCE(artist_id,'') AS artist_id, COALESCE(album_id,'') AS album_id,
    CAST(COALESCE((SELECT name FROM artists WHERE artists.id=tracks.artist_id),'') AS TEXT) AS artist_name,
    album_artist, album_name, genre, year,
    track_number, disc_number, duration_ms, bitrate_kbps, sample_rate_hz,
    codec, file_size_bytes, last_modified, fingerprint, acoustic_fingerprint, mb_recording_id,
    replay_gain_track, replay_gain_album, COALESCE(artwork_id,'') AS artwork_id,
    COALESCE(uploaded_by_user_id,'') AS uploaded_by_user_id, deleted_at,
    enriched_at, created_at, updated_at
FROM tracks WHERE tracks.deleted_at IS NULL AND album_name = ? AND COALESCE(album_artist,'') = ?
ORDER BY disc_number, track_number, title COLLATE NOCASE;
