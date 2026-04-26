-- name: CreateDriveGroup :one
INSERT INTO drive_groups (
    tenant_id,
    name,
    description,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(name),
    sqlc.arg(description),
    sqlc.arg(created_by_user_id)
)
RETURNING *;

-- name: GetDriveGroupByPublicIDForTenant :one
SELECT *
FROM drive_groups
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL;

-- name: GetDriveGroupByIDForTenant :one
SELECT *
FROM drive_groups
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL;

-- name: ListDriveGroups :many
SELECT *
FROM drive_groups
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
ORDER BY name ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: UpdateDriveGroup :one
UPDATE drive_groups
SET
    name = sqlc.arg(name),
    description = sqlc.arg(description),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDriveGroup :one
UPDATE drive_groups
SET
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: AddDriveGroupMember :one
INSERT INTO drive_group_members (
    group_id,
    user_id,
    added_by_user_id
) VALUES (
    sqlc.arg(group_id),
    sqlc.arg(user_id),
    sqlc.arg(added_by_user_id)
)
RETURNING *;

-- name: RemoveDriveGroupMember :one
UPDATE drive_group_members
SET
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE group_id = sqlc.arg(group_id)
  AND user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: ListDriveGroupMembers :many
SELECT *
FROM drive_group_members
WHERE group_id = sqlc.arg(group_id)
  AND deleted_at IS NULL
ORDER BY created_at ASC, id ASC;

-- name: ListDriveGroupsForUser :many
SELECT g.*
FROM drive_groups g
JOIN drive_group_members gm ON gm.group_id = g.id
WHERE gm.user_id = sqlc.arg(user_id)
  AND gm.deleted_at IS NULL
  AND g.tenant_id = sqlc.arg(tenant_id)
  AND g.deleted_at IS NULL
ORDER BY g.name ASC, g.id ASC;
