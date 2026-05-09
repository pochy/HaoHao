-- name: CreateDriveFolder :one
INSERT INTO drive_folders (
    tenant_id,
    workspace_id,
    parent_folder_id,
    name,
    created_by_user_id,
    inheritance_enabled
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.narg(workspace_id),
    sqlc.narg(parent_folder_id),
    sqlc.arg(name),
    sqlc.arg(created_by_user_id),
    sqlc.arg(inheritance_enabled)
)
RETURNING *;

-- name: GetDriveFolderByPublicIDForTenant :one
SELECT *
FROM drive_folders
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL;

-- name: GetDriveFolderByIDForTenant :one
SELECT *
FROM drive_folders
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL;

-- name: GetDeletedDriveFolderByPublicIDForTenant :one
SELECT *
FROM drive_folders
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NOT NULL;

-- name: ListDeletedDriveFolders :many
SELECT *
FROM drive_folders
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListDriveChildFolders :many
SELECT *
FROM drive_folders
WHERE tenant_id = sqlc.arg(tenant_id)
  AND parent_folder_id IS NOT DISTINCT FROM sqlc.narg(parent_folder_id)::bigint
  AND (
      sqlc.narg(workspace_id)::bigint IS NULL
      OR workspace_id = sqlc.narg(workspace_id)::bigint
  )
  AND deleted_at IS NULL
ORDER BY
    CASE WHEN sqlc.arg(sort_key)::text = 'name' AND sqlc.arg(direction)::text = 'asc' THEN lower(name) END ASC,
    CASE WHEN sqlc.arg(sort_key)::text = 'name' AND sqlc.arg(direction)::text = 'desc' THEN lower(name) END DESC,
    CASE WHEN sqlc.arg(sort_key)::text = 'updated_at' AND sqlc.arg(direction)::text = 'asc' THEN updated_at END ASC,
    CASE WHEN sqlc.arg(sort_key)::text = 'updated_at' AND sqlc.arg(direction)::text = 'desc' THEN updated_at END DESC,
    updated_at DESC,
    id DESC
LIMIT sqlc.arg(limit_count);

-- name: SearchDriveFolderCandidates :many
SELECT *
FROM drive_folders
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
  AND (
      sqlc.narg(query)::text IS NULL
      OR name ILIKE '%' || sqlc.narg(query)::text || '%'
  )
  AND (
      sqlc.narg(updated_after)::timestamptz IS NULL
      OR updated_at >= sqlc.narg(updated_after)::timestamptz
  )
  AND (
      sqlc.narg(updated_before)::timestamptz IS NULL
      OR updated_at <= sqlc.narg(updated_before)::timestamptz
  )
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: RenameDriveFolder :one
UPDATE drive_folders
SET
    name = sqlc.arg(name),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateDriveFolderDescription :one
UPDATE drive_folders
SET
    description = sqlc.arg(description),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: MoveDriveFolder :one
UPDATE drive_folders
SET
    parent_folder_id = sqlc.narg(parent_folder_id),
    workspace_id = sqlc.narg(workspace_id),
    inheritance_enabled = sqlc.arg(inheritance_enabled),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDriveFolder :one
UPDATE drive_folders
SET
    deleted_at = COALESCE(deleted_at, now()),
    deleted_by_user_id = sqlc.narg(deleted_by_user_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: RestoreDriveFolder :one
UPDATE drive_folders
SET
    parent_folder_id = sqlc.narg(parent_folder_id),
    workspace_id = sqlc.narg(workspace_id),
    deleted_at = NULL,
    deleted_by_user_id = NULL,
    deleted_parent_folder_id = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NOT NULL
RETURNING *;

-- name: IsDriveFolderDescendant :one
WITH RECURSIVE descendants AS (
    SELECT folder.id
    FROM drive_folders folder
    WHERE folder.id = sqlc.arg(source_folder_id)
      AND folder.tenant_id = sqlc.arg(tenant_id)
      AND folder.deleted_at IS NULL
    UNION ALL
    SELECT child.id
    FROM drive_folders child
    JOIN descendants d ON child.parent_folder_id = d.id
    WHERE child.tenant_id = sqlc.arg(tenant_id)
      AND child.deleted_at IS NULL
)
SELECT (
    sqlc.narg(candidate_parent_folder_id)::bigint IS NOT NULL
    AND EXISTS (
        SELECT 1
        FROM descendants
        WHERE id = sqlc.narg(candidate_parent_folder_id)::bigint
    )
)::boolean AS is_descendant;
