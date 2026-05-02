-- name: ListCustomerSignalSavedFilters :many
SELECT *
FROM customer_signal_saved_filters
WHERE tenant_id = $1
  AND owner_user_id = $2
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: SearchCustomerSignalSavedFilters :many
SELECT *
FROM customer_signal_saved_filters
WHERE tenant_id = sqlc.arg(tenant_id)
  AND owner_user_id = sqlc.arg(owner_user_id)
  AND deleted_at IS NULL
  AND (
      sqlc.narg(q)::text IS NULL
      OR btrim(sqlc.narg(q)::text) = ''
      OR to_tsvector('simple', name || ' ' || query || ' ' || filters::text)
         @@ websearch_to_tsquery('simple', sqlc.narg(q)::text)
  )
  AND (
      sqlc.narg(status)::text IS NULL
      OR filters->>'status' = sqlc.narg(status)::text
  )
  AND (
      sqlc.narg(priority)::text IS NULL
      OR filters->>'priority' = sqlc.narg(priority)::text
  )
  AND (
      sqlc.narg(source)::text IS NULL
      OR filters->>'source' = sqlc.narg(source)::text
  )
  AND (
      sqlc.narg(cursor_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::bigint)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(result_limit);

-- name: GetCustomerSignalSavedFilterForOwner :one
SELECT *
FROM customer_signal_saved_filters
WHERE public_id = $1
  AND tenant_id = $2
  AND owner_user_id = $3
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateCustomerSignalSavedFilter :one
INSERT INTO customer_signal_saved_filters (
    tenant_id,
    owner_user_id,
    name,
    query,
    filters
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpdateCustomerSignalSavedFilter :one
UPDATE customer_signal_saved_filters
SET
    name = $4,
    query = $5,
    filters = $6,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND owner_user_id = $3
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteCustomerSignalSavedFilter :execrows
UPDATE customer_signal_saved_filters
SET
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND owner_user_id = $3
  AND deleted_at IS NULL;
