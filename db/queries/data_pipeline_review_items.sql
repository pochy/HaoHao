-- name: UpsertDataPipelineReviewItem :one
INSERT INTO data_pipeline_review_items (
    tenant_id,
    pipeline_id,
    version_id,
    run_id,
    node_id,
    queue,
    reason,
    source_snapshot,
    source_fingerprint,
    created_by_user_id,
    updated_by_user_id
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(pipeline_id),
    sqlc.arg(version_id),
    sqlc.arg(run_id),
    sqlc.arg(node_id),
    sqlc.arg(queue),
    COALESCE(sqlc.arg(reason)::jsonb, '[]'::jsonb),
    COALESCE(sqlc.arg(source_snapshot)::jsonb, '{}'::jsonb),
    sqlc.arg(source_fingerprint),
    sqlc.narg(created_by_user_id),
    sqlc.narg(updated_by_user_id)
)
ON CONFLICT (tenant_id, run_id, node_id, source_fingerprint)
DO UPDATE
SET
    queue = EXCLUDED.queue,
    reason = EXCLUDED.reason,
    source_snapshot = EXCLUDED.source_snapshot,
    updated_by_user_id = EXCLUDED.updated_by_user_id,
    updated_at = now()
RETURNING *;

-- name: ListDataPipelineReviewItems :many
SELECT ri.*
FROM data_pipeline_review_items ri
JOIN data_pipelines p ON p.id = ri.pipeline_id
WHERE ri.tenant_id = sqlc.arg(tenant_id)
  AND p.public_id = sqlc.arg(pipeline_public_id)
  AND (
      sqlc.narg(status)::text IS NULL
      OR ri.status = sqlc.narg(status)::text
  )
ORDER BY ri.created_at DESC, ri.id DESC
LIMIT sqlc.arg(result_limit);

-- name: GetDataPipelineReviewItemForTenant :one
SELECT *
FROM data_pipeline_review_items
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
LIMIT 1;

-- name: TransitionDataPipelineReviewItem :one
UPDATE data_pipeline_review_items
SET
    status = sqlc.arg(status),
    decision_comment = sqlc.arg(decision_comment),
    updated_by_user_id = sqlc.narg(updated_by_user_id),
    decided_at = CASE
        WHEN sqlc.arg(status)::text IN ('approved', 'rejected', 'needs_changes', 'closed') THEN now()
        ELSE decided_at
    END,
    updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND public_id = sqlc.arg(public_id)
RETURNING *;

-- name: CreateDataPipelineReviewItemComment :one
INSERT INTO data_pipeline_review_item_comments (
    tenant_id,
    review_item_id,
    author_user_id,
    body
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(review_item_id),
    sqlc.narg(author_user_id),
    sqlc.arg(body)
)
RETURNING *;

-- name: ListDataPipelineReviewItemComments :many
SELECT *
FROM data_pipeline_review_item_comments
WHERE tenant_id = sqlc.arg(tenant_id)
  AND review_item_id = sqlc.arg(review_item_id)
ORDER BY created_at ASC, id ASC;
