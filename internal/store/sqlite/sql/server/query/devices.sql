-- name: UpsertDevice :exec
INSERT INTO devices (id, user_id, name, last_seen_at, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name, last_seen_at = excluded.last_seen_at;

-- name: DevicesByUser :many
SELECT
    id,
    user_id,
    name,
    last_seen_at,
    created_at
FROM devices
WHERE user_id = ?
ORDER BY last_seen_at DESC;

-- name: TouchDevice :exec
UPDATE devices SET last_seen_at = ?
WHERE id = ?;
