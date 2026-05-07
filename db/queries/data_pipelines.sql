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

-- name: UpsertDataPipelineSchemaColumn :one
INSERT INTO data_pipeline_schema_columns (
    tenant_id,
    domain,
    schema_type,
    target_column,
    description,
    aliases,
    examples,
    language,
    version
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(domain),
    sqlc.arg(schema_type),
    sqlc.arg(target_column),
    sqlc.arg(description),
    COALESCE(sqlc.narg(aliases)::jsonb, '[]'::jsonb),
    COALESCE(sqlc.narg(examples)::jsonb, '[]'::jsonb),
    sqlc.arg(language),
    sqlc.arg(version)
)
ON CONFLICT (tenant_id, domain, schema_type, target_column, version) DO UPDATE
SET
    description = EXCLUDED.description,
    aliases = EXCLUDED.aliases,
    examples = EXCLUDED.examples,
    language = EXCLUDED.language,
    archived_at = NULL,
    updated_at = now()
RETURNING *;

-- name: GetDataPipelineSchemaColumnByIDForTenant :one
SELECT *
FROM data_pipeline_schema_columns
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
  AND archived_at IS NULL
LIMIT 1;

-- name: GetDataPipelineSchemaColumnByPublicIDForTenant :one
SELECT *
FROM data_pipeline_schema_columns
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
  AND archived_at IS NULL
LIMIT 1;

-- name: ListDataPipelineSchemaColumnsForIndex :many
SELECT *
FROM data_pipeline_schema_columns
WHERE tenant_id = sqlc.arg(tenant_id)
  AND archived_at IS NULL
ORDER BY id
LIMIT sqlc.arg(limit_count);

-- name: ListDataPipelineMappingExamplesForIndex :many
SELECT
    e.id,
    e.public_id,
    e.tenant_id,
    e.pipeline_id,
    e.version_id,
    e.schema_column_id,
    e.source_column,
    e.sheet_name,
    e.sample_values,
    e.neighbor_columns,
    e.decision,
    e.decided_by_user_id,
    e.decided_at,
    e.shared_scope,
    e.shared_by_user_id,
    e.shared_at,
    e.created_at,
    e.updated_at,
    c.target_column
FROM data_pipeline_mapping_examples e
JOIN data_pipeline_schema_columns c
  ON c.tenant_id = e.tenant_id
 AND c.id = e.schema_column_id
 AND c.archived_at IS NULL
WHERE e.tenant_id = sqlc.arg(tenant_id)
ORDER BY e.id
LIMIT sqlc.arg(limit_count);

-- name: ListTenantAdminDataPipelineMappingExamples :many
SELECT
    e.public_id,
    e.source_column,
    e.sheet_name,
    e.sample_values,
    e.neighbor_columns,
    e.decision,
    e.shared_scope,
    e.decided_at,
    e.shared_at,
    e.created_at,
    e.updated_at,
    p.public_id AS pipeline_public_id,
    p.name AS pipeline_name,
    c.public_id AS schema_column_public_id,
    c.domain,
    c.schema_type,
    c.target_column,
    EXISTS (
        SELECT 1
        FROM local_search_documents d
        WHERE d.tenant_id = e.tenant_id
          AND d.resource_kind = 'mapping_example'
          AND d.resource_id = e.id
    ) AS search_document_materialized
FROM data_pipeline_mapping_examples e
JOIN data_pipelines p
  ON p.tenant_id = e.tenant_id
 AND p.id = e.pipeline_id
JOIN data_pipeline_schema_columns c
  ON c.tenant_id = e.tenant_id
 AND c.id = e.schema_column_id
WHERE e.tenant_id = sqlc.arg(tenant_id)
  AND (
      sqlc.narg(shared_scope)::text IS NULL
      OR e.shared_scope = sqlc.narg(shared_scope)::text
  )
  AND (
      sqlc.narg(decision)::text IS NULL
      OR e.decision = sqlc.narg(decision)::text
  )
  AND (
      sqlc.narg(query)::text IS NULL
      OR e.source_column ILIKE '%' || sqlc.narg(query)::text || '%'
      OR c.target_column ILIKE '%' || sqlc.narg(query)::text || '%'
      OR p.name ILIKE '%' || sqlc.narg(query)::text || '%'
      OR c.domain ILIKE '%' || sqlc.narg(query)::text || '%'
      OR c.schema_type ILIKE '%' || sqlc.narg(query)::text || '%'
  )
ORDER BY e.updated_at DESC, e.id DESC
LIMIT sqlc.arg(limit_count);

-- name: SearchDataPipelineSchemaMappingCandidates :many
SELECT
    c.id,
    c.public_id,
    c.target_column,
    c.description,
    c.aliases,
    c.examples,
    d.public_id AS document_public_id,
    d.snippet,
    ts_rank_cd(d.search_vector, websearch_to_tsquery('simple', sqlc.arg(query)::text))::float8 AS keyword_score
