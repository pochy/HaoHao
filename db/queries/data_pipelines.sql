-- name: CreateDataPipeline :one
INSERT INTO data_pipelines (
    tenant_id,
    created_by_user_id,
    updated_by_user_id,
    name,
    description
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(created_by_user_id),
    sqlc.arg(updated_by_user_id),
    sqlc.arg(name),
    sqlc.arg(description)
)
RETURNING *;

-- name: ListDataPipelines :many
WITH base AS (
    SELECT
        p.*,
        latest_run.public_id AS latest_run_public_id,
        latest_run.status AS latest_run_status,
        latest_run.created_at AS latest_run_at,
        COALESCE(schedule_summary.enabled_schedule_count, 0)::bigint AS enabled_schedule_count,
        COALESCE(schedule_summary.disabled_schedule_count, 0)::bigint AS disabled_schedule_count,
        schedule_summary.next_run_at,
        CASE
            WHEN COALESCE(schedule_summary.enabled_schedule_count, 0) > 0 THEN 'enabled'
            WHEN COALESCE(schedule_summary.disabled_schedule_count, 0) > 0 THEN 'disabled'
            ELSE 'none'
        END AS schedule_state
    FROM data_pipelines p
    LEFT JOIN LATERAL (
        SELECT public_id, status, created_at
        FROM data_pipeline_runs r
        WHERE r.tenant_id = p.tenant_id
          AND r.pipeline_id = p.id
        ORDER BY r.created_at DESC, r.id DESC
        LIMIT 1
    ) latest_run ON TRUE
    LEFT JOIN LATERAL (
        SELECT
            COUNT(*) FILTER (WHERE enabled)::bigint AS enabled_schedule_count,
            COUNT(*) FILTER (WHERE NOT enabled)::bigint AS disabled_schedule_count,
            MIN(next_run_at) FILTER (WHERE enabled) AS next_run_at
        FROM data_pipeline_schedules s
        WHERE s.tenant_id = p.tenant_id
          AND s.pipeline_id = p.id
    ) schedule_summary ON TRUE
    WHERE p.tenant_id = sqlc.arg(tenant_id)
      AND p.archived_at IS NULL
)
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    updated_by_user_id,
    name,
    description,
    status,
    published_version_id,
    created_at,
    updated_at,
    archived_at,
    COALESCE(latest_run_public_id::text, '') AS latest_run_public_id,
    COALESCE(latest_run_status, '') AS latest_run_status,
    latest_run_at,
    schedule_state,
    enabled_schedule_count,
    disabled_schedule_count,
    next_run_at::timestamptz AS next_run_at
