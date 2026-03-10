-- name: GetKV :one
SELECT value FROM kv WHERE key = ?;

-- name: SetKV :exec
INSERT INTO kv (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: DeleteKV :exec
DELETE FROM kv WHERE key = ?;
