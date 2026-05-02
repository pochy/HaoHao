-- name: CreateFileObject :one
INSERT INTO file_objects (
    tenant_id,
    uploaded_by_user_id,
    purpose,
    attached_to_type,
    attached_to_id,
    original_filename,
    content_type,
    byte_size,
    sha256_hex,
    storage_driver,
    storage_key,
    storage_bucket,
    content_sha256,
    etag
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, sqlc.narg(storage_bucket), NULLIF($9, ''), NULLIF(sqlc.arg(etag)::text, '')
)
RETURNING *;

-- name: GetFileObjectForTenant :one
SELECT *
FROM file_objects
WHERE public_id = $1
  AND tenant_id = $2
  AND purpose <> 'drive'
  AND deleted_at IS NULL;

-- name: GetFileObjectByIDForTenant :one
SELECT *
FROM file_objects
WHERE id = $1
  AND tenant_id = $2
  AND purpose <> 'drive'
  AND deleted_at IS NULL;

-- name: GetActiveFileObjectByIDForTenant :one
SELECT *
FROM file_objects
WHERE id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;

-- name: ListFileObjectsForAttachment :many
SELECT *
FROM file_objects
WHERE tenant_id = $1
  AND attached_to_type = $2
  AND attached_to_id = $3
  AND purpose <> 'drive'
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: ListActiveFileObjectsForTenant :many
SELECT *
FROM file_objects
WHERE tenant_id = $1
  AND purpose <> 'drive'
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: SoftDeleteFileObjectForTenant :one
UPDATE file_objects
SET
    status = 'deleted',
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND purpose <> 'drive'
  AND deleted_at IS NULL
RETURNING *;

-- name: ClaimDeletedFileObjectsForPurge :many
UPDATE file_objects
SET
    purge_locked_at = now(),
    purge_locked_by = sqlc.arg(worker_id)::text,
    purge_attempts = purge_attempts + 1,
    updated_at = now()
WHERE id IN (
    SELECT id
    FROM file_objects
    WHERE storage_driver = ANY(sqlc.arg(storage_drivers)::text[])
      AND status = 'deleted'
      AND deleted_at IS NOT NULL
      AND deleted_at < sqlc.arg(cutoff)::timestamptz
      AND purged_at IS NULL
      AND (
          purge_locked_at IS NULL
          OR purge_locked_at < now() - sqlc.arg(lock_timeout)::interval
      )
    ORDER BY deleted_at, id
    LIMIT sqlc.arg(batch_size)::int
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkFileObjectBodyPurged :one
UPDATE file_objects
SET
    purged_at = now(),
    purge_locked_at = NULL,
    purge_locked_by = NULL,
    last_purge_error = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'deleted'
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL
RETURNING *;

-- name: MarkFileObjectPurgeFailed :one
UPDATE file_objects
SET
    purge_locked_at = NULL,
    purge_locked_by = NULL,
    last_purge_error = left(sqlc.arg(last_error)::text, 1000),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND purged_at IS NULL
RETURNING *;
