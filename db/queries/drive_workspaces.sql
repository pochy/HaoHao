-- name: CreateDriveWorkspace :one
INSERT INTO drive_workspaces (
    tenant_id,
    name,
    created_by_user_id,
    storage_quota_bytes,
    policy_override
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(name),
    sqlc.narg(created_by_user_id),
    sqlc.narg(storage_quota_bytes),
    COALESCE(sqlc.narg(policy_override), '{}'::jsonb)
)
RETURNING *;

-- name: GetDriveWorkspaceByPublicIDForTenant :one
SELECT *
FROM drive_workspaces
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL;

-- name: GetDriveWorkspaceByIDForTenant :one
SELECT *
FROM drive_workspaces
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL;

-- name: ListDriveWorkspaces :many
SELECT *
FROM drive_workspaces
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
ORDER BY name ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: CountDriveWorkspaces :one
SELECT count(*)::bigint
FROM drive_workspaces
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL;

-- name: GetDefaultDriveWorkspace :one
SELECT *
FROM drive_workspaces
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
ORDER BY id ASC
LIMIT 1;

-- name: UpdateDriveWorkspace :one
UPDATE drive_workspaces
SET
    name = sqlc.arg(name),
    storage_quota_bytes = sqlc.narg(storage_quota_bytes),
    policy_override = COALESCE(sqlc.narg(policy_override), policy_override),
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDriveWorkspace :one
UPDATE drive_workspaces
SET
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;
