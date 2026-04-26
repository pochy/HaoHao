-- name: CreateDriveFolder :one
INSERT INTO drive_folders (
    tenant_id,
    parent_folder_id,
    name,
    created_by_user_id,
    inheritance_enabled
) VALUES (
    sqlc.arg(tenant_id),
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

-- name: ListDriveChildFolders :many
SELECT *
FROM drive_folders
WHERE tenant_id = sqlc.arg(tenant_id)
  AND parent_folder_id IS NOT DISTINCT FROM sqlc.narg(parent_folder_id)::bigint
  AND deleted_at IS NULL
ORDER BY name ASC, id ASC
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

-- name: MoveDriveFolder :one
UPDATE drive_folders
SET
    parent_folder_id = sqlc.narg(parent_folder_id),
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
