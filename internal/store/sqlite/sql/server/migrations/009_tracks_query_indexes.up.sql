CREATE INDEX IF NOT EXISTS idx_tracks_active_title
ON tracks (deleted_at, title COLLATE NOCASE);

CREATE INDEX IF NOT EXISTS idx_tracks_active_album_artist_sort
ON tracks (
    deleted_at,
    album_name,
    album_artist,
    disc_number,
    track_number,
    title COLLATE NOCASE
);

CREATE INDEX IF NOT EXISTS idx_tracks_fingerprint
ON tracks (fingerprint);
