-- name: UpsertMedallionAsset :one
INSERT INTO medallion_assets (
    tenant_id,
    layer,
    resource_kind,
    resource_id,
    resource_public_id,
    display_name,
    status,
    row_count,
    byte_size,
    schema_summary,
    metadata,
    created_by_user_id,
    updated_by_user_id,
    archived_at
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(layer),
    sqlc.arg(resource_kind),
    sqlc.arg(resource_id),
    sqlc.arg(resource_public_id),
    sqlc.arg(display_name),
    sqlc.arg(status),
    sqlc.narg(row_count),
    sqlc.narg(byte_size),
    sqlc.arg(schema_summary),
    sqlc.arg(metadata),
    sqlc.narg(created_by_user_id),
    sqlc.narg(updated_by_user_id),
    sqlc.narg(archived_at)
)
ON CONFLICT (tenant_id, resource_kind, resource_id) DO UPDATE
SET
    layer = EXCLUDED.layer,
    resource_public_id = EXCLUDED.resource_public_id,
    display_name = EXCLUDED.display_name,
    status = EXCLUDED.status,
    row_count = EXCLUDED.row_count,
    byte_size = EXCLUDED.byte_size,
    schema_summary = EXCLUDED.schema_summary,
    metadata = EXCLUDED.metadata,
    updated_by_user_id = COALESCE(EXCLUDED.updated_by_user_id, medallion_assets.updated_by_user_id),
    updated_at = now(),
    archived_at = EXCLUDED.archived_at
RETURNING *;

-- name: GetMedallionAssetByPublicIDForTenant :one
SELECT *
FROM medallion_assets
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
LIMIT 1;

-- name: GetMedallionAssetByResourceForTenant :one
SELECT *
FROM medallion_assets
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_kind = sqlc.arg(resource_kind)
  AND resource_id = sqlc.arg(resource_id)
LIMIT 1;

-- name: ListMedallionAssets :many
SELECT *
FROM medallion_assets
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (
      sqlc.narg(layer)::text IS NULL
      OR layer = sqlc.narg(layer)::text
  )
  AND (
      sqlc.narg(resource_kind)::text IS NULL
      OR resource_kind = sqlc.narg(resource_kind)::text
  )
  AND (
      sqlc.narg(q)::text IS NULL
      OR display_name ILIKE '%' || sqlc.narg(q)::text || '%'
      OR metadata::text ILIKE '%' || sqlc.narg(q)::text || '%'
      OR schema_summary::text ILIKE '%' || sqlc.narg(q)::text || '%'
      OR to_tsvector('simple', display_name || ' ' || metadata::text || ' ' || schema_summary::text)
         @@ websearch_to_tsquery('simple', sqlc.narg(q)::text)
  )
ORDER BY
  CASE
      WHEN sqlc.narg(q)::text IS NULL THEN 0
      ELSE ts_rank_cd(
          to_tsvector('simple', display_name || ' ' || metadata::text || ' ' || schema_summary::text),
          websearch_to_tsquery('simple', sqlc.narg(q)::text)
      )
  END DESC,
  updated_at DESC,
  id DESC
LIMIT sqlc.arg(limit_count);

-- name: UpsertMedallionAssetEdge :one
INSERT INTO medallion_asset_edges (
    tenant_id,
    source_asset_id,
    target_asset_id,
    relation_type,
    metadata
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(source_asset_id),
    sqlc.arg(target_asset_id),
    sqlc.arg(relation_type),
    sqlc.arg(metadata)
)
ON CONFLICT (tenant_id, source_asset_id, target_asset_id, relation_type) DO UPDATE
SET metadata = EXCLUDED.metadata
RETURNING *;

-- name: ListMedallionUpstreamAssets :many
SELECT source.*
FROM medallion_asset_edges edge
JOIN medallion_assets source ON source.id = edge.source_asset_id
WHERE edge.tenant_id = sqlc.arg(tenant_id)
  AND edge.target_asset_id = sqlc.arg(asset_id)
ORDER BY edge.created_at DESC, edge.id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListMedallionDownstreamAssets :many
SELECT target.*
FROM medallion_asset_edges edge
JOIN medallion_assets target ON target.id = edge.target_asset_id
WHERE edge.tenant_id = sqlc.arg(tenant_id)
  AND edge.source_asset_id = sqlc.arg(asset_id)
ORDER BY edge.created_at DESC, edge.id DESC
LIMIT sqlc.arg(limit_count);

