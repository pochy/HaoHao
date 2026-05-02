-- name: CreateRealtimeEvent :one
INSERT INTO realtime_events (
    tenant_id,
    recipient_user_id,
    event_type,
    resource_type,
    resource_public_id,
    payload,
    expires_at
) VALUES (
    sqlc.narg(tenant_id),
    sqlc.arg(recipient_user_id),
    sqlc.arg(event_type),
    sqlc.arg(resource_type),
    sqlc.arg(resource_public_id),
    sqlc.arg(payload),
    sqlc.arg(expires_at)
)
RETURNING *;

-- name: ListRealtimeEventsAfterCursor :many
SELECT *
FROM realtime_events
WHERE recipient_user_id = sqlc.arg(recipient_user_id)
  AND id > sqlc.arg(after_id)
  AND expires_at > now()
  AND (
    (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL)
    OR
    (sqlc.arg(tenant_id)::bigint <> 0 AND (tenant_id IS NULL OR tenant_id = sqlc.arg(tenant_id)))
  )
ORDER BY id ASC
LIMIT sqlc.arg(limit_count);

-- name: GetRealtimeCurrentCursor :one
SELECT COALESCE(MAX(id), 0)::bigint
FROM realtime_events
WHERE recipient_user_id = sqlc.arg(recipient_user_id)
  AND expires_at > now()
  AND (
    (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL)
    OR
    (sqlc.arg(tenant_id)::bigint <> 0 AND (tenant_id IS NULL OR tenant_id = sqlc.arg(tenant_id)))
  );

-- name: DeleteExpiredRealtimeEvents :execrows
DELETE FROM realtime_events
WHERE expires_at < now();
