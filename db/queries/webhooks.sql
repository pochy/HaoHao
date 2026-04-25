-- name: ListWebhookEndpoints :many
SELECT *
FROM webhook_endpoints
WHERE tenant_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: ListActiveWebhookEndpointsForEvent :many
SELECT *
FROM webhook_endpoints
WHERE tenant_id = $1
  AND active = true
  AND deleted_at IS NULL
  AND (sqlc.arg(event_type)::text = ANY(event_types))
ORDER BY created_at ASC, id ASC;

-- name: GetWebhookEndpointForTenant :one
SELECT *
FROM webhook_endpoints
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (
    tenant_id,
    created_by_user_id,
    name,
    url,
    event_types,
    secret_ciphertext,
    secret_key_version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpdateWebhookEndpoint :one
UPDATE webhook_endpoints
SET
    name = $3,
    url = $4,
    event_types = $5,
    active = $6,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: RotateWebhookSecret :one
UPDATE webhook_endpoints
SET
    secret_ciphertext = $3,
    secret_key_version = $4,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteWebhookEndpoint :one
UPDATE webhook_endpoints
SET
    active = false,
    deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
    tenant_id,
    webhook_endpoint_id,
    event_type,
    payload,
    max_attempts
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: SetWebhookDeliveryOutboxEvent :one
UPDATE webhook_deliveries
SET
    outbox_event_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetWebhookDeliveryByID :one
SELECT
    wd.id,
    wd.public_id,
    wd.tenant_id,
    wd.webhook_endpoint_id,
    wd.outbox_event_id,
    wd.event_type,
    wd.payload,
    wd.status,
    wd.attempt_count,
    wd.max_attempts,
    wd.next_attempt_at,
    wd.last_http_status,
    wd.last_error,
    wd.response_preview,
    wd.delivered_at,
    wd.created_at,
    wd.updated_at,
    we.public_id AS endpoint_public_id,
    we.name AS endpoint_name,
    we.url AS endpoint_url,
    we.secret_ciphertext AS endpoint_secret_ciphertext,
    we.secret_key_version AS endpoint_secret_key_version,
    we.active AS endpoint_active
FROM webhook_deliveries wd
JOIN webhook_endpoints we ON we.id = wd.webhook_endpoint_id
WHERE wd.id = $1
LIMIT 1;

-- name: ListWebhookDeliveriesForEndpoint :many
SELECT *
FROM webhook_deliveries
WHERE tenant_id = $1
  AND webhook_endpoint_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: GetWebhookDeliveryForTenant :one
SELECT *
FROM webhook_deliveries
WHERE public_id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: MarkWebhookDeliveryDelivered :one
UPDATE webhook_deliveries
SET
    status = 'delivered',
    attempt_count = attempt_count + 1,
    last_http_status = $2,
    last_error = NULL,
    response_preview = left($3, 1000),
    delivered_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkWebhookDeliveryFailed :one
UPDATE webhook_deliveries
SET
    status = CASE WHEN attempt_count + 1 >= max_attempts THEN 'dead' ELSE 'failed' END,
    attempt_count = attempt_count + 1,
    last_http_status = $2,
    last_error = left($3, 1000),
    response_preview = left($4, 1000),
    next_attempt_at = now() + $5::interval,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ResetWebhookDeliveryForRetry :one
UPDATE webhook_deliveries
SET
    status = 'pending',
    next_attempt_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
RETURNING *;

-- name: TouchWebhookEndpointDelivery :one
UPDATE webhook_endpoints
SET
    last_delivery_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;