-- name: UpsertMedallionPipelineRun :one
INSERT INTO medallion_pipeline_runs (
    tenant_id,
    pipeline_type,
    run_key,
    source_resource_kind,
    source_resource_id,
    source_resource_public_id,
    target_resource_kind,
    target_resource_id,
    target_resource_public_id,
    status,
    runtime,
    trigger_kind,
    retryable,
    error_summary,
    metadata,
    requested_by_user_id,
    started_at,
    completed_at
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_type),
    sqlc.arg(run_key),
    sqlc.narg(source_resource_kind),
    sqlc.narg(source_resource_id),
    sqlc.narg(source_resource_public_id),
    sqlc.narg(target_resource_kind),
    sqlc.narg(target_resource_id),
    sqlc.narg(target_resource_public_id),
    sqlc.arg(status),
    sqlc.arg(runtime),
    sqlc.arg(trigger_kind),
    sqlc.arg(retryable),
    sqlc.narg(error_summary),
    sqlc.arg(metadata),
    sqlc.narg(requested_by_user_id),
    sqlc.narg(started_at),
    sqlc.narg(completed_at)
)
ON CONFLICT (tenant_id, pipeline_type, run_key) DO UPDATE
SET
    source_resource_kind = COALESCE(EXCLUDED.source_resource_kind, medallion_pipeline_runs.source_resource_kind),
    source_resource_id = COALESCE(EXCLUDED.source_resource_id, medallion_pipeline_runs.source_resource_id),
    source_resource_public_id = COALESCE(EXCLUDED.source_resource_public_id, medallion_pipeline_runs.source_resource_public_id),
    target_resource_kind = COALESCE(EXCLUDED.target_resource_kind, medallion_pipeline_runs.target_resource_kind),
    target_resource_id = COALESCE(EXCLUDED.target_resource_id, medallion_pipeline_runs.target_resource_id),
    target_resource_public_id = COALESCE(EXCLUDED.target_resource_public_id, medallion_pipeline_runs.target_resource_public_id),
    status = EXCLUDED.status,
    runtime = EXCLUDED.runtime,
    trigger_kind = EXCLUDED.trigger_kind,
    retryable = EXCLUDED.retryable,
    error_summary = EXCLUDED.error_summary,
    metadata = EXCLUDED.metadata,
    requested_by_user_id = COALESCE(EXCLUDED.requested_by_user_id, medallion_pipeline_runs.requested_by_user_id),
    started_at = COALESCE(medallion_pipeline_runs.started_at, EXCLUDED.started_at),
    completed_at = EXCLUDED.completed_at,
    updated_at = now()
RETURNING *;

-- name: LinkMedallionPipelineRunAsset :one
INSERT INTO medallion_pipeline_run_assets (
    tenant_id,
    pipeline_run_id,
    asset_id,
    role
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_run_id),
    sqlc.arg(asset_id),
    sqlc.arg(role)
)
ON CONFLICT (pipeline_run_id, asset_id, role) DO UPDATE
SET tenant_id = EXCLUDED.tenant_id
RETURNING *;

-- name: ListMedallionPipelineRunsByAsset :many
SELECT run.*
FROM medallion_pipeline_run_assets link
JOIN medallion_pipeline_runs run ON run.id = link.pipeline_run_id
WHERE link.tenant_id = sqlc.arg(tenant_id)
  AND link.asset_id = sqlc.arg(asset_id)
ORDER BY run.created_at DESC, run.id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListMedallionPipelineRunAssetLinks :many
SELECT
    link.role,
    asset.*
FROM medallion_pipeline_run_assets link
JOIN medallion_assets asset ON asset.id = link.asset_id
WHERE link.tenant_id = sqlc.arg(tenant_id)
  AND link.pipeline_run_id = sqlc.arg(pipeline_run_id)
ORDER BY link.role ASC, asset.updated_at DESC, asset.id DESC;

-- name: ListMedallionPipelineRunsByResource :many
SELECT *
FROM medallion_pipeline_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (
      (source_resource_kind = sqlc.arg(resource_kind) AND source_resource_id = sqlc.arg(resource_id))
      OR (target_resource_kind = sqlc.arg(resource_kind) AND target_resource_id = sqlc.arg(resource_id))
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: GetDriveProductExtractionItemByPublicIDForTenant :one
SELECT *
FROM drive_product_extraction_items
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
LIMIT 1;
