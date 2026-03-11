-- name: UpsertWatchFolder :exec
INSERT OR IGNORE INTO watch_folders (id, path, user_id, created_at) VALUES (?, ?, ?, ?);

-- name: ListWatchFolders :many
SELECT id, path, user_id, created_at FROM watch_folders;
