-- name: UpsertDriveSearchDocument :one
INSERT INTO drive_search_documents (
    tenant_id,
    workspace_id,
    file_object_id,
    title,
    content_type,
    extracted_text,
    snippet,
    content_sha256,
    object_updated_at
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.narg(workspace_id),
    sqlc.arg(file_object_id),
    sqlc.arg(title),
    sqlc.arg(content_type),
    sqlc.arg(extracted_text),
    sqlc.arg(snippet),
    sqlc.narg(content_sha256),
    sqlc.narg(object_updated_at)
)
ON CONFLICT (file_object_id) DO UPDATE
SET
    workspace_id = EXCLUDED.workspace_id,
    title = EXCLUDED.title,
    content_type = EXCLUDED.content_type,
    extracted_text = EXCLUDED.extracted_text,
    snippet = EXCLUDED.snippet,
    content_sha256 = EXCLUDED.content_sha256,
    object_updated_at = EXCLUDED.object_updated_at,
    indexed_at = now(),
    updated_at = now()
RETURNING *;

-- name: DeleteDriveSearchDocument :exec
DELETE FROM drive_search_documents
WHERE tenant_id = sqlc.arg(tenant_id)
  AND file_object_id = sqlc.arg(file_object_id);

-- name: CreateDriveIndexJob :one
INSERT INTO drive_index_jobs (
    tenant_id,
    file_object_id,
    reason,
    status
) VALUES (
    sqlc.arg(tenant_id),
    sqlc.arg(file_object_id),
    sqlc.arg(reason),
    sqlc.arg(status)
)
RETURNING *;

-- name: SearchDriveIndexedFileCandidates :many
SELECT f.*
FROM file_objects f
LEFT JOIN drive_search_documents d ON d.file_object_id = f.id
WHERE f.tenant_id = sqlc.arg(tenant_id)
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
      OR d.search_vector @@ plainto_tsquery('simple', sqlc.narg(query)::text)
  )
ORDER BY
    CASE
        WHEN sqlc.narg(query)::text IS NULL THEN 0
        WHEN f.original_filename ILIKE '%' || sqlc.narg(query)::text || '%' THEN 0
        ELSE 1
    END ASC,
    f.updated_at DESC,
    f.id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListDriveSearchResults :many
SELECT
    f.*,
    COALESCE(d.snippet, '') AS search_snippet,
    d.indexed_at AS search_indexed_at
FROM file_objects f
LEFT JOIN drive_search_documents d ON d.file_object_id = f.id
WHERE f.tenant_id = sqlc.arg(tenant_id)
  AND f.purpose = 'drive'
  AND f.deleted_at IS NULL
  AND f.scan_status IN ('clean', 'skipped')
  AND f.dlp_blocked = false
  AND (
      sqlc.narg(content_type)::text IS NULL
      OR f.content_type = sqlc.narg(content_type)::text
  )
  AND (
      sqlc.narg(query)::text IS NULL
      OR f.original_filename ILIKE '%' || sqlc.narg(query)::text || '%'
      OR d.search_vector @@ plainto_tsquery('simple', sqlc.narg(query)::text)
  )
ORDER BY f.updated_at DESC, f.id DESC
LIMIT sqlc.arg(limit_count);
