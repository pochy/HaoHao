-- name: CreateDriveResourceShare :one
INSERT INTO drive_resource_shares (
    tenant_id,
    resource_type,
    resource_id,
    subject_type,
    subject_id,
    role,
    status,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(resource_type),
    sqlc.arg(resource_id),
    sqlc.arg(subject_type),
    sqlc.arg(subject_id),
    sqlc.arg(role),
    sqlc.arg(status),
    sqlc.arg(created_by_user_id)
)
RETURNING *;

-- name: MarkDriveResourceSharePendingSync :one
UPDATE drive_resource_shares
SET
    status = 'pending_sync',
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'revoked'
RETURNING *;

-- name: RevokeDriveResourceShare :one
UPDATE drive_resource_shares
SET
    status = 'revoked',
    revoked_by_user_id = sqlc.narg(revoked_by_user_id),
    revoked_at = now(),
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'revoked'
RETURNING *;

-- name: ListDriveResourceSharesByResource :many
SELECT *
FROM drive_resource_shares
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_type = sqlc.arg(resource_type)
  AND resource_id = sqlc.arg(resource_id)
  AND status IN ('active', 'pending_sync')
ORDER BY created_at ASC, id ASC;

-- name: GetDriveResourceShareByPublicIDForTenant :one
SELECT *
FROM drive_resource_shares
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'revoked';

-- name: ListActiveDriveResourceSharesBySubject :many
SELECT *
FROM drive_resource_shares
WHERE tenant_id = sqlc.arg(tenant_id)
  AND subject_type = sqlc.arg(subject_type)
  AND subject_id = sqlc.arg(subject_id)
  AND status = 'active'
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count);
