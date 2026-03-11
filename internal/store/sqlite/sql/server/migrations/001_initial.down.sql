-- Drop indexes first
DROP INDEX IF EXISTS idx_playlist_items_playlist;
DROP INDEX IF EXISTS idx_playlists_user;
DROP INDEX IF EXISTS idx_audit_target;
DROP INDEX IF EXISTS idx_audit_user;
DROP INDEX IF EXISTS idx_sessions_user;
DROP INDEX IF EXISTS idx_sessions_device;
DROP INDEX IF EXISTS idx_devices_user;
DROP INDEX IF EXISTS idx_tracks_path;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS playback_sessions;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS watch_folders;
DROP TABLE IF EXISTS playlist_items;
DROP TABLE IF EXISTS playlists;
DROP TABLE IF EXISTS tracks;
DROP TABLE IF EXISTS users;