FROM data_pipeline_schema_columns c
JOIN local_search_documents d
  ON d.tenant_id = c.tenant_id
 AND d.resource_kind = 'schema_column'
 AND d.resource_id = c.id
WHERE c.tenant_id = sqlc.arg(tenant_id)
  AND c.archived_at IS NULL
  AND (
      sqlc.narg(domain)::text IS NULL
      OR c.domain = sqlc.narg(domain)::text
  )
  AND (
      sqlc.narg(schema_type)::text IS NULL
      OR c.schema_type = sqlc.narg(schema_type)::text
  )
  AND (
      d.search_vector @@ websearch_to_tsquery('simple', sqlc.arg(query)::text)
      OR d.title ILIKE '%' || sqlc.arg(query)::text || '%'
      OR d.body_text ILIKE '%' || sqlc.arg(query)::text || '%'
  )
ORDER BY keyword_score DESC, c.updated_at DESC, c.id DESC
LIMIT sqlc.arg(limit_count);

-- name: CountDataPipelineMappingEvidence :many
SELECT
    schema_column_id,
    decision,
    count(*)::bigint AS evidence_count
FROM data_pipeline_mapping_examples
WHERE tenant_id = sqlc.arg(tenant_id)
  AND schema_column_id = ANY(sqlc.arg(schema_column_ids)::bigint[])
  AND (
      shared_scope = 'tenant'
      OR pipeline_id = sqlc.narg(pipeline_id)::bigint
  )
GROUP BY schema_column_id, decision;

-- name: GetDataPipelineMappingExampleByPublicIDForTenant :one
SELECT
    e.*,
    c.public_id AS schema_column_public_id,
    c.target_column
FROM data_pipeline_mapping_examples e
JOIN data_pipeline_schema_columns c
  ON c.tenant_id = e.tenant_id
 AND c.id = e.schema_column_id
WHERE e.tenant_id = sqlc.arg(tenant_id)
  AND e.public_id = sqlc.arg(public_id)
LIMIT 1;

-- name: UpdateDataPipelineMappingExampleSharing :one
UPDATE data_pipeline_mapping_examples
SET
    shared_scope = sqlc.arg(shared_scope),
    shared_by_user_id = CASE
        WHEN sqlc.arg(shared_scope)::text = 'tenant' THEN sqlc.narg(shared_by_user_id)::bigint
        ELSE NULL
    END,
    shared_at = CASE
        WHEN sqlc.arg(shared_scope)::text = 'tenant' THEN now()
        ELSE NULL
    END,
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
RETURNING *;

-- name: UpsertDataPipelineMappingExample :one
INSERT INTO data_pipeline_mapping_examples (
    tenant_id,
    pipeline_id,
    version_id,
    schema_column_id,
    source_column,
    sheet_name,
    sample_values,
    neighbor_columns,
    decision,
    decided_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_id),
    sqlc.narg(version_id),
    sqlc.arg(schema_column_id),
    sqlc.arg(source_column),
    sqlc.arg(sheet_name),
    COALESCE(sqlc.arg(sample_values)::jsonb, '[]'::jsonb),
    COALESCE(sqlc.arg(neighbor_columns)::jsonb, '[]'::jsonb),
    sqlc.arg(decision),
    sqlc.narg(decided_by_user_id)
)
ON CONFLICT (tenant_id, pipeline_id, version_id, source_column, schema_column_id, decision)
WHERE version_id IS NOT NULL
DO UPDATE
SET
    sheet_name = EXCLUDED.sheet_name,
    sample_values = EXCLUDED.sample_values,
    neighbor_columns = EXCLUDED.neighbor_columns,
    decided_by_user_id = EXCLUDED.decided_by_user_id,
    decided_at = now(),
    updated_at = now()
RETURNING *;

-- name: UpsertDataPipelineMappingExampleWithoutVersion :one
INSERT INTO data_pipeline_mapping_examples (
    tenant_id,
    pipeline_id,
    schema_column_id,
    source_column,
    sheet_name,
    sample_values,
    neighbor_columns,
    decision,
    decided_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_id),
    sqlc.arg(schema_column_id),
    sqlc.arg(source_column),
    sqlc.arg(sheet_name),
    COALESCE(sqlc.arg(sample_values)::jsonb, '[]'::jsonb),
    COALESCE(sqlc.arg(neighbor_columns)::jsonb, '[]'::jsonb),
    sqlc.arg(decision),
    sqlc.narg(decided_by_user_id)
)
ON CONFLICT (tenant_id, pipeline_id, source_column, schema_column_id, decision)
WHERE version_id IS NULL
DO UPDATE
SET
    sheet_name = EXCLUDED.sheet_name,
    sample_values = EXCLUDED.sample_values,
    neighbor_columns = EXCLUDED.neighbor_columns,
    decided_by_user_id = EXCLUDED.decided_by_user_id,
    decided_at = now(),
    updated_at = now()
RETURNING *;
