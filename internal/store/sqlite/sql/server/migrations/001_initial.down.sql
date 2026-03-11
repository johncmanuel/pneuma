-- Drop indexes first
DROP INDEX IF EXISTS idx_playlist_items_playlist;
DROP INDEX IF EXISTS idx_playlists_user;
DROP INDEX IF EXISTS idx_audit_target;
DROP INDEX IF EXISTS idx_audit_user;
DROP INDEX IF EXISTS idx_offline_user;
DROP INDEX IF EXISTS idx_sessions_user;
DROP INDEX IF EXISTS idx_sessions_device;
DROP INDEX IF EXISTS idx_devices_user;
DROP INDEX IF EXISTS idx_albums_artist;
DROP INDEX IF EXISTS idx_tracks_acoustic_fp;
DROP INDEX IF EXISTS idx_tracks_path;
DROP INDEX IF EXISTS idx_tracks_album;
DROP INDEX IF EXISTS idx_tracks_artist;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS offline_packs;
DROP TABLE IF EXISTS playback_sessions;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS watch_folders;
DROP TABLE IF EXISTS playlist_items;
DROP TABLE IF EXISTS playlists;
DROP TABLE IF EXISTS tracks;
DROP TABLE IF EXISTS albums;
DROP TABLE IF EXISTS artworks;
DROP TABLE IF EXISTS artists;
DROP TABLE IF EXISTS users;
