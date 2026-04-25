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
