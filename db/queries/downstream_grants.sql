-- name: UpsertOAuthUserGrant :one
INSERT INTO oauth_user_grants (
    user_id,
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
    $8
)
ON CONFLICT (user_id, provider, resource_server) DO UPDATE
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
  AND provider = $2
  AND resource_server = $3
LIMIT 1;

-- name: GetActiveOAuthUserGrant :one
SELECT
    id,
    user_id,
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
  AND provider = $2
  AND resource_server = $3
  AND revoked_at IS NULL
LIMIT 1;

-- name: ListOAuthUserGrantsByUserID :many
SELECT
    id,
    user_id,
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
ORDER BY resource_server, provider;

-- name: UpdateOAuthUserGrantAfterRefresh :one
UPDATE oauth_user_grants
SET refresh_token_ciphertext = $4,
    refresh_token_key_version = $5,
    scope_text = $6,
    last_refreshed_at = now(),
    last_error_code = NULL,
    updated_at = now()
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3
  AND revoked_at IS NULL
RETURNING
    id,
    user_id,
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
    last_error_code = $4,
    updated_at = now()
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3;

-- name: DeleteOAuthUserGrant :exec
DELETE FROM oauth_user_grants
WHERE user_id = $1
  AND provider = $2
  AND resource_server = $3;