FROM base
WHERE (
    sqlc.narg(q)::text IS NULL
    OR btrim(sqlc.narg(q)::text) = ''
    OR name ILIKE '%' || sqlc.narg(q)::text || '%'
    OR description ILIKE '%' || sqlc.narg(q)::text || '%'
    OR public_id::text ILIKE '%' || sqlc.narg(q)::text || '%'
)
AND (
    sqlc.narg(status)::text IS NULL
    OR status = sqlc.narg(status)::text
)
AND (
    sqlc.arg(publication)::text = 'all'
    OR (sqlc.arg(publication)::text = 'published' AND published_version_id IS NOT NULL)
    OR (sqlc.arg(publication)::text = 'unpublished' AND published_version_id IS NULL)
)
AND (
    sqlc.narg(run_status)::text IS NULL
    OR latest_run_status = sqlc.narg(run_status)::text
)
AND (
    sqlc.arg(schedule_state_filter)::text = 'all'
    OR schedule_state = sqlc.arg(schedule_state_filter)::text
)
AND (
    sqlc.narg(cursor_id)::bigint IS NULL
    OR (
        sqlc.arg(sort_key)::text = 'updated_desc'
        AND (updated_at, id) < (sqlc.narg(cursor_time)::timestamptz, sqlc.narg(cursor_id)::bigint)
    )
    OR (
        sqlc.arg(sort_key)::text = 'updated_asc'
        AND (updated_at, id) > (sqlc.narg(cursor_time)::timestamptz, sqlc.narg(cursor_id)::bigint)
    )
    OR (
        sqlc.arg(sort_key)::text = 'created_desc'
        AND (created_at, id) < (sqlc.narg(cursor_time)::timestamptz, sqlc.narg(cursor_id)::bigint)
    )
    OR (
        sqlc.arg(sort_key)::text = 'created_asc'
        AND (created_at, id) > (sqlc.narg(cursor_time)::timestamptz, sqlc.narg(cursor_id)::bigint)
    )
    OR (
        sqlc.arg(sort_key)::text = 'name_asc'
        AND (lower(name), id) > (sqlc.narg(cursor_text)::text, sqlc.narg(cursor_id)::bigint)
    )
    OR (
        sqlc.arg(sort_key)::text = 'name_desc'
        AND (lower(name), id) < (sqlc.narg(cursor_text)::text, sqlc.narg(cursor_id)::bigint)
    )
    OR (
        sqlc.arg(sort_key)::text = 'latest_run_desc'
        AND (COALESCE(latest_run_at, '0001-01-02'::timestamptz), id) < (sqlc.narg(cursor_time)::timestamptz, sqlc.narg(cursor_id)::bigint)
    )
)
ORDER BY
    CASE WHEN sqlc.arg(sort_key)::text = 'updated_desc' THEN updated_at END DESC,
    CASE WHEN sqlc.arg(sort_key)::text = 'updated_asc' THEN updated_at END ASC,
    CASE WHEN sqlc.arg(sort_key)::text = 'created_desc' THEN created_at END DESC,
    CASE WHEN sqlc.arg(sort_key)::text = 'created_asc' THEN created_at END ASC,
    CASE WHEN sqlc.arg(sort_key)::text = 'name_asc' THEN lower(name) END ASC,
    CASE WHEN sqlc.arg(sort_key)::text = 'name_desc' THEN lower(name) END DESC,
    CASE WHEN sqlc.arg(sort_key)::text = 'latest_run_desc' THEN COALESCE(latest_run_at, '0001-01-02'::timestamptz) END DESC,
    CASE WHEN sqlc.arg(sort_key)::text IN ('updated_desc', 'created_desc', 'name_desc', 'latest_run_desc') THEN id END DESC,
    CASE WHEN sqlc.arg(sort_key)::text IN ('updated_asc', 'created_asc', 'name_asc') THEN id END ASC
LIMIT sqlc.arg(result_limit);

-- name: GetDataPipelineForTenant :one
SELECT *
FROM data_pipelines
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND archived_at IS NULL
LIMIT 1;

-- name: GetDataPipelineByIDForTenant :one
SELECT *
FROM data_pipelines
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
  AND archived_at IS NULL
LIMIT 1;

-- name: UpdateDataPipeline :one
UPDATE data_pipelines
SET
    name = sqlc.arg(name),
    description = sqlc.arg(description),
    updated_by_user_id = sqlc.arg(updated_by_user_id),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND archived_at IS NULL
RETURNING *;

-- name: ArchiveDataPipeline :one
UPDATE data_pipelines
SET
    status = 'archived',
    updated_by_user_id = sqlc.arg(updated_by_user_id),
    updated_at = now(),
    archived_at = COALESCE(archived_at, now())
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND archived_at IS NULL
RETURNING *;

-- name: CreateDataPipelineVersion :one
INSERT INTO data_pipeline_versions (
    tenant_id,
    pipeline_id,
    version_number,
    status,
    graph,
    validation_summary,
    created_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_id),
    (
        SELECT COALESCE(MAX(version_number), 0) + 1
        FROM data_pipeline_versions
        WHERE pipeline_id = sqlc.arg(pipeline_id)
    ),
    'draft',
    sqlc.arg(graph),
    sqlc.arg(validation_summary),
    sqlc.arg(created_by_user_id)
)
RETURNING *;

-- name: GetDataPipelineVersionForTenant :one
SELECT *
FROM data_pipeline_versions
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
LIMIT 1;

-- name: GetDataPipelineVersionByIDForTenant :one
SELECT *
FROM data_pipeline_versions
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
LIMIT 1;

-- name: ListDataPipelineVersions :many
SELECT *
FROM data_pipeline_versions
WHERE tenant_id = sqlc.arg(tenant_id)
  AND pipeline_id = sqlc.arg(pipeline_id)
ORDER BY version_number DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: PublishDataPipelineVersion :one
UPDATE data_pipeline_versions
SET
    status = 'published',
    validation_summary = sqlc.arg(validation_summary),
    published_by_user_id = sqlc.arg(published_by_user_id),
    published_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND status <> 'archived'
RETURNING *;

