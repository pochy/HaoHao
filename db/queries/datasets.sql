-- name: CreateDataset :one
INSERT INTO datasets (
    tenant_id,
    created_by_user_id,
    source_file_object_id,
    name,
    original_filename,
    content_type,
    byte_size,
    raw_database,
    raw_table,
    work_database
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: ListDatasets :many
SELECT *
FROM datasets
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: GetDatasetForTenant :one
SELECT *
FROM datasets
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetDatasetByIDForTenant :one
SELECT *
FROM datasets
WHERE id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: MarkDatasetImporting :one
UPDATE datasets
SET
    status = 'importing',
    error_summary = NULL,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkDatasetReady :one
UPDATE datasets
SET
    status = 'ready',
    row_count = $2,
    error_summary = NULL,
    imported_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkDatasetFailed :one
UPDATE datasets
SET
    status = 'failed',
    error_summary = left($2, 1000),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteDataset :one
UPDATE datasets
SET
    status = 'deleted',
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteDatasetColumns :exec
DELETE FROM dataset_columns
WHERE dataset_id = $1;

-- name: CreateDatasetColumn :one
INSERT INTO dataset_columns (
    dataset_id,
    ordinal,
    original_name,
    column_name,
    clickhouse_type
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: ListDatasetColumns :many
SELECT *
FROM dataset_columns
WHERE dataset_id = $1
ORDER BY ordinal ASC;

-- name: CreateDatasetImportJob :one
INSERT INTO dataset_import_jobs (
    tenant_id,
    dataset_id,
    source_file_object_id,
    requested_by_user_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetDatasetImportJobByIDForTenant :one
SELECT *
FROM dataset_import_jobs
WHERE id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: GetLatestDatasetImportJob :one
SELECT *
FROM dataset_import_jobs
WHERE dataset_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: MarkDatasetImportJobProcessing :one
UPDATE dataset_import_jobs
SET
    status = 'processing',
    outbox_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CompleteDatasetImportJob :one
UPDATE dataset_import_jobs
SET
    status = 'completed',
    total_rows = $2,
    valid_rows = $3,
    invalid_rows = $4,
    error_sample = $5,
    error_summary = $6,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailDatasetImportJob :one
UPDATE dataset_import_jobs
SET
    status = 'failed',
    error_summary = left($2, 1000),
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateDatasetQueryJob :one
INSERT INTO dataset_query_jobs (
    tenant_id,
    requested_by_user_id,
    statement
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: CompleteDatasetQueryJob :one
UPDATE dataset_query_jobs
SET
    status = 'completed',
    result_columns = $2,
    result_rows = $3,
    row_count = $4,
    duration_ms = $5,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailDatasetQueryJob :one
UPDATE dataset_query_jobs
SET
    status = 'failed',
    error_summary = left($2, 1000),
    duration_ms = $3,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetDatasetQueryJobForTenant :one
SELECT *
FROM dataset_query_jobs
WHERE public_id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: ListDatasetQueryJobs :many
SELECT *
FROM dataset_query_jobs
WHERE tenant_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;
