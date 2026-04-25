-- name: CreateSupportAccessSession :one
INSERT INTO support_access_sessions (
    support_user_id,
    impersonated_user_id,
    tenant_id,
    reason,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetSupportAccessSessionByID :one
SELECT
    sas.id,
    sas.public_id,
    sas.support_user_id,
    sas.impersonated_user_id,
    sas.tenant_id,
    sas.reason,
    sas.status,
    sas.started_at,
    sas.expires_at,
    sas.ended_at,
    sas.created_at,
    sas.updated_at,
    su.public_id AS support_user_public_id,
    su.email AS support_user_email,
    su.display_name AS support_user_display_name,
    iu.public_id AS impersonated_user_public_id,
    iu.email AS impersonated_user_email,
    iu.display_name AS impersonated_user_display_name,
    t.slug AS tenant_slug,
    t.display_name AS tenant_display_name
FROM support_access_sessions sas
JOIN users su ON su.id = sas.support_user_id
JOIN users iu ON iu.id = sas.impersonated_user_id
JOIN tenants t ON t.id = sas.tenant_id
WHERE sas.id = $1
LIMIT 1;

-- name: EndSupportAccessSession :one
UPDATE support_access_sessions
SET
    status = 'ended',
    ended_at = COALESCE(ended_at, now()),
    updated_at = now()
WHERE id = $1
  AND status = 'active'
RETURNING *;

-- name: ExpireSupportAccessSession :one
UPDATE support_access_sessions
SET
    status = 'expired',
    ended_at = COALESCE(ended_at, now()),
    updated_at = now()
WHERE id = $1
  AND status = 'active'
RETURNING *;
