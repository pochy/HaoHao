-- name: CreateDriveFileObject :one
INSERT INTO file_objects (
    tenant_id,
    uploaded_by_user_id,
    purpose,
    attached_to_type,
    attached_to_id,
    workspace_id,
    drive_folder_id,
    original_filename,
    content_type,
    byte_size,
    sha256_hex,
    content_sha256,
    storage_driver,
    storage_key,
    storage_bucket,
    etag,
    scan_status,
    inheritance_enabled
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.narg(uploaded_by_user_id),
    'drive',
    NULL,
    NULL,
    sqlc.narg(workspace_id),
    sqlc.narg(drive_folder_id),
    sqlc.arg(original_filename),
    sqlc.arg(content_type),
    sqlc.arg(byte_size),
    sqlc.arg(sha256_hex),
    NULLIF(sqlc.arg(sha256_hex), ''),
    sqlc.arg(storage_driver),
    sqlc.arg(storage_key),
    sqlc.narg(storage_bucket),
    NULLIF(sqlc.arg(etag)::text, ''),
    sqlc.arg(scan_status),
    sqlc.arg(inheritance_enabled)
)
RETURNING *;

-- name: GetDriveFileByPublicIDForTenant :one
SELECT *
FROM file_objects
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL;

-- name: GetDriveFileByIDForTenant :one
SELECT *
FROM file_objects
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL;

-- name: GetDeletedDriveFileByPublicIDForTenant :one
SELECT *
FROM file_objects
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL;

-- name: ListDeletedDriveFiles :many
SELECT *
FROM file_objects
WHERE tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL
ORDER BY deleted_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListDriveChildFiles :many
SELECT *
FROM file_objects
WHERE tenant_id = sqlc.arg(tenant_id)
  AND drive_folder_id IS NOT DISTINCT FROM sqlc.narg(drive_folder_id)::bigint
  AND (
      sqlc.narg(workspace_id)::bigint IS NULL
      OR workspace_id = sqlc.narg(workspace_id)::bigint
  )
  AND purpose = 'drive'
  AND deleted_at IS NULL
ORDER BY original_filename ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: RenameDriveFile :one
UPDATE file_objects
SET
    original_filename = sqlc.arg(original_filename),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: MoveDriveFile :one
UPDATE file_objects
SET
    drive_folder_id = sqlc.narg(drive_folder_id),
    workspace_id = sqlc.narg(workspace_id),
    inheritance_enabled = sqlc.arg(inheritance_enabled),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateDriveFileObjectMetadata :one
UPDATE file_objects
SET
    content_type = sqlc.arg(content_type),
    byte_size = sqlc.arg(byte_size),
    sha256_hex = sqlc.arg(sha256_hex),
    storage_driver = sqlc.arg(storage_driver),
    storage_key = sqlc.arg(storage_key),
    storage_bucket = sqlc.narg(storage_bucket),
    etag = NULLIF(sqlc.arg(etag)::text, ''),
    content_sha256 = NULLIF(sqlc.arg(sha256_hex), ''),
    scan_status = sqlc.arg(scan_status),
    scan_reason = NULL,
    scan_engine = NULL,
    scanned_at = NULL,
    dlp_blocked = false,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: LockDriveFile :one
UPDATE file_objects
SET
    locked_at = now(),
    locked_by_user_id = sqlc.narg(locked_by_user_id),
    lock_reason = sqlc.narg(lock_reason),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: UnlockDriveFile :one
UPDATE file_objects
SET
    locked_at = NULL,
    locked_by_user_id = NULL,
    lock_reason = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDriveFile :one
UPDATE file_objects
SET
    status = 'deleted',
    deleted_at = COALESCE(deleted_at, now()),
    deleted_by_user_id = sqlc.narg(deleted_by_user_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: RestoreDriveFile :one
UPDATE file_objects
SET
    status = 'active',
    drive_folder_id = sqlc.narg(drive_folder_id),
    workspace_id = sqlc.narg(workspace_id),
    deleted_at = NULL,
    deleted_by_user_id = NULL,
    deleted_parent_folder_id = NULL,
    purge_locked_at = NULL,
    purge_locked_by = NULL,
    last_purge_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL
RETURNING *;

-- name: SearchDriveFileCandidates :many
SELECT *
FROM file_objects
WHERE tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
  AND deleted_at IS NULL
  AND (
      sqlc.narg(query)::text IS NULL
      OR original_filename ILIKE '%' || sqlc.narg(query)::text || '%'
  )
  AND (
      sqlc.narg(content_type)::text IS NULL
      OR content_type = sqlc.narg(content_type)::text
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
