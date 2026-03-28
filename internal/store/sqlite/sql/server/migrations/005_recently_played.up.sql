CREATE TABLE IF NOT EXISTS recent_albums (
    user_id TEXT NOT NULL,
    album_name TEXT NOT NULL,
    album_artist TEXT NOT NULL DEFAULT '',
    first_track_id TEXT NOT NULL DEFAULT '',
    played_at TEXT NOT NULL,
    PRIMARY KEY (user_id, album_name, album_artist)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS recent_playlists (
    user_id TEXT NOT NULL,
    playlist_id TEXT NOT NULL,
    played_at TEXT NOT NULL,
    PRIMARY KEY (user_id, playlist_id),
    FOREIGN KEY (playlist_id) REFERENCES playlists (id) ON DELETE CASCADE
) WITHOUT ROWID;
