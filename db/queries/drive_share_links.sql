-- name: CreateDriveShareLink :one
INSERT INTO drive_share_links (
    tenant_id,
    resource_type,
    resource_id,
    token_hash,
    role,
    can_download,
    expires_at,
    status,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(resource_type),
    sqlc.arg(resource_id),
    sqlc.arg(token_hash),
    sqlc.arg(role),
    sqlc.arg(can_download),
    sqlc.arg(expires_at),
    sqlc.arg(status),
    sqlc.arg(created_by_user_id)
)
RETURNING *;

-- name: LookupActiveDriveShareLinkByTokenHash :one
SELECT *
FROM drive_share_links
WHERE token_hash = sqlc.arg(token_hash)
  AND status = 'active'
  AND expires_at > now();

-- name: GetDriveShareLinkByPublicIDForTenant :one
SELECT *
FROM drive_share_links
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id);

-- name: UpdateDriveShareLink :one
UPDATE drive_share_links
SET
    can_download = sqlc.arg(can_download),
    expires_at = sqlc.arg(expires_at),
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'disabled'
RETURNING *;

-- name: DisableDriveShareLink :one
UPDATE drive_share_links
SET
    status = 'disabled',
    disabled_by_user_id = sqlc.narg(disabled_by_user_id),
    disabled_at = now(),
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'disabled'
RETURNING *;

-- name: MarkDriveShareLinkPendingSync :one
UPDATE drive_share_links
SET
    status = 'pending_sync',
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status <> 'disabled'
RETURNING *;

-- name: ListDriveShareLinksByResource :many
SELECT *
FROM drive_share_links
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_type = sqlc.arg(resource_type)
  AND resource_id = sqlc.arg(resource_id)
  AND status IN ('active', 'pending_sync')
ORDER BY created_at DESC, id DESC;
