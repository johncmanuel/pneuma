-- name: InsertAuditEntry :exec
INSERT INTO audit_log (
    id, user_id, action, target_type, target_id, detail, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListAuditEntries :many
SELECT
    id,
    user_id,
    action,
    target_type,
    target_id,
    detail,
    created_at
FROM audit_log
ORDER BY created_at DESC LIMIT ?;
