-- Keep the "name" field in here in case there are feature requests for device management

-- name: UpsertDevice :exec
INSERT INTO devices (
    id, user_id, name, created_at, last_active
) VALUES ( ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    last_active = excluded.last_active;

-- name: GetDevice :one
SELECT id, user_id, name, created_at, last_active
FROM devices
WHERE id = ? LIMIT 1;

-- name: ListUserDevices :many
SELECT id, user_id, name, created_at, last_active
FROM devices
WHERE user_id = ?
ORDER BY last_active DESC;

-- name: DeleteDevice :exec
DELETE FROM devices WHERE id = ?;
