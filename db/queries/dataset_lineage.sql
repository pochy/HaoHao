-- name: CreateDatasetLineageChangeSet :one
INSERT INTO dataset_lineage_change_sets (
    tenant_id,
    query_job_id,
    root_resource_type,
    root_resource_public_id,
    source_kind,
    title,
    description,
    created_by_user_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetDatasetLineageChangeSetByPublicID :one
SELECT *
FROM dataset_lineage_change_sets
WHERE public_id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: ListDatasetLineageChangeSets :many
SELECT *
FROM dataset_lineage_change_sets
WHERE tenant_id = $1
  AND ($2::text = '' OR status = $2)
ORDER BY updated_at DESC, id DESC
LIMIT $3;

-- name: ArchivePublishedDatasetLineageChangeSetsForRoot :exec
UPDATE dataset_lineage_change_sets
SET
    status = 'archived',
    archived_at = now(),
    updated_at = now()
WHERE tenant_id = $1
  AND source_kind = $2
  AND root_resource_type = $3
  AND root_resource_public_id IS NOT DISTINCT FROM $4
  AND status = 'published';

-- name: PublishDatasetLineageChangeSet :one
UPDATE dataset_lineage_change_sets
SET
    status = 'published',
    published_by_user_id = $3,
    published_at = now(),
    rejected_by_user_id = NULL,
    rejected_at = NULL,
    archived_at = NULL,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'draft'
RETURNING *;

-- name: RejectDatasetLineageChangeSet :one
UPDATE dataset_lineage_change_sets
SET
    status = 'rejected',
    rejected_by_user_id = $3,
    rejected_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'draft'
RETURNING *;

-- name: DeleteDatasetLineageEdgesByChangeSet :exec
DELETE FROM dataset_lineage_edges
WHERE change_set_id = $1;

-- name: DeleteDatasetLineageNodesByChangeSet :exec
DELETE FROM dataset_lineage_nodes
WHERE change_set_id = $1;

-- name: CreateDatasetLineageNode :one
INSERT INTO dataset_lineage_nodes (
    tenant_id,
    change_set_id,
    node_key,
    node_kind,
    source_kind,
    resource_type,
    resource_public_id,
    parent_node_key,
    column_name,
    label,
    description,
    position_x,
    position_y,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: CreateDatasetLineageEdge :one
INSERT INTO dataset_lineage_edges (
    tenant_id,
    change_set_id,
    edge_key,
    source_node_key,
    target_node_key,
    relation_type,
    source_kind,
    confidence,
    label,
    description,
    expression,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: ListDatasetLineageNodesForChangeSet :many
SELECT *
FROM dataset_lineage_nodes
WHERE tenant_id = $1
  AND change_set_id = $2
ORDER BY id ASC;

-- name: ListDatasetLineageEdgesForChangeSet :many
SELECT *
FROM dataset_lineage_edges
WHERE tenant_id = $1
  AND change_set_id = $2
ORDER BY id ASC;

-- name: ListDatasetLineageNodesForChangeSets :many
SELECT n.*
FROM dataset_lineage_nodes n
JOIN dataset_lineage_change_sets c ON c.id = n.change_set_id
WHERE n.tenant_id = $1
  AND c.status = ANY($2::text[])
ORDER BY n.id ASC
LIMIT $3;

-- name: ListDatasetLineageEdgesForChangeSets :many
SELECT e.*
FROM dataset_lineage_edges e
JOIN dataset_lineage_change_sets c ON c.id = e.change_set_id
WHERE e.tenant_id = $1
  AND c.status = ANY($2::text[])
ORDER BY e.id ASC
LIMIT $3;

-- name: CreateDatasetLineageParseRun :one
INSERT INTO dataset_lineage_parse_runs (
    tenant_id,
    query_job_id,
    requested_by_user_id
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: CompleteDatasetLineageParseRun :one
UPDATE dataset_lineage_parse_runs
SET
    status = 'completed',
    change_set_id = $3,
    table_ref_count = $4,
    column_edge_count = $5,
    error_summary = NULL,
    completed_at = now()
WHERE id = $1
  AND tenant_id = $2
RETURNING *;

-- name: FailDatasetLineageParseRun :one
UPDATE dataset_lineage_parse_runs
SET
    status = 'failed',
    error_summary = left($3, 1000),
    completed_at = now()
WHERE id = $1
  AND tenant_id = $2
RETURNING *;

-- name: ListDatasetLineageParseRuns :many
SELECT *
FROM dataset_lineage_parse_runs
WHERE tenant_id = $1
  AND query_job_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: GetDatasetByRawRefForTenant :one
SELECT *
FROM datasets
WHERE tenant_id = $1
  AND raw_database = $2
  AND raw_table = $3
  AND deleted_at IS NULL
LIMIT 1;
