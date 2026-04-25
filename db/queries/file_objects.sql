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
    storage_key
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetFileObjectForTenant :one
SELECT *
FROM file_objects
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;

-- name: GetFileObjectByIDForTenant :one
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
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: ListActiveFileObjectsForTenant :many
SELECT *
FROM file_objects
WHERE tenant_id = $1
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
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDeletedFileObjectsBefore :execrows
UPDATE file_objects
SET updated_at = now()
WHERE deleted_at IS NOT NULL
  AND deleted_at < $1;
