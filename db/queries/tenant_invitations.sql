-- name: CreateTenantInvitation :one
INSERT INTO tenant_invitations (
    tenant_id,
    invited_by_user_id,
    invitee_email_normalized,
    role_codes,
    token_hash,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListTenantInvitations :many
SELECT *
FROM tenant_invitations
WHERE tenant_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: GetTenantInvitationByPublicIDForTenant :one
SELECT *
FROM tenant_invitations
WHERE tenant_id = $1
  AND public_id = $2
LIMIT 1;

-- name: RenewTenantInvitationToken :one
UPDATE tenant_invitations
SET
    token_hash = $3,
    updated_at = now()
WHERE tenant_id = $1
  AND public_id = $2
  AND status = 'pending'
  AND expires_at > now()
RETURNING *;

-- name: GetPendingTenantInvitationByTokenHash :one
SELECT *
FROM tenant_invitations
WHERE token_hash = $1
  AND status = 'pending'
  AND expires_at > now();

-- name: GetPendingTenantInvitationByPublicIDAndTokenHash :one
SELECT *
FROM tenant_invitations
WHERE public_id = $1
  AND token_hash = $2
  AND status = 'pending'
  AND expires_at > now()
LIMIT 1;

-- name: ResolveTenantInvitationByTokenHash :one
SELECT
    i.*,
    t.slug AS tenant_slug,
    t.display_name AS tenant_display_name
FROM tenant_invitations i
JOIN tenants t ON t.id = i.tenant_id
WHERE i.token_hash = $1
LIMIT 1;

-- name: AcceptTenantInvitation :one
UPDATE tenant_invitations
SET
    status = 'accepted',
    accepted_by_user_id = $2,
    accepted_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'pending'
RETURNING *;

-- name: RevokeTenantInvitation :one
UPDATE tenant_invitations
SET
    status = 'revoked',
    revoked_at = now(),
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
  AND status = 'pending'
RETURNING *;

-- name: ExpireTenantInvitations :execrows
UPDATE tenant_invitations
SET
    status = 'expired',
    updated_at = now()
WHERE status = 'pending'
  AND expires_at <= now();
