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
WHERE recipient_user_id = $1
  AND ($2::bigint = 0 OR tenant_id = $2)
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: MarkNotificationRead :one
UPDATE notifications
SET
    status = 'read',
    read_at = COALESCE(read_at, now()),
    updated_at = now()
WHERE public_id = $1
  AND recipient_user_id = $2
RETURNING *;

-- name: DeleteReadNotificationsBefore :execrows
DELETE FROM notifications
WHERE read_at IS NOT NULL
  AND updated_at < $1;
