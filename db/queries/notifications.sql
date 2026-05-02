-- name: CreateNotification :one
INSERT INTO notifications (
    tenant_id,
    recipient_user_id,
    channel,
    template,
    subject,
    body,
    metadata,
    outbox_event_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: ListNotificationsForUser :many
SELECT *
FROM notifications
WHERE recipient_user_id = sqlc.arg(recipient_user_id)
  AND (
    (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL)
    OR
    (sqlc.arg(tenant_id)::bigint <> 0 AND (tenant_id IS NULL OR tenant_id = sqlc.arg(tenant_id)))
  )
  AND (
    sqlc.narg(q)::text IS NULL
    OR btrim(sqlc.narg(q)::text) = ''
    OR to_tsvector('simple', subject || ' ' || body || ' ' || template)
       @@ websearch_to_tsquery('simple', sqlc.narg(q)::text)
  )
  AND (
    sqlc.arg(read_state)::text = 'all'
    OR (sqlc.arg(read_state)::text = 'unread' AND read_at IS NULL)
    OR (sqlc.arg(read_state)::text = 'read' AND read_at IS NOT NULL)
  )
  AND (
    sqlc.narg(channel)::text IS NULL
    OR channel = sqlc.narg(channel)::text
  )
  AND (
    sqlc.narg(created_after)::timestamptz IS NULL
    OR created_at >= sqlc.narg(created_after)::timestamptz
  )
  AND (
    sqlc.narg(cursor_created_at)::timestamptz IS NULL
    OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::bigint)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(result_limit);

-- name: CountNotificationSummaryForUser :one
SELECT
    COUNT(*)::bigint AS total_count,
    COUNT(*) FILTER (WHERE read_at IS NULL)::bigint AS unread_count,
    COUNT(*) FILTER (WHERE read_at IS NOT NULL)::bigint AS read_count
FROM notifications
WHERE recipient_user_id = sqlc.arg(recipient_user_id)
  AND (
    (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL)
    OR
    (sqlc.arg(tenant_id)::bigint <> 0 AND (tenant_id IS NULL OR tenant_id = sqlc.arg(tenant_id)))
  );

-- name: CountFilteredNotificationsForUser :one
SELECT COUNT(*)::bigint
FROM notifications
WHERE recipient_user_id = sqlc.arg(recipient_user_id)
  AND (
    (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL)
    OR
    (sqlc.arg(tenant_id)::bigint <> 0 AND (tenant_id IS NULL OR tenant_id = sqlc.arg(tenant_id)))
  )
  AND (
    sqlc.narg(q)::text IS NULL
    OR btrim(sqlc.narg(q)::text) = ''
    OR to_tsvector('simple', subject || ' ' || body || ' ' || template)
       @@ websearch_to_tsquery('simple', sqlc.narg(q)::text)
  )
  AND (
    sqlc.arg(read_state)::text = 'all'
    OR (sqlc.arg(read_state)::text = 'unread' AND read_at IS NULL)
    OR (sqlc.arg(read_state)::text = 'read' AND read_at IS NOT NULL)
  )
  AND (
    sqlc.narg(channel)::text IS NULL
    OR channel = sqlc.narg(channel)::text
  )
  AND (
    sqlc.narg(created_after)::timestamptz IS NULL
    OR created_at >= sqlc.narg(created_after)::timestamptz
  );

-- name: MarkNotificationRead :one
UPDATE notifications
SET
    status = 'read',
    read_at = COALESCE(read_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND recipient_user_id = $2
RETURNING *;

-- name: MarkNotificationsReadByPublicIDs :many
UPDATE notifications
SET
    status = 'read',
    read_at = COALESCE(read_at, now()),
    updated_at = now()
WHERE recipient_user_id = sqlc.arg(recipient_user_id)
  AND public_id = ANY(sqlc.arg(public_ids)::uuid[])
  AND read_at IS NULL
RETURNING *;

-- name: MarkFilteredNotificationsRead :many
UPDATE notifications
SET
    status = 'read',
    read_at = COALESCE(read_at, now()),
    updated_at = now()
WHERE recipient_user_id = sqlc.arg(recipient_user_id)
  AND read_at IS NULL
  AND (
    (sqlc.arg(tenant_id)::bigint = 0 AND tenant_id IS NULL)
    OR
    (sqlc.arg(tenant_id)::bigint <> 0 AND (tenant_id IS NULL OR tenant_id = sqlc.arg(tenant_id)))
  )
  AND (
    sqlc.narg(q)::text IS NULL
    OR btrim(sqlc.narg(q)::text) = ''
    OR to_tsvector('simple', subject || ' ' || body || ' ' || template)
       @@ websearch_to_tsquery('simple', sqlc.narg(q)::text)
  )
  AND (
    sqlc.arg(read_state)::text = 'all'
    OR (sqlc.arg(read_state)::text = 'unread' AND read_at IS NULL)
    OR (sqlc.arg(read_state)::text = 'read' AND read_at IS NOT NULL)
  )
  AND (
    sqlc.narg(channel)::text IS NULL
    OR channel = sqlc.narg(channel)::text
  )
  AND (
    sqlc.narg(created_after)::timestamptz IS NULL
    OR created_at >= sqlc.narg(created_after)::timestamptz
  )
RETURNING *;

-- name: DeleteReadNotificationsBefore :execrows
DELETE FROM notifications
WHERE read_at IS NOT NULL
  AND updated_at < $1;
