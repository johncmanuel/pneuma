CREATE VIRTUAL TABLE IF NOT EXISTS tracks_fts USING fts5(
    title,
    album_name,
    album_artist,
    genre,
    track_id UNINDEXED
);

INSERT INTO tracks_fts (rowid, title, album_name, album_artist, genre, track_id)
SELECT
    rowid,
    COALESCE(title, ''),
    COALESCE(album_name, ''),
    COALESCE(album_artist, ''),
    COALESCE(genre, ''),
    id
FROM tracks;

CREATE TRIGGER IF NOT EXISTS tracks_fts_insert
AFTER INSERT ON tracks
BEGIN
    INSERT INTO tracks_fts (rowid, title, album_name, album_artist, genre, track_id)
    VALUES (
        NEW.rowid,
        COALESCE(NEW.title, ''),
        COALESCE(NEW.album_name, ''),
        COALESCE(NEW.album_artist, ''),
        COALESCE(NEW.genre, ''),
        NEW.id
    );
END;

CREATE TRIGGER IF NOT EXISTS tracks_fts_update
AFTER UPDATE ON tracks
BEGIN
    DELETE FROM tracks_fts WHERE rowid = OLD.rowid;
    INSERT INTO tracks_fts (rowid, title, album_name, album_artist, genre, track_id)
    VALUES (
        NEW.rowid,
        COALESCE(NEW.title, ''),
        COALESCE(NEW.album_name, ''),
        COALESCE(NEW.album_artist, ''),
        COALESCE(NEW.genre, ''),
        NEW.id
    );
END;

CREATE TRIGGER IF NOT EXISTS tracks_fts_delete
AFTER DELETE ON tracks
BEGIN
    DELETE FROM tracks_fts WHERE rowid = OLD.rowid;
END;

CREATE TABLE IF NOT EXISTS track_album_groups (
    key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    artist TEXT NOT NULL,
    track_count INTEGER NOT NULL DEFAULT 0,
    first_track_id TEXT NOT NULL DEFAULT ''
);

INSERT OR REPLACE INTO track_album_groups (key, name, artist, track_count, first_track_id)
SELECT
    CASE
        WHEN TRIM(COALESCE(album_name, '')) = '' THEN '__unorganized__'
        ELSE COALESCE(album_name, '') || '|||' || COALESCE(album_artist, '')
    END AS grp_key,
    COALESCE(NULLIF(TRIM(album_name), ''), '') AS album_name,
    COALESCE(album_artist, '') AS album_artist,
    COUNT(*) AS track_count,
    MIN(id) AS first_track_id
FROM tracks
WHERE deleted_at IS NULL
GROUP BY grp_key;

CREATE TRIGGER IF NOT EXISTS track_album_groups_rebuild_on_insert
AFTER INSERT ON tracks
BEGIN
    DELETE FROM track_album_groups;

    INSERT OR REPLACE INTO track_album_groups (key, name, artist, track_count, first_track_id)
    SELECT
        CASE
            WHEN TRIM(COALESCE(album_name, '')) = '' THEN '__unorganized__'
            ELSE COALESCE(album_name, '') || '|||' || COALESCE(album_artist, '')
        END AS grp_key,
        COALESCE(NULLIF(TRIM(album_name), ''), '') AS album_name,
        COALESCE(album_artist, '') AS album_artist,
        COUNT(*) AS track_count,
        MIN(id) AS first_track_id
    FROM tracks
    WHERE deleted_at IS NULL
    GROUP BY grp_key;
END;

CREATE TRIGGER IF NOT EXISTS track_album_groups_rebuild_on_update
AFTER UPDATE ON tracks
BEGIN
    DELETE FROM track_album_groups;

    INSERT OR REPLACE INTO track_album_groups (key, name, artist, track_count, first_track_id)
    SELECT
        CASE
            WHEN TRIM(COALESCE(album_name, '')) = '' THEN '__unorganized__'
            ELSE COALESCE(album_name, '') || '|||' || COALESCE(album_artist, '')
        END AS grp_key,
        COALESCE(NULLIF(TRIM(album_name), ''), '') AS album_name,
        COALESCE(album_artist, '') AS album_artist,
        COUNT(*) AS track_count,
        MIN(id) AS first_track_id
    FROM tracks
    WHERE deleted_at IS NULL
    GROUP BY grp_key;
END;

CREATE TRIGGER IF NOT EXISTS track_album_groups_rebuild_on_delete
AFTER DELETE ON tracks
BEGIN
    DELETE FROM track_album_groups;

    INSERT OR REPLACE INTO track_album_groups (key, name, artist, track_count, first_track_id)
    SELECT
        CASE
            WHEN TRIM(COALESCE(album_name, '')) = '' THEN '__unorganized__'
            ELSE COALESCE(album_name, '') || '|||' || COALESCE(album_artist, '')
        END AS grp_key,
        COALESCE(NULLIF(TRIM(album_name), ''), '') AS album_name,
        COALESCE(album_artist, '') AS album_artist,
        COUNT(*) AS track_count,
        MIN(id) AS first_track_id
    FROM tracks
    WHERE deleted_at IS NULL
    GROUP BY grp_key;
END;

CREATE INDEX IF NOT EXISTS idx_track_album_groups_name ON track_album_groups (name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_track_album_groups_artist ON track_album_groups (artist COLLATE NOCASE);
