DROP INDEX IF EXISTS idx_track_album_groups_artist;
DROP INDEX IF EXISTS idx_track_album_groups_name;
DROP TRIGGER IF EXISTS track_album_groups_rebuild_on_delete;
DROP TRIGGER IF EXISTS track_album_groups_rebuild_on_update;
DROP TRIGGER IF EXISTS track_album_groups_rebuild_on_insert;
DROP TABLE IF EXISTS track_album_groups;

DROP TRIGGER IF EXISTS tracks_fts_delete;
DROP TRIGGER IF EXISTS tracks_fts_update;
DROP TRIGGER IF EXISTS tracks_fts_insert;
DROP TABLE IF EXISTS tracks_fts;
