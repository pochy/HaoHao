-- name: ListCustomerSignalsByTenantID :many
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status,
    created_at,
    updated_at,
    deleted_at
FROM customer_signals
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: SearchCustomerSignals :many
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status,
    created_at,
    updated_at,
    deleted_at
FROM customer_signals
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
  AND (
      sqlc.narg(q)::text IS NULL
      OR btrim(sqlc.narg(q)::text) = ''
      OR to_tsvector('simple', customer_name || ' ' || title || ' ' || body)
         @@ websearch_to_tsquery('simple', sqlc.narg(q)::text)
  )
  AND (
      sqlc.narg(status)::text IS NULL
      OR status = sqlc.narg(status)::text
  )
  AND (
      sqlc.narg(priority)::text IS NULL
      OR priority = sqlc.narg(priority)::text
  )
  AND (
      sqlc.narg(source)::text IS NULL
      OR source = sqlc.narg(source)::text
  )
  AND (
      sqlc.narg(cursor_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::bigint)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(result_limit);

-- name: GetCustomerSignalByPublicIDForTenant :one
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status,
    created_at,
    updated_at,
    deleted_at
FROM customer_signals
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateCustomerSignal :one
INSERT INTO customer_signals (
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status,
    created_at,
    updated_at,
    deleted_at;

-- name: UpdateCustomerSignalByPublicIDForTenant :one
UPDATE customer_signals
SET
    customer_name = $3,
    title = $4,
    body = $5,
    source = $6,
    priority = $7,
    status = $8,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    customer_name,
    title,
    body,
    source,
    priority,
    status,
    created_at,
    updated_at,
    deleted_at;

-- name: SoftDeleteCustomerSignalByPublicIDForTenant :execrows
UPDATE customer_signals
SET
    deleted_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL;
