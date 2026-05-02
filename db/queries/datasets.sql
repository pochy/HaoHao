-- name: CreateDataset :one
INSERT INTO datasets (
    tenant_id,
    created_by_user_id,
    source_file_object_id,
    source_kind,
    name,
    original_filename,
    content_type,
    byte_size,
    raw_database,
    raw_table,
    work_database
) VALUES (
    $1, $2, $3, 'file', $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: CreateDatasetFromWorkTable :one
INSERT INTO datasets (
    tenant_id,
    created_by_user_id,
    source_kind,
    source_work_table_id,
    name,
    original_filename,
    content_type,
    byte_size,
    raw_database,
    raw_table,
    work_database,
    status,
    row_count,
    imported_at
) VALUES (
    $1, $2, 'work_table', $3, $4, $5, $6, $7, $8, $9, $10, 'pending', $11, NULL
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
    dataset_id,
    requested_by_user_id,
    statement
) VALUES (
    $1, $2, $3, $4
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

-- name: ListDatasetQueryJobsForDataset :many
SELECT *
FROM dataset_query_jobs
WHERE tenant_id = $1
  AND dataset_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: UpsertDatasetWorkTable :one
INSERT INTO dataset_work_tables (
    tenant_id,
    source_dataset_id,
    created_from_query_job_id,
    created_by_user_id,
    work_database,
    work_table,
    display_name,
    row_count,
    total_bytes,
    engine
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (tenant_id, work_database, work_table)
WHERE status = 'active' AND dropped_at IS NULL
DO UPDATE SET
    source_dataset_id = COALESCE(EXCLUDED.source_dataset_id, dataset_work_tables.source_dataset_id),
    created_from_query_job_id = COALESCE(EXCLUDED.created_from_query_job_id, dataset_work_tables.created_from_query_job_id),
    created_by_user_id = COALESCE(EXCLUDED.created_by_user_id, dataset_work_tables.created_by_user_id),
    display_name = EXCLUDED.display_name,
    row_count = EXCLUDED.row_count,
    total_bytes = EXCLUDED.total_bytes,
    engine = EXCLUDED.engine,
    updated_at = now()
RETURNING *;

-- name: ListDatasetWorkTables :many
SELECT *
FROM dataset_work_tables
WHERE tenant_id = $1
ORDER BY updated_at DESC, id DESC
LIMIT $2;

-- name: ListDatasetWorkTablesForDataset :many
SELECT *
FROM dataset_work_tables
WHERE tenant_id = $1
  AND source_dataset_id = $2
  AND status = 'active'
ORDER BY updated_at DESC, id DESC
LIMIT $3;

-- name: GetDatasetWorkTableForTenant :one
SELECT *
FROM dataset_work_tables
WHERE public_id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: GetDatasetWorkTableByIDForTenant :one
SELECT *
FROM dataset_work_tables
WHERE id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: GetActiveDatasetWorkTableByRefForTenant :one
SELECT *
FROM dataset_work_tables
WHERE tenant_id = $1
  AND work_database = $2
  AND work_table = $3
  AND status = 'active'
  AND dropped_at IS NULL
LIMIT 1;

-- name: LinkDatasetWorkTableToDataset :one
UPDATE dataset_work_tables
SET
    source_dataset_id = $3,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'active'
  AND dropped_at IS NULL
RETURNING *;

-- name: RenameDatasetWorkTableRecord :one
UPDATE dataset_work_tables
SET
    work_table = $3,
    display_name = $4,
    row_count = $5,
    total_bytes = $6,
    engine = $7,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'active'
  AND dropped_at IS NULL
RETURNING *;

-- name: UpdateDatasetWorkTableStats :one
UPDATE dataset_work_tables
SET
    row_count = $3,
    total_bytes = $4,
    engine = $5,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
RETURNING *;

-- name: MarkDatasetWorkTableDropped :one
UPDATE dataset_work_tables
SET
    status = 'dropped',
    dropped_at = COALESCE(dropped_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'active'
  AND dropped_at IS NULL
RETURNING *;

-- name: CreateDatasetWorkTableExport :one
INSERT INTO dataset_work_table_exports (
    tenant_id,
    work_table_id,
    requested_by_user_id,
    format,
    expires_at
) VALUES (
    $1, $2, $3, 'csv', $4
)
RETURNING *;

-- name: ListDatasetWorkTableExports :many
SELECT *
FROM dataset_work_table_exports
WHERE tenant_id = $1
  AND work_table_id = $2
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: GetDatasetWorkTableExportForTenant :one
SELECT *
FROM dataset_work_table_exports
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetDatasetWorkTableExportByIDForTenant :one
SELECT *
FROM dataset_work_table_exports
WHERE id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: MarkDatasetWorkTableExportProcessing :one
UPDATE dataset_work_table_exports
SET
    status = 'processing',
    outbox_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkDatasetWorkTableExportReady :one
UPDATE dataset_work_table_exports
SET
    status = 'ready',
    file_object_id = $2,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkDatasetWorkTableExportFailed :one
UPDATE dataset_work_table_exports
SET
    status = 'failed',
    error_summary = left($2, 1000),
    updated_at = now()
WHERE id = $1
RETURNING *;
