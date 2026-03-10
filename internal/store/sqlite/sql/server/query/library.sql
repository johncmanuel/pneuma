-- name: UpsertAlbum :exec
INSERT INTO albums (id, title, artist_id, year, mb_release_id, artwork_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    title=excluded.title, artist_id=excluded.artist_id, year=excluded.year,
    mb_release_id=excluded.mb_release_id, artwork_id=excluded.artwork_id;

-- name: AlbumByID :one
SELECT id, title, COALESCE(artist_id,'') AS artist_id, year,
    COALESCE(mb_release_id,'') AS mb_release_id, COALESCE(artwork_id,'') AS artwork_id, created_at
FROM albums WHERE id = ?;

-- name: AlbumByTitleArtist :one
SELECT id, title, COALESCE(artist_id,'') AS artist_id, year,
    COALESCE(mb_release_id,'') AS mb_release_id, COALESCE(artwork_id,'') AS artwork_id, created_at
FROM albums WHERE title = ? AND COALESCE(artist_id,'') = ? LIMIT 1;

-- name: ListAlbums :many
SELECT id, title, COALESCE(artist_id,'') AS artist_id, year,
    COALESCE(mb_release_id,'') AS mb_release_id, COALESCE(artwork_id,'') AS artwork_id, created_at
FROM albums ORDER BY title COLLATE NOCASE;

-- name: CountAlbums :one
SELECT COUNT(*) FROM albums;

-- name: CountAlbumsFiltered :one
SELECT COUNT(*) FROM albums WHERE title LIKE ?;

-- name: ListAlbumsPage :many
SELECT id, title, COALESCE(artist_id,'') AS artist_id, year,
    COALESCE(mb_release_id,'') AS mb_release_id, COALESCE(artwork_id,'') AS artwork_id, created_at
FROM albums ORDER BY title COLLATE NOCASE LIMIT ? OFFSET ?;

-- name: ListAlbumsPageFiltered :many
SELECT id, title, COALESCE(artist_id,'') AS artist_id, year,
    COALESCE(mb_release_id,'') AS mb_release_id, COALESCE(artwork_id,'') AS artwork_id, created_at
FROM albums WHERE title LIKE ? ORDER BY title COLLATE NOCASE LIMIT ? OFFSET ?;

-- name: UpsertArtist :exec
INSERT INTO artists (id, name, mb_artist_id, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET name=excluded.name, mb_artist_id=excluded.mb_artist_id;

-- name: ArtistByName :one
SELECT id, name, COALESCE(mb_artist_id,'') AS mb_artist_id, created_at
FROM artists WHERE name = ? LIMIT 1;

-- name: UpsertWatchFolder :exec
INSERT OR IGNORE INTO watch_folders (id, path, user_id, created_at) VALUES (?, ?, ?, ?);

-- name: ListWatchFolders :many
SELECT id, path, user_id, created_at FROM watch_folders;

-- name: UpsertArtwork :exec
INSERT INTO artworks (id, path, width, height, format, created_at) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET width=excluded.width, height=excluded.height, format=excluded.format;

-- name: ArtworkByPath :one
SELECT id, path, COALESCE(width,0) AS width, COALESCE(height,0) AS height,
    COALESCE(format,'') AS format, created_at
FROM artworks WHERE path = ? LIMIT 1;