-- name: ArchivePublishedDataPipelineVersionsExcept :exec
UPDATE data_pipeline_versions
SET status = 'archived'
WHERE tenant_id = sqlc.arg(tenant_id)
  AND pipeline_id = sqlc.arg(pipeline_id)
  AND id <> sqlc.arg(version_id)
  AND status = 'published';

-- name: SetDataPipelinePublishedVersion :one
UPDATE data_pipelines
SET
    status = 'published',
    published_version_id = sqlc.arg(version_id),
    updated_by_user_id = sqlc.arg(updated_by_user_id),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(pipeline_id)
  AND archived_at IS NULL
RETURNING *;

-- name: CreateDataPipelineRun :one
INSERT INTO data_pipeline_runs (
    tenant_id,
    pipeline_id,
    version_id,
    schedule_id,
    requested_by_user_id,
    trigger_kind
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_id),
    sqlc.arg(version_id),
    sqlc.narg(schedule_id),
    sqlc.narg(requested_by_user_id),
    sqlc.arg(trigger_kind)
)
RETURNING *;

-- name: SetDataPipelineRunOutboxEvent :one
UPDATE data_pipeline_runs
SET
    outbox_event_id = sqlc.arg(outbox_event_id),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: GetDataPipelineRunForTenant :one
SELECT *
FROM data_pipeline_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
LIMIT 1;

-- name: GetDataPipelineRunByIDForTenant :one
SELECT *
FROM data_pipeline_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
LIMIT 1;

-- name: ListDataPipelineRuns :many
SELECT *
FROM data_pipeline_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND pipeline_id = sqlc.arg(pipeline_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: MarkDataPipelineRunProcessing :one
UPDATE data_pipeline_runs
SET
    status = 'processing',
    started_at = COALESCE(started_at, now()),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
  AND status IN ('pending', 'processing')
RETURNING *;

-- name: CompleteDataPipelineRun :one
UPDATE data_pipeline_runs
SET
    status = 'completed',
    output_work_table_id = sqlc.narg(output_work_table_id),
    row_count = sqlc.arg(row_count),
    error_summary = NULL,
    completed_at = now(),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: FailDataPipelineRun :one
UPDATE data_pipeline_runs
SET
    status = 'failed',
    error_summary = left(sqlc.arg(error_summary), 1000),
    completed_at = now(),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: FinishDataPipelineRun :one
UPDATE data_pipeline_runs
SET
    status = sqlc.arg(status),
    output_work_table_id = sqlc.narg(output_work_table_id),
    row_count = sqlc.arg(row_count),
    error_summary = NULLIF(left(sqlc.arg(error_summary), 1000), ''),
    completed_at = now(),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: CreateDataPipelineRunOutput :one
INSERT INTO data_pipeline_run_outputs (
    tenant_id,
    run_id,
    node_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(run_id),
    sqlc.arg(node_id)
)
ON CONFLICT (run_id, node_id) DO UPDATE
SET updated_at = now()
RETURNING *;

-- name: ListDataPipelineRunOutputs :many
SELECT *
FROM data_pipeline_run_outputs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
ORDER BY id ASC;

-- name: MarkDataPipelineRunOutputProcessing :one
UPDATE data_pipeline_run_outputs
SET
    status = 'processing',
    started_at = COALESCE(started_at, now()),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
  AND node_id = sqlc.arg(node_id)
RETURNING *;

-- name: CompleteDataPipelineRunOutput :one
UPDATE data_pipeline_run_outputs
SET
    status = 'completed',
    output_work_table_id = sqlc.arg(output_work_table_id),
    row_count = sqlc.arg(row_count),
    error_summary = NULL,
    metadata = sqlc.arg(metadata),
    completed_at = now(),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
  AND node_id = sqlc.arg(node_id)
RETURNING *;

-- name: FailDataPipelineRunOutput :one
UPDATE data_pipeline_run_outputs
SET
    status = 'failed',
    error_summary = left(sqlc.arg(error_summary), 1000),
    metadata = sqlc.arg(metadata),
    completed_at = now(),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
  AND node_id = sqlc.arg(node_id)
RETURNING *;

-- name: CreateDataPipelineRunStep :one
INSERT INTO data_pipeline_run_steps (
    tenant_id,
    run_id,
    node_id,
    step_type
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(run_id),
    sqlc.arg(node_id),
    sqlc.arg(step_type)
)
ON CONFLICT (run_id, node_id) DO UPDATE
SET
    step_type = EXCLUDED.step_type,
    updated_at = now()
RETURNING *;

-- name: ListDataPipelineRunSteps :many
SELECT *
FROM data_pipeline_run_steps
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
ORDER BY id ASC;

-- name: MarkDataPipelineRunStepProcessing :one
UPDATE data_pipeline_run_steps
SET
    status = 'processing',
    started_at = COALESCE(started_at, now()),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
  AND node_id = sqlc.arg(node_id)
RETURNING *;

-- name: CompleteDataPipelineRunStep :one
UPDATE data_pipeline_run_steps
SET
    status = 'completed',
    row_count = sqlc.arg(row_count),
    error_summary = NULL,
    metadata = sqlc.arg(metadata),
    completed_at = now(),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
  AND node_id = sqlc.arg(node_id)
RETURNING *;

-- name: FailDataPipelineRunStep :one
UPDATE data_pipeline_run_steps
SET
    status = 'failed',
    error_summary = left(sqlc.arg(error_summary), 1000),
    error_sample = sqlc.arg(error_sample),
    completed_at = now(),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND run_id = sqlc.arg(run_id)
  AND node_id = sqlc.arg(node_id)
RETURNING *;

-- name: CreateDataPipelineSchedule :one
INSERT INTO data_pipeline_schedules (
    tenant_id,
    pipeline_id,
    version_id,
    created_by_user_id,
    frequency,
    timezone,
    run_time,
    weekday,
    month_day,
    enabled,
    next_run_at
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_id),
    sqlc.arg(version_id),
    sqlc.arg(created_by_user_id),
    sqlc.arg(frequency),
    sqlc.arg(timezone),
    sqlc.arg(run_time),
    sqlc.narg(weekday),
    sqlc.narg(month_day),
    sqlc.arg(enabled),
    sqlc.arg(next_run_at)
)
RETURNING *;

