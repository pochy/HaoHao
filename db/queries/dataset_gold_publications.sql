-- name: CreateDatasetGoldPublication :one
INSERT INTO dataset_gold_publications (
    tenant_id,
    source_work_table_id,
    created_by_user_id,
    updated_by_user_id,
    display_name,
    description,
    gold_database,
    gold_table,
    status,
    schema_summary,
    refresh_policy
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(source_work_table_id),
    sqlc.narg(created_by_user_id),
    sqlc.narg(updated_by_user_id),
    sqlc.arg(display_name),
    sqlc.arg(description),
    sqlc.arg(gold_database),
    sqlc.arg(gold_table),
    'pending',
    sqlc.arg(schema_summary),
    'manual'
)
RETURNING *;

-- name: ListDatasetGoldPublications :many
SELECT *
FROM dataset_gold_publications
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (
      COALESCE(sqlc.narg(include_archived)::boolean, false)
      OR archived_at IS NULL
  )
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListDatasetGoldPublicationsForWorkTable :many
SELECT *
FROM dataset_gold_publications
WHERE tenant_id = sqlc.arg(tenant_id)
  AND source_work_table_id = sqlc.arg(source_work_table_id)
  AND archived_at IS NULL
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: GetDatasetGoldPublicationForTenant :one
SELECT *
FROM dataset_gold_publications
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
LIMIT 1;

-- name: GetDatasetGoldPublicationByIDForTenant :one
SELECT *
FROM dataset_gold_publications
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
LIMIT 1;

-- name: CreateDatasetGoldPublishRun :one
INSERT INTO dataset_gold_publish_runs (
    tenant_id,
    publication_id,
    source_work_table_id,
    source_data_pipeline_run_id,
    source_data_pipeline_run_output_id,
    requested_by_user_id,
    status,
    gold_database,
    gold_table,
    internal_database,
    internal_table,
    schema_summary
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(publication_id),
    sqlc.arg(source_work_table_id),
    sqlc.narg(source_data_pipeline_run_id),
    sqlc.narg(source_data_pipeline_run_output_id),
    sqlc.narg(requested_by_user_id),
    'pending',
    sqlc.arg(gold_database),
    sqlc.arg(gold_table),
    sqlc.arg(internal_database),
    sqlc.arg(internal_table),
    sqlc.arg(schema_summary)
)
RETURNING *;

-- name: LinkDatasetGoldPublishRunOutboxEvent :one
UPDATE dataset_gold_publish_runs
SET
    outbox_event_id = sqlc.arg(outbox_event_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: GetDatasetGoldPublishRunByIDForTenant :one
SELECT *
FROM dataset_gold_publish_runs
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
LIMIT 1;

-- name: ListDatasetGoldPublishRuns :many
SELECT *
FROM dataset_gold_publish_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND publication_id = sqlc.arg(publication_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: MarkDatasetGoldPublishRunProcessing :one
UPDATE dataset_gold_publish_runs
SET
    status = 'processing',
    outbox_event_id = sqlc.arg(outbox_event_id),
    started_at = COALESCE(started_at, now()),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status IN ('pending', 'processing')
RETURNING *;

-- name: CompleteDatasetGoldPublishRun :one
UPDATE dataset_gold_publish_runs
SET
    status = 'completed',
    row_count = sqlc.arg(row_count),
    total_bytes = sqlc.arg(total_bytes),
    schema_summary = sqlc.arg(schema_summary),
    error_summary = NULL,
    completed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status IN ('pending', 'processing')
RETURNING *;

-- name: FailDatasetGoldPublishRun :one
UPDATE dataset_gold_publish_runs
SET
    status = 'failed',
    error_summary = left(sqlc.arg(error_summary), 1000),
    completed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status IN ('pending', 'processing')
RETURNING *;

-- name: MarkDatasetGoldPublicationActive :one
UPDATE dataset_gold_publications
SET
    status = 'active',
    row_count = sqlc.arg(row_count),
    total_bytes = sqlc.arg(total_bytes),
    schema_summary = sqlc.arg(schema_summary),
    last_publish_run_id = sqlc.arg(last_publish_run_id),
    published_by_user_id = COALESCE(sqlc.narg(published_by_user_id), published_by_user_id),
    updated_by_user_id = COALESCE(sqlc.narg(updated_by_user_id), updated_by_user_id),
    published_at = COALESCE(published_at, now()),
    unpublished_by_user_id = NULL,
    unpublished_at = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND archived_at IS NULL
RETURNING *;

-- name: MarkDatasetGoldPublicationPublishFailed :one
UPDATE dataset_gold_publications
SET
    status = CASE WHEN status = 'active' THEN status ELSE 'failed' END,
    last_publish_run_id = sqlc.arg(last_publish_run_id),
    updated_by_user_id = COALESCE(sqlc.narg(updated_by_user_id), updated_by_user_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND archived_at IS NULL
RETURNING *;

-- name: UnpublishDatasetGoldPublication :one
UPDATE dataset_gold_publications
SET
    status = 'unpublished',
    unpublished_by_user_id = sqlc.narg(unpublished_by_user_id),
    unpublished_at = now(),
    updated_by_user_id = sqlc.narg(updated_by_user_id),
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND archived_at IS NULL
  AND status <> 'archived'
RETURNING *;

-- name: ArchiveDatasetGoldPublication :one
UPDATE dataset_gold_publications
SET
    status = 'archived',
    archived_by_user_id = sqlc.narg(archived_by_user_id),
    archived_at = now(),
    updated_by_user_id = sqlc.narg(updated_by_user_id),
    updated_at = now()
WHERE public_id = sqlc.arg(public_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND archived_at IS NULL
RETURNING *;
