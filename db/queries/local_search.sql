-- name: UpsertLocalSearchDocument :one
INSERT INTO local_search_documents (
    tenant_id,
    resource_kind,
    resource_id,
    resource_public_id,
    file_object_id,
    medallion_asset_id,
    gold_publication_id,
    title,
    body_text,
    snippet,
    content_hash,
    source_updated_at
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(resource_kind),
    sqlc.arg(resource_id),
    sqlc.arg(resource_public_id),
    sqlc.narg(file_object_id),
    sqlc.narg(medallion_asset_id),
    sqlc.narg(gold_publication_id),
    sqlc.arg(title),
    sqlc.arg(body_text),
    sqlc.arg(snippet),
    sqlc.arg(content_hash),
    sqlc.narg(source_updated_at)
)
ON CONFLICT (tenant_id, resource_kind, resource_id) DO UPDATE
SET
    resource_public_id = EXCLUDED.resource_public_id,
    file_object_id = EXCLUDED.file_object_id,
    medallion_asset_id = EXCLUDED.medallion_asset_id,
    gold_publication_id = EXCLUDED.gold_publication_id,
    title = EXCLUDED.title,
    body_text = EXCLUDED.body_text,
    snippet = EXCLUDED.snippet,
    content_hash = EXCLUDED.content_hash,
    source_updated_at = EXCLUDED.source_updated_at,
    indexed_at = now(),
    updated_at = now()
RETURNING *;

-- name: DeleteLocalSearchDocumentForResource :exec
DELETE FROM local_search_documents
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_kind = sqlc.arg(resource_kind)
  AND resource_id = sqlc.arg(resource_id);

-- name: DeleteLocalSearchDocumentsForFile :exec
DELETE FROM local_search_documents
WHERE tenant_id = sqlc.arg(tenant_id)
  AND file_object_id = sqlc.arg(file_object_id);

-- name: GetLocalSearchDocumentByIDForTenant :one
SELECT *
FROM local_search_documents
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
LIMIT 1;

-- name: GetLocalSearchDocumentForResource :one
SELECT *
FROM local_search_documents
WHERE tenant_id = sqlc.arg(tenant_id)
  AND resource_kind = sqlc.arg(resource_kind)
  AND resource_id = sqlc.arg(resource_id)
LIMIT 1;

-- name: SearchLocalSearchDriveFileCandidates :many
WITH candidate_files AS (
    SELECT
        d.file_object_id,
        max(
            CASE
                WHEN sqlc.narg(query)::text IS NULL THEN 0
                ELSE ts_rank_cd(d.search_vector, websearch_to_tsquery('simple', sqlc.narg(query)::text))
            END
        ) AS rank,
        max(d.indexed_at) AS latest_indexed_at
    FROM local_search_documents d
    JOIN file_objects f ON f.id = d.file_object_id
    WHERE d.tenant_id = sqlc.arg(tenant_id)
      AND d.file_object_id IS NOT NULL
      AND f.tenant_id = sqlc.arg(tenant_id)
      AND f.purpose = 'drive'
      AND f.deleted_at IS NULL
      AND f.scan_status IN ('clean', 'skipped')
      AND f.dlp_blocked = false
      AND (
          sqlc.narg(content_type)::text IS NULL
          OR f.content_type = sqlc.narg(content_type)::text
      )
      AND (
          sqlc.narg(updated_after)::timestamptz IS NULL
          OR f.updated_at >= sqlc.narg(updated_after)::timestamptz
      )
      AND (
          sqlc.narg(updated_before)::timestamptz IS NULL
          OR f.updated_at <= sqlc.narg(updated_before)::timestamptz
      )
      AND (
          sqlc.narg(query)::text IS NULL
          OR f.original_filename ILIKE '%' || sqlc.narg(query)::text || '%'
          OR d.title ILIKE '%' || sqlc.narg(query)::text || '%'
          OR d.body_text ILIKE '%' || sqlc.narg(query)::text || '%'
          OR d.search_vector @@ websearch_to_tsquery('simple', sqlc.narg(query)::text)
      )
    GROUP BY d.file_object_id
)
SELECT f.*
FROM candidate_files c
JOIN file_objects f ON f.id = c.file_object_id
ORDER BY c.rank DESC, c.latest_indexed_at DESC, f.updated_at DESC, f.id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListLocalSearchMatchesForFile :many
SELECT
    d.resource_kind,
    d.resource_public_id::text AS resource_public_id,
    COALESCE(m.public_id::text, '') AS medallion_asset_public_id,
    COALESCE(m.layer, '') AS layer,
    d.snippet,
    d.indexed_at
