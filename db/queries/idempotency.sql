-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (
    tenant_id,
    actor_user_id,
    scope,
    idempotency_key_hash,
    method,
    path,
    request_hash,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetIdempotencyKeyForScope :one
SELECT *
FROM idempotency_keys
WHERE scope = $1
  AND idempotency_key_hash = $2
  AND expires_at > now();

-- name: CompleteIdempotencyKey :one
UPDATE idempotency_keys
SET
    status = 'completed',
    response_status = $2,
    response_summary = $3,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailIdempotencyKey :one
UPDATE idempotency_keys
SET
    status = 'failed',
    response_status = $2,
    response_summary = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys
WHERE expires_at <= now();
