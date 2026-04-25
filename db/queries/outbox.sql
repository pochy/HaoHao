-- name: CreateOutboxEvent :one
INSERT INTO outbox_events (
    tenant_id,
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    max_attempts
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ClaimOutboxEvents :many
UPDATE outbox_events
SET
    status = 'processing',
    locked_at = now(),
    locked_by = sqlc.arg(worker_id),
    attempts = attempts + 1,
    updated_at = now()
WHERE id IN (
    SELECT id
    FROM outbox_events
    WHERE status IN ('pending', 'failed')
      AND available_at <= now()
      AND attempts < max_attempts
    ORDER BY available_at, id
    LIMIT sqlc.arg(batch_size)
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkOutboxEventSent :one
UPDATE outbox_events
SET
    status = 'sent',
    locked_at = NULL,
    locked_by = NULL,
    last_error = NULL,
    processed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkOutboxEventRetry :one
UPDATE outbox_events
SET
    status = 'failed',
    locked_at = NULL,
    locked_by = NULL,
    last_error = left(sqlc.arg(last_error), 1000),
    available_at = now() + sqlc.arg(backoff),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: MarkOutboxEventDead :one
UPDATE outbox_events
SET
    status = 'dead',
    locked_at = NULL,
    locked_by = NULL,
    last_error = left(sqlc.arg(last_error), 1000),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteProcessedOutboxEventsBefore :execrows
DELETE FROM outbox_events
WHERE status IN ('sent', 'dead')
  AND updated_at < $1;
