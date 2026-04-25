-- name: CreateCustomerSignalImportJob :one
INSERT INTO customer_signal_import_jobs (
    tenant_id,
    requested_by_user_id,
    input_file_object_id,
    validate_only
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: ListCustomerSignalImportJobs :many
SELECT *
FROM customer_signal_import_jobs
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: GetCustomerSignalImportJobForTenant :one
SELECT *
FROM customer_signal_import_jobs
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetCustomerSignalImportJobByIDForTenant :one
SELECT *
FROM customer_signal_import_jobs
WHERE id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: MarkCustomerSignalImportJobProcessing :one
UPDATE customer_signal_import_jobs
SET
    status = 'processing',
    outbox_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CompleteCustomerSignalImportJob :one
UPDATE customer_signal_import_jobs
SET
    status = 'completed',
    total_rows = $2,
    valid_rows = $3,
    invalid_rows = $4,
    inserted_rows = $5,
    error_file_object_id = $6,
    error_summary = $7,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailCustomerSignalImportJob :one
UPDATE customer_signal_import_jobs
SET
    status = 'failed',
    error_summary = left($2, 1000),
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;
