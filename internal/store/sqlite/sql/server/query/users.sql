-- name: CreateUser :exec
INSERT INTO users (
    id,
    username,
    password_hash,
    is_admin,
    can_upload,
    can_edit,
    can_delete,
    created_at,
    updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateUserPermissions :exec
UPDATE users SET can_upload = ?, can_edit = ?, can_delete = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: ListUsers :many
SELECT
    id,
    username,
    password_hash,
    is_admin,
    can_upload,
    can_edit,
    can_delete,
    created_at,
    updated_at
FROM users
ORDER BY created_at;

-- name: UserByUsername :one
SELECT
    id,
    username,
    password_hash,
    is_admin,
    can_upload,
    can_edit,
    can_delete,
    created_at,
    updated_at
FROM users
WHERE username = ? LIMIT 1;

-- name: UserByID :one
SELECT
    id,
    username,
    password_hash,
    is_admin,
    can_upload,
    can_edit,
    can_delete,
    created_at,
    updated_at
FROM users
WHERE id = ? LIMIT 1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;
