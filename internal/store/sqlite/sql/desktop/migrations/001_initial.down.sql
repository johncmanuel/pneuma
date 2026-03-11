-- Drop indexes first
DROP INDEX IF EXISTS idx_lpi_playlist;
DROP INDEX IF EXISTS idx_lp_remote;
DROP INDEX IF EXISTS idx_lt_folder;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS local_playlist_items;
DROP TABLE IF EXISTS local_playlists;
DROP TABLE IF EXISTS local_tracks;
DROP TABLE IF EXISTS kv;
