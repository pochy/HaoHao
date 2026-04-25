-- name: UpsertOAuthUserGrant :one
INSERT INTO oauth_user_grants (
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
ON CONFLICT (user_id, provider, resource_server, tenant_id) DO UPDATE
SET provider_subject = EXCLUDED.provider_subject,
    refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
    refresh_token_key_version = EXCLUDED.refresh_token_key_version,
    scope_text = EXCLUDED.scope_text,
    granted_by_session_id = EXCLUDED.granted_by_session_id,
    granted_at = now(),
    last_refreshed_at = NULL,
    revoked_at = NULL,
    last_error_code = NULL,
    updated_at = now()
RETURNING
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at;

-- name: GetOAuthUserGrant :one
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4
LIMIT 1;

-- name: GetActiveOAuthUserGrant :one
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4
  AND revoked_at IS NULL
LIMIT 1;

-- name: ListOAuthUserGrantsByUserID :many
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
ORDER BY resource_server, provider;

-- name: UpdateOAuthUserGrantAfterRefresh :one
UPDATE oauth_user_grants
SET refresh_token_ciphertext = $5,
    refresh_token_key_version = $6,
    scope_text = $7,
    last_refreshed_at = now(),
    last_error_code = NULL,
    updated_at = now()
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4
  AND revoked_at IS NULL
RETURNING
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at;

-- name: MarkOAuthUserGrantRevoked :exec
UPDATE oauth_user_grants
SET revoked_at = now(),
    last_error_code = $5,
    updated_at = now()
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4;

-- name: DeleteOAuthUserGrant :exec
DELETE FROM oauth_user_grants
WHERE user_id = $1
  AND tenant_id = $2
  AND provider = $3
  AND resource_server = $4;

-- name: ListActiveOAuthUserGrantsByUserID :many
SELECT
    id,
    user_id,
    tenant_id,
    provider,
    resource_server,
    provider_subject,
    refresh_token_ciphertext,
    refresh_token_key_version,
    scope_text,
    granted_by_session_id,
    granted_at,
    last_refreshed_at,
    revoked_at,
    last_error_code,
    created_at,
    updated_at
FROM oauth_user_grants
WHERE user_id = $1
  AND revoked_at IS NULL
ORDER BY tenant_id, resource_server, provider;

-- name: DeleteOAuthUserGrantsByUserID :exec
DELETE FROM oauth_user_grants
WHERE user_id = $1;