FROM local_search_documents d
LEFT JOIN medallion_assets m ON m.id = d.medallion_asset_id
WHERE d.tenant_id = sqlc.arg(tenant_id)
  AND d.file_object_id = sqlc.arg(file_object_id)
  AND (
      sqlc.narg(query)::text IS NULL
      OR d.title ILIKE '%' || sqlc.narg(query)::text || '%'
      OR d.body_text ILIKE '%' || sqlc.narg(query)::text || '%'
      OR d.search_vector @@ websearch_to_tsquery('simple', sqlc.narg(query)::text)
  )
ORDER BY
    CASE
        WHEN sqlc.narg(query)::text IS NULL THEN 0
        ELSE ts_rank_cd(d.search_vector, websearch_to_tsquery('simple', sqlc.narg(query)::text))
    END DESC,
    d.indexed_at DESC,
    d.id DESC
LIMIT sqlc.arg(limit_count);

-- name: CreateLocalSearchIndexJob :one
INSERT INTO local_search_index_jobs (
    tenant_id,
    resource_kind,
    resource_id,
    resource_public_id,
    reason
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.narg(resource_kind),
    sqlc.narg(resource_id),
    sqlc.narg(resource_public_id),
    sqlc.arg(reason)
)
RETURNING *;

-- name: LinkLocalSearchIndexJobOutboxEvent :one
UPDATE local_search_index_jobs
SET
    outbox_event_id = sqlc.arg(outbox_event_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: MarkLocalSearchIndexJobProcessing :one
UPDATE local_search_index_jobs
SET
    status = 'processing',
    outbox_event_id = sqlc.arg(outbox_event_id),
    attempts = attempts + 1,
    started_at = COALESCE(started_at, now()),
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND status IN ('queued', 'processing', 'failed')
RETURNING *;

-- name: CompleteLocalSearchIndexJob :one
UPDATE local_search_index_jobs
SET
    status = sqlc.arg(status),
    indexed_count = sqlc.arg(indexed_count),
    skipped_count = sqlc.arg(skipped_count),
    failed_count = sqlc.arg(failed_count),
    last_error = NULL,
    completed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: FailLocalSearchIndexJob :one
UPDATE local_search_index_jobs
SET
    status = 'failed',
    failed_count = GREATEST(failed_count, 1),
    last_error = left(sqlc.arg(last_error), 1000),
    completed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: ListLocalSearchIndexJobs :many
SELECT *
FROM local_search_index_jobs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (
      sqlc.narg(status)::text IS NULL
      OR status = sqlc.narg(status)::text
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: GetLocalSearchIndexJobByIDForTenant :one
SELECT *
FROM local_search_index_jobs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
LIMIT 1;

-- name: UpsertLocalSearchEmbedding :one
INSERT INTO local_search_embeddings (
    tenant_id,
    resource_kind,
    resource_id,
    resource_public_id,
    document_id,
    chunk_ordinal,
    source_text,
    model,
    dimension,
    content_hash,
    embedding,
    metadata,
    indexed_at,
    status,
    error_summary
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(resource_kind),
    sqlc.arg(resource_id),
    sqlc.arg(resource_public_id),
    sqlc.arg(document_id),
    sqlc.arg(chunk_ordinal),
    sqlc.arg(source_text),
    sqlc.arg(model),
    sqlc.arg(dimension),
    sqlc.arg(content_hash),
    sqlc.narg(embedding)::vector,
    COALESCE(sqlc.narg(metadata)::jsonb, '{}'::jsonb),
    CASE WHEN sqlc.narg(embedding)::text IS NULL THEN NULL ELSE now() END,
    sqlc.arg(status),
    sqlc.narg(error_summary)
)
ON CONFLICT (document_id, chunk_ordinal, model, content_hash) DO UPDATE
SET
    tenant_id = EXCLUDED.tenant_id,
    resource_kind = EXCLUDED.resource_kind,
    resource_id = EXCLUDED.resource_id,
    resource_public_id = EXCLUDED.resource_public_id,
    source_text = EXCLUDED.source_text,
    dimension = EXCLUDED.dimension,
    embedding = EXCLUDED.embedding,
    metadata = EXCLUDED.metadata,
    indexed_at = EXCLUDED.indexed_at,
    status = EXCLUDED.status,
    error_summary = EXCLUDED.error_summary,
    updated_at = now()
RETURNING *;

-- name: DeleteLocalSearchEmbeddingsForDocument :exec
DELETE FROM local_search_embeddings
WHERE tenant_id = sqlc.arg(tenant_id)
  AND document_id = sqlc.arg(document_id);

-- name: ListPendingLocalSearchEmbeddings :many
SELECT d.*
FROM local_search_documents d
WHERE d.tenant_id = sqlc.arg(tenant_id)
  AND (
      sqlc.narg(resource_kind)::text IS NULL
      OR d.resource_kind = sqlc.narg(resource_kind)::text
  )
  AND NOT EXISTS (
      SELECT 1
      FROM local_search_embeddings e
      WHERE e.document_id = d.id
        AND e.model = sqlc.arg(model)
        AND e.content_hash = d.content_hash
        AND e.status = 'completed'
  )
ORDER BY d.indexed_at ASC, d.id ASC
LIMIT sqlc.arg(limit_count);

-- name: SearchLocalSearchEmbeddingsCosine :many
SELECT
    e.document_id,
    e.resource_kind,
    e.resource_id,
    e.resource_public_id::text AS resource_public_id,
    e.chunk_ordinal,
    e.source_text,
    e.model,
    e.content_hash,
    (1.0 - (e.embedding <=> sqlc.arg(query_embedding)::vector))::float8 AS score,
    e.indexed_at
FROM local_search_embeddings e
WHERE e.tenant_id = sqlc.arg(tenant_id)
  AND e.resource_kind = sqlc.arg(resource_kind)
  AND e.model = sqlc.arg(model)
  AND e.status = 'completed'
  AND e.embedding IS NOT NULL
ORDER BY e.embedding <=> sqlc.arg(query_embedding)::vector
LIMIT sqlc.arg(limit_count);

-- name: GetDriveProductExtractionItemByIDForTenant :one
SELECT *
FROM drive_product_extraction_items
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id = sqlc.arg(id)
LIMIT 1;

-- name: ListLocalSearchRebuildDriveFiles :many
SELECT *
FROM file_objects
WHERE tenant_id = sqlc.arg(tenant_id)
  AND purpose = 'drive'
ORDER BY id
LIMIT sqlc.arg(limit_count);

-- name: ListLocalSearchRebuildOCRRuns :many
SELECT *
FROM drive_ocr_runs
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status = 'completed'
ORDER BY id
LIMIT sqlc.arg(limit_count);

-- name: ListLocalSearchRebuildProductExtractionItems :many
SELECT *
FROM drive_product_extraction_items
WHERE tenant_id = sqlc.arg(tenant_id)
ORDER BY id
LIMIT sqlc.arg(limit_count);

-- name: ListLocalSearchRebuildGoldPublications :many
SELECT *
FROM dataset_gold_publications
WHERE tenant_id = sqlc.arg(tenant_id)
ORDER BY id
LIMIT sqlc.arg(limit_count);