-- name: GetDataPipelineScheduleForTenant :one
SELECT *
FROM data_pipeline_schedules
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
LIMIT 1;

-- name: ListDataPipelineSchedules :many
SELECT *
FROM data_pipeline_schedules
WHERE tenant_id = sqlc.arg(tenant_id)
  AND pipeline_id = sqlc.arg(pipeline_id)
ORDER BY updated_at DESC, id DESC;

-- name: UpdateDataPipelineSchedule :one
UPDATE data_pipeline_schedules
SET
    frequency = sqlc.arg(frequency),
    timezone = sqlc.arg(timezone),
    run_time = sqlc.arg(run_time),
    weekday = sqlc.narg(weekday),
    month_day = sqlc.narg(month_day),
    enabled = sqlc.arg(enabled),
    next_run_at = sqlc.arg(next_run_at),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
RETURNING *;

-- name: DisableDataPipelineSchedule :one
UPDATE data_pipeline_schedules
SET
    enabled = false,
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
RETURNING *;

-- name: ClaimDueDataPipelineSchedules :many
SELECT *
FROM data_pipeline_schedules
WHERE enabled
  AND next_run_at <= sqlc.arg(now)::timestamptz
ORDER BY next_run_at, id
LIMIT sqlc.arg(batch_limit)
FOR UPDATE SKIP LOCKED;

-- name: CountActiveDataPipelineRunsForSchedule :one
SELECT count(*)::bigint
FROM data_pipeline_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND schedule_id = sqlc.arg(schedule_id)
  AND status IN ('pending', 'processing');

-- name: MarkDataPipelineScheduleCreated :one
UPDATE data_pipeline_schedules
SET
    last_run_at = sqlc.arg(last_run_at),
    last_status = 'created',
    last_error_summary = NULL,
    last_run_id = sqlc.arg(last_run_id),
    next_run_at = sqlc.arg(next_run_at),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: MarkDataPipelineScheduleSkipped :one
UPDATE data_pipeline_schedules
SET
    last_run_at = sqlc.arg(last_run_at),
    last_status = 'skipped',
    last_error_summary = left(sqlc.arg(error_summary), 1000),
    next_run_at = sqlc.arg(next_run_at),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: MarkDataPipelineScheduleFailed :one
UPDATE data_pipeline_schedules
SET
    enabled = sqlc.arg(enabled),
    last_run_at = sqlc.arg(last_run_at),
    last_status = sqlc.arg(last_status),
    last_error_summary = left(sqlc.arg(error_summary), 1000),
    next_run_at = sqlc.arg(next_run_at),
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
RETURNING *;
