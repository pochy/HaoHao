-- name: GetUserByProviderSubject :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.subject = $2
LIMIT 1;

-- name: GetUserIdentityByUserIDProvider :one
SELECT
    id,
    user_id,
    provider,
    subject,
    email,
    email_verified,
    external_id,
    provisioning_source,
    created_at,
    updated_at
FROM user_identities
WHERE user_id = $1
  AND provider = $2
LIMIT 1;

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: CreateProvisionedUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    subject,
    email,
    email_verified,
    external_id,
    provisioning_source
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
);

-- name: UpdateUserIdentityProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;

-- name: UpdateUserIdentityProvisioningProfile :exec
UPDATE user_identities
SET email = $3,
    email_verified = $4,
    external_id = $5,
    provisioning_source = $6,
    updated_at = now()
WHERE provider = $1
  AND subject = $2;

-- name: GetProvisionedUserByExternalID :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id,
    ui.id AS identity_id,
    ui.provider,
    ui.subject,
    ui.external_id,
    ui.provisioning_source
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND ui.external_id = $2
LIMIT 1;

-- name: GetProvisionedUserByPublicID :one
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id,
    ui.id AS identity_id,
    ui.provider,
    ui.subject,
    ui.external_id,
    ui.provisioning_source
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
  AND u.public_id = $2
LIMIT 1;

-- name: ListProvisionedUsers :many
SELECT
    u.id,
    u.public_id,
    u.email,
    u.display_name,
    u.deactivated_at,
    u.default_tenant_id,
    ui.id AS identity_id,
    ui.provider,
    ui.subject,
    ui.external_id,
    ui.provisioning_source
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = $1
ORDER BY u.id
LIMIT $2
OFFSET $3;
