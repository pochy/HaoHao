-- name: CreateTenantDataExport :one
INSERT INTO tenant_data_exports (
    tenant_id,
    requested_by_user_id,
    format,
    expires_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateTenantDataExportFormat :one
UPDATE tenant_data_exports
SET
    format = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListTenantDataExports :many
SELECT *
FROM tenant_data_exports
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: GetTenantDataExportForTenant :one
SELECT *
FROM tenant_data_exports
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;

-- name: GetTenantDataExportByIDForTenant :one
SELECT *
FROM tenant_data_exports
WHERE id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;

-- name: MarkTenantDataExportProcessing :one
UPDATE tenant_data_exports
SET
    status = 'processing',
    outbox_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkTenantDataExportReady :one
UPDATE tenant_data_exports
SET
    status = 'ready',
    file_object_id = $2,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkTenantDataExportFailed :one
UPDATE tenant_data_exports
SET
    status = 'failed',
    error_summary = left($2, 1000),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteExpiredTenantDataExports :execrows
UPDATE tenant_data_exports
SET
    status = 'deleted',
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE expires_at <= now()
  AND deleted_at IS NULL;
